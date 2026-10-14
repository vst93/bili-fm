package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestMiniPinIconVariant 钉住迷你窗「窗口置顶」键的图标选择：原版是
// `<Pushpin theme={isPinned ? "filled" : "outline"}>`，置顶时换成 icon-park 的
// **实心**变体。此前原生版两态共用同一枚描边图钉（置顶只有蓝图标/白底的区别）。
func TestMiniPinIconVariant(t *testing.T) {
	if got := miniPinIcon(false); got != iconPin {
		t.Error("未置顶应使用描边图钉 iconPin")
	}
	if got := miniPinIcon(true); got != iconPinFilled {
		t.Error("置顶应使用实心图钉 iconPinFilled")
	}
	if iconPinFilled == iconPin {
		t.Error("实心与描边应是两枚不同的 SVG")
	}
	if iconPinFilled == nil || iconPin == nil {
		t.Fatal("图钉图标不应为 nil")
	}
}

// inkPockets 数出矩形内「被图标像素围住」的背景像素（四邻域从矩形四边泛洪，
// 淹不到的背景即封闭空洞）。描边图标（空心）会有空洞，实心图标不会有。
//
// ink 的判据沿用 TestMiniPinButtonState 的蓝色判据：置顶时图标是
// --studio-blue（#0284c7），B 通道比 R 高出 80 以上；按钮底色是中性白玻璃，
// 达不到这个差值。
func inkPockets(img *image.RGBA, r ui.Rect) int {
	ink := func(x, y int) bool {
		i := img.PixOffset(x, y)
		return int(img.Pix[i+2])-int(img.Pix[i]) > 80
	}
	W, H := int(r.W), int(r.H)
	seen := make([]bool, W*H)
	var stack [][2]int
	push := func(x, y int) {
		if x < 0 || y < 0 || x >= W || y >= H {
			return
		}
		k := y*W + x
		if seen[k] || ink(int(r.X)+x, int(r.Y)+y) {
			return
		}
		seen[k] = true
		stack = append(stack, [2]int{x, y})
	}
	for x := 0; x < W; x++ {
		push(x, 0)
		push(x, H-1)
	}
	for y := 0; y < H; y++ {
		push(0, y)
		push(W-1, y)
	}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		push(p[0]+1, p[1])
		push(p[0]-1, p[1])
		push(p[0], p[1]+1)
		push(p[0], p[1]-1)
	}
	n := 0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if !seen[y*W+x] && !ink(int(r.X)+x, int(r.Y)+y) {
				n++
			}
		}
	}
	return n
}

// TestMiniPinFilledIsSolid 无头像素实测：置顶时图钉真的画成了实心 —— 按钮矩形里
// 不该有被图标围住的背景空洞；把 miniPinIcon 换成描边图钉后应重新出现空洞。
// 这反证「实心变体」接在置顶态上（而不只是孤立的一枚 SVG 常量）。
func TestMiniPinFilledIsSolid(t *testing.T) {
	pinned := miniProbe(t, true)
	r, ok := pinned.Find("取消窗口置顶")
	if !ok {
		t.Fatal("找不到置顶键")
	}
	solid := inkPockets(pinned.Image(), r)

	saved := miniPinIcon
	miniPinIcon = func(bool) *ui.SVG { return iconPin }
	defer func() { miniPinIcon = saved }()
	outline := inkPockets(miniProbe(t, true).Image(), r)
	t.Logf("置顶键封闭空洞：实心 %d、描边 %d", solid, outline)

	if solid != 0 {
		t.Errorf("实心图钉仍有 %d 个封闭空洞，应为 0", solid)
	}
	if outline == 0 {
		t.Error("换成描边图钉后没有出现封闭空洞：反证失败（像素判据或接线有问题）")
	}
}

// TestTitleBarBtnHighlightSpecs 钉住标题栏图标键 / 迷你窗窗口控制键的高光阴影。
//
// 原版两条互斥路径：
//
//	::after（悬停 opacity 0→1）        inset 上高光 + inset 下暗线 + 外投影（三条）
//	aria-pressed 且不在悬停/按下/聚焦   只有 inset 上高光（一条）
func TestTitleBarBtnHighlightSpecs(t *testing.T) {
	// 置顶且鼠标不在键上：单层内高光（原版 mini-window-controls 那条直接阴影）。
	pin := titleBarBtnHighlightFor(true, false)
	if len(pin.Inset) != 1 || len(pin.Outer) != 0 {
		t.Fatalf("置顶未悬停应为 1 层内影 / 0 层外影，得到 %d / %d", len(pin.Inset), len(pin.Outer))
	}
	if want := (shadowSpec{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.90)}); pin.Inset[0] != want {
		t.Errorf("置顶未悬停内影 = %+v, want %+v", pin.Inset[0], want)
	}

	// 悬停（置顶或不置顶都一样）：与 .nav-icon-btn::after 逐字相同的三条。
	hover := titleBarBtnHighlightFor(false, true)
	want := navHoverHighlightSpecs()
	if len(hover.Inset) != len(want.Inset) || len(hover.Outer) != len(want.Outer) {
		t.Fatalf("悬停应为 %d 层内影 / %d 层外影，得到 %d / %d",
			len(want.Inset), len(want.Outer), len(hover.Inset), len(hover.Outer))
	}
	for i := range want.Inset {
		if hover.Inset[i] != want.Inset[i] {
			t.Errorf("悬停内影[%d] = %+v, want %+v", i, hover.Inset[i], want.Inset[i])
		}
	}
	for i := range want.Outer {
		if hover.Outer[i] != want.Outer[i] {
			t.Errorf("悬停外影[%d] = %+v, want %+v", i, hover.Outer[i], want.Outer[i])
		}
	}
	// 置顶未悬停与悬停必须不同（否则「单层」例外就是死代码）。
	if len(pin.Inset) == len(hover.Inset) && len(pin.Outer) == len(hover.Outer) {
		t.Error("置顶未悬停与悬停画了同一组阴影：aria-pressed 的单层例外没生效")
	}
}

// TestTitleBarBtnHighlightWired 无头实测这组高光真的铺在迷你窗置顶键上：把
// titleBarBtnHighlight 换成 no-op 后，按键矩形里有像素跟着变。
func TestTitleBarBtnHighlightWired(t *testing.T) {
	on := miniProbe(t, true)
	r, ok := on.Find("取消窗口置顶")
	if !ok {
		t.Fatal("找不到置顶键")
	}
	saved := titleBarBtnHighlight
	titleBarBtnHighlight = func(b ui.Element, active, hovered bool) ui.Element { return b }
	defer func() { titleBarBtnHighlight = saved }()
	off := miniProbe(t, true)

	diff, total := diffInRect(on.Image(), off.Image(), r)
	t.Logf("置顶键高光：%d/%d 像素随 titleBarBtnHighlight 变化", diff, total)
	if diff < total/20 {
		t.Errorf("只有 %d/%d 像素受高光影响（<5%%）—— 高光没接在窗口控制键上？", diff, total)
	}
}
