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

// playlistAccentRadius 是当前项左缘强调条的圆角（保留常量只为对照原版 `border-left`
// 的直边；当前项已不画强调条，见 playlistRowPaint）。
const playlistAccentRadius = 0

// playlistRowPadL 是记录行的左内边距：原版 p-2 = 8px，当前项也一样。
// 原版当前项的内容起点是 12px（border-left 4 + padding 8）；原生版去掉了那条
// 强调条之后，若继续把当前项的左内边距收成 0，缩略图就贴在行左缘（用户报
// 「当前播放项的左侧图片贴边了」）。不画强调条时，当前项按普通项的 8px 对齐。
const playlistRowPadL = 8

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

	// 两个来源列表都可以清空；合集列表此前没有清空入口。
	count := 0
	if a.PlaylistTab == ListUser {
		count = len(a.Playlist)
	} else if a.PlaylistTab == ListSeries {
		count = len(a.SeriesPlaylist)
	}
	if count > 0 {
		a.headerButton(c, "playlist-clear", "清空播放列表", iconDelete, 18, ui.Hex("#888888"), func() {
			if a.Act.ClearPlaylist != nil {
				a.Act.ClearPlaylist(a.PlaylistTab)
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
					Height(th).Padding(tsh, 0, 0, 0)
				ui.Text(c, "从选集列表中点击 + 添加选集到播放列表").FontSize(12).
					TextColor(t.Faint).Height(mh).Padding(msh, 0, 0, 0)
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
	a.prefetchPlaylistCovers(c, st, &l.rowsPrefetch, items)
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
//
// 可见行先 Touch 再 Bitmap：与卡片网格同一条可见优先语义（原版浏览器
// loading=lazy 的可见图先于屏外预取）。滚出预取窗口的行还要 Cancel ——
// 此前播放列表是唯一漏掉这一步的长列表：快速拖动时，途经的每一屏缩略图
// 都排进抓取队列，滚停后仍会一张张下载、解码，把 192w 的位图塞进 LRU，
// 既是瞬时内存尖峰，也会把当前可见的缩略图挤出缓存（原版 Tauri 的
// `<img loading="lazy">` 对滚出视口的图会取消加载；main 轮 31 也给播放列表
// 行补了位图卸载）。Cancel 只退还在排队的请求，在途的照常入库，滚回来
// 仍能直接命中。
func (a *App) prefetchPlaylistCovers(c *ui.Context, st *ui.ListState, gate *prefetchGate, items []PlayItem) {
	n := len(items)
	first, last := st.Visible()
	if last < first {
		first, last = 0, min(n, playlistInitialRows)
	}
	visFirst, visLast := first, last+1
	first = max(0, first-2)
	last = min(n, last+3)
	for i := visFirst; i < visLast && i < n; i++ {
		a.Images.Touch(playlistCoverURL(items[i]), imagecache.WidthPlaylist)
	}
	// 与卡片网格同一条滚动节流：窗口稳定后才真正发请求（见 prefetchGate）。
	if gate == nil || gate.stable(first, last) {
		for i := first; i < last; i++ {
			a.Images.Bitmap(playlistCoverURL(items[i]), imagecache.WidthPlaylist)
		}
	} else if c != nil {
		c.AnimationFrame() // 让稳定窗口的下一帧一定到来（同 prefetchWindowCovers）
	}
	// 窗口为空（n==0）时视为「没有请求过」，不取消 —— 与 cancelOffscreen
	// 同一条约定，避免空列表把上一帧的请求全退掉。
	if n == 0 || last <= first {
		return
	}
	for i := 0; i < n; i++ {
		if i < first || i >= last {
			a.Images.Cancel(playlistCoverURL(items[i]), imagecache.WidthPlaylist)
		}
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
// 说明（2026-10-26 修）：当前项:hover 的 CSS 是**三段**线性渐变
// （天蓝 .36 → .20@42% → 白 .62），mygo 此前只有两段（From/To），中间只能
// 按两端插值近似。渲染器按**预乘 alpha** 插值，两段近似在 42% 处混出的
// 颜色比原版亮一档但量级不大（浅色面板上实测逐通道 ≤6）；换成 v0.5.5 的
// `ui.LinearGradient.Stops` 真三段后与 CSS 逐值一致，也不再依赖面板底色
// （压在暗面板上时两段近似会明显偏白）。见 playlistCurrentHoverStops。
// 拖拽态原版本身就是两段（.24→白 .56），不动。
type playlistPaint struct {
	From, To ui.Color // 渐变/纯色底；Gradient 为假时只用 From
	Gradient bool
	// Stops 非空时替代 From/To（多停靠点渐变，逐值对应原版 CSS）。
	Stops       []ui.GradientStop
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

// playlistCurrentHoverStops 是当前项悬停时的三段渐变停靠点（原版
// `[data-slot="wrapper"] .playlist-current:hover`）：
//
//	linear-gradient(90deg, rgba(14,165,233,.36), rgba(14,165,233,.2) 42%, rgba(255,255,255,.62))
//
// 声明成 var 便于测试换桩退回旧的两段近似做反证。
var playlistCurrentHoverStops = func() []ui.GradientStop {
	return []ui.GradientStop{
		{Color: ui.Hex("#0ea5e9").Alpha(0.36), At: 0},
		{Color: ui.Hex("#0ea5e9").Alpha(0.20), At: 0.42},
		{Color: ui.Hex("#ffffff").Alpha(0.62), At: 1},
	}
}

// ---------------------------------------------------------------- 分集名的白色压印
//
// 原版给播放列表当前项的分集名还压了一条**文字阴影**（既不是盒子阴影，也不是
// 底色——这个元素此前 mygo 一层阴影都没画）：
//
//	[data-slot="wrapper"] .text-blue-600 {                    globals.css:3942
//	    color:#075985; font-weight:800;
//	    text-shadow: 0 1px 0 rgba(255,255,255,.52);
//	}
//	[data-slot="wrapper"] .playlist-current .text-blue-600,   globals.css:3948
//	[data-slot="wrapper"] .playlist-current p:first-of-type {
//	    color:#064e7a; font-weight:800;
//	    text-shadow: 0 1px 0 rgba(255,255,255,.65);
//	}
//
// 后两条特异度更高（(0,3,0) / (0,2,0) 对 (0,2,0) 且更靠后），而原版全文只有
// playlist.tsx:114 一处用 `text-blue-600`（正是当前项的首行 `<p>`），所以**实际
// 生效的永远是 .65 那一档**：首行 = #064e7a / 800 + 白 .65 压印，次行只有
// #334155 / 600（无压印）。这个压印是「当前项标题看着发亮、有厚度」的来源，
// 少了它就比原版平、比常态行更沉。
const (
	// playlistPartShadowY 是压印的下移量（text-shadow 的 y 偏移 1px；
	// blur 为 0，所以是硬边副本，不做高斯）。
	playlistPartShadowY = 1
	// playlistPartShadowInk 是压印的白不透明度（.playlist-current 的 .65；
	// 见上面两条规则的层叠说明）。
	playlistPartShadowInk = 0.65
)

// playlistPartEmboss 给分集名补原版那条 `text-shadow: 0 1px 0 rgba(255,255,255,.65)`。
//
// 画法同播放栏图标的白色压印：框架的 paint 顺序是「背景 → Draw → 字形」，
// 所以在元素自己的 Draw 里先用**同字体、同文本、同折行宽度**画一份下移 1px 的
// 白字，元素自己的深色字形随后盖上去——只有字形轮廓下缘露出 1px 白，与 CSS 的
// text-shadow 同义（不是被用户否掉的那种方形白底）。
//
// 两处细节：
//   - `ShapeText` 只按宽度折行、没有 MaxLines=1，长文案会多出一行；用 p.Clip
//     裁到元素盒，只留第一行（元素自己 SingleLine 时本来也只显示一行）。
//   - 字体只给 Size/Weight（与原版一样同字体同字号同字重，CJK 与拉丁都用
//     系统默认族），不重复主题里的族名。
//
// 声明成 var 便于测试换桩做「有/没有压印」的因果对照（同 playerIconShadowFn）。
var playlistPartEmboss = func(b ui.Element, s string, font ui.Font, ink ui.Color) ui.Element {
	return b.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Clip(r, 0, func() {
			p.TextLayout(ui.ShapeText(s, font, r.W), r.X, r.Y+playlistPartShadowY, ink)
		})
	})
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
		p.InsetTop = ui.Hex(white).Alpha(0.60)
		p.Shadow = ui.Hex("#0284c7").Alpha(0.12)
		p.ShadowY, p.ShadowBlur = 4, 14
	case current:
		p.From = ui.Hex("#0284c7").Alpha(0.16)
		p.Border = ui.Hex("#0284c7").Alpha(0.46)
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
			p.Stops = playlistCurrentHoverStops()
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
		p.Stops = nil
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

// playlistRowHoverAnim 是记录行悬停的过渡参数。原版 `[data-slot="wrapper"] .group`
// （globals.css:3902）声明：
//
//	transition: background 0.16s ease, border-color 0.16s ease,
//	            box-shadow 0.16s ease, transform 0.12s ease;
//
// 底色 / 描边 / 投影都走 160ms 的 `ease`（= cubic-bezier(.25,.1,.25,1)）；
// 按压缩放单独走 120ms（行内已有，见 playlistRow 的 AnimatePose）。
var playlistRowHoverAnim = ui.Animation{
	Duration: 160 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// playlistRowPaintAt 把「静息」与「悬停」两套取值按 k（0 = 静息，1 = 悬停）
// 混成这一帧的取值。
//
// 为什么要混：原版 `.group` 的底色 / 描边 / 两道投影都在 `transition` 名单里，
// 指针进出时是 160ms 渐变。原生版此前直接把 playlistRowPaint 的结果铺上去，
// 只有按压缩放在动，底色与蓝边当帧硬切 —— 鼠标沿列表上下滑时每行都在「闪」，
// 与上一轮修掉的抽屉卡片悬停（drawer.go 的 card-hover）是同一族问题。
//
// 形态（纯色 ↔ 渐变、停靠点、天蓝环）跟**目标态**走：CSS 的 background-image
// 在两个 gradient 之间不可插值（离散切换），所以当前项 / 拖拽态的渐变在悬停时
// 本来就是一跳；能插值的只有纯色底、描边、内影与投影 —— 与这里逐项插值的
// 范围一致。文字色也不参与（原版 transition 名单里没有 color，换集时本来就是
// 当帧切换）。
func playlistRowPaintAt(rest, hover playlistPaint, k float32) playlistPaint {
	if k <= 0 {
		return rest
	}
	if k >= 1 {
		return hover
	}
	out := hover
	lerp := func(a, b float32) float32 { return a + (b-a)*k }
	// 底色：两侧都是纯色时才插值（渐变形态见上，跟目标态走）。
	if !rest.Gradient && len(rest.Stops) == 0 && !hover.Gradient && len(hover.Stops) == 0 {
		out.From = rest.From.Mix(hover.From, k)
	}
	out.Border = rest.Border.Mix(hover.Border, k)
	out.InsetTop = rest.InsetTop.Mix(hover.InsetTop, k)
	out.InsetBot = rest.InsetBot.Mix(hover.InsetBot, k)
	out.Shadow = rest.Shadow.Mix(hover.Shadow, k)
	out.ShadowY = lerp(rest.ShadowY, hover.ShadowY)
	out.ShadowBlur = lerp(rest.ShadowBlur, hover.ShadowBlur)
	return out
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
	// 滚动锁：滚动中把悬停态一起冻住（与抽屉卡片同一约定，见 renderCard）。
	// 行在指针下滑过时 row.Hovered() 会在相邻行间跳，底/边/删除键显隐每帧翻转。
	locked := a.scrollHoldHeld
	// 原版行：p-2（8）、rounded-lg（8，但 .group 覆写成 10）、gap-2（8）。
	//
	// 左内边距：原版当前项多一条 `border-left: 4px`（内容起点 4+8=12px），
	// 原生版早先为了对齐这个 12px 把「当前项左内边距收成 0」，让强调条(4)+gap(8)
	// 把封面推到 12px。但强调条后来被删掉了（用户反馈那条竖线太脏，见
	// playlistRowPaint），左内边距却没收回来——当前项的缩略图直接贴在行左缘
	// （用户报「当前播放项的左侧图片贴边了」）。既然不画强调条，当前项就与
	// 普通项一样吃 8px 左内边距（内容起点 1px 边框 + 8 = 9），封面不再贴边。
	row := ui.Row(c).Key("pl-"+item.ID).FillWidth().Padding(8, 8, 8, playlistRowPadL).
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

	rowHovered := !locked && row.Hovered()
	if rowHovered {
		a.hoverChannels.Add(1)
	}

	// 悬停过渡：k 由单条 float 通道插值（原版 `.group` 的 160ms transition）。
	// 通道首次建立时以**第一个目标**为初值，所以先按静息态把 k 种到 0 —— 否则
	// 第一次悬停会瞬亮（见 cardglow.go 里同一处坑）。渐隐结束后把通道钉回 0，
	// 下一次悬停才是从 0 起跳。
	rest := playlistRowPaint(current, false, dragOver, row.Pressed())
	hover := playlistRowPaint(current, true, dragOver, row.Pressed())
	target := float32(0)
	if rowHovered {
		target = 1
	}
	k := row.AnimateTo("pl-row-hover", target, playlistRowHoverAnim)
	if target == 0 && k < 0.01 {
		k = row.Jump("pl-row-hover", 0)
	}
	p := playlistRowPaintAt(rest, hover, k)
	switch {
	case len(p.Stops) > 0:
		// 原版 .playlist-current:hover 的三段蓝→白渐变（逐值见停靠点函数）。
		row.LinearGradient(ui.LinearGradient{Angle: 90, Stops: p.Stops})
	case p.Gradient:
		// 拖拽悬停：原版 `.bg-blue-50` 覆写为两段天蓝→白渐变。
		row.Gradient(p.From, p.To, 90)
	default:
		row.Background(p.From)
	}
	row.Border(1, p.Border).InsetShadow(0, 1, 0, 0, p.InsetTop)
	if p.InsetBot != ui.Transparent {
		row.InsetShadow(0, -1, 0, 0, p.InsetBot)
	}
	row.Shadow(0, p.ShadowY, p.ShadowBlur, 0, p.Shadow)
	if p.Ring && k > 0.002 {
		// 原版 hover 的第二层投影：0 0 0 1px rgba(14,165,233,.12)。环本身也在
		// `box-shadow` 的 transition 里，所以它的 alpha 跟着 k 渐显（k=1 时与原值
		// 逐值相等）。
		row.Shadow(0, 0, 0, 1, ui.Hex("#0ea5e9").Alpha(0.12*k))
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
		// 用户反馈这条左缘竖线太突兀（行本身已有蓝色底 + 蓝边 + 蓝投影，
		// 再叠一条纯蓝 4px 竖条反而脏），本轮直接去掉：playlistRowPaint
		// 不再给任何状态设 AccentW（保持 0），这个分支就不会画。保留字段 /
		// 分支是为了必要时能恢复，且测试能钉住「不再有强调条」。
		//
		// 注意若将来恢复：要用 FillHeight 而不是 Fill —— **Fill() 会把宽度也设成
		// 100%**，色条会撑满整行、把封面和文字挤成 0 宽。
		if p.AccentW > 0 {
			ui.Box(c).Width(p.AccentW).FillHeight().Shrink(0).Radius(playlistAccentRadius).
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
				// .playlist-current p:first-of-type（#064e7a/800 + 白 .65 压印）。
				part := ui.Text(c, item.Part).FontSize(14).SingleLine().
					TextColor(p.PartInk).FontWeight(p.PartWeight).
					Height(th).Padding(tsh, 0, 0, 0)
				if current {
					// 压印只属于当前项（CSS 里那条规则挂在 .playlist-current 下）；
					// 常态行没有任何 text-shadow。Draw 是就地挂 paintFn，返回值不用接。
					playlistPartEmboss(part, item.Part,
						ui.Font{Size: 14, Weight: p.PartWeight},
						ui.Hex("#ffffff").Alpha(playlistPartShadowInk))
				}
			})
			// 原版次行：常态 .text-gray-400（#64748b），当前项
			// .playlist-current p:last-of-type（#334155/600）。
			ui.Text(c, item.Title).FontSize(12).SingleLine().
				TextColor(p.TitleInk).FontWeight(p.TitleWeight).
				Height(mh).Padding(msh, 0, 0, 0)
		})

		// 我的列表才允许删除（排序靠拖拽）。原版那颗键**始终渲染**，靠
		// `opacity-0 group-hover:opacity-100 transition-opacity` 淡入淡出 ——
		// 始终占位，悬停不会把文字列抽窄（见 playlistDeleteOpacity）。
		if editable {
			a.rowIconButton(c, fmt.Sprintf("pl-del-%s", item.ID), "删除", iconClose,
				ui.Hex("#999999"), playlistDeleteOpacity(rowHovered), func() {
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
