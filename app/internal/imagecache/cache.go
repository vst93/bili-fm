// Package imagecache 按 URL 抓取、解码并缓存封面图，供原生界面直接使用。
//
// 与旧版的区别：旧版走本地 HTTP 代理（127.0.0.1:4654/image-proxy），因为
// webview 里的 <img> 没法带 Referer。原生界面在 Go 里抓，可以直接带头，
// 所以不需要代理；那个代理留给视频弹窗（webview）用。
package imagecache

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// 与旧版 src/utils/string.tsx 的 graftingImage 一致：B 站图床支持在 URL
// 后面加 @240w.webp 让服务端下采样，单张从几百 KB 降到几 KB。
const (
	thumbWidth = 240
	ua         = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	referer    = "https://www.bilibili.com/"
)

// Cache 是一组封面图，按 URL 索引，带容量上限。
type Cache struct {
	client *http.Client

	// limit 是缓存位图数上限，超出后按最近最少使用淘汰。
	limit int

	mu      sync.Mutex
	items   map[string]*entry
	order   []string // 最近使用的在后
	notify  func()   // 有新图时请求重绘
	loading int
}

type entry struct {
	bitmap  *ui.Bitmap
	failed  bool
	pending bool
}

// New 建一个缓存。notify 会在某张图就绪时被调用，用来触发重绘；
// 它可能在任意 goroutine 上被调用。
func New(notify func()) *Cache {
	return &Cache{
		client: &http.Client{Timeout: 15 * time.Second},
		limit:  300,
		items:  map[string]*entry{},
		notify: notify,
	}
}

// Bitmap 立即返回缓存的位图；未命中时返回 nil，并在后台抓取，就绪后调用
// notify。视图每帧都会调用它，所以这里绝不能阻塞。
func (c *Cache) Bitmap(rawURL string) *ui.Bitmap {
	if rawURL == "" {
		return nil
	}
	c.mu.Lock()
	e := c.items[rawURL]
	if e != nil {
		c.touchLocked(rawURL)
		b := e.bitmap
		c.mu.Unlock()
		return b
	}
	e = &entry{pending: true}
	c.items[rawURL] = e
	c.order = append(c.order, rawURL)
	c.mu.Unlock()

	go c.fetch(rawURL)
	return nil
}

// Pending 返回还在抓取的张数，供界面显示占位状态。
func (c *Cache) Pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loading
}

func (c *Cache) fetch(rawURL string) {
	src := ThumbURL(rawURL)
	req, err := http.NewRequest("GET", src, nil)
	if err != nil {
		c.finish(rawURL, nil, true)
		return
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

	resp, err := c.client.Do(req)
	if err != nil {
		c.finish(rawURL, nil, true)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.finish(rawURL, nil, true)
		return
	}
	// 封面是缩略图，限制读取量，避免异常响应把内存吃满。
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		c.finish(rawURL, nil, true)
		return
	}
	bmp, err := ui.DecodeBitmap(data)
	c.finish(rawURL, bmp, err != nil)
}

func (c *Cache) finish(rawURL string, bmp *ui.Bitmap, failed bool) {
	c.mu.Lock()
	if e := c.items[rawURL]; e != nil {
		e.bitmap, e.failed, e.pending = bmp, failed, false
	}
	c.evictLocked()
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// touchLocked 把 URL 移到最近使用的一端。
func (c *Cache) touchLocked(rawURL string) {
	for i, u := range c.order {
		if u == rawURL {
			c.order = append(append(c.order[:i:i], c.order[i+1:]...), rawURL)
			return
		}
	}
}

// evictLocked 淘汰最久未用的条目，只丢已经加载完的（正在抓的不动）。
func (c *Cache) evictLocked() {
	for len(c.order) > c.limit {
		u := c.order[0]
		if e := c.items[u]; e != nil && e.pending {
			return // 队首还在抓，等它完成
		}
		c.order = c.order[1:]
		delete(c.items, u)
	}
}

// ThumbURL 把 B 站图片地址规范化，并加上服务端下采样后缀。
// 非 hdslb 图床的地址原样返回。
func ThumbURL(raw string) string {
	if raw == "" {
		return ""
	}
	src := raw
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	} else if strings.HasPrefix(src, "http://") {
		// B 站的动态接口会返回 http 的图床地址，统一走 https。
		src = "https://" + strings.TrimPrefix(src, "http://")
	}
	u, err := url.Parse(src)
	if err != nil {
		return src
	}
	if strings.HasSuffix(u.Hostname(), "hdslb.com") &&
		strings.Contains(u.Path, "/bfs/") &&
		!strings.Contains(u.Path, "@") {
		u.Path += "@" + itoa(thumbWidth) + "w.webp"
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
