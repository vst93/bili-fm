package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/video"
	"github.com/vst93/bili-fm/app/internal/view"
)

// 这一组钉住「原生播放不可用时询问用户」在主窗侧的契约（产品定调：WebView
// 播放与开关都保留，用户的自主选择权不能被剥夺）：
//
//   - 不可用 → 弹应用内对话框（为什么 + 怎么修 + 两个按钮），**不**自动切；
//   - 选「切换到 WebView 播放」→ 开关落盘 webview，并用 webview 重开这一集；
//   - 选「取消」→ 开关仍为原生、不建窗，下次起播仍会再问。

func newVideoFallbackController(t *testing.T) (*controller, *store.KV, *stubAudio) {
	t.Helper()
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := view.NewApp(func() {})
	a.NativeVideo = true
	a.Queue = []view.Track{{Aid: 1, Bvid: "BV1test", Cid: 2, Title: "测试稿件"}}
	a.Index = 0
	a.Dur = 300
	stub := &stubAudio{playing: true}
	c := &controller{app: a, kv: kv, vid: video.New(), audioStub: stub}
	c.vid.SetNativeEnabled(true)
	return c, kv, stub
}

// TestVideoFallbackPromptShowsDialogWithoutSwitching 钉住第一条：原生不可用时
// 弹的是应用内对话框（现有 Modal 组件），文案含「为什么」和「怎么修」，
// 两个按钮就是「切换到 WebView 播放」/「取消」；**没有**自动切到 webview。
func TestVideoFallbackPromptShowsDialogWithoutSwitching(t *testing.T) {
	c, kv, _ := newVideoFallbackController(t)
	c.showVideoFallbackPrompt(&video.FallbackPrompt{
		Reason: "当前系统无法使用原生视频播放：缺少 GStreamer 1.x 的播放插件",
		Hint:   "修复（复制到终端执行，装完重启应用）：\n  sudo apt install gstreamer1.0-libav",
	})

	m := c.app.Modal
	if m == nil {
		t.Fatal("原生不可用却没有弹对话框")
	}
	if m.Kind != "message" || m.Type != "warning" {
		t.Errorf("对话框类型不对：Kind=%q Type=%q", m.Kind, m.Type)
	}
	if m.Title != videoFallbackTitle {
		t.Errorf("标题 = %q，想要 %q", m.Title, videoFallbackTitle)
	}
	for _, want := range []string{
		"当前系统无法使用原生视频播放",
		"GStreamer",
		"sudo apt install",
	} {
		if !strings.Contains(m.Message, want) {
			t.Errorf("对话框正文缺少 %q：%q", want, m.Message)
		}
	}
	if len(m.Buttons) != 2 {
		t.Fatalf("按钮 %d 个，想要 2 个：%+v", len(m.Buttons), m.Buttons)
	}
	if m.Buttons[0].Label != "切换到 WebView 播放" || !m.Buttons[0].Primary {
		t.Errorf("主按钮不对：%+v", m.Buttons[0])
	}
	if m.Buttons[1].Label != "取消" || m.Buttons[1].Primary {
		t.Errorf("次按钮不对：%+v", m.Buttons[1])
	}
	for _, b := range m.Buttons {
		if strings.Contains(b.Label, "不再询问") || strings.Contains(b.Label, "总是") {
			t.Errorf("出现了会悄悄吃掉用户选择的选项：%q", b.Label)
		}
	}

	// 没有自动回退：开关还是原生、KV 没被写、Manager 的开关也没变。
	if !c.app.NativeVideo {
		t.Error("弹询问的同时把开关改成了 webview")
	}
	if got := kv.String(prefNativeVideo); got != "" {
		t.Errorf("还没选就写了 %s=%q", prefNativeVideo, got)
	}
	if !c.vid.NativeEnabled() {
		t.Error("Manager 的开关被悄悄改成了 webview")
	}
}

// TestVideoFallbackSwitchPersistsWebView 钉住「切换」：开关落盘 webview、立即
// 生效、对话框关闭、音频回到主窗（用 webview 重开这一集的动作在 internal/video
// 的 TestSwitchToWebViewReopensSameEpisode 里用桩断言）。
func TestVideoFallbackSwitchPersistsWebView(t *testing.T) {
	c, kv, stub := newVideoFallbackController(t)
	c.showVideoFallbackPrompt(&video.FallbackPrompt{Reason: "当前系统无法使用原生视频播放", Hint: "修复…"})
	if c.app.Modal == nil || c.app.Modal.Buttons[0].OnClick == nil {
		t.Fatal("对话框/主按钮没接上")
	}

	c.app.Modal.Buttons[0].OnClick()

	if c.app.NativeVideo {
		t.Error("选了「切换到 WebView 播放」，开关仍是原生")
	}
	if got := kv.String(prefNativeVideo); got != "false" {
		t.Errorf("开关没落盘成 webview：%s=%q", prefNativeVideo, got)
	}
	if c.vid.NativeEnabled() {
		t.Error("Manager 的开关没立即生效（下一集还会先走一遍原生）")
	}
	if c.app.Modal != nil {
		t.Error("选完之后对话框还开着")
	}
	if stub.pauses == 0 {
		t.Error("切换前没有停音频引擎（会双声）")
	}
}

// TestVideoFallbackCancelKeepsNative 钉住「取消」：不播（回主窗），开关保持
// 用户原来的选择，下次起播仍会再问（Manager 那边不落闩锁）。
func TestVideoFallbackCancelKeepsNative(t *testing.T) {
	c, kv, _ := newVideoFallbackController(t)
	c.showVideoFallbackPrompt(&video.FallbackPrompt{Reason: "当前系统无法使用原生视频播放", Hint: "修复…"})
	if c.app.Modal == nil || c.app.Modal.Buttons[1].OnClick == nil {
		t.Fatal("对话框/次按钮没接上")
	}

	c.app.Modal.Buttons[1].OnClick()

	if !c.app.NativeVideo {
		t.Error("选了取消，开关却被改成 webview")
	}
	if got := kv.String(prefNativeVideo); got != "" {
		t.Errorf("选了取消却写了 %s=%q", prefNativeVideo, got)
	}
	if !c.vid.NativeEnabled() {
		t.Error("Manager 的开关被改掉了（下次起播就不会再问）")
	}
	if c.app.Modal != nil {
		t.Error("选了取消，对话框没关")
	}
	if c.app.VideoOpen {
		t.Error("选了取消却还标着「视频窗开着」")
	}
}

// TestVideoFallbackPromptNotStacked 钉住「只提示一次不循环弹」：同一条询问
// 已经挂着时，再来一次（例如起播失败的回调与 Open 的返回撞在一起）不叠层。
func TestVideoFallbackPromptNotStacked(t *testing.T) {
	c, _, _ := newVideoFallbackController(t)
	p := &video.FallbackPrompt{Reason: "当前系统无法使用原生视频播放", Hint: "修复…"}
	c.showVideoFallbackPrompt(p)
	first := c.app.Modal
	c.showVideoFallbackPrompt(p)
	if c.app.Modal != first {
		t.Error("同一份询问被重复弹了一层")
	}
}

// TestVideoOpenErrorPromptMapping 钉住 Open 的两种错误分流：FallbackPrompt
// 要问用户，其它错误还是错误 toast（不能把真失败也说成「去切换 webview」）。
func TestVideoOpenErrorPromptMapping(t *testing.T) {
	p := &video.FallbackPrompt{Reason: "当前系统无法使用原生视频播放"}
	if got := videoOpenErrorPrompt(p); got != p {
		t.Errorf("FallbackPrompt 没被识别：%+v", got)
	}
	if got := videoOpenErrorPrompt(errors.New("建窗失败")); got != nil {
		t.Errorf("普通错误不该被当成询问：%+v", got)
	}
	if got := videoOpenErrorPrompt(nil); got != nil {
		t.Errorf("nil 不该被当成询问：%+v", got)
	}
}
