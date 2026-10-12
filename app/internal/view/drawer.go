package view

import (
	"fmt"
	"strings"
	"time"

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
// drawerIn 是抽屉上滑入场 / 遮罩淡入的时长。
const drawerIn = 220 * time.Millisecond

// drawerSnap 是抽屉下滑手势的落点（ui.Draggable 的 Snap）：松手时位置在上半
// 就弹回原位（{}），被甩过中点就落到最底（{Y: h}）等着关闭。用
// ui.Nearest 而不是自己判阈值：它拿的是**带速度投影**后的位置，
// 所以快速下甩即使没拖过中点也能甩下去（原版抽屉没有这个手势，
// 这是原生版补齐的「下滑关闭」）。
func drawerSnap(h float32) func(ui.Point) ui.Point {
	return ui.Nearest(ui.Point{}, ui.Point{Y: h})
}

// drawerDismissed 报告下滑的抽屉是否该关：已经被甩到底（offset 到了 h）
// 且**停稳了**。动画途中不能关，否则抽屉会瞬间消失而不是滑下去；
// 高度退化（窗口比播放栏还矮）时也不关，否则抽屉根本打不开。
func drawerDismissed(offsetY, h float32, transitioning bool) bool {
	return h > 0 && !transitioning && offsetY >= h
}

// drawerFade 是遮罩随下滑的透明度系数：贴底时 1，拖到底时 0。
func drawerFade(offsetY, h float32) float32 {
	if h <= 0 {
		return 1
	}
	return float32(1 - clampF(float64(offsetY), 0, float64(h))/float64(h))
}

// drawerMaskFade 把「遮罩不透明度」换算成 ui.Pose.Fade 的值。
//
// 关键坑：`Pose.Fade` 与 `Opacity` **方向相反**（ui/pose.go 的文档："from
// 0, opaque, to 1, gone"，paint.go 里是 `opacity *= 1 - poseFade`），而
// drawerFade 给的是不透明度（1 = 不透明）。直接把 drawerFade 塞进 Pose{Fade}
// 会把遮罩——以及作为它子节点的整个抽屉面板——涂成透明（opacity 0），
// 抽屉根本看不见。入场时从全透明（1）起步，由 AnimatePose 渐显到 maskFade。
func drawerMaskFade(offsetY, h float32, entering bool) float32 {
	if entering {
		return 1
	}
	return 1 - drawerFade(offsetY, h)
}

// drawerEnterStep 决定抽屉这一帧是不是「入场帧」，并返回要记下的 key 与
// 时间戳（供下一帧判断）。
//
// 返回 true 时，调用方把面板 pose 摆到窗口下方（Y=h），起手帧之后再让框架
// 用 AnimatePose 把它弹回 0。
//
// 难点在**同一帧里的多次 build pass**：点击打开抽屉的那一帧，`Clicked()`
// 会当场置 `rt.consumed`，框架会把整个视图重建第二趟（input.go:895）。
// 第一趟设置了入场，第二趟若按「已经不是新抽屉」处理就会把 pose 目标改成 0，
// 而 pose 只在帧末 commit 时采样一次——于是采到 0，面板首帧即到位，入场动画
// 消失。`now` 在整帧内不变（context.go 的 c.now 每帧设一次），所以用「与
// 入场时间戳相同」识别同帧后续 pass，让起手 pose 在整个入场帧里保持 h。
func drawerEnterStep(drawer, openKey string, enterAt, now time.Time) (entering bool, nextKey string, nextAt time.Time) {
	switch {
	case drawer != openKey:
		// 换抽屉 / 首次打开 / 关掉后重开：进入入场帧。
		return true, drawer, now
	case enterAt.Equal(now):
		// 同一帧的又一次 build pass：仍是入场帧，别把起始 pose 覆盖掉。
		return true, openKey, enterAt
	}
	return false, openKey, enterAt
}

// drawerExitDone 报告退场是否该收尾：至少隔了一帧（`exitAt` 那一刻设的起始
// pose 已经被 commit 采样，否则同一帧就收尾会让面板根本没动），且面板的
// 动效已经停稳。
func drawerExitDone(exitAt, now time.Time, transitioning bool) bool {
	return !exitAt.Equal(now) && !transitioning
}

// drawerStartPose 给出面板这一帧的起始 pose（Y 偏移）以及要不要请框架再建
// 一帧（`Context.AnimationFrame`）。
//
// 入场（entering）：把手面摆到窗口下方 h 处，并请框架多建一帧——下一帧
// entering 已为假，目标才会变成 0，AnimatePose 于是从 h 弹回 0（只走重绘）。
//
// 退场（closing）：目标直接设成 h（AnimatePose 从当前位置续走下滑）。**千万
// 不能在这里也请 AnimationFrame**：`Context.AnimationFrame` 会把帧标成
// 「整个视图要重建」（ui/context.go:358 置 rt.animating），而退场期间
// closing 每帧都为真，于是每一帧都重建整个抽屉视图 + 全窗重绘——退场比入场
// 还慢（实测无头：退场 16 帧 avg present 897ms、每帧 passes=1）。AnimatePose
// 自己会走只重绘的 repose 路径（ui/pose.go:124 置 rt.repainting），不需要请帧。
func drawerStartPose(entering, closing bool, h float32) (y float32, animationFrame bool) {
	switch {
	case entering:
		return h, true
	case closing:
		return h, false
	}
	return 0, false
}

// 同一时刻只有一个抽屉打开（App.Drawer）。
func (a *App) drawer(c *ui.Context) {
	// 关抽屉时 a.Drawer 被清空，但面板不能瞬间消失：先进入退场（用最后显示
	// 的 key 继续渲染，面板下滑 + 遮罩淡出），停稳后再真正移出视图。
	if a.Drawer == "" {
		if a.drawerShown == "" {
			a.drawerOpenKey = ""
			a.drawerClosing = false
			return
		}
		if !a.drawerClosing {
			a.drawerClosing, a.drawerExitAt = true, c.Now()
			// 退场一开始就把「已起步的抽屉」清掉：退场走 AnimatePose 的
			// 只重绘路径（见 drawerStartPose），动效停稳前不会再有 build 帧，
			// `drawerExitDone` 的收尾会晚一帧。若不在这里清 drawerOpenKey，
			// 同一抽屉关掉后马上重开会被 drawerEnterStep 当成「不是新抽屉」
			// 而丢掉入场动画。
			a.drawerOpenKey = ""
		}
	} else {
		a.drawerShown, a.drawerClosing = a.Drawer, false
	}
	// 渲染用的 key：退场时用最后显示的那个。
	key := pick(a.Drawer == "", a.drawerShown, a.Drawer)

	_, winH := c.Size()
	// min(92vh, 100vh - 54px)：54 是播放栏高度，抽屉不盖住它。
	h := min(winH*0.92, winH-54)

	// 入场：换抽屉 / 重新打开时面板这一帧先停在底部之外（Y=h），下一帧再把
	// 目标设成 0 让它弹上来。退场时不参与（key 用 drawerShown）。
	var entering bool
	if !a.drawerClosing {
		if a.Drawer != a.drawerOpenKey {
			a.drawerOffset = ui.Point{}
		}
		entering, a.drawerOpenKey, a.drawerEnterAt =
			drawerEnterStep(a.Drawer, a.drawerOpenKey, a.drawerEnterAt, c.Now())
	}
	overlay := ui.Box(c).Absolute().Fill()
	overlay.Background(ui.Hex("#0f172a").Alpha(0.28))
	var dragY float32
	overlay.Children(func() {
		// 原版抽屉的 ::before 有 backdrop-filter: blur(14px) saturate(1.35)
		// （--liquid-blur-thin：小面积用薄玻璃），所以面板本身也是真玻璃。
		// 上滑入场（原版抽屉是从底部滑上来的），位移交给 AnimatePose。
		panel := ui.Column(c).Absolute().Left(0).Right(0).Bottom(0).Height(h).
			Radius(18, 18, 0, 0).
			Material(glass.Glass{}).
			// 玻璃自带顶缘聚光，不再叠原版那条手搓的 1px 顶线。
			Shadow(0, -16, 50, 0, shadowInk.Alpha(0.08))
		// **用 AnimatePose，不要用 Translate(AnimateTo)**：AnimateTo 的值要在
		// 视图里读，意味着动画的每一帧都要重建整个抽屉视图——抽屉正文很重，
		// 这正是之前入场卡顿的根因（机制同 home.go 唱片那段注释）。
		// 起始 pose 与「要不要请框架再建一帧」由 drawerStartPose 决定（入场要、
		// 退场**不要**，否则退场每帧重建，见那里的注释）。
		y, animFrame := drawerStartPose(entering, a.drawerClosing, h)
		if animFrame {
			c.AnimationFrame()
		}
		panel.AnimatePose(ui.Pose{Y: y}, ui.Snappy)
		// v0.4.5 的 Draggable：按住面板空白处（表头、边距——正文的滚动区是
		// 更内层的交互元素，不会抢走手势）往下拖，松手弹回或被甩到底关闭。
		// 偏移叠在入场 Pose 之上，所以入场动画不受影响。
		panel.Draggable(&a.drawerOffset, ui.DraggableOptions{
			Axis:   ui.Vertical,
			Bounds: &ui.Rect{Y: 0, H: h},
			Snap:   drawerSnap(h),
		})
		// 正文按 key 渲染（退场时 a.Drawer 已空，临时摆回最后显示的 key）。
		real := a.Drawer
		a.Drawer = key
		panel.Children(func() {
			a.drawerHeader(c)
			a.drawerBody(c)
		})
		a.Drawer = real
		// DragOffset 是本帧面板实际显示的偏移（跟手 / 回弹途中都算），
		// 用它让遮罩跟着下滑变淡。
		dragY = panel.DragOffset().Y
		if !a.drawerClosing && drawerDismissed(a.drawerOffset.Y, h, panel.Transitioning()) {
			// 拖拽已经把它滑出去了，下一帧走退场收尾（offset 保持，别弹回）。
			a.Drawer = ""
		}
		// 退场收尾：至少隔一帧（让刚设的 pose 先被采样），且 pose 停稳后才移出。
		if a.drawerClosing && drawerExitDone(a.drawerExitAt, c.Now(), panel.Transitioning()) {
			a.drawerShown, a.drawerClosing = "", false
			a.drawerOpenKey, a.drawerOffset = "", ui.Point{}
		}
	})
	// 遮罩是投影淡入（repaint-only）。注意 Pose.Fade 与 Opacity 方向相反：
	// 取反才不会把抽屉一起涂成透明（见 drawerMaskFade）。退场时反过来淡出。
	overlay.AnimatePose(ui.Pose{Fade: drawerMaskFade(dragY, h, entering || a.drawerClosing)},
		ui.Animation{Duration: drawerIn})
	// 点覆盖层空白处关闭（面板在它上面，点面板不会关）。退场中不再响应。
	if !a.drawerClosing && overlay.Clicked() {
		a.Drawer = ""
	}
}

// drawerHeader 是抽屉头部（`[data-slot="wrapper"] > section > header`）：
// 高 48、右侧留 44 给绝对定位的关闭键、下边框 1px rgba(100,116,139,0.14)。
// 表头里放：抽屉自己的 tab（或标题）+ 刷新键。
func (a *App) drawerHeader(c *ui.Context) {
	t := a.Theme
	// HeroUI 的 DrawerHeader/DrawerBody 都是 px-6（24），所以表头左边距
	// 与正文一致；右侧 44 给绝对定位的关闭键让位。
	ui.Row(c).FillWidth().Height(48).Shrink(0).Padding(0, 44, 0, 24).
		Gap(8).AlignItems(ui.Center).Children(func() {
		a.drawerTabs(c)
		// 原版 drawer header 默认类是 flex + px-6 py-4（无 justify / 无弹性占位），
		// 组件又不加 spacer，所以表头内容全部靠左排（刷新键紧跟在 tab 后面，
		// 不是被推到右边）。收藏抽屉例外：横向滚动的收藏夹 tab 自己 flex:1，
		// 把右侧的滚动键与刷新键顶到右边。
		if a.refreshable() {
			// 原版刷新键：20px #333（弹幕抽屉是 18px，见下）。
			refreshSize := float32(20)
			if a.Drawer == DrawerDanmaku {
				refreshSize = 18
			}
			a.headerButton(c, "drawer-refresh", "刷新", iconRefresh, refreshSize, ui.Hex("#333333"), func() {
				if a.Act.Reload != nil {
					a.Act.Reload()
				}
			})
		}
		// 原版历史抽屉的顺序是「tab」「刷新」「隐身」，隐身开关在刷新键右边。
		if a.Drawer == "history" && a.HistTab == HistHistory {
			a.incognitoSwitch(c)
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
			buttonSurface(b, false, true, ui.Color{}, ui.Hex("#ffffff").Alpha(0.30))
		}
		b.Children(func() { ui.Icon(c, iconClose).Size(14, 14).TextColor(t.Muted) })
		if b.Clicked() {
			a.Drawer = ""
		}
	})
}

// drawerTabs 画抽屉表头左边的部分：列表抽屉是 tab，其余是一个标题。
func (a *App) drawerTabs(c *ui.Context) {
	switch a.Drawer {
	case DrawerSearch:
		// 原版 searchList 表头：「搜索」标题 + 排序 tab。
		ui.Text(c, "搜索").FontSize(14).FontWeight(600).
			TextColor(ui.Hex("#334155")).Shrink(0)
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
		// 隐身开关（只在「观看历史」页）由 drawerHeader 在刷新键右边渲染，
		// 顺序与原版一致（tab → 刷新 → 隐身）。
	case "favorite":
		// 原版 collectList 表头文案「收藏列表」（14/600 #334155，字重由 header 的
		// 全局规则统一给出）。
		ui.Text(c, "收藏列表").FontSize(14).FontWeight(600).
			TextColor(ui.Hex("#334155")).Shrink(0)
		if len(a.Folders) == 0 {
			a.emptyNote(c, "暂无收藏内容")
			break
		}
		// 收藏夹可能很多：左右滚动键 + 横向滚动（原版 collect-scroll-btn，
		// 每次 200px）。
		a.collectScrollBtn(c, "collect-left", "向左滚动", -200)
		ui.ScrollHorizontal(c).Grow(1).MinWidth(0).Height(32).
			TrackScroll(&a.CollectScroll).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				for _, f := range a.Folders {
					a.folderTab(c, f)
				}
			})
		})
		a.collectScrollBtn(c, "collect-right", "向右滚动", 200)
	case "feed":
		ui.Text(c, "动态列表").FontSize(14).FontWeight(600).TextColor(ui.Hex("#334155"))
	case DrawerUp:
		a.upHeader(c)
	case DrawerParts:
		a.partsHeader(c)
	case DrawerDanmaku:
		a.danmakuTabs(c)
	case DrawerPlaylist:
		a.playlistTabs(c)
	case DrawerSeries:
		ui.Text(c, a.seriesLabel()).FontSize(14).FontWeight(600).SingleLine().
			TextColor(ui.Hex("#334155")).Shrink(0)
		// 旧版的「播放全部」：把整个合集当队列从第一条开始放。
		if b := ui.ButtonBase(c.Key("series-play-all")).Height(32).Padding(0, 12).
			Radius(Radius).Center().Label("播放全部").Tooltip("播放合集中的全部视频"); true {
			buttonSurface(b, false, b.Hovered(),
				ui.Color{}, ui.Hex("#ffffff").Alpha(0.30))
			b.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconPlay).Size(16, 16).TextColor(ui.Hex("#666666"))
					ui.Text(c, "播放全部").FontSize(12).TextColor(ui.Hex("#475569"))
				})
			})
			if b.Clicked() && a.Act.SeriesPlayAll != nil {
				a.Act.SeriesPlayAll()
			}
		}
	default:
		ui.Text(c, a.drawerTitle()).FontSize(14).FontWeight(600).
			TextColor(ui.Hex("#334155"))
	}
}

// upHeader 是 UP 空间抽屉的表头（原版 upVideoList.tsx）：
// 「{名字}」的空间 + 关注 / 已关注 + 视频 / 合集 tab。
func (a *App) upHeader(c *ui.Context) {
	// 原版 upVideoList 表头：「name」的空间（14/600 #334155，见 header 全局规则）
	// + 粉丝数（12 default-500）+ 关注键（min-w 70、图标 14）。
	ui.Text(c, fmt.Sprintf("「%s」的空间", a.UpName)).FontSize(14).FontWeight(600).
		SingleLine().MaxWidth(200).TextColor(ui.Hex("#334155")).Shrink(0)
	if a.UpFans > 0 {
		ui.Text(c, "粉丝 "+fanCount(a.UpFans)).FontSize(12).
			TextColor(ui.Hex("#71717a")).Shrink(0)
	}

	label := pick(a.UpFollowed, "已关注", "关注")
	b := ui.ButtonBase(c.Key("up-follow")).MinWidth(70).Height(32).Padding(0, 12).
		Radius(Radius).Center().Label(label)
	buttonSurface(b, false, b.Hovered(),
		ui.Color{}, ui.Hex("#ffffff").Alpha(0.30))
	b.Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			// 原版：关注态是 Close #666 + 默认字色；未关注是 Add + text-primary。
			ui.Icon(c, pick(a.UpFollowed, iconFollowed, iconFollow)).Size(14, 14).
				TextColor(pick(a.UpFollowed, ui.Hex("#666666"), ui.Hex("#2563eb")))
			ui.Text(c, label).FontSize(14).
				TextColor(pick(a.UpFollowed, ui.Hex("#3f3f46"), ui.Hex("#2563eb")))
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
	// 原版 .history-incognito-switch：min-width 92、高 32、gap 7、padding 0 8、
	// 圆角 8、1px 边 rgba(100,116,139,.2)、底 rgba(255,255,255,.34)，
	// 内高光 + 淡投影；开启时转青（#0e7490 / 底 rgba(207,250,254,.5)）。
	b := ui.ButtonBase(c.Key("incognito")).MinWidth(92).Height(32).Padding(0, 8).
		Radius(Radius).Label("隐身").Tooltip("开启后不读取或上报云端播放记录与进度")
	// 半透明填充；开启隐身时用青色底表达（原版是 rgba(207,250,254,.5) 底）。
	buttonSurface(b, a.Incognito, b.Hovered(),
		ui.Hex("#06b6d4").Alpha(0.28), ui.Hex("#ffffff").Alpha(0.30)).
		Shadow(0, 2, 7, 0, ui.Hex("#0f172a").Alpha(0.05))
	ink := pick(a.Incognito, ui.Hex("#0e7490"), ui.Hex("#64748b"))
	b.Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconMask).Size(15, 15).TextColor(ink)
			// .history-incognito-label：12px / 650 / line-height 1。
			ui.Text(c, "隐身").FontSize(12).FontWeight(650).TextColor(ink)
			// 轨道 27×16（padding 2、圆角 999、内凹影），滑块 12，打开时右移 11。
			ui.Box(c).Width(27).Height(16).Shrink(0).Radius(RadiusPill).Padding(2).
				InsetShadow(0, 1, 2, 0, ui.Hex("#0f172a").Alpha(0.10)).
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

// refreshable 返回这个抽屉的表头有没有刷新键。原版里搜索抽屉（searchList）、
// 选集（pageList）、合集（seriesList）、播放列表（playlist）都没有刷新键，
// 其余列表抽屉有。
func (a *App) refreshable() bool {
	switch a.Drawer {
	case DrawerSearch, DrawerParts, DrawerSeries, DrawerPlaylist:
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
	// Icon 是可选的前置图标（原版弹幕/评论 tab 有 14px 图标，其余 tab 纯文字）。
	Icon *ui.SVG
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
	b := ui.ButtonBase(c.Key("tab-"+group+"-"+it.Key)).Height(32).Padding(0, 14).
		Radius(Radius).Center().Label(it.Label)
	// 半透明填充（选中态更实一点 = 底色更亮），不再用 Backdrop 真玻璃。
	buttonSurface(b, active, b.Hovered(),
		ui.Hex("#ffffff").Alpha(0.52), ui.Hex("#ffffff").Alpha(0.30))
	pressPose(b)
	b.Children(func() {
		// 原版 tab：13px / 600；选中 #0369a1，悬停（未选中）#334155。
		ink := ui.Hex("#475569")
		if active {
			ink = ui.Hex("#0369a1")
		} else if b.Hovered() {
			ink = ui.Hex("#334155")
		}
		if it.Icon != nil {
			// 原版是 flex items-center gap-1（4px）+ 14px 图标。
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, it.Icon).Size(14, 14).TextColor(ink)
				ui.Text(c, it.Label).FontSize(13).FontWeight(600).TextColor(ink)
			})
			return
		}
		ui.Text(c, it.Label).FontSize(13).FontWeight(600).TextColor(ink)
	})
	if b.Clicked() && !active {
		onSelect(it.Key)
	}
}

// folderTab 是一个收藏夹 tab（原版显示「标题 (条数)」）。
func (a *App) folderTab(c *ui.Context, f Folder) {
	active := f.ID == a.FolderID
	label := fmt.Sprintf("%s (%d)", f.Title, f.Count)
	b := ui.ButtonBase(c.Key(fmt.Sprintf("folder-%d", f.ID))).Height(32).Padding(0, 14).
		Radius(Radius).Center().Label(label)
	buttonSurface(b, active, b.Hovered(),
		ui.Hex("#ffffff").Alpha(0.52), ui.Hex("#ffffff").Alpha(0.30))
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
// headerButton 是抽屉表头的图标按钮。原版各按钮的图标尺寸/颜色并不统一：
// 刷新 20px #333、定位/播放模式/清空 18px #888，弹幕抽屉的刷新是 18px。
func (a *App) headerButton(c *ui.Context, key, label string, ic *ui.SVG, size float32, ink ui.Color, fn func()) {
	b := ui.ButtonBase(c.Key(key)).Size(32, 32).Radius(Radius).Center().Label(label).Tooltip(label)
	buttonSurface(b, false, b.Hovered(),
		ui.Color{}, ui.Hex("#ffffff").Alpha(0.30))
	pressPose(b)
	b.Children(func() { ui.Icon(c, ic).Size(size, size).TextColor(ink) })
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
// 卡片列表（搜索/分区/动态等）与弹幕列表都用 mygo 的虚拟化 ui.List 自带
// 滚动（只建可见行）；其余内容不多，统一套一层 ui.Scroll。
func (a *App) drawerBody(c *ui.Context) {
	l := a.ListFor(a.Drawer)
	if a.Drawer == DrawerDanmaku {
		if a.DanmakuTab == TabDanmaku {
			// 弹幕 tab 自己用虚拟化 ui.List 当滚动容器（见 danmakuBody）。
			a.danmakuBody(c)
			return
		}
		// 原版 .danmaku-scroll-area 的 padding 是 8px 10px 16px 10px
		// （上下不对称，底部多留 16 给最后一行）。
		ui.Scroll(c).FillWidth().Grow(1).Padding(8, 10, 16, 10).TrackScroll(&l.Scroll).
			Children(func() { a.repliesBody(c) })
		if a.RepliesHasMore && !a.RepliesLoading &&
			l.Scroll.MaxY > 0 && l.Scroll.Y >= l.Scroll.MaxY-100 && a.Act.LoadComments != nil {
			a.Act.LoadComments(a.ReplyPage + 1)
		}
		return
	}
	// 卡片网格：虚拟化列表自己滚，顺带负责到底翻页。
	if a.usesCardGrid() {
		// 退场时不再翻页（内容马上就看不见了）。
		if a.listBody(c) && !a.drawerClosing && l.HasMore && !l.Loading && a.Act.LoadMore != nil {
			// 内容超过一屏且滚到底（first>0）才翻页：与旧逻辑的 MaxY>0 一致，
			// 避免首屏不满却反复请求空页。
			if first, last := l.Grid.Visible(); first > 0 && last >= a.gridRows()-1 {
				a.Act.LoadMore()
			}
		}
		return
	}

	// 其余抽屉内容不多，统一套一层滚动。
	//
	// 注意 TrackScroll 是双向绑定：容器每帧从这个状态读位置、滚动时写回。
	// 所以必须传**稳定地址**（List 常驻在 map 里），不能按值拷进出 ——
	// 拷贝会让容器每帧被旧位置拽回去，表现为触摸板/滚轮滚不动。
	ui.Scroll(c).FillWidth().Grow(1).Padding(8, 24).TrackScroll(&l.Scroll).Children(func() {
		switch a.Drawer {
		case DrawerParts:
			a.partsBody(c)
		case DrawerUp:
			// UP 空间有两个 tab：视频是卡片列表（上面已处理），合集是一排胶囊。
			a.seriesListBody(c)
		case DrawerPlaylist:
			a.playlistBody(c)
		}
	})
}

// usesCardGrid 报告当前抽屉是否是三列卡片网格（搜索/分区/UP 视频/合集）。
// 选集、合集列表、播放列表内容少，仍走 ui.Scroll。
func (a *App) usesCardGrid() bool {
	switch a.Drawer {
	case DrawerParts, DrawerPlaylist, DrawerDanmaku:
		return false
	case DrawerUp:
		return a.UpTab != UpTabSeries
	}
	return true
}

// gridRows 是当前卡片网格的行数（每行 cardGridCols 张）。
func (a *App) gridRows() int {
	n := len(a.list().Cards)
	return (n + cardGridCols - 1) / cardGridCols
}

// seriesListBody 是 UP 空间的「合集」tab（原版 upVideoList.tsx 的合集列表）：
// 一排下拉的合集胶囊，点一个就进那个合集。
func (a *App) seriesListBody(c *ui.Context) {
	switch {
	case a.list().Loading && len(a.SeriesList) == 0:
		a.emptyNote(c, "加载中…")
		return
	case len(a.SeriesList) == 0:
		a.emptyNote(c, "暂无合集视频")
		return
	}
	// 原版 UP 空间的合集 tab：纵向排列的卡片（flex flex-col gap-2），
	// 每张卡片 padding 8，标题 14px bold 两行截断 + 「N 个视频」12px default-500。
	ui.Column(c).Gap(8).Children(func() {
		for _, s := range a.SeriesList {
			a.seriesChip(c, s)
		}
	})
}

// seriesChip 是一个合集胶囊：封面缩略图 + 「名字 (条数)」。
func (a *App) seriesChip(c *ui.Context, s Series) {
	t := a.Theme
	active := s.ID == a.SeriesID
	// 原版是抽屉里的 Card（与视频卡片同一套半透明填充），当前合集加 2px 主色边。
	b := ui.ButtonBase(c.Key(fmt.Sprintf("series-%d", s.ID))).FillWidth().
		Padding(8).Radius(12).AlignItems(ui.Start)
	// 与列表卡片同一套半透明填充（不是 Backdrop 真玻璃）+ 当前项 2px 主色边。
	switch {
	case active:
		b.Background(ui.Hex("#006FEE").Alpha(0.18)).
			Border(2, ui.Hex("#006FEE"))
	case b.Hovered():
		b.Background(t.GlassHover).
			Border(1, ui.Hex("#0369a1").Alpha(0.60)).
			Shadow(0, 10, 28, 0, ui.Hex("#0f172a").Alpha(0.12))
	default:
		b.Background(t.Glass).
			Border(1, t.GlassBorder).
			InsetShadow(0, 1, 1, 0, t.Highlight).
			Shadow(0, 4, 12, 0, ui.Hex("#0f172a").Alpha(0.04))
	}
	b.Children(func() {
		ui.Column(c).FillWidth().Children(func() {
			// <b className="line-clamp-2 text-sm">：14px bold、两行截断。
			ui.Text(c, s.Title).FontSize(14).FontWeight(700).
				TextColor(t.Ink).MaxLines(2)
			// <p className="text-xs text-default-500 mt-1">。
			ui.Text(c, fmt.Sprintf("%d 个视频", s.Count)).FontSize(12).
				Margin(4, 0, 0, 0).TextColor(ui.Hex("#71717a"))
		})
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

// cardGridCols 是卡片网格的列数（原版抽屉网格 grid-cols-2 sm:grid-cols-3；
// 抽屉里卡片窄，始终按 3 列排）。
const cardGridCols = 3

// listBody 是搜索结果 / 分区 / UP 视频 / 合集等卡片网格。返回是否画了真正的
// 卡片（供 drawerBody 决定要不要检查翻页；骨架/空态不算）。
func (a *App) listBody(c *ui.Context) bool {
	l := a.list()
	switch {
	case l.Loading && len(l.Cards) == 0:
		a.scrollBody(c, l, func() { a.skeletonGrid(c) })
		return false
	case len(l.Cards) == 0:
		a.scrollBody(c, l, func() { a.emptyNote(c, a.emptyHint()) })
		return false
	}
	a.cardGrid(c, &l.Grid, l.Cards)
	return true
}

// scrollBody 把短小的内容（骨架屏 / 空态）套一层 ui.Scroll。
func (a *App) scrollBody(c *ui.Context, l *List, body func()) {
	ui.Scroll(c).FillWidth().Grow(1).Padding(8, 24).TrackScroll(&l.Scroll).
		Children(body)
}

// cardGrid 用 mygo 的虚拟化 ui.List 画三列卡片网格：每个 List 行是一排最多
// cardGridCols 张卡，行高由框架量（卡片封面 100 + 单行标题，所以每排等高）。
//
// 之前这里是 ui.Column 逐行构建：滚动时视图每帧重建，会把**所有**卡片（含
// 几百张）连同各自的材质一起重建——这是「订阅/动态」等列表滚动卡顿的根因
// （与上一轮弹幕列表恢复 ui.List 同一机制，见 danmakuBody）。ui.List 只建
// 可见的几排。
func (a *App) cardGrid(c *ui.Context, st *ui.ListState, cards []Card) {
	// 行键取首张卡的 bvid：翻页追加 / 滑动窗口裁掉头部时锚点不跳。
	st.Key = func(r int) any {
		if i := r * cardGridCols; i < len(cards) && cards[i].Bvid != "" {
			return cards[i].Bvid
		}
		return r
	}
	rows := (len(cards) + cardGridCols - 1) / cardGridCols
	ui.List(c, st, rows, func(r int) {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
			start := r * cardGridCols
			end := min(start+cardGridCols, len(cards))
			for j := start; j < end; j++ {
				a.card(c, j)
			}
			for j := end; j < start+cardGridCols; j++ {
				ui.Box(c).Grow(1).Basis(0)
			}
		})
	}).FillWidth().Grow(1).Gap(8).Padding(8, 24)
}

// skeletonGrid 是列表加载中的骨架屏（原版 ListSkeleton）：灰底占位卡片。
func (a *App) skeletonGrid(c *ui.Context) {
	// 原版 .list-skeleton-*：卡片底 rgba(255,255,255,.4)、封面 16:10 圆角 8、
	// 两行文本条（12px 圆角 6，第二行 60% 宽），opacity 1.4s 脉冲。
	// mygo 没有 opacity 动画，用底色的深浅交替近似（帧驱动）。
	cover := ui.Hex("#64748b").Alpha(0.16)
	line := ui.Hex("#64748b").Alpha(0.16)
	card := ui.Hex("#ffffff").Alpha(0.40)
	const cols = 3
	ui.Column(c).Gap(8).Children(func() {
		for row := 0; row < 3; row++ {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
				for j := 0; j < cols; j++ {
					// 原版是 1.4s 的 opacity 脉冲（.list-skeleton-pulse：
					// 0.55 ↔ 1）。用 Loop 的进度做三角波驱动透明度。
					item := ui.Column(c).Key(fmt.Sprintf("skel-%d-%d", row, j)).
						Grow(1).Basis(0).Padding(8).Radius(12).Background(card).Gap(0)
					ph := float64(item.Loop("skel", 1400*time.Millisecond, ui.Linear))
					tri := 1 - abs(2*ph-1) // 0 → 1 → 0
					item.Opacity(float32(0.55 + 0.45*tri))
					item.Children(func() {
						ui.Box(c).FillWidth().AspectRatio(16.0 / 10.0).Radius(8).
							Background(cover)
						ui.Box(c).FillWidth().Height(12).Margin(8, 0, 0, 0).Radius(6).
							Background(line)
						ui.Box(c).WidthPercent(60).Height(12).Margin(8, 0, 0, 0).
							Radius(6).Background(line)
					})
				}
			})
		}
	})
}

// emptyNote 是列表空态/加载中的提示行：居中、上下各 32px 留白、slate-400
// （原版是 text-center text-default-400 py-8，而 default-400 在抽屉里被
// globals.css 覆写成 #94a3b8）。之前是左对齐一行小字，在抽屉里显得很空。
func (a *App) emptyNote(c *ui.Context, text string) {
	ui.Row(c).FillWidth().Padding(32, 0).Justify(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(14).TextColor(ui.Hex("#94a3b8"))
	})
}

// emptyHint 是列表为空时的提示。需要登录的分区（以及「推荐」这种未登录必空的 tab）
// 提示去登录，而不是让用户看白屏。
func (a *App) emptyHint() string {
	if a.needsLogin() {
		return "这个分区需要登录，点右上角头像扫码登录"
	}
	// 空态文案对齐原版各列表组件。
	switch a.Drawer {
	case DrawerSearch:
		return "没有找到相关视频"
	}
	if s := a.CurrentSection(); s.Key == a.Drawer {
		switch a.Drawer {
		case "favorite":
			return "暂无收藏内容"
		case "feed":
			return "暂无动态内容"
		case "history":
			if a.HistTab == HistWatchLater {
				return "暂无稍后再看视频"
			}
			return "暂无观看历史"
		case "popular":
			if a.RecTab == RecRecommend {
				return "暂无推荐内容"
			}
			return "暂无热门内容"
		}
	}
	if a.Drawer == DrawerUp {
		if a.UpTab == UpTabSeries {
			return "暂无合集视频"
		}
		return "该UP主暂无视频"
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
	card := a.list().Cards[index]
	playing := a.Current() != nil && card.Bvid != "" && a.Current().Bvid == card.Bvid
	spec := cardSpec{
		Key:      fmt.Sprintf("card-%d-%s", index, card.Bvid),
		Cover:    card.Cover,
		Title:    card.Title,
		Duration: card.Duration,
		Active:   playing,
		Playing:  playing,
		// 三列比例：作者 40% / 播放量 30% / 发布时间 30%；卡片窄了从末尾
		// 依次隐藏（阈值与原版的容器查询一致）。
		Fields: []cardField{
			{Kind: fieldAuthor, Text: card.Up},
			{Kind: fieldViews, Text: card.Views, DanmakuIcon: card.ViewsDanmaku},
			{Kind: fieldPubdate, Text: card.Pubdate},
			{Kind: fieldExtra, Text: card.Extra},
		},
		OnClick: func() {
			// 与原版一致：点卡片只是打开选集面板，不直接起播。
			if a.Act.OpenCard != nil {
				a.Act.OpenCard(index)
			}
		},
	}
	// 「稍后再看」列表在封面上多一个移除键（原版 historyList）。
	if a.Drawer == "history" && a.HistTab == HistWatchLater && card.Track.Aid != 0 {
		aid := card.Track.Aid
		spec.Cover2 = func(c *ui.Context) {
			// 请求中的置灰防连点（原版 pendingAids）。
			pending := a.WLRemovePending != nil && a.WLRemovePending[aid]
			b := ui.ButtonBase(c.Key("wl-remove-"+card.Bvid)).Size(22, 22).Radius(4).
				Center().Label("从稍后再看移除").Tooltip("从稍后再看移除")
			b.Disabled(pending)
			b.Absolute().Right(4).Top(4)
			b.Background(ui.Hex("#0f172a").Alpha(
				pick(b.Hovered(), float32(0.62), float32(0.45)))).
				Border(1, ui.Hex("#ffffff").Alpha(0.20))
			b.Children(func() {
				ui.Icon(c, iconClose).Size(12, 12).TextColor(ui.Hex("#ffffff"))
			})
			if b.Clicked() && !pending && a.Act.RemoveWatchLater != nil {
				a.Act.RemoveWatchLater(aid)
			}
		}
	}
	a.renderCard(c, spec)
}

// cardFieldKind 是卡片 meta 行的字段类型。它决定列的比例 / 伸展 / 对齐与
// 窄卡隐藏阈值——这些全部对齐原版 globals.css 的 .card-meta-field.is-* 规则。
type cardFieldKind string

const (
	fieldAuthor   cardFieldKind = "author"
	fieldViews    cardFieldKind = "views"
	fieldPubdate  cardFieldKind = "pubdate"
	fieldDuration cardFieldKind = "duration"
	fieldExtra    cardFieldKind = "extra"
)

// cardField 是卡片 meta 行里的一个字段。
type cardField struct {
	Kind cardFieldKind
	Text string
	// DanmakuIcon 为真表示这是「播放量缺失、用弹幕数顶上」的回退，前导图标
	// 用评论图标（原版 viewsMetaField 的语义）。仅 views 字段有意义。
	DanmakuIcon bool
}

// fieldStyle 返回某类字段的列样式，取自原版 globals.css：
//
//	author   40% / grow 4 / 左对齐 / 永不隐藏
//	views    30% / grow 3 / 右对齐 / 窄于 150 隐藏
//	pubdate  30% / grow 4 / 右对齐 / 窄于 200 隐藏（剩余宽可「借」给日期）
//	duration 30% / grow 3 / 右对齐 / 窄于 240 隐藏
//	extra    30% / grow 3 / 右对齐 / 窄于 260 隐藏
func fieldStyle(kind cardFieldKind) (basis, grow, minW float32, align ui.Align) {
	switch kind {
	case fieldAuthor:
		return 40, 4, 0, ui.Start
	case fieldViews:
		return 30, 3, 150, ui.End
	case fieldPubdate:
		return 30, 4, 200, ui.End
	case fieldDuration:
		return 30, 3, 240, ui.End
	case fieldExtra:
		return 30, 3, 260, ui.End
	}
	return 30, 3, 0, ui.End
}

// resolveFieldWidths 按 flex（basis% + grow 分配剩余空间）算出每个字段的
// 实际列宽，供发布时间的「按实测宽度降级」使用。被隐藏的字段宽度为 0。
func resolveFieldWidths(fields []cardField, rowW float32) []float32 {
	widths := make([]float32, len(fields))
	var basisSum, growSum float32
	for i, f := range fields {
		if f.Text == "" {
			continue
		}
		basis, grow, minW, _ := fieldStyle(f.Kind)
		if rowW < minW {
			continue
		}
		w := rowW * basis / 100
		widths[i] = w
		basisSum += w
		growSum += grow
	}
	free := rowW - basisSum
	if free > 0 && growSum > 0 {
		for i, f := range fields {
			if widths[i] == 0 {
				continue
			}
			_, grow, _, _ := fieldStyle(f.Kind)
			widths[i] += free * grow / growSum
		}
		return widths
	}
	if free < 0 {
		// 四列时 basis 之和 130% > 行宽：CSS flex-shrink 按 basis 比例收缩（
		// 各列 shrink=1、min-width:0）到恰好填满行宽。
		var shrinkSum float32
		for _, w := range widths {
			shrinkSum += w
		}
		if shrinkSum > 0 {
			for i := range widths {
				widths[i] *= rowW / shrinkSum
			}
		}
	}
	return widths
}

// cardSpec 是一张列表卡片的内容。
type cardSpec struct {
	Key      string
	Cover    string
	Title    string
	Duration string
	Fields   []cardField
	// Active 表示「正在播放」：蓝 Tint + 2px 蓝边。
	Active  bool
	Playing bool
	// Cover2 是叠在封面上的附加内容（添加键、移除键…）。
	Cover2  func(*ui.Context)
	OnClick func()
	// Scroll 为真时把卡片滚进视野（定位当前播放）。
	Scroll bool
}

// renderCard 画一张列表卡片。
//
// 搜索结果 / 推荐 / 收藏 / 稍后再看 / UP 空间 / **选集**……全走这一套：
// 原版就是一个 ListCard 组件，各列表只传不同的 fields 与 coverChildren。
// 之前选集自己另写了一份（padding 6、圆角 8、封面 16:9），和别的列表对
// 不上 —— 现在合成一份，样式不再碎片化。
func (a *App) renderCard(c *ui.Context, spec cardSpec) {
	t := a.Theme
	// 原版抽屉卡片（globals.css 的 [role=button][class*=bg-content]）：
	// 圆角 12、半透明填充（近似玻璃底）、封面通栏（.img-container 高 100）、
	// 页脚另有一层浅底。
	box := ui.Column(c).Key(spec.Key).Grow(1).Basis(0).
		Radius(12).Clip().Cursor(ui.CursorPointer)
	// 卡片是**半透明填充**，不是真玻璃（每张卡一次 backdrop 重算）。
	//
	// 原版 globals.css `[data-slot="wrapper"] [role="button"][class*="bg-content"]`
	// 用的是 `background: var(--glass-bg)`（半透明白 0.24）+ 1px 玻璃边 +
	// inset 高光/内影 + 小投影，**整条规则里没有 backdrop-filter**——卡片浮在抽屉
	// 面板（面板才是有 backdrop-filter 的真玻璃）的模糊之上。
	// 之前这里误用了框架的 `glass.Glass{}`（Backdrop 后处理）：每张可见卡都多出
	// 一次 backdrop 采样/模糊（macOS 上每次一个 render pass + 目标 reload），
	// 一屏 12 张卡就是 12 次——这是抽屉滚动/入场卡顿的主要来源（实测抽屉帧的
	// Backdrop 效果 24 个里 12 个来自卡片）。改回原版的填充近似后与抽屉面板的
	// 模糊叠一起，观感一致而少 12 次 backdrop 重算。
	switch {
	case spec.Active:
		// 原版选集卡片的在播态就是 border-2 border-[#0284c7]。
		box.Background(t.Blue.Alpha(0.22)).
			Border(2, ui.Hex("#0284c7")).
			Shadow(0, 10, 28, 0, ui.Hex("#0f172a").Alpha(0.12))
	case box.Hovered():
		box.Background(t.GlassHover).
			Border(1, ui.Hex("#0369a1").Alpha(0.60)).
			InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.80)).
			Shadow(0, 0, 0, 2, t.Blue.Alpha(0.20)).
			Shadow(0, 10, 28, 0, ui.Hex("#0f172a").Alpha(0.12))
	default:
		box.Background(t.Glass).
			Border(1, t.GlassBorder).
			InsetShadow(0, 1, 1, 0, t.Highlight).
			InsetShadow(0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.04)).
			Shadow(0, 4, 12, 0, ui.Hex("#0f172a").Alpha(0.04))
	}
	// 悬停「抬起」：原版是 transform: translateY(-3px)（0.16s ease）。
	// v0.4.3 的 Transforms 对任意元素生效，用弹簧做，中途移开会顺着速度回去。
	lift := float32(0)
	if box.Hovered() || spec.Active {
		lift = -3
	}
	box.Translate(0, box.AnimateTo("lift", lift, ui.Snappy))
	if spec.Scroll {
		box.ScrollIntoView()
	}

	box.Children(func() {
		// 封面：通栏、固定 100px（原版 .img-container / .c-cover）。
		ui.Box(c).FillWidth().Height(100).Shrink(0).
			Background(t.CoverPlaceholder()).Clip().Children(func() {
			if bmp := a.Images.Bitmap(spec.Cover); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			}
			// 时长角标（原版 .c-cover-duration）：右下贴角，左上圆角 5。
			if spec.Duration != "" {
				ui.Text(c, spec.Duration).FontSize(11).TextColor(ui.Hex("#f8fafc")).
					Absolute().Right(0).Bottom(0).
					Padding(2, 5).Radius(5, 0, 0, 0).Background(ui.Hex("#0f172a").Alpha(0.68))
			}
			if spec.Playing {
				a.playingBadge(c)
			}
			if spec.Cover2 != nil {
				spec.Cover2(c)
			}
		})

		// 页脚（原版 [data-slot=footer]）：px-2 py-1，只铺很薄一层白洗色 +
		// 顶部一线分隔，让卡片玻璃透出来。
		ui.Column(c).FillWidth().Grow(1).Padding(4, 8).
			Background(ui.Hex("#ffffff").Alpha(0.12)).
			InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.18)).
			Children(func() {
				// 原版标题：text-small（14）/ bold / 单行截断。
				ui.Text(c, spec.Title).FontSize(14).FontWeight(700).
					TextColor(t.Ink).SingleLine()
				ui.Box(c).FillWidth().Margin(4, 0, 0, 0).Children(func() {
					a.cardMeta(c, spec.Key, spec.Fields)
				})
			})
	})
	if box.Clicked() && spec.OnClick != nil {
		spec.OnClick()
	}
}

// cardMeta 是卡片的 meta 行（原版 CardMeta + globals.css 里的容器查询）。
//
// 列间距进列自身的左内边距（原版 `+ .card-meta-field { padding-left: 8px }`），
// 这样比例正好落在整行上。列比例 / 伸展 / 对齐 / 隐藏阈值均取自原版
// `.card-meta-field.is-*` 规则；播放量带前导图标；发布时间按**实测列宽**
// 在「具体 → 抽象」的形态阶梯里选第一个放得下的（原版轮 30 的 PubDateField）。
//
// 宽度取**上一帧的真实布局宽度**（Element.Bounds）；首帧还没有布局，
// 按窗口宽度估一个。
func (a *App) cardMeta(c *ui.Context, key string, fields []cardField) {
	row := ui.Row(c).Key(key + "-meta").FillWidth().AlignItems(ui.Center)
	w := row.Bounds().W
	if w <= 0 {
		w = estimateCardWidth(c)
	}
	widths := resolveFieldWidths(fields, w)
	now := time.Now()
	ink := ui.Hex("#71717a") // 原版 text-default-500（zinc-500）
	row.Children(func() {
		first := true
		for i, f := range fields {
			if f.Text == "" {
				continue
			}
			basis, grow, minW, align := fieldStyle(f.Kind)
			if w < minW {
				continue
			}
			avail := widths[i]
			col := ui.Box(c).BasisPercent(basis).Grow(grow).Shrink(1).MinWidth(0)
			if f.Kind == fieldAuthor {
				// 原版 .is-author 的 max-width: 40%：两列行里作者列不得超四成。
				col.MaxWidthPercent(40)
			}
			if !first {
				col.Padding(0, 0, 0, 8)
				avail -= 8
			}
			first = false
			col.Children(func() {
				switch f.Kind {
				case fieldViews:
					icon := iconPlay
					if f.DanmakuIcon {
						icon = iconDanmaku
					}
					ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.Center).Gap(2).Children(func() {
						// 原版 .card-meta-field svg { opacity: .72 }。
						ui.Icon(c, icon).Size(12, 12).TextColor(ink.Alpha(0.72))
						ui.Text(c, f.Text).FontSize(12).TextColor(ink).SingleLine()
					})
				case fieldPubdate:
					text := pubdateFit(c, f.Text, avail, now)
					ui.Text(c, text).FontSize(12).TextColor(ink).SingleLine().
						FillWidth().TextAlign(align)
				default:
					ui.Text(c, f.Text).FontSize(12).TextColor(ink).SingleLine().
						FillWidth().TextAlign(align)
				}
			})
		}
	})
}

// pubdateFit 在发布时间形态阶梯里挑第一个**实测宽度**放得下的形态
// （原版 PubDateField 的降级）。avail 是该列的可用文本宽。
func pubdateFit(c *ui.Context, value string, avail float32, now time.Time) string {
	cands := pubdateCandidates(value, now)
	if len(cands) <= 1 {
		return value
	}
	return pickDateForm(cands, func(s string) bool {
		w, _ := c.MeasureText(0, ui.Span{Text: s, Size: 12})
		return w <= avail
	})
}

// playingBadge 是封面左上角的「正在播放」胶囊（原版 .part-playing-badge）：
// 22 高、圆角 999、深玻璃底 rgba(15,23,42,.55) + 1px 白边，左侧三根均衡器小柱。
func (a *App) playingBadge(c *ui.Context) {
	ui.Row(c).Absolute().Left(6).Top(6).Height(22).Padding(0, 8, 0, 7).
		Radius(RadiusPill).Gap(5).AlignItems(ui.Center).
		Background(ui.Hex("#0f172a").Alpha(0.55)).
		Border(1, ui.Hex("#ffffff").Alpha(0.20)).Children(func() {
		ui.Row(c).Gap(2).AlignItems(ui.End).Height(10).Children(func() {
			for _, h := range []float32{4, 9, 6} {
				ui.Box(c).Width(2).Height(h).Radius(1).
					Background(ui.Hex("#7dd3fc")).AlignSelf(ui.End)
			}
		})
		ui.Text(c, "正在播放").FontSize(11).FontWeight(600).
			TextColor(ui.Hex("#e2e8f0"))
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
	// 原版 pageList 表头：「选集(N)」是 text-sm（14/600，见 header 全局规则）。
	ui.Text(c, fmt.Sprintf("选集(%d)", n)).FontSize(14).FontWeight(600).
		TextColor(ui.Hex("#334155")).Shrink(0)

	if a.Track != nil {
		a.headerButton(c, "parts-locate", "定位到当前播放的位置", iconLocate, 18, ui.Hex("#888888"), func() {
			a.locateNow = true
		})
	}
	// 选集搜索（原版 header .part-search）：flex 0 1 180px（不撑满）、
	// 高 32、圆角 8、内边距 0 10、gap 6，内凹底（rgba(15,23,42,.035)）。
	ui.Row(c).Width(180).MaxWidth(180).Shrink(1).Height(32).Radius(Radius).
		Gap(6).Padding(0, 10).
		Background(ui.Hex("#0f172a").Alpha(0.035)).
		Border(1, ui.Hex("#475569").Alpha(0.50)).
		AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconMagnifier).Size(15, 15).TextColor(t.Faint)
		in := ui.TextInputBase(c, &a.PartSearch).Grow(1).FontSize(12).
			TextColor(ui.Hex("#334155")).Placeholder("搜索选集，回车跳转").Label("搜索选集")
		if in.Submitted() {
			a.locatePart(a.PartSearch)
		}
	})
	if n > 0 {
		b := ui.ButtonBase(c.Key("parts-add-all")).Height(32).Padding(0, 12).
			Radius(Radius).Center().Label("全部添加")
		buttonSurface(b, false, b.Hovered(),
			ui.Color{}, ui.Hex("#ffffff").Alpha(0.30))
		b.Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconAdd).Size(16, 16).TextColor(ui.Hex("#475569"))
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
	var parts []Part
	if a.Info != nil {
		parts = a.Info.Parts
	}
	if len(parts) == 0 {
		a.emptyNote(c, "没有分集信息")
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
	active := a.Track != nil && a.Track.Cid == p.Cid
	inList := a.playlistHas(p.Cid)
	cover := p.FirstFrame
	if cover == "" && a.Info != nil {
		cover = a.Info.Pic
	}
	dur := ""
	if p.Duration > 0 {
		dur = fmtTime(float64(p.Duration))
	}
	// 原版选集卡片就是同一个 ListCard，只是 fields 换成
	// [{播放量}, {时长}]、封面叠「正在播放」+「添加到播放列表」。
	spec := cardSpec{
		Key:      fmt.Sprintf("part-%d", p.Cid),
		Cover:    cover,
		Title:    p.Part,
		Duration: dur,
		Active:   active,
		Playing:  active,
		Fields: []cardField{
			{Kind: fieldViews, Text: statText(a.Info, func(i *Info) int64 { return i.View })},
			{Kind: fieldDuration, Text: dur},
		},
		// 定位：点「定位到当前」滚到正在播的，选集搜索滚到匹配的。
		Scroll: (active && a.locateNow) || (a.locateNow && index == a.locatePartIndex),
		Cover2: func(c *ui.Context) {
			// 右上角的添加键。
			label := "添加到播放列表"
			if inList {
				label = "已在播放列表中"
			}
			add := ui.ButtonBase(c.Key(fmt.Sprintf("part-add-%d", p.Cid))).
				Size(24, 24).Radius(RadiusPill).Center().Label(label).Tooltip(label)
			add.Absolute().Right(4).Top(4)
			// 压在封面上：深色半透明填充（不是 backdrop 真玻璃）。
			add.Background(ui.Hex("#0f172a").Alpha(
				pick(add.Hovered(), float32(0.62), float32(0.45)))).
				Border(1, ui.Hex("#ffffff").Alpha(0.20))
			add.Children(func() {
				ui.Icon(c, pick(inList, iconCheck, iconAdd)).Size(13, 13).
					TextColor(pick(inList, ui.Hex("#4ade80"), ui.Hex("#ffffff")))
			})
			if add.Clicked() && a.Act.AddToPlaylist != nil {
				a.Act.AddToPlaylist(p)
			}
		},
		OnClick: func() {
			if a.Act.Play != nil {
				a.Act.Play(index)
			}
			a.Drawer = ""
		},
	}
	a.renderCard(c, spec)
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

// danmakuTabs 是表头（原版 danmakuList.tsx）：弹幕(N) / 评论(total) 两个 tab
// （各自带 14px 图标），后面是「定位当前」（弹幕）与刷新。
func (a *App) danmakuTabs(c *ui.Context) {
	tabs := []tabItem{{
		Key:   TabDanmaku,
		Label: fmt.Sprintf("弹幕 (%d)", len(a.Danmaku)),
		Icon:  iconText,
	}}
	// 原版评论 tab 文案：有 total_count 时是「评论 (共N条)」，否则「评论 (N)」，
	// 没有评论时就是「评论」。
	replyLabel := "评论"
	if a.ReplyTotal > 0 {
		replyLabel = fmt.Sprintf("评论 (共%d条)", a.ReplyTotal)
	} else if len(a.Comments) > 0 {
		replyLabel = fmt.Sprintf("评论 (%d)", len(a.Comments))
	}
	tabs = append(tabs, tabItem{Key: TabReply, Label: replyLabel, Icon: iconDanmaku})

	a.tabs(c, "dm", tabs, a.DanmakuTab, func(k string) {
		a.DanmakuTab = k
		a.danmakuScrollIdx = -1
		if a.Act.SwitchDanmakuTab != nil {
			a.Act.SwitchDanmakuTab(k)
		}
	})

	if a.DanmakuTab == TabDanmaku {
		a.danmakuFollowButton(c)
	}
}

// danmakuFollowButton 是弹幕 tab 的「自动跟随」键（原版 .danmaku-follow-btn）：
// 32×32 图标钮，未开启是灰玻璃、开启是蓝边 + 蓝字 + 一圈光晕。
// 它不走通用 headerButton —— 原版给了单独的 is-active 态，通用键没有。
func (a *App) danmakuFollowButton(c *ui.Context) {
	label := pick(a.DanmakuAutoScroll, "暂停跟随", "开始跟随")
	b := ui.ButtonBase(c.Key("dm-autoscroll")).Size(32, 32).Radius(Radius).
		Center().Label(label).Tooltip(label)
	if a.DanmakuAutoScroll {
		// .danmaku-follow-btn.is-active：蓝边 + 淡蓝渐变底 + 蓝字 + 2px 蓝晕。
		buttonSurface(b, true, b.Hovered(),
			ui.Hex("#0ea5e9").Alpha(0.24), ui.Hex("#0ea5e9").Alpha(0.24)).
			Border(1, ui.Hex("#0284c7").Alpha(0.45)).
			Shadow(0, 0, 0, 2, ui.Hex("#0ea5e9").Alpha(0.14))
	} else {
		buttonSurface(b, false, b.Hovered(),
			ui.Color{}, ui.Hex("#ffffff").Alpha(0.30)).
			Border(1, ui.Hex("#64748b").Alpha(0.22))
	}
	pressPose(b)
	b.Children(func() {
		ink := pick(a.DanmakuAutoScroll, ui.Hex("#0369a1"), ui.Hex("#64748b"))
		ui.Icon(c, iconConnection).Size(18, 18).TextColor(ink)
	})
	if b.Clicked() {
		a.DanmakuAutoScroll = !a.DanmakuAutoScroll
		a.danmakuScrollIdx = -1
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

// danmakuGroups 返回按秒分组的弹幕，带缓存。
//
// 分组要建 map、排序，弹幕列表又长；滚动时视图每帧重建，每帧重算几十毫秒
// 的分配 + 排序就是卡顿的来源。只在弹幕切片真的换了（换集 / 加载完成）时
// 重算，用底层数组首地址 + 长度当身份键。
func (a *App) danmakuGroups() []danmakuGroup {
	if a.dmGroups == nil || !sameSlice(a.dmGroupsSrc, a.Danmaku) {
		a.dmGroupsSrc, a.dmGroups = a.Danmaku, groupDanmaku(a.Danmaku)
	}
	return a.dmGroups
}

// sortedComments 返回按发送时间倒序的评论，带缓存（同 danmakuGroups）。
func (a *App) sortedComments() []Comment {
	if a.cmSorted == nil || !sameSlice(a.cmSortedSrc, a.Comments) {
		sorted := make([]Comment, len(a.Comments))
		copy(sorted, a.Comments)
		for i := 1; i < len(sorted); i++ {
			for j := i; j > 0 && sorted[j].SendTime > sorted[j-1].SendTime; j-- {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			}
		}
		a.cmSortedSrc, a.cmSorted = a.Comments, sorted
	}
	return a.cmSorted
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

// nearestGroup 返回当前播放时间最近的弹幕组下标（原版 danmakuList 的
// currentIndex）：先二分找到第一个 second ≥ pos 的组，再和它前一组比距离，
// 取更近的那个。不是「最后一个不超过 pos 的组」。
func nearestGroup(groups []danmakuGroup, pos float64) int {
	n := len(groups)
	if n == 0 {
		return -1
	}
	low, high := 0, n
	for low < high {
		mid := (low + high) >> 1
		if float64(groups[mid].Second) < pos {
			low = mid + 1
		} else {
			high = mid
		}
	}
	if low == 0 {
		return 0
	}
	if low == n {
		return n - 1
	}
	if pos-float64(groups[low-1].Second) <= float64(groups[low].Second)-pos {
		return low - 1
	}
	return low
}

// fmtClock 把秒格式化成弹幕时间标签的 mm:ss（原版 formatTime 用
// padStart(2, "0") 把分秒都补到两位；分钟不进位，超过 60 分就是 62:05）。
func fmtClock(sec float64) string {
	if sec < 0 || sec != sec {
		sec = 0
	}
	s := int(sec)
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

// danmakuBody 画弹幕列表（评论列表见 repliesBody，由 drawerBody 按 tab 分派）。
// 弹幕按秒分组：组头「时间 + N 条」，组内去重、按次数排序、重复显示 ×N，
// 文字用弹幕自身颜色（太亮/太暗换对比色）——对齐原版 danmakuList。
func (a *App) danmakuBody(c *ui.Context) {
	if len(a.Danmaku) == 0 {
		a.emptyNote(c, "暂无弹幕")
		return
	}
	groups := a.danmakuGroups()

	// 自动跟随：把当前时间所在的组滚进视野（只在下标变化时滚，不干扰手滑）。
	cur := -1
	if a.DanmakuAutoScroll && a.Pos > 0 {
		cur = nearestGroup(groups, a.Pos)
	}

	// 换集时重置虚拟列表的滚动位置与行高，否则新一集会沿用上一集的锚点。
	if a.Track != nil && a.Track.Cid != a.danmakuCid {
		a.danmakuList = ui.ListState{}
		a.danmakuCid = a.Track.Cid
		a.danmakuScrollIdx = -1
	}

	// 虚拟化：这条列表在 bba0c7c 里从 ui.List 改成了 ui.Column，之后每帧都重建
	// **所有**分组行（分组上千时是主要开销之一）。恢复 ui.List 后只建可见的
	// 十几行。
	theme := a.Theme
	sep := ui.Hex("#64748b").Alpha(0.14)
	ui.List(c, &a.danmakuList, len(groups), func(i int) {
		a.danmakuGroupRow(c, groups[i], i == cur, sep, theme)
	}).FillWidth().Grow(1).Padding(8, 10, 16, 10)

	// 只在下标变化时滚，不干扰手滑。
	if cur >= 0 && a.danmakuScrollIdx != cur {
		a.danmakuList.ScrollIntoView(cur)
		a.danmakuScrollIdx = cur
	}
}

// danmakuGroupRow 画一个弹幕分组行（原版 .danmaku-time-group）：整行 grid
// （64px 时间列 + 1fr 消息列）、gap 10、padding 11 12 13、底部 1px 分隔线；
// 组之间没有间距，靠这条线分开。由 danmakuBody 的虚拟列表按需构建。
func (a *App) danmakuGroupRow(c *ui.Context, g danmakuGroup, isCur bool, sep ui.Color, t Theme) {
	box := ui.Row(c).Key(fmt.Sprintf("dmg-%d", g.Second)).FillWidth().
		Margin(2, 0).
		Padding(11, 12, 13).Gap(10).AlignItems(ui.Start).
		Cursor(ui.CursorPointer).
		InsetShadow(0, -1, 0, 0, sep)
	// 悬停/当前态只改底部分隔线的颜色（原版如此，不铺底色）。
	switch {
	case isCur:
		box.InsetShadow(0, -1, 0, 0, ui.Hex("#0284c7").Alpha(0.30))
	case box.Hovered():
		box.InsetShadow(0, -1, 0, 0, t.Blue.Alpha(0.24))
	}
	box.Children(func() {
		// 左列 64：时间标签 + 「N 条」，右缘一线分隔（.danmaku-time-head）。
		// 右缘用 x=-1 的内影（正 x 画的是左缘）。
		ui.Column(c).Width(64).Shrink(0).Gap(4).Justify(ui.Center).
			InsetShadow(-1, 0, 0, 0, sep).Children(func() {
			lbl := ui.Text(c, fmtClock(float64(g.Second))).FontSize(13).
				FontWeight(700).Padding(3, 5).Radius(6)
			if isCur {
				// 当前组：时间标签变实心蓝胶囊（原版 .is-current 的 label）。
				lbl.Background(ui.Hex("#0284c7")).TextColor(ui.Hex("#ffffff")).
					InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.28)).
					Shadow(0, 2, 7, 0, ui.Hex("#0284c7").Alpha(0.22))
			} else {
				lbl.TextColor(ui.Hex("#334155"))
			}
			// .danmaku-time-total：11px #64748b（当前组转蓝加粗）。
			tot := ui.Text(c, fmt.Sprintf("%d 条", g.TotalCount)).FontSize(11).
				TextColor(ui.Hex("#64748b"))
			if isCur {
				tot.TextColor(ui.Hex("#0369a1")).FontWeight(700)
			}
		})

		// 右列：消息横排换行的一串小胶囊（.danmaku-message-list，
		// align-items/align-content 都是 center）。
		ui.Row(c).Grow(1).MinWidth(0).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
			for _, e := range g.Entries {
				chip := ui.Row(c).Key(fmt.Sprintf("dmg-%d-%s", g.Second, e.Text)).
					Height(27).Padding(3, 7, 3, 9).Gap(5).
					Radius(8).AlignItems(ui.Center).
					// 原版是白色半透明小胶囊（无 backdrop-filter）：填充而非真玻璃。
					Background(ui.Hex("#ffffff").Alpha(0.42)).
					Border(1, ui.Hex("#ffffff").Alpha(0.50)).
					Shadow(0, 2, 6, 0, ui.Hex("#0f172a").Alpha(0.045))
				switch {
				case isCur:
					chip.Background(ui.Hex("#0ea5e9").Alpha(0.20)).
						Border(1, t.Blue.Alpha(0.48)).
						Shadow(0, 2, 7, 0, ui.Hex("#0284c7").Alpha(0.10))
				case box.Hovered():
					chip.Background(ui.Hex("#ffffff").Alpha(0.22))
				}
				chip.Children(func() {
					// .danmaku-message-text：13px / 650 / 单行截断。
					ui.Text(c, e.Text).FontSize(13).FontWeight(650).
						TextColor(danmakuInk(e.Color)).SingleLine()
					if e.Count > 1 {
						// .danmaku-repeat-count：小蓝胶囊 ×N。
						ui.Text(c, fmt.Sprintf("×%d", e.Count)).FontSize(10).
							FontWeight(700).TextColor(ui.Hex("#0369a1")).
							Padding(0, 5).Radius(7).MinWidth(24).
							Background(ui.Hex("#e0f2fe").Alpha(0.70)).
							Border(1, t.Blue.Alpha(0.20))
					}
				})
				// 点一条弹幕跳到它出现的时间（原版没有，但弹幕播放器都这么做）。
				if chip.Clicked() && a.Act.Seek != nil {
					a.Act.Seek(float64(g.Second))
				}
			}
		})
	})
}

// repliesBody 画评论列表（原版 danmakuList 的评论 tab）：玻璃卡片行 + 楼中楼
// 预览，滚到底自动加载下一页（原版 handleScroll，不是「加载更多」按钮）。
func (a *App) repliesBody(c *ui.Context) {
	if len(a.Comments) == 0 {
		if a.RepliesLoading {
			a.emptyNote(c, "加载中…")
		} else {
			a.emptyNote(c, "暂无评论")
		}
		return
	}
	// 评论按时间倒序（原版 danmakuList 的 sortedReplies：b.ctime - a.ctime）。
	// 带缓存：排序带拷贝，滚动时每帧重排会白白分配 + 吃 CPU（见 sortedComments）。
	sorted := a.sortedComments()
	// 原版把 HeroUI 的 default 调色板整套覆写成 slate 系（globals.css 里
	// [data-slot="wrapper"] .text-default-*），所以昵称/时间/正文都不是 zinc。
	const (
		ink700 = "#0f172a" // 昵称（.text-default-700 → 0f172a/600）
		ink400 = "#475569" // 时间、点赞、查看更多（.text-default-400 → 475569）
		body   = "#142033" // 正文（.leading-relaxed → 142033/14.5px）
		prim   = "#0284c7" // 楼中楼昵称（.text-primary-500 → 0284c7）
		muted  = "#94a3b8" // 空态/加载中
	)
	ui.Column(c).Children(func() {
		for i, cm := range sorted {
			// 原版行：py-2 px-8 border-b，但 globals.css 又把它覆写成玻璃卡片：
			// margin 4px 6px、圆角 10、白 32% 底 + 32% 白边 + 顶部内高光。
			row := ui.Column(c).Key(fmt.Sprintf("cm-%d", i)).FillWidth().
				Margin(4, 6).Padding(8, 32).Gap(8).Radius(10)
			if row.Hovered() {
				// :hover 底更实、边转蓝、抬起一层淡投影。
				row.Background(ui.Hex("#ffffff").Alpha(0.58)).
					Border(1, ui.Hex("#0ea5e9").Alpha(0.22)).
					InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.58)).
					Shadow(0, 5, 14, 0, ui.Hex("#0f172a").Alpha(0.06))
			} else {
				row.Background(ui.Hex("#ffffff").Alpha(0.32)).
					Border(1, ui.Hex("#ffffff").Alpha(0.32)).
					InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.42))
			}
			row.Children(func() {
				// 用户行：32 圆头像 + 昵称 / 「Lv.N · 时间」。
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					// 原版 .bg-default-200 被覆写成 rgba(226,232,240,.5)。
					face := ui.Box(c).Size(32, 32).Shrink(0).Radius(RadiusPill).
						Background(ui.Hex("#e2e8f0").Alpha(0.5)).Clip()
					face.Children(func() {
						if bmp := a.Images.Bitmap(cm.Avatar); bmp != nil {
							ui.Image(c, bmp).Fill().Fit(ui.Cover)
						}
					})
					ui.Column(c).Children(func() {
						ui.Text(c, cm.User).FontSize(14).FontWeight(600).
							TextColor(ui.Hex(ink700)).SingleLine()
						// text-xs text-default-400：Lv.N · 时间。
						meta := cm.Time
						if cm.Level > 0 {
							meta = fmt.Sprintf("Lv.%d · %s", cm.Level, cm.Time)
						}
						ui.Text(c, meta).FontSize(12).TextColor(ui.Hex(ink400)).SingleLine()
					})
				})
				// 正文区左缩进 40（原版 pl-10，与头像同宽）。
				ui.Column(c).Padding(0, 0, 0, 40).Children(func() {
					// text-sm leading-relaxed → 14.5px / #142033。
					ui.Text(c, cm.Content).FontSize(14.5).TextColor(ui.Hex(body)).MaxLines(10)
					if cm.Likes > 0 {
						ui.Text(c, fmt.Sprintf("👍 %d", cm.Likes)).FontSize(12).
							Margin(4, 0, 0, 0).TextColor(ui.Hex(ink400))
					}
					// 楼中楼：mt-2 pl-3 + 左侧 2px 分隔线（border 覆写成 slate）。
					if len(cm.Replies) > 0 {
						ui.Column(c).Margin(8, 0, 0, 0).Padding(0, 0, 0, 12).
							Gap(8).InsetShadow(2, 0, 0, 0, ui.Hex("#64748b").Alpha(0.18)).Children(func() {
							for j, rp := range cm.Replies {
								ui.Row(c).Key(fmt.Sprintf("cm-%d-r%d", i, j)).
									FillWidth().Gap(4).AlignItems(ui.Start).Children(func() {
									ui.Text(c, rp.User+":").FontSize(12).
										FontWeight(500).TextColor(ui.Hex(prim)).Shrink(0)
									ui.Text(c, rp.Content).FontSize(12).
										TextColor(ui.Hex(body)).Grow(1).MaxLines(4)
								})
							}
							if cm.RepliesMore || len(cm.Replies) >= 3 {
								ui.Text(c, "查看更多回复...").FontSize(12).
									TextColor(ui.Hex(muted))
							}
						})
					}
				})
			})
		}
		// 原版没有「加载更多」按钮：滚到底自动翻页（见 drawerBody），
		// 加载中在末尾提示一行「加载中...」。
		if a.RepliesLoading && len(sorted) > 0 {
			ui.Row(c).FillWidth().Padding(8, 0).Justify(ui.Center).Children(func() {
				ui.Text(c, "加载中...").FontSize(12).TextColor(ui.Hex(muted))
			})
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

// collectScrollBtn 是收藏夹表头的左右滚动键（原版 collect-scroll-btn）：
// 把横向滚动位置拨 200px。
func (a *App) collectScrollBtn(c *ui.Context, key, label string, delta float32) {
	t := a.Theme
	b := ui.ButtonBase(c.Key(key)).Size(26, 28).Radius(7).Center().
		Label(label).Tooltip(label)
	b.Background(ui.Hex("#0f172a").Alpha(0.06))
	if b.Hovered() {
		b.Background(ui.Hex("#0f172a").Alpha(0.12))
	}
	b.Children(func() {
		ic := pick(delta < 0, iconLeft, iconRight)
		ui.Icon(c, ic).Size(14, 14).TextColor(pick(b.Hovered(), t.Blue, ui.Hex("#475569")))
	})
	if b.Clicked() {
		a.CollectScroll.X = max(0, a.CollectScroll.X+delta)
	}
}
