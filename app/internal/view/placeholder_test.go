package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestPlaceholderArtFollowsAppearance 钉住占位图按解析后的明暗切换。深色下
// 继续用纯白底那张就是用户报的「暗色模式里一块白」。
func TestPlaceholderArtFollowsAppearance(t *testing.T) {
	light := ThemeFor(PeriodAt(12)).withAppearance("light", 0)
	dark := ThemeFor(PeriodAt(12)).withAppearance("dark", 0)
	if placeholderCoverFor(light) != placeholderCover {
		t.Fatal("浅色应使用 cover.png 占位图")
	}
	if placeholderCoverFor(dark) != placeholderCoverDark {
		t.Fatal("深色应使用 cover-dark.png 占位图")
	}
	if coverLogoFor(light) != LogoCover {
		t.Fatal("浅色封面应使用白底 logo")
	}
	if coverLogoFor(dark) != LogoCoverDark {
		t.Fatal("深色封面应使用深色底 logo")
	}
	if placeholderCoverDark == nil || LogoCoverDark == nil {
		t.Fatal("深色占位图解码失败")
	}
}

// TestCardPlaceholderDarkNotWhite 是无头实测：深色卡片封面未就绪时，占位图
// 左上背景应是深色（不是纯白）；浅色下仍是浅底。采样点避开居中的台标。
func TestCardPlaceholderDarkNotWhite(t *testing.T) {
	luma := func(mode string) int {
		a := NewApp(func() {})
		a.SetAppearance(mode, 0)
		a.Images = newRecordingCache()
		spec := cardSpec{
			Key: "ph-card", Cover: "", Title: "标题",
			Lazy:   true,
			Fields: [cardFieldsMax]cardField{{Kind: fieldAuthor, Text: "UP主"}}, NFields: 1,
		}
		img := ui.Render(func(c *ui.Context) {
			c.Root().Background(ui.RGB(90, 90, 90))
			ui.Box(c).Absolute().Left(20).Top(20).Size(220, 170).Children(func() {
				a.renderCard(c, spec)
			})
		}, 300, 260, 1)
		p := img.RGBAAt(30, 30) // 封面左上，台标之外
		return (int(p.R)*299 + int(p.G)*587 + int(p.B)*114) / 1000
	}
	if dark := luma("dark"); dark > 90 {
		t.Fatalf("深色卡片占位图仍然偏白：左上亮度 %d", dark)
	}
	if light := luma("light"); light < 170 {
		t.Fatalf("浅色卡片占位图应保持浅底：左上亮度 %d", light)
	}
}

// TestDefaultCoverDarkIsNotWhite 钉住主封面盘面在深色下也不再铺白底：
// 无封面时用深色底台标，采样盘面左侧（台标之外）应为深色。
func TestDefaultCoverDarkIsNotWhite(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.SetAppearance("dark", 0)
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"} // 无 Cover
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	r, ok := tst.Find("播放")
	if !ok {
		t.Fatalf("找不到盘面：%v", tst.Texts())
	}
	x, y := int(r.X+r.W*0.18), int(r.Y+r.H*0.5)
	p := tst.Image().RGBAAt(x, y)
	l := (int(p.R)*299 + int(p.G)*587 + int(p.B)*114) / 1000
	if l > 90 {
		t.Fatalf("深色默认封面盘面亮度 %d 偏白——仍在用白底占位图", l)
	}
}
