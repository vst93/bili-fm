package view

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestAppearancePreservesPreferences(t *testing.T) {
	a := NewApp(func() {})
	a.SetAppearance("dark", .237)
	a.RefreshTheme()
	a.SetReduceTransparency(true)
	if a.Theme.Mode != "dark" || a.Theme.Transparency != .237 || a.Theme.backgroundOpacity() != 1 {
		t.Fatalf("appearance lost across refresh / accessibility change: %+v", a.Theme)
	}
	a.SetReduceTransparency(false)
	if math.Abs(float64(a.Theme.backgroundOpacity())-.763) > 0.0001 {
		t.Fatal("transparency did not restore")
	}
	a.SetAppearance("light", 0)
	base := ThemeFor(a.Theme.Period)
	if a.Theme.Ink != base.Ink || a.Theme.PlayerSurface != base.PlayerSurface {
		t.Fatal("light appearance did not restore")
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 2} {
		a.SetAppearance("unknown", value)
		if a.AppearancePref() != "light" || a.Theme.Mode != "light" || a.Theme.Transparency < 0 || a.Theme.Transparency > .4 || math.IsNaN(a.Theme.Transparency) {
			t.Fatal("invalid persisted appearance was not normalized")
		}
	}
}

func TestAppearanceControls(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	saved := 0
	a.Act.PersistAppearance = func() { saved++ }
	tt := shellTester(t, a)
	trigger, ok := tt.Find("外观")
	if !ok {
		t.Fatal("missing appearance trigger")
	}
	premium, ok := tt.Find("关闭高级质感（更省电）")
	if !ok || math.Abs(float64(trigger.Y-premium.Y)) > 1 || math.Abs(float64(trigger.X+trigger.W+6-premium.X)) > 1 {
		t.Fatalf("appearance is not next to existing cover controls: %v / %v", trigger, premium)
	}
	tt.Move(trigger.X+trigger.W/2, trigger.Y+trigger.H/2)
	tt.Advance(250 * time.Millisecond)
	if err := tt.Click("外观"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.ShowAppearance {
		t.Fatal("trigger did not open appearance")
	}
	if err := tt.Click("深色主题"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Theme.Mode != "dark" || !a.ShowAppearance || saved != 1 {
		t.Fatalf("theme choice: mode=%s open=%v saved=%d", a.Theme.Mode, a.ShowAppearance, saved)
	}
	if err := tt.Click("透明度 25%"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Theme.Transparency != .25 || saved != 2 {
		t.Fatal("preset did not apply and persist")
	}
	r, ok := tt.Find("调整背景透明度")
	if !ok {
		t.Fatal("missing slider")
	}
	// Click near a detent, then use the keyboard to leave it by a fine step.
	tt.ClickAt(r.X+8+(r.W-16)*.245/.4, r.Y+r.H/2)
	tt.Frame()
	if math.Abs(a.Theme.Transparency-.25) > .00001 {
		t.Fatalf("pointer failed to snap: %v, rect %v", a.Theme.Transparency, r)
	}
	before := saved
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if a.Theme.Transparency <= .25 || a.Theme.Transparency >= .26 || saved <= before {
		t.Fatalf("keyboard fine adjustment failed: %v saved=%d", a.Theme.Transparency, saved)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.ShowAppearance {
		t.Fatal("escape did not close appearance")
	}
}

// These renders exercise the same native view tree as the app and can be
// exported for visual QA without opening a user's data store or playback.
func TestAppearanceRender(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		for _, width := range []int{800, 400} {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.SetAppearance(mode, .25)
			a.Info = &Info{Bvid: "BVdemo", Title: "让喜欢的声音，陪你度过今天", OwnerName: "音乐现场", Desc: "一段旋律，一点日常。"}
			a.Track = &Track{Bvid: "BVdemo", Title: a.Info.Title, Up: a.Info.OwnerName}
			a.ShowAppearance = true
			tt := shellTesterSize(t, a, width, 600)
			for _, label := range []string{"浅色主题", "自动主题", "深色主题", "透明度 40%"} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > float32(width)+1 || r.Y+r.H > 600 {
					t.Fatalf("%s %d: clipped %s: %v", mode, width, label, r)
				}
			}
			if dir := os.Getenv("BILI_APPEARANCE_SHOTS"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%d.png", mode, width)))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, tt.Image())
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestAppearanceDragPreview(t *testing.T) {
	a := NewApp(func() {})
	a.ShowAppearance = true
	saved := 0
	a.Act.PersistAppearance = func() { saved++ }
	tt := shellTester(t, a)
	r, ok := tt.Find("调整背景透明度")
	if !ok {
		t.Fatal("missing slider")
	}
	x := func(value float32) float32 { return r.X + 8 + (r.W-16)*value/.4 }
	y := r.Y + r.H/2
	tt.Press(x(.1), y)
	tt.Move(x(.197), y)
	tt.Frame()
	if math.Abs(a.Theme.Transparency-.197) > .001 || saved != 0 {
		t.Fatalf("continuous live preview: value=%v saves=%d", a.Theme.Transparency, saved)
	}
	tt.Move(x(.246), y)
	tt.Frame()
	if a.Theme.Transparency != .25 || saved != 0 {
		t.Fatalf("drag snap: value=%v saves=%d", a.Theme.Transparency, saved)
	}
	tt.Release(x(.246), y)
	tt.Frame()
	if saved != 1 {
		t.Fatalf("release must persist once, got %d", saved)
	}
	tt.ClickAt(10, 500)
	tt.Frame()
	if a.ShowAppearance {
		t.Fatal("outside click did not close appearance")
	}
}

func TestAppearanceWindowAlpha(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		for _, mini := range []bool{false, true} {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.Mini = mini
			a.Info = &Info{Title: "保持文字清晰"}
			a.Track = &Track{Title: a.Info.Title}
			a.SetAppearance(mode, .4)
			tt := shellTester(t, a)
			alpha := tt.Image().RGBAAt(10, 300).A
			if alpha >= 245 || alpha < 150 {
				t.Fatalf("%s mini=%v: transparent background alpha=%d", mode, mini, alpha)
			}
			if !mini {
				r, ok := tt.Find("保持文字清晰")
				if !ok {
					t.Fatal("missing foreground title")
				}
				opaqueInk := false
				frame := tt.Image()
				for y := int(r.Y); y < int(r.Y+r.H); y++ {
					for x := int(r.X); x < int(r.X+r.W); x++ {
						if frame.RGBAAt(x, y).A == 255 {
							opaqueInk = true
							break
						}
					}
				}
				if !opaqueInk {
					t.Fatal("window transparency faded the foreground title")
				}
			}
			a.SetReduceTransparency(true)
			tt.Frame()
			if alpha := tt.Image().RGBAAt(10, 300).A; alpha != 255 {
				t.Fatalf("reduced transparency alpha=%d", alpha)
			}
			if a.Theme.Transparency != .4 {
				t.Fatal("accessibility override discarded saved preference")
			}
		}
	}
}

func TestAppearanceDarkSurfaces(t *testing.T) {
	for _, scenario := range []string{"playlist", "modal", "login", "mini", "light-playlist", "controls"} {
		t.Run(scenario, func(t *testing.T) {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.SetAppearance("dark", .4)
			a.Track = &Track{Bvid: "BVdemo", Title: "喜欢的声音", Part: "现场演出", Up: "音乐现场"}
			width, height := 800, 600
			switch scenario {
			case "playlist", "light-playlist":
				if scenario == "light-playlist" {
					a.SetAppearance("light", .4)
				}
				a.Drawer, a.PlaylistTab, a.PlayingPlaylist = DrawerPlaylist, ListUser, ListUser
				a.Playlist = []PlayItem{{ID: "a", Part: "第一集 · 钢琴", Title: "午后音乐会"}, {ID: "b", Part: "第二集 · 小提琴", Title: "现场演奏"}}
			case "modal":
				a.Modal = &Modal{Kind: "message", Type: "question", Title: "发现新版本", Message: "新版本已准备好，是否现在更新？", Buttons: []ModalButton{{Label: "立即更新", Primary: true}, {Label: "稍后再说"}}}
				a.modalOpenedAt = time.Now().Add(-time.Second)
			case "login":
				a.Modal = &Modal{Kind: "login"}
				a.modalOpenedAt = time.Now().Add(-time.Second)
			case "controls":
				a.Playlist = []PlayItem{{ID: "a", Title: "喜欢的声音"}, {ID: "b", Title: "继续聆听"}}
				a.PlayingPlaylist = ListUser
				a.Info = &Info{Title: "喜欢的声音", OwnerName: "音乐现场", Desc: "一段旋律，一点日常。"}
			case "mini":
				a.Mini = true
				width, height = 400, 155
			}
			tt := shellTesterSize(t, a, width, height)
			tt.Advance(time.Second)
			if scenario == "modal" {
				r, ok := tt.Find("稍后再说")
				if !ok {
					t.Fatal("missing secondary action")
				}
				pixel := tt.Image().RGBAAt(int(r.X+8), int(r.Y+r.H/2))
				if pixel.R > 110 {
					t.Fatalf("dark secondary button has a light fill: %v", pixel)
				}
			}
			if dir := os.Getenv("BILI_APPEARANCE_SHOTS"); dir != "" {
				f, err := os.Create(filepath.Join(dir, "dark-"+scenario+".png"))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, tt.Image())
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// Render the real panes over contrasting backgrounds: more transparency must
// reveal more background variation without fading the controls themselves.
func TestAppearanceGlassTransmitsBackground(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		for _, pane := range []string{"search", "player", "drawer"} {
			luma := func(transparency float64, background string) float64 {
				a := NewApp(func() {})
				a.SetAppearance(mode, transparency)
				tt := settleTester(ui.NewTester(func(c *ui.Context) {
					c.SetTheme(a.Theme.uiTheme())
					c.Root().Background(ui.Hex(background))
					switch pane {
					case "search":
						a.searchBar(c, homeLayout{SearchWidth: 520})
					case "player":
						a.playerBar(c)
					case "drawer":
						a.Drawer = DrawerPlaylist
						a.drawer(c)
					}
				}, 600, 300))
				x, y := 300, 5
				if pane == "drawer" {
					y = 270
				}
				p := tt.Image().RGBAAt(x, y)
				return .2126*float64(p.R) + .7152*float64(p.G) + .0722*float64(p.B)
			}
			spread := func(transparency float64) float64 {
				return luma(transparency, "#dce5f0") - luma(transparency, "#17263b")
			}
			solid, clear := spread(0), spread(.4)
			// 深色的大面积 veil 已经收到 0.06×0.40 ≈ .024（用户 2026-11-11
			// 要求「深色再透一点」），滑块拉满能再加的余量本来就小，所以
			// 增量门槛按比例放松；**单调性**（clear > solid）仍然钉死。
			minGain := 8.0
			if mode == "dark" {
				minGain = 3
			}
			if clear < 75 || clear < solid+minGain {
				t.Fatalf("%s %s: backdrop variation %.1f -> %.1f; transparency should make the pane lighter", mode, pane, solid, clear)
			}
		}
	}
}

func TestAppearanceReducedTransparencyControls(t *testing.T) {
	a := NewApp(func() {})
	a.SetAppearance("dark", .25)
	a.SetReduceTransparency(true)
	a.ShowAppearance = true
	saved := 0
	a.Act.PersistAppearance = func() { saved++ }
	tt := shellTester(t, a)
	for _, label := range []string{"透明度 40%", "调整背景透明度"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %s", label)
		}
		tt.ClickAt(r.X+r.W*.8, r.Y+r.H/2)
		tt.Frame()
	}
	if a.Theme.Transparency != .25 || saved != 0 {
		t.Fatalf("disabled controls changed preference: value=%v saves=%d rt=%v", a.Theme.Transparency, saved, a.Theme.ReducedTransparency)
	}
	if err := tt.Click("浅色主题"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Theme.Mode != "light" || a.Theme.backgroundOpacity() != 1 {
		t.Fatal("appearance choice must still work with reduced transparency")
	}
}

func TestAppearanceDarkEdgesStayInsideGlass(t *testing.T) {
	for _, size := range []struct {
		width, height int
		radius        float32
	}{{42, 42, 21}, {520, 46, 13}, {520, 300, 18}} {
		th := ThemeFor(PeriodAt(12)).withAppearance("dark", .25)
		bg := ui.Hex("#101d31")
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(th.uiTheme())
			c.Root().Background(bg)
			ui.Box(c).Absolute().Left(12).Top(12).Size(float32(size.width), float32(size.height)).
				Draw(func(p *ui.Painter, r ui.Rect) { paintPlayerRim(p, r, size.radius, .3) })
		}, size.width+24, size.height+24)
		frame := tt.Image()
		for x := 12; x < 12+size.width; x++ {
			pixel := frame.RGBAAt(x, 12+size.height)
			if pixel.R != bg.R || pixel.G != bg.G || pixel.B != bg.B {
				t.Fatalf("%dx%d: bright fringe escaped the bottom curve at x=%d: %v", size.width, size.height, x, pixel)
			}
		}
		top := frame.RGBAAt(12+size.width/2, 12)
		bottom := frame.RGBAAt(12+size.width/2, 11+size.height)
		if int(top.R)-int(bottom.R) < 10 {
			t.Fatalf("glass reflection should fade toward the bottom: %v / %v", top, bottom)
		}
	}
}

// TestAppearanceTriggerDoesNotShiftWhenPressed 钉住封面上的外观键按下时
// 几何不变：此前它用 b.Scale(.97,.97) 做按压缩放，而旁边三颗封面开关
// 只做底色过渡；缩放还在亚像素落点上让图标看起来“往下跳一下”。
func TestAppearanceTriggerDoesNotShiftWhenPressed(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tt := shellTester(t, a)
	rest, ok := tt.Find("外观")
	if !ok {
		t.Fatal("missing appearance trigger")
	}
	tt.Move(rest.X+rest.W/2, rest.Y+rest.H/2)
	tt.Advance(300 * time.Millisecond)
	tt.Press(rest.X+rest.W/2, rest.Y+rest.H/2)
	tt.Advance(100 * time.Millisecond)
	pressed, _ := tt.Find("外观")
	if abs32(pressed.X-rest.X) > .01 || abs32(pressed.Y-rest.Y) > .01 ||
		abs32(pressed.W-rest.W) > .01 || abs32(pressed.H-rest.H) > .01 {
		t.Fatalf("外观键按下时几何变了：rest=%+v pressed=%+v", rest, pressed)
	}
	tt.Release(rest.X+rest.W/2, rest.Y+rest.H/2)
	tt.Advance(100 * time.Millisecond)
	if !a.ShowAppearance {
		t.Fatal("appearance trigger no longer opens the popover")
	}
}

// TestAppearanceAutoPeriods 钉住自动模式的时间表：与播放栏烟熏玻璃共用
// darkPeriods，18:00–06:00 深色，06:00–18:00 浅色。
func TestAppearanceAutoPeriods(t *testing.T) {
	for _, tc := range []struct {
		hour int
		want string
	}{
		{0, "dark"}, {5, "dark"}, {6, "light"}, {7, "light"},
		{12, "light"}, {17, "light"}, {18, "dark"}, {21, "dark"}, {23, "dark"},
	} {
		got := ThemeFor(PeriodAt(tc.hour)).withAppearance("auto", .2).Mode
		if got != tc.want {
			t.Errorf("%02d:00 auto mode = %s, want %s", tc.hour, got, tc.want)
		}
	}
}

// TestAppearanceAutoPreserved 钉住 RefreshTheme / 减少透明度重推时不会把
// "auto" 偏好丢掉，并且每次按新的时段重新解析。
func TestAppearanceAutoPreserved(t *testing.T) {
	a := NewApp(func() {})
	a.SetAppearance("auto", .25)
	a.RefreshTheme()
	want := appearanceForPeriod(PeriodAt(time.Now().Hour()))
	if a.AppearancePref() != "auto" || a.Theme.Mode != want || a.Theme.Transparency != .25 {
		t.Fatalf("auto lost on refresh: pref=%s mode=%s transparency=%v", a.AppearancePref(), a.Theme.Mode, a.Theme.Transparency)
	}
	a.SetReduceTransparency(true)
	want = appearanceForPeriod(PeriodAt(time.Now().Hour()))
	if a.AppearancePref() != "auto" || a.Theme.Mode != want || a.Theme.Transparency != .25 {
		t.Fatalf("auto lost on reduce-transparency: pref=%s mode=%s transparency=%v", a.AppearancePref(), a.Theme.Mode, a.Theme.Transparency)
	}
}

// TestAppearanceAutoControl 走真实面板：点「自动主题」后偏好是 auto、
// 解析到当前时段；再点「浅色主题」能回到显式浅色。
func TestAppearanceAutoControl(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(21)) // evening → auto dark
	saved := 0
	a.Act.PersistAppearance = func() { saved++ }
	tt := shellTester(t, a)
	trigger, ok := tt.Find("外观")
	if !ok {
		t.Fatal("missing appearance trigger")
	}
	tt.Move(trigger.X+trigger.W/2, trigger.Y+trigger.H/2)
	tt.Advance(250 * time.Millisecond)
	if err := tt.Click("外观"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("自动主题"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.AppearancePref() != "auto" || a.Theme.Mode != "dark" || saved != 1 {
		t.Fatalf("auto selection: pref=%s mode=%s saved=%d", a.AppearancePref(), a.Theme.Mode, saved)
	}
	if _, ok := tt.Find("自动 · 当前深色（18:00–06:00）· 拖动调整"); !ok {
		t.Fatalf("auto hint missing: %v", tt.Texts())
	}
	if err := tt.Click("浅色主题"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.AppearancePref() != "light" || a.Theme.Mode != "light" || saved != 2 {
		t.Fatalf("explicit light after auto: pref=%s mode=%s saved=%d", a.AppearancePref(), a.Theme.Mode, saved)
	}
}

// TestAppearancePopoverHeightStableForAuto 钉住弹层高度不随「自动」选择变化：
// 自动提示替换同一条底部提示，不再额外插一行（用户报设置窗口一会儿高一会儿低）。
func TestAppearancePopoverHeightStableForAuto(t *testing.T) {
	height := func(mode string) float32 {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(21)) // 固定 evening，auto 解析为 dark
		a.SetAppearance(mode, .25)
		a.ShowAppearance = true
		tt := shellTesterSize(t, a, 900, 620)
		r, ok := tt.Find("外观设置")
		if !ok {
			t.Fatalf("%s: missing appearance panel", mode)
		}
		return r.H
	}
	light, dark, auto := height("light"), height("dark"), height("auto")
	if abs32(light-dark) > .5 || abs32(light-auto) > .5 {
		t.Fatalf("popover height changes with mode: light=%v dark=%v auto=%v", light, dark, auto)
	}
}
