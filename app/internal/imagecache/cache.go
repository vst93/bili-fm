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
// （原版各调用点各自走 graftingImage，URL 后缀不同即不同请求）。
type key struct {
	url   string
	width int
}

// Cache 是一组封面图，按（URL, 宽度）索引，带容量上限。
type Cache struct {
	// Client 抓图用的 HTTP 客户端。为 nil 时用带 15s 超时的默认客户端；
	// 测试可替换它指向本地服务器（封面地址可以是任意图床）。
	Client *http.Client

	// transcodeURL 把原图地址换算成服务端转码地址（动图 webp 兑底用）。
	// 默认 ThumbTranscoded（只处理 hdslb 图床）；测试的本地服务器不是
	// hdslb，注入一个适用于自己地址的实现。
	transcodeURL func(raw string, width int) string

	// limit 是缓存位图数上限，超出后按最近最少使用淘汰。默认 cacheLimit：
	// 原版按「滚出 2 屏就丢位图」主动卸载，这里用 LRU 等效兜底，避免长列表
	// 越滚位图越积（长列表 300 张 240w 卡片全存下约 57MB）。
	limit int

	mu      sync.Mutex
	items   map[key]*entry
	order   []key  // 最近使用的在后
	notify  func() // 有新图时请求重绘
	loading int    // 正在抓取的张数（Pending 用）
	stats   Stats  // 抓取计数器（见 Stats），受 mu 保护

	// 抓取并发门（严格优先级信号量）：slots 是在途名额数（≤ maxConcurrent），
	// waitHi / waitLo 是两级等待队列的长度。抓取 goroutine 在 fetch 开头等
	// 名额：优先队列有人等就不放普通队列 —— 新滚到的可见图永远排在先到的
	// 屏外预取前面（原版浏览器 loading=lazy 的可见优先语义）。等待中的普通
	// 请求被 Touch（变为可见）会当场升级进优先队，被 Cancel（滚出窗口）会
	// 退队回滚。解码后才能 notify 重绘，而 goroutine 在这里等，所以渲染帧
	// 零等待。maxConcurrentFetches 是 var（perfmeasure 档位对比要调它），
	// 0 表示不设闸（上界 = 全部在途请求）。
	slots          int
	waitHi, waitLo int
	slotCond       *sync.Cond

	// priority 记录 Bitmap 时被视为「可见」的（URL, 宽度）：预取方每帧把
	// 可见窗口先交给 Touch 再 Bitmap，fetch 启动时查它决定排优先队还是
	// 普通队（原版靠浏览器 loading=lazy 天然可见优先，这里显式给出）。
	priority map[key]struct{}
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
	// tries 是这条 URL 连续失败的次数：冷却时长按它指数退避。
	tries int

	// 抓取并发门的排队状态（mu 保护）：high 是优先级（Touch 可升级）；
	// waiting 是在 fetch 的等名额循环里；cancelled 是被 Cancel 退队。
	// 只约束「还在排队」的请求：已拿到名额在下载的不打断（见 Cancel）。
	high, waiting, cancelled bool
}

// State 是一张图在缓存里的状态，供测试与诊断区分「没请求 / 在途 / 已就绪 /
// 失败」四种情况——封面空白时必须能说清是哪一种。
type State int

const (
	// StateAbsent 表示这张图从没被请求过（没进缓存）。
	StateAbsent State = iota
	// StatePending 表示已经发起抓取，还没回来。
	StatePending
	// StateReady 表示位图已就绪。
	StateReady
	// StateFailed 表示抓取失败，正在冷却（retryFailedAfter）里。
	StateFailed
)

// Stats 是抓取的计数器。之前所有失败路径都是静默的，没法判断空白到底是
// 「没请求」还是「请求失败了」，也没法看失败原因分布；这里把每条路径都记上。
type Stats struct {
	// Fetches 是发起的 HTTP 请求数（进入 fetch 即计）。
	Fetches int64
	// Ready 是成功解码入库的图数；Failed 是失败总数，等于下面四项之和。
	Ready, Failed      int64
	ReqErr, StatusErr  int64 // 请求构造/发送失败、非 200
	ReadErr, DecodeErr int64 // 读响应体失败、解码失败
	Evicted            int64 // 被 LRU 淘汰的条目数
	Retried            int64 // 失败冷却到期后重新发起的次数
	// LastError 是最近一次失败的原因描述（含状态码 / 错误文本）。
	LastError string
}

// State 返回一张图当前的状态（不触发抓取）。
func (c *Cache) State(rawURL string, width int) State {
	if rawURL == "" {
		return StateAbsent
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.items[key{rawURL, width}]
	switch {
	case e == nil:
		return StateAbsent
	case e.pending:
		return StatePending
	case e.failed:
		return StateFailed
	case e.bitmap != nil:
		return StateReady
	}
	return StateAbsent
}

// Stats 返回抓取计数器的快照。
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// NewTransport 返回抓图用的 HTTP 传输：连接池上限跟着抓取并发走，暖连接
// 能复用。net/http 默认 Transport 的 MaxIdleConnsPerHost=2，并发 8 抓一屏
// 封面时会反复把暖连接挤出空闲池，下一张图又要重做 TCP（生产是 HTTPS，还
// 要重做 TLS 握手）——每张图多花一个 RTT。这里把空闲/最大连接数都抬到并发
// 上限，让一屏的请求复用同一批连接；MaxConnsPerHost 同时兜住「并发开到 50
// 会开 50 条 TCP 把图床打限流」的风险。maxConns ≤ 0 时取当前并发量。
func NewTransport(maxConns int) *http.Transport {
	if maxConns <= 0 {
		maxConns = maxConcurrentFetches
	}
	if maxConns < 1 {
		maxConns = 1
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          2 * maxConns,
		MaxIdleConnsPerHost:   maxConns,
		MaxConnsPerHost:       maxConns,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ForceAttemptHTTP2:     true,
	}
}

// New 建一个缓存。notify 会在某张图就绪时被调用，用来触发重绘；
// 它可能在任意 goroutine 上被调用。
func New(notify func()) *Cache {
	c := &Cache{
		Client:       &http.Client{Timeout: 15 * time.Second, Transport: NewTransport(0)},
		limit:        cacheLimit,
		items:        map[key]*entry{},
		notify:       notify,
		priority:     map[key]struct{}{},
		transcodeURL: ThumbTranscoded,
	}
	c.slotCond = sync.NewCond(&c.mu)
	return c
}

// acquire 拿一个抓取名额，返回释放函数。严格优先级：优先队列（本轮可见
// 的图）有人等时，释放的名额先给优先队列 —— 新滚到的可见图不排在旧的
// 屏外预取后面。调用方（fetch goroutine）在这里阻塞，渲染帧零等待。
// maxConcurrentFetches ≤ 0 时不设闸（perfmeasure 的「无上限」档）。等待
// 期间被 Cancel 退队时返回 nil，调用方直接放弃这次抓取。
func (c *Cache) acquire(e *entry, high bool) func() {
	c.mu.Lock()
	e.high, e.waiting = high, true
	if high {
		c.waitHi++
	} else {
		c.waitLo++
	}
	for !e.cancelled && (maxConcurrentFetches > 0 && c.slots >= maxConcurrentFetches || c.waitHi > 0 && !e.high) {
		c.slotCond.Wait()
	}
	e.waiting = false
	if e.cancelled {
		// Cancel 已把名额账目结清（退队、广播）；若曾升过优先级，优先队列
		// 长度已减、普通等待者可以前进，补一次广播。
		high := e.high
		e.high = false
		c.mu.Unlock()
		if high {
			c.mu.Lock()
			c.slotCond.Broadcast()
			c.mu.Unlock()
		}
		return nil
	}
	if e.high {
		c.waitHi--
	} else {
		c.waitLo--
	}
	e.high = false
	c.slots++
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		c.slots--
		// Broadcast 而不是 Signal：等待队列是两级优先的，Signal 可能唤醒一个
		// 普通等待者（它看到优先队列有人又睡回去），而真正该醒的优先等待者
		// 就再也没有人叫了 —— 表现为「后面的图永远不请求」（实测 flake）。
		c.slotCond.Broadcast()
		c.mu.Unlock()
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
//
// 原版（Tauri）封面走 <img loading="lazy">，应用层不设闸、由浏览器连接池
// 调度（HTTP/1.1 每主机 6 并发、HTTP/2 多路复用更多），一屏十几张基本同时
// 在途；显式的 PRELOAD_CONCURRENCY=2 只管首屏预热。
//
// 取值 16 来自 TestConcurrencyLadder 的 8/16/32/50 四档实测（一屏 15 张 +
// 滚动追加 15 张，图床 RTT 40ms）：
//
//	档位  不限流首屏  限流首屏  限流滚动  峰值在途  新建连接  被限流
//	 8     85ms      85ms      85ms       8        8        0
//	16     45ms     405ms     404ms      15       15       14
//	32     45ms     405ms     403ms      15       15       14
//	50     45ms     406ms     402ms      15       15       14
//
// 关键是**峰值在途数**：一屏可见 + overscan 的封面约 15 张，超过 15 的档位
// （32/50）在途数完全一样、首屏耗时逐毫秒相同——用户提议的 50 没有任何额外
// 收益，只多开 TCP。8→16 是唯一有意义的一跳：一屏 15 张从 2 批压到 1 批，
// 首屏 85→45ms（约 2 倍）。16 是实测的拐点，也是「一屏一批」的最小值。
//
// 关于「图床 >8 软限流」：那是上一轮为了保守估的模型（不是真机 CDN 实测）。
// 本轮把传输换成 NewTransport（连接池上限 = 并发量、暖连接复用）：默认
// Transport 的 MaxIdleConnsPerHost=2 会在并发 8 时把暖连接挤掉、两批 16 个
// 请求要新建 14 条连接（TestConnectionReuse），换成 NewTransport 后只建 8 条。
// 连接复用的收益是实打实的（生产 HTTPS 每条新连接多一次 TLS 握手），而并发
// 开到 16 只多 8 条连接——比旧实现「并发 8 + 连接反复重开」对图床更友好。
//
// 0 = 不设闸（仅供水位对比）；运行时恒 ≥1。
var maxConcurrentFetches = 16

// httpClient 返回抓图用的客户端，未显式设置时给出默认的。
func (c *Cache) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

// 失败重试的冷却时间：太短会在每帧重发请求，太长则封面一直空着。
// 重试用指数退避（retryFailedAfter << 重试次数，封顶 retryFailedAfter），
// 第一次失败 2.5s 后就重试——网络抖动不至于把封面锁死 15s。
const (
	retryFailedAfter     = 15 * time.Second
	retryFailedFirstWait = 2500 * time.Millisecond
)

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
		// 之前失败过，过了冷却期就重新抓（原版 RetryImg 的 onError 重试）。
		// 冷却按失败次数指数退避：第一次 2.5s，之后逐次翻倍、封顶 15s。
		if e.failed && !e.pending && time.Since(e.failedAt) > c.retryWait(e.tries) {
			e.failed, e.pending = false, true
			c.loading++
			c.stats.Retried++
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

// Touch 把（URL, 宽度）标记为「本轮可见」：预取方每帧先对可见窗口调
// Touch 再 Bitmap，fetch 启动时查这个集合决定排优先队还是普通队 ——
// 新滚到的可见图永远排在先到的屏外预取前面（原版靠浏览器 loading=lazy
// 天然可见优先，这里由预取顺序显式给出）。普通优先图抢占队列，因为
// priority 集合在 fetch 启动时才消费，先到的屏外预取已经在队列里等。
func (c *Cache) Touch(rawURL string, width int) {
	if rawURL == "" {
		return
	}
	c.mu.Lock()
	k := key{rawURL, width}
	if c.priority == nil {
		c.priority = map[key]struct{}{}
	}
	c.priority[k] = struct{}{}
	// 新滚到的可见图如果在普通队里排队，当场升级：否则它要等所有先到的
	// 屏外预取拿完名额（原版浏览器对滚到的图立即加载，不等前面的）。
	if e := c.items[k]; e != nil && e.waiting && !e.high {
		e.high = true
		c.waitLo--
		c.waitHi++
		// 名额可能空着（waitHi==0 时普通等待者抢走）：唤醒队列重新分配。
		c.slotCond.Broadcast()
	}
	c.mu.Unlock()
}

// Cancel 把一张图的抓取请求退出队列（原版浏览器对滚出视口的图取消加载）：
// 滚动后不再可见的屏外预取不应再占用并发名额。只退**还在排队**的：已拿到
// 名额在途的下载照常完成（结果入库，将来滚回来直接命中；名额在完成时释放）。
// 失败冷却不受影响：cancelled 不是失败，不记 tries / failedAt。
func (c *Cache) Cancel(rawURL string, width int) {
	if rawURL == "" {
		return
	}
	k := key{rawURL, width}
	c.mu.Lock()
	delete(c.priority, k)
	e := c.items[k]
	if e == nil || !e.waiting || e.cancelled {
		// 还没进队列（fetch 未启动）或已拿到名额 / 已退过：不做。前者由
		// fetch 启动时再查一次 priority（见 fetch），滚出后不会被拉优先级。
		c.mu.Unlock()
		return
	}
	e.cancelled = true
	if e.high {
		e.high = false
		c.waitHi--
	} else {
		c.waitLo--
	}
	c.slotCond.Broadcast()
	c.mu.Unlock()
}

func (c *Cache) fetch(rawURL string, width int) {
	k := key{rawURL, width}
	// 可见优先：Bitmap 发起时预取方已用 Touch 把可见窗口登记在 priority 里；
	// 在这里查它决定排优先队（本轮可见）还是普通队（先到的屏外预取）。
	c.mu.Lock()
	e := c.items[k]
	_, high := c.priority[k]
	if high {
		delete(c.priority, k)
	}
	c.mu.Unlock()
	release := c.acquire(e, high)
	if release == nil {
		// 排队期间被 Cancel 退队（滚出窗口）：请求作废。条目还没产出位图、
		// 也没进失败冷却，直接从缓存删掉 —— 等效于从未请求过，滚回来时
		// Bitmap 会重新发起。不清的话它永远停在 pending：不可淘汰、不被
		// 重试、Pending() 永不清零（实测挂死 TestCancelQueuedFetch 的根因）。
		// 重新 Bitmap 不会产生新条目（pending 时代它是空操作），所以这里
		// 删除不会孤儿化任何新请求。
		c.mu.Lock()
		if cur := c.items[k]; cur == e {
			delete(c.items, k)
			for i, u := range c.order {
				if u == k {
					c.order = append(c.order[:i], c.order[i+1:]...)
					break
				}
			}
			if e.pending {
				e.pending = false
				c.loading--
			}
		}
		c.mu.Unlock()
		return
	}
	defer release()
	bmp, small, err := c.download(ThumbURL(rawURL, width))
	if err != nil && isAnimatedWebPError(err) {
		// 动图 webp 解不出来（x/image/webp 不支持 ANIM/ANMF，见 decodeWebP）。
		// 换 @<w>w.jpg 让服务端转码成 JPEG 再抓一次 —— B 站图床对任意后缀
		// 都会重新编码。只重试一次；头像这类动图从此就绪，不再反复失败。
		c.statsFail("动图转码")
		bmp, small, err = c.download(c.transcodeURL(rawURL, width))
	}
	if err != nil {
		c.fail(rawURL, width, err.Error(), errKind(err))
		return
	}
	c.mu.Lock()
	c.stats.Fetches++
	c.mu.Unlock()
	c.finish(rawURL, width, bmp, small, false)
}

// download 抓一个图片地址并解码成位图 + 氛围光小图。errKind(err) 给出失败
// 分类（与 Stats 的字段一一对应）。
func (c *Cache) download(src string) (*ui.Bitmap, *image.RGBA, error) {
	req, err := http.NewRequest("GET", src, nil)
	if err != nil {
		return nil, nil, kindErr(errReason("请求构造", err), statReqErr)
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, nil, kindErr(errReason("请求发送", err), statReqErr)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, kindErr("状态码 "+itoa(resp.StatusCode), statStatusErr)
	}
	// 封面是缩略图，限制读取量，避免异常响应把内存吃满。
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, nil, kindErr(errReason("读取响应", err), statReadErr)
	}
	img, err := decodeWebP(data)
	if err != nil {
		return nil, nil, err
	}
	// 只解码一次：位图和氛围光的小图都从同一张 image 派生。旧版先
	// ui.DecodeBitmap 再 image.Decode，同一份数据解两遍（CPU 翻倍）。
	// 代价是不再走 DecodeBitmap 的 JPEG EXIF 旋转 —— B 站封面/头像都是
	// 服务端生成的无 EXIF 图，不受影响。
	return ui.NewBitmap(img), downscale(img, ambientSource), nil
}

// 失败原因分类，与 Stats 的字段一一对应。
const (
	statReqErr = iota
	statStatusErr
	statReadErr
	statDecodeErr
)

// kindErr 把失败原因和分类打进一个 error，download 往上返、fetch 用
// errKind / errReason 拆开。
type kindError struct {
	msg  string
	kind int
}

func (e *kindError) Error() string { return e.msg }

func kindErr(msg string, kind int) error { return &kindError{msg, kind} }

// errKind 取出失败分类；不是 kindError 就当请求错误。
func errKind(err error) int {
	if ke, ok := err.(*kindError); ok {
		return ke.kind
	}
	return statReqErr
}

// errReason 把失败包装成可读的一句话（原因分类 + 底层错误）。
func errReason(what string, err error) string { return what + "失败: " + err.Error() }

// statsFail 记一次中间失败（动图转码重试）：转码也是一次真实 HTTP 请求，
// 计入 Fetches；它成功时 Ready 在 finish 里另行计数，所以 Failed 会大于
// 最终失败的条目数 —— 这正是中间失败该有的口径（失败过程可观测）。
func (c *Cache) statsFail(reason string) {
	c.mu.Lock()
	c.stats.Fetches++
	c.stats.Failed++
	c.stats.DecodeErr++
	c.stats.LastError = reason
	c.mu.Unlock()
}

// decodeWebP 解码图片数据。B 站头像有一部分是**动图 webp**（RIFF 头里带
// ANIM/ANMF 块），x/image/webp 的解码器不认识这两个块、整张报
// 「webp: invalid format」—— 而这些 URL 结尾就是 .webp，走 @<w>w.webp
// 后缀拿回来的还是动图。这里检测出动图后报 animWebPError，让 fetch 换
// @<w>w.jpg 让服务端转码成 JPEG 再抓一次。
func decodeWebP(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		if isAnimatedWebPData(data) {
			return nil, animWebPError{}
		}
		return nil, kindErr(errReason("解码", err), statDecodeErr)
	}
	return img, nil
}

// animWebPError 标记「这是动图 webp，需要服务端转码」这类失败。
type animWebPError struct{}

func (animWebPError) Error() string { return "动图 webp（服务端转码重试）" }

// isAnimatedWebPError 报告 err 是否是动图 webp。
func isAnimatedWebPError(err error) bool {
	_, ok := err.(animWebPError)
	return ok
}

// isAnimatedWebPData 按 WebP 规范（RFC 9649 §Animated）看 RIFF 头里的块：
// 动图 = VP8X（带 animation 位）+ ANIM + ANMF。只需要前几十字节就能判断。
func isAnimatedWebPData(data []byte) bool {
	if len(data) < 16 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return false
	}
	off := 12
	for off+8 <= len(data) {
		fourcc := string(data[off : off+4])
		size := int(uint32(data[off+4]) | uint32(data[off+5])<<8 | uint32(data[off+6])<<16 | uint32(data[off+7])<<24)
		if fourcc == "ANMF" || fourcc == "ANIM" {
			return true
		}
		if fourcc == "VP8X" && size >= 1 && off+8 < len(data) {
			// 动图必然有 ANIM/ANMF；这里继续扫下一个块。
		}
		off += 8 + size + size&1
	}
	return false
}

// fail 记下失败原因（分类计数 + 最近一条）再走 finish。
func (c *Cache) fail(rawURL string, width int, reason string, kind int) {
	c.mu.Lock()
	c.stats.Fetches++
	c.stats.Failed++
	switch kind {
	case statReqErr:
		c.stats.ReqErr++
	case statStatusErr:
		c.stats.StatusErr++
	case statReadErr:
		c.stats.ReadErr++
	case statDecodeErr:
		c.stats.DecodeErr++
	}
	c.stats.LastError = reason
	c.mu.Unlock()
	c.finish(rawURL, width, nil, nil, true)
}

func (c *Cache) finish(rawURL string, width int, bmp *ui.Bitmap, small *image.RGBA, failed bool) {
	k := key{rawURL, width}
	c.mu.Lock()
	if !failed {
		c.stats.Ready++
	}
	if e := c.items[k]; e != nil {
		if e.pending {
			c.loading--
		}
		e.bitmap, e.small, e.failed, e.pending = bmp, small, failed, false
		if failed {
			e.failedAt = time.Now()
			e.tries++
		} else {
			e.tries = 0
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
			c.stats.Evicted++
			evicted = true
			i--
		}
		if !evicted {
			return // 全是抓取中的，等 finish 再淘汰
		}
	}
}

// retryWait 返回第 tries 次连续失败后的重试冷却：第一次 retryFailedFirstWait
// ，之后逐次翻倍、封顶 retryFailedAfter。原版 RetryImg 的 onError 立即换占位图
// （下一次渲染就重试）；这里等价于「短冷却 + 每帧探查」，网络抖动不至于把
// 封面锁死半分钟。
func (c *Cache) retryWait(tries int) time.Duration {
	if tries < 1 {
		tries = 1
	}
	wait := retryFailedFirstWait
	for i := 1; i < tries; i++ {
		wait *= 2
		if wait >= retryFailedAfter {
			return retryFailedAfter
		}
	}
	return wait
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

// ThumbTranscoded 返回让 B 站图床**转码成 JPEG** 的地址：原地址可能是动图
// webp（头像），x/image/webp 解不了；对任意后缀重新编码是图床本来就有的能力
// （实测 .webp 源加 @96w.jpg 返回 image/jpeg）。非 hdslb 图床原样返回 ——
// 测试服务器不支持转码，重试一次原图即可。
func ThumbTranscoded(raw string, width int) string {
	src := ThumbURL(raw, width)
	u, err := url.Parse(src)
	if err != nil || !strings.HasSuffix(u.Hostname(), "hdslb.com") ||
		!strings.Contains(u.Path, "/bfs/") || strings.HasSuffix(u.Path, ".jpg") {
		return src
	}
	// 已有的下采样后缀换成 .jpg：@240w.webp → @240w.jpg；没有后缀的（非 /bfs/
	// 或已带 @ 的原样地址）直接追加。
	if at := strings.LastIndex(u.Path, "@"); at >= 0 {
		u.Path = u.Path[:at] + "@" + itoa(width) + "w.jpg"
	} else {
		if width <= 0 {
			width = WidthCard
		}
		u.Path += "@" + itoa(width) + "w.jpg"
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
