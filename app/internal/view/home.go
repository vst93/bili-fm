package view

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// ---------------------------------------------------------------- 搜索药丸

// searchBar 是居中的液态玻璃搜索药丸（原版 .home-searchbar）：
// max-width 520、高 46、圆角 13、内边距 4、子元素间距 8，
// 内容依次是「输入内腔 + 四个内容入口 + 头像」。
func (a *App) searchBar(c *ui.Context) {
	ui.Row(c).FillWidth().Height(SearchBarHeight).Shrink(0).Justify(ui.Center).Children(func() {
		// 原版 .home-searchbar 自己有 backdrop-filter（blur(20px) saturate(1.45)），
		// 是真玻璃，所以这里用 glass.Glass 而不是半透明填充。
		pill := ui.Row(c).Width(SearchBarWidth).Height(SearchBarHeight).Radius(SearchRadius).
			Padding(SearchPadding).Gap(8).AlignItems(ui.Center).
			Material(glass.Glass{})
		pill.Children(func() {
			a.searchField(c)
			a.sectionButtons(c)
			a.userButton(c)
		})
	})
}

// searchField 是药丸左侧的输入内腔（.home-search-input + .search-submit-btn）：
// 白底、圆角 8、内边距 4，右端一个 32×32 的放大镜提交键。
func (a *App) searchField(c *ui.Context) {
	t := a.Theme
	slot := ui.Row(c).Grow(1).Height(SearchSlotH).Radius(SearchSlotR).Padding(0, 4).
		Background(ui.Hex("#ffffff").Alpha(0.80)).
		Border(1, ui.Hex("#94a3b8").Alpha(0.35)).
		AlignItems(ui.Center)
	var in ui.Element
	slot.Children(func() {
		in = ui.TextInputBase(c, &a.Query).Grow(1).FontSize(14).
			TextColor(ui.Hex("#334155")).
			Placeholder("B站 / 关键词 / 视频链接").Label("搜索")
		if in.Submitted() {
			a.runSearch()
		}
		submit := ui.ButtonBase(c.Key("search-submit")).Size(ToolButton, ToolButton).
			Radius(ToolButtonR).Center().Label("搜索").Tooltip("搜索")
		if submit.Hovered() {
			submit.Background(t.GlassHover)
		} else {
			submit.Background(ui.Transparent)
		}
		submit.Children(func() { ui.Icon(c, iconMagnifier).Size(15, 15).TextColor(t.Muted) })
		if submit.Clicked() {
			a.runSearch()
		}
	})
	// 点内腔任意处都聚焦到输入框（原版是 HeroUI Input 自己的行为）。
	if slot.Clicked() && in.Valid() {
		in.Focus()
	}
}

func (a *App) runSearch() {
	if a.Query == "" || a.Act.Search == nil {
		return
	}
	a.Drawer = DrawerSearch
	a.Act.Search(a.Query)
}

// sectionButtons 是药丸右侧的四个内容入口（原版 .home-global-actions）：
// 动态 / 热门与推荐 / 收藏 / 历史，点开对应的分区抽屉。
func (a *App) sectionButtons(c *ui.Context) {
	ui.Row(c).Gap(4).Shrink(0).AlignItems(ui.Center).Children(func() {
		a.sectionButton(c, "feed", "动态", iconFeed)
		a.sectionButton(c, "popular", "热门与推荐", iconPopular)
		a.sectionButton(c, "favorite", "收藏", iconFavorite)
		a.sectionButton(c, "history", "历史", iconHistory)
	})
}

func (a *App) sectionButton(c *ui.Context, key, label string, ic *ui.SVG) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("nav-"+key)).Size(ToolButton, ToolButton).Radius(ToolButtonR).
		Center().Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(ui.Transparent)
	}
	b.Children(func() { ui.Icon(c, ic).Size(17, 17).TextColor(t.Muted) })
	if b.Clicked() {
		a.openSection(key)
	}
}

// openSection 打开某个分区的抽屉；数据没拉过（或换了分区）就拉第一页。
// 再点一次同一个入口就关掉。
func (a *App) openSection(key string) {
	if a.Drawer == key {
		a.Drawer = ""
		return
	}
	i := SectionIndex(key)
	changed := i != a.Section
	a.Section = i
	a.Drawer = key
	if changed || len(a.list().Cards) == 0 {
		a.reload()
	}
}

// userButton 是药丸最右端的圆形头像（原版 .home-user-button，36px）：
// 点击登录；已登录时显示头像。
func (a *App) userButton(c *ui.Context) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("user")).Size(AvatarSize, AvatarSize).Radius(RadiusPill).
		Center().Clip().Background(t.Glass).
		Border(1, ui.Hex("#ffffff").Alpha(0.55)).
		Label(pick(a.LoggedIn, "已登录", "扫码登录")).
		Tooltip(pick(a.LoggedIn, "已登录 "+a.UName, "扫码登录"))
	b.Children(func() {
		if bmp := a.Images.Bitmap(a.Face); bmp != nil {
			ui.Image(c, bmp).Fill().Fit(ui.Cover)
		} else {
			ui.Text(c, avatarInitial(a.UName)).FontSize(13).Bold().TextColor(t.Muted)
		}
	})
	if b.Clicked() && a.Act.Login != nil {
		a.Act.Login()
	}
}

// avatarInitial 是没头像时显示的字（用户名首字，没有就一个问号）。
func avatarInitial(name string) string {
	for _, r := range name {
		return string(r)
	}
	return "?"
}

// ---------------------------------------------------------------- 正在播放

// nowPlaying 是主区下半部分（原版 .home-now-playing）：
// 两栏 grid「304px + minmax(0,1fr)」、间距 28、纵向居中、底部留 32。
func (a *App) nowPlaying(c *ui.Context) {
	ui.Row(c).FillWidth().Grow(1).Shrink(1).Gap(28).AlignItems(ui.Center).
		Padding(0, 8, 32, 8).Children(func() {
		ui.Box(c).Width(304).Shrink(0).Children(func() { a.coverDisc(c) })
		a.videoInfo(c)
	})
}

// coverDisc 是左栏的封面圆盘（.cover-shell 292px，内部 #video-cover 90%）：
// 一个圆，播放时 22s 转一圈，点一下切换播放/暂停。
func (a *App) coverDisc(c *ui.Context) {
	ui.Row(c).FillWidth().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		ring := ui.Box(c).Size(292, 292).Shrink(0).Radius(RadiusPill).
			Border(1, ui.Hex("#ffffff").Alpha(0.20)).Center()
		ring.Children(func() {
			disc := ui.ButtonBase(c.Key("disc")).Size(263, 263).Radius(RadiusPill).Clip().
				Background(ui.Hex("#0f172a")).
				Border(1, ui.Hex("#ffffff").Alpha(0.20)).
				Label(pick(a.Playing, "暂停", "播放"))
			a.spinDisc(disc)
			disc.Children(func() {
				if bmp := a.Images.Bitmap(a.coverURL()); bmp != nil {
					ui.Image(c, bmp).Fill().Fit(ui.Cover)
				}
			})
			if disc.Clicked() && a.Act.TogglePlay != nil {
				a.Act.TogglePlay()
			}
		})
	})
}

// spinDisc 让封面圆盘转起来（原版 #video-cover.record-disc 是 22s 一圈的
// CSS 动画，animation-play-state 跟着播放状态）。
//
// Loop 的作用是让 mygo 持续出帧；角度按真实时间自己累加，这样暂停再继续
// 不会跳。暂停时不调 Loop，帧自然停下来。
func (a *App) spinDisc(e ui.Element) {
	now := time.Now()
	if a.Playing {
		e.Loop("disc", time.Hour, ui.Linear)
		a.discDeg += float32(now.Sub(a.discAt).Seconds()) * (360.0 / 22.0)
	}
	a.discAt = now
	e.Rotate(a.discDeg)
}

// videoInfo 是右栏（原版 #video-info，固定 366 高）：
// UP 主行 / 标题 / 简介 / 选集胶囊 / 操作行 / 工具条，自上而下。
func (a *App) videoInfo(c *ui.Context) {
	t := a.Theme
	ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Start).Children(func() {
		a.videoInfoHead(c)
		ui.Text(c, a.infoTitle()).FontSize(22).Bold().TextColor(t.Ink).
			MaxLines(2).Margin(8, 0, 0, 0)
		ui.Text(c, a.infoDesc()).FontSize(13).TextColor(t.Muted).
			MaxLines(3).Margin(14, 0, 0, 0)
		a.partPill(c)
		a.contentActions(c)
		a.contextDock(c)
	})
}

// videoInfoHead 是 UP 主行（.video-info-head）：40px 圆头像 + 「UP 主」+ 名字。
// 点头像或名字进 UP 主的空间（旧版的 onOwnerClick）。
func (a *App) videoInfoHead(c *ui.Context) {
	t := a.Theme
	ui.Row(c).Gap(14).AlignItems(ui.Center).Margin(0, 0, 18, 0).Children(func() {
		face := ui.ButtonBase(c.Key("owner-face")).Size(40, 40).Radius(RadiusPill).Clip().
			Background(t.CoverPlaceholder()).Label("UP 主空间")
		face.Children(func() {
			if bmp := a.Images.Bitmap(a.ownerFace()); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			}
		})
		if face.Clicked() {
			a.openUp()
		}

		ui.Column(c).MinWidth(0).Children(func() {
			ui.Text(c, "UP 主").FontSize(10).Bold().TextColor(ui.Hex("#8a95a6"))
			name := ui.ButtonBase(c.Key("owner-name")).Padding(0).Label("UP 主空间")
			name.Background(ui.Transparent)
			name.Children(func() {
				ui.Text(c, a.ownerName()).FontSize(15).Bold().
					TextColor(pick(name.Hovered(), t.Blue, ui.Hex("#334155"))).
					SingleLine().MaxWidth(240)
			})
			if name.Clicked() {
				a.openUp()
			}
		})
	})
}

// openUp 打开当前视频 UP 主的空间。没有 mid 就什么都不做。
func (a *App) openUp() {
	var mid int64
	if a.Info != nil {
		mid = a.Info.OwnerMid
	}
	if mid == 0 || a.Act.OpenUp == nil {
		return
	}
	a.Act.OpenUp(mid, a.ownerName())
}

// partPill 是选集胶囊（.video-part-pill）：小圆点 + 来源 + 分集标题。
func (a *App) partPill(c *ui.Context) {
	blue := ui.Hex("#0369a1")
	ui.Row(c).MaxWidth(430).Height(36).Margin(12, 0, 0, 0).Padding(5, 11).
		Radius(RadiusPill).Border(1, ui.Hex("#ffffff").Alpha(0.38)).
		Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Shrink(0).Radius(RadiusPill).Background(ui.Hex("#0284c7"))
		ui.Text(c, "当前选集").FontSize(13).Bold().TextColor(blue).Shrink(0)
		ui.Text(c, a.partTitle()).FontSize(13).Bold().TextColor(blue).SingleLine()
	})
}

// contentActions 是操作行（.video-content-actions）：点赞 / 投币 / 收藏
// 三个带计数的按钮，再加浏览器打开 / 视频 / 弹幕三个纯图标按钮。
func (a *App) contentActions(c *ui.Context) {
	ui.Row(c).Margin(14, 0, 0, 0).Gap(4).AlignItems(ui.Start).Children(func() {
		a.statButton(c, "like", iconLike, a.Liked, a.statLike(), a.Act.Like)
		a.statButton(c, "coin", iconCoin, a.Coined, a.statCoin(), a.Act.Coin)
		a.statButton(c, "fav", iconStar, a.Faved, a.statFav(), a.Act.Favorite)
		a.iconButton(c, "browser", "浏览器打开", iconBrowser, func() {})
		a.iconButton(c, "video", "视频播放", iconVideo, func() {
			if a.VideoOpen {
				if a.Act.CloseVideo != nil {
					a.Act.CloseVideo()
				}
			} else if a.Act.OpenVideo != nil {
				a.Act.OpenVideo()
			}
		})
		a.iconButton(c, "danmaku", "弹幕/评论", iconDanmaku, a.openDanmaku)
	})
}

// contextDock 是工具条（.video-context-dock）：
// 搜索结果 / 选集 / 合集 / 播放列表（旧版四个）。
func (a *App) contextDock(c *ui.Context) {
	ui.Row(c).Margin(12, 0, 0, 0).Padding(3).Radius(10).Gap(2).AlignItems(ui.Center).
		Background(ui.Hex("#0f172a").Alpha(0.03)).
		Border(1, ui.Hex("#64748b").Alpha(0.14)).Children(func() {
		a.iconButton(c, "dock-search", "搜索结果", iconSearch, func() {
			a.Drawer = DrawerSearch
		})
		a.iconButton(c, "dock-parts", "选集", iconList, a.openParts)
		a.iconButton(c, "dock-series", "合集", iconSeries, a.openSeries)
		a.iconButton(c, "dock-playlist", "播放列表", iconMusicList, func() {
			a.Drawer = DrawerInfo
		})
	})
}

// openSeries 打开当前合集（旧版：还没选过合集就提示先去 UP 空间选一个）。
func (a *App) openSeries() {
	if a.Drawer == DrawerSeries {
		a.Drawer = ""
		return
	}
	if a.SeriesID == 0 {
		// 旧版这里弹 toast 让用户先去 UP 空间选；没有 toast 就直接进 UP 空间的
		// 合集 tab，少一步操作。
		a.UpTab = UpTabSeries
		a.openUp()
		return
	}
	a.Drawer = DrawerSeries
	if a.Act.SelectSeries != nil {
		a.Act.SelectSeries(a.SeriesID)
	}
}

// openParts 打开分集抽屉；分集数据由上层拉取。
func (a *App) openParts() {
	if a.Track == nil {
		return
	}
	if a.Drawer == DrawerParts {
		a.Drawer = ""
		return
	}
	a.Drawer = DrawerParts
	if a.Act.OpenParts != nil {
		a.Act.OpenParts(*a.Track)
	}
}

// openDanmaku 打开弹幕抽屉；弹幕数据由上层拉取。
func (a *App) openDanmaku() {
	if a.Drawer == DrawerDanmaku {
		a.Drawer = ""
		return
	}
	a.Drawer = DrawerDanmaku
	if a.Act.ToggleDanmaku != nil {
		a.Act.ToggleDanmaku()
	}
}

// statButton 是带计数的小按钮：上面 32×32 图标，下面 10px 计数。
func (a *App) statButton(c *ui.Context, key string, ic *ui.SVG, active bool, count string, fn func()) {
	t := a.Theme
	ui.Column(c).Gap(2).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c.Key("stat-"+key)).Size(ToolButton, ToolButton).
			Radius(ToolButtonR).Center().Label(key).Tooltip(key)
		switch {
		case active:
			b.Background(t.Blue.Alpha(0.22))
		case b.Hovered():
			b.Background(t.GlassHover)
		default:
			b.Background(ui.Transparent)
		}
		b.Children(func() {
			ui.Icon(c, ic).Size(17, 17).TextColor(pick(active, t.Blue, ui.Hex("#475569")))
		})
		if b.Clicked() && fn != nil {
			fn()
		}
		if count != "" {
			ui.Text(c, count).FontSize(10).TextColor(t.Faint)
		}
	})
}

// iconButton 是 32×32 的纯图标按钮（.nav-icon-btn）。
func (a *App) iconButton(c *ui.Context, key, label string, ic *ui.SVG, fn func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("icon-"+key)).Size(ToolButton, ToolButton).Radius(ToolButtonR).
		Center().Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(ui.Transparent)
	}
	b.Children(func() { ui.Icon(c, ic).Size(17, 17).TextColor(ui.Hex("#475569")) })
	if b.Clicked() && fn != nil {
		fn()
	}
}

// ---------------------------------------------------------------- 文案取值

func (a *App) infoTitle() string {
	if a.Info != nil && a.Info.Title != "" {
		return a.Info.Title
	}
	if a.Track != nil && a.Track.Title != "" {
		return a.Track.Title
	}
	return "暂无播放内容"
}

func (a *App) infoDesc() string {
	if a.Info != nil && a.Info.Desc != "" {
		return a.Info.Desc
	}
	return "无描述"
}

func (a *App) ownerName() string {
	if a.Info != nil && a.Info.OwnerName != "" {
		return a.Info.OwnerName
	}
	if a.Track != nil && a.Track.Up != "" {
		return a.Track.Up
	}
	return "神秘的UP主"
}

func (a *App) ownerFace() string {
	if a.Info != nil {
		return a.Info.OwnerFace
	}
	return ""
}

func (a *App) partTitle() string {
	if a.Track != nil && a.Track.Part != "" {
		return a.Track.Part
	}
	return "无选集标题"
}

// coverURL 是圆盘上显示的封面：优先详情里的首帧，退回列表卡片封面
// （对应原版的 graftingImage(pageFirstFrame || displayVideoInfo?.pic)）。
func (a *App) coverURL() string {
	if a.Info != nil && a.Info.Pic != "" {
		return a.Info.Pic
	}
	if a.Track != nil {
		return a.Track.Cover
	}
	return ""
}

func (a *App) statLike() string { return statText(a.Info, func(i *Info) int64 { return i.Like }) }
func (a *App) statCoin() string { return statText(a.Info, func(i *Info) int64 { return i.Coin }) }
func (a *App) statFav() string  { return statText(a.Info, func(i *Info) int64 { return i.Favorite }) }

func statText(info *Info, pick func(*Info) int64) string {
	if info == nil {
		return ""
	}
	if n := pick(info); n > 0 {
		return compactCount(n)
	}
	return ""
}
