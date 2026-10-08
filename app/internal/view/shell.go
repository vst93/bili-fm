package view

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// Shell 画出整个主窗口。
func (a *App) Shell(c *ui.Context) {
	if a.Mini {
		a.MiniShell(c)
		return
	}
	t := a.Theme
	a.shortcuts(c)
	c.Root().Background(t.Period.Sample(0.5))

	// 背景：时段渐变（ui.LinearGradient 只有两个颜色，按条带叠出多段）。
	ui.Column(c).Fill().Children(func() { a.backdrop(c) })

	// 前景浮在背景之上。
	ui.Column(c).Absolute().Fill().Children(func() {
		a.titleBar(c)
		a.sectionTabs(c)
		a.content(c)
		a.playerBar(c)
		if a.ShowParts {
			a.partsPanel(c)
		}
		if a.ShowDanmaku {
			a.danmakuPanel(c)
		}
		if a.ShowInfo {
			a.infoPanel(c)
		}
	})
}

// backdrop 叠出时段背景渐变。
func (a *App) backdrop(c *ui.Context) {
	const bands = 12
	for _, b := range a.Theme.Period.BackdropBands(bands) {
		ui.Box(c).FillWidth().Grow(1).LinearGradient(ui.LinearGradient{
			From: b[0], To: b[1], Angle: 180,
		})
	}
}

// titleBar 是自绘标题栏：品牌、状态、搜索框、窗口按钮。
func (a *App) titleBar(c *ui.Context) {
	t := a.Theme
	ui.Row(c).FillWidth().Height(TitleBarHeight).Shrink(0).
		AlignItems(ui.Center).Padding(0, 8, 0, 12).Gap(10).DragWindow().Children(func() {
		ui.Text(c, "bili-FM").FontSize(13).Bold().TextColor(t.Ink)
		ui.Text(c, a.statusLine()).FontSize(11).TextColor(t.Faint).SingleLine().MaxWidth(200)
		ui.Box(c).Grow(1)
		if f := ui.SearchField(c, &a.Query).Width(260).Label("搜索视频"); f.Submitted() {
			if a.Act.Search != nil {
				a.Act.Search(a.Query)
			}
		}
		ui.Box(c).Grow(1)
		a.windowButtons(c)
	})
}

func (a *App) statusLine() string {
	if a.Loading {
		return "加载中…"
	}
	if a.Status != "" {
		return a.Status
	}
	if a.UName != "" {
		return "已登录：" + a.UName
	}
	return ""
}

// windowButtons 是自绘的最小化 / 关闭（无边框窗口）。
func (a *App) windowButtons(c *ui.Context) {
	t := a.Theme
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		min := ui.ButtonBase(c.Key("win-min")).Size(28, 22).Radius(RadiusSmall).Center().
			Label("最小化").Tooltip("最小化")
		if min.Hovered() {
			min.Background(t.GlassHover)
		} else {
			min.Background(t.Glass)
		}
		min.Children(func() { ui.Icon(c, iconMin).Size(14, 14).TextColor(t.Muted) })
		if min.Clicked() && a.Act.Minimize != nil {
			a.Act.Minimize()
		}

		closeB := ui.ButtonBase(c.Key("win-close")).Size(28, 22).Radius(RadiusSmall).Center().
			Label("关闭").Tooltip("关闭")
		if closeB.Hovered() {
			closeB.Background(t.Rose.Alpha(0.28))
		} else {
			closeB.Background(t.Glass)
		}
		closeB.Children(func() { ui.Icon(c, iconClose).Size(13, 13).TextColor(t.Muted) })
		if closeB.Clicked() && a.Act.Quit != nil {
			a.Act.Quit()
		}
	})
}

// sectionTabs 是分区切换。
func (a *App) sectionTabs(c *ui.Context) {
	t := a.Theme
	ui.Row(c).FillWidth().Shrink(0).Padding(2, 14, 8, 14).Gap(6).AlignItems(ui.Center).Children(func() {
		for i, s := range Sections {
			active := i == a.Section
			b := ui.ButtonBase(c.Key("sec-"+s.Key)).Height(26).Padding(0, 12).
				Radius(RadiusPill).Center().Label(s.Label)
			switch {
			case active:
				b.Background(t.GlassHover).Border(1, t.GlassBorder)
			case b.Hovered():
				b.Background(t.GlassHover)
			default:
				b.Background(t.Glass)
			}
			b.Children(func() {
				ui.Text(c, s.Label).FontSize(12).TextColor(pick(active, t.Ink, t.Muted))
			})
			if b.Clicked() && i != a.Section {
				a.Section = i
				a.Page = 1
				a.Cards = nil
				if a.Act.LoadSection != nil {
					a.Act.LoadSection(s.Key, 1)
				}
			}
		}
		ui.Box(c).Grow(1)
		if a.LoggedIn {
			if b := ui.ButtonBase(c.Key("logout-hint")).Height(26).Padding(0, 10).Radius(RadiusPill).
				Center().Label("已登录"); b.Hovered() {
				b.Background(t.GlassHover)
			}
		} else if b := ui.ButtonBase(c.Key("login")).Height(26).Padding(0, 12).Radius(RadiusPill).
			Center().Label("扫码登录"); b.Hovered() {
			b.Background(t.GlassHover)
		} else {
			b.Background(t.Glass)
			if b.Clicked() && a.Act.Login != nil {
				a.Act.Login()
			}
		}
	})
}

// content 是中间的内容区：卡片网格 + 空态 / 加载态 / 加载更多。
func (a *App) content(c *ui.Context) {
	t := a.Theme
	ui.Column(c).Grow(1).Padding(4, 14, 4, 14).Children(func() {
		switch {
		case a.Loading && len(a.Cards) == 0:
			ui.Text(c, "加载中…").FontSize(12).TextColor(t.Faint)
			return
		case len(a.Cards) == 0:
			ui.Text(c, a.emptyHint()).FontSize(12).TextColor(t.Faint)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				a.cardGrid(c)
				if a.HasMore {
					more := ui.ButtonBase(c.Key("load-more")).FillWidth().Height(32).Radius(Radius).
						Center().Label("加载更多")
					if more.Hovered() {
						more.Background(t.GlassHover)
					} else {
						more.Background(t.Glass)
					}
					more.Children(func() {
						ui.Text(c, pick(a.Loading, "加载中…", "加载更多")).FontSize(12).TextColor(t.Muted)
					})
					if more.Clicked() && !a.Loading && a.Act.LoadMore != nil {
						a.Act.LoadMore()
					}
				}
			})
		})
	})
}

func (a *App) emptyHint() string {
	if a.CurrentSection().NeedLogin && !a.LoggedIn {
		return "这个分区需要登录，点右上角「扫码登录」"
	}
	return "点上面的分区加载内容，或用搜索框找视频"
}

// cardGrid 按固定列数排卡片。
func (a *App) cardGrid(c *ui.Context) {
	const cols = 4
	for i := 0; i < len(a.Cards); i += cols {
		end := min(i+cols, len(a.Cards))
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
			for j := i; j < end; j++ {
				a.card(c, j)
			}
			for j := end; j < i+cols; j++ {
				ui.Box(c).Grow(1).Basis(0)
			}
		})
	}
}

// card 是一张列表卡片：封面 + 时长角标 + 标题 + meta 行。
func (a *App) card(c *ui.Context, index int) {
	t := a.Theme
	card := a.Cards[index]

	playing := a.Current() != nil && card.Bvid != "" && a.Current().Bvid == card.Bvid
	box := ui.Column(c).Key(fmt.Sprintf("card-%d-%s", index, card.Bvid)).Grow(1).Basis(0).
		Padding(6).Radius(Radius).Gap(6).Cursor(ui.CursorPointer)
	switch {
	case playing:
		box.Background(t.GlassHover).Border(1, t.Blue.Alpha(0.65))
	case box.Hovered():
		box.Background(t.GlassHover)
	default:
		box.Background(t.Glass)
	}

	box.Children(func() {
		ui.Box(c).FillWidth().AspectRatio(16.0 / 9.0).Radius(RadiusSmall).
			Background(t.CoverPlaceholder()).Clip().Children(func() {
			if bmp := a.Images.Bitmap(card.Cover); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			}
			if card.Duration != "" {
				ui.Text(c, card.Duration).FontSize(10).TextColor(ui.Hex("#ffffff")).
					Absolute().Right(4).Bottom(3).
					Padding(1, 4).Radius(3).Background(ui.Hex("#0f172a").Alpha(0.62))
			}
			if playing {
				ui.Text(c, "正在播放").FontSize(9).TextColor(ui.Hex("#ffffff")).
					Absolute().Left(4).Top(4).
					Padding(1, 5).Radius(3).Background(t.Blue.Alpha(0.85))
			}
		})
		ui.Text(c, card.Title).FontSize(12).TextColor(t.Ink).MaxLines(2)
		if len(card.Meta) > 0 {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				for k, m := range card.Meta {
					if m == "" {
						continue
					}
					if k > 0 {
						ui.Text(c, "·").FontSize(10).TextColor(t.Faint)
					}
					ui.Text(c, m).FontSize(10).TextColor(t.Faint).SingleLine()
				}
			})
		}
	})
	if box.Clicked() {
		a.Index = index
		if a.Act.Play != nil {
			a.Act.Play(index)
		}
	}
}

// CoverPlaceholder 是封面未加载时的底色。
func (t Theme) CoverPlaceholder() ui.Color {
	if t.Dark {
		return ui.Hex("#ffffff").Alpha(0.08)
	}
	return ui.Hex("#0f172a").Alpha(0.08)
}

func pick[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
