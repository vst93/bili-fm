package video

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

// ---------------------------------------------------------------- 纯逻辑

// TestNativeEligible 钉住「什么地址才试原生」。这条判据决定了会不会出现
// 「开个黑窗再回退」——空地址、自定义协议一律直接走 webview。
func TestNativeEligible(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"http://127.0.0.1:4654/audio-proxy?url=x", true},
		{"https://upos-sz.bilivideo.com/x.mp4", true},
		{"file:///tmp/a.mp4", true},
		{"", false},
		{"/video.html?src=x", false},
		{"mygo-video://data/1.mp4", false},
	}
	for _, c := range cases {
		if got := nativeEligible(Options{Src: c.src}); got != c.want {
			t.Errorf("nativeEligible(%q)=%v，想要 %v", c.src, got, c.want)
		}
	}
}

// TestSeekTarget 钉住进度条取值夹取：负值归 0、超出时长夹到时长、时长未知
// 时原样（不能把「还没读到 duration」当成 0 秒把人拽回开头）。
func TestSeekTarget(t *testing.T) {
	cases := []struct{ v, dur, want float64 }{
		{-3, 100, 0},
		{42, 100, 42},
		{120, 100, 100},
		{42, 0, 42},
		{0, 0, 0},
	}
	for _, c := range cases {
		if got := seekTarget(c.v, c.dur); got != c.want {
			t.Errorf("seekTarget(%v,%v)=%v，想要 %v", c.v, c.dur, got, c.want)
		}
	}
}

// TestNativeFailText 钉住两种失败的用户文案：缺媒体框架要明确指出
// GStreamer（Linux 用户唯一能自己动手修的一条），其它错误要带上原因。
func TestNativeFailText(t *testing.T) {
	unsup := nativeFailText(video.ErrUnsupported)
	if !contains(unsup, "GStreamer") || !contains(unsup, "已改用内置播放器") {
		t.Errorf("ErrUnsupported 的文案不对：%q", unsup)
	}
	// 包装过的 ErrUnsupported（ui/video 会 fmt.Errorf("%w: …", ErrUnsupported)）
	wrapped := nativeFailText(errWrap(video.ErrUnsupported))
	if !contains(wrapped, "GStreamer") {
		t.Errorf("包装后的 ErrUnsupported 没被识别：%q", wrapped)
	}
	other := nativeFailText(errors.New("decoder missing"))
	if !contains(other, "decoder missing") || !contains(other, "已改用内置播放器") {
		t.Errorf("其它失败的文案不对：%q", other)
	}
	if nativeFailText(nil) != "" {
		t.Error("nil 错误不该有文案")
	}
	if !contains(actionFallbackText(errors.New("boom")), "boom") {
		t.Error("webview 也失败时要把原因带上")
	}
}

func errWrap(err error) error { return &wrapErr{err} }

type wrapErr struct{ e error }

func (w *wrapErr) Error() string { return "video: " + w.e.Error() }
func (w *wrapErr) Unwrap() error { return w.e }

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestManagerFallbackLatch 钉住回退闩锁：一旦判定原生不可用，后续 Open 不再
// 反复尝试（否则每次开视频都先黑一下再回退）。
func TestManagerFallbackLatch(t *testing.T) {
	m := New()
	m.nativeOK, m.nativeChecked = false, true
	if reason, ok := m.tryNative(Options{Src: "http://x/y.mp4"}); ok || reason != "" {
		t.Fatalf("闩锁后仍尝试原生：ok=%v reason=%q", ok, reason)
	}
}

// TestManagerFallbackNotifies 钉住「回退要让用户可感知」：OnFallback 注册后
// reportFallback 必须真的回调。
func TestManagerFallbackNotifies(t *testing.T) {
	m := New()
	var got string
	m.OnFallback(func(r string) { got = r })
	m.reportFallback("缺少 GStreamer")
	if got != "缺少 GStreamer" {
		t.Fatalf("OnFallback 未收到原因：%q", got)
	}
}

// TestManagerNativeActive 钉住状态查询：没有原生窗时 NativeActive 为假。
func TestManagerNativeActive(t *testing.T) {
	m := New()
	if m.NativeActive() {
		t.Error("刚建的管理器不该有原生窗")
	}
	m.native = &nativeWindow{}
	if !m.NativeActive() {
		t.Error("设了原生窗后 NativeActive 应为真")
	}
	if !m.IsOpen() {
		t.Error("有原生窗时 IsOpen 应为真")
	}
}

// ---------------------------------------------------------------- 真实播放

// TestNativePlaysRGB 用 ui/video 真的播一段 H.264+AAC（mygo 的 rgb.mp4），
// 验证这条链路在本机可用：起播、画面非黑、seek、倍速、静音。
//
// 没有媒体框架的机器（Linux 缺 GStreamer）会 skip —— 那正是回退路径存在的
// 理由，不是测试失败。
func TestNativePlaysRGB(t *testing.T) {
	p, err := video.Open("testdata/rgb.mp4")
	if errors.Is(err, video.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("打开失败：%v", err)
	}
	defer p.Close()
	p.SetMuted(true)
	tt := ui.NewTester(func(c *ui.Context) { ui.Video(c, p).Size(64, 48) }, 64, 48)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if err := p.Err(); err != nil {
			t.Fatalf("播放出错：%v", err)
		}
		if p.Ready() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !p.Ready() {
		t.Fatalf("一直没就绪（err=%v）", p.Err())
	}
	if w, h := p.Size(); w != 64 || h != 48 {
		t.Errorf("画面尺寸 %dx%d，想要 64x48", w, h)
	}
	// 第一段是红色（0.4s 红 → 绿 → 蓝），像素抽样确认真的解出了画面。
	tt.Frame()
	r, g, b, _ := tt.Image().RGBAAt(32, 24).RGBA()
	if r>>8 < 150 || g>>8 > 90 || b>>8 > 90 {
		t.Errorf("首帧像素 rgb=(%d,%d,%d)，应为红色", r>>8, g>>8, b>>8)
	}
	// seek 到 0.9s（蓝段）并等它落地。
	p.SetRate(1)
	p.Seek(900 * time.Millisecond)
	landed := false
	for i := 0; i < 200; i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
		if pos := p.Position(); pos >= 850*time.Millisecond && pos <= 1150*time.Millisecond {
			landed = true
			break
		}
	}
	if !landed {
		t.Errorf("seek 到 0.9s 没落地，位置 %v", p.Position())
	}
	// 倍速与音量/静音接口在原生引擎上必须是可用的（不是 no-op 空实现）。
	p.SetRate(2)
	if p.Rate() != 2 {
		t.Errorf("SetRate(2) 后 Rate=%v", p.Rate())
	}
	p.SetVolume(0.4)
	if p.Volume() != 0.4 {
		t.Errorf("SetVolume(0.4) 后 Volume=%v", p.Volume())
	}
	p.SetMuted(false)
	if p.Muted() {
		t.Error("SetMuted(false) 后仍是静音")
	}
	if p.Duration() <= 0 {
		t.Error("原生引擎应报出时长")
	}
}

// TestNativePlaysRealBili 用真实的 B 站渐进式 MP4 验证「线上那条源确实能被
// ui/video 播」。默认跳过：它要联网、要一条会过期的播放地址。
//
//	BILI_FM_TEST_VIDEO_URL=$(curl ...) go test -run TestNativePlaysRealBili ./internal/video/
//
// 本轮实施时的实测结论（记录在 PLAN.md）：起播 368ms、seek 到 60s 落地、
// 2 倍速生效、640×360 非黑像素 225998/230400。
func TestNativePlaysRealBili(t *testing.T) {
	src := os.Getenv("BILI_FM_TEST_VIDEO_URL")
	if src == "" {
		t.Skip("未设置 BILI_FM_TEST_VIDEO_URL")
	}
	p, err := video.OpenURL(src)
	if errors.Is(err, video.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("打开失败：%v", err)
	}
	defer p.Close()
	p.SetMuted(true)
	tt := ui.NewTester(func(c *ui.Context) { ui.Video(c, p).Fill().Fit(ui.Contain) }, 640, 360)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if err := p.Err(); err != nil {
			t.Fatalf("播放出错：%v", err)
		}
		if p.Ready() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !p.Ready() {
		t.Fatalf("一直没就绪（err=%v）", p.Err())
	}
	if w, _ := p.Size(); w != 640 {
		t.Errorf("画面宽 %d，想要 640（360P 合流）", w)
	}
	if p.Duration() < 10*time.Second {
		t.Errorf("时长 %v 不像一条正常稿件", p.Duration())
	}
	p.Seek(60 * time.Second)
	for i := 0; i < 300; i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
		if pos := p.Position(); pos > 59*time.Second && pos < 63*time.Second {
			return
		}
	}
	t.Errorf("seek 到 60s 没落地，位置 %v", p.Position())
}

// ---------------------------------------------------------------- 视图

// stubPlayer 是 playerAPI 的纯逻辑桩：无头测试不碰系统媒体框架，也能验证
// 视图把「播放/暂停/倍速/静音/seek」都真的下发到了播放器。
type stubPlayer struct {
	playing   bool
	ended     bool
	ready     bool
	buffering bool
	dur       time.Duration
	pos       time.Duration
	rate      float64
	vol       float64
	muted     bool
	seeks     []time.Duration
	closed    bool
}

func (s *stubPlayer) Play()                   { s.playing, s.ended = true, false }
func (s *stubPlayer) Pause()                  { s.playing = false }
func (s *stubPlayer) Playing() bool           { return s.playing }
func (s *stubPlayer) Ended() bool             { return s.ended }
func (s *stubPlayer) Ready() bool             { return s.ready }
func (s *stubPlayer) Buffering() bool         { return s.buffering }
func (s *stubPlayer) Err() error              { return nil }
func (s *stubPlayer) Duration() time.Duration { return s.dur }
func (s *stubPlayer) Position() time.Duration { return s.pos }
func (s *stubPlayer) Seek(d time.Duration)    { s.seeks = append(s.seeks, d) }
func (s *stubPlayer) SetRate(r float64)       { s.rate = r }
func (s *stubPlayer) SetVolume(v float64)     { s.vol = v }
func (s *stubPlayer) SetMuted(m bool)         { s.muted = m }
func (s *stubPlayer) Close()                  { s.closed = true }

// TestNativeViewControls 渲染原生窗的视图，钉住三件事：
//  1. 顶栏三颗键（弹幕 / 最小化 / 退出视频播放）与控件条的播放/倍速/音量/全屏
//     都在（原生窗没有浏览器原生控件兜底）；控件条**不再**有关闭键；
//  2. 点「暂停」真的把 Pause 下发到播放器；
//  3. 点倍速键按档位循环（与主窗一致）。
func TestNativeViewControls(t *testing.T) {
	pl := &stubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second, rate: 1, vol: 1}
	n := &nativeWindow{pl: pl, title: "测试稿件", speed: 1, volume: 1}
	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()

	for _, want := range []string{"暂停", "静音", "退出视频播放", "最小化", "1x", "弹幕", "全屏", "音量"} {
		if !tt.HasText(want) {
			t.Errorf("控件条缺少 %q（现有：%v）", want, tt.Texts())
		}
	}
	// 关闭键只此一颗（顶栏），控件条不再重复；旧文案「关闭视频」也不许回来。
	if got := strings.Count(strings.Join(tt.Texts(), "\x00"), "退出视频播放"); got != 1 {
		t.Errorf("「退出视频播放」出现 %d 次，想要 1（上下各一颗就是用户报的重复）", got)
	}
	if tt.HasText("关闭视频") {
		t.Errorf("旧文案「关闭视频」还在：%v", tt.Texts())
	}
	if !tt.HasText("测试稿件") {
		t.Error("标题栏没有显示标题")
	}
	if !tt.HasText("0:30") || !tt.HasText("1:40") {
		t.Errorf("时间文案不对：%v", tt.Texts())
	}

	if err := tt.Click("暂停"); err != nil {
		t.Fatalf("点暂停失败：%v", err)
	}
	tt.Frame()
	if pl.playing {
		t.Error("点了暂停，播放器仍在播放")
	}

	// 倍速键的文案就是档位本身（1x），点它循环到下一档。
	if err := tt.Click("1x"); err != nil {
		t.Fatalf("点倍速失败：%v", err)
	}
	tt.Frame()
	if pl.rate != 1.25 {
		t.Errorf("倍速从 1x 点一下应为 1.25x，得到 %v", pl.rate)
	}

	if err := tt.Click("静音"); err != nil {
		t.Fatalf("点静音失败：%v", err)
	}
	tt.Frame()
	if !pl.muted {
		t.Error("点了静音，播放器没静音")
	}
}

// TestNativeViewSeek 钉住进度条真的 seek 到播放器（主窗底部进度条与弹窗内
// 进度条是同一语义：改的是视频位置，不是音频引擎）。
func TestNativeViewSeek(t *testing.T) {
	pl := &stubPlayer{playing: true, ready: true, dur: 200 * time.Second, pos: 10 * time.Second, rate: 1, vol: 1}
	n := &nativeWindow{pl: pl, title: "x", speed: 1, volume: 1}
	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()
	// 在进度条上按下（中点略偏右）再松手：SliderBase 的拖拽输入会改外壳的
	// seek 值，Changed 时 seek 一次。
	rect, ok := tt.Find("播放进度")
	if !ok {
		t.Fatalf("找不到进度条（现有：%v）", tt.Texts())
	}
	x := rect.X + rect.W*0.6
	tt.Press(x, rect.Y+rect.H/2)
	tt.Frame()
	tt.Release(x, rect.Y+rect.H/2)
	tt.Frame()
	if len(pl.seeks) == 0 {
		t.Fatal("改进度条没有 seek 播放器")
	}
	got := pl.seeks[len(pl.seeks)-1].Seconds()
	if got < 100 || got > 140 {
		t.Errorf("seek 到 %.1fs，期望落在 100~140s（点的是 60%% 位置）", got)
	}
}

// TestNativeViewEndedReplaysFromZero 钉住「播完再点播放从 0 开始」：直接
// Play() 对已结束的引擎是空操作（AVFoundation 停在末尾），必须先 Seek(0)。
func TestNativeViewEndedReplaysFromZero(t *testing.T) {
	pl := &stubPlayer{playing: false, ended: true, ready: true, dur: 60 * time.Second, pos: 60 * time.Second, rate: 1, vol: 1}
	n := &nativeWindow{pl: pl, title: "x", speed: 1, volume: 1}
	// 已经报过播完：重播必须把这个闩复位，否则重播到结尾不会再报一次。
	n.endedFired = true
	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()
	if !tt.HasText("播放") {
		t.Fatalf("结束时按钮文案应为「播放」，现有 %v", tt.Texts())
	}
	if err := tt.Click("播放"); err != nil {
		t.Fatalf("点播放失败：%v", err)
	}
	tt.Frame()
	if len(pl.seeks) == 0 || pl.seeks[len(pl.seeks)-1] != 0 {
		t.Errorf("播完再点播放应先 Seek(0)，实际 seeks=%v", pl.seeks)
	}
	if !pl.playing {
		t.Error("播完再点播放应重新开始播放")
	}
	if n.endedFired {
		t.Error("重播后 endedFired 没有复位，重播到结尾不会再报播完")
	}
}
