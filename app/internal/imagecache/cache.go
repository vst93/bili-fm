// Package imagecache 按 URL 抓取、解码并缓存封面图，供原生界面直接使用。
//
// 与旧版的区别：旧版走本地 HTTP 代理（127.0.0.1:4654/image-proxy），因为
// webview 里的 <img> 没法带 Referer。原生界面在 Go 里抓，可以直接带头，
// 所以不需要代理；那个代理留给视频弹窗（webview）用。
package imagecache

import (
	"bytes"
	"image"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// 与旧版 src/utils/string.tsx 的 graftingImage 一致：B 站图床支持在 URL
// 后面加 @<n>w.webp 让服务端下采样，单张从几百 KB 降到几 KB。
//
// 宽度**按调用点的展示尺寸分别传**（原版 graftingImage 的 width 参数），
// 不再所有封面一律 240：列表卡片 240、主播放器封面 480、氛围光 320、
// 播放列表缩略图 192、各类头像 96。原版注释记录过 400w→300w→240w 的
// 降级（解码位图是内存大头），但那只针对卡片列表；其余调用点是显式宽度。
const (
	// WidthCard 是列表卡片封面（原版 graftingImage 默认 240w）。
	// 卡片封面上限 245 CSS px（800 窗宽、抽屉 24 内边距、3 列、8 间距），
	// 240 ≥ 原版注释里的 229，100% 缩放下基本不欠采样。
	WidthCard = 240
	// WidthPlayer 是主播放器封面 / 唱片（原版显式 480；碟片展示 263 CSS px，
	// 480 覆盖 2x 屏 526 设备像素）。
	WidthPlayer = 480
	// WidthAmbient 是背景氛围光（原版显式 320）。
	WidthAmbient = 320
	// WidthPlaylist 是播放列表行的缩略图（原版显式 192；展示 64×36）。
	WidthPlaylist = 192
	// WidthAvatar 是各类头像（原版显式 96；展示 32–40 CSS px）。
	WidthAvatar = 96
)

const (
	ua      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	referer = "https://www.bilibili.com/"
)

// key 定位缓存里的一张图：同一原始 URL 会按不同宽度各存一份
//（原版各调用点各自走 graftingImage，URL 后缀不同即不同请求）。
type key struct {
	url   string
	width int
}

// Cache 是一组封面图，按（URL, 宽度）索引，带容量上限。
type Cache struct {
	// Client 抓图用的 HTTP 客户端。为 nil 时用带 15s 超时的默认客户端；
	// 测试可替换它指向本地服务器（封面地址可以是任意图床）。
	Client *http.Client

	// limit 是缓存位图数上限，超出后按最近最少使用淘汰。默认 cacheLimit：
	// 原版按「滚出 2 屏就丢位图」主动卸载，这里用 LRU 等效兜底，避免长列表
	// 越滚位图越积（长列表 300 张 240w 卡片全存下约 57MB）。
	limit int

	mu      sync.Mutex
	items   map[key]*entry
	order   []key // 最近使用的在后
	notify  func() // 有新图时请求重绘
	loading int   // 正在抓取的张数（Pending 用）

	// sem 限制同时进行的抓取 + 解码数：打开抽屉会请求可见的一批图，
	// 不设限就是几十个 goroutine 同时 decode，抢走渲染线程的 CPU，
	// 也让 GPU 纹理一次性集中上传。
	sem chan struct{}
}

type entry struct {
	bitmap *ui.Bitmap
	// small 是同一张图的缩小版，只为氛围光的调色留一份源
	//（见 ambient.go）。64×64 的图像约 16KB。
	small   *image.RGBA
	failed  bool
	pending bool
	// failedAt 是上次失败的时间，用于冷却后再试（原版 RetryImg 会重试）。
	failedAt time.Time
}

// New 建一个缓存。notify 会在某张图就绪时被调用，用来触发重绘；
// 它可能在任意 goroutine 上被调用。
func New(notify func()) *Cache {
	return &Cache{
		Client: &http.Client{Timeout: 15 * time.Second},
		limit:  cacheLimit,
		items:  map[key]*entry{},
		notify: notify,
		sem:    make(chan struct{}, maxConcurrentFetches),
	}
}

// cacheLimit 是缓存位图数上限（条目数，不是字节数）。
//
// 对齐原版 useViewportImageUnload 的意图：滚出视口较远的卡片位图要能释放。
// mygo 的 ui.List 回收行后不会通知缓存，所以用有界 LRU 等效兜底：
// 一张 240w 卡片的解码位图约 190KB（240×135×4 + 64px 小图 + 半采样层），
// 64 条约 12MB，够覆盖 4 屏卡片（15 张/屏）；再远就淘汰，需要时重新抓。
// 从前是 300 条（约 57MB），长列表从头滚到尾会把所有位图都攒下来。
const cacheLimit = 64

// maxConcurrentFetches 是同时抓取 / 解码的封面数。
const maxConcurrentFetches = 4

// httpClient 返回抓图用的客户端，未显式设置时给出默认的。
func (c *Cache) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

// 失败重试的冷却时间：太短会在每帧重发请求，太长则封面一直空着。
const retryFailedAfter = 15 * time.Second

// Bitmap 立即返回缓存的位图；未命中时返回 nil，并在后台抓取，就绪后调用
// notify。视图每帧都会调用它，所以这里绝不能阻塞。width 是服务端下采样的
// 目标宽度（B 站图床的 @<width>w.webp）。
func (c *Cache) Bitmap(rawURL string, width int) *ui.Bitmap {
	if rawURL == "" {
		return nil
	}
	k := key{rawURL, width}
	c.mu.Lock()
	e := c.items[k]
	if e != nil {
		// 之前失败过，过了冷却期就重新抓（原版 RetryImg 的重试）。
		if e.failed && !e.pending && time.Since(e.failedAt) > retryFailedAfter {
			e.failed, e.pending = false, true
			c.loading++
			c.touchLocked(k)
			c.mu.Unlock()
			go c.fetch(rawURL, width)
			return nil
		}
		c.touchLocked(k)
		b := e.bitmap
		c.mu.Unlock()
		return b
	}
	e = &entry{pending: true}
	c.items[k] = e
	c.order = append(c.order, k)
	c.loading++
	c.mu.Unlock()

	go c.fetch(rawURL, width)
	return nil
}

// Pending 返回还在抓取的张数，供界面显示占位状态。
func (c *Cache) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loading
}

// Len 返回缓存里已就绪的位图数（测试用来验证有界与离屏释放）。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, e := range c.items {
		if e.bitmap != nil {
			n++
		}
	}
	return n
}

// Limit 返回位图数上限。
func (c *Cache) Limit() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.limit
}

// Bytes 估算缓存里解码位图占用的字节数（位图 + 64px 氛围光源图），
// 供测试量「滚动越久内存会不会无限涨」。
func (c *Cache) Bytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	var n int64
	for _, e := range c.items {
		if e.bitmap != nil {
			w, h := e.bitmap.Size()
			n += int64(w) * int64(h) * 4
		}
		if e.small != nil {
			b := e.small.Bounds()
			n += int64(b.Dx()) * int64(b.Dy()) * 4
		}
	}
	return n
}

// Get 只读缓存，不发起抓取：已加载的返回位图，其余（含正在抓 / 失败）
// 返回 nil。视图用它画已就绪的封面（未就绪由调用方先跑一趟预取），
// 这样屏外、未预取的图绝不会被 decode。
func (c *Cache) Get(rawURL string, width int) *ui.Bitmap {
	if rawURL == "" {
		return nil
	}
	k := key{rawURL, width}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.items[k]
	if e == nil {
		return nil
	}
	c.touchLocked(k)
	return e.bitmap
}

func (c *Cache) fetch(rawURL string, width int) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()
	src := ThumbURL(rawURL, width)
	req, err := http.NewRequest("GET", src, nil)
	if err != nil {
		c.finish(rawURL, width, nil, nil, true)
		return
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		c.finish(rawURL, width, nil, nil, true)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.finish(rawURL, width, nil, nil, true)
		return
	}
	// 封面是缩略图，限制读取量，避免异常响应把内存吃满。
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		c.finish(rawURL, width, nil, nil, true)
		return
	}
	// 只解码一次：位图和氛围光的小图都从同一张 image 派生。旧版先
	// ui.DecodeBitmap 再 image.Decode，同一份数据解两遍（CPU 翻倍）；
	// 这里用 image.Decode + ui.NewBitmap。代价是不再走 DecodeBitmap 的
	// JPEG EXIF 旋转 —— B 站封面/头像都是服务端生成的无 EXIF 图，不受影响。
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		c.finish(rawURL, width, nil, nil, true)
		return
	}
	c.finish(rawURL, width, ui.NewBitmap(img), downscale(img, ambientSource), false)
}

func (c *Cache) finish(rawURL string, width int, bmp *ui.Bitmap, small *image.RGBA, failed bool) {
	k := key{rawURL, width}
	c.mu.Lock()
	if e := c.items[k]; e != nil {
		if e.pending {
			c.loading--
		}
		e.bitmap, e.small, e.failed, e.pending = bmp, small, failed, false
		if failed {
			e.failedAt = time.Now()
		}
	}
	c.evictLocked()
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// touchLocked 把 key 移到最近使用的一端。就地搬移，不分配。
func (c *Cache) touchLocked(k key) {
	for i, u := range c.order {
		if u == k {
			copy(c.order[i:], c.order[i+1:])
			c.order[len(c.order)-1] = k
			return
		}
	}
}

// evictLocked 淘汰最久未用的条目，只丢已经加载完的（正在抓的不动）。
//
// 从队首往后找第一个非 pending 的淘汰；跳过 pending 的（它们就绪后会
// 变回可淘汰）。这样即使一次涌入大量抓取请求，缓存也不会无界增长。
func (c *Cache) evictLocked() {
	for len(c.order) > c.limit {
		evicted := false
		for i := 0; i < len(c.order) && len(c.order) > c.limit; i++ {
			u := c.order[i]
			if e := c.items[u]; e != nil && e.pending {
				continue // 抓取中的先留着，就绪后下一轮淘汰
			}
			c.order = append(c.order[:i], c.order[i+1:]...)
			delete(c.items, u)
			evicted = true
			i--
		}
		if !evicted {
			return // 全是抓取中的，等 finish 再淘汰
		}
	}
}

// ThumbURL 把 B 站图片地址规范化，并加上服务端下采样后缀 @<width>w.webp。
// 非 hdslb 图床的地址原样返回。宽度由调用点按展示尺寸传（见 Width* 常量）。
func ThumbURL(raw string, width int) string {
	if raw == "" {
		return ""
	}
	src := raw
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}
	u, err := url.Parse(src)
	if err != nil {
		return src
	}
	// 只有 B 站图床才做规范化：先前这里把所有 http:// 都改成 https://，
	// 非 B 站图床（本地代理 / 测试服务器）被改坏、图抓不到。
	if strings.HasSuffix(u.Hostname(), "hdslb.com") {
		// B 站的动态接口会返回 http 的图床地址，统一走 https。
		if u.Scheme == "http" {
			u.Scheme = "https"
		}
		if strings.Contains(u.Path, "/bfs/") && !strings.Contains(u.Path, "@") {
			if width <= 0 {
				width = WidthCard
			}
			u.Path += "@" + itoa(width) + "w.webp"
		}
	}
	return u.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
