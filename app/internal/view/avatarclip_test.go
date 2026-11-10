package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestUserButtonSilhouetteStaysInCircle 钉住未登录头像的人形剪影被裁进正圆。
//
// 剪影是元素**自己**的 Draw，而 mygo 的 `Element.Clip()` 只裁子元素 ——
// 肩部底边撑满方盒下缘、两角落在圆外，不自己裁就会从圆框里探出来
// （用户 2026-11-10 报「未登录时的登录图标超出了背景的圆框」，Linux 实测）。
//
// 判法：把键的方盒按内接圆分成圆内 / 圆外，断言**圆外没有任何墨迹**
// （比白底暗 3 灰阶以上）。反证：去掉 Draw 里的 p.Clip → 左肩下角立刻变暗
// （实测 142 灰阶）。
func TestUserButtonSilhouetteStaysInCircle(t *testing.T) {
	orig := userAvatarSilhouetteFn
	defer func() { userAvatarSilhouetteFn = orig }()

	render := func(stub bool) (*image.RGBA, ui.Rect) {
		userAvatarSilhouetteFn = orig
		if stub {
			userAvatarSilhouetteFn = func(*ui.Painter, ui.Rect, ui.Color) {}
		}
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.LoggedIn = false
		a.Face = "" // 没有头像位图 → 走剪影分支
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.Theme.uiTheme())
			c.Root().Background(ui.Hex("#ffffff"))
			a.userButton(c)
		}, 60, 60)
		tt.Frame()
		userAvatarSilhouetteFn = orig
		r, ok := tt.Find("登录账号")
		if !ok {
			t.Fatal("找不到登录键")
		}
		return tt.Image(), r
	}

	on, r := render(false)
	off, _ := render(true)
	cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W/2
	// 圆外：两张图必须**逐像素相同** —— 剪影（含肩部两角）一点都不能露在圆外。
	outside, ink := 0, 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			dx, dy := float32(x)-cx, float32(y)-cy
			i := on.PixOffset(x, y)
			j := off.PixOffset(x, y)
			changed := on.Pix[i] != off.Pix[j] || on.Pix[i+1] != off.Pix[j+1] || on.Pix[i+2] != off.Pix[j+2]
			if dx*dx+dy*dy > rad*rad {
				if changed {
					outside++
				}
				continue
			}
			if changed {
				ink++
			}
		}
	}
	t.Logf("登录键 %+v，圆外差异像素 = %d，圆内剪影像素 = %d", r, outside, ink)
	// 允许极少量：裁剪边自身的抗锯齿会在圆周线上多压出一两个像素
	//（实测 2）。修之前这里是 60+（整条肩部下缘），量级完全不同。
	if outside > 4 {
		t.Errorf("剪影在圆框外画了 %d 个像素（未裁进正圆）", outside)
	}
	if ink < 40 {
		t.Errorf("圆内剪影只有 %d 像素，像是被裁没了", ink)
	}
}
