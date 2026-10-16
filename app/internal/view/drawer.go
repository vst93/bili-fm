package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/imagecache"
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

// cardLiftAnim 是抽屉卡片悬停上浮 / 回落的动画：原版
// `[data-slot="wrapper"] [role="button"][class*="bg-content"]` 的
// `transition: ... transform 0.16s ease`（globals.css:3662），上浮
// `translateY(-3px)`、移开回落，全程 160ms。之前用 500ms 的 Snappy 弹簧，
// 用户报「浮动动画太慢、移开回落不利落」。CSS 的 `ease` 即
// cubic-bezier(0.25, 0.1, 0.25, 1)，用 mygo 的同名曲线逐字对齐。
var cardLiftAnim = ui.Animation{
	Duration: 160 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// 抽屉列表里几处悬停 / 当前态的色值过渡，逐条取自原版 globals.css：
//
//	评论行 .py-2.px-8.border-b     transition: background-color/border-color
//	                                /box-shadow 140ms ease
//	弹幕小胶囊 .danmaku-message-row 同上 140ms ease
//	弹幕时间标签 .danmaku-time-label background-color/color/box-shadow 140ms ease
//	弹幕分组行 .danmaku-time-group   border-color 150ms ease
//
// 此前这些状态都是瞬时切换（悬停时整行 / 整颗胶囊「啪」地换色）。mygo 的
// 阴影没有逐通道动画通道，box-shadow 仍随底色当帧切换；这里只补色值。
var (
	drawerFadeAnim = ui.Animation{
		Duration: 140 * time.Millisecond,
		Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
	}
	danmakuBorderAnim = ui.Animation{
		Duration: 150 * time.Millisecond,
		Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
	}
)

// drawerFadeColor 把一处色值交给动画层做过渡。拆成 var 便于测试换桩做反证：
// AnimateColor 在目标变化的那一帧返回**上一帧的颜色**（曲线 t=0），直接铺
// 目标色会瞬时跳变。key 在同一元素内须唯一。
//
// 桌面要求减少动态效果时，原版对评论行 / 弹幕胶囊 / 时间标签 / 分组行写了
// `transition: none`（globals.css 的 prefers-reduced-motion 块），所以这里
// 直接铺目标色（当帧到位）。
var drawerFadeColor = func(c *ui.Context, b ui.Element, key string, target ui.Color, a ui.Animation) ui.Color {
	if reduceMotion(c) {
		return target
	}
	return b.AnimateColor(key, target, a)
}

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

// drawerEnterStep 决定抽屉这一帧是不是「入场帧」，并返回要记下的 key 与
// 时间戳（供下一帧算动画进度）。
//
// 难点在**同一帧里的多次 build pass**：点击打开抽屉的那一帧，`Clicked()`
// 会当场置 `rt.consumed`，框架会把整个视图重建第二趟（input.go:895）。
// 第一趟记下入场时间戳，第二趟若按「已经不是新抽屉」处理就会把时间戳清掉，
// 入场动画从中间起跳。`now` 在整帧内不变（context.go 的 c.now 每帧设一次），
// 所以用「与入场时间戳相同」识别同帧后续 pass，保持起始时间不变。
func drawerEnterStep(drawer, openKey string, enterAt, now time.Time) (entering bool, nextKey string, nextAt time.Time) {
	switch {
	case drawer != openKey:
		// 换抽屉 / 首次打开 / 关掉后重开：进入入场。
		return true, drawer, now
	case enterAt.Equal(now):
		// 同一帧的又一次 build pass：仍是入场帧，别把起始时间覆盖掉。
		return true, openKey, enterAt
	}
	return false, openKey, enterAt
}

// drawerExitDone 报告退场是否该收尾：至少隔了一帧（`exitAt` 那一刻设的
// 起始位移已经被显示过，否则同一帧就收尾会让面板没动过），且动画已经走完。
func drawerExitDone(exitAt, now time.Time, moving bool) bool {
	return !exitAt.Equal(now) && !moving
}

// drawerAnimOffset 给出「用布局位移做动画」时面板这一帧的 Y 偏移（DIP，
// 向下为正，0=贴底就位）。
//
// 为什么用布局位移（`Bottom`）而不是 `AnimatePose` 的 transform：mygo 的
// 软件渲染器对任何 `Transform.Set` 的 op 一律走逐像素的 `transformed` 慢路径
// （internal/raster/raster.go:210 的 dispatch：`if op.Transform.Set { r.transformed(op,
// nil, nil) } else { r.fill(op) }`），而抽屉面板连同它的卡片/文字整棵子树都会
// 被 pose 打上 transform，于是整窗重绘变成「逐像素 + archExp/SDRoundRect」。
// 无头实测（1400×900@2x、120 张卡、受控时钟走生产 surfaceFrame，拆 build/layout/
// paint/present）：退场 16 帧 present avg 3877ms（worst 4208ms）；改成纯布局位移
// （transform 保持 identity，走扫描线快路径）后 avg 55–86ms（worst 80–140ms），
// 约 70×。入场也从 avg 599ms 降到 68ms。代价是每帧多一次 build
// （实测 build 0.3ms + layout 0.5–1.5ms，随载荷），远小于省下的绘制。
//
// moving 为假时面板停在终点（入场=0 贴底、退场=h 窗口外）；closing 时从 0
// 滑到 h，入场时从 h 升到 0；未走完时第二个返回值为真，调用方要
// `Context.AnimationFrame()` 请下一帧。返回值向下为正（叠在面板的
// `Bottom(-offset)` 上：正 offset 把面板推离窗口底边、向下移出）。
func drawerAnimOffset(moving, closing bool, start, now time.Time, h float32) (offset float32, animating bool) {
	target, from := float32(0), h // 入场：从窗口下方 h 升到贴底 0
	if closing {
		target, from = h, 0 // 退场：从贴底 0 降到窗口下方 h
	}
	if !moving || start.IsZero() {
		return target, false
	}
	t := float32(0)
	if d := now.Sub(start); d > 0 {
		t = float32(d) / float32(drawerIn)
	}
	if t >= 1 {
		return target, false
	}
	// easeOutCubic：与原版 HeroUI 抽屉的 ease-out 接近。
	e := 1 - (1-t)*(1-t)*(1-t)
	return from + (target-from)*e, true
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
			// 退场一开始就把「已起步的抽屉」清掉：否则同一抽屉关掉后马上重开会被
			// drawerEnterStep 当成「不是新抽屉」而丢掉入场动画。退场进度由
			// drawerExitAt 独立计时（见 drawerAnimOffset），不依赖 pose 状态。
			a.drawerOpenKey = ""
			// 原版 danmakuList 的 handleOpenChange(false)：关抽屉时把自动跟随
			// 重置为开、滚动锚点清掉，下次打开重新跟随当前播放位置。
			if a.drawerShown == DrawerDanmaku {
				a.DanmakuAutoScroll = true
				a.danmakuScrollIdx = -1
			}
		} else if drawerExitDone(a.drawerExitAt, c.Now(), c.Now().Sub(a.drawerExitAt) < drawerIn) {
			// 退场动画走完：面板已经滑到窗口外，这一帧起真正移出视图。放在构建
			// 之前，省掉最后一帧的整窗重绘。
			a.drawerShown, a.drawerClosing = "", false
			a.drawerOpenKey, a.drawerOffset = "", ui.Point{}
			return
		}
	} else {
		a.drawerShown, a.drawerClosing = a.Drawer, false
	}
	// 渲染用的 key：退场时用最后显示的那个。
	key := pick(a.Drawer == "", a.drawerShown, a.Drawer)

	_, winH := c.Size()
	// min(92vh, 100vh - 54px)：54 是播放栏高度，抽屉不盖住它。
	h := min(winH*0.92, winH-54)

	// 入场：换抽屉 / 重新打开时记下起始时间，面板从窗口下方（偏移 h）往上滑。
	// 退场时不参与（key 用 drawerShown）。
	if !a.drawerClosing {
		if a.Drawer != a.drawerOpenKey {
			a.drawerOffset = ui.Point{}
		}
		_, a.drawerOpenKey, a.drawerEnterAt =
			drawerEnterStep(a.Drawer, a.drawerOpenKey, a.drawerEnterAt, c.Now())
	}
	// 动画进度用「布局位移」算，不用 transform（见 drawerAnimOffset）：入场从
	// h 升到 0、退场从 0 降到 h；未走完就请下一帧（AnimationFrame 会整视图重建，
	// 这正是重算布局位置所需的）。
	startAt := a.drawerEnterAt
	if a.drawerClosing {
		startAt = a.drawerExitAt
	}
	moving := !startAt.IsZero() && c.Now().Sub(startAt) < drawerIn
	offset, animating := drawerAnimOffset(moving, a.drawerClosing, startAt, c.Now(), h)
	if animating {
		c.AnimationFrame()
	}
	overlay := ui.Box(c).Absolute().Fill()
	overlay.Background(ui.Hex("#0f172a").Alpha(0.28))
	var dragY float32
	overlay.Children(func() {
		// 原版抽屉的 ::before 有 backdrop-filter: blur(14px) saturate(1.35)
		// （--liquid-blur-thin），面板确实是真玻璃。但这一层是抽屉打开时**面积
		// 最大**的 backdrop（2.7Mpx 级），mygo 对每个 backdrop 都要断一次渲染
		// 通道 + 回读下采样 + 两次模糊，层数直接乘进每帧——上一轮实测 24→4 层
		// 后用户实机反馈「仍卡」，所以本轮按既定下一档把面板降级成半透明填充：
		//   background --liquid-bg + 1px 亮边 + 顶部内高光（原版 ::after 的聚光）
		// 唯一的观感取舍：面板由「磨砂」变成实色。
		//
		// 降级成填充后底色不能再用 0.22 的 --liquid-bg（那是给 blur 用的）：没有
		// 模糊时 22% 白太透，用户反馈「影响阅读」。改用原版给无 backdrop-filter
		// 平台的兜底 --liquid-bg(0.9)（见 drawerPanelFill）。
		//
		// 位移走 Bottom（布局）而不是 AnimatePose 的 transform：软件渲染器对带
		// transform 的 op 一律逐像素重绘，整棵抽屉子树（面板 + 卡片 + 文字）都会
		// 中招（见 drawerAnimOffset 的实测）。每帧重建 ≈0.5ms，远小于省下的绘制。
		panel := ui.Column(c).Absolute().Left(0).Right(0).Bottom(-offset).Height(h).
			Radius(18, 18, 0, 0).
			Clip().
			Background(drawerPanelFill()).
			Border(1, a.Theme.GlassBorder).
			InsetShadow(0, 1, 1, 0, a.Theme.Highlight).
			Shadow(0, -16, 50, 0, shadowInk.Alpha(0.08))
		// v0.4.5 的 Draggable：按住面板空白处（表头、边距——正文的滚动区是
		// 更内层的交互元素，不会抢走手势）往下拖，松手弹回或被甩到底关闭。
		// 偏移叠在布局位移之上。
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
		// 只有渲染期间没人动过 a.Drawer 时才把临时的 key 摆回原值。
		// 若表头 / 正文的动作改了它（最典型的是点关闭键 a.Drawer = ""，
		// 或点条目跳到别的抽屉），那正是用户的意图——此前无条件
		// `a.Drawer = real` 会把它改回去，关闭键因此点了没反应。
		if a.Drawer == key {
			a.Drawer = real
		}
		// DragOffset 是本帧面板实际显示的偏移（跟手 / 回弹途中都算），
		// 叠在布局位移之上，用它让遮罩跟着下滑变淡。
		dragY = offset + panel.DragOffset().Y
		if !a.drawerClosing && drawerDismissed(a.drawerOffset.Y, h, panel.Transitioning()) {
			// 拖拽已经把它滑出去了，下一帧走退场收尾（offset 保持，别弹回）。
			a.Drawer = ""
		}
	})
	// 遮罩跟着面板位置直接改不透明度（不再用 AnimatePose 的 Fade：那会把遮罩
	// 也打上 transform 的慢路径）：贴底 28%、滑到底 0%。
	overlay.Background(ui.Hex("#0f172a").Alpha(0.28 * drawerFade(dragY, h)))
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
	//
	// 表头层次：原版 drawer header 是实底（不透底层内容），此前 mygo
	// 的 header Row 没有铺材质，直接透了主页搜索栏的液态玻璃，看着古怪。
	// 这里给 header 铺一层近实底背景，与面板主体形成层次区分。
	// 不用 Material(glassLayer(...))：那会多一个 backdrop 层（每帧多一次
	// 渲染通道打断 + 回读 + 模糊），而实底背景已经足够遮住底层。
	ui.Row(c).FillWidth().Height(48).Shrink(0).Padding(0, 44, 0, 24).
		Gap(8).AlignItems(ui.Center).
		Background(ui.Hex("#f1f5f9").Alpha(0.92)).
		Children(func() {
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
			// 刷新键的形态按抽屉分流（原版两种不同的按钮实现，见
			// refreshKeyStateFor）：历史抽屉「图标自转」、弹幕/评论抽屉
			// 「Spinner 替掉图标」，两者的加载期都必须禁用。
			histLoading := a.Drawer == "history" && a.list().Loading
			st := refreshKeyStateFor(a.Drawer, a.HistTab,
				histLoading, a.DanmakuLoading, a.RepliesLoading)
			a.headerButtonOff(c, "drawer-refresh", st.Label, iconRefresh, refreshSize, ui.Hex("#333333"), st.Spin, st.Spinner, func() {
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

	// 关闭键浮在右上角（原版是绝对定位的 close 按钮）。圆角是 HeroUI 按钮默认的
	// md（8），不是工具键的 6；材质见 drawerCloseSurface（常驻玻璃，不再只在悬停时出现）。
	ui.Box(c).Absolute().Right(10).Top(8).Children(func() {
		b := ui.ButtonBase(c.Key("drawer-close")).Size(32, 32).Radius(8).Center().
			Label("关闭").Tooltip("关闭")
		drawerCloseSurface(b, b.Hovered(), b.Pressed())
		drawerClosePress(b)
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
		ui.Text(c, "搜索").FontSize(14).FontWeight(600).SingleLine().
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
		ui.Text(c, "收藏列表").FontSize(14).FontWeight(600).SingleLine().
			TextColor(ui.Hex("#334155")).Shrink(0)
		// 原版表头**不判空**：收藏夹还没回来（或一个都没有）时照样画左右滚动键
		// 与（空的）胶囊容器，空态只出现在正文里（.history-empty-tip）。之前这里
		// 直接换成一行「暂无收藏内容」，既和正文的空态重复，也把两个滚动键藏掉了。
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
		ui.Text(c, "动态列表").FontSize(14).FontWeight(600).SingleLine().TextColor(ui.Hex("#334155")).Shrink(0)
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
		// 原版 seriesList 的按钮有二态：列表为空或正在拉取时禁用
		//（isDisabled / isLoading），拉取中画 Spinner 替代播放图标。
		spinner, enabled := seriesPlayAllSpec(a.SeriesPlayAllPending,
			len(a.ListFor(DrawerSeries).Cards) > 0)
		if b := ui.ButtonBase(c.Key("series-play-all")).Height(32).Padding(0, 12).
			Radius(Radius).Center().Label("播放全部").Tooltip("播放合集中的全部视频"); true {
			b.Disabled(!enabled)
			drawerHeaderSurface(b, b.Hovered())
			drawerHeaderPress(b)
			b.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					if spinner {
						// 原版 isLoading 时 HeroUI 画 Spinner 替代 <Play />。
						ui.Spinner(c).Size(16, 16).TextColor(ui.Hex("#2563eb"))
					} else {
						// 原版 <Play fill="#666" size={16} theme="filled" />：灰色圆盘
						// + 挖白三角（逐色 Image）。
						ui.Image(c, iconPlayFilled).Size(16, 16).TextColor(ui.Hex("#666666"))
					}
					ui.Text(c, "播放全部").FontSize(12).TextColor(ui.Hex("#475569"))
				})
			})
			if b.Clicked() && enabled && a.Act.SeriesPlayAll != nil {
				a.Act.SeriesPlayAll()
			}
		}
	default:
		ui.Text(c, a.drawerTitle()).FontSize(14).FontWeight(600).SingleLine().
			TextColor(ui.Hex("#334155")).Shrink(0)
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
		ui.Text(c, "粉丝 "+fanCount(a.UpFans)).FontSize(12).SingleLine().
			TextColor(ui.Hex("#71717a")).Shrink(0)
	}

	label, spinner := upFollowSpec(a.UpFollowPending, a.UpFollowed)
	b := ui.ButtonBase(c.Key("up-follow")).MinWidth(70).Height(32).Padding(0, 12).
		Radius(Radius).Center().Label(label).Tooltip(label)
	// 原版关注键在「查关注状态 / 关注请求在途」时禁用（isCheckingFollow /
	// isFollowingLoading）：挡住连点，避免 follow 与 unfollow 各发一次。
	b.Disabled(a.UpFollowPending)
	drawerHeaderSurface(b, b.Hovered())
	drawerHeaderPress(b)
	b.Children(func() {
		if spinner {
			// 原版 <Spinner size="sm" />：转圈替代图标 + 文案。
			ui.Spinner(c).Size(16, 16).TextColor(ui.Hex("#2563eb"))
			return
		}
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			// 原版：关注态是 Close #666 + 默认字色；未关注是 Add + text-primary。
			ui.Icon(c, pick(a.UpFollowed, iconFollowed, iconFollow)).Size(14, 14).
				TextColor(pick(a.UpFollowed, ui.Hex("#666666"), ui.Hex("#2563eb")))
			ui.Text(c, label).FontSize(14).
				TextColor(pick(a.UpFollowed, ui.Hex("#3f3f46"), ui.Hex("#2563eb")))
		})
	})
	if b.Clicked() && !a.UpFollowPending && a.Act.ToggleFollow != nil {
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

// upFollowSpec 给出 UP 空间表头关注键这一帧的文案与是否转圈。
//
// 原版 upVideoList 的按钮：
//
//	isCheckingFollow（关注状态还在查）→ Spinner，禁用
//	isFollowingLoading（关注 / 取关在途）→ isLoading，禁用
//	已关注                              → Close 图标 + 「已关注」
//	否则                                → Add 图标 + 「关注」
//
// pending 为真是前两种：转圈并禁用（文案保持「关注」，让 Find / tooltip 稳定）。
func upFollowSpec(pending, followed bool) (label string, spinner bool) {
	if pending {
		return "关注", true
	}
	return pick(followed, "已关注", "关注"), false
}

// ---------------------------------------------------------- 抽屉内的窄窗响应式
//
// 原版 globals.css 还有两条宽度媒体查询此前没接（上一轮补了主区的 ≤680）：
//
//	@media (max-width: 520px)  历史抽屉的隐身开关 min-width 92→66、文字标签隐藏
//	@media (max-width: 420px)  弹幕分组左列 64→54、gap 10→8、inline 内边距 12→10
//
// 两条断点都是**视口**宽度（媒体查询作用于窗口，不是抽屉面板），所以取
// c.Size() 的宽。抽成纯函数便于单测断点与取值。

const (
	// incognitoNarrowWidth / danmakuNarrowWidth 与 globals.css 的媒体查询阈值一致。
	incognitoNarrowWidth = 520
	danmakuNarrowWidth   = 420

	// danmakuLabelHeight 是 `.danmaku-time-label` 的高度：line-height:1 的
	// 13px 行盒 + 上下 padding 3px。mygo 的行高取字体度量的硬底（13px 字体
	// 的自然行盒约 20px），压不到 13，所以给标签显式 Height 固定胶囊高度；
	// 横向 padding 保留 5px，竖向 padding 去掉（否则文字会在 19px 盒里下坠）。
	danmakuLabelHeight = 19

	// danmakuTotalHeight / danmakuTotalShift 对应 `.danmaku-time-total` 的
	// `font-size:11px; line-height:1.2`（行盒 13.2px）。mygo 的行高硬底是
	// 11px 字体的自然行盒约 17px，压不到 13.2，所以固定元素盒高；原版行盒
	// 比自然行盒矮，字形在上下各溢出约 1.9px（负 half-leading），再把字形
	// 上移同样的量，让「N 条」的落点与原版一致。
	danmakuTotalHeight = 13.2
	danmakuTotalShift  = -1.9

	// danmakuRepeatCountHeight / danmakuRepeatCountShift 对应
	// `.danmaku-repeat-count` 的 `font-size:10px; height:18px; line-height:16px`。
	// 框架自然行盒约 15px（10×1.5），比原版 18px 矮 3px；显式 Height + Translate
	// 把 ×N 胶囊压回原版高度。
	danmakuRepeatCountHeight = 18.0
	danmakuRepeatCountShift  = 1.5
)

// windowWidth 是这一帧的窗口宽度（DIP）——抽屉里的媒体查询断点按视口宽度判定。
func windowWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	return w
}

// danmakuGroupMaxHeight 是 `.danmaku-time-group` 的 `max-height: 80dvh`。
// viewportH 是窗口（视口）高度。
func danmakuGroupMaxHeight(viewportH float32) float32 {
	return max(0, 0.8*viewportH)
}

// danmakuMessageListMaxHeight 是 `.danmaku-message-list` 的
// `max-height: calc(80dvh - 20px)`：同一秒里弹幕很多时，消息列不跟着分组
// 无限长高，而是在分组内滚动（原版 overflow-y:auto）。
func danmakuMessageListMaxHeight(viewportH float32) float32 {
	return max(0, 0.8*viewportH-20)
}

// incognitoLayout 给出隐身开关这一帧的 min-width 与是否画「隐身」文字标签
// （原版 .history-incognito-switch / .history-incognito-label 的 ≤520 分支：
// min-width 收到 66，文字 display:none，只剩图标 + 轨道）。
func incognitoLayout(width float32) (minWidth float32, showLabel bool) {
	if width <= incognitoNarrowWidth {
		return 66, false
	}
	return 92, true
}

// danmakuRowLayout 是弹幕分组行（.danmaku-time-group）在某个窗口宽度下的
// 左列宽 / 元素间距 / 左右内边距取值。
type danmakuRowLayout struct {
	Col, Gap, PadInline float32
}

// danmakuRowLayoutFor 按窗口宽度给出弹幕分组行的布局：基线 grid-template-columns
// 64px / gap 10 / padding-inline 12；≤420 时 54 / 8 / 10。
func danmakuRowLayoutFor(width float32) danmakuRowLayout {
	if width <= danmakuNarrowWidth {
		return danmakuRowLayout{Col: 54, Gap: 8, PadInline: 10}
	}
	return danmakuRowLayout{Col: 64, Gap: 10, PadInline: 12}
}

// incognitoTip 是隐身开关的 tooltip（原版 historyList 的 Tooltip content）：
// 开着时提示关掉的后果，关着时提示打开的效果。
func incognitoTip(on bool) string {
	return pick(on,
		"关闭后恢复云端播放记录与进度同步",
		"开启后不读取或上报云端播放记录与进度")
}

// historyRefreshLabel 是历史抽屉刷新键的文案（原版按 tab 切换 aria-label）。
func historyRefreshLabel(tab string) string {
	if tab == HistWatchLater {
		return "刷新稍后再看"
	}
	return "刷新历史记录"
}

// incognitoInk 是隐身开关的墨色（图标 / 文字 / 轨道都用它）：开启时青
// #0e7490，关闭时 #64748b。原版 `.history-incognito-switch[data-active="true"]`
// 把图标与文字都改成青色。
func incognitoInk(on bool) ui.Color {
	return pick(on, ui.Hex("#0e7490"), ui.Hex("#64748b"))
}

// incognitoTrack 画开关右端的小轨道（原版 .history-incognito-track /
// .history-incognito-thumb）：27×16（padding 2、圆角 999、内凹影）、
// 滑块 12，打开时右移 11。抽屉表头与设置菜单里那颗开关共用它。
func incognitoTrack(c *ui.Context, on bool) {
	ui.Box(c).Width(27).Height(16).Shrink(0).Radius(RadiusPill).Padding(2).
		InsetShadow(0, 1, 2, 0, ui.Hex("#0f172a").Alpha(0.10)).
		Background(pick(on, ui.Hex("#0891b2"), ui.Hex("#64748b").Alpha(0.32))).
		Children(func() {
			ui.Box(c).Size(12, 12).Radius(RadiusPill).Background(ui.Hex("#ffffff")).
				Margin(0, 0, 0, pick(on, float32(11), float32(0)))
		})
}


// 32 高、图标 + 文字 + 一个小轨道。打开后不读也不写云端进度。
func (a *App) incognitoSwitch(c *ui.Context) {
	// 原版 .history-incognito-switch：min-width 92、高 32、gap 7、padding 0 8、
	// 圆角 8、1px 边 rgba(100,116,139,.2)、底 rgba(255,255,255,.34)，
	// 内高光 + 淡投影；开启时转青（#0e7490 / 底 rgba(207,250,254,.5)）。
	// ≤520 时原版把 min-width 收到 66 并隐藏文字标签（只剩图标 + 轨道）。
	minW, showLabel := incognitoLayout(windowWidth(c))
	// 原版这颗键是 `role="switch"` + `aria-checked={isIncognitoMode}`：读屏用户
	// 需要听到开关的「开/关」状态。mygo 对应 RoleSwitch + Checked(on)（两者都读
	// 同一个 a.Incognito，与图标 filled/outline、轨道色、底色同源）。
	b := ui.ButtonBase(c.Key("incognito")).MinWidth(minW).Height(32).Padding(0, 8).
		Radius(Radius).Role(ui.RoleSwitch).Label("隐身").Tooltip(incognitoTip(a.Incognito)).
		Checked(a.Incognito)
	// 半透明填充；开启隐身时用青色底表达（原版是 rgba(207,250,254,.5) 底）。
	buttonSurface(b, a.Incognito, b.Hovered(),
		ui.Hex("#06b6d4").Alpha(0.28), ui.Hex("#ffffff").Alpha(0.30)).
		Shadow(0, 2, 7, 0, ui.Hex("#0f172a").Alpha(0.05))
	// 原版 `.history-incognito-switch:focus-visible`：`outline: 2px solid
	// rgba(14,165,233,.44); outline-offset: 2px`（没有换色 / 换底的整态差异）。
	focusOutline(b, Radius, focusSky44)
	ink := incognitoInk(a.Incognito)
	b.Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, pick(a.Incognito, iconMaskFilled, iconMask)).Size(15, 15).TextColor(ink)
			// .history-incognito-label：12px / 650 / line-height 1；≤520 时不画
			//（原版 display:none，min-width 同步收到 66）。
			if showLabel {
				ui.Text(c, "隐身").FontSize(12).FontWeight(650).TextColor(ink)
			}
			incognitoTrack(c, a.Incognito)
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

// seriesPlayAllSpec 给出合集抽屉表头「播放全部」这一帧是否转圈 / 是否可点。
//
// 原版 seriesList 的按钮：
//
//	isDisabled={seriesVideos.length === 0}  列表为空 → 禁用 + 点不动
//	isLoading={isPlayingAll}                 拉取在途 → Spinner（替掉 <Play />）+ 禁用
//
// pending 为真时转圈且禁用；否则列表为空也禁用（原版点空白不会有任何反应，
// 原生版此前会每点一次起一套分页 + 详情请求，且没有反馈）。
func seriesPlayAllSpec(pending, hasVideos bool) (spinner, enabled bool) {
	if pending {
		return true, false
	}
	return false, hasVideos
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

// ---------------------------------------------------------------- 表头 tab 材质

// drawerTabPaint 是抽屉表头 tab（原版
// `[data-slot="wrapper"] header [data-slot="tab"]`）某一状态解析好的材质：
//
//	常态  1px --glass-border（白 .55）边、--liquid-bg（白 .22）底、box-shadow none
//	悬停  1px --glass-border-bright（白 .72）边、--liquid-bg-hover（白 .30）底、
//	      box-shadow --liquid-shadow（0 8 24 墨 .07 + 0 1 4 墨 .03）
//	选中  1px --glass-border-bright 边、白 .52 底、
//	      box-shadow --liquid-shadow-lifted（0 16 38 墨 .11 + 0 2 8 墨 .04）
//
// 常态的 `box-shadow: none` 对应两层零值 shadowSpec（几何与色值都为零，CSS 把
// none 当成一组零长透明影子参与插值）。Rim 是 ::after 聚光描边的整圈不透明度：
// 常态 .65、选中 1（悬停未选中不覆写，仍是 .65）。
type drawerTabPaint struct {
	BG, Border     ui.Color
	LiftLG, LiftSM shadowSpec
	Rim            float32
}

// drawerTabPaintFor 由（选中 / 悬停）解析材质。
func drawerTabPaintFor(active, hovered bool) drawerTabPaint {
	p := drawerTabPaint{
		BG:     ui.Hex("#ffffff").Alpha(0.22), // --liquid-bg
		Border: ui.Hex("#ffffff").Alpha(0.55), // --glass-border
		Rim:    0.65,
	}
	switch {
	case active:
		p.BG = ui.Hex("#ffffff").Alpha(0.52)
		p.Border = ui.Hex("#ffffff").Alpha(0.72) // --glass-border-bright
		p.Rim = 1
		p.LiftLG = shadowSpec{0, 16, 38, 0, shadowInk.Alpha(0.11)}
		p.LiftSM = shadowSpec{0, 2, 8, 0, shadowInk.Alpha(0.04)}
	case hovered:
		p.BG = ui.Hex("#ffffff").Alpha(0.30) // --liquid-bg-hover
		p.Border = ui.Hex("#ffffff").Alpha(0.72)
		p.LiftLG = shadowSpec{0, 8, 24, 0, shadowInk.Alpha(0.07)}
		p.LiftSM = shadowSpec{0, 1, 4, 0, shadowInk.Alpha(0.03)}
	}
	return p
}

// drawerTabInk 是 tab 的文字色：常态 #475569、悬停（未选中）#334155、
// 选中 #0369a1。
func drawerTabInk(active, hovered bool) ui.Color {
	switch {
	case active:
		return ui.Hex("#0369a1")
	case hovered:
		return ui.Hex("#334155")
	}
	return ui.Hex("#475569")
}

// drawerTabAnim 是 tab 状态切换的过渡：原版 `[data-slot="tab"]` 的
// `transition: border-color 0.2s ease, box-shadow 0.2s ease, color 0.2s ease`，
// 加上 `::before { transition: background-color 0.2s ease }` 与
// `::after { transition: opacity 0.2s ease }`（CSS ease = cubic-bezier(.25,.1,.25,1)）。
var drawerTabAnim = ui.Animation{
	Duration: 200 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// drawerTabFade 把一处色值交给 0.2s 过渡（同 partSearchFade）。声明成 var 便于
// 测试冻结动画相位。
var drawerTabFade = func(b ui.Element, key string, target ui.Color) ui.Color {
	return b.AnimateColor(key, target, drawerTabAnim)
}

// drawerTabShadow 同 partSearchShadow：box-shadow 的几何与色值一起走 0.2s 过渡。
var drawerTabShadow = func(b ui.Element, key string, s shadowSpec) shadowSpec {
	return shadowSpec{
		X:      b.AnimateTo(key+"-x", s.X, drawerTabAnim),
		Y:      b.AnimateTo(key+"-y", s.Y, drawerTabAnim),
		Blur:   b.AnimateTo(key+"-blur", s.Blur, drawerTabAnim),
		Spread: b.AnimateTo(key+"-spread", s.Spread, drawerTabAnim),
		C:      b.AnimateColor(key+"-ink", s.C, drawerTabAnim),
	}
}

// drawerTabShadowInto 把一条过渡中的 box-shadow 画到 tab 上。
func drawerTabShadowInto(b ui.Element, key string, s shadowSpec) {
	r := drawerTabShadow(b, key, s)
	b.Shadow(r.X, r.Y, r.Blur, r.Spread, r.C)
}

// drawerTabRimOpacity 把 ::after 聚光描边的整圈不透明度做 0.2s 过渡。
var drawerTabRimOpacity = func(b ui.Element, target float32) float32 {
	return b.AnimateTo("tab-rim", target, drawerTabAnim)
}

// drawerTabGlass 画 tab 的 `::before` 洗光（--glass-sheen-static）与 `::after`
// 的 1px 聚光描边（--liquid-rim）。与 partSearchGlass / partPillGlass 同一套近似：
// mygo 没有径向 / conic 渐变接口，光泽复用播放栏那套烘焙位图（sheenGlassBaked
// 就是 --glass-sheen-static），描边复用四边线性近似（paintPlayerRim）。
// sheen 走 paintBakedSheen：按 tab 自己的宽高比烘一张，线性层的角度不被拉偏。
// 声明成 var 便于测试换桩做反证。
var drawerTabGlass = func(b ui.Element, rimOpacity float32) ui.Element {
	return b.Draw(func(p *ui.Painter, r ui.Rect) {
		paintBakedSheen(p, r, Radius, sheenGlassBaked)
		if rimOpacity > 0 {
			paintPlayerRim(p, r, Radius, rimOpacity)
		}
	})
}

// drawerTabSurface 给抽屉表头 tab（HeroUI Tabs）铺原版那套液态玻璃材质：
// 1px 玻璃边 + --liquid-bg 底 + ::before 的 --glass-sheen-static 洗光 +
// ::after 的 1px 聚光描边与两条内影 / 色散边；悬停 / 选中换边色、换底、抬起一档。
//
// 此前 tab 借的是播放栏的 buttonSurface —— 那是 `#player .player-button` 的材质
// （白 .22 底 + 1px 白 .55 边 + 两层内影 + **常驻**小投影），于是 tab 多出一层
// 原版没有的常驻投影、少了两层液态玻璃，顶部内高光也暗一档（.55 vs
// --glass-highlight 的 .9），悬停 / 选中的抬起档位更是完全没有。
// 声明成 var 便于测试换桩做反证。
var drawerTabSurface = func(b ui.Element, active, hovered bool) ui.Element {
	p := drawerTabPaintFor(active, hovered)
	// 原版 transition: border-color / box-shadow / color 0.2s ease，
	// ::before { background-color 0.2s ease }、::after { opacity 0.2s ease }。
	b.Background(drawerTabFade(b, "tab-bg", p.BG)).
		Border(1, drawerTabFade(b, "tab-border", p.Border))
	drawerTabShadowInto(b, "tab-lift-lg", p.LiftLG)
	drawerTabShadowInto(b, "tab-lift-sm", p.LiftSM)
	// ::after 上那组不随状态变的内影与色散边（原版 ::after 的 box-shadow：
	// inset 0 1px 1px .5px --glass-highlight、inset 0 -1px 1px 墨 .06、
	// --liquid-chromatic 两条 0.5px 亚像素边）。整圈不透明度由 ::after 的
	// opacity 统一缩放，所以这里也按 rim 缩一遍。
	rim := drawerTabRimOpacity(b, p.Rim)
	b.InsetShadow(0, 1, 1, 0.5, ui.Hex("#ffffff").Alpha(0.90*rim))
	b.InsetShadow(0, -1, 1, 0, shadowInk.Alpha(0.06*rim))
	b.Shadow(0, 0.5, 0, 0, ui.Hex("#fffaf0").Alpha(0.50*rim))
	b.Shadow(0, -0.5, 0, 0, ui.Hex("#c8dcf0").Alpha(0.20*rim))
	return drawerTabGlass(b, rim)
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
	// 原版 header 是单行 flex：tab 不参与收缩（globals.css:4055 的
	// `header [data-slot="base"]:has([data-slot="tabList"]) { flex: 0 0 auto }`
	// 与 `[data-slot="tab"] { white-space: nowrap }`）——长标签（如条数变大后的
	// 「我的列表(12345)」）会把 tab 挤窄、文字折行。这里按钮 Shrink(0)、文字
	// SingleLine，宽度随标签内容快速变化，永不换行。
	b := ui.ButtonBase(c.Key("tab-"+group+"-"+it.Key)).Height(32).Padding(0, 14).
		Shrink(0).Radius(Radius).Center().Label(it.Label)
	// 液态玻璃材质（见 drawerTabSurface）：1px 玻璃边 + --liquid-bg 底 +
	// ::before 洗光 + ::after 聚光描边；悬停 / 选中各自换色抬起。
	// 原版 `header [data-slot="tab"]:active { transform: none !important }`——
	// tab 按下不缩放（与小键 / 导航键不同），所以这里不加 pressPose。
	b = drawerTabSurface(b, active, b.Hovered())
	// 原版 tab 的文字色带 `transition: color 0.2s ease`：选中 #0369a1，
	// 悬停（未选中）#334155，常态 #475569。
	ink := drawerTabFade(b, "tab-ink", drawerTabInk(active, b.Hovered()))
	b.Children(func() {
		if it.Icon != nil {
			// 原版是 flex items-center gap-1（4px）+ 14px 图标。
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, it.Icon).Size(14, 14).TextColor(ink)
				ui.Text(c, it.Label).FontSize(13).FontWeight(600).TextColor(ink).SingleLine()
			})
			return
		}
		ui.Text(c, it.Label).FontSize(13).FontWeight(600).TextColor(ink).SingleLine()
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
		Shrink(0).Radius(Radius).Center().Label(label)
	// 收藏夹 tab 也是 HeroUI 的 `[data-slot="tab"]`（只在 .collect-tabs-scroll
	// 里横向滚动），材质与普通 tab 完全一致。
	b = drawerTabSurface(b, active, b.Hovered())
	b.Children(func() {
		ui.Text(c, label).FontSize(13).Bold().SingleLine().MaxWidth(180).
			TextColor(drawerTabFade(b, "tab-ink", drawerTabInk(active, b.Hovered())))
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
	a.headerButtonOff(c, key, label, ic, size, ink, false, false, fn)
}

// drawerHeaderPaint 是抽屉表头小键某一状态解析好的材质（原版
// `[data-slot="wrapper"] header button[class*="variant-light"]` / `variant-flat`
// / `variant-solid` 三条逐字相同的规则）：
//
//	常态  无边框、--glass-bg-active（白 .16）底 + inset 0 1px 1px --glass-highlight（白 .9）
//	悬停  --glass-bg-hover（白 .38）底 + inset 0 1px 1px 白 .9
//	      + inset 0 -1px 1px 墨 .04 + 0 4px 12px 墨 .06
//
// 按下回到 --glass-bg-active（缩放由 drawerHeaderPress 承担）。BotIn / Shadow
// 的零值（A == 0）表示该状态不画这两层。
type drawerHeaderPaint struct {
	BG     ui.Color // 底色
	Inset  ui.Color // inset 0 1px 1px（顶部内高光）
	BotIn  ui.Color // inset 0 -1px 1px（底部内暗线）
	Shadow ui.Color // 0 4px 12px（外投影）
}

// drawerHeaderPaintFor 解析常态 / 悬停两态。取值都是全站不随时段覆写的玻璃令牌。
func drawerHeaderPaintFor(hovered bool) drawerHeaderPaint {
	if hovered {
		return drawerHeaderPaint{
			BG:     ui.Hex("#ffffff").Alpha(0.38), // --glass-bg-hover
			Inset:  ui.Hex("#ffffff").Alpha(0.90), // --glass-highlight
			BotIn:  ui.Hex("#0f172a").Alpha(0.04),
			Shadow: ui.Hex("#0f172a").Alpha(0.06),
		}
	}
	return drawerHeaderPaint{
		BG:    ui.Hex("#ffffff").Alpha(0.16), // --glass-bg-active
		Inset: ui.Hex("#ffffff").Alpha(0.90),
	}
}

// drawerHeaderSurface 给抽屉表头小键（刷新 / 定位 / 全部添加 / 关注 / 播放全部）
// 铺原版那套材质。此前这里借用播放栏的 buttonSurface —— 那是
// `#player .player-button` 的材质（白 .22 底 + 1px 白 .55 边 + 两层内影 + 常驻
// 小投影），于是表头小键多出一圈白边和常驻投影、顶部内高光也暗一档
// （.55 vs 原版 --glass-highlight 的 .9）。声明成 var 便于测试换桩做反证。
var drawerHeaderSurface = func(b ui.Element, hovered bool) ui.Element {
	p := drawerHeaderPaintFor(hovered)
	b.Background(p.BG).InsetShadow(0, 1, 1, 0, p.Inset)
	if p.BotIn.A > 0 {
		b.InsetShadow(0, -1, 1, 0, p.BotIn)
	}
	if p.Shadow.A > 0 {
		b.Shadow(0, 4, 12, 0, p.Shadow)
	}
	return b
}

// drawerHeaderPressAnim 是表头小键的按压动画：原版 transition 里的
// `transform 0.1s ease`（底色 / box-shadow 是 0.18s，本轮不动那两条）。
var drawerHeaderPressAnim = ui.Animation{
	Duration: 100 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// drawerHeaderPress 给表头小键加原版那套按压手感：**只按下缩到 .94、悬停不上浮**
// —— 原版表头按钮只有 `:active { transform: scale(0.94) }`，没有 :hover 位移规则
// （与 `.nav-icon-btn` 的悬停上浮不同）。每帧都要调（未按下时把目标写回 0），
// 否则姿势会停在上一帧的 .94。声明成 var 便于测试换桩。
var drawerHeaderPress = func(b ui.Element) {
	pose := ui.Pose{}
	if b.Pressed() {
		pose = ui.Pose{Scale: 0.94}
	}
	b.AnimatePose(pose, drawerHeaderPressAnim)
}

// drawerClosePaint 是抽屉右上角关闭键在某一状态下解析好的样式。原版
// globals.css:4112-4140 的 `button[aria-label="Close"]` 三条规则：
//
//	常态  background --glass-bg（白 .24）+ 1px --glass-border（白 .55）
//	      + inset 0 1px 1px --glass-highlight（白 .9）
//	      + inset 0 -1px 1px 墨 .04 + 0 6px 16px 墨 .08
//	悬停  background --glass-bg-hover（白 .38），投影抬到 0 8px 20px 墨 .10
//	按下  background --glass-bg-active（白 .16）、transform: scale(.9)
//
// 原版 :active 只覆写 background 与 transform，所以按下态沿用常态的描边 /
// 内影 / 投影（只有 :hover 才换投影），不能「先铺常态再叠悬停」。
type drawerClosePaint struct {
	BG         ui.Color
	Border     ui.Color
	Inset      ui.Color
	BotIn      ui.Color
	Shadow     ui.Color
	ShadowY    float32
	ShadowBlur float32
	// Scale 是按下时的缩放，其余状态为 1。
	Scale float32
}

// drawerClosePaintFor 解析常态 / 悬停 / 按下三态。
func drawerClosePaintFor(hovered, pressed bool) drawerClosePaint {
	p := drawerClosePaint{
		BG:         ui.Hex("#ffffff").Alpha(0.24), // --glass-bg
		Border:     ui.Hex("#ffffff").Alpha(0.55), // --glass-border
		Inset:      ui.Hex("#ffffff").Alpha(0.90), // --glass-highlight
		BotIn:      ui.Hex("#0f172a").Alpha(0.04),
		Shadow:     ui.Hex("#0f172a").Alpha(0.08),
		ShadowY:    6,
		ShadowBlur: 16,
		Scale:      1,
	}
	if hovered {
		p.BG = ui.Hex("#ffffff").Alpha(0.38) // --glass-bg-hover
		p.BotIn = ui.Hex("#0f172a").Alpha(0.05)
		p.Shadow = ui.Hex("#0f172a").Alpha(0.10)
		p.ShadowY, p.ShadowBlur = 8, 20
	}
	if pressed {
		p.BG = ui.Hex("#ffffff").Alpha(0.16) // --glass-bg-active
		p.Scale = 0.9
	}
	return p
}

// drawerCloseSurface 把解析出的玻璃材质铺到关闭键上。常驻（不再只在悬停时画），
// 否则关闭键在常态是一枚裸图标。声明成 var 便于测试换桩做反证（同 drawerHeaderSurface）。
var drawerCloseSurface = func(b ui.Element, hovered, pressed bool) ui.Element {
	p := drawerClosePaintFor(hovered, pressed)
	return b.Background(p.BG).Border(1, p.Border).
		InsetShadow(0, 1, 1, 0, p.Inset).
		InsetShadow(0, -1, 1, 0, p.BotIn).
		Shadow(0, p.ShadowY, p.ShadowBlur, 0, p.Shadow)
}

// drawerClosePressAnim 是关闭键的 :active 过渡：原版 `transition: ... transform
// 0.1s ease`。色值由 drawerCloseSurface 当帧铺，这里只补缩放这一路。
var drawerClosePressAnim = ui.Animation{Duration: 100 * time.Millisecond, Ease: ui.EaseOut}

// drawerClosePress 给关闭键加原版 :active 的 scale(.9)。**悬停无位移**（原版这条
// 规则没有 :hover 的 transform），所以不悬停时把目标姿势写回 0，避免停在上一帧。
var drawerClosePress = func(b ui.Element) {
	pose := ui.Pose{}
	if b.Pressed() {
		pose = ui.Pose{Scale: 0.9}
	}
	b.AnimatePose(pose, drawerClosePressAnim)
}

// headerButtonOff 同 headerButton，但可置为加载形态。对应原版抽屉表头里
// 两种**不同的**按钮实现：
//
//	spin=true     historyList 的 <Button isDisabled={isRefreshing}> +
//	              <Refresh className="history-refresh-spinning">：
//	              图标 800ms/圈 匀速自转，按钮置灰。
//	spinner=true  danmakuList 的
//	              <Button isLoading={isLoadingDanmaku || isLoadingReply}>：
//	              HeroUI 的 isIconOnly 分支在 isLoading 时**丢掉 children**
//	              只画 <Spinner>，并且 isDisabled = isDisabledProp || isLoading
//	              （所以加载中既转圈又点不动）。
//
// 两种形态都属 HeroUI <Button>：禁用不透明度是框架 token opacity-disabled
// （= .5），不是 `.nav-icon-btn:disabled` 的 .34。框架对禁用节点会再乘 .5，
// 所以这里不写元素不透明度（默认 1 → 绘制 .5）；**不能**沿用 disableNav。
func (a *App) headerButtonOff(c *ui.Context, key, label string, ic *ui.SVG, size float32, ink ui.Color, spin, spinner bool, fn func()) {
	b := ui.ButtonBase(c.Key(key)).Size(32, 32).Radius(Radius).Center().Label(label).Tooltip(label)
	b.Disabled(spin || spinner)
	drawerHeaderSurface(b, b.Hovered())
	drawerHeaderPress(b)
	b.Children(func() {
		if spinner {
			// 原版 Spinner size="sm"（HeroUI button.sm → spinner sm），w-5 h-5。
			// 起个名字给辅助技术（也是无头测试认它的锚点：HeroUI 的 Spinner
			// 同样带 role=status）。
			ui.Spinner(c).Size(20, 20).TextColor(ink).Label("加载中")
			return
		}
		icon := ui.Icon(c, ic).Size(size, size).TextColor(ink)
		if refreshKeySpin(spin, reduceMotion(c)) {
			spinRefreshIcon(icon, b.Loop("spin", refreshSpinPeriod, ui.Linear))
		}
	})
	if b.Clicked() {
		fn()
	}
}

// refreshKeyState 是抽屉表头刷新键这一帧的形态（对应原版两种不同的按钮实现，
// 见 headerButtonOff 的注释）。
type refreshKeyState struct {
	// Label 是文案 / tooltip（各列表抽屉按原版 aria-label，见 refreshLabelFor）。
	Label string
	// Spin 为真时图标自转（历史抽屉 .history-refresh-spinning）。
	Spin bool
	// Spinner 为真时用 Spinner 替掉图标（弹幕/评论抽屉 HeroUI isLoading）。
	Spinner bool
}

// Off 报告刷新键这一帧是否禁用：两种加载形态都禁用（原版 isDisabled /
// isLoading 都会把按钮置为 disabled）。
func (s refreshKeyState) Off() bool { return s.Spin || s.Spinner }

// refreshKeySpin 报告历史抽屉刷新键的图标这一帧是否自转：正在刷新且桌面没有
// 要求减少动态效果。原版 `.history-refresh-spinning` 在
// `@media (prefers-reduced-motion: reduce)` 下是 `animation: none` —— 图标
// 停住但按钮仍保持 disabled（禁用与自转是两件事）。
func refreshKeySpin(spinning, reduce bool) bool {
	return spinning && !reduce
}

// spinRefreshIcon 给历史刷新图标套上自转（进度 0→1 转满一圈）。拆成 var
// 便于测试观察接线：减少动态效果下这一层不该被调到。
var spinRefreshIcon = func(icon ui.Element, progress float32) {
	icon.Rotate(spinDegrees(true, progress))
}

// refreshKeyStateFor 按当前抽屉给出刷新键的形态：
//
//	history    <Button isDisabled={isRefreshing}> + 图标自转；文案随 tab。
//	danmaku    <Button isLoading={isLoadingDanmaku || isLoadingReply}>：
//	           弹幕预取/刷新在途、或评论在途（含滚到底自动翻页）都转圈。
//	其余抽屉    常态（其中动态 / 热门推荐 / 收藏三个列表的文案见 refreshLabelFor）。
func refreshKeyStateFor(drawer, histTab string, histLoading, danmakuLoading, repliesLoading bool) refreshKeyState {
	s := refreshKeyState{Label: refreshLabelFor(drawer, histTab)}
	switch drawer {
	case "history":
		s.Spin = histLoading
	case DrawerDanmaku:
		s.Spinner = danmakuLoading || repliesLoading
	}
	return s
}

// refreshLabelFor 给出刷新键的文案 / 无障碍名（原版每个列表抽屉各自的
// aria-label，见各组件源码）：
//
//	feed      feedList.tsx       aria-label="刷新动态列表"
//	popular   recommendList.tsx  aria-label="刷新当前列表"
//	favorite  collectList.tsx    aria-label="刷新收藏列表"
//	history   historyList.tsx    随 tab：观看历史 / 稍后再看
//	其余      danmaku / up 等原版按钮没有 aria-label，沿用通用的「刷新」。
func refreshLabelFor(drawer, histTab string) string {
	switch drawer {
	case "history":
		return historyRefreshLabel(histTab)
	case "feed":
		return "刷新动态列表"
	case "popular":
		return "刷新当前列表"
	case "favorite":
		return "刷新收藏列表"
	default:
		return "刷新"
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
	// 「滚动时锁住上浮」的帧首闸门：默认松开，下面哪个含卡片的滚动容器
	// 在滚，就把它按住（滚动停下当帧自动恢复，见 scrollHold）。
	a.scrollHoldHeld = false
	if a.Drawer == DrawerDanmaku {
		if a.DanmakuTab == TabDanmaku {
			// 弹幕 tab 自己用虚拟化 ui.List 当滚动容器（见 danmakuBody）。
			a.danmakuBody(c)
			return
		}
		// 评论文本不多但行高不一：repliesBody 自己用虚拟化 ui.List 当滚动容器
		//（见 repliesBody）。
		a.repliesBody(c)
		return
	}
	// 卡片网格：虚拟化列表自己滚，顺带负责到底翻页。
	if a.usesCardGrid() {
		// 退场时不再翻页（内容马上就看不见了）。
		if a.listBody(c) && !a.drawerClosing && a.Act.LoadMore != nil {
			// 内容超过一屏且滚到底（first>0）才翻页：与旧逻辑的 MaxY>0 一致，
			// 避免首屏不满却反复请求空页。canAutoLoadMore 还会拦住「上一次翻页
			// 失败」的情况，否则会每帧重发。
			if first, last := l.Grid.Visible(); canAutoLoadMore(l, first, last, a.gridRows(c)) {
				a.Act.LoadMore()
			}
		}
		return
	}

	// 选集也是三列卡片网格，同样用虚拟化 ui.List（只是不翻页）。
	if a.Drawer == DrawerParts {
		a.partsBody(c)
		return
	}

	// 播放列表记录行也用虚拟化 ui.List（可能上百条，逐行全建会把上百张
	// 缩略图每帧抢缓存、互相挤出 LRU，用户报「左侧图标不显示」）。
	if a.Drawer == DrawerPlaylist {
		a.playlistBody(c)
		return
	}

	// 其余抽屉内容不多，统一套一层滚动。
	//
	// 注意 TrackScroll 是双向绑定：容器每帧从这个状态读位置、滚动时写回。
	// 所以必须传**稳定地址**（List 常驻在 map 里），不能按值拷进出 ——
	// 拷贝会让容器每帧被旧位置拽回去，表现为触摸板/滚轮滚不动。
	a.holdScrollBody(c, l)
	ui.Scroll(c).FillWidth().Grow(1).Padding(8, 24).TrackScroll(&l.Scroll).Children(func() {
		switch a.Drawer {
		case DrawerUp:
			// UP 空间有两个 tab：视频是卡片列表（上面已处理），合集是一排胶囊。
			a.seriesListBody(c)
		}
	})
}

// usesCardGrid 报告当前抽屉是否是三列卡片网格（搜索/分区/UP 视频/合集）。
// 合集列表、播放列表内容少，仍走 ui.Scroll；选集也是三列网格，但单独走
// partsGrid（它用同一个虚拟化 List，只是不翻页）。
func (a *App) usesCardGrid() bool {
	switch a.Drawer {
	case DrawerParts, DrawerPlaylist, DrawerDanmaku:
		return false
	case DrawerUp:
		return a.UpTab != UpTabSeries
	}
	return true
}

// gridRows 是当前卡片网格的行数（每行 gridColsFor 张）。宽度断点见
// cardGridBreakpoint：窄窗两列时行数翻倍，翻页判定必须跟着变。
func (a *App) gridRows(c *ui.Context) int {
	n := len(a.list().Cards)
	cols := gridColsFor(windowWidth(c))
	return (n + cols - 1) / cols
}

// seriesListBody 是 UP 空间的「合集」tab（原版 upVideoList.tsx 的合集列表）：
// 一排下拉的合集胶囊，点一个就进那个合集。
func (a *App) seriesListBody(c *ui.Context) {
	switch {
	case a.list().Loading && len(a.SeriesList) == 0:
		a.emptyState(c, "加载中…")
		return
	case len(a.SeriesList) == 0:
		a.emptyState(c, "暂无合集视频")
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

// cardGridBreakpoint 是原版所有列表抽屉网格的 Tailwind `sm` 断点
// （min-width: 640px）。原版抽屉里的网格一律是
// `gap-2 grid grid-cols-2 sm:grid-cols-3`（searchList / collectList /
// recommendList / upVideoList / feedList / historyList / seriesList /
// pageList 全都是这一行），骨架屏 .list-skeleton-grid 用的也是同一对
// （globals.css 里单独写了一条 `@media (min-width: 640px)`）。Tailwind 的
// `sm:` 判定的是**视口**宽度（不是容器宽度），所以这里取窗口宽度：窄窗两列、
// ≥640 三列。
const cardGridBreakpoint = 640

// gridColsFor 返回卡片网格的列数：<640 两列、≥640 三列（原版 sm 断点）。
func gridColsFor(winW float32) int {
	if winW < cardGridBreakpoint {
		return 2
	}
	return 3
}

// listBody 是搜索结果 / 分区 / UP 视频 / 合集等卡片网格。返回是否画了真正的
// 卡片（供 drawerBody 决定要不要检查翻页；骨架/空态不算）。
func (a *App) listBody(c *ui.Context) bool {
	l := a.list()
	switch {
	case l.Loading && len(l.Cards) == 0:
		// 原版只有 searchList / collectList / recommendList 三处 import 了
		// ListSkeleton（见 skeletonDrawers），其余卡片网格抽屉首载渲染的是空
		// 网格。这里同源：只有这三个抽屉铺骨架屏，其余留空（空态仍由下一分支
		// 在加载完成后给出）。
		a.scrollBody(c, l, func() {
			if skeletonFor(a.Drawer) {
				a.skeletonGrid(c)
			}
		})
		return false
	case len(l.Cards) == 0:
		a.scrollBody(c, l, func() { a.emptyState(c, a.emptyHint()) })
		return false
	}
	a.cardGrid(c, l)
	return true
}

// scrollBody 把短小的内容（骨架屏 / 空态）套一层 ui.Scroll。
func (a *App) scrollBody(c *ui.Context, l *List, body func()) {
	a.holdScrollBody(c, l)
	ui.Scroll(c).FillWidth().Grow(1).Padding(8, 24).TrackScroll(&l.Scroll).
		Children(body)
}

// holdScrollBody 观测普通 ui.Scroll 容器的滚动（评论 / 骨架屏等），滚动
// 中按住上浮锁。ScrollTo 动画或偏移变化都算「在滚」。
func (a *App) holdScrollBody(c *ui.Context, l *List) {
	if !scrollHoldEnabled {
		return
	}
	locked, needFrame := l.liftHold.observe(float64(l.Scroll.Y), true)
	if l.Scroll.Scrolling() {
		locked, needFrame = true, true
	}
	if needFrame {
		c.AnimationFrame()
	}
	a.scrollHoldHeld = a.scrollHoldHeld || locked
}

// holdListScroll 观测虚拟化 ui.List（卡片网格 / 选集网格 / 播放列表）的
// 滚动。滚轮/拖拽的偏移在事件里已同步写回 ScrollState（ui.scrollBy →
// state.scrollTo），本帧构建就能读到；再叠上首行号，把 List 自己重排
// （行高变化改锚点）引起的偏移变化也一并算进去。首行号左移 20 位与偏移
// 拼成一个 float64：偏移精度不受首行大小影响（float32 会在长列表里丢掉
// 个位数的慢速滚动）。
func (a *App) holdListScroll(c *ui.Context, hold *scrollHold, st *ui.ListState, sc *ui.ScrollState, shown bool) {
	if !scrollHoldEnabled {
		return
	}
	offset := float64(0)
	if shown {
		first, _ := st.Visible()
		offset = float64(first)*1048576 + float64(sc.Y)
	}
	locked, needFrame := hold.observe(offset, shown)
	if sc.Scrolling() {
		locked, needFrame = true, true
	}
	if needFrame {
		c.AnimationFrame()
	}
	a.scrollHoldHeld = a.scrollHoldHeld || locked
}

// cardGrid 用 mygo 的虚拟化 ui.List 画卡片网格（列数按窗口宽度的 sm 断点，见
// gridColsFor）：每个 List 行是一排最多 cols 张卡，行高由框架量（卡片封面
// 100 + 单行标题，所以每排等高）。
//
// 之前这里是 ui.Column 逐行构建：滚动时视图每帧重建，会把**所有**卡片（含
// 几百张）连同各自的材质一起重建——这是「订阅/动态」等列表滚动卡顿的根因
// （与上一轮弹幕列表恢复 ui.List 同一机制，见 danmakuBody）。ui.List 只建
// 可见的几排。
func (a *App) cardGrid(c *ui.Context, l *List) {
	st, cards := &l.Grid, l.Cards
	// 列数按窗口宽度的 sm 断点（原版 grid-cols-2 sm:grid-cols-3）。
	cols := gridColsFor(windowWidth(c))
	rows := (len(cards) + cols - 1) / cols
	// 翻页失败时在网格末尾多铺一行「加载失败，点击重试」（原版 historyList
	// 的 .history-load-more-retry）——它是列表的最后一个虚拟行，跟着一起滚。
	extra := 0
	if l.LoadErr {
		extra = 1
	}
	// 行键取首张卡的 bvid：翻页追加 / 滑动窗口裁掉头部时锚点不跳。
	st.Key = func(r int) any {
		if i := r * cols; i < len(cards) && cards[i].Bvid != "" {
			return cards[i].Bvid
		}
		if r == rows && extra == 1 {
			return "loaderr:" + a.Drawer
		}
		return r
	}
	a.prefetchCardCovers(st, cards, cols)
	a.holdListScroll(c, &l.liftHold, st, &l.GridScroll, true)
	ui.List(c, st, rows+extra, func(r int) {
		if r == rows {
			a.loadMoreRetry(c, l)
			return
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
			start := r * cols
			end := min(start+cols, len(cards))
			for j := start; j < end; j++ {
				a.card(c, j)
			}
			for j := end; j < start+cols; j++ {
				ui.Box(c).Grow(1).Basis(0)
			}
		})
	}).FillWidth().Grow(1).Gap(8).Padding(8, 24).TrackScroll(&l.GridScroll)
}

// canAutoLoadMore 报告卡片网格这一帧该不该自动翻页：有更多、不在加载中、
// 上一次翻页没失败，且已经滚到底（首行不在顶部）。抽成纯函数便于单测。
//
// `LoadErr` 是这里的核心闸门：mygo 的翻页是在渲染里触发的，而渲染每帧都跑，
// 所以一次失败若不记下来，下一帧就会立刻重发，变成请求/提示风暴。
func canAutoLoadMore(l *List, first, last, rows int) bool {
	if !l.HasMore || l.Loading || l.LoadErr {
		return false
	}
	return first > 0 && last >= rows-1
}

// loadMoreRetry 是网格末尾的「加载失败，点击重试」行（原版
// .history-load-more-retry：整宽、rose 淡底 + rose 细边、13px #be123c）。
func (a *App) loadMoreRetry(c *ui.Context, l *List) {
	b := ui.ButtonBase(c.Key("list-load-retry")).FillWidth().Height(38).
		Margin(10, 0, 4, 0).Radius(10).Center().
		Label("加载失败，点击重试").Tooltip("加载失败，点击重试")
	b.Background(ui.Hex("#f43f5e").Alpha(0.06)).
		Border(1, ui.Hex("#f43f5e").Alpha(0.20))
	if b.Hovered() {
		b.Background(ui.Hex("#f43f5e").Alpha(0.12))
	}
	b.Children(func() {
		ui.Text(c, "加载失败，点击重试").FontSize(13).
			TextColor(ui.Hex("#be123c"))
	})
	// 加载中不重复触发；点一次清掉错误态再重试（同原版按钮）。
	if b.Clicked() && !l.Loading {
		l.LoadErr = false
		if a.Act.LoadMore != nil {
			a.Act.LoadMore()
		}
	}
}

// initialCardRows 是列表还没布局过时预取的初始行数。一屏约放 5 行（1400×900
// 抽屉展开后），给 6 留点余量。
const initialCardRows = 6

// prefetchCardCovers 只给可见行（外加几行 overscan）的卡片请求封面，
// 同时把上一帧预取、本帧已滚出窗口的封面请求取消（原版浏览器对滚出
// loading=lazy 视口的图取消加载；见 imagecache.Cancel —— 只退排队的，
// 已在途的照常入库，将来滚回来直接命中）。
//
// 从前 renderCard 在构建时直接 Bitmap()：而 mygo 的 List 在行高还没量到之前，
// 用主题 Space(8)（默认 Spacing 4 → 32px）当估计（mygo ui/list.go:381 的
// heights.def），所以抽屉刚开、一屏实际只放得下 ~5 行时，它会先建 30+ 行，
// 于是一次请求 90+ 张封面并立刻并发 decode/上传。这里把「请求」和「构建」拆开：
// 构建只 Get（读缓存），请求只按上一帧的可见窗口发。首帧还没布局
// （Visible 返回 0,-1）时只预取头几行，绝不铺开屏外的图。
//
// 可见行先 Touch 再 Bitmap：抓取队列优先服务当前可见的图，滚动时新可见的
// 卡片不会被先到的屏外预取插队（原版浏览器 loading=lazy 的可见优先语义）。
func (a *App) prefetchCardCovers(st *ui.ListState, cards []Card, cols int) {
	a.prefetchWindowCovers(st, len(cards), cols,
		func(i int) string { return cards[i].Cover })
}

// prefetchPartCovers 是选集网格版的封面预取（同一套窗口，封面地址走 partCover）。
func (a *App) prefetchPartCovers(st *ui.ListState, parts []Part, cols int) {
	a.prefetchWindowCovers(st, len(parts), cols,
		func(i int) string { return a.partCover(parts[i]) })
}

// prefetchWindowCovers 是两个网格预取的公共实现：本帧窗口内的图 Touch（可见
// 行）+ Bitmap（发起/命中），上一帧在窗口内、本帧已滚出的 Cancel 退队。
// cols 是这一帧的网格列数（见 gridColsFor）——行→卡片下标的换算必须与
// cardGrid / partsGrid 的构建用同一个值，否则窄窗下预取窗口会错位。
func (a *App) prefetchWindowCovers(st *ui.ListState, n, cols int, cover func(int) string) {
	first, last := prefetchWindow(st, n, cols)
	visFirst, visLast := visibleCards(st, n, cols)
	for i := first; i < last; i++ {
		if i >= visFirst && i < visLast {
			a.Images.Touch(cover(i), imagecache.WidthCard)
		}
		a.Images.Bitmap(cover(i), imagecache.WidthCard)
	}
	a.cancelOffscreen(n, first, last, cover)
}

// cancelOffscreen 把滚出预取窗口的封面请求退队。窗口为空（len==0）时视为
// 「没有请求过」，不取消 —— 切抽屉/换列表的空窗口一帧里所有封面都会被退，
// 而新列表的 Bitmap 还没来得及发。
func (a *App) cancelOffscreen(n, first, last int, cover func(int) string) {
	if n == 0 || last <= first {
		return
	}
	for i := 0; i < n; i++ {
		if i < first || i >= last {
			a.Images.Cancel(cover(i), imagecache.WidthCard)
		}
	}
}

// visibleCards 把上一帧的可见**行**换算成卡片下标区间 [first, last)。
// 未布局过（Visible 返回 0,-1）时按初始几行算。
func visibleCards(st *ui.ListState, n, cols int) (first, last int) {
	firstRow, lastRow := st.Visible()
	if lastRow < firstRow {
		firstRow, lastRow = 0, initialCardRows-2
	}
	return min(n, max(0, firstRow)*cols), min(n, (lastRow+1)*cols)
}

// prefetchWindow 返回网格列表应当预取的卡片下标区间 [first, last)。
// 未布局（Visible 返回 0,-1）时只覆盖初始几行；布局后覆盖可见行上下各一行。
func prefetchWindow(st *ui.ListState, n, cols int) (first, last int) {
	firstRow, lastRow := st.Visible()
	if lastRow < firstRow {
		// 还没布局过：只预取预估的首屏。lastRow 是闭区间上界。
		firstRow, lastRow = 0, initialCardRows-1
	}
	first = max(0, firstRow-1) * cols
	last = min(n, (lastRow+2)*cols)
	return first, last
}

// skeletonItems 是骨架屏占位卡的总数。原版 ListSkeleton 的 `rows` 默认 6，
// 三个调用点（searchList / collectList / recommendList）都用默认值，即**固定
// 6 张**——不是「按列数铺满行数」。所以三列时是 2 排、两列时是 3 排；此前原生版
// 写死 `for row := 0; row < 3`，宽窗（≥640 三列）下多铺了一整排（9 张 vs 原版 6 张）。
const skeletonItems = 6

// skeletonDrawers 是原版 ListSkeleton 的**调用点**：searchList / collectList /
// recommendList（main:src/components/listSkeleton.tsx 只被这三个文件 import）。
// 其余卡片网格抽屉（动态 feed / 历史 history / UP 空间 up / 合集 series）首载
// 渲染的是空网格，不铺骨架屏。键与 Sections / DrawerSearch 同源。
var skeletonDrawers = map[string]bool{
	DrawerSearch: true, // searchList
	"favorite":   true, // collectList
	"popular":    true, // recommendList（热门与推荐）
}

// skeletonFor 报告某个抽屉首载是否该铺骨架屏（原版 ListSkeleton 的调用点）。
func skeletonFor(drawer string) bool { return skeletonDrawers[drawer] }

// skeletonRows 返回骨架屏的行数：把 skeletonItems 张卡按 cols 张一排铺开。
func skeletonRows(winW float32) int {
	cols := gridColsFor(winW)
	return (skeletonItems + cols - 1) / cols
}

// skeletonEase 是原版 `.list-skeleton-item { animation: list-skeleton-pulse
// 1.4s ease-in-out infinite }` 的 CSS 曲线。CSS 的 `ease-in-out` 是
// cubic-bezier(0.42, 0, 0.58, 1)；mygo 的 ui.EaseInOut 是标准三次 in-out，
// 与它在 1/8 周期处相差约 0.03 不透明度（0.578 vs 0.608）。直接用框架的
// CubicBezier 逐字对齐原版曲线。
var skeletonEase = ui.CubicBezier(0.42, 0, 0.58, 1)

// skeletonPulse 把一次脉冲周期里的线性进度（0→1）换算成骨架卡片的不透明度，
// 对齐原版 `.list-skeleton-item` 的 `animation: list-skeleton-pulse 1.4s
// ease-in-out infinite`：keyframes 0%/100% → .55、50% → 1，且分段走 CSS
// `ease-in-out`（所以是「缓入缓出的三角波」，不是线性三角波）。
func skeletonPulse(ph float64) float64 {
	tri := 1 - abs(2*ph-1) // 0 → 1 → 0 的三角波
	return 0.55 + 0.45*float64(skeletonEase(float32(tri)))
}

// skeletonGrid 是列表加载中的骨架屏（原版 ListSkeleton）：灰底占位卡片。
func (a *App) skeletonGrid(c *ui.Context) {
	// 原版 .list-skeleton-*：卡片底 rgba(255,255,255,.4)、封面 16:10 圆角 8、
	// 两行文本条（12px 圆角 6，第二行 60% 宽），opacity 1.4s 脉冲。
	cover := ui.Hex("#64748b").Alpha(0.16)
	line := ui.Hex("#64748b").Alpha(0.16)
	card := ui.Hex("#ffffff").Alpha(0.40)
	// 骨架屏与真卡片共用同一个 sm 断点：.list-skeleton-grid 基线是
	// repeat(2, 1fr)，globals.css 单独写一条 @media (min-width: 640px) 才变三列。
	cols := gridColsFor(windowWidth(c))
	rows := skeletonRows(windowWidth(c))
	n := 0
	// 原版 `.list-skeleton-grid` 带 `aria-hidden="true"`（纯装饰，对读屏隐藏）。
	// mygo 没有 aria-hidden，但有等价的 `RoleNone`（“把元素本身从无障碍树里
	// 拿掉，保留子元素”）：骨架元素本来就都是无标签的裸 Box，子元素也不会被
	// 读屏播报，这里再显式给容器标一个装饰态。
	ui.Column(c).Gap(8).Role(ui.RoleNone).Children(func() {
		for row := 0; row < rows; row++ {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
				for j := 0; j < cols && n < skeletonItems; j++ {
					n++
					// 原版是 1.4s 的 opacity 脉冲（.list-skeleton-pulse：
					// 0.55 ↔ 1，ease-in-out）。用 Loop 的线性进度驱动。
					item := ui.Column(c).Key(fmt.Sprintf("skel-%d-%d", row, j)).
						Grow(1).Basis(0).Padding(8).Radius(12).Background(card).Gap(0)
					ph := float64(item.Loop("skel", 1400*time.Millisecond, ui.Linear))
					item.Opacity(float32(skeletonPulse(ph)))
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
	// 点进某个合集后的空态（原版 seriesList 的 role="status" 文案）——
	// 此前漏了这一支，回落到通用「没有内容」。
	if a.Drawer == DrawerSeries {
		return "暂无合集视频"
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
		Badge:    card.Badge,
		Duration: card.Duration,
		Active:   playing,
		Playing:  playing,
		Lazy:     true,
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
			b := ui.ButtonBase(c.Key("wl-remove-"+card.Bvid)).
				Size(wlRemoveSize, wlRemoveSize).Radius(wlRemoveRadius).
				Center().Label("从稍后再看移除").Tooltip("从稍后再看移除")
			// 原版 .watchlater-remove-btn:disabled 是 opacity .55（绝对值）。
			// mygo 的禁用节点会再乘 .5（ui/paint.go），.55 会被压到 .275，
			// 而元素不透明度封顶 1 最多只能补到 .5，补不回来（见 disabled.go）。
			// 所以这里不借框架的 Disabled，改用原版同样的语义「自己挡住交互」
			//（下面 Clicked 已带 !pending），只把那层绝对值落到 Opacity 上。
			// 原版 .watchlater-remove-btn:disabled 同时把 transform 归零，所以
			// 悬停放大在请求中也关掉。
			b.Absolute().Right(wlRemoveInset).Top(wlRemoveInset)
			b.Background(wlRemoveFill(b.Hovered() && !pending)).
				Border(1, wlRemoveBorder())
			b.Opacity(wlRemoveOpacity(pending))
			// 原版 `.watchlater-remove-btn:focus-visible`：`outline: 2px solid
			// rgba(14,165,233,.62); outline-offset: 2px`。
			focusOutline(b, wlRemoveRadius, focusSky62)
			// 悬停放大 1.04（原版 :hover transform: scale(1.04)）；禁用时不放大。
			b.AnimatePose(ui.Pose{Scale: wlRemoveScale(b.Hovered() && !pending)},
				ui.Animation{Duration: 160 * time.Millisecond, Ease: ui.EaseOut})
			b.Children(func() {
				ui.Icon(c, iconClose).Size(wlRemoveIconSize, wlRemoveIconSize).
					TextColor(wlRemoveInk(b.Hovered()))
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
	Key   string
	Cover string
	Title string
	// Badge 是标题前的小胶囊（原版 feedList 的 titlePrefix，如「充电专属」）；
	// 空则不画。
	Badge    string
	Duration string
	Fields   []cardField
	// Active 表示「正在播放」：蓝 Tint + 2px 蓝边。
	Active  bool
	Playing bool
	// Cover2 是叠在封面上的附加内容（添加键、移除键…）。
	Cover2  func(*ui.Context)
	OnClick func()
	// Lazy 为真时只读缓存、不发起抓取：封面由调用方按可见窗口预取
	//（虚拟化卡片网格 / 选集网格）。非虚拟化的短列表（播放列表）为假，
	// 它们每帧都建全部卡片，直接请求就行。
	Lazy bool
}

// renderCard 画一张列表卡片。
//
// 搜索结果 / 推荐 / 收藏 / 稍后再看 / UP 空间 / **选集**……全走这一套：
// 原版就是一个 ListCard 组件，各列表只传不同的 fields 与 coverChildren。
// 之前选集自己另写了一份（padding 6、圆角 8、封面 16:9），和别的列表对
// 不上 —— 现在合成一份，样式不再碎片化。
// 卡片页脚（原版 `[data-slot="footer"]`）里两条文本的行盒：标题是 HeroUI 的
// text-small（14px / 20px 行高），meta 行是 Tailwind 的 text-xs（12px / 16px
// 行高）。框架的自然行盒分别是 22 / 18（≈1.5× 字号），页脚因此比原版高 4px、
// 每张卡都偏高。用显式盒高 + 字形上移压回原版（见 linebox.go 的 textSmLH /
// textXsLH）。
// cardFooterBG 是卡片页脚的洗色：原版 `[data-slot="footer"]` 的
// `background: rgba(255,255,255,.22) !important`；悬停时该元素随
// `--hover-strength=1` 涨到 `.22 + 1×.14 = .36`。此前误写成 .12（漏了
// 一位），页脚比原版暗一档。
func cardFooterBG(hovered bool) ui.Color {
	if hovered {
		return ui.Hex("#ffffff").Alpha(0.36)
	}
	return ui.Hex("#ffffff").Alpha(0.22)
}

// cardTitleBadgePadY 是卡片标题前胶囊的上下内边距。原版 feedList.tsx 的
// titlePrefix span 是 `px-1 py-0.5`，py-0.5 = 2px。声明成常量便于测试钉值。
const cardTitleBadgePadY = 2

func (a *App) renderCard(c *ui.Context, spec cardSpec) {
	t := a.Theme
	// 原版抽屉卡片（globals.css 的 [role=button][class*=bg-content]）：
	// 圆角 12、半透明填充（近似玻璃底）、封面通栏（.img-container 高 100）、
	// 页脚另有一层浅底。
	box := ui.Column(c).Key(spec.Key).Grow(1).Basis(0).
		Radius(12).Clip()
	// 滚动锁：滚动进行中把 hover 视觉一起冻住，而不只是锁上浮位移。
	// 每帧都要在算 hover 之前定下（比较 renderCard 内的 lift/glow 也用它）。
	locked := a.scrollHoldHeld
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
	//
	// hover 在滚动中冻结：ui.List 滚动时卡片在指针下滑过，box.Hovered() 会
	// 在相邻卡片之间来回跳，底/边/页脚洗色每帧翻转——用户看到的就是「选中
	// 抖动」。锁定期间一律按常态渲染（在播卡片的 Active 态与指针无关，照旧）。
	hovered := box.Hovered() && !locked
	if hovered {
		a.hoverChannels.Add(1)
	}
	switch {
	case spec.Active:
		// 原版选集卡片的在播态就是 border-2 border-[#0284c7]。
		box.Background(t.Blue.Alpha(0.22)).
			Border(2, ui.Hex("#0284c7")).
			Shadow(0, 10, 28, 0, ui.Hex("#0f172a").Alpha(0.12))
	case hovered:
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
	// 用 160ms 曲线，不用弹簧：用户报 500ms 的弹簧太慢、移开回落不利落。
	//
	// 滚动时锁住：在播卡片的 -3px 是常驻的，ui.List 滚动时可见行重建，
	// 每帧都把 lift 动画重设目标（rested 动画仍在动画列表里，引擎每帧
	// 照常 build/layout/paint 整窗）——用户报快速滚动卡顿正是来自这里。
	// 滚动进行中不挂动画、停下当帧再恢复：
	// 悬停 / 在播的上浮效果只在静止时渲染，与原版 :hover 观感一致。
	if !locked {
		lift := float32(0)
		if box.Hovered() || spec.Active {
			lift = -3
		}
		a.liftChannels.Add(1)
		box.Translate(0, box.AnimateTo("lift", lift, cardLiftAnim))
	}

	// 指针跟随光晕（原版卡片 ::before 的径向高光，跟着鼠标走）。只给被悬停
	// 的那张挂上：PointerPosition 会让引擎在指针每次移动时补帧，全网格都挂就
	// 成了「鼠标一动重建整窗」。
	if !locked && box.Hovered() {
		cardGlow(box)
	}

	box.Children(func() {
		// 封面：通栏、固定 100px（原版 .img-container / .c-cover）。
		ui.Box(c).FillWidth().Height(100).Shrink(0).
			Background(t.CoverPlaceholder()).Clip().Children(func() {
			bmp := a.Images.Get(spec.Cover, imagecache.WidthCard)
			if !spec.Lazy {
				bmp = a.Images.Bitmap(spec.Cover, imagecache.WidthCard)
			}
			if bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			} else if placeholderCover != nil {
				// 原版 ListCard 的 RetryImg 是 `src={src || fallbackSrc}`：空封面
				// 也画 /cover.png（原版数据结构里 cover 可以是 ""，首帧未填充的
				// 卡片就是这种）。所以这里不再按 `spec.Cover != ""` 分岔，一律铺
				// 占位图 —— 解码好的真图下一帧盖掉，渐进式加载。
				ui.Image(c, placeholderCover).Fill().Fit(ui.Cover)
			}
			// 时长角标（原版 .c-cover-duration）：右下贴角，左上圆角 5。
			// 原版 `font-size:11px; line-height:1.2; padding:2px 5px`（盒高
			// 2+13.2+2 = 17.2）。框架的自然行盒（11px = 17）下不去，角标会
			// 比原版高约 4px、盖住更多封面；用显式盒高 + 字形上移复现（见
			// linebox.go）。
			if spec.Duration != "" {
				dh, dsh := textLineBox(11, 1.2)
				numericText(c, spec.Duration).FontSize(11).TextColor(ui.Hex("#f8fafc")).
					Absolute().Right(0).Bottom(0).
					Height(dh + 4).Translate(0, dsh).
					Padding(2, 5).Radius(5, 0, 0, 0).Background(ui.Hex("#0f172a").Alpha(0.68))
			}
			if spec.Playing {
				a.playingBadge(c)
			}
			if spec.Cover2 != nil {
				spec.Cover2(c)
			}
		})

		// 页脚（原版 [data-slot=footer]）：px-2 py-1，白 .22 洗色 + 顶部一线
		// 分隔（白 .18），悬停时洗色涨到 .36。标题与 meta 行都按原版的行盒
		// （text-small 14/20、text-xs 12/16）压回原版高度。
		ui.Column(c).FillWidth().Grow(1).Padding(4, 8).
			Background(cardFooterBG(hovered)).
			InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.18)).
			Children(func() {
				th, tsh := textLineBox(14, textSmLH)
				// 原版标题：text-small（14）/ bold / 单行截断。
				// 有标题前胶囊时（原版 feedList 的 titlePrefix）把标题行换成
				// Row：胶囊在前（bg-red-400 / px-1 py-0.5 / rounded-lg /
				// text-white / mr-1），标题在后并占满剩余宽度。
				if spec.Badge != "" {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						// 原版胶囊是 `py-0.5`（上下 2px）+ 行盒；mygo 这里是 border-box
						// 且盒高锁在标题行盒 th 上（胶囊是 inline span，内边距不撑高
						// 行盒，见 cardfooter_test.go 的 TestCardTitleBadgeLineBox），
						// 所以只在盒内补 2px 上下内边距，盒高保持 th。
						ui.Text(c, spec.Badge).FontSize(14).FontWeight(700).
							TextColor(ui.Hex("#ffffff")).Height(th).Translate(0, tsh).
							Padding(cardTitleBadgePadY, 4).Radius(8).
							Background(ui.Hex("#f87171")).Margin(0, 4, 0, 0).Shrink(0)
						ui.Text(c, spec.Title).FontSize(14).FontWeight(700).
							TextColor(t.Ink).SingleLine().Height(th).Translate(0, tsh).
							Grow(1).MinWidth(0).Tooltip(spec.Title)
					})
				} else {
					ui.Text(c, spec.Title).FontSize(14).FontWeight(700).
						TextColor(t.Ink).SingleLine().Height(th).Translate(0, tsh).Tooltip(spec.Title)
				}
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
	// meta 行是 Tailwind 的 text-xs（12px / 16px 行高）：框架自然行盒 18，
	// 整行因此比原版高 2px（见 textXsLH / linebox.go）。
	mh, msh := textLineBox(12, textXsLH)
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
					icon := iconViews
					if f.DanmakuIcon {
						icon = iconDanmaku
					}
					ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.Center).Gap(2).Children(func() {
						// 原版 .card-meta-field svg { opacity: .72 }。
						ui.Icon(c, icon).Size(12, 12).TextColor(ink.Alpha(0.72))
						ui.Text(c, f.Text).FontSize(12).TextColor(ink).SingleLine().
							Height(mh).Translate(0, msh)
					})
				case fieldPubdate:
					text := pubdateFit(c, f.Text, avail, now)
					// 原版 CardMeta 给发布时间加了 title tooltip（轮 28）：
					// 日期若被截，悬浮仍可看完整值。
					ui.Text(c, text).FontSize(12).TextColor(ink).SingleLine().
						Height(mh).Translate(0, msh).
						FillWidth().TextAlign(align).Tooltip(f.Text)
				default:
					// 原版 CardMeta 给作者名加了 title tooltip（轮 22）：
					// 窄卡片截断时悬浮显示完整名字。
					if f.Kind == fieldAuthor {
						ui.Text(c, f.Text).FontSize(12).TextColor(ink).SingleLine().
							Height(mh).Translate(0, msh).
							FillWidth().TextAlign(align).Tooltip(f.Text)
					} else {
						ui.Text(c, f.Text).FontSize(12).TextColor(ink).SingleLine().
							Height(mh).Translate(0, msh).
							FillWidth().TextAlign(align)
					}
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
// 22 高、圆角 999、深玻璃底 rgba(15,23,42,.55) + 1px 白边（.28），左侧三根
// 均衡器小柱按 part-eq 关键帧上下伸缩（见 eqBars）。
func (a *App) playingBadge(c *ui.Context) {
	badge := ui.Row(c).Absolute().Left(6).Top(6).Height(22).Padding(0, 8, 0, 7).
		Radius(RadiusPill).Gap(5).AlignItems(ui.Center).
		Background(ui.Hex("#0f172a").Alpha(0.55)).
		Border(1, ui.Hex("#ffffff").Alpha(0.28))
	badge.Children(func() {
		// Loop 拿到循环进度并顺带请下一帧，三根柱的相位差在 eqBarScale 里。
		eqBars(c, badge.Loop("eq", eqPeriod, ui.Linear))
		ui.Text(c, "正在播放").FontSize(11).FontWeight(600).
			TextColor(ui.Hex("#e2e8f0"))
	})
}

// estimateCardWidth 估一张卡片的内容宽度：抽屉正文左右各 24，网格间距 8，
// 卡片自身内边距 6。只在首帧（Bounds 还没有值）用；列数跟着 sm 断点走，
// 窄窗两列时每张卡宽得多（meta 行的容器查询就靠这个宽度）。
func estimateCardWidth(c *ui.Context) float32 {
	winW, _ := c.Size()
	if winW <= 0 {
		winW = 800
	}
	cols := float32(gridColsFor(winW))
	return (winW-48-8*(cols-1))/cols - 12
}

// ---------------------------------------------------------------- 分集抽屉

// Browse 返回选集抽屉要展示的那条视频（「浏览中」）：优先 BrowseInfo，
// 没有（还没浏览过）就退回 Info（正在播放的那条）——「打开当前视频的选集」
// 是浏览的一种，此时两者本就同一条，回退保证抽屉永远有内容。
func (a *App) Browse() *Info {
	if a.BrowseInfo != nil {
		return a.BrowseInfo
	}
	return a.Info
}

// BrowseTo 记下「浏览中」的视频并打开选集抽屉 —— 原版 handleUrlJump 的
// setVideoInfo(info) + setShowPageList(true)。
//
// 注意它**不碰 a.Info**：主区显示跟的是正在播放的那条（原版 displayVideoInfo），
// 浏览另一条视频不该把主区的标题 / 封面 / 简介换掉。
func (a *App) BrowseTo(info *Info) {
	a.BrowseInfo = info
	a.Drawer = DrawerParts
}

// partsAddAllTip 是「全部添加」键的工具提示 —— 原版 pageList.tsx 里这个键的
// `title="将全部选集添加到播放列表"`。此前原生版只有可见文字「全部添加」，
// 悬停看不到「加到哪儿」的说明。
const partsAddAllTip = "将全部选集添加到播放列表"

// partsAddAllInk 是「全部添加」键的图标 / 文字色。原版 pageList.tsx 里这个键的
// 图标是 `<AddOne fill="#666" size={16}/>`，后面的 span 继承同一文字色。此前原生
// 版用的是抽屉表头通用的 #475569（偏蓝的灰），色相不对；这里对齐 #666666。
// 声明成 var 便于测试逐值钉住（同 partSearchPlaceholderInk）。
var partsAddAllInk = ui.Hex("#666666")

// partsHeader 是选集抽屉的表头（原版 pageList.tsx）：「选集(N)」+ 定位当前 +
// 「全部添加」到播放列表。
func (a *App) partsHeader(c *ui.Context) {
	n := 0
	if bi := a.Browse(); bi != nil {
		n = len(bi.Parts)
	}
	// 原版 pageList 表头：「选集(N)」是 text-sm（14/600，见 header 全局规则）。
	ui.Text(c, fmt.Sprintf("选集(%d)", n)).FontSize(14).FontWeight(600).SingleLine().
		TextColor(ui.Hex("#334155")).Shrink(0)

	if a.Track != nil {
		a.headerButton(c, "parts-locate", "定位到当前播放的位置", iconLocate, 18, ui.Hex("#888888"), func() {
			a.locateNow = true
		})
	}
	// 选集搜索（原版 header .part-search）：flex 0 1 180px（不撑满）、
	// 高 32、圆角 8、内边距 0 10、gap 6，内凹底（rgba(15,23,42,.035)）。
	// 悬停 / 聚焦三态见 partSearchPaintFor（原版 :hover / :focus-within）；
	// ::before 的 --glass-sheen-static 与 ::after 的 --liquid-rim 见 partSearchGlass。
	box := ui.Row(c).Width(180).MaxWidth(180).Shrink(1).Height(32).Radius(Radius).
		Gap(6).Padding(0, 10).AlignItems(ui.Center)
	paint := partSearchPaintFor(box.Hovered(), box.FocusWithin())
	// 原版 `.part-search` 的 `transition: border-color 0.2s ease, box-shadow 0.2s
	// ease` 与 `::before` 的 `background-color 0.2s ease`：描边 / 填充 / 内影 /
	// 抬起全部走 200ms 曲线过渡（见 partSearchShadow 的说明）。
	box.Background(partSearchFade(box, "ps-bg", paint.BG)).
		Border(1, partSearchFade(box, "ps-border", paint.Border))
	// ::before 的两条内影（内凹腔：顶暗底亮）。三态的几何（0 1 2 / 0 -.5 0）
	// 完全一致，只有色值在变，所以直接交给 partSearchFade 做 0.2s 颜色过渡
	// （原版 ::before 的 `transition: box-shadow 0.2s ease`）。
	box.InsetShadow(0, 1, 2, 0, partSearchFade(box, "ps-inset-top", paint.InsetTop))
	box.InsetShadow(0, -0.5, 0, 0, partSearchFade(box, "ps-inset-bot", paint.InsetBottom))
	// 悬停 / 聚焦时整框抬起（原版 box-shadow: var(--liquid-shadow)）。常态
	// box-shadow 为 none —— CSS 把 none 当成一组「0 长度 + 透明」的影子参与
	// 插值，所以几何也得从零长出来，交给 partSearchShadow 一起动几何与色值。
	if paint.Lift {
		partSearchShadowInto(box, "ps-lift-lg", shadowSpec{0, 8, 24, 0, shadowInk.Alpha(0.07)})
		partSearchShadowInto(box, "ps-lift-sm", shadowSpec{0, 1, 4, 0, shadowInk.Alpha(0.03)})
	} else {
		partSearchShadowInto(box, "ps-lift-lg", shadowSpec{})
		partSearchShadowInto(box, "ps-lift-sm", shadowSpec{})
	}
	// 聚焦光环 0 0 0 2px rgba(14,165,233,.14)：同样从零长出来。
	ring := shadowSpec{}
	if paint.Ring.A > 0 {
		ring = shadowSpec{0, 0, 0, 2, paint.Ring}
	}
	partSearchShadowInto(box, "ps-ring", ring)
	// ::after 的聚光描边做 0.2s 不透明度过渡（常态 .5、聚焦 1），再叠上
	// ::before 的光泽。
	box = partSearchGlass(box, partSearchRimOpacity(box, paint.Rim))
	box.Children(func() {
		icon := ui.Icon(c, iconMagnifier).Size(15, 15)
		icon.TextColor(partSearchIconFade(icon, "ps-icon", paint.Icon))
		// field 把输入框与自绘占位文本收在一起，占位文本 Attach 到它的左缘。
		field := ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center)
		field.Children(func() {
			// 原版 input.part-search-input：13px / 500 / #0f172a，且 `outline: none`——
			// 聚焦时只由外层 .part-search:focus-within 给蓝色描边与光环，输入框自己
			// 不再画框架的聚焦环（否则会多出一圈内环）。占位文本见
			// partSearchPlaceholder：框架占位色固定走主题 TextMuted（#94a3b8），
			// 与原版 #475569 / 400 不符，故自己叠一层。
			in := ui.TextInputBase(c, &a.PartSearch).Grow(1).FontSize(13).FontWeight(500).
				FocusRing(false).
				TextColor(ui.Hex("#0f172a")).Label("搜索选集").
				Description("搜索选集，回车跳转")
			if in.Submitted() {
				a.locatePart(a.PartSearch)
			}
			if a.PartSearch == "" {
				a.partSearchPlaceholder(c)
			}
		})
	})
	if n > 0 {
		b := ui.ButtonBase(c.Key("parts-add-all")).Height(32).Padding(0, 12).
			Radius(Radius).Center().Label("全部添加").Tooltip(partsAddAllTip)
		drawerHeaderSurface(b, b.Hovered())
		drawerHeaderPress(b)
		b.Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconAdd).Size(16, 16).TextColor(partsAddAllInk)
				ui.Text(c, "全部添加").FontSize(12).TextColor(partsAddAllInk)
			})
		})
		if b.Clicked() && a.Act.AddAllToPlaylist != nil {
			a.Act.AddAllToPlaylist()
		}
	}
}

// 原版 `input.part-search-input::placeholder`：color #475569、font-weight 400
// （输入框正文是 500）。框架的占位符固定用主题 TextMuted（#94a3b8）并继承元素
// 字重，无法按元素覆写，所以这里不交框架画，自己叠一层同字号文本。声明成变量
// 是为了让测试能替换它做反证（同 partSearchPaintFor）。
var (
	partSearchPlaceholderInk    = ui.Hex("#475569")
	partSearchPlaceholderWeight = 400
)

// partSearchPlaceholder 在输入框左缘叠一层原版配色的占位文本：Attach 到输入框
// 所在的 field（不参与布局、不撑高容器），PassThrough 让点击落到输入框上。
func (a *App) partSearchPlaceholder(c *ui.Context) {
	ui.Text(c, "搜索选集，回车跳转").FontSize(13).FontWeight(partSearchPlaceholderWeight).
		TextColor(partSearchPlaceholderInk).SingleLine().PassThrough().
		Attach(ui.AnchorLeft, ui.AnchorLeft)
}

// partSearchPaint 是选集搜索框在某一状态下解析好的样式（原版 header
// `.part-search` 的四条规则）。Ring 的零值（A == 0）表示不画外圈光环。
type partSearchPaint struct {
	Border ui.Color // 1px 描边
	BG     ui.Color // ::before 的填充色（常态内凹底 / 悬停白底 / 聚焦薄白）
	Ring   ui.Color // 聚焦时的 2px 外圈光环
	Icon   ui.Color // 左侧放大镜
	// Lift 为真时叠一层 --liquid-shadow（原版悬停 / 聚焦都把整个框抬起来）。
	Lift bool
	// Rim 是 ::after 聚光描边的整圈不透明度（原版常态 .5、聚焦 1）。
	Rim float32
	// InsetTop / InsetBottom 是 ::before 的两条内影（顶暗底亮 = 内凹腔）。
	InsetTop, InsetBottom ui.Color
}

// partSearchPaintFor 解析选集搜索框的三态，逐值对应原版 globals.css：
//
//	常态          边 rgba(71,85,105,.5)   底 rgba(15,23,42,.035)   图标 #475569
//	              内影 dark .08 / white .35，::after opacity .5
//	:hover        边 rgba(71,85,105,.65)  底 --liquid-bg-hover(白 .30) 图标 #475569
//	              叠 --liquid-shadow（抬起）
//	:focus-within 边 rgba(14,165,233,.4)  底 rgba(255,255,255,.05) 图标 #0284c7
//	              内影 dark .04 / white .40，::after opacity 1，叠 --liquid-shadow
//	              外加 0 0 0 2px rgba(14,165,233,.14) 的蓝色光环
//
// 聚焦优先于悬停（原版 focus-within 的两条规则都排在 hover 之后，且同特异度、
// 源序在后 ⇒ 覆盖 hover 的白底）。此前原生版只画了常态度，悬停/聚焦都没有
// 反馈；且悬停底误当「不变」，漏了原版 `:hover::before { background-color:
// var(--liquid-bg-hover) }` 这一档（与 .liquid-glass / .home-searchbar / 表头
// tab 的悬停填充同一档）。
//
// 声明成变量是为了让测试能替换它做反证（同 navHoverHighlight）。
var partSearchPaintFor = func(hovered, focused bool) partSearchPaint {
	p := partSearchPaint{
		Border:      ui.Hex("#475569").Alpha(0.50),
		BG:          ui.Hex("#0f172a").Alpha(0.035),
		Icon:        ui.Hex("#475569"),
		Rim:         0.5,
		InsetTop:    ui.Hex("#0f172a").Alpha(0.08),
		InsetBottom: ui.Hex("#ffffff").Alpha(0.35),
	}
	if hovered {
		p.Border = ui.Hex("#475569").Alpha(0.65)
		// 原版 `.part-search:hover::before { background-color: var(--liquid-bg-hover) }`
		// —— 悬停把内凹底换成 --liquid-bg-hover（白 .30）。
		p.BG = ui.Hex("#ffffff").Alpha(0.30)
		p.Lift = true
	}
	if focused {
		p.Border = ui.Hex("#0ea5e9").Alpha(0.40)
		p.BG = ui.Hex("#ffffff").Alpha(0.05)
		p.Ring = ui.Hex("#0ea5e9").Alpha(0.14)
		p.Icon = ui.Hex("#0284c7")
		p.Rim = 1
		p.InsetTop = ui.Hex("#0f172a").Alpha(0.04)
		p.InsetBottom = ui.Hex("#ffffff").Alpha(0.40)
		p.Lift = true
	}
	return p
}

// partSearchAnim 是选集搜索框状态切换的过渡：原版 `.part-search` 的
// `transition: border-color 0.2s ease, box-shadow 0.2s ease` 与 `::before` 的
// `background-color 0.2s ease`（CSS ease = cubic-bezier(0.25,0.1,0.25,1)）。
var partSearchAnim = ui.Animation{
	Duration: 200 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// partSearchFade 把框体的一处色值交给动画层做过渡。拆成 var 便于测试换桩 /
// 冻结：AnimateColor 在目标变化的那一帧返回**上一帧的颜色**（曲线 t=0），
// 直接铺目标色会瞬时跳变。key 在同一元素内须唯一。
var partSearchFade = func(b ui.Element, key string, target ui.Color) ui.Color {
	return b.AnimateColor(key, target, partSearchAnim)
}

// partSearchShadow 把一个 box-shadow 的四项几何（x/y/blur/spread）与颜色一起
// 交给 0.2s 过渡，返回本帧的解析值。此前认为「mygo 的 Shadow 没有逐通道动画
// 通道、只能当帧切」——其实 AnimateTo / AnimateColor 就是逐通道通道：原版
// `.part-search { transition: border-color 0.2s ease, box-shadow 0.2s ease }`，
// CSS 把 `box-shadow: none` 当成一组「0 长度 + 透明」的影子参与插值，所以常态 →
// 悬停 / 聚焦时几何与颜色都从零起步，而不是当帧抬起。声明成 var 便于测试冻结
// 动画相位（同 partSearchFade）。
var partSearchShadow = func(b ui.Element, key string, s shadowSpec) shadowSpec {
	return shadowSpec{
		X:      b.AnimateTo(key+"-x", s.X, partSearchAnim),
		Y:      b.AnimateTo(key+"-y", s.Y, partSearchAnim),
		Blur:   b.AnimateTo(key+"-blur", s.Blur, partSearchAnim),
		Spread: b.AnimateTo(key+"-spread", s.Spread, partSearchAnim),
		C:      b.AnimateColor(key+"-ink", s.C, partSearchAnim),
	}
}

// partSearchShadowInto 把一条过渡中的 box-shadow 画到框体上。
func partSearchShadowInto(b ui.Element, key string, s shadowSpec) {
	r := partSearchShadow(b, key, s)
	b.Shadow(r.X, r.Y, r.Blur, r.Spread, r.C)
}

// partSearchRimOpacity 把 ::after 聚光描边的整圈不透明度做 0.2s 过渡（原版
// `::after { transition: opacity 0.2s ease }`）。
var partSearchRimOpacity = func(b ui.Element, target float32) float32 {
	return b.AnimateTo("ps-rim", target, partSearchAnim)
}

// partSearchIconAnim 是左侧放大镜换色的过渡：原版 `.part-search-icon` 单写一条
// `transition: color 0.16s ease`，比框体的 0.2s 略快，别并到 partSearchAnim。
var partSearchIconAnim = ui.Animation{
	Duration: 160 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// partSearchIconFade 把放大镜的墨色做 160ms 过渡（原版 `.part-search-icon {
// transition: color 0.16s ease }`）。拆成 var 便于测试冻结动画相位。
var partSearchIconFade = func(b ui.Element, key string, target ui.Color) ui.Color {
	return b.AnimateColor(key, target, partSearchIconAnim)
}

// partSearchGlass 画 `.part-search::before` 的 --glass-sheen-static 光泽与
// `::after` 的 1px 聚光描边（--liquid-rim）。mygo 没有径向渐变接口、也画不了
// conic-gradient，光泽复用播放栏那套烘焙位图（sheenGlassBaked 就是
// --glass-sheen-static），描边复用播放栏的四边线性近似（paintPlayerRim）。
// sheen 走 paintBakedSheen：搜索框是 200×36 的宽盒，按它的宽高比烘一张，
// 线性层的角度才不会被正方形位图的非等比拉伸带偏。
// 声明成 var 便于测试换桩做反证。
var partSearchGlass = func(b ui.Element, rimOpacity float32) ui.Element {
	return b.Draw(func(p *ui.Painter, r ui.Rect) {
		paintBakedSheen(p, r, Radius, sheenGlassBaked)
		if rimOpacity > 0 {
			paintPlayerRim(p, r, Radius, rimOpacity)
		}
	})
}

// partsBody 是选集抽屉正文：三列卡片网格（虚拟化 ui.List）。
func (a *App) partsBody(c *ui.Context) {
	bi := a.Browse()
	if bi == nil || len(bi.Parts) == 0 {
		a.scrollBody(c, a.list(), func() { a.emptyState(c, "没有分集信息") })
		return
	}
	a.partsGrid(c)
}

// partsGrid 用与 cardGrid 相同的虚拟化 ui.List 画选集三列网格。
//
// 之前 partsBody 是 ui.Column 逐张构建：多 P 视频每帧都会把**全部**分集卡片
// 连同各自的材质重建一遍，而且卡片封面在构建期直接 Bitmap() 抓取（每帧重发
// 一遍屏外请求）。改成 List 后只建可见行，封面也走「可见窗口预取」——与卡片
// 网格同一套（见 prefetchCardCovers / prefetchPartCovers）。
func (a *App) partsGrid(c *ui.Context) {
	parts := a.Browse().Parts
	l := a.list()
	st := &l.Grid
	cols := gridColsFor(windowWidth(c))
	// 行键取首张分集的 cid：换视频 / 刷新分集时不沿用旧锚点。
	st.Key = func(r int) any {
		if i := r * cols; i < len(parts) {
			return parts[i].Cid
		}
		return r
	}
	// 定位（「定位到当前」 / 选集搜索）：屏外的行不构建，不能再靠卡片自己的
	// ScrollIntoView，得把目标**行**交给 ListState，让它布局时滚过去。
	if a.locateNow {
		if row := partsLocateRow(parts, a.Track, a.locatePartIndex, cols); row >= 0 {
			st.ScrollTo(row, ui.Start)
		}
	}
	rows := (len(parts) + cols - 1) / cols
	a.prefetchPartCovers(st, parts, cols)
	a.holdListScroll(c, &a.partsHold, st, &l.GridScroll, true)
	ui.List(c, st, rows, func(r int) {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
			start := r * cols
			end := min(start+cols, len(parts))
			for j := start; j < end; j++ {
				a.partCard(c, j, parts[j])
			}
			for j := end; j < start+cols; j++ {
				ui.Box(c).Grow(1).Basis(0)
			}
		})
	}).FillWidth().Grow(1).Gap(8).Padding(8, 24).TrackScroll(&l.GridScroll)

	// 定位标志只在这一次构建里生效。
	a.locateNow = false
	a.locatePartIndex = -1
}

// partsLocateRow 返回选集定位要滚到的行号：优先选集搜索匹配的分集，
// 否则回退到正在播放的那一集；都没有返回 -1。cols 是当前网格列数
// （见 gridColsFor），窄窗两列时同一分集落在不同的行上。
func partsLocateRow(parts []Part, track *Track, locateIndex, cols int) int {
	if locateIndex >= 0 && locateIndex < len(parts) {
		return locateIndex / cols
	}
	if track != nil {
		for i := range parts {
			if parts[i].Cid == track.Cid {
				return i / cols
			}
		}
	}
	return -1
}

// partCover 返回一张选集卡片的封面地址：分集有自己的首帧图就用它，
// 否则回退到整支视频的封面。
func (a *App) partCover(p Part) string {
	if p.FirstFrame != "" {
		return p.FirstFrame
	}
	if bi := a.Browse(); bi != nil {
		return bi.Pic
	}
	return ""
}

// partCard 是一张选集卡片。
func (a *App) partCard(c *ui.Context, index int, p Part) {
	active := a.Track != nil && a.Track.Cid == p.Cid
	inList := a.playlistHas(p.Cid)
	cover := a.partCover(p)
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
		// 封面走「可见窗口预取」（partsGrid 的 prefetchPartCovers），构建只读缓存。
		Lazy: true,
		Fields: []cardField{
			{Kind: fieldViews, Text: statText(a.Browse(), func(i *Info) int64 { return i.View })},
			{Kind: fieldDuration, Text: dur},
		},
		// 定位由 partsGrid 交给 ListState 滚动（屏外的卡片不构建）。
		Cover2: func(c *ui.Context) {
			// 右上角的添加键（原版 pageList 的 className：absolute top-1
			// right-1 min-w-6 w-6 h-6 rounded-full bg-black/30 border-0）。
			label := "添加到播放列表"
			if inList {
				label = "已在播放列表中"
			}
			add := ui.ButtonBase(c.Key(fmt.Sprintf("part-add-%d", p.Cid))).
				Size(partAddSize, partAddSize).Radius(RadiusPill).
				Center().Label(label).Tooltip(label)
			add.Absolute().Right(partAddInset).Top(partAddInset)
			// 压在封面上：纯黑半透明填充、无边框（bg-black/30 + border-0）。
			add.Background(partAddFill(add.Hovered()))
			add.Children(func() {
				ui.Icon(c, pick(inList, iconCheck, iconAdd)).
					Size(partAddIconSize, partAddIconSize).
					TextColor(partAddInk(inList))
			})
			if add.Clicked() && a.Act.AddToPlaylist != nil {
				a.Act.AddToPlaylist(p)
			}
		},
		OnClick: func() {
			if a.Act.Play != nil {
				a.Act.Play(index)
			}
			// 原版 PageList.handleVideoSelect 在 onVideoSelect 后调
			// handleOpenChange(false) 关面板；视图层只发 Act.Play，关不关由
			// 控制器 playIndex 决定（自动续播/播完切集仍保留面板，见
			// keepDrawerOnPlay），所以这里不能硬写 a.Drawer = ""。
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
		// 原版 danmakuList.tsx 的 handleViewModeChange：切 tab 时
		// setIsAutoScroll(true) + lastScrollIndexRef=-1，即自动跟随重置为开（与
		// 关抽屉时 handleOpenChange(false) 的行为一致，见 drawer.go 的关抽屉分支）。
		a.DanmakuAutoScroll = true
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
		// .danmaku-follow-btn.is-active：蓝边 + 145° 淡蓝渐变底 + 蓝字 +
		// inset 0 1px 0 白 .72，外围还有一圈 `0 0 0 2px rgba(14,165,233,.14)`
		// 的聚光蓝环。原版是 linear-gradient(145deg,
		// rgba(14,165,233,.22), rgba(255,255,255,.5))，用 Gradient 直接对齐。
		// 此前只画了内高光与外投影记成了 `Shadow(0,2,...)`，2px 无偏移的
		// 光环整条漏掉（那是 is-active 与常态最显眼的区别）。
		b.Gradient(ui.Hex("#0ea5e9").Alpha(0.22), ui.Hex("#ffffff").Alpha(0.5), 145).
			Border(1, ui.Hex("#0284c7").Alpha(0.45)).
			InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.72)).
			Shadow(0, 0, 0, 2, ui.Hex("#0ea5e9").Alpha(0.14))
	} else {
		// .danmaku-follow-btn 常态：白 .36 底 + inset 0 1px 0 白 .5，无外投影。
		// 此前用 buttonSurface（常态 fill 白 .22 + 外投影），两处都与原版不符。
		b.Background(ui.Hex("#ffffff").Alpha(0.36)).
			Border(1, ui.Hex("#64748b").Alpha(0.22)).
			InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.5))
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
//
// 原版逻辑（isDarkMode = false）：
//
//	brightness > 200               → #1a1a1a
//	160 < brightness < 200 且 < 170 → #333333
//	其他                           → 原色 hex
//	color = 0                      → #000000（原版 (0).toString(16).padStart(6,"0")）
// danmakuInk 把弹幕颜色转成显示色：太亮/太暗时换对比色（原版 getColorStyle）。
// dark 为真表示系统深色模式（原版 document.documentElement.classList.contains("dark")）：
// 亮度 >200 的弹幕在深色底上直接用白字，浅色底上换深灰 #1a1a1a；
// 亮度 160–200 区间也按深浅分档。
func danmakuInk(color int, dark bool) ui.Color {
	if color <= 0 {
		return ui.Hex("#000000")
	}
	r := (color >> 16) & 0xff
	g := (color >> 8) & 0xff
	b := color & 0xff
	brightness := (float64(r)*299 + float64(g)*587 + float64(b)*114) / 1000
	if brightness > 200 {
		if dark {
			return ui.Hex("#ffffff")
		}
		return ui.Hex("#1a1a1a")
	}
	if brightness > 160 && brightness < 200 {
		if dark && brightness > 180 {
			return ui.Hex("#e0e0e0")
		}
		if !dark && brightness < 170 {
			return ui.Hex("#333333")
		}
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

// danmakuCurrentIndex 返回当前播放位置最近的弹幕组下标（原版 danmakuList 的
// currentIndex）。它与「自动跟随」开关**无关**：原版开关只决定要不要把当前组
// 滚进视野（useEffect 里的 isAutoScroll），is-current 高亮只看 currentTime。
// 此前两者被耦合成 `if a.DanmakuAutoScroll && a.Pos > 0`，于是关掉跟随、当前
// 组的时间胶囊就不再变蓝；也丢了原版 currentTime 初始 0 时高亮首组的行为
// （原版 currentIndex 在 currentTime >= 0 时就取最近组，0 对应首组）。
func (a *App) danmakuCurrentIndex(groups []danmakuGroup) int {
	if len(groups) == 0 || a.Pos < 0 {
		return -1
	}
	return nearestGroup(groups, a.Pos)
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

	// 当前组高亮：只跟播放位置走（见 danmakuCurrentIndex），与「自动跟随」
	// 开关无关；滚动另受开关约束（见下方 ScrollIntoView）。
	cur := a.danmakuCurrentIndex(groups)

	// 弹幕数据换了来源（换集 / 浏览到另一条视频）就重置虚拟列表的滚动位置与
	// 行高，否则新一集会沿用上一条的锚点。来源以 DanmakuCid（已加载数据属于
	// 谁）为准，不再看正在播放的 Track —— 抽屉跟的是浏览中那条（原版
	// videoInfo），两者可能不是同一条。
	if a.DanmakuCid != 0 && a.DanmakuCid != a.danmakuCid {
		a.danmakuList = ui.ListState{}
		a.danmakuCid = a.DanmakuCid
		a.danmakuScrollIdx = -1
	}

	// 虚拟化：这条列表在 bba0c7c 里从 ui.List 改成了 ui.Column，之后每帧都重建
	// **所有**分组行（分组上千时是主要开销之一）。恢复 ui.List 后只建可见的
	// 十几行。
	theme := a.Theme
	// 弹幕分组行与胶囊的配色按平台选值（原版 body.platform-linux 那组覆写）：
	// 原生版主平台就是 Linux，无 backdrop-filter 时要用不透明兜底。
	linux := isLinux()
	// 弹幕分组行的列宽/间距随窗口宽度收窄（原版 .danmaku-time-group 的
	// @media (max-width: 420px)）：这一帧算一次，传给每行复用。
	row := danmakuRowLayoutFor(windowWidth(c))
	// 分组与消息列的 max-height 用 dvh（视口高度）：这一帧取一次。
	_, viewportH := c.Size()
	ui.List(c, &a.danmakuList, len(groups), func(i int) {
		a.danmakuGroupRow(c, groups[i], i == cur, theme, row, linux, viewportH)
	}).FillWidth().Grow(1).Padding(8, 10, 16, 10)

	// 自动跟随：把当前组滚到视口中央，且只在下标变化时滚，不干扰手滑。原版是
	// `danmakuVirtualizer.scrollToIndex(index, { align: "center" })`，不是
	// 「最少滚动让它露出来」——此前用 ui.ScrollIntoView（near 对齐）会把当前组
	// 怼到视口下缘，与原版的位置稳定感不同。
	if a.DanmakuAutoScroll && cur >= 0 && a.danmakuScrollIdx != cur {
		a.danmakuList.ScrollTo(cur, ui.Center)
		a.danmakuScrollIdx = cur
	}
}

// danmakuGroupRow 画一个弹幕分组行（原版 .danmaku-time-group）：整行 grid
// （时间列 + 1fr 消息列）、底部 1px 分隔线；组之间没有间距，靠这条线分开。
// 列宽/间距/inline 内边距由 l 给出（宽窗 64/10/12，≤420 时 54/8/10）。
// viewportH 是窗口高度，用来算 .danmaku-time-group 的 80dvh 上限与
// .danmaku-message-list 的 calc(80dvh - 20px) 内滚动高度。
// 由 danmakuBody 的虚拟列表按需构建。
func (a *App) danmakuGroupRow(c *ui.Context, g danmakuGroup, isCur bool, t Theme, l danmakuRowLayout, linux bool, viewportH float32) {
	box := ui.Row(c).Key(fmt.Sprintf("dmg-%d", g.Second)).FillWidth().
		Margin(2, 0).
		Padding(11, l.PadInline, 13).Gap(l.Gap).AlignItems(ui.Start)
	// 原版 `.danmaku-time-group` 是 `max-height:80dvh; overflow:hidden`：
	// 消息列的内滚动高度是 80dvh-20，再加分组上下 padding 24，会比 80dvh
	// 多 4px，原版由 overflow:hidden 裁掉；这里同样封顶并裁剪。
	if h := danmakuGroupMaxHeight(viewportH); h > 0 {
		box.MaxHeight(h)
		box.Clip()
	}
	// 悬停/当前态只改底部分隔线的颜色（原版如此，不铺底色）；原版
	// `.danmaku-time-group` 是 `transition: border-color 150ms ease`。
	sep, hoverSep, curSep := danmakuSepColors(linux)
	sepTarget := sep
	switch {
	case isCur:
		sepTarget = curSep
	case box.Hovered():
		sepTarget = hoverSep
	}
	box.InsetShadow(0, -1, 0, 0, drawerFadeColor(c, box, "dmg-sep", sepTarget, danmakuBorderAnim))
	box.Children(func() {
		// 左列（宽窗 64 / ≤420 时 54）：时间标签 + 「N 条」，右缘一线分隔
		//（.danmaku-time-head）。右缘用 x=-1 的内影（正 x 画的是左缘）。
		// `.danmaku-time-head` 的 border-right 是写死的 rgba(100,116,139,.14)，
		// Linux 覆写只动分组自身的 border-color，不动这条，所以它不随平台变。
		ui.Column(c).Width(l.Col).Shrink(0).Gap(4).Justify(ui.Center).
			// AlignItems(Start)：原版 `.danmaku-time-label` 是 `width: fit-content`，
			// 列默认 cross 轴 stretch 会把标签背景铺满 64px 左列。
			AlignItems(ui.Start).
			InsetShadow(-1, 0, 0, 0, ui.Hex("#64748b").Alpha(0.14)).Children(func() {
			lbl := numericText(c, fmtClock(float64(g.Second))).FontSize(13).
				FontWeight(700).Height(danmakuLabelHeight).Padding(0, 5).Radius(6)
			// 当前组：时间标签变实心蓝胶囊（原版 .is-current 的 label）。
			// 原版 `.danmaku-time-label` 的
			// `transition: background-color 140ms ease, color 140ms ease`——
			// 底色 / 墨色都走过渡（阴影无动画通道，当帧切换）。
			labelBG := ui.Transparent
			labelInk := ui.Hex("#334155")
			if isCur {
				labelBG = ui.Hex("#0284c7")
				labelInk = ui.Hex("#ffffff")
			}
			lbl.Background(drawerFadeColor(c, lbl, "dmg-label-bg", labelBG, drawerFadeAnim)).
				TextColor(drawerFadeColor(c, lbl, "dmg-label-ink", labelInk, drawerFadeAnim))
			if isCur {
				lbl.InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.28)).
					Shadow(0, 2, 7, 0, ui.Hex("#0284c7").Alpha(0.22))
			}
			// .danmaku-time-total：11px #64748b（当前组转蓝加粗），
			// `line-height:1.2`（11×1.2=13.2）由显式盒高 + 字形上移复现，
			// 否则框架的自然行盒（约 17）会把左列撑高约 4px。
			tot := ui.Text(c, fmt.Sprintf("%d 条", g.TotalCount)).FontSize(11).
				TextColor(ui.Hex("#64748b")).
				Height(danmakuTotalHeight).Translate(0, danmakuTotalShift)
			if isCur {
				tot.TextColor(ui.Hex("#0369a1")).FontWeight(700)
			}
		})

		// 右列：消息横排换行的一串小胶囊（.danmaku-message-list，
		// align-items/align-content 都是 center）。原版给这列
		// `max-height: calc(80dvh - 20px); overflow-y: auto`：同一秒弹幕很多时
		// 在分组内滚动，而不是把整行无限拉长（此前原生版就是后者）。
		ui.Scroll(c).Grow(1).MinWidth(0).
			MaxHeight(danmakuMessageListMaxHeight(viewportH)).Children(func() {
			ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).AlignContent(ui.Center).Children(func() {
				for _, e := range g.Entries {
					paint := danmakuChipPaint(isCur, box.Hovered(), linux)
					chip := ui.Row(c).Key(fmt.Sprintf("dmg-%d-%s", g.Second, e.Text)).
						Height(27).MinWidth(32).Padding(3, 7, 3, 9).Gap(5).
						Radius(8).AlignItems(ui.Center)
					// 原版是白色半透明小胶囊（无 backdrop-filter）：填充而非真玻璃。
					// 原版 `.danmaku-message-row` 的
					// `transition: background-color/border-color 140ms ease`。
					chip.Background(drawerFadeColor(c, chip, "dmg-chip-bg", paint.BG, drawerFadeAnim)).
						Border(1, drawerFadeColor(c, chip, "dmg-chip-border", paint.Border, drawerFadeAnim)).
						InsetShadow(0, 1, 0, 0, paint.Inset).
						Shadow(0, 2, paint.ShadowBlur, 0, paint.Shadow)
					chip.Children(func() {
						// 原版 `.danmaku-message-text` 只有一条白色 text-shadow、**没有底色**。
						// 此前用「白 .68 Absolute 副本」模拟那条阴影（见 4408e18），在胶囊
						// 里显出一块方形的白底——用户报「弹幕文字默认白底，没有必要」。
						// 核对 main 原版：文字层无 background，白底只该来自胶囊本体
						// （.danmaku-message-row 的 white .48 / Linux .9）。本轮去掉
						// 白层与透明占位那套 Absolute 手法，文字回到 flow：胶囊宽度由
						// 文字 + ×N 自然撑开，也不再需要手动让开 ×N 的右缘。
						ui.Text(c, e.Text).FontSize(13).FontWeight(650).
							FixedLineHeight(18).
							TextColor(danmakuInk(e.Color, t.Dark)).SingleLine()
						if e.Count > 1 {
							// .danmaku-repeat-count：小蓝胶囊 ×N。
							// 原版 height:18px; line-height:16px；框架自然行盒约 15px，
							// 显式 Height + Translate 压回原版高度。
							numericText(c, fmt.Sprintf("×%d", e.Count)).FontSize(10).
								FontWeight(700).TextColor(ui.Hex("#0369a1")).
								Height(danmakuRepeatCountHeight).Translate(0, danmakuRepeatCountShift).
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
	})
}

// danmakuSepColors 给出弹幕分组行分隔线（.danmaku-time-group 的 border-bottom）
// 在基础 / 悬停 / 当前组三态下的颜色。
//
// 原版对 Linux 有一组覆写（body.platform-linux .danmaku-time-group）：
// 基础 rgba(100,116,139,.18)、当前组 rgba(2,132,199,.34)。注意 Linux 下悬停
// **不改**分隔线：`.danmaku-time-group:hover` 的蓝线特异度是 (0,2,0)，被
// `body.platform-linux .danmaku-time-group` (0,2,1) 盖掉 —— 原版如此（悬停反馈
// 改由胶囊底色承担），这里如实照搬。
//
// 其它平台：基础 rgba(100,116,139,.14)、悬停 sky-500 rgba(14,165,233,.24)、
// 当前组 studio-blue rgba(2,132,199,.30)。
func danmakuSepColors(linux bool) (base, hover, cur ui.Color) {
	if linux {
		base = ui.Hex("#64748b").Alpha(0.18)
		return base, base, ui.Hex("#0284c7").Alpha(0.34)
	}
	return ui.Hex("#64748b").Alpha(0.14),
		ui.Hex("#0ea5e9").Alpha(0.24),
		ui.Hex("#0284c7").Alpha(0.30)
}

// chipPaint 是一种弹幕小胶囊（.danmaku-message-row）的底色 / 边 / 内高光 / 投影。
type chipPaint struct {
	BG, Border, Inset, Shadow ui.Color
	ShadowBlur                float32
}

// danmakuChipPaint 返回弹幕小胶囊在某状态下的配色，逐条对应原版
// globals.css 里 .danmaku-message-row 的规则：
//
//	基础        底 rgba(255,255,255,.48) 边 .52 内高光 .72 投影 15,23,42 .045
//	:hover      底 rgba(255,255,255,.58) 边 .68（比基础**更实**，不是更淡）
//	.is-current 底 rgba(240,249,255,.76) 边 rgba(14,165,233,.48) 投影 2,132,199 .1
//
// 之前悬停写成白 .22（比基础 .42 更淡），当前组又用了饱和的 rgba(14,165,233,.2)
// —— 两个状态都与原版相反（悬停该变实、当前组该是极淡的蓝白）。
//
// linux 为真时叠上原版 `body.platform-linux .danmaku-message-row` 那条覆写：
// 底换近乎不透明的白 .9、边换 slate-400 .28、box-shadow none（Linux 没有
// backdrop-filter，不透明底才读得清，原版自己就这么兜底）。悬停态走
// `.danmaku-time-group:hover .danmaku-message-row`，特异度 (0,3,0) 高于 Linux
// 规则的 (0,2,1)，所以底/边盖回白 .58 / 白 .68，但 box-shadow 仍归 Linux 的
// none；is-current 也是 (0,3,0)，Linux 下与其它平台完全一样。
func danmakuChipPaint(isCur, hovered, linux bool) chipPaint {
	const white = "#ffffff"
	switch {
	case isCur:
		return chipPaint{
			BG:         ui.Hex("#f0f9ff").Alpha(0.76),
			Border:     ui.Hex("#0ea5e9").Alpha(0.48),
			Inset:      ui.Hex(white).Alpha(0.82),
			Shadow:     ui.Hex("#0284c7").Alpha(0.10),
			ShadowBlur: 7,
		}
	case hovered:
		p := chipPaint{
			BG:         ui.Hex(white).Alpha(0.58),
			Border:     ui.Hex(white).Alpha(0.68),
			Inset:      ui.Hex(white).Alpha(0.72),
			Shadow:     ui.Hex("#0f172a").Alpha(0.045),
			ShadowBlur: 6,
		}
		if linux {
			// box-shadow: none —— 内高光与投影都去掉。
			p.Inset, p.Shadow, p.ShadowBlur = ui.Transparent, ui.Transparent, 0
		}
		return p
	default:
		p := chipPaint{
			BG:         ui.Hex(white).Alpha(0.48),
			Border:     ui.Hex(white).Alpha(0.52),
			Inset:      ui.Hex(white).Alpha(0.72),
			Shadow:     ui.Hex("#0f172a").Alpha(0.045),
			ShadowBlur: 6,
		}
		if linux {
			// 底白 .9、边 slate-400 .28、box-shadow none。
			p.BG = ui.Hex(white).Alpha(0.90)
			p.Border = ui.Hex("#94a3b8").Alpha(0.28)
			p.Inset, p.Shadow, p.ShadowBlur = ui.Transparent, ui.Transparent, 0
		}
		return p
	}
}

// 评论列表的配色。原版把 HeroUI 的 default 调色板整套覆写成 slate 系
// （globals.css 里 [data-slot="wrapper"] .text-default-*），取名与选择器对齐：
//
//	replyInkName  昵称            .text-default-700 → #0f172a（字重 600）
//	replyInkMeta  时间/点赞/更多   .text-xs.text-default-400 → #64748b
//	              （基础 .text-default-400 是 #475569，但 .text-xs 那条更特异）
//	replyInkBody  正文            .leading-relaxed → #142033
//	replyInkPrim  楼中楼昵称      .text-primary-500 → #0284c7（字重 600）
//	replyInkSub   楼中楼正文      .text-default-600 → #1e293b
//	replyInkMuted 空态/加载中     .text-center.text-default-400 → #94a3b8
const (
	replyInkName  = "#0f172a"
	replyInkMeta  = "#64748b"
	replyInkBody  = "#142033"
	replyInkPrim  = "#0284c7"
	replyInkSub   = "#1e293b"
	replyInkMuted = "#94a3b8"
)

// replyMetaText 是评论用户名下方的「Lv.N · 时间」（原版恒带 Lv，0 级也显示）。
func replyMetaText(level int, when string) string {
	return fmt.Sprintf("Lv.%d · %s", level, when)
}

// FormatCommentTime 把评论发送时间（unix 秒）格式化成原版 danmakuList 的
// formatDate：`new Date(timestamp * 1000).toLocaleString("zh-CN")`。
//
// zh-CN 的 toLocaleString 长这样：`2023/11/15 06:13:20`——年/月/日之间用 `/`，
// 月与日**不补零**，时/分/秒补到两位并带秒。此前用
// `time.Format("2006-01-02 15:04")`，分隔符、补零、缺秒三处都与原版不同。
func FormatCommentTime(sec int64) string {
	t := time.Unix(sec, 0)
	return fmt.Sprintf("%d/%d/%d %02d:%02d:%02d",
		t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// repliesBody 画评论列表（原版 danmakuList 的评论 tab）：玻璃卡片行 + 楼中楼
// 预览，滚到底自动加载下一页（原版 handleScroll，不是「加载更多」按钮）。
//
// 用 mygo 的虚拟化 ui.List 当滚动容器（只建可见的十几行）。此前这里是
// ui.Scroll + ui.Column 逐行构建：抽屉里驻留到上限的 120 行每帧全建一遍
// （头像、玻璃卡片、楼中楼），滚动时是主要开销；与弹幕列表 / 卡片网格同一机制。
// 行键取评论身份（见 commentKey），滑动窗口从头部释放已读那批时锚点行跟着
// 挪下标，视口停在原评论上不跳（见 main.go 的 retainComments）。
func (a *App) repliesBody(c *ui.Context) {
	if len(a.Comments) == 0 {
		// 清空（刷新 / 换视频）时把虚拟列表也复位，下一批评论从顶部开始，
		// 不沿用上一批的行锚点。
		a.repliesList = ui.ListState{}
		l := a.ListFor(a.Drawer)
		a.holdScrollBody(c, l)
		// 原版 .danmaku-scroll-area 的 padding 是 8px 10px 16px 10px
		// （上下不对称，底部多留 16 给最后一行）。
		ui.Scroll(c).FillWidth().Grow(1).Padding(8, 10, 16, 10).TrackScroll(&l.Scroll).Children(func() {
			// 原版 renderReplies：列表为空时恒显示「暂无评论」，加载中也不换成
			// 「加载中…」——那条加载提示只在**已有评论**时追加在列表末尾
			// （见下方 n+extra 的那一行）。
			a.emptyNote(c, "暂无评论")
		})
		return
	}
	// 评论按时间倒序（原版 danmakuList 的 sortedReplies：b.ctime - a.ctime）。
	// 带缓存：排序带拷贝，滚动时每帧重排会白白分配 + 吃 CPU（见 sortedComments）。
	sorted := a.sortedComments()

	// 评论数据换了来源（换集 / 浏览到另一条视频）就重置虚拟列表的滚动位置与
	// 行锚点，否则新一集会沿用上一条的锚点。来源以 ReplyOid（已加载评论属于
	// 谁）为准，不再看正在播放的 Track。
	if a.ReplyOid != 0 && a.ReplyOid != a.repliesCid {
		a.repliesList = ui.ListState{}
		a.repliesCid = a.ReplyOid
	}
	st := &a.repliesList
	st.Key = func(r int) any {
		if r < len(sorted) {
			cm := sorted[r]
			return commentKey{user: cm.User, sendTime: cm.SendTime, content: cm.Content}
		}
		return "comments-loading"
	}
	// 原版没有「加载更多」按钮：滚到底自动翻页；加载中在末行提示「加载中...」。
	n := len(sorted)
	extra := 0
	if a.RepliesLoading {
		extra = 1
	}
	ui.List(c, st, n+extra, func(r int) {
		if r >= n {
			// 「加载中...」是 `.text-center.text-default-400.py-2.text-xs` → 12/16。
			loadingH, loadingShift := textLineBox(12, textXsLH)
			ui.Row(c).FillWidth().Padding(8, 0).Justify(ui.Center).Children(func() {
				ui.Text(c, "加载中...").FontSize(12).TextColor(ui.Hex(replyInkMuted)).
					Height(loadingH).Translate(0, loadingShift)
			})
			return
		}
		a.commentRow(c, sorted[r], r)
	}).FillWidth().Grow(1).Padding(8, 10, 16, 10)

	// 滚到底自动翻页（原版 handleScroll：距底 100px 内）。
	if first, last := st.Visible(); canLoadMoreComments(a.RepliesHasMore, a.RepliesLoading, first, last, n) &&
		a.Act.LoadComments != nil {
		a.Act.LoadComments(a.ReplyPage + 1)
	}
}

// commentKey 是一条评论在虚拟列表里的身份（用户 + 发送时间 + 正文）。滑动
// 窗口从头部释放已读那批后，同一条评论的下标变了但键不变，ui.List 据此把
// 锚点行移位（heights.shift），视口停在原评论上。
type commentKey struct {
	user     string
	sendTime int64
	content  string
}

// canLoadMoreComments 报告评论列表这一帧该不该自动翻页：还有更多、不在加载中，
// 且已经滚到底（首行不在顶部、末行已进视野）。抽成纯函数便于单测。
//
// `first > 0` 是这里的闸门：首屏都还没滚动（或首屏不满）时不请求，避免每帧
// 重发下一页。
func canLoadMoreComments(hasMore, loading bool, first, last, rows int) bool {
	if !hasMore || loading {
		return false
	}
	return first > 0 && last >= rows-1
}

// commentRow 画一行评论（原版 danmakuList 评论 tab 的卡片行）：32 圆头像 +
// 昵称 / 「Lv.N · 时间」、正文、点赞数、楼中楼预览。由 repliesBody 的虚拟列表
// 按需构建。
//
// 外层盒吃原版卡片的 `margin: 4px 6px`（用 Padding 而非 Margin）：ui.List 给
// 行根的宽度恒为内容宽（layoutBox(r.e, cw, …)），行根再 `FillWidth()` 就是
// cw，叠上 margin 会**溢出**右侧（原版块级 div 的 margin 会从宽度里扣）。
// 卡片因此会比原版宽 12px、右缘探进 6px（原版右缘离滚动区内容边 6px，
// 原生版探到 -6px）。外层盒吃内边距、卡片 FillWidth，宽度就与原版一致，
// 垂直间距不变（4 + 卡片 + 4）。
func (a *App) commentRow(c *ui.Context, cm Comment, i int) {
	ui.Column(c).Key(fmt.Sprintf("cm-%d", i)).FillWidth().Padding(4, 6).
		Children(func() { a.commentCard(c, cm, i) })
}

// commentCard 是评论行的玻璃卡片本体（原版 `.py-2.px-8.border-b` 被 globals.css
// 覆写后的样子）：圆角 10、白 32% 底 + 32% 白边 + 顶部内高光。
func (a *App) commentCard(c *ui.Context, cm Comment, i int) {
	row := ui.Column(c).FillWidth().Padding(8, 32).Gap(8).Radius(10)
	// 悬停换底 / 换边：原版 `.py-2.px-8.border-b` 的
	// `transition: background-color .14s ease, border-color .14s ease,
	// box-shadow .14s ease`——底与边走 140ms 过渡（阴影无逐通道
	// 动画，仍当帧切换）。
	targetBG := ui.Hex("#ffffff").Alpha(0.32)
	targetBorder := ui.Hex("#ffffff").Alpha(0.32)
	inset := ui.Hex("#ffffff").Alpha(0.42)
	hovered := row.Hovered()
	if hovered {
		// :hover 底更实、边转蓝，抬起一层淡投影。
		targetBG = ui.Hex("#ffffff").Alpha(0.58)
		targetBorder = ui.Hex("#0ea5e9").Alpha(0.22)
		inset = ui.Hex("#ffffff").Alpha(0.58)
	}
	row.Background(drawerFadeColor(c, row, "cm-row-bg", targetBG, drawerFadeAnim)).
		Border(1, drawerFadeColor(c, row, "cm-row-border", targetBorder, drawerFadeAnim)).
		InsetShadow(0, 1, 0, 0, inset)
	if hovered {
		row.Shadow(0, 5, 14, 0, ui.Hex("#0f172a").Alpha(0.06))
	}
	// 评论行内各段文本按原版的 Tailwind / HeroUI 字号类压回行盒：
	// 昵称 text-sm（14/20），「Lv.N · 时间」/ 点赞数 / 「查看更多回复...」/
	// 「加载中...」/ 楼中楼昵称都是 text-xs（12/16）。框架把行盒夹在
	// max(指定行高, ascent+descent) 之上（14px→22、12px→18），不压的话
	// 用户名两行高 40（原版 36）把整行从 36 撑到 40（头像只有 32）。
	// 见 linebox.go 的 textSmLH / textXsLH。
	nameH, nameShift := textLineBox(14, textSmLH)
	metaH, metaShift := textLineBox(12, textXsLH)
	row.Children(func() {
		// 用户行：32 圆头像 + 昵称 / 「Lv.N · 时间」。
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			// 原版 .bg-default-200 有两条覆写，后者生效：rgba(203,213,225,.6)。
			face := ui.Box(c).Size(32, 32).Shrink(0).Radius(RadiusPill).
				Background(ui.Hex("#cbd5e1").Alpha(0.6)).Clip()
			face.Children(func() {
				if bmp := a.Images.Bitmap(cm.Avatar, imagecache.WidthAvatar); bmp != nil {
					ui.Image(c, bmp).Fill().Fit(ui.Cover)
				} else if placeholderCover != nil {
					// 原版 danmakuList.tsx:302 的 getMemberInfo：无 face 时
					// face="/default-avatar.png"（该图 404 → RetryImg onError 退
					// /cover.png）。这里同退占位图，不再只铺一层底色。
					ui.Image(c, placeholderCover).Fill().Fit(ui.Cover)
				}
			})
			// 昵称 / 「Lv.N · 时间」两行行盒（见 commentRow 顶部）。
			ui.Column(c).Children(func() {
				ui.Text(c, cm.User).FontSize(14).FontWeight(600).
					TextColor(ui.Hex(replyInkName)).SingleLine().
					Height(nameH).Translate(0, nameShift)
				// text-xs text-default-400 → #64748b（见 replyInkMeta）。
				ui.Text(c, replyMetaText(cm.Level, cm.Time)).FontSize(12).
					TextColor(ui.Hex(replyInkMeta)).SingleLine().
					Height(metaH).Translate(0, metaShift)
			})
		})
		// 正文区左缩进 40（原版 pl-10，与头像同宽）。
		ui.Column(c).Padding(0, 0, 0, 40).Children(func() {
			// text-sm leading-relaxed → 14.5px / line-height 1.72 / #142033
			//（globals.css 的 `[data-slot="wrapper"] .leading-relaxed` 把
			// text-sm 的 14px/1.625 覆写成 14.5px/1.72，行高 24.94）。原版正文
			// <p> 没有行数上限（globals.css 只给标题/简介挂 -webkit-line-clamp），
			// 长评论整段铺开。行高必须显式给：框架默认取字体自然行盒（14.5px
			// 时约 22），比原版的 24.94 每行少约 3px，多行评论会明显更挤。
			ui.Text(c, cm.Content).FontSize(14.5).LineHeight(1.72).TextColor(ui.Hex(replyInkBody))
			if cm.Likes > 0 {
				// .mt-1.text-xs → 12/16。
				ui.Text(c, fmt.Sprintf("👍 %d", cm.Likes)).FontSize(12).
					Margin(4, 0, 0, 0).TextColor(ui.Hex(replyInkMeta)).
					Height(metaH).Translate(0, metaShift)
			}
			// 楼中楼：mt-2 pl-3 + 左侧 2px 分隔线（border-default-200 → 灰蓝 .25）。
			if len(cm.Replies) > 0 {
				ui.Column(c).Margin(8, 0, 0, 0).Padding(0, 0, 0, 12).
					Gap(8).InsetShadow(2, 0, 0, 0, ui.Hex("#64748b").Alpha(0.25)).Children(func() {
					for j, rp := range cm.Replies {
						ui.Row(c).Key(fmt.Sprintf("cm-%d-r%d", i, j)).
							FillWidth().Gap(replyFirstLinePad).AlignItems(ui.Start).Children(func() {
							// .text-primary-500 同时覆写成 600 字重。昵称有 Shrink(0)、
							// 不参与换行（自然宽即单行），可以安全地压行盒；正文可换行，
							// 行距由框架逐行决定、压不下去，改自绘逐行落点（见 replyBody）。
							ui.Text(c, rp.User+":").FontSize(12).
								FontWeight(600).TextColor(ui.Hex(replyInkPrim)).Shrink(0).
								Height(metaH).Translate(0, metaShift)
							a.replyBody(c, rp)
						})
					}
					if cm.RepliesMore || len(cm.Replies) >= 3 {
						// .text-xs.text-default-400，悬停转 .text-primary-500。
						more := ui.Text(c, "查看更多回复...").FontSize(12).
							TextColor(ui.Hex(replyInkMeta)).
							Height(metaH).Translate(0, metaShift)
						if more.Hovered() {
							more.TextColor(ui.Hex(replyInkPrim))
						}
					}
				})
			}
		})
	})
}

// replyFont 是楼中楼正文的字体：原版 `<span className="ml-1 …">` 继承外层
// `div.text-xs` → 12px、字重 400。
var replyFont = ui.Font{Size: 12}

// replyMaxLines 是楼中楼正文的行数上限。原版 `<span>` 没有任何行数截断
// （globals.css 只给标题/简介/迷你标题挂 -webkit-line-clamp），这里给一个足够
// 大的数，让 wrapTextLines 只负责折行、不做省略号截断（它要求 maxLines > 0）。
const replyMaxLines = 1 << 20

// replyLinePitch 是楼中楼正文的**像素**行距：原版 Tailwind `text-xs` 是
// 12px / 16px 行高，即每行 16px（textXsLH 是倍数 16/12，乘回字号才是像素）。
var replyLinePitch = textXsLH * replyFont.Size

// replyBodyChrome 是楼中楼正文宽度链上**除昵称以外**的固定内边距之和（DIP）。
// 抽屉面板铺满整窗，正文列宽可以从窗口宽度按这条链算出来：
//
//	抽屉面板 Border(1)          -2
//	List.Padding(0,10)        -20
//	评论行外层 Padding(0,6)    -12
//	评论卡 Border(1)           -2
//	评论卡 Padding(0,32)       -64
//	正文列 Padding(0,0,0,40)   -40
//	楼中楼列 Padding(0,0,0,12) -12
//	行 Gap(4)                   -4
//
// （评论行外层的横向内边距就是原版卡片的 `margin: 4px 6px`，见 commentRow。）
const replyBodyChrome = 2 + 20 + 12 + 2 + 64 + 40 + 12 + 4

// replyBodyWidthFor 是楼中楼正文的**首行**可用宽（DIP），也是原生版行里
// body 盒的实宽：窗口宽减去 replyBodyChrome，再减去昵称的实测宽。原版首行
// 接着昵称（两个 inline span），可用宽正是「容器宽 − 昵称宽 − 4」；原生版把
// body 盒（Grow(1)）放在昵称 + `Gap(4)` 之后，其左缘 = 昵称右缘 + 4，盒宽恰好
// 就是同一个值 —— 首行在盒内从盒左缘起、用盒整宽，逐值吻合。
//
// 框架的布局期拿不到「本帧可用宽」（Context 只暴露 MeasureText，见 home.go
// videoInfo 的同类说明），而多行文本的行距由框架逐行决定、盒高压不下去
// （linebox.go），只能自绘逐行落点（multilinetext.go）—— 自绘要在**布局期**
// 知道折行数，所以用这条与布局同源的算式代替实测宽。
// TestReplyBodyWidthMatchesLayout 钉住它与实际盒宽逐值相等。
func replyBodyWidthFor(winW float32, user string) float32 {
	return winW - replyBodyChrome - textWidth(user+":", ui.Font{Size: 12, Weight: 600})
}

// replyFirstLinePad 是「昵称:」与正文首行之间的间距（原版第二个 span 的
// `ml-1`，即 4px；原生版用行容器的 `Gap(4)` 实现）。
const replyFirstLinePad = 4

// replyRestWidthFor 是正文**续行**的可用宽：原版续行从容器左缘起、用整行
// （两个 inline span 的 inline 流，续行没有悬挂缩进）。它比首行宽多出
// 「昵称宽 + 4」那一截。
func replyRestWidthFor(winW float32, user string) float32 {
	return replyBodyWidthFor(winW, user) + textWidth(user+":", ui.Font{Size: 12, Weight: 600}) + replyFirstLinePad
}

// replyLines 折出楼中楼正文的每一行：**悬挂缩进**（首行被昵称挤窄、续行用整行），
// 复现原版 danmakuList.tsx:375-380 两个 inline span 的 inline 流。
func replyLines(winW float32, rp Comment) []string {
	return wrapHangingLines(rp.Content, replyFont,
		replyBodyWidthFor(winW, rp.User), replyRestWidthFor(winW, rp.User), replyMaxLines)
}

// replyBody 画楼中楼正文（原版 danmakuList.tsx:379 的第二个 span）：12px 的
// 多行文本，原版行高是 Tailwind `text-xs` 的 16px。框架的自然行盒是 18，
// `LineHeight` 只能把行距放大、压不下去（linebox.go），所以自绘逐行落点
// （行距 replyLinePitch = 16，断行仍交给框架）。盒高 = 折行数 × 16。
//
// 折行按**悬挂缩进**：原版正文与昵称同处一个段落（两个 inline span），首行
// 接着昵称、被昵称挤窄，续行从容器左缘起用整行；此前用 Row(Gap(4)) + Grow(1)
// 把正文与昵称分开，正文盒整列左缘对齐昵称右缘，**续行也带悬挂缩进**
// （残留差异）。这里让首行在盒内从盒左缘起（= 昵称右缘 + 4），续行向左回退到
// 容器左缘（盒左缘 − 昵称宽 − 4）、用整行宽。
func (a *App) replyBody(c *ui.Context, rp Comment) {
	winW := windowWidth(c)
	name := rp.User + ":"
	nameW := textWidth(name, ui.Font{Size: 12, Weight: 600})
	lines := replyLines(winW, rp)
	box := ui.Box(c).Grow(1).MinWidth(0).Height(float32(len(lines)) * replyLinePitch)
	if rp.Content != "" {
		// Label 让无头测试与读屏都能拿到正文（原版就是一段可见文本）。
		box.Label(rp.Content)
	}
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		paintHangingLines(p, r, replyLines(winW, rp),
			replyFont, replyLinePitch, nameW+replyFirstLinePad, ui.Hex(replyInkSub))
	})
}

// locatePart 把选集搜索框的关键词跳到下一个匹配的分集（连续回车继续，
// 到尾再循环；原版 handleSearchPart）。从当前游标往后找，找不到再从头找。
func (a *App) locatePart(keyword string) {
	kw := strings.TrimSpace(keyword)
	bi := a.Browse()
	if kw == "" || bi == nil || len(bi.Parts) == 0 {
		return
	}
	n := len(bi.Parts)
	start := a.PartSearchCursor
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		if strings.Contains(bi.Parts[idx].Part, kw) {
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

// collectScrollBtnPaintFor 解析收藏夹滚动键的常态 / 悬停 / 按下底色。原版
// globals.css 的 `.collect-scroll-btn`：常态 `rgba(15,23,42,.06)`；`:hover
// background: rgba(14,165,233,.14)`（浅蓝 #0ea5e9 14%）；`:active
// background: rgba(15,23,42,.12)`（深灰蓝）。此前 hover 误用了 :active 的深灰蓝，
// 底色方向偏了（原版悬停是浅蓝）。按下优先于悬停（:active 在 :hover 之后）。
func collectScrollBtnPaintFor(hovered, pressed bool) ui.Color {
	switch {
	case pressed:
		return ui.Hex("#0f172a").Alpha(0.12)
	case hovered:
		return ui.Hex("#0ea5e9").Alpha(0.14)
	}
	return ui.Hex("#0f172a").Alpha(0.06)
}

// collectScrollBtn 是收藏夹表头的左右滚动键（原版 collect-scroll-btn）：
// 把横向滚动位置拨 200px。
func (a *App) collectScrollBtn(c *ui.Context, key, label string, delta float32) {
	t := a.Theme
	b := ui.ButtonBase(c.Key(key)).Size(26, 28).Radius(7).Center().
		Label(label).Tooltip(label)
	b.Background(collectScrollBtnPaintFor(b.Hovered(), b.Pressed()))
	// 原版 `.collect-scroll-btn:focus-visible`：`outline: 0; color: #0284c7;
	// box-shadow: 0 0 0 2px rgba(2,132,199,.4)` —— 换色跟悬停同值（聚光蓝），
	// 环是 titleBarFocus 那一款无偏移的 2px 蓝环。
	b.Children(func() {
		ic := pick(delta < 0, iconLeft, iconRight)
		lit := b.Hovered() || b.FocusVisible()
		ui.Icon(c, ic).Size(14, 14).TextColor(pick(lit, t.Blue, ui.Hex("#475569")))
	})
	focusOutline(b, 7, focusGlow)
	if b.Clicked() {
		a.CollectScroll.X = max(0, a.CollectScroll.X+delta)
	}
}
