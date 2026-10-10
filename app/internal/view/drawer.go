package view

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// 抽屉（原版是 HeroUI 的 Drawer，样式在 globals.css 里被整体覆写）：
//
//	覆盖层  div:has(> [data-slot="wrapper"]) > div:first-child
//	        rgba(15, 23, 42, 0.28)，不模糊
//	面板    [data-slot="wrapper"] > section
//	        底部对齐、上圆角 18、上边框 1px --glass-border、
//	        max-height min(92vh, 100vh - 54px)、投影 0 -16px 50px
//	头部    section > header
//	        高 48、右侧留 44 给绝对定位的关闭键、下边框 1px rgba(100,116,139,0.14)
//
// 同一时刻只有一个抽屉打开（App.Drawer）。
func (a *App) drawer(c *ui.Context) {
	if a.Drawer == "" {
		return
	}
	_, winH := c.Size()
	// min(92vh, 100vh - 54px)：54 是播放栏高度，抽屉不盖住它。
	h := min(winH*0.92, winH-54)

	overlay := ui.Box(c).Absolute().Fill().Background(ui.Hex("#0f172a").Alpha(0.28))
	overlay.Children(func() {
		// 原版抽屉的 ::before 有 backdrop-filter: blur(14px) saturate(1.35)
		// （--liquid-blur-thin：小面积用薄玻璃），所以面板本身也是真玻璃。
		panel := ui.Column(c).Absolute().Left(0).Right(0).Bottom(0).Height(h).
			Radius(18, 18, 0, 0).
			Material(glass.Glass{}).
			Shadow(0, -16, 50, 0, shadowInk.Alpha(0.08))
		panel.Children(func() {
			a.drawerHeader(c)
			a.drawerBody(c)
		})
	})
	// 点覆盖层空白处关闭（面板在它上面，点面板不会关）。
	if overlay.Clicked() {
		a.Drawer = ""
	}
}

// drawerHeader 是抽屉头部（`[data-slot="wrapper"] > section > header`）：
// 高 48、右侧留 44 给绝对定位的关闭键、下边框 1px rgba(100,116,139,0.14)。
// 表头里放：抽屉自己的 tab（或标题）+ 刷新键。
func (a *App) drawerHeader(c *ui.Context) {
	t := a.Theme
	ui.Row(c).FillWidth().Height(48).Shrink(0).Padding(0, 44, 0, 16).
		Gap(8).AlignItems(ui.Center).Children(func() {
		a.drawerTabs(c)
		// 收藏抽屉的收藏夹 tab 自己占满剩余宽度（可横向滚动），不再放弹性占位。
		if a.Drawer != "favorite" {
			ui.Box(c).Grow(1)
		}
		// 原版只有搜索抽屉不显示条数（排序 tab 占满了表头）。
		if a.Drawer == DrawerSearch {
			ui.Text(c, fmt.Sprintf("%d 条", len(a.list().Cards))).FontSize(11).TextColor(t.Faint)
		}
		if a.refreshable() {
			a.headerButton(c, "drawer-refresh", "刷新", iconRefresh, func() {
				if a.Act.Reload != nil {
					a.Act.Reload()
				}
			})
		}
	})
	// 分割线：灰蓝暗线 + 下方 1px 白高光，模拟玻璃截面（原版 header 的
	// border-bottom + box-shadow 两层）。
	ui.Box(c).FillWidth().Height(2).Shrink(0).Children(func() {
		ui.Box(c).FillWidth().Height(1).Shrink(0).Background(ui.Hex("#64748b").Alpha(0.14))
		ui.Box(c).FillWidth().Height(1).Shrink(0).Background(ui.Hex("#ffffff").Alpha(0.40))
	})

	// 关闭键浮在右上角（原版是绝对定位的 close 按钮）。
	ui.Box(c).Absolute().Right(10).Top(8).Children(func() {
		b := ui.ButtonBase(c.Key("drawer-close")).Size(32, 32).Radius(ToolButtonR).Center().
			Label("关闭").Tooltip("关闭")
		if b.Hovered() {
			b.Background(t.GlassHover)
		} else {
			b.Background(ui.Transparent)
		}
		b.Children(func() { ui.Icon(c, iconClose).Size(14, 14).TextColor(t.Muted) })
		if b.Clicked() {
			a.Drawer = ""
		}
	})
}

// drawerTabs 画抽屉表头左边的部分：列表抽屉是 tab，其余是一个标题。
func (a *App) drawerTabs(c *ui.Context) {
	t := a.Theme
	switch a.Drawer {
	case DrawerSearch:
		a.tabs(c, "sort", []tabItem{
			{Key: SortTotal, Label: "综合"},
			{Key: SortClick, Label: "最多播放"},
			{Key: SortUpdate, Label: "最新发布"},
		}, a.SortOrder, func(k string) {
			a.SortOrder = k
			a.reload()
		})
	case "popular":
		a.tabs(c, "rec", []tabItem{
			{Key: RecHot, Label: "热门"},
			{Key: RecRecommend, Label: "推荐"},
		}, a.RecTab, func(k string) {
			a.RecTab = k
			a.reload()
		})
	case "history":
		a.tabs(c, "hist", []tabItem{
			{Key: HistHistory, Label: "观看历史"},
			{Key: HistWatchLater, Label: "稍后再看"},
		}, a.HistTab, func(k string) {
			a.HistTab = k
			a.reload()
		})
		// 隐身开关只在「观看历史」这一页（原版如此）。
		if a.HistTab == HistHistory {
			a.incognitoSwitch(c)
		}
	case "favorite":
		ui.Text(c, "收藏").FontSize(14).Bold().TextColor(ui.Hex("#334155")).Shrink(0)
		if len(a.Folders) == 0 {
			ui.Text(c, "还没有收藏夹").FontSize(12).TextColor(t.Faint)
			break
		}
		// 收藏夹可能很多：横向滚动，不把后面的挤掉。
		ui.ScrollHorizontal(c).Grow(1).MinWidth(0).Height(32).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				for _, f := range a.Folders {
					a.folderTab(c, f)
				}
			})
		})
	case "feed":
		ui.Text(c, "动态列表").FontSize(14).Bold().TextColor(ui.Hex("#334155"))
	case DrawerUp:
		a.upHeader(c)
	case DrawerParts:
		a.partsHeader(c)
	case DrawerDanmaku:
		a.danmakuTabs(c)
	case DrawerPlaylist:
		a.playlistTabs(c)
	case DrawerSeries:
		ui.Text(c, a.seriesLabel()).FontSize(14).Bold().SingleLine().
			TextColor(ui.Hex("#334155")).Shrink(0)
		// 旧版的「播放全部」：把整个合集当队列从第一条开始放。
		if b := ui.ButtonBase(c.Key("series-play-all")).Height(32).Padding(0, 12).
			Radius(Radius).Center().Label("播放全部").Tooltip("播放合集中的全部视频"); true {
			if b.Hovered() {
				b.Background(t.GlassHover)
			} else {
				b.Background(t.GlassActive)
			}
			b.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconPlay).Size(12, 12).TextColor(ui.Hex("#475569"))
					ui.Text(c, "播放全部").FontSize(12).TextColor(ui.Hex("#475569"))
				})
			})
			if b.Clicked() && a.Act.SeriesPlayAll != nil {
				a.Act.SeriesPlayAll()
			}
		}
	default:
		ui.Text(c, a.drawerTitle()).FontSize(14).Bold().TextColor(ui.Hex("#334155"))
	}
}

// upHeader 是 UP 空间抽屉的表头（原版 upVideoList.tsx）：
// 「{名字}」的空间 + 关注 / 已关注 + 视频 / 合集 tab。
func (a *App) upHeader(c *ui.Context) {
	t := a.Theme
	ui.Text(c, fmt.Sprintf("「%s」的空间", a.UpName)).FontSize(14).Bold().
		SingleLine().MaxWidth(200).TextColor(ui.Hex("#334155")).Shrink(0)
	if a.UpFans > 0 {
		ui.Text(c, "粉丝 "+compactCount(a.UpFans)).FontSize(12).TextColor(t.Faint).Shrink(0)
	}

	label := pick(a.UpFollowed, "已关注", "关注")
	b := ui.ButtonBase(c.Key("up-follow")).Height(32).Padding(0, 12).
		Radius(Radius).Center().Label(label)
	if b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(t.GlassActive)
	}
	b.Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, pick(a.UpFollowed, iconFollowed, iconFollow)).Size(12, 12).
				TextColor(pick(a.UpFollowed, t.Muted, t.Blue))
			ui.Text(c, label).FontSize(12).
				TextColor(pick(a.UpFollowed, t.Muted, t.Blue))
		})
	})
	if b.Clicked() && a.Act.ToggleFollow != nil {
		a.Act.ToggleFollow()
	}

	a.tabs(c, "up", []tabItem{
		{Key: UpTabVideos, Label: "视频"},
		{Key: UpTabSeries, Label: "合集"},
	}, a.UpTab, func(k string) {
		a.UpTab = k
		a.reload()
	})
}

// incognitoSwitch 是历史抽屉表头里的「隐身」开关（原版 .history-incognito-switch）：
// 32 高、图标 + 文字 + 一个小轨道。打开后不读也不写云端进度。
func (a *App) incognitoSwitch(c *ui.Context) {
	b := ui.ButtonBase(c.Key("incognito")).Height(32).Padding(0, 8).Radius(Radius).
		Label("隐身").Tooltip("开启后不读取或上报云端播放记录与进度")
	if a.Incognito {
		b.Background(ui.Hex("#cffafe").Alpha(0.5)).Border(1, ui.Hex("#0e7490").Alpha(0.34))
	} else if b.Hovered() {
		b.Background(ui.Hex("#ffffff").Alpha(0.5)).Border(1, ui.Hex("#64748b").Alpha(0.32))
	} else {
		b.Background(ui.Hex("#ffffff").Alpha(0.34)).Border(1, ui.Hex("#64748b").Alpha(0.2))
	}
	ink := pick(a.Incognito, ui.Hex("#0e7490"), ui.Hex("#64748b"))
	b.Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconMask).Size(15, 15).TextColor(ink)
			ui.Text(c, "隐身").FontSize(12).TextColor(ink)
			// 轨道 27×16，滑块 12，打开时右移 11。
			ui.Box(c).Width(27).Height(16).Shrink(0).Radius(RadiusPill).Padding(2).
				Background(pick(a.Incognito, ui.Hex("#0891b2"), ui.Hex("#64748b").Alpha(0.32))).
				Children(func() {
					ui.Box(c).Size(12, 12).Radius(RadiusPill).Background(ui.Hex("#ffffff")).
						Margin(0, 0, 0, pick(a.Incognito, float32(11), float32(0)))
				})
		})
	})
	if b.Clicked() && a.Act.ToggleIncognito != nil {
		a.Act.ToggleIncognito()
	}
}

// seriesLabel 是合集抽屉的标题（旧版显示「名字 (条数)」）。
func (a *App) seriesLabel() string {
	for _, s := range a.SeriesList {
		if s.ID == a.SeriesID {
			return fmt.Sprintf("%s (%d)", s.Title, s.Count)
		}
	}
	if a.SeriesName != "" {
		return a.SeriesName
	}
	return "合集"
}

// refreshable 返回这个抽屉的表头有没有刷新键（原版每个列表抽屉都有）。
func (a *App) refreshable() bool {
	switch a.Drawer {
	case DrawerParts, DrawerPlaylist:
		return false
	}
	return a.Drawer != ""
}

// reload 按当前抽屉的 tab / 排序重拉第一页（重置和拉取都在装配层做）。
func (a *App) reload() {
	if a.Act.Reload != nil {
		a.Act.Reload()
	}
}

// ---------------------------------------------------------------- 表头 tab

type tabItem struct {
	Key   string
	Label string
}

// tabs 是一排胶囊 tab（原版抽屉表头的 HeroUI Tabs）：
// 高 32、内边距 0 14、圆角 8、字号 13/600；选中态填充更实、文字变蓝。
func (a *App) tabs(c *ui.Context, group string, items []tabItem, selected string, onSelect func(string)) {
	ui.Row(c).Gap(6).Shrink(0).AlignItems(ui.Center).Children(func() {
		for _, it := range items {
			a.tab(c, group, it, it.Key == selected, onSelect)
		}
	})
}

func (a *App) tab(c *ui.Context, group string, it tabItem, active bool, onSelect func(string)) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("tab-"+group+"-"+it.Key)).Height(32).Padding(0, 14).
		Radius(Radius).Center().Label(it.Label)
	switch {
	case active:
		b.Background(pressFeedback(b, ui.Hex("#ffffff").Alpha(0.52), ui.Hex("#ffffff").Alpha(0.62), t.LiquidBgActive)).
			Border(1, t.GlassBorderBright)
	case b.Hovered():
		b.Background(t.LiquidBgHover).Border(1, t.GlassBorderBright)
	default:
		b.Background(t.LiquidBg).Border(1, t.GlassBorder)
	}
	b.Children(func() {
		ui.Text(c, it.Label).FontSize(13).Bold().
			TextColor(pick(active, ui.Hex("#0369a1"), ui.Hex("#475569")))
	})
	if b.Clicked() && !active {
		onSelect(it.Key)
	}
}

// folderTab 是一个收藏夹 tab（原版显示「标题 (条数)」）。
func (a *App) folderTab(c *ui.Context, f Folder) {
	t := a.Theme
	active := f.ID == a.FolderID
	label := fmt.Sprintf("%s (%d)", f.Title, f.Count)
	b := ui.ButtonBase(c.Key(fmt.Sprintf("folder-%d", f.ID))).Height(32).Padding(0, 14).
		Radius(Radius).Center().Label(label)
	switch {
	case active:
		b.Background(ui.Hex("#ffffff").Alpha(0.52)).Border(1, t.GlassBorderBright)
	case b.Hovered():
		b.Background(t.LiquidBgHover).Border(1, t.GlassBorderBright)
	default:
		b.Background(t.LiquidBg).Border(1, t.GlassBorder)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(13).Bold().SingleLine().MaxWidth(180).
			TextColor(pick(active, ui.Hex("#0369a1"), ui.Hex("#475569")))
	})
	if b.Clicked() && !active {
		a.FolderID = f.ID
		a.reload()
	}
}

// headerButton 是表头里的图标键（原版是 variant="light" 的小按钮）。
func (a *App) headerButton(c *ui.Context, key, label string, ic *ui.SVG, fn func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key(key)).Size(32, 32).Radius(Radius).Center().Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(t.GlassActive)
	}
	b.Children(func() { ui.Icon(c, ic).Size(17, 17).TextColor(ui.Hex("#475569")) })
	if b.Clicked() {
		fn()
	}
}

func (a *App) drawerTitle() string {
	switch a.Drawer {
	case DrawerSearch:
		return "搜索结果"
	case DrawerParts:
		return "选集"
	case DrawerDanmaku:
		return "弹幕"
	}
	if s := a.CurrentSection(); s.Key == a.Drawer {
		return s.Label
	}
	return "列表"
}

// drawerBody 按抽屉类型画内容。
//
// 弹幕抽屉自己用虚拟化列表当滚动容器（行数可能上千），所以不走外层的
// ui.Scroll；其余抽屉内容不多，统一套一层滚动。
func (a *App) drawerBody(c *ui.Context) {
	// 滚动到底自动翻页（原版各列表抽屉的 handleScroll：距底 ≤80px 加载下一页；
	// 弹幕/评论抽屉的评论 tab 是 <100px），不是「加载更多」按钮。
	//
	// 注意 TrackScroll 是双向绑定：容器每帧从这个状态读位置、滚动时写回。
	// 所以必须传**稳定地址**（List 常驻在 map 里），不能按值拷进出 ——
	// 拷贝会让容器每帧被旧位置拽回去，表现为触摸板/滚轮滚不动。
	l := a.ListFor(a.Drawer)
	if a.Drawer == DrawerDanmaku {
		ui.Scroll(c).FillWidth().Grow(1).Padding(8, 24).TrackScroll(&l.Scroll).
			Children(func() { a.danmakuBody(c) })
		if a.DanmakuTab == TabReply && a.RepliesHasMore && !a.RepliesLoading &&
			l.Scroll.MaxY > 0 && l.Scroll.Y >= l.Scroll.MaxY-100 && a.Act.LoadComments != nil {
			a.Act.LoadComments(a.ReplyPage + 1)
		}
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Padding(8, 24).TrackScroll(&l.Scroll).Children(func() {
		switch a.Drawer {
		case DrawerParts:
			a.partsBody(c)
		case DrawerUp:
			// UP 空间有两个 tab：视频是卡片列表，合集是一排可选的胶囊。
			if a.UpTab == UpTabSeries {
				a.seriesListBody(c)
			} else {
				a.listBody(c)
			}
		case DrawerPlaylist:
			a.playlistBody(c)
		default:
			a.listBody(c)
		}
	})

	// 到底附近就请求下一页（HasMore / 非加载中才发，避免重复）。
	if l.HasMore && !l.Loading && len(l.Cards) > 0 && l.Scroll.MaxY > 0 &&
		l.Scroll.Y >= l.Scroll.MaxY-80 && a.Act.LoadMore != nil {
		a.Act.LoadMore()
	}
}

// seriesListBody 是 UP 空间的「合集」tab（原版 upVideoList.tsx 的合集列表）：
// 一排下拉的合集胶囊，点一个就进那个合集。
func (a *App) seriesListBody(c *ui.Context) {
	t := a.Theme
	switch {
	case a.list().Loading && len(a.SeriesList) == 0:
		ui.Text(c, "加载中…").FontSize(12).TextColor(t.Faint)
		return
	case len(a.SeriesList) == 0:
		ui.Text(c, "这位 UP 主还没有合集").FontSize(12).TextColor(t.Faint)
		return
	}
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Start).Children(func() {
		for _, s := range a.SeriesList {
			a.seriesChip(c, s)
		}
	})
}

// seriesChip 是一个合集胶囊：封面缩略图 + 「名字 (条数)」。
func (a *App) seriesChip(c *ui.Context, s Series) {
	t := a.Theme
	active := s.ID == a.SeriesID
	b := ui.ButtonBase(c.Key(fmt.Sprintf("series-%d", s.ID))).Height(36).Padding(0, 14).
		Radius(Radius).Label(fmt.Sprintf("%s (%d)", s.Title, s.Count))
	switch {
	case active:
		b.Background(ui.Hex("#ffffff").Alpha(0.52)).Border(1, t.GlassBorderBright)
	case b.Hovered():
		b.Background(t.LiquidBgHover).Border(1, t.GlassBorderBright)
	default:
		b.Background(t.LiquidBg).Border(1, t.GlassBorder)
	}
	b.Children(func() {
		ui.Text(c, fmt.Sprintf("%s (%d)", s.Title, s.Count)).FontSize(13).Bold().
			SingleLine().MaxWidth(320).
			TextColor(pick(active, ui.Hex("#0369a1"), ui.Hex("#475569")))
	})
	if b.Clicked() {
		a.SeriesID = s.ID
		a.SeriesName = s.Title
		a.Drawer = DrawerSeries
		if a.Act.SelectSeries != nil {
			a.Act.SelectSeries(s.ID)
		}
	}
}

// ---------------------------------------------------------------- 列表抽屉

// listBody 是搜索结果 / 分区列表：三列卡片网格 + 加载更多。
// （原版抽屉里的网格是 grid-cols-2 sm:grid-cols-3 gap-2。）
func (a *App) listBody(c *ui.Context) {
	t := a.Theme
	switch {
	case a.list().Loading && len(a.list().Cards) == 0:
		a.skeletonGrid(c)
		return
	case len(a.list().Cards) == 0:
		ui.Text(c, a.emptyHint()).FontSize(12).TextColor(t.Faint)
		return
	}

	const cols = 3
	ui.Column(c).Gap(8).Children(func() {
		for i := 0; i < len(a.list().Cards); i += cols {
			end := min(i+cols, len(a.list().Cards))
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
				for j := i; j < end; j++ {
					a.card(c, j)
				}
				for j := end; j < i+cols; j++ {
					ui.Box(c).Grow(1).Basis(0)
				}
			})
		}
		// 加载更多改为滚动到底自动翻页（见 drawerBody），不再画按钮。
	})
}

// skeletonGrid 是列表加载中的骨架屏（原版 ListSkeleton）：灰底占位卡片。
func (a *App) skeletonGrid(c *ui.Context) {
	t := a.Theme
	placeholder := ui.Hex("#0f172a").Alpha(0.06)
	const cols = 3
	ui.Column(c).Gap(8).Children(func() {
		for row := 0; row < 3; row++ {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
				for j := 0; j < cols; j++ {
					ui.Column(c).Grow(1).Basis(0).Padding(6).Radius(Radius).
						Background(t.Glass).Gap(8).Children(func() {
						ui.Box(c).FillWidth().AspectRatio(16.0 / 9.0).Radius(RadiusSmall).
							Background(placeholder)
						ui.Box(c).FillWidth().Height(14).Radius(4).Background(placeholder)
						ui.Box(c).WidthPercent(60).Height(10).Radius(4).Background(placeholder)
					})
				}
			})
		}
	})
}

// emptyHint 是列表为空时的提示。需要登录的分区（以及「推荐」这种未登录必空的 tab）
// 提示去登录，而不是让用户看白屏。
func (a *App) emptyHint() string {
	if a.needsLogin() {
		return "这个分区需要登录，点右上角头像扫码登录"
	}
	return "没有内容"
}

// needsLogin 返回当前抽屉的内容是否因为未登录而拿不到。
func (a *App) needsLogin() bool {
	if a.LoggedIn {
		return false
	}
	switch a.Drawer {
	case DrawerSearch:
		return false
	case "popular":
		// 热门不需要登录；推荐接口未登录时返回空。
		return a.RecTab == RecRecommend
	case DrawerUp:
		// UP 主的视频列表走动态接口、合集列表接口都要登录，未登录时拿不到。
		return true
	}
	return a.CurrentSection().NeedLogin
}

// card 是一张列表卡片：封面 + 时长角标 + 标题 + meta 行（原版 ListCard）。
func (a *App) card(c *ui.Context, index int) {
	t := a.Theme
	card := a.list().Cards[index]

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
				ui.Text(c, card.Duration).FontSize(11).TextColor(ui.Hex("#f8fafc")).
					Absolute().Right(0).Bottom(0).
					Padding(2, 5).Radius(5, 0, 0, 0).Background(ui.Hex("#0f172a").Alpha(0.68))
			}
			if playing {
				ui.Text(c, "正在播放").FontSize(9).TextColor(ui.Hex("#ffffff")).
					Absolute().Left(4).Top(4).
					Padding(1, 5).Radius(3).Background(t.Blue.Alpha(0.85))
			}
			// 「稍后再看」列表的移除键（原版 historyList）。
			if a.Drawer == "history" && a.HistTab == HistWatchLater && card.Track.Aid != 0 {
				aid := card.Track.Aid
				// 请求中的置灰防连点（原版 pendingAids）。
				pending := a.WLRemovePending != nil && a.WLRemovePending[aid]
				b := ui.ButtonBase(c.Key("wl-remove-"+card.Bvid)).Size(22, 22).Radius(4).
					Center().Label("从稍后再看移除").Tooltip("从稍后再看移除")
				b.Disabled(pending)
				b.Absolute().Right(4).Top(4)
				switch {
				case pending:
					b.Background(ui.Hex("#0f172a").Alpha(0.30))
				case b.Hovered():
					b.Background(ui.Hex("#0f172a").Alpha(0.62))
				default:
					b.Background(ui.Hex("#0f172a").Alpha(0.42))
				}
				b.Children(func() {
					ui.Icon(c, iconClose).Size(12, 12).TextColor(ui.Hex("#ffffff"))
				})
				if b.Clicked() && !pending && a.Act.RemoveWatchLater != nil {
					a.Act.RemoveWatchLater(aid)
				}
			}
		})
		// 原版 ListCard 的标题是单行截断（line-clamp-1）。
		ui.Text(c, card.Title).FontSize(14).TextColor(t.Ink).SingleLine()
		a.metaRow(c, card)
	})
	if box.Clicked() {
		// 与原版一致：点卡片只是打开选集面板，不直接起播。
		if a.Act.OpenCard != nil {
			a.Act.OpenCard(index)
		}
	}
}

// metaRow 是卡片的 meta 行（旧版 CardMeta + globals.css 里的容器查询）。
//
// 三列比例：作者 40% / 播放量 30% / 发布时间 30%，列的间距进列自身的左内边距
// （旧版用 `+ .card-meta-field { padding-left: 8px }`，这样比例正好落在整行上）。
//
// 卡片窄了从末尾隐藏字段，阈值与旧版一致：
//
//	<= 260px 隐藏附加（进度等）
//	<= 240px 隐藏时长（时长在封面角标上，meta 里本来就不放）
//	<= 200px 隐藏发布时间
//	<= 150px 隐藏播放量
//
// 宽度取**上一帧的真实布局宽度**（Element.Bounds，旧版用 ResizeObserver）；
// 首帧还没有布局，按窗口宽度估一个。
func (a *App) metaRow(c *ui.Context, card Card) {
	t := a.Theme
	row := ui.Row(c).Key("meta-" + card.Bvid).FillWidth().AlignItems(ui.Center)
	w := row.Bounds().W
	if w <= 0 {
		w = estimateCardWidth(c)
	}

	type field struct {
		text  string
		ratio float32
	}
	fields := []field{{text: card.Up, ratio: 40}}
	if w > 150 {
		fields = append(fields, field{text: card.Views, ratio: 30})
	}
	if w > 200 {
		fields = append(fields, field{text: card.Pubdate, ratio: 30})
	}

	row.Children(func() {
		first := true
		for _, f := range fields {
			if f.text == "" {
				continue
			}
			col := ui.Box(c).BasisPercent(f.ratio).Shrink(1).MinWidth(0)
			if !first {
				col.Padding(0, 0, 0, 8)
			}
			first = false
			col.Children(func() {
				ui.Text(c, f.text).FontSize(12).TextColor(t.Faint).SingleLine()
			})
		}
		// 附加字段（稍后再看的进度）不参与三列比例，窄了直接不显示。
		if card.Extra != "" && w > 260 {
			ui.Box(c).Padding(0, 0, 0, 8).Children(func() {
				ui.Text(c, card.Extra).FontSize(12).TextColor(t.Faint).SingleLine()
			})
		}
	})
}

// estimateCardWidth 估一张卡片的内容宽度：抽屉正文左右各 24，三列网格间距 8，
// 卡片自身内边距 6。只在首帧（Bounds 还没有值）用。
func estimateCardWidth(c *ui.Context) float32 {
	winW, _ := c.Size()
	if winW <= 0 {
		winW = 800
	}
	return (winW-48-16)/3 - 12
}

// ---------------------------------------------------------------- 分集抽屉

// partsHeader 是选集抽屉的表头（原版 pageList.tsx）：「选集(N)」+ 定位当前 +
// 「全部添加」到播放列表。
func (a *App) partsHeader(c *ui.Context) {
	t := a.Theme
	n := 0
	if a.Info != nil {
		n = len(a.Info.Parts)
	}
	ui.Text(c, fmt.Sprintf("选集(%d)", n)).FontSize(14).Bold().
		TextColor(ui.Hex("#334155")).Shrink(0)

	if a.Track != nil {
		a.headerButton(c, "parts-locate", "定位到当前播放的位置", iconLocate, func() {
			a.locateNow = true
		})
	}
	// 选集搜索（原版 .part-search）：输入标题片段，回车滚到下一个匹配的分集
	//（连续回车继续下一个，循环）。
	ui.Box(c).Grow(1).MinWidth(0).Height(30).Radius(RadiusSmall).
		Background(ui.Hex("#ffffff").Alpha(0.80)).
		Border(1, ui.Hex("#94a3b8").Alpha(0.35)).
		AlignItems(ui.Center).Padding(0, 6).Children(func() {
		in := ui.TextInputBase(c, &a.PartSearch).Grow(1).FontSize(12).
			TextColor(ui.Hex("#334155")).Placeholder("搜索选集，回车跳转").Label("搜索选集")
		if in.Submitted() {
			a.locatePart(a.PartSearch)
		}
	})
	if n > 0 {
		b := ui.ButtonBase(c.Key("parts-add-all")).Height(32).Padding(0, 12).
			Radius(Radius).Center().Label("全部添加")
		if b.Hovered() {
			b.Background(t.GlassHover)
		} else {
			b.Background(t.GlassActive)
		}
		b.Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconAdd).Size(13, 13).TextColor(ui.Hex("#475569"))
				ui.Text(c, "全部添加").FontSize(12).TextColor(ui.Hex("#475569"))
			})
		})
		if b.Clicked() && a.Act.AddAllToPlaylist != nil {
			a.Act.AddAllToPlaylist()
		}
	}
}

// partsBody 是选集抽屉正文：三列卡片网格，每张卡片右上角一个「添加到播放列表」。
func (a *App) partsBody(c *ui.Context) {
	t := a.Theme
	var parts []Part
	if a.Info != nil {
		parts = a.Info.Parts
	}
	if len(parts) == 0 {
		ui.Text(c, "没有分集信息").FontSize(12).TextColor(t.Faint)
		return
	}

	const cols = 3
	ui.Column(c).Gap(8).Children(func() {
		for i := 0; i < len(parts); i += cols {
			end := min(i+cols, len(parts))
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
				for j := i; j < end; j++ {
					a.partCard(c, j, parts[j])
				}
				for j := end; j < i+cols; j++ {
					ui.Box(c).Grow(1).Basis(0)
				}
			})
		}
	})
	// 定位标志只在这一次构建里生效。
	a.locateNow = false
	a.locatePartIndex = -1
}

// partCard 是一张选集卡片。
func (a *App) partCard(c *ui.Context, index int, p Part) {
	t := a.Theme
	active := a.Track != nil && a.Track.Cid == p.Cid
	inList := a.playlistHas(p.Cid)
	box := ui.Column(c).Key(fmt.Sprintf("part-%d", p.Cid)).Grow(1).Basis(0).
		Padding(6).Radius(Radius).Gap(6).Cursor(ui.CursorPointer)
	switch {
	case active:
		box.Background(t.GlassHover).Border(2, t.Blue)
	case box.Hovered():
		box.Background(t.GlassHover)
	default:
		box.Background(t.Glass)
	}
	// 定位：点「定位到当前」滚到正在播的，选集搜索滚到匹配的。
	if (active && a.locateNow) || (a.locateNow && index == a.locatePartIndex) {
		box.ScrollIntoView()
	}

	box.Children(func() {
		cover := ui.Box(c).FillWidth().AspectRatio(16.0 / 9.0).Radius(RadiusSmall).
			Background(t.CoverPlaceholder()).Clip()
		cover.Children(func() {
			src := p.FirstFrame
			if src == "" && a.Info != nil {
				src = a.Info.Pic
			}
			if bmp := a.Images.Bitmap(src); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			}
			if p.Duration > 0 {
				ui.Text(c, fmtTime(float64(p.Duration))).FontSize(11).TextColor(ui.Hex("#f8fafc")).
					Absolute().Right(0).Bottom(0).Padding(2, 5).
					Radius(5, 0, 0, 0).Background(ui.Hex("#0f172a").Alpha(0.68))
			}
			if active {
				ui.Text(c, "正在播放").FontSize(9).TextColor(ui.Hex("#ffffff")).
					Absolute().Left(4).Top(4).Padding(1, 5).Radius(3).
					Background(t.Blue.Alpha(0.85))
			}
			// 右上角的添加键。
			addLabel := "添加到播放列表"
			if inList {
				addLabel = "已在播放列表中"
			}
			add := ui.ButtonBase(c.Key(fmt.Sprintf("part-add-%d", p.Cid))).
				Size(24, 24).Radius(RadiusPill).Center().Label(addLabel).Tooltip(addLabel)
			add.Absolute().Right(4).Top(4)
			if inList {
				add.Background(ui.Hex("#0f172a").Alpha(0.42))
			} else if add.Hovered() {
				add.Background(ui.Hex("#0f172a").Alpha(0.62))
			} else {
				add.Background(ui.Hex("#0f172a").Alpha(0.42))
			}
			add.Children(func() {
				ui.Icon(c, pick(inList, iconCheck, iconAdd)).Size(13, 13).
					TextColor(pick(inList, ui.Hex("#4ade80"), ui.Hex("#ffffff")))
			})
			if add.Clicked() && a.Act.AddToPlaylist != nil {
				a.Act.AddToPlaylist(p)
			}
		})
		ui.Text(c, p.Part).FontSize(13).TextColor(t.Ink).MaxLines(2)
	})
	if box.Clicked() {
		if a.Act.Play != nil {
			a.Act.Play(index)
		}
		a.Drawer = ""
	}
}

// playlistHas 返回这一集是否已在「我的列表」里。
func (a *App) playlistHas(cid int64) bool {
	for _, it := range a.Playlist {
		if it.Cid == cid {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- 弹幕 / 评论抽屉

// danmakuTabs 是表头（原版 danmakuList.tsx）：弹幕(N) / 评论(total) 两个 tab，
// 后面是「定位当前」（弹幕）与刷新。
func (a *App) danmakuTabs(c *ui.Context) {
	tabs := []tabItem{{
		Key:   TabDanmaku,
		Label: fmt.Sprintf("弹幕 (%d)", len(a.Danmaku)),
	}}
	replyLabel := "评论"
	if a.ReplyTotal > 0 {
		replyLabel = fmt.Sprintf("评论 (%d)", a.ReplyTotal)
	} else if len(a.Comments) > 0 {
		replyLabel = fmt.Sprintf("评论 (%d)", len(a.Comments))
	}
	tabs = append(tabs, tabItem{Key: TabReply, Label: replyLabel})

	a.tabs(c, "dm", tabs, a.DanmakuTab, func(k string) {
		a.DanmakuTab = k
		a.danmakuScrollIdx = -1
		if a.Act.SwitchDanmakuTab != nil {
			a.Act.SwitchDanmakuTab(k)
		}
	})

	if a.DanmakuTab == TabDanmaku {
		a.headerButton(c, "dm-autoscroll", pick(a.DanmakuAutoScroll, "关闭自动跟随", "开启自动跟随"),
			iconLocate, func() {
				a.DanmakuAutoScroll = !a.DanmakuAutoScroll
				a.danmakuScrollIdx = -1
			})
	}
}

// danmakuGroup 是同一秒的弹幕组（原版 DanmakuTimeGroup）。
type danmakuGroup struct {
	Second int
	// Entries 是这一秒里去重后的弹幕，按出现次数从多到少排。
	Entries []danmakuEntry
	// TotalCount 是这一秒的原始总条数。
	TotalCount int
}

type danmakuEntry struct {
	Text  string
	Color int
	Count int
}

// groupDanmaku 按秒分组：同一秒内相同文本合并计数，组内按次数排序
// （原版 groupedDanmaku 的逻辑）。
func groupDanmaku(list []Danmaku) []danmakuGroup {
	type key struct {
		second int
		text   string
	}
	counts := map[key]*danmakuEntry{}
	order := map[int][]string{}
	var seconds []int
	for _, d := range list {
		sec := int(d.Time)
		text := strings.Join(strings.Fields(d.Text), " ")
		k := key{sec, text}
		if _, ok := counts[k]; !ok {
			counts[k] = &danmakuEntry{Text: text, Color: d.Color}
			if _, seen := order[sec]; !seen {
				seconds = append(seconds, sec)
			}
			order[sec] = append(order[sec], text)
		}
		counts[k].Count++
	}
	out := make([]danmakuGroup, 0, len(seconds))
	for _, sec := range seconds {
		g := danmakuGroup{Second: sec}
		seenText := map[string]bool{}
		for _, text := range order[sec] {
			if seenText[text] {
				continue
			}
			seenText[text] = true
			g.Entries = append(g.Entries, *counts[key{sec, text}])
			g.TotalCount += counts[key{sec, text}].Count
		}
		// 组内按出现次数从多到少（原版排序）。
		for i := 1; i < len(g.Entries); i++ {
			for j := i; j > 0 && g.Entries[j].Count > g.Entries[j-1].Count; j-- {
				g.Entries[j], g.Entries[j-1] = g.Entries[j-1], g.Entries[j]
			}
		}
		out = append(out, g)
	}
	// 按秒升序（原版 sort）。
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Second < out[j-1].Second; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// danmakuInk 把弹幕颜色转成显示色：太亮/太暗时换对比色（原版 getColorStyle
// 的 YIQ 亮度逻辑；原生界面只有浅色主题，对应 isDarkMode = false 分支）。
func danmakuInk(color int) ui.Color {
	if color <= 0 {
		return ui.Hex("#1e293b")
	}
	r := (color >> 16) & 0xff
	g := (color >> 8) & 0xff
	b := color & 0xff
	brightness := (float64(r)*299 + float64(g)*587 + float64(b)*114) / 1000
	if brightness > 200 {
		return ui.Hex("#1a1a1a")
	}
	if brightness > 160 {
		return ui.Hex("#333333")
	}
	return ui.RGB(uint8(r), uint8(g), uint8(b))
}

// danmakuBody 画弹幕或评论列表。
// 弹幕按秒分组：组头「时间 + N 条」，组内去重、按次数排序、重复显示 ×N，
// 文字用弹幕自身颜色（太亮/太暗换对比色）——对齐原版 danmakuList。
func (a *App) danmakuBody(c *ui.Context) {
	if a.DanmakuTab == TabReply {
		a.repliesBody(c)
		return
	}
	t := a.Theme
	if len(a.Danmaku) == 0 {
		ui.Text(c, "还没有弹幕").FontSize(12).TextColor(t.Faint)
		return
	}
	groups := groupDanmaku(a.Danmaku)

	// 自动跟随：把当前时间所在的组滚进视野（只在下标变化时滚，不干扰手滑）。
	cur := -1
	if a.DanmakuAutoScroll && a.Pos > 0 {
		for i, g := range groups {
			if float64(g.Second) <= a.Pos {
				cur = i
			} else {
				break
			}
		}
	}

	ui.Column(c).Gap(6).Children(func() {
		for i, g := range groups {
			isCur := i == cur
			box := ui.Column(c).Key(fmt.Sprintf("dmg-%d", g.Second)).FillWidth().
				Padding(4, 8).Radius(RadiusSmall).Gap(3).
				Cursor(ui.CursorPointer)
			switch {
			case isCur:
				box.Background(t.Blue.Alpha(0.10))
			case box.Hovered():
				box.Background(t.GlassHover)
			}
			if isCur && a.danmakuScrollIdx != cur {
				box.ScrollIntoView()
			}
			box.Children(func() {
				// 组头：时间 + 这一秒的总条数。
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmtTime(float64(g.Second))).FontSize(10).
						TextColor(pick(isCur, t.Blue, t.Faint)).Width(44)
					ui.Text(c, fmt.Sprintf("%d 条", g.TotalCount)).FontSize(10).TextColor(t.Faint)
				})
				for _, e := range g.Entries {
					row := ui.Row(c).Key(fmt.Sprintf("dmg-%d-%s", g.Second, e.Text)).
						FillWidth().Gap(6).AlignItems(ui.Center)
					row.Children(func() {
						ui.Text(c, e.Text).FontSize(12).TextColor(danmakuInk(e.Color)).Grow(1).MaxLines(2)
						if e.Count > 1 {
							ui.Text(c, fmt.Sprintf("×%d", e.Count)).FontSize(10).
								TextColor(t.Faint).Shrink(0)
						}
					})
					if row.Clicked() && a.Act.Seek != nil {
						a.Act.Seek(float64(g.Second))
					}
				}
			})
			if box.Clicked() && a.Act.Seek != nil {
				a.Act.Seek(float64(g.Second))
			}
		}
	})
	a.danmakuScrollIdx = cur
}

// repliesBody 画评论列表（原版 danmakuList 的评论 tab）：热评 + 楼中楼预览 +
// 加载更多。
func (a *App) repliesBody(c *ui.Context) {
	t := a.Theme
	if len(a.Comments) == 0 {
		if a.RepliesLoading {
			ui.Text(c, "加载中…").FontSize(12).TextColor(t.Faint)
		} else {
			ui.Text(c, "还没有评论").FontSize(12).TextColor(t.Faint)
		}
		return
	}
	// 评论按时间倒序（原版 danmakuList 的 sortedReplies：b.ctime - a.ctime）。
	sorted := make([]Comment, len(a.Comments))
	copy(sorted, a.Comments)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].SendTime > sorted[j-1].SendTime; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	ui.Column(c).Gap(12).Children(func() {
		for i, cm := range sorted {
			ui.Column(c).Key(fmt.Sprintf("cm-%d", i)).FillWidth().Gap(4).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, cm.User).FontSize(12).Bold().TextColor(t.Blue).SingleLine()
					ui.Text(c, cm.Time).FontSize(10).TextColor(t.Faint)
					if cm.Likes > 0 {
						ui.Text(c, fmt.Sprintf("赞 %d", cm.Likes)).FontSize(10).TextColor(t.Faint)
					}
				})
				ui.Text(c, cm.Content).FontSize(12).TextColor(t.Ink).MaxLines(8)
				for j, rp := range cm.Replies {
					ui.Row(c).Key(fmt.Sprintf("cm-%d-r%d", i, j)).FillWidth().
						Padding(4, 8).Radius(RadiusSmall).
						Background(t.LiquidBg).Gap(6).Children(func() {
						ui.Text(c, rp.User+"：").FontSize(11).TextColor(t.Blue).Shrink(0)
						ui.Text(c, rp.Content).FontSize(11).TextColor(t.Muted).Grow(1).MaxLines(3)
					})
				}
			})
		}
		if a.RepliesHasMore {
			more := ui.ButtonBase(c.Key("reply-more")).FillWidth().Height(32).Radius(Radius).
				Center().Label("加载更多评论")
			if more.Hovered() {
				more.Background(t.GlassHover)
			} else {
				more.Background(t.Glass)
			}
			more.Children(func() {
				ui.Text(c, pick(a.RepliesLoading, "加载中…", "加载更多评论")).FontSize(12).TextColor(t.Muted)
			})
			if more.Clicked() && !a.RepliesLoading && a.Act.LoadComments != nil {
				a.Act.LoadComments(a.ReplyPage + 1)
			}
		}
	})
}

// locatePart 把选集搜索框的关键词跳到下一个匹配的分集（连续回车继续，
// 到尾再循环；原版 handleSearchPart）。从当前游标往后找，找不到再从头找。
func (a *App) locatePart(keyword string) {
	kw := strings.TrimSpace(keyword)
	if kw == "" || a.Info == nil || len(a.Info.Parts) == 0 {
		return
	}
	n := len(a.Info.Parts)
	start := a.PartSearchCursor
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		if strings.Contains(a.Info.Parts[idx].Part, kw) {
			a.PartSearchCursor = (idx + 1) % n
			a.locatePartIndex = idx
			a.locateNow = true
			a.Win.Update(func() {})
			return
		}
	}
	// 没有匹配就提示（不移动）。
	a.Notify("没有匹配的分集：" + kw)
}
