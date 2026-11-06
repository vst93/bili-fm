package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestCircleHitTest 钉住 mygo 0.8.0 的 Element.HitTest 用法：正圆按钮只在
// 圆内吃指针，四角落到下面那层（那里没有动作）。
//
// 唱片（263px 正圆）与 UP 头像（44px 正圆）此前命中区是整个方盒：四角点到
// 「空白」其实也在按唱片（用户报的「点封面边角也能暂停」这类误触）。
func TestCircleHitTest(t *testing.T) {
	r := ui.Rect{X: 100, Y: 50, W: 200, H: 200}
	cx, cy := float32(200), float32(150)
	rad := float32(100)

	// 圆内（含圆心与贴近圆周内侧的点）要吃指针。
	for _, p := range [][2]float32{
		{cx, cy},           // 圆心
		{cx + 90, cy},      // 右侧内侧
		{cx - 60, cy - 70}, // 左上内侧
	} {
		if !circleHitTest(p[0], p[1], r) {
			t.Errorf("圆内点 (%v, %v) 应吃指针", p[0], p[1])
		}
	}

	// 四角（外接正方形的角）不该吃。
	for _, p := range [][2]float32{
		{cx + rad, cy + rad},         // 右下角
		{cx - rad, cy - rad},         // 左上角
		{cx + rad - 1, cy + rad - 1}, // 角内 1px
	} {
		if circleHitTest(p[0], p[1], r) {
			t.Errorf("方形角落 (%v, %v) 不该吃指针", p[0], p[1])
		}
	}

	// 正方形外（离圆心超过半径）也不吃。
	if circleHitTest(cx+rad+1, cy, r) {
		t.Error("圆外的点不该吃指针")
	}

	// 椭圆盒（宽高不等）取短边做半径：不会伸到短边的圆外。
	ell := ui.Rect{X: 0, Y: 0, W: 300, H: 100}
	ecx, ecy := float32(150), float32(50)
	if !circleHitTest(ecx, ecy+49, ell) {
		t.Error("椭圆短边内侧应吃指针")
	}
	if circleHitTest(ecx+140, ecy, ell) {
		t.Error("椭圆长边方向超出短边半径的点不该吃指针")
	}
}

// TestDiscCornersAreNotClickable 是无头实测的反证：唱片的四个角点下去不会
// 触发播放/暂停，圆心点下去会。
func TestDiscCornersAreNotClickable(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Info = &Info{Bvid: "BV1", Cid: 2, Title: "标题"}
	plays := 0
	a.Act.TogglePlay = func() { plays++ }

	tt := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600))
	rect, ok := tt.Find("播放")
	if !ok {
		t.Fatal("找不到唱片（标签「播放」）")
	}
	// 圆心：应该能点。
	tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	tt.Frame()
	if plays != 1 {
		t.Fatalf("点唱片圆心：plays = %d，想 1", plays)
	}
	// 右上角内侧 2px（方盒内、圆外）：不该触发。
	cornerX, cornerY := rect.X+rect.W-2, rect.Y+2
	tt.ClickAt(cornerX, cornerY)
	tt.Frame()
	if plays != 1 {
		t.Fatalf("点唱片方盒的右上角（圆外）：plays = %d，想仍是 1（不该误触）", plays)
	}
}
