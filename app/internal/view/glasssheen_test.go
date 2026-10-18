package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本文件钉住两块**玻璃浮层**面板上的 `--glass-sheen-static` 洗光：
//
//	#settings-menu                globals.css:1928（`!important`）
//	.video-owner-picker-menu      globals.css:2136
//
// 原版这两条的 `background-image` 都是同一条令牌 —— 左上径向热点
// （`radial-gradient(140% 80% at 22% -5%, 白 .24, transparent 48%)`）叠一条
// 170° 斜向洗光（白 .07 → .015@35% → 0@60%）。原生版此前两块面板都只画了
// 玻璃材质（框架 glass.Glass 的边缘聚光只在 1px 环上），面板**内部**是平的。
//
// 判据不是「绝对亮度」（面板底本来就接近白，白 .16 的洗光在 luma 上只有个位数），
// 而是**同一次渲染里开/关洗光的逐像素差**：开洗光的那一帧与把
// `playerSheenPaint` 换桩成空的那一帧相减，面板左上角的像素必须变（说明这层真的
// 画在面板上），而面板下缘（径向到 48%、170° 线性到 60% 就归零）几乎不变 ——
// 这正是「左上亮、右下无」的定向洗光。

// panelRegion 是面板内一块采样矩形（整像素）。
type panelRegion struct{ x0, y0, x1, y1 int }

// panelTopRegion / panelBottomRegion 取面板左上 / 左下两块**避开条目文案**的
// 区域（文案从 panel.left+16 起，所以 x 只取到 left+14；上下各让开 6px 边框
// 与 `inset 0 1px 1px` 的内高光）。
func panelTopRegion(panel ui.Rect) panelRegion {
	return panelRegion{int(panel.X) + 3, int(panel.Y) + 10, int(panel.X) + 14, int(panel.Y) + 34}
}

func panelBottomRegion(panel ui.Rect) panelRegion {
	return panelRegion{int(panel.X) + 3, int(panel.Y+panel.H) - 34, int(panel.X) + 14, int(panel.Y+panel.H) - 10}
}

// meanDiff 返回两块同尺寸图上该区域的平均逐通道绝对差（0..255）。
func meanDiff(t *testing.T, a, b *image.RGBA, reg panelRegion) float64 {
	t.Helper()
	sum, n := 0, 0
	for y := reg.y0; y < reg.y1; y++ {
		for x := reg.x0; x < reg.x1; x++ {
			ca, cb := pixelAt(t, a, x, y), pixelAt(t, b, x, y)
			for i := 0; i < 3; i++ {
				d := ca[i] - cb[i]
				if d < 0 {
					d = -d
				}
				sum += d
			}
			n += 3
		}
	}
	if n == 0 {
		t.Fatal("空采样区域")
	}
	return float64(sum) / float64(n)
}

// sheenDiff 渲染两帧（开洗光 / 关洗光）并返回面板左上与左下的平均差。
func sheenDiff(t *testing.T, tst *ui.Tester, panel ui.Rect) (top, bottom float64) {
	t.Helper()
	on := tst.Image()
	saved := playerSheenPaint
	playerSheenPaint = func(*ui.Painter, ui.Rect, float32, *ui.Bitmap) {}
	defer func() { playerSheenPaint = saved }()
	tst.Frame()
	off := tst.Image()
	// 换桩状态已还原，后续断言用不到再取帧。
	return meanDiff(t, on, off, panelTopRegion(panel)), meanDiff(t, on, off, panelBottomRegion(panel))
}

// TestSettingsMenuSheen 实测设置菜单面板的 `--glass-sheen-static` 洗光。
func TestSettingsMenuSheen(t *testing.T) {
	_, tst := settingsProbe(t, 21)
	items := menuRects(t, tst)
	top0 := items[0].Y - settingsMenuPadY
	last := items[len(items)-1]
	panel := ui.Rect{
		X: items[0].X, Y: top0, W: items[0].W,
		H: last.Y + last.H + settingsMenuPadY - top0,
	}
	top, bottom := sheenDiff(t, tst, panel)
	t.Logf("设置菜单 面板 %+v 洗光开/关 平均差：左上 %.2f / 左下 %.2f", panel, top, bottom)
	if top < 5 {
		t.Errorf("面板左上角只差 %.2f —— `--glass-sheen-static` 洗光没铺上", top)
	}
	if bottom > 0.5 {
		t.Errorf("面板左下角也差 %.2f —— 洗光不该到底部还亮（径向 48%% / 线性 60%% 已归零）", bottom)
	}
}

// TestCollabMenuSheen 实测合作 UP 主下拉面板的洗光（同一族、同一个令牌）。
func TestCollabMenuSheen(t *testing.T) {
	_, tst := collabProbe(t, 21)
	_, first := openCollab(t, tst)
	panel := ui.Rect{
		X: first.X - collabMenuPad, Y: first.Y - collabMenuPad,
		W: first.W + 2*collabMenuPad,
		// 三位参与者撑不到 max-height：高度按 3 条最小条目高推。
		H: 3*collabItemMinH + 2*collabMenuPad,
	}
	top, bottom := sheenDiff(t, tst, panel)
	t.Logf("合作下拉 面板 %+v 洗光开/关 平均差：左上 %.2f / 左下 %.2f", panel, top, bottom)
	if top < 5 {
		t.Errorf("面板左上角只差 %.2f —— `--glass-sheen-static` 洗光没铺上", top)
	}
	if bottom > 0.5 {
		t.Errorf("面板左下角也差 %.2f —— 洗光不该到底部还亮", bottom)
	}
}
