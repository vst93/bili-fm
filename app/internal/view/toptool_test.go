package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestTopToolStyleFor 钉住搜索药丸两类小键（.top-tool-btn / .search-submit-btn）
// 的三态：基态透明 + opacity .82 + 图标 #64748b；悬停铺白底、opacity 1、
// 图标转蓝 #0284c7；按下底换 --glass-bg-active（白 .16），图标色 / 不透明度
// 沿用悬停（原版 :active 不重置它们）。两类键只差悬停底色。
func TestTopToolStyleFor(t *testing.T) {
	// 入口键（.top-tool-btn）：悬停底白 .65。
	base := topToolStyleFor(topToolEntry, false, false)
	if base.Bg.A != 0 {
		t.Errorf("基态底色应透明，得到 %v", base.Bg)
	}
	if base.Ink != ui.Hex("#64748b") {
		t.Errorf("基态图标色 = %v，应为 #64748b", base.Ink)
	}
	if base.Opacity != 0.82 {
		t.Errorf("基态不透明度 = %v，应为 .82", base.Opacity)
	}
	hover := topToolStyleFor(topToolEntry, true, false)
	if hover.Bg != ui.Hex("#ffffff").Alpha(0.65) {
		t.Errorf("入口键悬停底 = %v，应为白 .65", hover.Bg)
	}
	if hover.Ink != ui.Hex("#0284c7") {
		t.Errorf("悬停图标色 = %v，应为 #0284c7", hover.Ink)
	}
	if hover.Opacity != 1 {
		t.Errorf("悬停不透明度 = %v，应为 1", hover.Opacity)
	}
	press := topToolStyleFor(topToolEntry, false, true)
	if press.Bg != ui.Hex("#ffffff").Alpha(0.16) {
		t.Errorf("按下底 = %v，应为白 .16（--glass-bg-active）", press.Bg)
	}
	if press.Ink != ui.Hex("#0284c7") || press.Opacity != 1 {
		t.Errorf("按下应沿用悬停的图标色/不透明度，得到 %v / %v", press.Ink, press.Opacity)
	}

	// 提交键（.search-submit-btn）：与入口键逐项相同，只差悬停底白 .50。
	sub := topToolStyleFor(topToolSubmit, true, false)
	if sub.Bg != ui.Hex("#ffffff").Alpha(0.50) {
		t.Errorf("提交键悬停底 = %v，应为白 .50", sub.Bg)
	}
	if sub.Ink != hover.Ink || sub.Opacity != hover.Opacity {
		t.Errorf("提交键悬停其余项应与入口键相同，得到 %v / %v", sub.Ink, sub.Opacity)
	}
	if topToolHoverBg(topToolEntry) == topToolHoverBg(topToolSubmit) {
		t.Error("两类键的悬停底色应不同（.65 / .50）")
	}
}

// TestSearchPillShadows 钉住搜索药丸的两态投影：各两条、且悬停那组更强
// （原版 --liquid-shadow-lifted）。mygo 的 Shadow 是累加的，两态必须二选一 ——
// 之前是「先铺常态再叠悬停」，悬停时会同时挂上四道投影。
func TestSearchPillShadows(t *testing.T) {
	rest := searchPillShadows(false)
	hover := searchPillShadows(true)
	if len(rest) != 2 || len(hover) != 2 {
		t.Fatalf("两态各应两条，得到 %d / %d", len(rest), len(hover))
	}
	// 常态：0 8px 24px rgba(15,23,42,.07) + 0 1px 4px .03。
	if rest[0] != (shadowSpec{0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.07)}) {
		t.Errorf("常态首条 = %+v", rest[0])
	}
	if rest[1] != (shadowSpec{0, 1, 4, 0, ui.Hex("#0f172a").Alpha(0.03)}) {
		t.Errorf("常态次条 = %+v", rest[1])
	}
	// 悬停：0 16px 38px .11 + 0 2px 8px .04。
	if hover[0] != (shadowSpec{0, 16, 38, 0, ui.Hex("#0f172a").Alpha(0.11)}) {
		t.Errorf("悬停首条 = %+v", hover[0])
	}
	if hover[1] != (shadowSpec{0, 2, 8, 0, ui.Hex("#0f172a").Alpha(0.04)}) {
		t.Errorf("悬停次条 = %+v", hover[1])
	}
	if hover[0].Blur <= rest[0].Blur {
		t.Error("悬停投影应比常态更散（lifted）")
	}
}

// TestSectionButtonHoverTurnsBlue 是整窗无头实测：搜索药丸里四颗内容入口键
// （.top-tool-btn）悬停时图标要转 #0284c7。之前原生版按 .nav-icon-btn 画
// （图标 --studio-muted #475569、悬停不变色），悬停没有任何颜色反馈。
//
// 取按钮矩形里最暗的像素（即图标笔画）：
//   静息 = #64748b 以 opacity .82 压在药丸上 ≈ [127 140 159]
//   悬停 = [2 132 199]（#0284c7 满不透明度）
func TestSectionButtonHoverTurnsBlue(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tt := shellTester(t, a)
	r, ok := tt.Find("热门与推荐")
	if !ok {
		t.Fatal("找不到「热门与推荐」入口键")
	}
	inRect := func() [3]int {
		return darkestIn(tt.Image(), int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
	}
	rest := inRect()
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	hover := inRect()
	t.Logf("入口键图标：静息=%v 悬停=%v", rest, hover)

	// 静息是 #64748b 的 .82 透明版本（明显偏亮、不是 #475569 满值）。
	if !near(rest, [3]int{127, 140, 159}, 4) {
		t.Errorf("静息图标 = %v，应为 #64748b@.82 ≈ [127 140 159]", rest)
	}
	if !near(hover, [3]int{2, 132, 199}, 4) {
		t.Errorf("悬停图标 = %v，应为 #0284c7 = [2 132 199]", hover)
	}
	restBR, hoverBR := rest[2]-rest[0], hover[2]-hover[0]
	if hoverBR-restBR < 80 {
		t.Errorf("悬停应把图标染蓝：B-R %d → %d", restBR, hoverBR)
	}
}

// TestAvatarHoverShadow 是整窗无头实测：头像键（.home-user-avatar）常态有
// 0 6px 16px 的墨色投影、悬停换成 0 12px 28px 的蓝调投影（原版 CSS）。
// 取头像正下方 3px 的像素：常态偏灰（B-R 小），悬停偏蓝（B-R 明显变大）。
func TestAvatarHoverShadow(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tt := shellTester(t, a)
	r, ok := tt.Find("登录账号")
	if !ok {
		t.Fatal("找不到头像键")
	}
	px, py := int(r.X+r.W/2), int(r.Y+r.H+3)
	rest := rgbAt(tt.Image(), px, py)
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	hover := rgbAt(tt.Image(), px, py)
	t.Logf("头像下方：静息=%v 悬停=%v", rest, hover)

	restBR, hoverBR := rest[2]-rest[0], hover[2]-hover[0]
	if restBR >= 12 {
		t.Errorf("静息投影应是墨色（B-R 小），得到 %v (B-R=%d)", rest, restBR)
	}
	if hoverBR-restBR < 12 {
		t.Errorf("悬停投影应转蓝：B-R %d → %d", restBR, hoverBR)
	}
}

// near 比较三个通道是否都在容差内。
func near(got, want [3]int, tol int) bool {
	for i := 0; i < 3; i++ {
		d := got[i] - want[i]
		if d < 0 {
			d = -d
		}
		if d > tol {
			return false
		}
	}
	return true
}
