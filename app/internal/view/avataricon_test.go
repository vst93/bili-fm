package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestUserButtonPersonIcon 钉住未登录头像画的是**描边人形图标**（iconPerson），
// 且完全落在圆框里。
//
// 历史：最早画 "?" 首字；后来照 HeroUI 的 fallback 手绘实心剪影 —— 但剪影是
// 元素**自己**的 Draw，而 mygo 的 `Element.Clip()` 只裁子元素，肩部两角就从圆框
// 里探出来（用户 2026-11-10 第一轮反馈）；补上自裁之后用户说「剪切之后更丑」，
// 于是换成与界面其它图标同一套自绘语言的描边图标。图标是**子元素**，父键的
// `Clip()` 天然把它裁进正圆。
func TestUserButtonPersonIcon(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.LoggedIn = false
	a.Face = ""
	a.UName = ""
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.Theme.uiTheme())
		c.Root().Background(ui.Hex("#ffffff"))
		a.userButton(c)
	}, 60, 60)
	tt.Frame()
	r, ok := tt.Find("登录账号")
	if !ok {
		t.Fatal("找不到登录键")
	}
	img := tt.Image()
	cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W/2
	ink, outside := 0, 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			p := img.RGBAAt(x, y)
			g := (int(p.R)*299 + int(p.G)*587 + int(p.B)*114) / 1000
			if 255-g <= 30 {
				continue
			}
			dx, dy := float32(x)+0.5-cx, float32(y)+0.5-cy
			if dx*dx+dy*dy > rad*rad {
				outside++
				continue
			}
			ink++
		}
	}
	t.Logf("登录键 %+v：圆内图标墨迹 %d 像素，圆外 %d 像素", r, ink, outside)
	if outside != 0 {
		t.Errorf("图标在圆框外画了 %d 个像素", outside)
	}
	// 描边图标（不是实心剪影）：墨迹像素数落在一个明确区间里 —— 太少说明图标
	// 没画 / 被裁没了，太多说明又变回实心块。
	if ink < 30 || ink > 200 {
		t.Errorf("圆内图标墨迹 %d 像素，超出描边人形应有的范围（30~200）", ink)
	}
}
