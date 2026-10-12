package imagecache

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// animatedWebP 手工拼一个最小动图 webp：RIFF/WEBP + VP8X（带 animation 位）+
// ANIM + ANMF（内嵌一个 1×1 静态 VP8 帧大小的空壳）。x/image/webp 解不了它
// （decode.go 的 switch 里没有 ANIM/ANMF，循环到 EOF → errInvalidFormat），
// 但 isAnimatedWebPData 应该认出来。
func animatedWebP() []byte {
	vp8x := []byte{0x10, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x01, 0x00} // animation bit=2
	anim := []byte{0x00, 0x00, 0x00, 0x00, 0xff, 0xff}
	anmf := make([]byte, 16) // 帧参数空壳（内容无所谓，解码器根本走不到）
	anmf[15] = 0x10
	var b bytes.Buffer
	b.WriteString("RIFF")
	var body bytes.Buffer
	chunk := func(fourcc string, data []byte) {
		body.WriteString(fourcc)
		n := uint32(len(data))
		body.Write([]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)})
		body.Write(data)
		if len(data)%2 == 1 {
			body.WriteByte(0)
		}
	}
	chunk("VP8X", vp8x)
	chunk("ANIM", anim)
	chunk("ANMF", anmf)
	// RIFF 尺寸 = "WEBP" + body
	n := uint32(4 + body.Len())
	b.Write([]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)})
	b.WriteString("WEBP")
	b.Write(body.Bytes())
	return b.Bytes()
}

func TestIsAnimatedWebPData(t *testing.T) {
	if !isAnimatedWebPData(animatedWebP()) {
		t.Fatal("手工拼的动图 webp 没被识别")
	}
	// 静态 webp（viP8/VP8L 开头）与 PNG 不误报。
	if isAnimatedWebPData([]byte("RIFF\x00\x00\x00\x00WEBPVP8 \x00\x00\x00\x00")) {
		t.Fatal("静态 webp 被误报为动图")
	}
	if isAnimatedWebPData([]byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("PNG 被误报为动图")
	}
	if _, _, err := image.Decode(bytes.NewReader(animatedWebP())); err == nil {
		t.Fatal("x/image/webp 居然解开了动图（前提不成立，测试无意义）")
	}
}

// TestThumbTranscoded 钉住动图兜底地址：@240w.webp → @240w.jpg；
// 非 hdslb / 非 /bfs/ 原样返回。
func TestThumbTranscoded(t *testing.T) {
	cases := []struct{ in, width, want string }{
		{"https://i0.hdslb.com/bfs/face/a.webp", "96", "https://i0.hdslb.com/bfs/face/a.webp@96w.jpg"},
		{"https://i0.hdslb.com/bfs/face/a.webp@240w.webp", "240", "https://i0.hdslb.com/bfs/face/a.webp@240w.jpg"},
		{"http://i2.hdslb.com/bfs/archive/a.jpg", "240", "https://i2.hdslb.com/bfs/archive/a.jpg@240w.jpg"},
	}
	for _, c := range cases {
		if got := ThumbTranscoded(c.in, mustAtoi(c.width)); got != c.want {
			t.Errorf("ThumbTranscoded(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := ThumbTranscoded("https://example.com/a.png", 96); got != "https://example.com/a.png" {
		t.Errorf("非 hdslb 应原样返回，得到 %q", got)
	}
}

func mustAtoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// pngBytes 造一张 1×1 PNG。
func pngBytes() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	return b.Bytes()
}

// TestAnimatedWebPFallsBackToJPEG 钉住「动图头像也能显示」：服务器对
// @<w>w.webp 返回动图 webp、对 @<w>w.jpg 返回静态 JPEG。改前头像永远
// 空白（解码失败 → 15s 冷却 → 每帧重发失败 → 永远 nil）；改后第一次
// 失败立即换 .jpg 转码地址重试、当次就绪。
func TestAnimatedWebPFallsBackToJPEG(t *testing.T) {
	jpg := pngBytes() // 内容不重要，解码器认 png 就行
	var mu sync.Mutex
	webpHits, jpgHits := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if strings.HasSuffix(r.URL.Path, ".webp") || strings.HasSuffix(r.URL.Path, "w.webp") {
			webpHits++
			w.Header().Set("Content-Type", "image/webp")
			w.Write(animatedWebP())
			return
		}
		jpgHits++
		w.Header().Set("Content-Type", "image/png")
		w.Write(jpg)
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.Client = srv.Client()
	// 本地测试服务器不是 hdslb，注入适用于它的转码地址（.webp → .jpg）。
	c.transcodeURL = func(raw string, width int) string {
		return strings.TrimSuffix(ThumbURL(raw, width), ".webp") + ".jpg"
	}
	c.limit = 8
	url := srv.URL + "/face.webp"
	bmp := c.Bitmap(url, WidthAvatar)
	if bmp != nil {
		t.Fatal("首次请求应异步返回 nil")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && c.Pending() > 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if c.Pending() != 0 {
		t.Fatal("动图转码重试应完成")
	}
	if bmp = c.Get(url, WidthAvatar); bmp == nil {
		t.Fatalf("转码重试后应就绪: stats=%+v", c.Stats())
	}
	mu.Lock()
	defer mu.Unlock()
	if webpHits != 1 || jpgHits != 1 {
		t.Fatalf("应各请求一次：webp=%d jpg=%d", webpHits, jpgHits)
	}
	st := c.Stats()
	if st.Ready != 1 {
		t.Errorf("Ready=%d, want 1（转码重试成功）", st.Ready)
	}
	if st.LastError == "" || !strings.Contains(st.LastError, "动图") {
		t.Errorf("LastError 应记录动图转码：%q", st.LastError)
	}
}

// TestRetryBackoffQuickFirstRetry 钉住失败重试的指数退避：第一次失败
// 2.5s 后就重试（不再锁死 15s），连续失败逐次翻倍、封顶 15s。
func TestRetryBackoffQuickFirstRetry(t *testing.T) {
	cases := []struct {
		tries int
		want  time.Duration
	}{
		{1, 2500 * time.Millisecond},
		{2, 5 * time.Second},
		{3, 10 * time.Second},
		{4, 15 * time.Second},
		{9, 15 * time.Second},
	}
	for _, c := range cases {
		if got := (*Cache)(nil).retryWait(c.tries); got != c.want {
			t.Errorf("retryWait(%d) = %v, want %v", c.tries, got, c.want)
		}
	}
}

// TestFailureRetriesAfterBackoff 端到端钉住：服务器先全 429，2.5s 冷却到期
// 后自动重试并就绪（改前是 15s）。
func TestFailureRetriesAfterBackoff(t *testing.T) {
	ok := pngBytes()
	var mu sync.Mutex
	fail := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		f := fail
		mu.Unlock()
		if f {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write(ok)
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.Client = srv.Client()
	url := srv.URL + "/c0.png"
	c.Bitmap(url, WidthCard)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && c.Pending() > 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if c.Get(url, WidthCard) != nil {
		t.Fatal("服务器仍失败，不应就绪")
	}
	start := time.Now()
	mu.Lock()
	fail = false
	mu.Unlock()
	// 第一次冷却 2.5s：2.5s 后下一次 Bitmap 调用应触发重试并就绪。
	deadline = time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		c.Bitmap(url, WidthCard)
		if c.Get(url, WidthCard) != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := time.Since(start); got > 6*time.Second {
		t.Errorf("服务器恢复后 %.1fs 仍未就绪（冷却没有退避生效）", got.Seconds())
	}
	if c.Get(url, WidthCard) == nil {
		t.Fatalf("冷却到期应重试就绪: stats=%+v", c.Stats())
	}
	if st := c.Stats(); st.Retried < 1 {
		t.Errorf("Retried=%d, want ≥1", st.Retried)
	}
}

// TestVisiblePriorityOrdering 钉住「可见优先」：先把一屏屏外预取灌满队列
// （不 Touch），再 Touch 一张新可见的图 —— 它必须先于所有屏外预取就绪
// （原版浏览器 loading=lazy 的可见优先语义；改前 FIFO 先到先得）。
func TestVisiblePriorityOrdering(t *testing.T) {
	// 钉住的是排队语义，把并发压到 4（恢复原值）——12 张全在名额内、
	// 根本不排队，就测不出优先级了（maxConcurrentFetches 是 var）。
	old := maxConcurrentFetches
	maxConcurrentFetches = 4
	// 恢复必须发生在所有 fetch goroutine 退出之后（读这个变量的 race）：
	// 放进下面 gate cleanup 的 Pending 归零之后，不能用 defer（defer 先于
	// cleanup 执行，goroutine 还活着）。

	var mu sync.Mutex
	gate := make(chan struct{}, 16) // 一次只放行一个请求
	started := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		started[r.URL.Path] = true
		mu.Unlock()
		<-gate // 等测试放行
		w.Write(pngBytes())
	}))
	c := New(nil)
	c.Client = srv.Client()
	// 测试退出时 fetch goroutine 可能还在读 maxConcurrentFetches（race）：
	// cleanup LIFO —— Pending 等待注册在 gate 放行 cleanup **之后**才对，
	// 但 c 在那之后才存在；所以这里只放 gate 放行逻辑，Pending 等待挪进
	// 同一个 cleanup 的 srv.Close 之前（放行后必然可达）。
	t.Cleanup(func() {
		// 放行所有卡在 gate 上的 handler，否则 srv.Close 永远等不到。
		for i := 0; i < 32; i++ {
			select {
			case gate <- struct{}{}:
			default:
				// gate 容量 16 且 handler 逐个取，塞满即可。
				time.Sleep(2 * time.Millisecond)
			}
		}
		for c.Pending() > 0 {
			time.Sleep(2 * time.Millisecond)
		}
		maxConcurrentFetches = old
		srv.Close()
	})

	// 8 张屏外预取：4 张占满名额，4 张排队（普通队）。
	for i := 0; i < 8; i++ {
		c.Bitmap(fmt.Sprintf("%s/off%d.png", srv.URL, i), WidthCard)
	}
	waitStarted := func(n int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			got := len(started)
			mu.Unlock()
			if got >= n {
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
		t.Fatalf("只发起了 %d 个请求，want %d", func() int { mu.Lock(); defer mu.Unlock(); return len(started) }(), n)
	}
	snapshot := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, 0, len(started))
		for p := range started {
			out = append(out, p)
		}
		return out
	}
	waitStarted(4)

	// 新滚到的可见图：Touch 后 Bitmap；再来一张不 Touch 的（普通队尾）。
	c.Touch(srv.URL+"/visible.png", WidthCard)
	c.Bitmap(srv.URL+"/visible.png", WidthCard)
	c.Bitmap(srv.URL+"/last.png", WidthCard)

	// 放行 1 个在途请求，下一个发请求的应是 visible（优先队先于普通队）。
	gate <- struct{}{}
	time.Sleep(50 * time.Millisecond) // 让被放行的完成、名额释放、队列前进
	waitStarted(5)
	names := snapshot()
	has := func(suffix string) bool {
		for _, p := range names {
			if strings.HasSuffix(p, suffix) {
				return true
			}
		}
		return false
	}
	if !has("/visible.png") {
		t.Fatalf("放行 1 个后，visible 应已发起请求；实际 %v", names)
	}
	// 关键断言：last.png（比 visible 晚进队、不 Touch）不得在 visible 之前
	// 或同一轮动 —— 优先队列严格先于普通队列。
	if has("/last.png") {
		t.Fatalf("普通队尾的 last 不应先于/伴随 visible 发起：%v", names)
	}
}

// TestCancelQueuedFetchReleasesSlot 钉住「离屏取消」：并发闸满、请求在普通队
// 里排队时 Cancel 把它退队，名额立即让给后续请求（原版浏览器对滚出视口的
// loading=lazy 图取消加载）。已拿到名额在途的不打断（结果照常入库）。
func TestCancelQueuedFetchReleasesSlot(t *testing.T) {
	old := maxConcurrentFetches
	maxConcurrentFetches = 2
	// 恢复必须等 fetch goroutine 全部退出（acquire 在读这个变量）：不能
	// defer —— defer 先于 cleanup 执行，排队中的 goroutine 还活着。

	var mu sync.Mutex
	started := map[string]bool{}
	gate := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		started[r.URL.Path] = true
		mu.Unlock()
		<-gate // 等测试放行
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngBytes())
	}))
	t.Cleanup(srv.Close)
	var cc *Cache
	t.Cleanup(func() {
		// 放行所有卡在 gate 上的 handler，直到缓存排空（非阻塞 send 可能
		// 落在两个请求之间的空档，所以循环重试到 Pending 归零或超时）。
		deadline := time.Now().Add(3 * time.Second)
		for cc != nil && cc.Pending() > 0 && time.Now().Before(deadline) {
			select {
			case gate <- struct{}{}:
			default:
				time.Sleep(2 * time.Millisecond)
			}
		}
		maxConcurrentFetches = old
	})
	c := New(nil)
	cc = c
	c.Client = srv.Client()

	// 4 张抢 2 个名额：哪 2 张先拿到名额不保证（Go 的 go 不保证按创建顺序
	// 调度），所以不能写死取消 a —— 改为等队列稳定后从**实际排队**的 URL
	// 里挑一张取消。
	c.Bitmap(srv.URL+"/in0.png", WidthCard)
	c.Bitmap(srv.URL+"/in1.png", WidthCard)
	c.Bitmap(srv.URL+"/a.png", WidthCard)
	c.Bitmap(srv.URL+"/b.png", WidthCard)
	all := []string{"in0", "in1", "a", "b"}
	// 等 2 个名额被占满（2 张在途）、剩 2 张在普通队里排队。
	var queued []string
	d := time.Now().Add(3 * time.Second)
	for time.Now().Before(d) {
		queued = queued[:0]
		c.mu.Lock()
		for _, name := range all {
			if e := c.items[key{srv.URL + "/" + name + ".png", WidthCard}]; e != nil && e.waiting {
				queued = append(queued, name)
			}
		}
		c.mu.Unlock()
		if len(queued) >= 2 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if len(queued) < 2 {
		t.Fatalf("应有 2 张在普通队排队，实际 %v", queued)
	}
	victim := queued[0] // 实际排队的其中一张
	other := srv.URL + "/" + queued[1] + ".png"
	vURL := srv.URL + "/" + victim + ".png"

	// 取消排队中的 victim：名额让给还在排队的（other / 后到的 c；同为普通
	// 优先级时谁先醒不保证 —— 锁竞争顺序；钉住的是被取消的 victim 不再
	// 发请求、名额被让出）。
	c.Cancel(vURL, WidthCard)
	c.Bitmap(srv.URL+"/c.png", WidthCard)
	gate <- struct{}{} // 放行一个在途，腾出名额

	// 取消不是失败：被退队的条目等效于从未请求（回到 Absent）。
	d = time.Now().Add(3 * time.Second)
	for time.Now().Before(d) {
		if c.State(vURL, WidthCard) == StateAbsent {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if st := c.State(vURL, WidthCard); st != StateAbsent {
		t.Fatalf("被退队的条目应回到 Absent（等效从未请求），实际 %v", st)
	}
	mu.Lock()
	victimStarted := started["/"+victim+".png"]
	mu.Unlock()
	if victimStarted {
		t.Fatalf("被 Cancel 的 %s 不应再发起请求", victim)
	}
	// 名额让给了还在排队的（other 或 c）。
	d = time.Now().Add(3 * time.Second)
	for time.Now().Before(d) {
		if c.State(other, WidthCard) == StateReady || c.State(other, WidthCard) == StatePending {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	// 取消后重新 Bitmap victim：等效从未请求，应重新发起。
	c.Bitmap(vURL, WidthCard)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		s := started["/"+victim+".png"]
		mu.Unlock()
		if s {
			break
		}
		select {
		case gate <- struct{}{}:
		default:
			time.Sleep(2 * time.Millisecond)
		}
	}
	mu.Lock()
	s := started["/"+victim+".png"]
	mu.Unlock()
	if !s {
		t.Fatalf("取消后重新 Bitmap，%s 应重新发起请求", victim)
	}
	if st := c.State(vURL, WidthCard); st != StatePending {
		t.Fatalf("取消后重新请求应回到 Pending，实际 %v", st)
	}
}

// TestTouchUpgradesQueuedFetch 钉住「排队中的普通请求被 Touch 当场升级」：
// 一屏屏外预取把普通队排满，新滚到的可见图（先 Bitmap 后 Touch 也会发生，
// 同一帧内顺序不保证）不应等整队普通请求拿完名额。
func TestTouchUpgradesQueuedFetch(t *testing.T) {
	old := maxConcurrentFetches
	maxConcurrentFetches = 1
	// 同上：恢复放在 cleanup 里（Pending 归零之后）。

	var mu sync.Mutex
	started := map[string]bool{}
	order := []string{}
	gate := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		started[r.URL.Path] = true
		order = append(order, r.URL.Path)
		mu.Unlock()
		<-gate
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngBytes())
	}))
	t.Cleanup(srv.Close)
	var cc *Cache
	t.Cleanup(func() {
		// 放行所有卡在 gate 上的 handler，直到缓存排空（非阻塞 send 可能
		// 落在两个请求之间的空档，循环重试到 Pending 归零或超时）。
		deadline := time.Now().Add(3 * time.Second)
		for cc != nil && cc.Pending() > 0 && time.Now().Before(deadline) {
			select {
			case gate <- struct{}{}:
			default:
				time.Sleep(2 * time.Millisecond)
			}
		}
		maxConcurrentFetches = old
	})
	c := New(nil)
	cc = c
	c.Client = srv.Client()

	// 4 个请求抢 1 个名额：谁先抢到不定（fetch goroutine 启动顺序不定），
	// 但它一定是这 4 个之一；其余 3 个排普通队。
	c.Bitmap(srv.URL+"/occ.png", WidthCard)
	for i := 0; i < 3; i++ {
		c.Bitmap(fmt.Sprintf("%s/off%d.png", srv.URL, i), WidthCard)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := started["/occ.png"]
		mu.Unlock()
		if got {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	// 先 Bitmap（此时未 Touch → 进普通队），随后 Touch（变可见）—— 升级。
	// 等 late 真正进普通队（waiting 且非 high），保证升级发生在排队中。
	u := srv.URL + "/late.png"
	c.Bitmap(u, WidthCard)
	d := time.Now().Add(3 * time.Second)
	for time.Now().Before(d) {
		c.mu.Lock()
		e := c.items[key{u, WidthCard}]
		queued := e != nil && e.waiting && !e.high
		c.mu.Unlock()
		if queued {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	c.Touch(u, WidthCard)
	// 放行名额占用者（无论它是 occ 还是某个 off）：唯一的名额空出，late(high)
	// 必须越过全部普通等待者下一个到达。
	gate <- struct{}{}
	d = time.Now().Add(3 * time.Second)
	for time.Now().Before(d) {
		mu.Lock()
		ok := started["/late.png"]
		mu.Unlock()
		if ok {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if !started["/late.png"] {
		t.Fatalf("被升级的 late 3s 内未发起（order=%v）", order)
	}
	if len(order) < 2 || !strings.HasSuffix(order[1], "/late.png") {
		t.Fatalf("late 应是第 2 个到达的（第 1 个是先抢到名额的初始请求），实际 order=%v", order)
	}
}

// TestVisibleCoversStateBreakdown 给出「图片不显示」的**计数器证据**：一屏
// N 张可见封面里，明确区分「从未请求（Absent）/ 请求中（Pending）/ 已就绪
// （Ready）/ 请求失败（Failed）」各多少。这是排查「封面空白」的关键口径 ——
// 改前所有失败路径静默，没法判断空白到底是没请求还是请求挂了。
//
// 场景：15 张可见封面，服务器对其中 3 张返回 500。收敛后应 12 Ready、
// 3 Failed、0 Pending、0 Absent（15 张全都被请求过）。
func TestVisibleCoversStateBreakdown(t *testing.T) {
	const visible = 15
	ok := pngBytes()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// c3 / c7 / c11 返回 500，其余正常。
		if p := r.URL.Path; p == "/c3.png" || p == "/c7.png" || p == "/c11.png" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(ok)
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.Client = srv.Client()
	urls := make([]string, 0, visible)
	for i := 0; i < visible; i++ {
		u := fmt.Sprintf("%s/c%d.png", srv.URL, i)
		urls = append(urls, u)
		c.Touch(u, WidthCard) // 可见优先（预取方的真实顺序）
		c.Bitmap(u, WidthCard)
	}
	// 等抓取收敛（Pending 归零）。
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && c.Pending() > 0 {
		time.Sleep(5 * time.Millisecond)
	}

	counts := map[State]int{}
	for _, u := range urls {
		counts[c.State(u, WidthCard)]++
	}
	t.Logf("可见 %d 张：就绪=%d 请求中=%d 失败=%d 从未请求=%d（stats=%+v）",
		visible, counts[StateReady], counts[StatePending], counts[StateFailed], counts[StateAbsent], c.Stats())

	if counts[StateAbsent] != 0 {
		t.Errorf("可见封面里有 %d 张从未请求（预取没覆盖到）", counts[StateAbsent])
	}
	if counts[StatePending] != 0 {
		t.Errorf("收敛后仍有 %d 张在途", counts[StatePending])
	}
	if counts[StateReady] != visible-3 {
		t.Errorf("就绪=%d, want %d", counts[StateReady], visible-3)
	}
	if counts[StateFailed] != 3 {
		t.Errorf("失败=%d, want 3（能区分失败 vs 空白）", counts[StateFailed])
	}
	st := c.Stats()
	if st.StatusErr != 3 {
		t.Errorf("StatusErr=%d, want 3（非 200 归类到状态码错误）", st.StatusErr)
	}
	if st.LastError == "" || !strings.Contains(st.LastError, "500") {
		t.Errorf("LastError 应含状态码 500：%q", st.LastError)
	}
}
