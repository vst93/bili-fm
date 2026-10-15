package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestRefreshKeyStateFor 钉住抽屉表头刷新键的两条加载路径（原版是两种不同的
// 按钮实现）：
//
//	history  <Button isDisabled={isRefreshing}> + Refresh（图标 .history-refresh-spinning）
//	danmaku  <Button isLoading={isLoadingDanmaku || isLoadingReply}>
//
// 两者的加载期都必须禁用；弹幕抽屉的加载态画 Spinner（HeroUI 的 isIconOnly
// 分支在 isLoading 时丢掉 children），不是自转齿轮。标签：历史抽屉按 tab 变，
// 动态 / 热门推荐 / 收藏三个列表各按原版 aria-label（见 refreshLabelFor）。
func TestRefreshKeyStateFor(t *testing.T) {
	cases := []struct {
		name                           string
		drawer, histTab                string
		histLoading, dmLoading, rpLoad bool
		wantLabel                      string
		wantSpin, wantSpinner          bool
	}{
		{"历史·加载中", "history", HistHistory, true, false, false, "刷新历史记录", true, false},
		{"历史·稍后再看·加载中", "history", HistWatchLater, true, false, false, "刷新稍后再看", true, false},
		{"历史·常态", "history", HistHistory, false, false, false, "刷新历史记录", false, false},
		{"弹幕·拉取中", DrawerDanmaku, "", false, true, false, "刷新", false, true},
		{"弹幕·评论在途", DrawerDanmaku, "", false, false, true, "刷新", false, true},
		{"弹幕·常态", DrawerDanmaku, "", false, false, false, "刷新", false, false},
		{"动态列表", "feed", "", false, false, false, "刷新动态列表", false, false},
		{"热门推荐", "popular", "", false, false, false, "刷新当前列表", false, false},
		{"收藏列表", "favorite", "", false, false, false, "刷新收藏列表", false, false},
		// 其它抽屉（UP 主空间 / 未知名）没有加载态：即便这些标志被误置也不该
		// 影响（历史抽屉的 Loading 只对 "history" 生效，弹幕的两个标志只对弹幕
		// 抽屉生效）。
		{"其它抽屉", DrawerUp, "", true, true, true, "刷新", false, false},
	}
	for _, c := range cases {
		got := refreshKeyStateFor(c.drawer, c.histTab, c.histLoading, c.dmLoading, c.rpLoad)
		if got.Label != c.wantLabel || got.Spin != c.wantSpin || got.Spinner != c.wantSpinner {
			t.Errorf("%s: got %+v，想 label=%q spin=%v spinner=%v",
				c.name, got, c.wantLabel, c.wantSpin, c.wantSpinner)
		}
		if wantOff := c.wantSpin || c.wantSpinner; got.Off() != wantOff {
			t.Errorf("%s: Off()=%v，想 %v —— 加载形态必须禁用", c.name, got.Off(), wantOff)
		}
	}
}

// TestDrawerRefreshKeyLabelsWired 无头整窗实测：动态 / 热门推荐 / 收藏三个
// 列表抽屉的表头刷新键真的带上了原版各自的文案（能被按名字找到），而不是
// 一律「刷新」。纯函数 refreshLabelFor 的用例见 TestRefreshKeyStateFor；这条
// 证明文案确实接进了按钮的 Label/Tooltip。
//
// 反证：去掉 refreshLabelFor 的 feed/popular/favorite 分支（回到通用「刷新」）后，
// 前三个子用例都找不到各自文案，随即变红。
func TestDrawerRefreshKeyLabelsWired(t *testing.T) {
	cases := []struct {
		drawer string
		label  string
	}{
		{"feed", "刷新动态列表"},
		{"popular", "刷新当前列表"},
		{"favorite", "刷新收藏列表"},
		{DrawerUp, "刷新"}, // 原版 upVideoList 的刷新键没有 aria-label，仍是通用文案。
	}
	for _, c := range cases {
		t.Run(c.drawer, func(t *testing.T) {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.Drawer = c.drawer
			tt := shellTester(t, a)
			tt.Frame()
			if _, ok := tt.Find(c.label); !ok {
				t.Fatalf("抽屉 %q 的表头刷新键找不到文案 %q", c.drawer, c.label)
			}
			if c.label != "刷新" {
				if _, ok := tt.Find("刷新"); ok {
					t.Errorf("抽屉 %q 仍存在通用「刷新」文案，应只保留 %q", c.drawer, c.label)
				}
			}
		})
	}
}

// TestDrawerRefreshKeyDisabled 无头整窗真点刷新键：弹幕拉取中 / 评论在途 /
// 历史刷新中，点不动（Act.Reload 不触发）；弹幕常态点一次触发一次。
//
// 原版三种加载态都由 HeroUI <Button> 的 isDisabled / isLoading 把按钮禁掉，
// 此前原生版弹幕/评论刷新键完全没有加载态与禁用，连点会各起一次请求。
func TestDrawerRefreshKeyDisabled(t *testing.T) {
	render := func(t *testing.T, setup func(a *App)) int {
		t.Helper()
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		setup(a)
		n := 0
		a.Act.Reload = func() { n++ }
		tt := shellTester(t, a)
		tt.Frame()
		label := "刷新"
		if a.Drawer == "history" {
			label = historyRefreshLabel(a.HistTab)
		}
		if err := tt.Click(label); err != nil {
			t.Fatalf("找不到刷新键 %q：%v", label, err)
		}
		tt.Frame()
		return n
	}

	cases := []struct {
		name  string
		setup func(a *App)
		want  int
	}{
		{"弹幕拉取在途→禁用", func(a *App) {
			a.Drawer = DrawerDanmaku
			a.DanmakuTab = TabDanmaku
			a.DanmakuLoading = true
		}, 0},
		{"评论在途→禁用", func(a *App) {
			a.Drawer = DrawerDanmaku
			a.DanmakuTab = TabReply
			a.RepliesLoading = true
		}, 0},
		{"历史刷新中→禁用", func(a *App) {
			a.Drawer = "history"
			a.HistTab = HistHistory
			a.ListFor("history").Loading = true
		}, 0},
		{"弹幕常态→可点", func(a *App) {
			a.Drawer = DrawerDanmaku
			a.DanmakuTab = TabDanmaku
		}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := render(t, c.setup); got != c.want {
				t.Errorf("刷新键点击触发 Reload %d 次，想 %d", got, c.want)
			}
		})
	}
}

// TestDanmakuRefreshKeySpinnerRenders 无头实测：弹幕抽屉刷新键在加载态下
// 真的用 Spinner 替掉了刷新齿轮（原版 HeroUI isIconOnly + isLoading 的渲染），
// 而不是只把图标压暗。Spinner 带着 Label「加载中」，常态下没有这个名字
// 的进度指示器。
//
// 反证：去掉 headerButtonOff 的 spinner 分支（回到只画图标）后，加载态就
// 找不到「加载中」，这条会变红。
func TestDanmakuRefreshKeySpinnerRenders(t *testing.T) {
	render := func(loading bool) (*ui.Tester, ui.Rect) {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Drawer = DrawerDanmaku
		a.DanmakuTab = TabDanmaku
		a.DanmakuLoading = loading
		tt := shellTester(t, a)
		tt.Frame()
		r, ok := tt.Find("刷新")
		if !ok {
			t.Fatal("找不到弹幕抽屉的刷新键")
		}
		return tt, r
	}

	loadTT, btn := render(true)
	spinner, ok := loadTT.Find("加载中")
	if !ok {
		t.Fatal("弹幕拉取中时刷新键里没有 Spinner（找不到「加载中」）")
	}
	if spinner.X < btn.X || spinner.X+spinner.W > btn.X+btn.W ||
		spinner.Y < btn.Y || spinner.Y+spinner.H > btn.Y+btn.H {
		t.Errorf("Spinner %+v 不在刷新键 %+v 里", spinner, btn)
	}

	idleTT, _ := render(false)
	if _, ok := idleTT.Find("加载中"); ok {
		t.Error("常态下的刷新键里不该有 Spinner（「加载中」不该出现）")
	}
}

// TestFrameworkDisabledOpacityIsHalf 钉住 headerButtonOff 依赖的框架契约：
// 禁用节点整棵子树的不透明度再乘 .5（internal 的 paint 里 `p.opacity *= .5`）。
// HeroUI <Button> 的 disabled 不透明度是 token opacity-disabled = .5，所以
// 抽屉表头刷新键（HeroUI Button）禁用时**不写**元素不透明度，框架正好给到 .5；
// 不能像 `.nav-icon-btn:disabled` 那样按 .34 近似（那样会暗掉一截）。
func TestFrameworkDisabledOpacityIsHalf(t *testing.T) {
	ink := ui.Hex("#333333")
	paint := func(opts func(b ui.Element)) *image.RGBA {
		return ui.Render(func(c *ui.Context) {
			c.Root().Background(ui.Hex("#ffffff"))
			b := ui.Box(c).Absolute().Left(4).Top(4).Size(32, 32).Center()
			opts(b)
			b.Children(func() { ui.Icon(c, iconRefresh).Size(20, 20).TextColor(ink) })
		}, 40, 40, 1)
	}
	disabled := paint(func(b ui.Element) { b.Disabled(true) })
	half := paint(func(b ui.Element) { b.Opacity(0.5) })
	third := paint(func(b ui.Element) { b.Opacity(0.34) })
	r := ui.Rect{X: 4, Y: 4, W: 32, H: 32}
	if d, _ := diffInRect(disabled, half, r); d != 0 {
		t.Errorf("禁用渲染与显式 .5 不透明度有 %d 像素差异，框架的禁用系数不是 .5？", d)
	}
	if d, _ := diffInRect(disabled, third, r); d == 0 {
		t.Errorf("禁用渲染与显式 .34 逐像素相同：无法区分 .5 与 .34，测试失去意义")
	}
}
