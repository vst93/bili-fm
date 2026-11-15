package video

// 视频窗内置设置面板在 internal/video 侧的接线测试：设置面板里的
// 「原生视频播放」与弹幕基本设置必须一路回到 Manager 的回调（主窗据此落盘），
// 并且「能否用原生」这个事实由 Manager 下发给窗口（n.nativeVideo）。
//
// 无头测试不碰系统媒体框架，也不建真窗口（n.win 为 nil）：SetNativeEnabled
// 对已开着的窗口的那一步在没有窗口时是「不改」，正好顺带钉住不 panic。

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

// TestNativeViewSettingsWiring 渲染原生窗视图，钉住设置面板的两条回路：
//
//   - 面板里点「原生视频播放」→ chrome.NativeVideo 翻转 → nativeWindow 的
//     nativeVideo 同步 + Manager 的 OnNativeToggle 回调收到新值；
//   - 面板里改「不透明度」→ OnDanmakuSettings 收到新设置（主窗落盘入口）。
func TestNativeViewSettingsWiring(t *testing.T) {
	pl := &stubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second, rate: 1, vol: 1}
	m := New()
	var native []bool
	var styles []danmaku.Settings
	m.OnNativeToggle(func(on bool) { native = append(native, on) })
	m.OnDanmakuStyle(func(s danmaku.Settings) { styles = append(styles, s) })

	// tryNative 的接线就是这两行（m.onXxx → n.onXxx）。
	n := &nativeWindow{pl: pl, title: "x", nativeVideo: true,
		dmSettings: danmaku.DefaultSettings()}
	n.onNativeToggle = m.onNativeToggle
	n.onDanmakuStyle = m.onDanmakuStyle

	tt := ui.NewTester(n.view, 700, 900)
	tt.Frame()
	if n.chrome == nil {
		t.Fatal("视图没有建出外壳")
	}
	if !n.chrome.NativeVideo {
		t.Fatal("开窗时的原生状态没有传给外壳（应默认开）")
	}
	// 打开设置面板。
	if err := tt.Click("视频设置"); err != nil {
		t.Fatalf("点不到顶栏设置键：%v", err)
	}
	tt.Frame()

	// 「原生视频播放」：关掉 → 窗口字段 + 主窗回调。
	if err := tt.Click("视频设置：原生视频播放"); err != nil {
		t.Fatalf("点不到面板里的原生开关：%v", err)
	}
	tt.Frame()
	if n.nativeVideo {
		t.Error("点了开关后 nativeWindow.nativeVideo 仍为真")
	}
	if len(native) != 1 || native[0] {
		t.Fatalf("OnNativeToggle 应收到一次 false，实际 %v", native)
	}
	// 再点回来（用户的选择权：随时能改回去）。
	if err := tt.Click("视频设置：原生视频播放"); err != nil {
		t.Fatalf("第二次点不到原生开关：%v", err)
	}
	tt.Frame()
	if !n.nativeVideo || len(native) != 2 || !native[1] {
		t.Fatalf("再点一次应回到开：native=%v got=%v", n.nativeVideo, native)
	}

	// 弹幕不透明度：落窗口 + 回主窗。
	if err := tt.Click("视频设置：不透明度 70%"); err != nil {
		t.Fatalf("点不到不透明度档位：%v", err)
	}
	tt.Frame()
	if len(styles) != 1 || styles[0].Opacity != 0.7 {
		t.Fatalf("OnDanmakuStyle 应收到一次 0.7，实际 %+v", styles)
	}
	if n.dmSettings.Opacity != 0.7 {
		t.Errorf("窗口侧弹幕设置没跟上：%+v", n.dmSettings)
	}
}

// TestVideoSettingsSourceWiring 钉住 Manager → nativeWindow 的那几行：回调与
// 「原生可用」状态各是一行赋值，没有纯逻辑可单测（newNativeWindow 要建真窗口），
// 按本仓库既有做法逐段核源码。
func TestVideoSettingsSourceWiring(t *testing.T) {
	flat := func(path string) string {
		t.Helper()
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", path, err)
		}
		return strings.Join(strings.Fields(string(src)), "")
	}
	want := []struct{ file, snippet string }{
		// 开窗时的初始值（能走原生就一定是开的）。
		{"native.go", "nativeVideo:true,"},
		// Manager 把两个回调交给原生窗（与弹幕 / 置顶那条同一个位置）。
		{"video.go", "n.onNativeToggle=m.onNativeToggle"},
		{"video.go", "n.onDanmakuStyle=m.onDanmakuStyle"},
		{"video.go", "n.nativeVideo=true"},
		// 面板点击 → 窗口字段 + 主窗回调。
		{"native.go", "OnNativeToggle:func(onbool){n.nativeVideo=on"},
		{"native.go", "OnDanmakuSettings:func(sdanmaku.Settings){n.dmSettings=s"},
		// 每帧同步（主窗改了开关，面板那一行跟着变）。
		{"native.go", "ch.NativeVideo=n.nativeVideo"},
		{"native.go", "n.nativeVideo=ch.NativeVideo"},
		// 主窗改开关时，已开着的窗口也要跟上。
		{"video.go", "n.apply(func(){n.nativeVideo=on})"},
	}
	for _, w := range want {
		if !strings.Contains(flat(w.file), w.snippet) {
			t.Errorf("%s 缺少接线：%q", w.file, w.snippet)
		}
	}
}
