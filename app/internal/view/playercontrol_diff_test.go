package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放控制区域对照 main 的差异修复）的回归测试，四条判据各配一个
// 会在改错时变红的断言：
//
//  1. 时间标签行盒：原版 `#player .player-time { line-height: 18px }`
//     （globals.css:3051；mini 覆写 globals.css:4969 只改 font-size 9px，
//     行高仍是 18px）——框架自然行盒（11px≈17）达不到，timeTextAt 必须
//     用显式盒高压出 18。
//  2. 倍速档位胶囊：原版 `.player-speed-option { min-width:36px;
//     padding:4px 8px }`（globals.css:3361）**没有固定高**——胶囊高度 =
//     行盒 + 上下内边距；此前误写 Height(24)+Padding(0,8)。
//  3. 恰饭键悬停：原版 `.player-sponsor-button` 不在工具键 :hover 名单
//     （globals.css:2988-3003 只含 next/volume/speed/eq），也没有通用的
//     `.player-button:hover`——悬停完全不变色。
//  4. 恰饭键按下：不在 :active 特殊名单（globals.css:3021-3043 只含
//     play/next/volume/speed/eq），只有通用 `.player-button:active` 的
//     scale(.93)（globals.css:2848）——没有内压阴影、不换底色。

// nearEq 是容差比较（浮点盒高避免直接判等）。
func nearEq(got, want float32) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d <= 0.01
}

// TestPlayerTimeLineBox 钉住时间标签的行盒：主窗 11px 与 mini 9px 两处
// （共用 timeTextAt）的元素盒高都必须是原版 line-height 的 18px。
func TestPlayerTimeLineBox(t *testing.T) {
	a := NewApp(func() {})
	tst := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			a.timeText(c, 754)
		})
	}, 140, 40)
	tst.Frame()
	lbl, ok := tst.Find(fmtTime(754))
	if !ok {
		t.Fatal("主窗时间标签没画出来")
	}
	if !nearEq(lbl.H, 18) {
		t.Errorf("主窗时间标签盒高 %v，want 18（原版 line-height:18px）", lbl.H)
	}

	// mini：字号 9、列宽 44，行高覆写不变（body.mini-mode 那条）。
	mst := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			a.timeTextAt(c, 754, 9, 44)
		})
	}, 100, 40)
	mst.Frame()
	m, ok := mst.Find(fmtTime(754))
	if !ok {
		t.Fatal("mini 时间标签没画出来")
	}
	if !nearEq(m.H, 18) {
		t.Errorf("mini 时间标签盒高 %v，want 18（mini 覆写保持 line-height:18px）", m.H)
	}
}

// TestSpeedOptionCapsuleAutoHeight 钉住档位胶囊是「padding 撑出的 auto 高度」：
// 盒高 = 上下内边距 8 + 11px 文本的自然行盒。若有人改回固定 Height（此前是
// Height(24)+Padding(0,8)），盒高就不再等于这个值，测试变红。
func TestSpeedOptionCapsuleAutoHeight(t *testing.T) {
	a := NewApp(func() {})
	a.Speed = 1.0
	tst := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			a.speedOption(c, 1.25)
		})
	}, 200, 60)
	tst.Frame()
	opt, ok := tst.Find("1.25x")
	if !ok {
		t.Fatal("倍速档位胶囊没画出来")
	}
	want := float32(8) + naturalLineBox(11)
	if !nearEq(opt.H, want) {
		t.Errorf("档位胶囊盒高 %v，want %v（原版 padding:4px 8px、auto 高度）", opt.H, want)
	}
}

// TestSponsorButtonHoverRests 是无头实测：指针悬停在恰饭键上时键面底色
// 必须与静息完全一致（原版没有给这颗键任何 :hover 规则）。
func TestSponsorButtonHoverRests(t *testing.T) {
	_, tst := playerBarProbe(t, nil)
	b, ok := tst.Find(sponsorTip())
	if !ok {
		t.Fatal("播放栏里找不到恰饭键")
	}
	// 取键内左缘附近的底色像素：避开图标（居中 ±10px）与右上角状态点；
	// tooltip 有 600ms 悬停延迟且画在键外上方，不会盖到采样点。
	sx, sy := int(b.X+4), int(b.Y+b.H/2)
	rest := rgbAt(tst.Image(), sx, sy)
	tst.Move(b.X+b.W/2, b.Y+b.H/2)
	tst.Frame()
	hov := rgbAt(tst.Image(), sx, sy)
	t.Logf("恰饭键底色 静息=%v 悬停=%v", rest, hov)
	if rest != hov {
		t.Errorf("恰饭键悬停不应变色（原版无 :hover 规则）：%v → %v", rest, hov)
	}
}

// TestSponsorButtonPressOnlyScales 是无头实测：按住恰饭键时键面底色必须与
// 静息一致——原版只吃通用 `.player-button:active` 的 scale(.93)，不在换底
// 色/画内压阴影的 :active 特殊名单里。采样点取键内左侧 4px：缩放 .93 后
// 半宽仍有 ~15.8px，采样点（距中心 13px）仍在键内。
func TestSponsorButtonPressOnlyScales(t *testing.T) {
	_, tst := playerBarProbe(t, nil)
	b, ok := tst.Find(sponsorTip())
	if !ok {
		t.Fatal("播放栏里找不到恰饭键")
	}
	sx, sy := int(b.X+4), int(b.Y+b.H/2)
	rest := rgbAt(tst.Image(), sx, sy)
	tst.Press(b.X+b.W/2, b.Y+b.H/2)
	tst.Frame()
	prs := rgbAt(tst.Image(), sx, sy)
	tst.Release(b.X+b.W/2, b.Y+b.H/2)
	t.Logf("恰饭键底色 静息=%v 按下=%v", rest, prs)
	if rest != prs {
		t.Errorf("恰饭键按下不应换底/画内压（原版不在 :active 特殊名单）：%v → %v", rest, prs)
	}
}
