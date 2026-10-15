package view

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/imagecache"
)

// 播放列表抽屉（原版 playlist.tsx）：表头是「我的列表 / 合集列表」两个 tab +
// 定位当前 + 播放模式 + 清空；正文是记录行（缩略图 + 分集名 + 视频名 + 删除）。
//
// 记录行支持拖拽排序（原版 draggable + onDragOver/onDrop），拖到别的行上即
// 插入到那个位置。

// playlistRowRadius 是记录行的圆角：原版 Tailwind 是 rounded-lg（8），但
// globals.css 的 `[data-slot="wrapper"] .group` 把它覆写成 10px !important。
const playlistRowRadius = 10

// playlistTabs 是播放列表抽屉的表头。
func (a *App) playlistTabs(c *ui.Context) {
	tabs := []tabItem{{Key: ListUser, Label: fmt.Sprintf("我的列表(%d)", len(a.Playlist))}}
	if len(a.SeriesPlaylist) > 0 {
		tabs = append(tabs, tabItem{Key: ListSeries, Label: fmt.Sprintf("合集列表(%d)", len(a.SeriesPlaylist))})
	}
	a.tabs(c, "playlist", tabs, a.PlaylistTab, func(k string) {
		a.PlaylistTab = k
		if a.Act.SwitchPlaylistTab != nil {
			a.Act.SwitchPlaylistTab(k)
		}
	})

	// 定位到当前播放的记录：只在当前 tab 就是正在播放的来源时有效。
	if a.PlayingPlaylist == a.PlaylistTab {
		if items := a.playlistActive(a.PlaylistTab); a.Index >= 0 && a.Index < len(items) {
			a.headerButton(c, "playlist-locate", "定位到当前播放的记录", iconLocate, 18, ui.Hex("#888888"), func() {
				a.locateNow = true
			})
		}
	}

	a.playModeButton(c)

	if a.PlaylistTab == ListUser && len(a.Playlist) > 0 {
		a.headerButton(c, "playlist-clear", "清空播放列表", iconDelete, 18, ui.Hex("#888888"), func() {
			if a.Act.ClearPlaylist != nil {
				a.Act.ClearPlaylist(ListUser)
			}
		})
	}
}

// playModeIconInk 解析播放模式图标的颜色（原版 playlist.tsx）：顺序用 Order，
// fill="#888"；单曲循环 LoopOnce 与随机 Shuffle 都是 fill="#3b82f6"（蓝）。
// 此前三种模式统一用抽屉表头的 #475569、非顺序才转主题蓝，色值都不对。
func playModeIconInk(mode string) ui.Color {
	if mode == PlayModeSequence {
		return ui.Hex("#888888")
	}
	return ui.Hex("#3b82f6")
}

// playModeButton 是播放模式开关（顺序 → 单曲循环 → 随机 → 顺序）。
func (a *App) playModeButton(c *ui.Context) {
	t := a.Theme
	ic := iconOrder
	label := "当前：顺序播放"
	switch a.PlayMode {
	case PlayModeSingle:
		ic, label = iconLoopOne, "当前：单曲循环"
	case PlayModeShuffle:
		ic, label = iconShuffle, "当前：随机播放"
	}
	b := ui.ButtonBase(c.Key("play-mode")).Size(32, 32).Radius(Radius).Center().
		Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(t.GlassActive)
	}
	b.Children(func() {
		ui.Icon(c, ic).Size(18, 18).TextColor(playModeIconInk(a.PlayMode))
	})
	if b.Clicked() && a.Act.CyclePlayMode != nil {
		a.Act.CyclePlayMode()
	}
}

// playlistBody 画记录列表。
//
// 用虚拟化 ui.List（同卡片网格）：播放列表可能上百条，逐行全建会让每行都
// 调一次 Bitmap，上百张 192w 缩略图每帧争抢缓存、互相把对方挤出 64 条 LRU
// （实测 100 行 352 次重复抓取、可见缩略图只剩 1 张），用户报「左侧图标不
// 显示」。改成只建可见行 + 只预取可见窗口后，抓取次数回落到与可见行同阶。
func (a *App) playlistBody(c *ui.Context) {
	t := a.Theme
	items := a.playlistActive(a.PlaylistTab)
	if len(items) == 0 {
		// 原版 playlist.tsx 的空态（两个 tab 共用同一块）：
		//
		//	<div className="flex flex-col items-center justify-center h-full text-gray-400">
		//	  <p className="text-sm">播放列表为空</p>
		//	  <p className="text-xs mt-1">从选集列表中点击 + 添加选集到播放列表</p>
		//	</div>
		//
		// 关键是 `h-full` + `justify-center`：文案在**整个正文里竖直居中**，
		// 不是贴在正文顶上。原生版此前只有一段 `Margin(20,0,0,0)` 的近似，
		// 文案压在表头下约 28px（无头实测文案块中心 156，原版 348），
		// 下面空出一大片。这里用 `Grow(1)` 把容器撑满正文剩下的高度，再 Center。
		//
		// 两行的行盒按原版压回：text-sm 是 14/20、text-xs 是 12/16（见
		// linebox.go），副行与原版一样 `mt-1` = 4 的间距。
		ui.Row(c).FillWidth().Grow(1).Padding(8, 24).Center().Children(func() {
			if a.PlaylistTab == ListSeries {
				// 本 fork 为「合集列表」tab 单独写的一句（原版两个 tab 共用上面那段
				// 「播放列表为空」文案，对合集 tab 是错的）。位置与主列表空态一致：
				// 正文里居中。合集列表非空时才会出现该 tab，所以这一支只在「切到
				// 合集 tab 后列表被清空」的边缘场景可见。
				ui.Text(c, "合集列表为空（在合集里点「播放全部」加载）").FontSize(12).TextColor(t.Faint)
				return
			}
			th, tsh := textLineBox(14, textSmLH)
			mh, msh := textLineBox(12, textXsLH)
			ui.Column(c).Gap(4).Center().Children(func() {
				ui.Text(c, "播放列表为空").FontSize(14).TextColor(t.Faint).
					Height(th).Translate(0, tsh)
				ui.Text(c, "从选集列表中点击 + 添加选集到播放列表").FontSize(12).
					TextColor(t.Faint).Height(mh).Translate(0, msh)
			})
		})
		return
	}
	l := a.list()
	st := &l.Rows
	// 行键取记录 ID：拖拽排序 / 删除后，行状态跟着记录走。
	st.Key = func(i int) any { return items[i].ID }
	// 定位当前：屏外的行不构建，把目标行交给 ListState 让它布局时滚过去。
	if a.locateNow && a.Index >= 0 && a.Index < len(items) {
		st.ScrollTo(a.Index, ui.Start)
	}
	a.prefetchPlaylistCovers(st, items)
	a.holdListScroll(c, &a.playlistHold, st, &l.RowsScroll, true)
	ui.List(c, st, len(items), func(i int) { a.playlistRow(c, i, items[i]) }).
		FillWidth().Grow(1).Gap(4).Padding(8, 24).TrackScroll(&l.RowsScroll)
	// 定位标志只在这一次构建里生效。
	a.locateNow = false
}

// playlistCoverURL 是一条播放列表记录要显示的缩略图地址：分集首帧优先，
// 否则视频封面（原版 `graftingImage(item.first_frame || item.pic, 192)`）。
func playlistCoverURL(it PlayItem) string {
	if it.FirstFrame != "" {
		return it.FirstFrame
	}
	return it.Pic
}

// playlistInitialRows 是播放列表还没布局过时预取的初始行数。
const playlistInitialRows = 8

// prefetchPlaylistCovers 只给可见行（外加上下几行 overscan）预取缩略图。
// 与 prefetchCardCovers 同一思路：构建只 Get，请求只按上一帧的可见窗口发。
func (a *App) prefetchPlaylistCovers(st *ui.ListState, items []PlayItem) {
	first, last := st.Visible()
	if last < first {
		first, last = 0, min(len(items), playlistInitialRows)
	}
	first = max(0, first-2)
	last = min(len(items), last+3)
	for i := first; i < last; i++ {
		a.Images.Bitmap(playlistCoverURL(items[i]), imagecache.WidthPlaylist)
	}
}

// playlistRowPaint 是播放列表一行的配色与字重，逐条对应原版 globals.css 里
// `[data-slot="wrapper"]` 作用域下的几条规则。原版行有三套互斥的 class
// （playlist.tsx 的三元）：常态带 hover:bg-gray-100、当前项带
// bg-blue-100+playlist-current、拖拽悬停带 bg-blue-50。层叠后实际生效的是：
//
//	常态        .group 白 .42 底、白 .28 边、顶部内高光白 .42、
//	            外投影 15,23,42 .04（y2/blur8）
//	常态:hover  .hover\:bg-gray-100:hover 白 .55 底（比 .group:hover 的
//	            .68 更靠后，同特异度，故胜出）+ 边转天蓝 .34、内高光白 .70、
//	            外投影 15,23,42 .10（y8/blur22）+ 一圈 1px 天蓝环
//	当前项      bg-blue-100 覆写为平铺 2,132,199 .16（比 .playlist-current 的
//	            渐变更靠后、同特异度，故胜；渐变只在悬停时因 (0,3,0) 胜出），
//	            边 2,132,199 .46、左缘 4px 强调条 3,105,161 .78、
//	            内高光白 .64 / 底部内影 2,132,199 .16、
//	            外投影 2,132,199 .14（y8/blur22）
//	当前项:hover .playlist-current:hover 的蓝→白渐变、边转 2,132,199 .58
//	:active     常态白 .30、当前项 2,132,199 .1（外加 scale .985）
//	拖拽悬停    [class*="bg-blue-50"] 天蓝→白渐变、左缘 4px 强调条 .68、
//	            内高光白 .60、外投影 2,132,199 .12（y4/blur14）
//
// 文案色/字重来自同一作用域的覆写：.text-gray-800 → #1e293b/600、
// .text-gray-400 → #64748b、当前项首行 → #064e7a/800、末行 → #334155/600。
//
// 说明（框架硬底）：当前项:hover / 拖拽态的 CSS 是**三段**线性渐变
// （天蓝 .36 → .20@42% → 白 .62），mygo 的 LinearGradient 只有两段
// （From/To），所以两端取值、中间用两段插值近似。
type playlistPaint struct {
	From, To    ui.Color // 渐变/纯色底；Gradient 为假时只用 From
	Gradient    bool
	Border      ui.Color
	Accent      ui.Color // 左缘强调条；AccentWidth 为 0 表示不画
	AccentW     float32
	InsetTop    ui.Color
	InsetBot    ui.Color
	Shadow      ui.Color
	ShadowY     float32
	ShadowBlur  float32
	Ring        bool // hover 的 1px 天蓝环
	PartInk     ui.Color
	PartWeight  int
	TitleInk    ui.Color
	TitleWeight int
}

// playlistRowPaint 返回播放列表一行的配色。状态优先级与原版 CSS 一致：
// 拖拽悬停 > 当前项 > 悬停；按下态只改底色（:active 比 :hover 更靠后）。
func playlistRowPaint(current, hovered, dragging, pressed bool) playlistPaint {
	const white = "#ffffff"
	p := playlistPaint{
		From:        ui.Hex(white).Alpha(0.42),
		Border:      ui.Hex(white).Alpha(0.28),
		InsetTop:    ui.Hex(white).Alpha(0.42),
		Shadow:      ui.Hex("#0f172a").Alpha(0.04),
		ShadowY:     2,
		ShadowBlur:  8,
		PartInk:     ui.Hex("#1e293b"),
		PartWeight:  600,
		TitleInk:    ui.Hex("#64748b"),
		TitleWeight: 400,
	}
	switch {
	case dragging:
		p.From = ui.Hex("#0ea5e9").Alpha(0.24)
		p.To = ui.Hex(white).Alpha(0.56)
		p.Gradient = true
		p.Accent = ui.Hex("#0369a1").Alpha(0.68)
		p.AccentW = 4
		p.InsetTop = ui.Hex(white).Alpha(0.60)
		p.Shadow = ui.Hex("#0284c7").Alpha(0.12)
		p.ShadowY, p.ShadowBlur = 4, 14
	case current:
		p.From = ui.Hex("#0284c7").Alpha(0.16)
		p.Border = ui.Hex("#0284c7").Alpha(0.46)
		p.Accent = ui.Hex("#0369a1").Alpha(0.78)
		p.AccentW = 4
		p.InsetTop = ui.Hex(white).Alpha(0.64)
		p.InsetBot = ui.Hex("#0284c7").Alpha(0.16)
		p.Shadow = ui.Hex("#0284c7").Alpha(0.14)
		p.ShadowY, p.ShadowBlur = 8, 22
		p.PartInk = ui.Hex("#064e7a")
		p.PartWeight = 800
		p.TitleInk = ui.Hex("#334155")
		p.TitleWeight = 600
		if hovered {
			p.From = ui.Hex("#0ea5e9").Alpha(0.36)
			p.To = ui.Hex(white).Alpha(0.62)
			p.Gradient = true
			p.Border = ui.Hex("#0284c7").Alpha(0.58)
		}
	case hovered:
		p.From = ui.Hex(white).Alpha(0.55)
		p.Border = ui.Hex("#0ea5e9").Alpha(0.34)
		p.InsetTop = ui.Hex(white).Alpha(0.70)
		p.Shadow = ui.Hex("#0f172a").Alpha(0.10)
		p.ShadowY, p.ShadowBlur = 8, 22
		p.Ring = true
	}
	if pressed {
		p.Gradient = false
		p.To = ui.Transparent
		p.Ring = false
		if current {
			p.From = ui.Hex("#0284c7").Alpha(0.10)
		} else {
			p.From = ui.Hex(white).Alpha(0.30)
		}
	}
	return p
}

// playlistDeleteFadeAnim 是记录行删除键的淡入淡出：原版那颗键是
// `opacity-0 group-hover:opacity-100 transition-opacity`，Tailwind 的
// `transition-opacity` 默认 150ms + cubic-bezier(.4, 0, .2, 1)。声明成 var
// 便于测试换桩反证。
var playlistDeleteFadeAnim = ui.Animation{
	Duration: 150 * time.Millisecond,
	Ease:     ui.CubicBezier(0.4, 0, 0.2, 1),
}

// playlistDeleteOpacity 是删除键这一帧的不透明度目标：行悬停 1、否则 0。
// 原版那颗键**始终在 DOM 里**（只切不透明度），所以它占的 26px + 8px 间距
// 一直从文字列里扣走；原生版此前把「是否存在」挂在悬停上，行一悬停文字列
// 就被抽窄 34px，长标题当场重新截断、整行文字跳一下。
func playlistDeleteOpacity(rowHovered bool) float32 {
	if rowHovered {
		return 1
	}
	return 0
}

// playlistRowOpacity 是记录行**自身**这一帧的不透明度：原版被拖的那一行
// 带 `opacity-40`（Tailwind 40% = 0.4），其余 1。它是瞬时切换 —— 行上的
// transition 只有 `transition-colors`，不含 opacity，所以不淡出。
//
// 拖拽副本不受影响：框架画副本时会临时清掉源元素自身的 opacity
// （`ui/paintDrag` 里 `e.opacitySet = false`）。
func playlistRowOpacity(selfDragging bool) float32 {
	if selfDragging {
		return 0.4
	}
	return 1
}

// playlistDragOver 把框架的 DragOver 结果映射成原版的 `isDragOver`：原版
// `handleDragOver` 只在 `dragIndex !== index` 时置位，所以被拖的那一行即使
// 指针回到自己身上也只带 `opacity-40`、不会切成 bg-blue-50。mygo 的
// ui.DragOver 不排除源行，这里显式压掉。
func playlistDragOver(selfDragging, over bool) bool {
	return over && !selfDragging
}

// playlistRow 是一条记录。我的列表支持拖拽排序（拖到别的行上即插入到那个位置）。
func (a *App) playlistRow(c *ui.Context, index int, item PlayItem) {
	current := a.PlayingPlaylist == a.PlaylistTab && a.Index == index
	// 原版行：p-2（8）、rounded-lg（8，但 .group 覆写成 10）、gap-2（8）。
	// 当前项有 border-left 4px 强调条：原来是把 4px 色条当普通子元素放在
	// 8px 左内边距之后，缩略图被推到 20px；原版是 4+8=12px。当前项把左内边距
	// 收成 0，让「强调条(4) + gap(8)」正好把内容推到 12px。
	padL := float32(8)
	if current {
		padL = 0
	}
	row := ui.Row(c).Key("pl-"+item.ID).FillWidth().Padding(8, 8, 8, padL).
		Radius(playlistRowRadius).Gap(8).AlignItems(ui.Center)

	// 拖拽排序：拖自己（值=记录 id），放到别的行上就插入到那个位置。
	editable := a.PlaylistTab == ListUser
	if editable {
		row.Drag(item.ID)
	}
	_, dragOver := ui.DragOver[string](row)
	droppedID, dropped := ui.Drop[string](row)

	// 原版把「拖拽悬停」与「自身正在被拖」分开：`isDragOver` 只在
	// dragIndex !== index 时置位，所以被拖的那一行即使指针回到自己身上也
	// 不会切成 bg-blue-50；它只带 `opacity-40`。mygo 的 DragOver 不排除
	// 源行，这里显式压掉。
	selfDragging := editable && row.Dragging()
	dragOver = playlistDragOver(selfDragging, dragOver)
	// 原版 `${isDragging ? "opacity-40" : ""}`：被拖的整行瞬时变 40% 不透明。
	row.Opacity(playlistRowOpacity(selfDragging))

	p := playlistRowPaint(current, row.Hovered(), dragOver, row.Pressed())
	if p.Gradient {
		// 原版 .playlist-current 的蓝→白 90° 渐变。
		row.Gradient(p.From, p.To, 90)
	} else {
		row.Background(p.From)
	}
	row.Border(1, p.Border).InsetShadow(0, 1, 0, 0, p.InsetTop)
	if p.InsetBot != ui.Transparent {
		row.InsetShadow(0, -1, 0, 0, p.InsetBot)
	}
	row.Shadow(0, p.ShadowY, p.ShadowBlur, 0, p.Shadow)
	if p.Ring {
		// 原版 hover 的第二层投影：0 0 0 1px rgba(14,165,233,.12)。
		row.Shadow(0, 0, 0, 1, ui.Hex("#0ea5e9").Alpha(0.12))
	}
	// 原版 .group:active / .playlist-current:active 的按压缩放（松开回位）。
	pose := ui.Pose{}
	if row.Pressed() {
		pose = ui.Pose{Scale: 0.985}
	}
	row.AnimatePose(pose, ui.Animation{Duration: 120 * time.Millisecond, Ease: ui.EaseOut})
	// 定位已交给 playlistBody 的 ListState.ScrollTo（屏外的行不构建，
	// 行自己的 ScrollIntoView 对虚拟列表无效）。

	row.Children(func() {
		// 原版 .playlist-current 的 border-left 4px 强调条。
		//
		// 注意用 FillHeight 而不是 Fill：**Fill() 会把宽度也设成 100%**，
		// 这条 4px 色条于是撑满整行、把封面和文字挤成 0 宽
		//（表现就是选中项变成一条纯蓝色横条）。
		if p.AccentW > 0 {
			ui.Box(c).Width(p.AccentW).FillHeight().Shrink(0).Radius(2).
				Background(p.Accent)
		}
		// 缩略图 64×36。原版 .bg-gray-200 被覆写成 rgba(226,232,240,.6)。
		ui.Box(c).Width(64).Height(36).Shrink(0).Radius(4).
			Background(ui.Hex("#e2e8f0").Alpha(0.6)).Clip().Children(func() {
			src := playlistCoverURL(item)
			// 构建只读缓存：缩略图由 playlistBody 的可见窗口预取负责抓。
			if bmp := a.Images.Get(src, imagecache.WidthPlaylist); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			} else if placeholderCover != nil {
				// 未就绪 / 本来就没缩略图（src 空，原版 RetryImg `src || fallbackSrc`
				// 同样画 /cover.png）都先铺占位图，与列表卡片同一套渐进式加载。
				ui.Image(c, placeholderCover).Fill().Fit(ui.Cover)
			}
		})

		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			// 原版两行都是 Tailwind 字号类：首行 text-sm（14px / 20px 行高）、
			// 次行 text-xs（12px / 16px）。框架的自然行盒是 22 / 18，整行因此
			// 比原版高 4px（行高 54→58）；用显式盒高 + 字形上移压回（见
			// linebox.go 的 textSmLH / textXsLH）。
			th, tsh := textLineBox(14, textSmLH)
			mh, msh := textLineBox(12, textXsLH)
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				if current {
					// 原版 <Play fill="#3b82f6" size={12} theme="filled" />：蓝色圆盘
					// + 挖白三角，两层色，走逐色 Image。
					ui.Image(c, iconPlayFilled).Size(12, 12).TextColor(ui.Hex("#3b82f6"))
				}
				// 原版首行：常态 .text-gray-800（#1e293b/600），当前项
				// .playlist-current p:first-of-type（#064e7a/800）。
				ui.Text(c, item.Part).FontSize(14).SingleLine().
					TextColor(p.PartInk).FontWeight(p.PartWeight).
					Height(th).Translate(0, tsh)
			})
			// 原版次行：常态 .text-gray-400（#64748b），当前项
			// .playlist-current p:last-of-type（#334155/600）。
			ui.Text(c, item.Title).FontSize(12).SingleLine().
				TextColor(p.TitleInk).FontWeight(p.TitleWeight).
				Height(mh).Translate(0, msh)
		})

		// 我的列表才允许删除（排序靠拖拽）。原版那颗键**始终渲染**，靠
		// `opacity-0 group-hover:opacity-100 transition-opacity` 淡入淡出 ——
		// 始终占位，悬停不会把文字列抽窄（见 playlistDeleteOpacity）。
		if editable {
			a.rowIconButton(c, fmt.Sprintf("pl-del-%s", item.ID), "删除", iconClose,
				ui.Hex("#999999"), playlistDeleteOpacity(row.Hovered()), func() {
					if a.Act.DeletePlaylistItem != nil {
						a.Act.DeletePlaylistItem(ListUser, item.ID)
					}
				})
		}
	})
	if row.Clicked() && a.Act.PlayPlaylist != nil {
		a.Act.PlayPlaylist(a.PlaylistTab, index)
	}
	// 放下：把被拖的记录插到这一行的位置。
	if dropped && editable && a.Act.ReorderPlaylist != nil {
		from := -1
		for i, it := range a.Playlist {
			if it.ID == droppedID {
				from = i
				break
			}
		}
		if from >= 0 && from != index {
			a.Act.ReorderPlaylist(ListUser, from, index)
		}
	}
}

// rowIconButton 是记录行右侧的小图标键（row 的子元素，不冒泡触发播放）。
// opacity 是这一帧的目标不透明度（删除键常态 0、行悬停 1），走
// playlistDeleteFadeAnim 淡入淡出。
func (a *App) rowIconButton(c *ui.Context, key, label string, ic *ui.SVG, ink ui.Color, opacity float32, fn func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key(key)).Size(26, 26).Radius(RadiusSmall).Center().
		Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.GlassActive)
	} else {
		b.Background(ui.Transparent)
	}
	b.Opacity(b.AnimateTo("pl-del-fade", opacity, playlistDeleteFadeAnim))
	b.Children(func() { ui.Icon(c, ic).Size(16, 16).TextColor(ink) })
	if b.Clicked() {
		fn()
	}
}

// PlayItemFromPart 把当前详情的某一集转成播放列表记录。
func PlayItemFromPart(info *Info, bvid string, aid int64, part Part) PlayItem {
	title, pic := "", ""
	if info != nil {
		title, pic = info.Title, info.Pic
	}
	return PlayItem{
		ID:         fmt.Sprintf("%s-%d", bvid, part.Cid),
		Bvid:       bvid,
		Aid:        aid,
		Cid:        part.Cid,
		Part:       part.Part,
		FirstFrame: "",
		Title:      title,
		Pic:        pic,
	}
}
