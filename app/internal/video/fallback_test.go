package video

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/video"

	"github.com/vst93/bili-fm/app/internal/danmaku"
	"github.com/vst93/bili-fm/app/internal/view"
)

// 这一组钉住本轮的契约：原生播放不可用时**不再自动回退**，而是问用户
// 「要不要切换到 WebView 播放」。开关（nativeVideoPlayer）是用户的，不能替
// 他改；选「切换」才落盘 webview 并重开这一集，选「取消」什么都不改。

// unavailableNative 是「本机没有媒体框架」的原生起播桩（对应 Linux 缺
// GStreamer 时 ui/video 返回的 ErrUnsupported）。
func unavailableNative() func(string) (playerAPI, error) {
	return func(string) (playerAPI, error) {
		return nil, fmt.Errorf("video: %w (playbin/appsink 缺失)", video.ErrUnsupported)
	}
}

// stubNativeWindow 给 tryNative 一个没有真实窗口的原生窗（无头测试不碰
// 系统窗口与媒体框架）。
func stubNativeWindow(o Options, pl playerAPI) *nativeWindow {
	return &nativeWindow{
		pl:       pl,
		title:    o.Title,
		dm:       &danmaku.Overlay{},
		settings: &view.PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1},
	}
}

// TestNativeUnavailableAsksInsteadOfFallingBack 钉住：原生不可用 + 开关=原生
// → Open 返回一条询问，**不**开 webview、**不**改开关。
func TestNativeUnavailableAsksInsteadOfFallingBack(t *testing.T) {
	m := New()
	webCalls := 0
	m.openWebViewFn = func(Options) error { webCalls++; return nil }
	m.openNative = unavailableNative()

	err := m.Open(Options{Src: "http://127.0.0.1:4654/audio-proxy?url=x", Title: "第一集", StartAt: 12, Speed: 1})
	var prompt *FallbackPrompt
	if !errors.As(err, &prompt) {
		t.Fatalf("Open 没有返回「询问用户」的提示：%v", err)
	}
	if webCalls != 0 {
		t.Errorf("还没问用户就自动开了 webview（%d 次）", webCalls)
	}
	if !m.NativeEnabled() {
		t.Error("开关被悄悄从原生改成了 webview")
	}
	if !m.PendingFallback() {
		t.Error("失败的那一集没有挂起，用户点「切换」时就没得重开")
	}
	if !contains(prompt.Reason, "当前系统无法使用原生视频播放") || !contains(prompt.Reason, "GStreamer") {
		t.Errorf("询问的「为什么」不对：%q", prompt.Reason)
	}
	if prompt.Hint == "" {
		t.Error("询问缺少「怎么修」")
	}
}

// TestSwitchToWebViewReopensSameEpisode 钉住「切换」这一支：开关落盘为
// webview（由主窗调 SetNativeEnabled）、当前这一集用 webview 重开、进度尽量
// 接上、之后起播不再问。
func TestSwitchToWebViewReopensSameEpisode(t *testing.T) {
	m := New()
	var opened []Options
	m.openWebViewFn = func(o Options) error {
		opened = append(opened, o)
		return nil
	}
	m.openNative = unavailableNative()

	first := Options{Src: "http://127.0.0.1:4654/audio-proxy?url=a", Title: "第一集", Poster: "p.png", StartAt: 12, Speed: 1}
	var prompt *FallbackPrompt
	if err := m.Open(first); !errors.As(err, &prompt) {
		t.Fatalf("第一集应先问用户：%v", err)
	}

	// 原生窗其实播过一小段（起播成功后才发现失败的情形也走同一套挂起请求）。
	m.mu.Lock()
	m.st = State{Time: 47.5, Duration: 300}
	m.mu.Unlock()

	// 主窗的「切换」：先落盘（SetNativeEnabled），再重开。
	m.SetNativeEnabled(false)
	if err := m.SwitchToWebView(); err != nil {
		t.Fatalf("切换到 WebView 失败：%v", err)
	}
	if len(opened) != 1 {
		t.Fatalf("webview 重开了 %d 次，想要 1", len(opened))
	}
	got := opened[0]
	if got.Src != first.Src || got.Title != first.Title || got.Poster != first.Poster {
		t.Errorf("重开的不是同一集：%+v", got)
	}
	if got.StartAt != 47.5 {
		t.Errorf("进度没接上：StartAt=%v，想要 47.5", got.StartAt)
	}
	if m.PendingFallback() {
		t.Error("切换后仍挂着旧请求（再点一次会重开一遍）")
	}

	// 开关已是 webview：下一集直接走 webview，不再问。
	if err := m.Open(Options{Src: first.Src, Title: "第二集", Speed: 1}); err != nil {
		t.Fatalf("切换后起播不该再询问：%v", err)
	}
	if len(opened) != 2 {
		t.Fatalf("第二集没走 webview（webview 调用 %d 次）", len(opened))
	}
}

// TestCancelFallbackKeepsNativeChoice 钉住「取消」这一支：开关仍是原生、
// 不建任何窗口、下次起播仍会再问。
func TestCancelFallbackKeepsNativeChoice(t *testing.T) {
	m := New()
	webCalls := 0
	m.openWebViewFn = func(Options) error { webCalls++; return nil }
	m.openNative = unavailableNative()

	o := Options{Src: "http://127.0.0.1:4654/audio-proxy?url=a", Title: "第一集", Speed: 1}
	var prompt *FallbackPrompt
	if err := m.Open(o); !errors.As(err, &prompt) {
		t.Fatalf("第一次起播应先问用户：%v", err)
	}
	m.CancelFallback()
	if m.NativeEnabled() != true {
		t.Error("选了取消，开关却变成 webview")
	}
	if webCalls != 0 {
		t.Errorf("选了取消却建了 %d 个 webview 窗", webCalls)
	}
	if m.PendingFallback() {
		t.Error("选了取消，挂起请求没清掉")
	}
	// 下次起播：还是原生 → 仍然会问（用户的自主选择权保留）。
	if err := m.Open(o); !errors.As(err, &prompt) {
		t.Fatalf("取消后再次起播应当重新询问：%v", err)
	}
	if webCalls != 0 {
		t.Errorf("再次起播仍然不许自动开 webview（%d 次）", webCalls)
	}
}

// TestNativeAvailableOpensNativeWithoutAsking 钉住正常路径：原生可用 + 开关
// 原生 → 走原生、不弹询问、不碰 webview。
func TestNativeAvailableOpensNativeWithoutAsking(t *testing.T) {
	m := New()
	pl := &stubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 5 * time.Second}
	m.openNative = func(string) (playerAPI, error) { return pl, nil }
	m.newNativeWin = stubNativeWindow
	m.openWebViewFn = func(Options) error {
		t.Error("原生可用却走了 webview")
		return nil
	}
	prompts := 0
	m.OnFallback(func(*FallbackPrompt) { prompts++ })

	if err := m.Open(Options{Src: "http://127.0.0.1:4654/a.mp4", Title: "x", Speed: 1}); err != nil {
		t.Fatalf("原生可用时不该失败：%v", err)
	}
	if !m.NativeActive() {
		t.Error("原生可用时没有走原生窗")
	}
	if prompts != 0 || m.PendingFallback() {
		t.Errorf("原生可用时不该询问用户（prompts=%d pending=%v）", prompts, m.PendingFallback())
	}
}

// TestNativeStartFailureAsksUser 钉住「原生窗起来了但起播失败」那一支：
// 关窗 + 询问用户（同一个 FallbackPrompt），不自动切 webview、不改开关。
func TestNativeStartFailureAsksUser(t *testing.T) {
	m := New()
	webCalls := 0
	m.openWebViewFn = func(Options) error { webCalls++; return nil }
	bad := &stubPlayer{err: errors.New("decoder missing")}
	m.openNative = func(string) (playerAPI, error) { return bad, nil }
	m.newNativeWin = stubNativeWindow

	promptCh := make(chan *FallbackPrompt, 1)
	m.OnFallback(func(p *FallbackPrompt) { promptCh <- p })

	if err := m.Open(Options{Src: "http://127.0.0.1:4654/a.mp4", Title: "x", Speed: 1}); err != nil {
		t.Fatalf("原生窗应当先开出来（失败要等起播后才判定）：%v", err)
	}
	n := m.native
	if n == nil {
		t.Fatal("原生窗没记到管理器上")
	}
	// 视图构建里那一帧：report 发现播放器报错 → 询问。
	n.report()

	select {
	case p := <-promptCh:
		if !contains(p.Reason, "decoder missing") {
			t.Errorf("起播失败的原因没带上：%q", p.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("起播失败后没有询问用户")
	}
	if webCalls != 0 {
		t.Errorf("起播失败后自动切到了 webview（%d 次）", webCalls)
	}
	if !m.NativeEnabled() {
		t.Error("起播失败后悄悄把开关改成了 webview")
	}
	if !m.PendingFallback() {
		t.Error("失败的那一集没挂起")
	}
}

// TestNativeReportWaitsForFailure 钉住防抖：还在加载 / 缓冲时不许判定失败
// （判据是播放器真的报错，不是「还没就绪」）。
func TestNativeReportWaitsForFailure(t *testing.T) {
	n := &nativeWindow{pl: &stubPlayer{ready: false, buffering: true, dur: 60 * time.Second}}
	failed := 0
	n.onFailed = func(error) { failed++ }
	for i := 0; i < 3; i++ {
		n.report() // 模拟三帧还在起播
	}
	if failed != 0 {
		t.Errorf("还在加载就弹了「原生不可用」（%d 次）", failed)
	}
}

// TestNativeFailureIgnoringFirstFrameTiming 钉住「首帧快慢不影响起播失败判定」。
//
// mygo v0.9.1 的 macOS 两条（#1832 首帧唤醒 / #1743 不在主线程读 asset tracks）
// 会让首帧到达的时刻变快或变慢。我们的判据是**事件式**的：只有播放器真的报错
// （pl.Err() 非空）才算失败，没有任何「等了多少帧 / 多少秒还没画面」的计时器；
// 而且闩锁是一次性的（onFailed 只触发一次）。所以 240 帧「还在加载」不许弹询问，
// 真报错时正好一次。
func TestNativeFailureIgnoringFirstFrameTiming(t *testing.T) {
	pl := &stubPlayer{ready: false, buffering: true, dur: 60 * time.Second}
	n := &nativeWindow{pl: pl}
	failed := 0
	n.onFailed = func(error) { failed++ }
	// 慢首帧：240 帧都还没就绪（≈ 4 秒的画面帧），一帧都不许判失败。
	for i := 0; i < 240; i++ {
		n.report()
	}
	if failed != 0 {
		t.Fatalf("首帧还没到就判了起播失败（%d 次）", failed)
	}
	// 快首帧：就绪也不改变什么（就绪不是「已判定成功」，失败仍按 Err 走）。
	pl.ready, pl.buffering = true, false
	for i := 0; i < 10; i++ {
		n.report()
	}
	if failed != 0 {
		t.Fatalf("就绪后出现了失败判定（%d 次）", failed)
	}
	// 真的报错：一次。之后同一错误再报也不会二次回退（闩锁）。
	pl.err = errors.New("decoder missing")
	n.report()
	n.report()
	pl.err = errors.New("decoder missing again")
	n.report()
	if failed != 1 {
		t.Errorf("起播失败应只触发一次回退，实际 %d 次", failed)
	}
}

// TestSwitchToWebViewWithoutPending 钉住边界：没有挂起请求时「切换」不建窗。
func TestSwitchToWebViewWithoutPending(t *testing.T) {
	m := New()
	called := false
	m.openWebViewFn = func(Options) error { called = true; return nil }
	if err := m.SwitchToWebView(); !errors.Is(err, ErrNoPendingFallback) {
		t.Fatalf("没有挂起请求时应返回 ErrNoPendingFallback，实际 %v", err)
	}
	if called {
		t.Error("没有挂起请求却建了 webview 窗")
	}
}

// --- SrcFallback：DASH 分片起播失败时自动退回合流一次 ---

// failingPlayer 是「起播就报错」的播放器替身。
func failingPlayer() *stubPlayer { return &stubPlayer{err: errors.New("decode failed")} }

// TestSrcFallbackRetriedOnceWithoutPrompt 钉住自动降级：Src（DASH 分片）起播
// 失败时用 SrcFallback（合流 MP4）重开一次，**不**弹「改用 WebView」的询问。
func TestSrcFallbackRetriedOnceWithoutPrompt(t *testing.T) {
	m := New()
	var tried []string
	m.openNative = func(src string) (playerAPI, error) {
		tried = append(tried, src)
		if len(tried) == 1 {
			return failingPlayer(), nil // 分片地址起播失败
		}
		return &stubPlayer{playing: true, ready: true, dur: 100 * time.Second}, nil
	}
	m.newNativeWin = stubNativeWindow
	m.openWebViewFn = func(Options) error {
		t.Error("自动降级成功时不该走 webview")
		return nil
	}
	prompts := 0
	m.OnFallback(func(*FallbackPrompt) { prompts++ })

	o := Options{
		Src:         "http://127.0.0.1:1/master.m3u8",
		SrcFallback: "http://127.0.0.1:4654/audio-proxy?url=combined",
		Title:       "一集",
		Speed:       1,
	}
	if err := m.Open(o); err != nil {
		t.Fatalf("打开失败：%v", err)
	}
	// 触发一次失败上报（原生窗每帧调 report）。
	n := m.native
	if n == nil {
		t.Fatal("原生窗没建起来")
	}
	n.report()

	waitFor(t, func() bool { return len(tried) == 2 })
	if tried[0] != o.Src || tried[1] != o.SrcFallback {
		t.Fatalf("重试顺序不对：%v", tried)
	}
	if prompts != 0 {
		t.Errorf("自动降级不该问用户（问了 %d 次）", prompts)
	}
	if !m.NativeActive() {
		t.Error("自动降级后原生窗不该关掉")
	}
}

// TestSrcFallbackAsksWhenRetryAlsoFails 钉住「退完还是不行才问人」。
func TestSrcFallbackAsksWhenRetryAlsoFails(t *testing.T) {
	m := New()
	var tried []string
	m.openNative = func(src string) (playerAPI, error) {
		tried = append(tried, src)
		return failingPlayer(), nil
	}
	m.newNativeWin = stubNativeWindow
	m.openWebViewFn = func(Options) error { return nil }
	prompts := make(chan *FallbackPrompt, 4)
	m.OnFallback(func(p *FallbackPrompt) { prompts <- p })

	o := Options{Src: "http://x/master.m3u8", SrcFallback: "http://x/combined.mp4", Title: "一集", Speed: 1}
	if err := m.Open(o); err != nil {
		t.Fatalf("打开失败：%v", err)
	}
	m.native.report() // 第一次失败 → 重试
	waitFor(t, func() bool { return len(tried) == 2 })
	// 重试换上的新播放器也要报一次错才会问人（switchNative 会把 onFailed
	// 重新挂到新窗口上）。
	waitFor(t, func() bool { return m.native != nil && m.native.onFailed != nil })
	m.native.report()

	select {
	case p := <-prompts:
		if p == nil || p.Reason == "" {
			t.Error("询问里应当带原因")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("两次都失败却没问用户")
	}
}

// TestSrcFallbackNotRetriedTwice 钉住「只退一次」。
func TestSrcFallbackNotRetriedTwice(t *testing.T) {
	m := New()
	var tried []string
	m.openNative = func(src string) (playerAPI, error) {
		tried = append(tried, src)
		return failingPlayer(), nil
	}
	m.newNativeWin = stubNativeWindow
	m.OnFallback(func(*FallbackPrompt) {})

	o := Options{Src: "http://x/a.m3u8", SrcFallback: "http://x/b.mp4", Title: "一集", Speed: 1}
	if err := m.Open(o); err != nil {
		t.Fatal(err)
	}
	m.native.report()
	waitFor(t, func() bool { return len(tried) == 2 })
	waitFor(t, func() bool { return m.native != nil })
	// 第二次失败不该再退（b.mp4 就是最后一条了）。
	m.native.report()
	time.Sleep(150 * time.Millisecond)
	if len(tried) != 2 {
		t.Errorf("重试了不止一次：%v", tried)
	}
}

// TestSrcFallbackNotRetriedOnUnsupported 钉住：平台缺解码器（ErrUnsupported）
// 时**不**重试 —— 换地址救不了，仍然问用户要不要用 WebView。
func TestSrcFallbackNotRetriedOnUnsupported(t *testing.T) {
	m := New()
	var tried []string
	m.openNative = func(src string) (playerAPI, error) {
		tried = append(tried, src)
		return nil, video.ErrUnsupported
	}
	m.openWebViewFn = func(Options) error { return nil }

	o := Options{Src: "http://x/a.m3u8", SrcFallback: "http://x/b.mp4", Title: "一集", Speed: 1}
	var prompt *FallbackPrompt
	if err := m.Open(o); !errors.As(err, &prompt) {
		t.Fatalf("平台不支持时应当问用户：%v", err)
	}
	if len(tried) != 1 {
		t.Errorf("ErrUnsupported 不该用备用地址重试：%v", tried)
	}
}

// TestSrcFallbackResetsPerEpisode 钉住：换集后新一集有它自己的重试机会。
func TestSrcFallbackResetsPerEpisode(t *testing.T) {
	m := New()
	var tried []string
	m.openNative = func(src string) (playerAPI, error) {
		tried = append(tried, src)
		return failingPlayer(), nil
	}
	m.newNativeWin = stubNativeWindow
	m.OnFallback(func(*FallbackPrompt) {})

	first := Options{Src: "http://x/e1.m3u8", SrcFallback: "http://x/e1.mp4", Title: "第一集", Speed: 1}
	if err := m.Open(first); err != nil {
		t.Fatal(err)
	}
	m.native.report() // 用掉第一集的降级机会
	waitFor(t, func() bool { return len(tried) == 2 })

	second := Options{Src: "http://x/e2.m3u8", SrcFallback: "http://x/e2.mp4", Title: "第二集", Speed: 1}
	if err := m.Open(second); err != nil {
		t.Fatal(err)
	}
	// 换集复位 fallbackTried（switchNative 里做），新一集失败时应当再次降级。
	// 这里直接 report 而不是先等「fallbackTried==false」：复位发生在换集
	// 的同一段逻辑里，等那个条件会和「重试已把它置真」赛跑。
	m.native.report()
	waitFor(t, func() bool { return len(tried) >= 4 })
	if tried[len(tried)-1] != second.SrcFallback {
		t.Errorf("第二集没用自己的备用地址：%v", tried)
	}
}

// TestSrcFallbackIgnoredWhenEmpty 钉住：没有备用地址时行为不变（直接问人）。
func TestSrcFallbackIgnoredWhenEmpty(t *testing.T) {
	m := New()
	m.openNative = func(string) (playerAPI, error) { return failingPlayer(), nil }
	m.newNativeWin = stubNativeWindow
	m.openWebViewFn = func(Options) error { return nil }
	prompts := make(chan *FallbackPrompt, 1)
	m.OnFallback(func(p *FallbackPrompt) { prompts <- p })

	if err := m.Open(Options{Src: "http://x/a.mp4", Title: "一集", Speed: 1}); err != nil {
		t.Fatal(err)
	}
	m.native.report()
	select {
	case <-prompts:
	case <-time.After(2 * time.Second):
		t.Fatal("没有备用地址时应当直接问用户")
	}
}

// waitFor 等一个条件成立（最多 2 秒）。自动降级跑在 goroutine 上。
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("等待超时")
}

// TestWebViewUsesCombinedSource 钉住：webview 回退路径拿到的是合流地址，
// 不是本地 HLS 播放列表（页面里的 <video> 播不了 m3u8 + 分片）。
func TestWebViewUsesCombinedSource(t *testing.T) {
	m := New()
	var got Options
	m.openNative = func(string) (playerAPI, error) { return nil, video.ErrUnsupported }
	m.openWebViewFn = func(o Options) error { got = o; return nil }

	o := Options{
		Src:         "http://127.0.0.1:9/master.m3u8",
		SrcFallback: "http://127.0.0.1:4654/audio-proxy?url=combined",
		Title:       "一集",
		Speed:       1,
	}
	var prompt *FallbackPrompt
	if err := m.Open(o); !errors.As(err, &prompt) {
		t.Fatalf("平台不支持时应先问用户：%v", err)
	}
	if err := m.SwitchToWebView(); err != nil {
		t.Fatal(err)
	}
	if got.Src != o.SrcFallback {
		t.Errorf("webview 的源 = %q，想要合流地址 %q", got.Src, o.SrcFallback)
	}
	if got.SrcFallback != "" {
		t.Error("webview 路径不该还带着 SrcFallback")
	}
}
