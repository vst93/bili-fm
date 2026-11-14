package video

// 视频窗「置顶」键在 internal/video 侧的接线测试：外壳（view.VideoChrome）的
// 状态与 nativeWindow 的 pinned 字段必须同源，点击要一路回到 Manager 的
// OnPinToggle 回调（主窗据此落盘 videoPinned）。
//
// 无头测试不碰系统媒体框架，也不建真窗口（n.win 为 nil）：置顶下发那一步在
// 没有窗口时是空操作，正好顺带钉住「窗口不存在也不 panic」。

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestNativeViewPinWiring 渲染原生窗视图，钉住置顶键的完整回路：
// 键的按压态来自 n.pinned、点击翻转并调用 Manager 的 OnPinToggle 回调，
// 新状态在下一帧仍然一致（n.pinned 与 chrome.Pinned 同源）。
func TestNativeViewPinWiring(t *testing.T) {
	pl := &stubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second, rate: 1, vol: 1}
	m := New()
	var got []bool
	m.OnPinToggle(func(on bool) { got = append(got, on) })

	// tryNative 的接线就是这一行（m.onPinToggle → n.onPinToggle）。
	n := &nativeWindow{pl: pl, title: "x", pinned: true}
	n.onPinToggle = m.onPinToggle

	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()
	if n.chrome == nil {
		t.Fatal("视图没有建出外壳")
	}
	if !n.chrome.Pinned {
		t.Error("开窗时的置顶状态没有传给外壳")
	}
	if _, ok := tt.Find("取消置顶"); !ok {
		t.Fatalf("置顶态应显示「取消置顶」：%v", tt.Texts())
	}

	// 点一下：关置顶，回调收到 false。
	if err := tt.Click("取消置顶"); err != nil {
		t.Fatalf("点不到置顶键：%v", err)
	}
	tt.Frame()
	if n.pinned || n.chrome.Pinned {
		t.Error("点了置顶键后 nativeWindow / 外壳仍是置顶")
	}
	if len(got) != 1 || got[0] {
		t.Fatalf("OnPinToggle 应收到一次 false，实际 %v", got)
	}
	// 再点一下：开置顶，回调收到 true。
	if err := tt.Click("置顶"); err != nil {
		t.Fatalf("点不到置顶键（第二下）：%v", err)
	}
	tt.Frame()
	if !n.pinned || !n.chrome.Pinned {
		t.Error("再点一次后应重新置顶")
	}
	if len(got) != 2 || !got[1] {
		t.Fatalf("OnPinToggle 应收到 [false true]，实际 %v", got)
	}
}

// TestVideoPinnedSourceWiring 钉住「主窗落盘值 → 开窗状态」与
// 「Manager 回调 → nativeWindow」这两根线。它们各是一行赋值，没有纯逻辑可
// 单测（newNativeWindow 要建真窗口），所以按本仓库既有做法逐段核源码。
func TestVideoPinnedSourceWiring(t *testing.T) {
	flat := func(path string) string {
		t.Helper()
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", path, err)
		}
		return strings.Join(strings.Fields(string(src)), "")
	}

	want := []struct{ file, snippet string }{
		// 开窗时的初始置顶（Options.VideoPinned 来自主窗的 videoPinned）。
		{"native.go", "pinned:o.VideoPinned,"},
		// Manager 把回调交给原生窗（与弹幕那条同一个位置/写法）。
		{"video.go", "n.onPinToggle=m.onPinToggle"},
		// 点键 → 外壳回调 → 窗口字段 + 主窗回调。
		{"native.go", "OnPinToggle:func(onbool){n.pinned=on"},
		// 每帧把窗口状态同步进外壳，再把外壳的点击结果收回来。
		{"native.go", "ch.Pinned=n.pinned"},
		{"native.go", "n.pinned=ch.Pinned"},
	}
	for _, w := range want {
		if !strings.Contains(flat(w.file), w.snippet) {
			t.Errorf("%s 缺少接线：%q", w.file, w.snippet)
		}
	}
}
