package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/imagecache"
)

// ---------------------------------------------------------------- 主区响应式

// homeLayout 是主区（.home-stage / .home-now-playing / .home-searchbar）
// 在某个窗口宽度下的布局取值。原版靠 globals.css 的媒体查询切换：
//
//	基线        .home-stage padding 20 28 0；.home-searchbar max-width 520；
//	            .home-now-playing 两栏「304px + 1fr」gap 28 padding 0 8 32；
//	            .cover-shell 292。
//	≤680px      .home-stage padding 12 16 0；.home-searchbar max-width 100%；
//	            .home-now-playing 单列 1fr（align-content:start、gap 12、
//	            padding 16 8 12、overflow-y:auto）；
//	            .cover-shell width min(196px, 56vw)。
//
// 注意搜索药丸的基线是 `width:100%; max-width:520px` —— 它本来就随父宽收窄，
// 原生版此前把宽度写死成 520，窗口窄于 576（520 + 两侧 28）时药丸会溢出窗口。
type homeLayout struct {
	StagePadY   float32 // .home-stage 顶部内边距
	StagePadX   float32 // .home-stage 左右内边距
	SearchWidth float32 // 搜索药丸宽度 = min(max-width, 可用宽)
	Single      bool    // .home-now-playing 是否单列
	CoverShell  float32 // .cover-shell 边长（正方形）
}

// homeNarrowWidth 是主区单列的断点（原版 @media (max-width: 680px)）。
const homeNarrowWidth = 680

// nowPlayingPadX 是 .home-now-playing 的左右内边距（原版基线/窄窗都是 8px，
// 见 globals.css 的 `padding: 0 8px 32px` 与窄窗 `padding: 16px 8px 12px`）。
// 声明成常量供标题列宽预算与内边距共用，避免两处各写一个 8 而漂移。
const nowPlayingPadX float32 = 8

// faceHoverAnim 是主区 UP 主头像悬停放大 / 回落的动画：原版 #video-owner-face 的
// `transition: box-shadow 0.2s ease, transform 0.18s ease`（globals.css:2178），
// 用 CSS `ease` = cubic-bezier(.25,.1,.25,1)。之前用 ui.Snappy 弹簧（约 500ms），
// 悬停放大/回落偏黏。
var faceHoverAnim = ui.Animation{
	Duration: 180 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// homeLayoutFor 按窗口宽度算出主区这一帧的布局。抽成纯函数便于单测断点与取值。
func homeLayoutFor(width float32) homeLayout {
	l := homeLayout{StagePadY: 20, StagePadX: 28, CoverShell: 292}
	if width <= homeNarrowWidth {
		l.StagePadY, l.StagePadX = 12, 16
		l.Single = true
		l.CoverShell = minF(196, 0.56*width)
	}
	// .home-searchbar width:100% / max-width（窄窗改 100%，但基线 520 本就封顶）。
	l.SearchWidth = minF(SearchBarWidth, width-2*l.StagePadX)
	return l
}

// ---------------------------------------------------------------- 搜索药丸

// searchBar 是居中的液态玻璃搜索药丸（原版 .home-searchbar）：
// max-width 520、高 46、圆角 13、内边距 4、子元素间距 8，
// 内容依次是「输入内腔 + 四个内容入口 + 头像」。
func (a *App) searchBar(c *ui.Context, l homeLayout) {
	ui.Row(c).FillWidth().Height(SearchBarHeight).Shrink(0).Justify(ui.Center).Children(func() {
		// 原版 .home-searchbar 自己有 backdrop-filter（blur(20px) saturate(1.45)）
		// + 半透明白底（--liquid-bg 白 .22）。这里用 cssGlass 按这个顺序铺：
		// 只模糊、不染色，再把白底和洗光铺上去（见 cssglass.go）。
		// 原版 .home-searchbar：1px 玻璃边、::before 是 blur(20px)+白 22%、
		// ::after 是 1px 内圈描光（上缘亮、下缘暗），外面两层淡投影；
		// 悬停时描边转亮、投影抬起。
		// 材质是框架玻璃，它自带边缘聚光与描光，所以不再叠 1px 边框/inset 内影
		//（那是原版手搓同一件事的做法，两层描边会显脏）。只保留外层投影。
		pill := ui.Row(c).Width(l.SearchWidth).Height(SearchBarHeight).Radius(SearchRadius).
			Padding(SearchPadding).Gap(8).AlignItems(ui.Center).
			// 原版 .home-searchbar::before：blur(20px) + 白 22% + 静态洗光。
			// 不再用 glass.Glass（Regular 会把背景的黑抬到 54%，整块发白、不透）。
			Material(cssGlassLayer(liquidBlurRadius, searchPillVeil(), sheenGlassBaked)).
			// 原版 ::after 的 conic 描光 + 色散边（玻璃材质只管 ::before 那层）。
			Draw(func(p *ui.Painter, r ui.Rect) {
				paintPlayerRim(p, r, SearchRadius, playerRimOpacity(a.Theme.Dark, false))
			})
		// 原版 box-shadow 走 --liquid-shadow / --liquid-shadow-lifted 二选一（
		// mygo 的 Shadow 累加，见 searchPillShadows）。
		applyShadows(pill, searchPillShadows(pill.Hovered()))
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
	// 搜索中：原版给输入框与提交键都挂 `disabled={isSearching}`，Enter 与点击
	// 都提前 return（searchForm.tsx 的 handleKeyDown / onClick），挡住连点重复搜。
	searching := a.list().Loading
	// 原版内腔：padding-left 10（文字视觉起点）/ padding-right 1（提交键
	// 离右 1px、离上下 1px，四周等距）；内凹 = 顶部深影 + 底部亮高光。
	// 原版 [data-slot="input-wrapper"]：高 36、左内边距 10、右 1；
	// 1px 边 rgba(100,116,139,.14)、底 rgba(15,23,42,.025)（不是白底，
	// 是很淡的内凹腔）、内影 inset 0 1px 2px rgba(15,23,42,.06) +
	// inset 0 -0.5px 0 白 .3；聚焦时描边转蓝并加一圈 2px 光环。
	slot := ui.Row(c).Grow(1).Height(SearchSlotH).Radius(SearchSlotR).
		Padding(0, 1, 0, 10).
		Background(ui.Hex("#0f172a").Alpha(0.025)).
		Border(1, ui.Hex("#64748b").Alpha(0.14)).
		InsetShadow(0, 1, 2, 0, ui.Hex("#0f172a").Alpha(0.06)).
		InsetShadow(0, -1, 0, 0, ui.Hex("#ffffff").Alpha(0.30)).
		AlignItems(ui.Center)
	if slot.FocusWithin() {
		slot.Border(1, ui.Hex("#0ea5e9").Alpha(0.40)).
			Background(ui.Hex("#ffffff").Alpha(0.05)).
			Shadow(0, 0, 0, 2, ui.Hex("#0ea5e9").Alpha(0.14))
	}
	var in ui.Element
	slot.Children(func() {
		in = ui.TextInputBase(c, &a.Query).Grow(1).FontSize(14).
			TextColor(ui.Hex("#334155")).
			Placeholder("B站 / 关键词 / 视频链接").Label("搜索")
		if in.Submitted() && !searching {
			a.runSearch()
		}
		// 原版 .search-submit-btn：32×32、圆角 6、默认透明 + opacity .82、
		// 图标 #64748b；悬停 opacity 1、图标转 #0284c7、底白 50%、上浮 1px；
		// 按下缩到 .92。底色/图标色 0.18s、不透明度 0.16s 淡入
		// （topToolBgTo / topToolInk / topToolFade），不是瞬时切换。
		submit := ui.ButtonBase(c.Key("search-submit")).Size(ToolButton, ToolButton).
			Radius(ToolButtonR).Center().Label("搜索").Tooltip("搜索")
		// 搜索中这颗键在原版是 `disabled`：不响应点击，也吃不到 `:active`（但仍吃
		// `:hover`）。原生版不再借框架的 Disabled（见下面的说明），所以显式把「按下」
		// 这一路在搜索中掐掉。
		pressed := submit.Pressed() && !searching
		sub := topToolStyleFor(topToolSubmit, submit.Hovered(), pressed)
		topToolBgTo(submit, sub.Bg)
		topToolFade(submit, sub.Opacity)
		// 原版 .search-submit-btn 常态带左上角径向聚光，按下换成居中光。
		if pressed {
			btnLightPress.draw(submit, ToolButtonR)
		} else {
			btnLightTool.draw(submit, ToolButtonR)
		}
		// 原版 .search-submit-btn::after：悬停才显形的内高光 + 弱投影（不是描边）；
		// 按下时 ::after 归零、换成 :active 的内压阴影；搜索中（`disabled`）原版
		// `.search-submit-btn:disabled::after { opacity: 0 }` 把它整层关掉，所以
		// 即便悬停在键上也不再铺（悬停的底色 / 图标色仍照旧生效）。
		if searching {
			// 搜索中 ::after 归零。
		} else if pressed {
			navPressShadow(submit)
		} else if submit.Hovered() {
			topToolHoverHighlight(submit, topToolSubmit)
		}
		topToolPose(submit)
		// 原版 `.search-submit-btn:disabled` **不改**不透明度（只把 ::after 归零），
		// 所以搜索中这颗键仍是常态度 .82、悬停也照样生效——但 mygo 的禁用节点会把
		// 整棵子树再乘 .5（ui/paint.go），.82 会被压到 .41，转圈比原版暗一半，而元
		// 素不透明度封顶 1 最多只能补到 .5，补不回 .82。所以这里不借框架的 Disabled，
		// 改用原版同样语义「自己挡住交互」：下面 Submitted / Clicked 都带 !searching
		//（disable.go 里有这条例外的说明）。
		submit.Children(func() {
			if searching {
				// 搜索中：原版 `.search-submit-spinner` 是 14×14、2px 浅蓝整圈 +
				// 深蓝顶弧的圆环（0.7s linear infinite）——不是 mygo 的辐条
				// Spinner，也不是 18px 的对话框尺寸；用共用实现逐值复现。
				ringSpinner(c, t.Blue, searchSubmitSpinnerSize, searchSubmitSpinnerRing,
					"search-spinner", 0, 0, 0, 0).Label("搜索中")
			} else {
				topToolInk(ui.Icon(c, iconMagnifier).Size(18, 18), sub.Ink)
			}
		})
		if submit.Clicked() && !searching {
			a.runSearch()
		}
	})
	// 点内腔任意处都聚焦到输入框（原版是 HeroUI Input 自己的行为）。
	if slot.Clicked() && in.Valid() {
		in.Focus()
	}
}

func (a *App) runSearch() {
	if a.Query == "" {
		return
	}
	// 粘贴的是 B 站视频链接就直接打开它，不走搜索。
	if strings.Contains(a.Query, "bilibili.com/video/") && a.Act.UrlJump != nil {
		a.Act.UrlJump(a.Query)
		return
	}
	if a.Act.Search == nil {
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
	b := ui.ButtonBase(c.Key("nav-"+key)).Size(ToolButton, ToolButton).Radius(ToolButtonR).
		Center().Label(label).Tooltip(label)
	// 原版 .top-tool-btn：基态透明 + opacity .82 + 图标 #64748b；悬停铺白 .65、
	// 图标转 #0284c7、上浮 1px；按下缩到 .92。底色/图标色 0.18s、不透明度
	// 0.16s 淡入（topToolBgTo / topToolInk / topToolFade），不是瞬时切换。
	s := topToolStyleFor(topToolEntry, b.Hovered(), b.Pressed())
	topToolBgTo(b, s.Bg)
	topToolFade(b, s.Opacity)
	// 原版 .top-tool-btn 常态带左上角径向聚光，按下换成居中光。
	if b.Pressed() {
		btnLightPress.draw(b, ToolButtonR)
	} else {
		btnLightTool.draw(b, ToolButtonR)
	}
	// 原版 .top-tool-btn::after：悬停才显形的内高光 + 弱投影（不是描边）。
	// 之前这里用「白 .55 描边 + inset .55」近似，比原版（白 .9 / 10px）低一档。
	// 按下时 ::after 归零、换成 :active 的内压阴影。
	if b.Pressed() {
		navPressShadow(b)
	} else if b.Hovered() {
		topToolHoverHighlight(b, topToolEntry)
	}
	topToolPose(b)
	b.Children(func() { topToolInk(ui.Icon(c, ic).Size(18, 18), s.Ink) })
	// 原版 `.top-tool-btn:focus-visible`：`outline: 2px solid rgba(14,165,233,.45);
	// outline-offset: 2px`。
	focusOutline(b, ToolButtonR, focusSky45)
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
	// 圆形头像键：半透明填充 + 白边 + 两态投影（原版 .home-user-avatar 无
	// backdrop-filter，但常态有 0 6px 16px 的墨色投影、悬停上浮并转蓝调投影）。
	// 文案与原版一致：未登录是「登录账号」，已登录是「切换登录账号」（aria-label），
	// title 分别提示「点击登录账号」/「点击登录其他账号」。
	b := ui.ButtonBase(c.Key("user")).Size(AvatarSize, AvatarSize).Radius(RadiusPill).
		Center().Clip().Background(ui.Hex("#ffffff").Alpha(0.32)).
		Border(1, ui.Hex("#ffffff").Alpha(0.55)).
		Label(pick(a.LoggedIn, "切换登录账号", "登录账号")).
		Tooltip(pick(a.LoggedIn, "点击登录其他账号", "点击登录账号"))
	avatarShadow(b)
	avatarLift(b)
	b.Children(func() {
		// 头像一直挂在药丸最右端（抽屉开着时也可见），每帧重申「常驻」，
		// 免得被抽屉里一轮封面抓取挤出 LRU 后重抓（表现为头像不定期闪动）。
		a.Images.Retain(a.Face, imagecache.WidthAvatar)
		if bmp := a.Images.Bitmap(a.Face, imagecache.WidthAvatar); bmp != nil {
			ui.Image(c, bmp).Fill().Fit(ui.Cover)
		} else if !a.LoggedIn {
			// 未登录：原版 HeroUI Avatar 的默认 fallback 是人形剪影，不是首字
			//（未登录 UName 为空，从前这里画出一个 "?"）。
			b.Draw(func(p *ui.Painter, r ui.Rect) { userAvatarSilhouette(p, r, t.Faint) })
		} else {
			ui.Text(c, avatarInitial(a.UName)).FontSize(13).Bold().TextColor(t.Muted)
		}
	})
	if b.Clicked() && a.Act.Login != nil {
		a.Act.Login()
	}
	// 原版 `.home-user-button:focus-visible`：`outline: 2px solid
	// var(--studio-blue); outline-offset: 3px`。
	focusOutline(b, RadiusPill, focusStudioWide)
}

// avatarInitial 是已登录但头像还没就绪时显示的字（用户名首字）。未登录不
// 走这里：原版 HeroUI Avatar 的默认 fallback 是人形剪影（见
// userAvatarSilhouette），从前未登录 UName 为空会画出一个 "?"。
func avatarInitial(name string) string {
	for _, r := range name {
		return string(r)
	}
	return "?"
}

// userAvatarSilhouette 画原版 @heroui/avatar 默认 fallback 的人形剪影：
// 一个头 + 一段肩（灰色 currentColor，`AvatarIcon`/`AvatarFallback` 里的
// person 图形）。未登录时画它，替代从前的 "?" 首字。
func userAvatarSilhouette(p *ui.Painter, r ui.Rect, col ui.Color) {
	s := min(r.W, r.H)
	if s <= 0 {
		return
	}
	cx := r.X + r.W/2
	// 头：半径约 0.18·边长，圆心落在上 35% 处。
	var head ui.Path
	head.Circle(cx, r.Y+r.H*0.35, s*0.18)
	p.FillPath(&head, col)
	// 肩：底边贴盒底、跨度 0.72·边长，顶点半高，用二次曲线收成圆肩。
	var body ui.Path
	body.MoveTo(cx-s*0.36, r.Y+r.H)
	body.QuadTo(cx, r.Y+r.H*0.56, cx+s*0.36, r.Y+r.H)
	body.Close()
	p.FillPath(&body, col)
}

// ---------------------------------------------------------------- 正在播放

// nowPlaying 是主区下半部分（原版 .home-now-playing）：
// 宽窗是两栏 grid「304px + minmax(0,1fr)」、间距 28、纵向居中、底部留 32；
// 窄窗（≤680）退化成单列 —— 封面在上、信息在下，间距 12、整体可纵向滚动
// （原版 @media (max-width: 680px) 把 grid 改成 1fr 并加 overflow-y:auto）。
func (a *App) nowPlaying(c *ui.Context, l homeLayout) {
	if l.Single {
		ui.Scroll(c).FillWidth().Grow(1).Shrink(1).Padding(16, nowPlayingPadX, 12, nowPlayingPadX).Children(func() {
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				a.coverDisc(c, l.CoverShell)
				// 窄窗标题盒高随折行数变化，需要标题列宽：主区宽减两侧内边距
				// （本块的 nowPlayingPadX，与下同）。
				a.videoInfo(c, false, windowWidth(c)-2*(l.StagePadX+nowPlayingPadX))
			})
		})
		return
	}
	ui.Row(c).FillWidth().Grow(1).Shrink(1).Gap(28).AlignItems(ui.Center).
		Padding(0, nowPlayingPadX, 32, nowPlayingPadX).Children(func() {
		ui.Box(c).Width(304).Shrink(0).Children(func() { a.coverDisc(c, l.CoverShell) })
		a.videoInfo(c, true, 0)
	})
}

// coverDisc 是左栏的封面（原版 .cover-shell 292px，内部 #video-cover）：
//
//	disc   （默认）圆形唱片，播放时 22s 转一圈，点一下切换播放/暂停；
//	square 静态圆角方块（省 GPU），点一下也切换播放/暂停。
//
// side 是 .cover-shell 的边长（宽窗 292；窄窗 min(196, 56vw)）——内部的玻璃环
// （inset 3%）与封面（90%）都按比例缩放。
//
// 右下角三个开关（原版 .cover-mode-toggle）：切换碟片/封面、封面背景、高级质感。
func (a *App) coverDisc(c *ui.Context, side float32) {
	ui.Row(c).FillWidth().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		shell := ui.Box(c).Size(side, side).Shrink(0).Center()
		// 原版 .cover-shell 是 `display:grid; place-items:center` —— 玻璃环与
		// 封面都在 292 的 shell 里**两轴居中**。之前碟片模式只给了
		// `AlignSelf(ui.Center)`（只居中横轴），玻璃环贴着 shell 顶边，
		// 唱片整体比方形封面高 9px。
		// 原版 .cover-mode-toggle 默认 opacity:0，.cover-shell:hover 才显形。
		shellHovered := shell.Hovered()
		shell.Children(func() {
			if a.CoverMode == "square" {
				// 原版 #video-cover.cover-square 同样是 shell 的 90%（= 263），
				// 而不是铺满 292 的整块 —— 之前用 Fill() 把方块封面放大到
				// 292，比碟片模式（263）大了一圈，右下角三个开关也贴到了边上。
				// 圆角 12（.cover-art-square / #video-cover.cover-square），不是正圆。
				sq := ui.ButtonBase(c.Key("disc")).Size(side*0.9, side*0.9).Radius(12).Clip().
					Background(ui.Hex("#0f172a")).
					Border(1, ui.Hex("#ffffff").Alpha(0.20)).
					Label(pick(a.Playing, "暂停", "播放"))
				// 方形封面只有常态 / 悬停两态（原版没有 .cover-square.is-playing）。
				cs := coverSurfaceFor(false, sq.Hovered())
				dy, blur, outer, inset := coverShadowAnim(sq, cs)
				sq.InsetShadow(0, 0, 0, 1, inset).
					Shadow(0, dy, blur, 0, outer)
				sq.Children(func() { a.coverOrLogo(c) })
				if sq.Clicked() && a.Act.TogglePlay != nil {
					a.Act.TogglePlay()
				}
			} else {
				// 外圈玻璃环（原版 .cover-shell::before）：inset 3% →
				// 292 × 0.94 ≈ 274，1px 白边 + 柔和投影；唱片 263 居中其内。
				ring := ui.Box(c).Size(side*0.94, side*0.94).Radius(RadiusPill).
					Border(1, ui.Hex("#ffffff").Alpha(0.20)).
					Shadow(0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.12)).Center()
				ring.Children(func() {
					// 唱片（原版 #video-cover）：90% 于外环 = 263，深底 + 1px
					// 白边 + 内侧 1px 白线（inset 0 0 0 1px rgba(255,255,255,.1)）。
					disc := ui.ButtonBase(c.Key("disc")).Size(side*0.9, side*0.9).Radius(RadiusPill).Clip().
						Background(ui.Hex("#0f172a")).
						Border(1, ui.Hex("#ffffff").Alpha(0.20)).
						Label(pick(a.Playing, "暂停", "播放"))
					// #video-cover 的三态描边/投影：常态深色投影、悬停与播放中转
					// 蓝色光晕。之前只画了内侧 1px 白线，唱片像平贴在玻璃环里，
					// 播放时也没有那圈蓝光。
					cs := coverSurfaceFor(a.Playing, disc.Hovered())
					dy, blur, outer, inset := coverShadowAnim(disc, cs)
					disc.InsetShadow(0, 0, 0, 1, inset).
						Shadow(0, dy, blur, 0, outer)
					disc.Children(func() {
						// 唱片：播放时 22s 一圈（原版 #video-cover.record-disc）。
						//
						// **用 AnimatePose，不要用 Rotate**：Rotate 的角度是在
						// 视图里算的，意味着每一帧都要重建整个视图 —— 实测
						// （llvmpipe 纯 CPU 渲染、800×600）连续旋转要吃掉 200%
						// 单核，一个音乐播放器不该这样。AnimatePose 的中间帧走
						// 框架「只重绘不重建」那条路（runtime 的 repaintFrame +
						// repose），代价低一个数量级。
						//
						// 做法：每次视图重建时把目标角度推到 discSpinStep 之后
						// 应到的角度，用等长的线性过渡覆盖这一段；中间帧由框架
						// 插值，角速度恒定。播放时进度上报每 500ms 会重建一次
						// 视图，所以 1s 的步长留了余量。
						// 桌面要求减少动态效果时原版把这条 animation 整个撤掉
						// （`.record-disc { animation: none }`）——唱片不再转、回正到
						// 0°；这里停步进并复位角度（见 discTargetAngle / resetDisc）。
						if a.coverBitmap() != nil || coverLogo() != nil {
							reduce := reduceMotion(c)
							now := time.Now()
							if reduce {
								a.resetDisc(now)
							} else {
								a.advanceDisc(now)
							}
							img := a.coverImage(c)
							img.AnimatePose(ui.Pose{Rotate: discTargetAngle(reduce, a.Playing, a.discAngle)},
								ui.Animation{Duration: discSpinStep, Ease: ui.Linear})
						}
					})
					if disc.Clicked() && a.Act.TogglePlay != nil {
						a.Act.TogglePlay()
					}
				})
			}

			// 三个小开关：碟片/封面、封面背景、高级质感。
			// 原版三个键都是 absolute，靠各自的 `right` 贴边：模式键 right:8（最右）、
			// 封面背景 right:42（中）、高级质感 right:76（最左）——间距 6（28 + 6 = 34）。
			// 为了用一个 Row 表达同样的几何，**子元素从左到右的顺序要反过来**
			// （premium → ambient → mode），否则三个键左右镜像（之前就是反的）。
			// 整组随封面悬停淡入淡出（原版 .cover-mode-toggle 的
			// opacity 0 ↔ .85 + 0.2s 过渡）。交给 AnimateTo 平滑。
			group := ui.Row(c).Absolute().Right(8).Bottom(8).Gap(6)
			target := float32(0)
			if shellHovered {
				target = 0.85
			}
			group.Opacity(group.AnimateTo("cover-toggles", target,
				ui.Animation{Duration: 200 * time.Millisecond})).Children(func() {
				// tooltip 文案与原版一致（碟片/封面模式、封面背景、高级质感）。
				// 顺序必须是「高级质感 / 封面背景 / 模式切换」——对应的 right 偏移
				// 是 76 / 42 / 8。
				a.coverToggle(c, "premium",
					pick(a.Premium, "关闭高级质感（更省电）", "开启高级质感"),
					iconSparkles, func() {
						if a.Act.TogglePremium != nil {
							a.Act.TogglePremium()
						}
					})
				a.coverToggle(c, "ambient",
					pick(a.Ambient, "关闭封面背景", "开启封面背景"),
					iconHalo, func() {
						if a.Act.ToggleAmbient != nil {
							a.Act.ToggleAmbient()
						}
					})
				a.coverToggle(c, "cover-mode",
					pick(a.CoverMode == "square", "切换为碟片模式", "切换为封面模式"),
					iconRefresh, func() {
						if a.Act.SetCoverMode == nil {
							return
						}
						if a.CoverMode == "square" {
							a.Act.SetCoverMode("disc")
						} else {
							a.Act.SetCoverMode("square")
						}
					})
			})
		})
	})
}

// coverSurface 是 #video-cover（碟片 / 方形封面）这一帧的描边与外投影。
// 取值直接抄原版 globals.css 的三条规则：
//
//	常态    inset 0 0 0 1px rgba(255,255,255,.10) + 0 8px 24px rgba(15,23,42,.10)
//	悬停    inset ... .15                        + 0 12px 32px rgba(14,165,233,.12)
//	播放中  inset ... .15                        + 0 12px 36px rgba(14,165,233,.16)
//
// 描边色固定在 1px、不随态变（Inset 只是颜色）；Dy/Blur 是外投影的偏移与模糊。
type coverSurface struct {
	Inset    ui.Color
	Dy, Blur float32
	Outer    ui.Color
}

// coverShadowAnim 返回带过渡的阴影参数（y-offset、blur、外阴影色、内阴影色），
// 对齐原版 `#video-cover { transition: box-shadow 0.3s ease }` 与
// `#video-cover.cover-square { transition: box-shadow 0.3s ease }`。
// 原版三态投影切换（常态/悬停/播放中）有 0.3s 渐变过渡，mygo 用
// AnimateTo 过渡 y/blur、用 AnimateColor 过渡颜色来复现。
// 无头测试环境（hasState=false）下 AnimateTo/AnimateColor 直接返回 target，
// 不影响渲染；减少动态效果偏好下框架自动跳到终态（与原版 animation: none 等效）。
func coverShadowAnim(el ui.Element, cs coverSurface) (dy, blur float32, outer, inset ui.Color) {
	anim := ui.Animation{Duration: 300 * time.Millisecond, Ease: ui.CubicBezier(0.25, 0.5, 0.25, 1)}
	dy = el.AnimateTo("shadow-y", cs.Dy, anim)
	blur = el.AnimateTo("shadow-blur", cs.Blur, anim)
	outer = el.AnimateColor("shadow-color", cs.Outer, anim)
	inset = el.AnimateColor("inset-shadow", cs.Inset, anim)
	return
}

// coverSurfaceFor 给出 #video-cover 当前该用的描边/投影。
//
// playing 压过 hovered：`#video-cover.is-playing` 与 `#video-cover:hover`
// 特异度相同（1,1,0），而前者在 globals.css 里排在后者之后。
// （方形封面原版没有 .is-playing 规则，调用方传 playing=false。）
func coverSurfaceFor(playing, hovered bool) coverSurface {
	s := coverSurface{
		Inset: ui.Hex("#ffffff").Alpha(0.10),
		Dy:    8, Blur: 24, Outer: ui.Hex("#0f172a").Alpha(0.10),
	}
	if hovered {
		s.Inset = ui.Hex("#ffffff").Alpha(0.15)
		s.Dy, s.Blur = 12, 32
		s.Outer = ui.Hex("#0ea5e9").Alpha(0.12)
	}
	if playing {
		s.Inset = ui.Hex("#ffffff").Alpha(0.15)
		s.Dy, s.Blur = 12, 36
		s.Outer = ui.Hex("#0ea5e9").Alpha(0.16)
	}
	return s
}

// coverToggleActive 报告封面上的某个小开关这一帧是不是激活态。
//
// 原版 CSS 只给两个键定义了 `is-active` —— `.cover-background-toggle.is-active`
// 与 `.cover-premium-toggle.is-active`；模式切换键就是 `.cover-mode-toggle`
// 本身（没有后缀、也没有 is-active 规则），它的底色一直是中性深色。之前这里
// 给三个键统一传 true，模式键被误画成了蓝色激活态。
func coverToggleActive(key string, ambient, premium bool) bool {
	switch key {
	case "ambient":
		return ambient
	case "premium":
		return premium
	default:
		return false
	}
}

// coverToggleStyle 返回封面小开关这一帧的底色、边色与图标色，对应原版
// `.cover-mode-toggle` 的几条规则（按 CSS 里出现的先后与特异度定优先级）：
//
//	默认   background rgba(15,23,42,.35)  border rgba(255,255,255,.25)  color rgba(255,255,255,.70)
//	悬停   background rgba(15,23,42,.55)  color rgba(255,255,255,.95)
//	激活   background rgba(14,165,233,.28) border rgba(125,211,252,.45)
//
// `.x-toggle.is-active` 与 `.cover-mode-toggle:hover` 特异度相同（都是两个类），
// 但 is-active 规则在 CSS 里更靠后，所以**激活态的底色/描边盖过悬停**，而
// 图标色只有 base 与 :hover 在写 —— 即激活态图标亮度仍由 hover 决定
// （之前激活直接把图标提到 .95，是多余的偏差）。
func coverToggleStyle(active, hovered bool) (bg, border, ink ui.Color) {
	switch {
	case active:
		bg, border = ui.Hex("#0ea5e9").Alpha(0.28), ui.Hex("#7dd3fc").Alpha(0.45)
	case hovered:
		bg, border = ui.Hex("#0f172a").Alpha(0.55), ui.Hex("#ffffff").Alpha(0.25)
	default:
		bg, border = ui.Hex("#0f172a").Alpha(0.35), ui.Hex("#ffffff").Alpha(0.25)
	}
	ink = ui.Hex("#ffffff").Alpha(0.70)
	if hovered {
		ink = ui.Hex("#ffffff").Alpha(0.95)
	}
	return bg, border, ink
}

// coverToggleStateful 报告封面上的某个小开关是不是**保持型开关**。原版只给
// 「封面背景 / 高级质感」两颗键写了 `aria-pressed`，模式切换键（Refresh）没有——
// 它是「点一下换一种模式」的动作键，不是有开/关状态的切换键。无障碍上 aria-pressed
// 的 button 是 toggle button（RoleToggleButton + Checked），动作键保持普通按钮。
func coverToggleStateful(key string) bool {
	return key == "ambient" || key == "premium"
}

// coverToggle 是封面上的一个小开关（原版 .cover-mode-toggle）。激活态由
// coverToggleActive 按 key 决定（只有封面背景 / 高级质感有 is-active）。
//
// 无障碍：原版给封面背景 / 高级质感两颗键 `aria-pressed={ambientBackgroundEnabled}`
// / `aria-pressed={premiumTexture}`。mygo 对应 RoleToggleButton + Checked(active)，
// 且两者必须在**同一条元素链**上（普通 RoleButton 会把 Checked 丢掉，Windows/macOS
// 只有 toggle 角色才给 IToggleProvider / AXValue）。模式键无 aria-pressed，不接。
func (a *App) coverToggle(c *ui.Context, key, label string, ic *ui.SVG, fn func()) {
	b := ui.ButtonBase(c.Key("cover-"+key)).Size(28, 28).Radius(RadiusPill).
		Center().Label(label).Tooltip(label)
	// 这几个键浮在封面上：深色半透明填充 + 1px 白边（原版
	// .cover-mode-toggle 也没有 backdrop-filter，之前用 Clear 玻璃多算了
	// 三次 backdrop 重算）。
	active := coverToggleActive(key, a.Ambient, a.Premium)
	if coverToggleStateful(key) {
		b.Role(ui.RoleToggleButton).Checked(active)
	}
	bg, border, ink := coverToggleStyle(active, b.Hovered())
	// 原版 .cover-mode-toggle { transition: opacity 0.2s ease, background 0.2s ease }
	// —— 背景色切换有 0.2s 过渡。opacity 过渡已由外层 group.AnimateTo 提供，
	// 这里用 AnimateColor 给背景色加同样的 0.2s 过渡。
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 200 * time.Millisecond, Ease: ui.CubicBezier(0.25, 0.5, 0.25, 1)})).
		Border(1, border)
	b.Children(func() {
		ui.Icon(c, ic).Size(14, 14).TextColor(ink)
	})
	if b.Clicked() {
		fn()
	}
}

// discSpinPeriod 是唱片一圈的周期（原版 #video-cover.record-disc 的 22s）。
const discSpinPeriod = 22 * time.Second

// discSpinStep 是每次视图重建时往前推的旋转过渡长度。播放时进度上报每
// 500ms 会重建一次视图，这里取 1s 留余量（目标始终按挂钟算，早推不会跳）。
const discSpinStep = time.Second

// discSeekThreshold 是判定「这是 seek 而不是正常播放」的最小异常位移（秒）：
// 播放位置相对挂钟该到的位置偏出这么多，才当成跳转。
const discSeekThreshold = 0.6

// discSeekMax 是一次 seek 最多补偿的角度（度）：快进正转、快退倒转，都
// 封顶，免得大跨度跳转把唱片甩好几圈。
const discSeekMax = 180

// discAdvance 返回这一帧唱片角度该推进的增量（度）。
//
// dt 是挂钟经过的秒数，posDelta 是这一帧播放位置的变化（秒），speed 是
// 播放倍速。正常播放按挂钟匀速推进；位置相对挂钟出现异常位移（seek）时，
// 按方向再补一笔：快进为正（往前冲一点）、快退为负（往回转），并封顶。
func discAdvance(dt, posDelta, speed float64) float32 {
	if dt <= 0 {
		return 0
	}
	deg := dt / discSpinPeriod.Seconds() * 360
	excess := posDelta - dt*speed
	if excess > discSeekThreshold || excess < -discSeekThreshold {
		bump := excess / discSpinPeriod.Seconds() * 360
		bump = max(-discSeekMax, min(discSeekMax, bump))
		deg += bump
	}
	return float32(deg)
}

// advanceDisc 把唱片角度按经过的时间推进；暂停 / 缓冲时不推，停在原地。
//
// 角度不取模（一直累加）：取模会让帧间目标突然倒退一整圈——看起来就是
// 「唱片在倒转」，而且每一圈都会发生一次。
func (a *App) advanceDisc(now time.Time) {
	if a.discLast.IsZero() {
		a.discLast, a.discLastPos = now, a.Pos
		return
	}
	if dt := now.Sub(a.discLast); dt > 0 && a.Playing && !a.Buffering {
		a.discAngle += discAdvance(dt.Seconds(), a.Pos-a.discLastPos, a.Speed)
	}
	a.discLast, a.discLastPos = now, a.Pos
}

// resetDisc 停转唱片（桌面要求减少动态效果）：角度归零，并把基准时间 / 位置
// 刷新到当前，免得之后恢复时把停转期间的挂钟差当成一次 seek 补一大笔角度。
// 对应原版 `.record-disc { animation: none }` 把旋转整个撤掉（回到 0°）。
func (a *App) resetDisc(now time.Time) {
	a.discAngle = 0
	a.discLast, a.discLastPos = now, a.Pos
}

// discTargetAngle 给出唱片下一帧该转到的角度：减少动态效果时恒为 0（不转、
// 回正），否则播放中按 discSpinStep 往前推一格（AnimatePose 的线性过渡覆盖
// 这一段，中间帧由框架插值），暂停 / 缓冲时停在当前角。拆成 var 便于测试观察
// 接线（减少动态效果偏好有没有传到封面上）。
var discTargetAngle = func(reduce, playing bool, angle float32) float32 {
	if reduce {
		return 0
	}
	if playing {
		return angle + float32(discSpinStep)/float32(discSpinPeriod)*360
	}
	return angle
}

// infoDescMinH 是简介块的最小高度：原版 `#video-desc p` 的 `min-height: 46px`
// （13px × 1.55 ≈ 20.15/行 ⇒ 约 2.28 行）。简介不足两行时（详情还没拉到时
// 固定文案「无描述」），这一块也要占满 46px，否则只有 ~20px（一行），下方的
// 选集胶囊 / 操作行 / 工具条会跟着上移。声明成 var 便于测试归零做反证。
var infoDescMinH float32 = 46

// videoInfoHeight 是宽窗右栏的固定盒高：原版 `#video-info { height: 366px }`。
//
// 固定盒高 + 内容从盒顶排（`.video-meta-area { align-content: start }`）是原版
// 右栏不随内容上下漂的关键：整块由外层 `.home-now-playing` 的
// `align-items: center` 居中，盒高恒定时居中位置也就恒定。此前原生版高度随
// 内容走、又被外层居中，简介行数一变整块就漂（漂幅还只有内容差的一半）。
//
// 注：原版 366 是按「计数绝对定位」算的（操作行 14+34+14）。原生版此前把计数
// 画成按钮下方的等高列（+13px），三行简介时内容约 369、比盒高多 3px，直接渲染
// 在盒外（原生版不裁剪）；现计数已改回按钮的绝对定位子元素（见 statButton），
// 内容回到 366 以内，固定盒高完全装得下。声明成 var 便于测试归零做反证。
var videoInfoHeight float32 = 366

// videoInfoPad 是宽窗右栏的内边距，对应原版 `#video-info` 的
// `padding: 2px 0 2px 3px`（上 / 右 / 下 / 左，border-box 下由 366 的固定盒高吸收）。
// 上边距 2px 是「UP 主」整块离盒顶的距离；左边距 3px 给头像的聚焦环 / hover 缩放
// 留空间（原版注释）。窄窗原版 `#video-info { height: auto; padding: 0 }`，所以
// 只在宽窗应用。声明成 var 便于测试归零做反证。
var videoInfoPad = [4]float32{2, 0, 2, 3}

// videoTitleBoxH 是标题盒高：原版 `#video-title h2` 的 `min-height: 52px`
// （`line-height: 1.18` 两行 2 × 22 × 1.18 ≈ 51.92，正好落在 52 上），宽窗下
// 1 行与 2 行都是 52。
//
// 必须用**固定高度**而不是 `MinHeight(52).MaxHeight(58)`：框架把每行高夹在
// `max(指定行高, 字体 ascent+descent)` 之上（22px 中文的自然行盒 ≈ 33px = 1.5em，
// LineHeight(1.18) 的 25.96 过不了这道夹），两行内容 ≈ 66；旧写法只用
// `MaxHeight(58)` 把它截在 58 —— 比原版高 6px，把下方简介 / 胶囊 / 操作行 /
// 工具条整体压下 6px。固定 52 后框架不再按内容撑高，超出的部分走元素的
// 「overflow 可见」（原版 CSS 同样是 overflow: visible、行盒比字形矮，靠行与行
// 重叠吸收）—— 布局高度因此与原版一致，只是第二行比原版低 7px（框架硬底）。
// 声明成 var 便于测试做反证（见 videometrics_test）。
var videoTitleBoxH float32 = 52

// 标题的排版参数（原版 globals.css `#video-title h2`）：22px / 800 /
// line-height 1.18。infoTitlePitch 是 1.18 × 22 ≈ 25.96px 的**像素**行距，
// 交给 multilinetext.go 的 paintTextLines 逐行落点（框架的自然行盒 33 压不下去）。
const (
	infoTitleSize       = 22
	infoTitleWeight     = 800
	infoTitleLineHeight = 1.18
)

var (
	infoTitleFont = fontOf(infoTitleSize, infoTitleWeight)
	// infoTitlePitch 由 textLineBox 算出（= 1.18 × 22），一并声明成 var 便于
	// 测试换桩做反证。
	infoTitlePitch = func() float32 { h, _ := cjkTextLineBox(infoTitleSize, infoTitleLineHeight); return h }()
)

// videoInfo 是右栏（原版 #video-info，宽窗固定 366 高）：
// UP 主行 / 标题 / 简介 / 选集胶囊 / 操作行 / 工具条，自上而下。
//
// wide 为 false 时是窄窗单列（原版 @media ≤680px）：不再 Grow 充满高度，
// 标题的 min-height 也从 52 放开成 auto（由内容决定），整块跟随外层滚动。
//
// titleW 只在窄窗用得上：标题盒高随折行数变化（1 行 25.96 / 2 行 51.92），
// 而框架的布局期拿不到可用宽（`Draw` 里才有 `r.W`），所以由调用方按
// `windowWidth − 两侧内边距` 预算出标题列宽传进来定盒高；宽窗盒高恒为 52，
// 传 0 即可。
func (a *App) videoInfo(c *ui.Context, wide bool, titleW float32) {
	t := a.Theme
	// 空态：连一条待播内容都没有（首次启动 / 播放列表为空）。
	// 与「有曲目、详情还在路上」分开——后者标题、分集、操作行都该照常画。
	idle := a.idle()
	col := ui.Column(c).FillWidth().AlignItems(ui.Start)
	if wide {
		// Grow(1) 在 Row 里吃的是**横向**剩余宽度（原版 1fr 列）；
		// Height(videoInfoHeight) 固定**纵向**盒高，两者互不冲突。
		col.Grow(1).MinWidth(0)
		// 空态不锁 366 的盒高：没有 UP 主 / 简介 / 操作行可排，固定盒高会
		// 把三行字顶在盒顶，和旁边居中的封面错开一大截。
		if !idle {
			col.Height(videoInfoHeight)
		}
		col.Padding(videoInfoPad[0], videoInfoPad[1], videoInfoPad[2], videoInfoPad[3])
	}
	col.Children(func() {
		if idle {
			a.emptyInfo(c)
			a.contextDock(c)
			return
		}
		a.videoInfoHead(c)
		// 原版 #video-title h2：22px / 800 / line-height 1.18 / 2 行截断 /
		// min-height 52、max-height 58。#video-title 与 h2 都是 margin: 0（此前
		// 多加了 8px 上边距，会把 366 的固定盒高顶穿）。盒高固定 52（要么自绘
		// 盒高，要么 ui.Text + MaxHeight），否则固定盒高里吃不完的剩余空间会被
		// 标题吸收（长标题时盒子涨到 66，把下方的简介 / 胶囊 / 操作行整体顶下去）。
		if wide {
			// 宽窗标题自绘逐行落点：框架把每行行盒夹在自然行盒（22px 中文实测
			// 33 = 1.5em）之上，`LineHeight(1.18)` 的 25.96 过不了那道夹，两行
			// 的**行距**会被顶到 33 —— 第二行比原版低 7px（盒高 52 已对齐）。
			// 这里按原版行距 1.18 × 22 逐行落点绘制（断行仍交给框架，见
			// multilinetext.go），单行时字形也随负 half-leading 上移 3.5px。
			title := ui.Box(c).FillWidth().Height(videoTitleBoxH).Label(a.infoTitle())
			title.Draw(func(p *ui.Painter, r ui.Rect) {
				lines := wrapTextLines(a.infoTitle(), infoTitleFont, r.W, 2)
				paintTextLines(p, r, lines, infoTitleFont, infoTitlePitch, t.Ink)
			})
		} else {
			// 窄窗原版 @media (max-width: 680px) 只把 `#video-title h2` 的
			// `min-height` 放开成 auto，`max-height: 58px` 与 line-height 1.18
			// 仍在 —— 两行内容 51.92 < 52，单行则是 25.96（不拉满 52）。
			//
			// 框架的自然行盒是 33/行，`ui.Text` 无论单行（33 vs 25.96）还是两行
			// （66 截到 52）都对不上，所以与宽窗一样自绘逐行落点：用预算出的标题
			// 列宽 titleW 在**布局期**折出折行数定盒高（lines × infoTitlePitch），
			// `Draw` 里再用实测 `r.W` 重折一遍绘制 —— 绘制按真实盒宽，高度按
			// 预算宽，两者只在宽度跨过折行临界点时才可能差一行（届时像素上表现
			// 为末行略出走形，远优于固定差 7px/行的行距偏离）。
			h := float32(len(wrapTextLines(a.infoTitle(), infoTitleFont, titleW, 2))) * infoTitlePitch
			title := ui.Box(c).FillWidth().Height(h).Label(a.infoTitle())
			title.Draw(func(p *ui.Painter, r ui.Rect) {
				lines := wrapTextLines(a.infoTitle(), infoTitleFont, r.W, 2)
				paintTextLines(p, r, lines, infoTitleFont, infoTitlePitch, t.Ink)
			})
		}
		// 原版 #video-desc（后者覆盖前者）：13px / var(--studio-muted) /
		// margin-top 14 / max-height 60 = 3 行（line-height 1.55）；内部 p 还有
		// min-height 46（≈ 2.28 行）—— 简介不足两行时这一块也要占满 46px，
		// 否则只有 ~20px（一行），下方的选集胶囊 / 操作行 / 工具条会整体上移。
		// Shrink(0)：原版 p 是 `max-height: 60px; overflow: hidden`，不因固定盒高
		// 被压短（计数改回绝对定位后内容已回到 366 以内，但锁住收缩更稳，
		// 长标题等极端内容下简介仍是完整 3 行）。
		ui.Text(c, a.infoDesc()).FontSize(13).TextColor(ui.Hex("#475569")).
			MaxLines(3).MinHeight(infoDescMinH).Margin(14, 0, 0, 0).Shrink(0).Selectable()
		a.partPill(c)
		a.contentActions(c)
		a.contextDock(c)
	})
}

// emptyInfo 是「还没有在听的内容」时的右栏。
//
// 原版（以及此前的原生版）在这个状态下把占位当成内容来画：
// 「神秘的UP主」「暂无播放内容」「无描述」「当前选集 无选集标题」，
// 再加一排全部禁用的计数与操作键。首次启动看到的是「有内容但都坏了」，
// 而真正该给的只有一件事：下一步做什么。
//
// 所以空态只保留一句标题 + 一句带具体动作的说明，工具条（搜索结果 /
// 选集 / 合集 / 播放列表）照旧画在下方 —— 播放列表往往已经有歌，
// 那是空态里唯一真正能点的入口。
func (a *App) emptyInfo(c *ui.Context) {
	t := a.Theme
	ui.Text(c, "还没有正在听的内容").FontSize(infoTitleSize).
		FontWeight(infoTitleWeight).TextColor(t.Ink).SingleLine().Shrink(0)
	// 引导按手头的内容取：播放列表里有歌时把「从播放列表继续」也算进去，
	// 那是最省事的一条路（工具条就在下面）。
	hint := "搜索关键词，或把 B 站视频链接粘贴到上面的搜索框。"
	if len(a.Playlist) > 0 {
		hint = "搜索关键词、粘贴 B 站链接，或从下方播放列表继续。"
	}
	ui.Text(c, hint).FontSize(13).TextColor(ui.Hex("#475569")).MaxLines(2).
		Margin(10, 0, 0, 0).Shrink(0)
}

// videoInfoHead 是 UP 主行（.video-info-head）：44px 圆头像（带磨砂玻璃罩）+
// 「UP 主」+ 名字。点头像或名字进 UP 主的空间（旧版的 onOwnerClick）。
func (a *App) videoInfoHead(c *ui.Context) {
	t := a.Theme
	ui.Row(c).Gap(14).AlignItems(ui.Center).Margin(0, 0, 18, 0).Children(func() {
		// .video-owner-wrap：头像 + 名字列，间距 11（原版 gap）。玻璃罩锚在这一层
		// 的左上角，所以头像与名字必须同处一个 wrap。
		wrap := ui.Row(c).Gap(ownerWrapGap).AlignItems(ui.Center).MinWidth(0)
		wrap.Children(func() {
			// .video-owner-wrap::before：48×48 磨砂玻璃罩，常态 opacity 1、
			// 悬停 .7（原版）。盖在头像上但不吃指针。
			ownerGlassOverlay(c, wrap.Hovered())

			// #video-owner-face：44×44 + 2px margin（外框 48）、1.5px 白描边、
			// 蓝环 + 墨影；悬停换蓝影并 scale(1.06)。
			face := ui.ButtonBase(c.Key("owner-face")).
				Size(ownerFaceSize, ownerFaceSize).Margin(ownerFaceMargin).
				Radius(RadiusPill).Clip().
				Background(t.CoverPlaceholder()).Border(1.5, ownerFaceBorder()).
				Label("UP 主空间")
			paint := ownerFacePaintFor(face.Hovered())
			// 原版 `#video-owner-face:focus-visible`：`outline: none`，改用
			// `box-shadow: 0 0 0 2px var(--studio-blue), 0 4px 12px 墨 .1`
			// （注释里说是因为 #video-info overflow:hidden 会把 outline 裁掉，
			// 所以换成同样会画在外侧、但贴合圆形的 box-shadow）。这条
			// :focus-visible 排在 :hover 之后、特异度相同，所以聚焦时
			// **整组替换**悬停那两道影。
			if face.FocusVisible() {
				face.FocusRing(false)
				paint.Shadows = ownerFaceFocusShadows()
			}
			applyShadows(face, paint.Shadows)
			// 原版 #video-owner-face 的 `transition: box-shadow 0.2s ease,
			// transform 0.18s ease`：悬停 scale(1.06) 走 180ms 曲线。之前用
			// ui.Snappy（500ms 弹簧），比原版慢近三倍，悬停放大/回落显得黏。
			face.AnimatePose(ui.Pose{Scale: paint.Scale}, faceHoverAnim)
			face.Children(func() {
				// UP 头像与登录头像同理：主区右栏一直可见，别被抽屉封面挤走。
				a.Images.Retain(a.ownerFace(), imagecache.WidthAvatar)
				if bmp := a.Images.Bitmap(a.ownerFace(), imagecache.WidthAvatar); bmp != nil {
					ui.Image(c, bmp).Fill().Fit(ui.Cover)
				} else if placeholderCover != nil {
					// 默认头像 / 真头像还没解码好：先铺占位图（原版 RetryImg 的
					// onError 退 /cover.png 那一层），解码完再被真图盖掉。
					ui.Image(c, placeholderCover).Fill().Fit(ui.Cover)
				}
			})
			if face.Clicked() {
				a.openUp()
			}

			ui.Column(c).MinWidth(0).Children(func() {
				// 原版 .video-kicker：10px / 700 / line-height 1 / margin-bottom 1px。
				// 框架自然行盒（10px ≈ 15）下不去，Kicker 会高 5px 并把名字挤低；
				// 用显式盒高 + 字形上移复现（见 linebox.go）。
				//
				kh, ksh := textLineBox(10, 1)
				ui.Text(c, "UP 主").FontSize(10).Bold().TextColor(kickerInk).
					Height(kh).Margin(0, 0, 1, 0).Padding(ksh, 0, 0, 0)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					name := ui.ButtonBase(c.Key("owner-name")).Padding(0).Label("UP 主空间")
					name.Background(ui.Transparent)
					// 原版 `#video-owner-name:focus-visible`：`outline: 2px solid
					// var(--studio-blue); outline-offset: 2px`。
					focusOutline(name, 4, focusStudio)
					name.Children(func() {
						// 原版 #video-owner-name：15px / 650 / line-height 1.25。
						// 框架自然行盒（15px ≈ 22.5）下不去，名字会高约 4px；用显式
						// 盒高 + 字形上移复现（见 linebox.go）。
						nh, nsh := textLineBox(15, 1.25)
						ui.Text(c, a.ownerName()).FontSize(15).FontWeight(650).
							TextColor(pick(name.Hovered(), ui.Hex("#0284c7"), ui.Hex("#334155"))).
							SingleLine().MaxWidth(240).Height(nh).Padding(nsh, 0, 0, 0)
					})
					if name.Clicked() {
						a.openUp()
					}
					// 合作视频：UP 主名字旁的选择键（原版只有 staff 非空、
					// creators.length > 1 时才画）。
					if a.Info != nil && len(a.Info.Staff) > 1 {
						a.collabPicker(c)
					}
				})
			})
		})
	})
}

// kickerInk 是「UP 主」那行小标题的墨色（原版 .video-kicker 的 #8a95a6）。
// 在线人数接在同一行，共用这个颜色，读起来才是一行注脚而不是两块东西。
var kickerInk = ui.Hex("#8a95a6")

// onlineBadgeText 是「N 人在看」的文案；没有可展示的数据时为空串。
//
// 文案用「N 人在看」：与 B 站网页播放器那个气泡同一个说法，用户一眼就能
// 对上「这个数是什么」。（中途试过「同时在线 N 人」这种更笼统的说法，用户
// 2026-10-29 决定还是回到「在看」—— 与 B 站口径一致，不需要额外解释。）
//
// 归属检查落在界面上（而不只靠调用方清状态）：万一哪条换集路径漏了清的
// 那一步，也不会把上一集的在线人数挂到新视频上。
func (a *App) onlineBadgeText() string {
	if a.OnlineCount <= 0 || a.OnlineCid == 0 {
		return ""
	}
	if a.Track != nil && a.Track.Cid != 0 && a.Track.Cid != a.OnlineCid {
		return ""
	}
	if a.Info != nil && a.Info.Cid != 0 && a.Info.Cid != a.OnlineCid {
		return ""
	}
	return onlineCountText(a.OnlineCount) + " 人在看"
}

// onlineViewersBadge 画「N 人在看」，接在**选集胶囊**里、分集标题之后
// （x/player/online/total 的 count，由 app/viewers.go 每 30s 轮询写进
// a.OnlineCount / a.OnlineCid）。与 View（累计播放量）是两个不同的数。
//
// 位置（用户第四轮反馈「放在 UP 主名字下有点奇怪，它应该属于作品的播放信息」，
// 第五轮又说操作行行尾「一样很丑，甚至比之前还丑」）：落在 `.video-part-pill`
// 里面 —— 那颗胶囊本来写的就是「这一集在播什么」（来源 + 分集标题），在线人数
// 是同一件事的另一面，接在后面读作「● 当前选集 P1 · 152 人在看」最自然：
//
//   - 不新增行：右栏是固定 366 高的盒（videoInfoHeight），多一行会把下方整体
//     顶下去；
//   - 不是「贴边」：胶囊本身是一条有边界的信息条，文字接在它**内部**、随内容
//     自然排布，不像行尾对齐那样孤零零地飘在右缘；
//   - 与胶囊同档：13px 正文大小（不是更小的 12px），墨色 --studio-faint
//     （#64748b，比胶囊标题的 #475569 轻一档，但比胶囊里的小标签深）。
//
// 前面用一个细竖线（"·" 与胶囊 gap 8 的间距）与分集标题分开，读作同一行的
// 两个字段，而不是两段话。
//
// 之前试过都不行：① 胶囊底 + 白描边 + 玫瑰红圆点 + 600 字重 —— 366 宽的右栏里
// 过于扎眼；② 挂在 UP 主行行尾 —— 离名字 238px、垂直中心对的是 44px 头像而
// 不是名字基线（高 5px），像浮在那儿；③ 接在「UP 主」kicker 后面 —— 语义不对
// （那是 UP 主模块）；④ 操作行行尾 —— 与那一排图标键同一行但远隔 100px，
// 读不成一组。
//
// 没数据就整块不画：接口没给数字 / B 站把 show_switch.count 关掉 / 换集后这一
// 轮响应还没回来，都算「不知道」；写个「0 人在看」是误导。
//
// 两种形态：胶囊在时（见 partPillShown）进胶囊、跟分集标题同一行；胶囊被省略
// 时（单 P 视频没有分集标题）退到操作行行尾，这个数不能因此消失。
func (a *App) onlineViewersBadge(c *ui.Context) {
	text := a.onlineBadgeText()
	if text == "" {
		return
	}
	if a.partPillShown() {
		// 胶囊内：分隔点 + 文字。胶囊的 gap 已经给了 8px，不再加内边距。
		// 墨色比胶囊标题（#475569）淡一档，避免它比两侧文字还响。
		// 行盒与胶囊里其它文字共用（pillText），否则「P1」这种拉丁标题的行盒
		// 比中文矮 3px，两个盒顶会差 1.5px。
		pillText(c, "·", a.Theme.Faint, 400)
		pillText(c, text, a.Theme.Faint, 400).SingleLine().Tooltip(onlineBadgeTip)
		return
	}
	// 退路（操作行行尾）：一个 Grow(1) 空盒子吃剩余宽度（mygo 没有
	// margin-left: auto），左端那排键的落点因此不变。
	ui.Box(c).Grow(1).Height(0).Shrink(1)
	ui.Text(c, text).FontSize(12).TextColor(a.Theme.Faint).SingleLine().
		Shrink(0).Height(34).LineHeight(34 / 12).Tooltip(onlineBadgeTip)
}

// onlineBadgeTip 是这一块的小提示（主窗与退路共用）。
const onlineBadgeTip = "此刻正在看这一集的人数（来自 B 站当前在线），每 30 秒刷新"

// collabOwner 返回用户在合作视频里选中的参与 UP 主。没选过、选的不在当前
// 视频里（换了视频）都不算，退回主 UP —— 对应原版 videoInfo 的 displayedOwner
// （`creators.find(c => c.mid === selectedOwner?.mid) || creators[0]`）。
func (a *App) collabOwner() (Collaborator, bool) {
	if a.Info == nil || a.CollabMid == 0 || a.CollabFor != a.Info.Bvid {
		return Collaborator{}, false
	}
	for _, st := range a.Info.Staff {
		if st.Mid == a.CollabMid {
			return st, true
		}
	}
	return Collaborator{}, false
}

// openUp 打开当前面板显示的那个 UP 主的空间（合作视频里选过就是选中的那位）。
// 没有 mid 就提示一下。
func (a *App) openUp() {
	var mid int64
	name := a.ownerName()
	if st, ok := a.collabOwner(); ok {
		mid = st.Mid
		name = st.Name
	} else if a.Info != nil {
		mid = a.Info.OwnerMid
	}
	if mid == 0 || a.Act.OpenUp == nil {
		a.NotifyType("warning", "该视频没有可用的 UP 主空间")
		return
	}
	a.Act.OpenUp(mid, name)
}

// partDotSize 是 .video-part-pill 里小圆点的边长（原版 7px）。
const partDotSize float32 = 7

// partTitleInk 是 .video-part-title 的文本色（原版 #475569，比胶囊本体的
// #0369a1 浅一档）。
var partTitleInk = ui.Hex("#475569")

// partPill 是选集胶囊（.video-part-pill）：小圆点 + 来源 + 分集标题。
// 从播放列表播入时前缀显示「播放列表」（原版 isPlaylistMode）。
//
// 返回值报告胶囊这一帧是否真的画出来了 —— 在线人数（onlineViewersBadge）
// 挂在胶囊里，胶囊被省略时（单 P 视频没有分集标题、也不是播放列表模式）
// 调用方要把它退到操作行行尾，否则这个数就没地方显示了。
func (a *App) partPill(c *ui.Context) {
	t := a.Theme
	blue := ui.Hex("#0369a1")
	// 播放列表模式的圆点是玫红（原版 .video-part-dot-playlist 用
	// --studio-rose），非播放列表才是蓝。
	inPlaylist := a.PlayingPlaylist != ""
	// 没有分集标题、也不是从播放列表播的：胶囊只能写出「当前选集」四个字，
	// 不携带任何信息（单 P 视频就是这样）。整块省略，把空间还给工具条。
	// 播放列表模式仍保留胶囊：来源本身就是有效信息。
	part := a.partTitle()
	if !a.partPillShown() {
		return
	}
	dotInk := pick(inPlaylist, t.Rose, ui.Hex("#0284c7"))
	source := "当前选集"
	if inPlaylist {
		source = "播放列表"
	}
	// 原版 .video-part-pill：min-height 36、margin-top 12、padding 5 11、
	// 薄玻璃底 rgba(255,255,255,.15) + 1px 白边（.38）、外侧 0 2px 10px
	// rgba(15,23,42,.035)、内侧顶部一线高光。
	// 半透明填充（原版 .video-part-pill 没有 backdrop-filter）：薄白底 +
	// 1px 白边 + 顶部一线内高光；::before 的 --glass-sheen-static 光泽与
	// ::after 的 --liquid-rim 聚光描边见 partPillGlass。
	pill := ui.Row(c).MaxWidth(430).MinHeight(36).Margin(12, 0, 0, 0).Padding(5, 11).
		Radius(RadiusPill).
		Background(ui.Hex("#ffffff").Alpha(0.15)).
		Border(1, ui.Hex("#ffffff").Alpha(0.38)).
		InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.50)).
		Shadow(0, 2, 10, 0, ui.Hex("#0f172a").Alpha(0.035)).
		Gap(8).AlignItems(ui.Center)
	pill = partPillGlass(pill)
	pill.Children(func() {
		// .video-part-dot：7px（不是 6）——比原版小 1px 时圆点在胶囊里显得发飘。
		ui.Box(c).Size(partDotSize, partDotSize).Shrink(0).Radius(RadiusPill).Background(dotInk)
		// 三段的字号/行盒都相同（见 pillText）：中文与拉丁的行盒高度不同，
		// 不给共用盒高的话三段的盒顶会各差 1.5px，读起来像没对齐。
		//
		// .video-part-source 继承胶囊的 #0369a1 / 700。
		pillText(c, source, blue, 700)
		// .video-part-title 单独覆写成 #475569 / 600（原版只有来源是蓝的，
		// 分集标题是中性深灰），之前这里也染成蓝色/700，读起来比原版抢眼。
		if part != "" {
			pillText(c, part, partTitleInk, 600).SingleLine()
		}
		// 在线人数接在胶囊里、分集标题之后（见 onlineViewersBadge）：
		// 「● 当前选集 P1 · 152 人在看」—— 整条读作「这一集在播什么、
		// 有多少人同时在听」，是同一件事的两面，不需要额外一行也不破坏
		// 右栏 366 的固定盒高。
		a.onlineViewersBadge(c)
	})
}

// partPillFontSize 是选集胶囊里所有文字的字号（原版 .video-part-pill 13px）。
const partPillFontSize = float32(13)

// partPillLineBox 是胶囊里文字共用的行盒高度。
//
// 胶囊里混着中文（「当前选集」「152 人在看」）和拉丁（分集标题「P1」）：主题
// 字体栈里 Inter 的拉丁行盒比中文回退字体矮（13px：17 vs 20），各自 auto 高度
// 的话三段盒顶会差半个行盒差（1.5px），在同一行里看着就是「在线人数被抬高了」。
// 统一取两者较大的那个当共用盒高，每段文字再按自己的行盒补半个差值。
var partPillLineBox = max32(naturalLineBox(partPillFontSize), cjkLineBox(partPillFontSize))

// pillText 画胶囊里的一段文字：字号 / 字重 / 颜色固定，行盒统一成
// partPillLineBox，并按这段文字自己的自然行盒补上居中位移。
func pillText(c *ui.Context, s string, ink ui.Color, weight int) ui.Element {
	e := ui.Text(c, s).FontSize(partPillFontSize).FontWeight(weight).TextColor(ink).
		Shrink(0).Height(partPillLineBox)
	if d := (partPillLineBox - textLineBoxFor(s, partPillFontSize)) / 2; d != 0 {
		// 负内边距把字形上移（同 linebox.go 的约定：不用 Translate，避免
		// 圆角 clip 升级成预烘焙遮罩）。
		e = e.Padding(d, 0, 0, 0)
	}
	return e
}

// partPillShown 报告选集胶囊这一帧是否会出现（纯函数，渲染期不写状态）。
// 在线人数优先挂在胶囊里（见 onlineViewersBadge），胶囊被省略时改挂到操作行
// 行尾 —— 单 P 视频没有分集标题，胶囊整块省略，但这个数不能因此消失。
func (a *App) partPillShown() bool {
	return a.partTitle() != "" || a.PlayingPlaylist != ""
}

// partPillGlass 画 `.video-part-pill` 的两层液态玻璃（原版 globals.css）：
//
//	::before  inset:0 + border-radius:inherit，铺 --glass-sheen-static（低透明度
//	          斜向洗光：径向 140%×80% @22% -5% 白 .24 + 线性 170deg 白 .07→.015）
//	::after   inset:0 + padding 1px + mask exclude，背景 --liquid-rim
//	          （1px 聚光描边；conic 定向光，最亮点偏左上）
//
// 两层此前都没画：胶囊只有一层平铺的白 .15 底，比原版平一档。光泽复用播放栏那套
// 逐像素烘焙的 --glass-sheen-static 位图（mygo 没有径向渐变接口；sheenGlassBaked），
// 描边复用播放栏的 --liquid-rim 绘制（paintPlayerRim）——胶囊面积小，走真 conic 分支——
// 与 .part-search 的 partSearchGlass 同一套。整个函数声明成 var 便于测试换桩反证。
//
// 视觉近似：位图按胶囊自己的宽高比烘焙（paintBakedSheen → sheenBaked.bitmapFor），
// 径向（按盒的百分比定义）拉伸后与 CSS 完全一致，线性那层的角度也因为烘焙时
// 就带上了盒的宽高比而不再被拉偏。
var partPillGlass = func(b ui.Element) ui.Element {
	return b.Draw(func(p *ui.Painter, r ui.Rect) {
		paintBakedSheen(p, r, RadiusPill, sheenGlassBaked)
		// ::after 没有 opacity 规则 ⇒ 整圈 opacity 1。
		paintPlayerRim(p, r, RadiusPill, 1)
	})
}

// contentActions 是操作行（.video-content-actions）：点赞 / 投币 / 收藏
// 三个带计数的按钮，再加浏览器打开 / 视频 / 弹幕三个纯图标按钮。
func (a *App) contentActions(c *ui.Context) {
	// 原版 videoInfo 里 bvid = displayVideoInfo?.bvid、aid = displayVideoInfo?.aid。
	// 三个互动键的禁用只看**有没有视频**（like/coin 用 bvid、收藏用 aid），
	// 不看计数：计数 0 是完全正常的状态（新视频的投币数就是 0），
	// 之前用「计数文案为空」当禁用判据，会把 0 币的视频的投币键锁死。
	var bvid string
	var aid, cid int64
	if a.Info != nil {
		bvid, aid, cid = a.Info.Bvid, a.Info.Aid, a.Info.Cid
	}
	// 原版 .video-content-actions: margin-top 14、padding 0 0 14px、gap 4。
	// 底部 14px 是给绝对定位的计数留的展示区（计数不占布局高，见 statButton）。
	// FillWidth 只为退路形态（单 P 视频把在线人数顶到行尾）：胶囊在时这一行
	// 没有 Grow 子元素，宽度不影响任何键的落点。
	ui.Row(c).FillWidth().Margin(14, 0, 0, 0).Padding(0, 0, 14, 0).Gap(4).
		AlignItems(ui.Start).Children(func() {
		// 激活色与原版一致：点赞 #e11d48、投币 #ca8a04、收藏 #eab308。
		// 原版这三个是 size={20}，tooltip（title）随激活态变文案。
		a.statButton(c, statSpec{
			Key: "like", Label: pick(a.Liked, "取消点赞", "点赞"),
			Icon: iconLike, Size: 20, Active: a.Liked,
			ActiveInk: ui.Hex("#e11d48"), Count: a.statLike(),
			Off: bvid == "", OnClick: a.Act.Like,
		})
		a.statButton(c, statSpec{
			// 原版投的是一次两币（coin_video 的 multiply: 2）。
			Key: "coin", Label: pick(a.Coined, "已投币", "投币(2个)"),
			Icon: iconCoin, Size: 20, Active: a.Coined,
			ActiveInk: ui.Hex("#ca8a04"), Count: a.statCoin(),
			Off: bvid == "", OnClick: a.Act.Coin,
		})
		a.statButton(c, statSpec{
			Key: "fav", Label: pick(a.Faved, "取消收藏", "收藏"),
			Icon: pick(a.Faved, iconStarFilled, iconStar), Size: 20, Active: a.Faved,
			ActiveInk: ui.Hex("#eab308"), Count: a.statFav(),
			// 原版收藏键有 `aria-pressed={isFavorite}`（点赞 / 投币没有）。
			Toggle: true,
			// 原版收藏键在「收藏状态还在查」时也禁用（isFavoriteUpdating），
			// 否则会用上一条视频的状态误操作。
			Off: aid == 0 || a.FavPending, OnClick: a.Act.Favorite,
		})
		// 原版这三枚图标 JSX 写的是 size={18}（浏览器 / 视频 / 弹幕），但 CSS
		// `.video-content-actions .nav-icon-btn > svg { width/height: 20px }`
		// （globals.css:2556）盖掉了 SVG 的 width/height **表现属性**（表现属性
		// 在 CSS 里是最低优先级），所以 DOM 里的 svg 实际都是 20 —— 原版注释也
		// 写了「统一图标光学尺寸：源码里混着 18/20 两种规格，同排看有微妙的
		// 参差」。cd94979 换 icon-park path 时按 JSX 改成了 18，把这条覆写漏了
		// （同排三枚图标比原版小一圈）。
		a.contentIconButton(c, "browser", "浏览器打开", iconBrowser, iconOpticalSize, bvid == "", func() {
			if a.Act.OpenBrowser != nil {
				a.Act.OpenBrowser()
			}
		}).ContextMenu(func(m *ui.Menu) {
			// 右键还能复制链接（原版分享按钮的等价物）。
			if m.Item("复制链接").Chosen() && a.Act.CopyLink != nil {
				a.Act.CopyLink()
			}
			m.Separator()
			if m.Item("浏览器打开").Chosen() && a.Act.OpenBrowser != nil {
				a.Act.OpenBrowser()
			}
		})
		a.contentIconButton(c, "video", "视频播放", iconVideo, iconOpticalSize, bvid == "", func() {
			if a.VideoOpen {
				if a.Act.CloseVideo != nil {
					a.Act.CloseVideo()
				}
			} else if a.Act.OpenVideo != nil {
				a.Act.OpenVideo()
			}
		})
		// 这里是弹幕 / 评论 / 章节 / 字幕的统一入口，不再是「弹幕/评论」
		// 两件套；图标也换成文字稿图标，避免继续暗示只有评论和弹幕。
		a.contentIconButton(c, "danmaku", "弹幕、评论、章节与字幕", iconText, iconOpticalSize, cid == 0, a.openDanmaku)
		// 「顺着听」是视频详情的一部分，不属于播放列表；放在内容操作行，
		// 和弹幕/视频/浏览器入口同一排，不再藏进播放列表抽屉。
		a.radioContentButton(c, a.Current() == nil)
		// 胶囊没画出来时（单 P 视频），在线人数退到这里（见 onlineViewersBadge）。
		if !a.partPillShown() {
			a.onlineViewersBadge(c)
		}
	})
}

// contextDock 是工具条（.video-context-dock）：
// 搜索结果 / 选集 / 合集 / 播放列表（旧版四个）。
func (a *App) contextDock(c *ui.Context) {
	t := a.Theme
	// 原版 `bvid = displayVideoInfo?.bvid`：没有视频详情时「选集」是禁用态。
	var bvid string
	if a.Info != nil {
		bvid = a.Info.Bvid
	}
	ui.Row(c).Margin(12, 0, 0, 0).Padding(3).Radius(10).Gap(2).AlignItems(ui.Center).
		Background(ui.Hex("#0f172a").Alpha(0.03)).
		Border(1, ui.Hex("#64748b").Alpha(0.14)).
		InsetShadow(0, 1, 2, 0, ui.Hex("#0f172a").Alpha(0.05)).
		InsetShadow(0, -1, 0, 0, ui.Hex("#ffffff").Alpha(0.35)).Children(func() {
		// 搜索结果：没有搜索结果时禁用（原版 disabled={!searchResultsCount}）。
		a.iconButton(c, "dock-search", "搜索结果", iconSearch, iconOpticalSize,
			len(a.ListFor(DrawerSearch).Cards) == 0, func() {
				a.Drawer = DrawerSearch
			})
		// 选集：非播放列表模式且有视频时图标是蓝色（原版 fill 逻辑）。
		// 禁用判据同原版 `disabled={!bvid}`。
		partsInk := ui.Color{}
		if a.PlayingPlaylist == "" && a.Track != nil {
			partsInk = t.Blue
		}
		a.inkButton(c, "dock-parts", "选集", iconList, iconOpticalSize, partsInk, bvid == "", a.openParts)
		// 合集：还没选过合集时禁用。
		a.iconButton(c, "dock-series", "合集", iconSeries, iconOpticalSize, a.SeriesID == 0, a.openSeries)
		// 播放列表：带数量角标（>99 显示 99+），播放中按来源变蓝/紫。
		// 图标染色用 inkButton —— 原版是
		// fill={isPlaylistMode ? (series ? 紫 : 蓝) : 灰}，之前这里算出了 ink
		// 却用 iconButton 丢掉了（`_ = ink`），播放列表在播时图标一直是灰的。
		plistInk := ui.Color{}
		if a.PlayingPlaylist != "" {
			plistInk = t.Blue
			if a.PlayingPlaylist == ListSeries {
				plistInk = ui.Hex("#a855f7") // 合集来源是紫色（原版）
			}
		}
		// 角标是**这颗按钮的子元素**（原版：`<span class="nav-badge">` 就写在
		// `<button class="nav-icon-btn">` 里，`.nav-icon-btn .nav-badge` 把它压到
		// 按钮本体的右上角 top/right:-4px）。此前把角标建成 dock 行的**兄弟**：
		// Absolute 的定位基准是 dock 行的 padding box（不是按钮），于是角标整块
		// 偏到行右上角——比原版高 4px（含行 padding 3）、右 4px，飘在「合集」与
		// 「播放列表」键之间的上沿（用户可感知）。挂到按钮上才与原版几何一致。
		plist := a.inkButton(c, "dock-playlist", "播放列表", iconMusicList, iconOpticalSize,
			plistInk, false, func() {
				if a.Drawer == DrawerPlaylist {
					a.Drawer = ""
				} else {
					a.Drawer = DrawerPlaylist
				}
			})
		if n := a.playlistBadgeCount(); n > 0 {
			plist.Children(func() { a.playlistBadge(c) })
		}
	})
}

// playlistBadge 是播放列表键右上角的数量角标（原版 `.nav-badge` +
// `.nav-icon-btn .nav-badge`）：
//
//	.nav-badge                  height:14px; min-width:14px; padding:0 3px;
//	                            line-height:1; border-radius:7; 9px bold 白字；
//	                            底 rgba(225,29,72,.72)（合集来源换成 #a855f7）；
//	                            1px 白边 + 上白 .5 / 下墨 .08 内影 + 玫红投影。
//	.nav-icon-btn .nav-badge    top:-4px; right:-4px（压到按钮本体右上角）。
//
// 它是**按钮的子元素**（见 contextDock 的注释），Absolute 的基准就是按钮本体；
// PassThrough 让它只显示，不吃按钮的指针事件。
func (a *App) playlistBadge(c *ui.Context) {
	badge := ui.Text(c, a.playlistBadgeText()).FontSize(9).Bold().
		TextColor(ui.Hex("#ffffff")).
		Absolute().Right(-4).Top(-4).Height(14).MinWidth(14).Padding(0, 3).Radius(RadiusPill).
		Background(a.playlistBadgeInk()).
		Border(1, ui.Hex("#ffffff").Alpha(0.55)).
		InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.50)).
		InsetShadow(0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.08)).
		Shadow(0, 2, 6, 0, a.Theme.Rose.Alpha(0.20))
	badge.Label("播放列表数量")
	badge.PassThrough()
}

// playlistBadgeCount 是播放列表按钮角标要显示的条数（当前播放的来源）。
func (a *App) playlistBadgeCount() int {
	if a.PlayingPlaylist == ListSeries {
		return len(a.SeriesPlaylist)
	}
	return len(a.Playlist)
}

// playlistBadgeText 把条数变成角标文案（>99 显示 99+）。
func (a *App) playlistBadgeText() string {
	n := a.playlistBadgeCount()
	if n > 99 {
		return "99+"
	}
	return fmt.Sprint(n)
}

// playlistBadgeInk 是角标底色：只有「正在放合集」时是紫色，其余（含不在播放
// 列表模式）都是原版 .nav-badge 的玫红 rgba(225,29,72,.72)，不是蓝色。
func (a *App) playlistBadgeInk() ui.Color {
	if a.PlayingPlaylist == ListSeries {
		return ui.Hex("#a855f7")
	}
	return a.Theme.Rose.Alpha(0.72)
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

// playlistActive 返回当前播放列表抽屉 tab 对应的记录。
func (a *App) playlistActive(tab string) []PlayItem {
	if tab == ListSeries {
		return a.SeriesPlaylist
	}
	return a.Playlist
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

// statSpec 是一个互动按钮（点赞 / 投币 / 收藏）的参数。
//
// 计数是按钮的**绝对定位子元素**（原版 `.nav-icon-btn > .nav-stat-value`：
// absolute、top: calc(100% + 1px)、left 50% + translateX(-50%)），不占布局
// 高度——行高只由按钮 34px + 行容器的 padding-bottom 14（展示区）决定。
//
// statCountBoxH / statCountShift 压回那条规则里的 `line-height: 12px`：
// 框架把 9px 文本的行盒夹在**自然高度 14**（本机 Pango 的 ascent 11 + descent 3，
// 见 linebox.go）之上，计数因此比原版高 2px、数字落点低 1px。计数不占布局，
// 这 2px 只体现在计数自身的盒高（无头实测：矩形高 14 → 12）。用显式盒高 +
// 字形上移复现原版（line.Height 是写死的像素值，直接走 pxLineBox）。
//
// statCountWeight 是同一条规则里的 `font-weight: 600`：计数是按钮下方的小号
// 数值，原版加粗一档才与图标同重量；原生版此前走 numericText 的默认 400，
// 计数比原版轻一档。
var statCountBoxH, statCountShift = pxLineBox(9, 12)

var statCountWeight = 600

type statSpec struct {
	Key string
	// Label 同时作为 tooltip 与无障碍标签（原版按钮的 title）。
	Label string
	Icon  *ui.SVG
	// Size 是图标尺寸（原版这三个是 20）。
	Size float32
	// Active 是已点赞 / 已投币 / 已收藏。
	Active    bool
	ActiveInk ui.Color
	// Count 是按钮下方的计数文案（原版 formatCompactCount，0 也显示）。
	Count string
	// Off 为真时整钮禁用（没有视频 / 收藏状态请求中）。
	Off bool
	// Toggle 为真时这颗键是**保持型开关**，把 Active 作为按压态交给无障碍层
	//（原版只有收藏键写了 `aria-pressed={isFavorite}`；点赞 / 投币没有 aria-pressed，
	// 它们只有 title + disabled）。mygo 对应 RoleToggleButton + Checked(Active)。
	Toggle  bool
	OnClick func()
}

// statButton 是互动按钮（原版 .video-content-actions .nav-icon-btn）：
// 34×34 图标钮（圆角 8），9px 计数挂在按钮正下方；激活时图标按动作变色——
// 点赞红、投币黄、收藏黄。
func (a *App) statButton(c *ui.Context, spec statSpec) {
	b := ui.ButtonBase(c.Key("stat-"+spec.Key)).Size(ContentIcon, ContentIcon).
		Radius(ContentIconR).Center().Label(spec.Label).Tooltip(spec.Label)
	// 原版收藏键是 `aria-pressed={isFavorite}` 的 button —— 无障碍上是 toggle
	// button，靠 RoleToggleButton + Checked 让各平台读出「已按下」；点赞 / 投币
	// 没有 aria-pressed，保持普通按钮。角色必须与 Checked 同一条链。
	if spec.Toggle {
		b.Role(ui.RoleToggleButton).Checked(spec.Active)
	}
	// 内容操作行的基础透明度（.video-content-actions .nav-icon-btn 的 .82），
	// 禁用由 disableNav 覆盖成原版的 .34；两条都走 0.16s 过渡
	// （navIconFade 在 disableNav 之后，末次写入生效）。
	disableNav(b, spec.Off)
	navIconFade(b, spec.Off, navIconRestOpacityFor(navIconContent))
	navIconPose(b)
	// 按钮面：赞/币/藏三键**任何状态都不铺底色 / 不画边框**。
	//
	// 原版 JSX 只把 svg 的 `fill` 在激活时换成动作色（`ThumbsUp fill={isLiked ?
	// "#e11d48" : gray}` 等），globals.css 里**没有** `.nav-stat-btn[data-active]`、
	// `.nav-icon-btn[aria-pressed]` 之类的激活面规则——`.nav-icon-btn.nav-stat-btn`
	// 只是把尺寸定成 32×32，面还是通用 `.nav-icon-btn` 的「常态透明、悬停
	// --glass-bg-hover（白 .38）、按下 --glass-bg-active（白 .16）」。
	//
	// 此前原生版给激活态铺了 `ActiveInk.Alpha(0.26)` 的着色面 + 1px 白边
	//（`glassOn`→`buttonSurface`，dd2cb91 的「玻璃 Tint 时代」遗留）—— 已赞是红底、
	// 已投币是琥珀底、已收藏是黄底加白框，与只有图标变色的原版完全对不上。
	// 本轮去掉：三个键统一走通用 `.nav-icon-btn` 的底色迁移。
	if spec.Off {
		navIconBgTo(b, ui.Transparent)
	} else {
		// 常态透明，悬停 --glass-bg-hover、按下 --glass-bg-active；底色淡入
		//（background-color 0.18s ease），悬停的内高光由下面 ::after 补齐。
		// 激活与否都不改变底色：悬停时即使已赞也是白 .38 的通用底。
		navIconBgTo(b, pressFeedback(b, ui.Transparent, a.Theme.GlassHover, a.Theme.GlassActive))
	}
	// 原版 .nav-icon-btn.nav-stat-btn 仍带常态的左上角径向聚光。
	btnLightNav.draw(b, ContentIconR)
	// 悬停 ::after 内高光；按下时它归零、换成 :active 的内压阴影。
	if b.Pressed() {
		navPressShadow(b)
	} else if b.Hovered() {
		navHoverHighlight(b)
	}
	b.Children(func() {
		ui.Icon(c, spec.Icon).Size(spec.Size, spec.Size).
			TextColor(pick(spec.Active, spec.ActiveInk, ui.Hex("#475569")))
		// 原版这一格始终有值（没有视频时是 "0"），且是 `.nav-icon-btn` 的
		// **绝对定位子元素**：top: calc(100% + 1px)、水平居中、不占布局高。
		// 用 Attach(Bottom, Top) 把计数上缘挂到按钮下缘中点，再 +1px —— 等价
		// 于 CSS 的绝对定位，计数不再撑高按钮所在行（行高由 .video-content-actions
		// 的 padding-bottom 14 预留展示区）。作为按钮子元素，它还随按钮一起
		// 上浮 1px / 按下缩放（原版数值就是按钮子元素）。
		//
		// 计数墨色：原版 `.nav-icon-btn > .nav-stat-value` 恒为 `color: #64748b`，
		// 激活（已赞/已投币/已收藏）**不改**计数色（变色的只有图标）；禁用时
		// `.nav-stat-btn:disabled .nav-stat-value { color: inherit }` 继承按钮
		// 的 `#475569`。此前原生版让激活态计数跟图标一起染动作色（红 / 琥珀 /
		// 黄），与恒灰的原版对不上，本轮改回。
		if spec.Count != "" {
			valueInk := ui.Hex("#64748b")
			if spec.Off {
				valueInk = ui.Hex("#475569")
			}
			numericText(c, spec.Count).FontSize(9).FontWeight(statCountWeight).TextColor(valueInk).
				Opacity(navIconStatValueOpacity(spec.Off)).
				SingleLine().
				Height(statCountBoxH).Translate(0, statCountShift).
				Attach(ui.AnchorBottom, ui.AnchorTop).Top(1)
		}
	})
	if b.Clicked() && spec.OnClick != nil {
		spec.OnClick()
	}
}

// disableNav 把按钮设成原版 `.nav-icon-btn:disabled` 的样子：禁用 +
// opacity .34。mygo 的禁用节点会自己再乘 .5（ui/paint.go），所以元素值要写
// disabledDim(.34)=.68 抵消掉（见 disabled.go 的总说明）。
func disableNav(b ui.Element, off bool) ui.Element {
	b.Disabled(off)
	if off {
		b.Opacity(disabledDim(navIconDisabledOpacity))
	}
	return b
}

// iconButton 是 34×34 的纯图标按钮（.video-context-dock .nav-icon-btn）。
// off 为真时按原版 `.nav-icon-btn:disabled` 置灰（调用方给 disabled 判据）。
func (a *App) iconButton(c *ui.Context, key, label string, ic *ui.SVG, size float32, off bool, fn func()) ui.Element {
	return a.inkButton(c, key, label, ic, size, ui.Color{}, off, fn)
}

// contentIconButton 是内容操作行的图标钮（.video-content-actions .nav-icon-btn）：
// 34×34、圆角 8（比工具条的 7 大）。off 为真时按原版 `.nav-icon-btn:disabled`
// 置灰（浏览器打开 / 视频播放按 `!bvid`、弹幕/评论按 `!cid`）。
func (a *App) contentIconButton(c *ui.Context, key, label string, ic *ui.SVG, size float32, off bool, fn func()) ui.Element {
	t := a.Theme
	b := ui.ButtonBase(c.Key("icon-"+key)).Size(ContentIcon, ContentIcon).Radius(ContentIconR).
		Center().Label(label).Tooltip(label)
	// 原版 `.video-content-actions .nav-icon-btn { opacity: .82 }`：基础透明度与
	// 禁用态（disableNav 的 .6 → mygo 乘 .5 得 .3）都走 0.16s 淡入淡出。
	disableNav(b, off)
	navIconFade(b, off, navIconRestOpacityFor(navIconContent))
	navIconPose(b)
	// 底色淡入（background-color 0.18s ease），不是瞬时切换。
	navIconBgTo(b, pressFeedback(b, ui.Transparent, t.GlassHover, t.GlassActive))
	// 原版 .nav-icon-btn 常态就带一层左上角径向聚光（background-image），
	// 之前原生版只有填充、少了这道光（见 btnlight.go）。
	btnLightNav.draw(b, ContentIconR)
	// 悬停叠 ::after 内高光（原版不是描边）；按下换成 :active 的内压阴影。
	if b.Pressed() {
		navPressShadow(b)
	} else if b.Hovered() {
		navHoverHighlight(b)
	}
	b.Children(func() { ui.Icon(c, ic).Size(size, size).TextColor(ui.Hex("#475569")) })
	if b.Clicked() && fn != nil {
		fn()
	}
	return b
}

// radioContentButton 是主页播放详情里的「顺着听」开关。
// 它不改变播放列表本身，所以放在内容操作行，而不是播放列表表头。
func (a *App) radioContentButton(c *ui.Context, off bool) {
	t := a.Theme
	label := pick(a.RadioOn, "关闭顺着听", "顺着听")
	tip := pick(a.RadioOn, "关闭顺着听", "顺着听：以当前视频为种子，自动续播相关推荐")
	b := ui.ButtonBase(c.Key("icon-radio")).Size(ContentIcon, ContentIcon).
		Radius(ContentIconR).Center().Label(label).Tooltip(tip).
		Role(ui.RoleToggleButton).Checked(a.RadioOn)
	disableNav(b, off)
	navIconFade(b, off, navIconRestOpacityFor(navIconContent))
	navIconPose(b)
	bg := pressFeedback(b, ui.Transparent, t.GlassHover, t.GlassActive)
	if a.RadioOn && !b.Hovered() && !b.Pressed() {
		bg = t.GlassActive
	}
	navIconBgTo(b, bg)
	btnLightNav.draw(b, ContentIconR)
	if b.Pressed() {
		navPressShadow(b)
	} else if b.Hovered() {
		navHoverHighlight(b)
	}
	b.Children(func() {
		ink := ui.Hex("#475569")
		if a.RadioOn {
			ink = ui.Hex("#3b82f6")
		}
		ui.Icon(c, iconSparkles).Size(iconOpticalSize, iconOpticalSize).TextColor(ink)
	})
	if b.Clicked() && a.Act.ToggleRadio != nil {
		a.Act.ToggleRadio()
	}
}

// inkButton 同 iconButton，ink 非零时用它做图标颜色（原版部分按钮是蓝色）。
// off 为真时置灰（`disableNav` 在函数内先做，这样 navIconPose 能正确跳过
// 禁用按钮的上浮）。
func (a *App) inkButton(c *ui.Context, key, label string, ic *ui.SVG, size float32, ink ui.Color, off bool, fn func()) ui.Element {
	t := a.Theme
	textInk := ui.Hex("#475569")
	if ink.A != 0 {
		textInk = ink
	}
	// 原版 .video-context-dock .nav-icon-btn：34×34、圆角 7、无边框、径向高光底。
	b := ui.ButtonBase(c.Key("icon-"+key)).Size(DockIcon, DockIcon).Radius(DockIconR).
		Center().Label(label).Tooltip(label)
	// 原版 `.video-context-dock .nav-icon-btn { opacity: .78 }`（悬停也不回 1，
	// 与内容行同理）。基础透明度与禁用态（disableNav 的 .6）都走 0.16s 过渡。
	disableNav(b, off)
	navIconFade(b, off, navIconRestOpacityFor(navIconDock))
	// 原版 .nav-icon-btn:hover 是玻璃底 + 上浮 1px、:active 再缩 .94；底色
	// 0.18s 淡入，位移见 navIconPose。
	navIconPose(b)
	navIconBgTo(b, pressFeedback(b, ui.Transparent, t.GlassHover, t.GlassActive))
	// 原版 .nav-icon-btn 常态的左上角径向聚光（见 btnlight.go）。
	btnLightNav.draw(b, DockIconR)
	// 悬停叠 ::after 内高光（原版不是描边）；按下换成 :active 的内压阴影。
	if b.Pressed() {
		navPressShadow(b)
	} else if b.Hovered() {
		navHoverHighlight(b)
	}
	b.Children(func() { ui.Icon(c, ic).Size(size, size).TextColor(textInk) })
	if b.Clicked() && fn != nil {
		fn()
	}
	return b
}

// ---------------------------------------------------------------- 文案取值

// idle 报告「连一条待播内容都没有」：既没有详情，也没有当前曲目。
//
// 它只用来区分真空态与「有曲目、详情还在路上」—— 后者只是转圈等数据，
// 标题、分集标题、操作行都得照常占位。
func (a *App) idle() bool { return a.Info == nil && a.Track == nil }

func (a *App) infoTitle() string {
	if a.Info != nil && a.Info.Title != "" {
		return a.Info.Title
	}
	if a.Track != nil && a.Track.Title != "" {
		return a.Track.Title
	}
	return "还没有正在听的内容"
}

func (a *App) infoDesc() string {
	if a.Info != nil && a.Info.Desc != "" {
		return a.Info.Desc
	}
	return "无描述"
}

func (a *App) ownerName() string {
	// 合作视频里选过参与者就显示 TA（原版 displayedOwner）。
	if st, ok := a.collabOwner(); ok && st.Name != "" {
		return st.Name
	}
	if a.Info != nil && a.Info.OwnerName != "" {
		return a.Info.OwnerName
	}
	if a.Track != nil && a.Track.Up != "" {
		return a.Track.Up
	}
	return "神秘的UP主"
}

// ownerFace 返回 UP 主头像地址。没有 face 时退 noFaceAvatar（B 站官方默认
// 头像），对齐原版 videoInfo.tsx:253 的 `displayedOwner?.face || noface.jpg`
// —— 否则头像位置在下载完之前（乃至没有 face 时）只有一层底色。
//
// 合作视频选中了参与者就整组看 TA：原版 `displayedOwner` 一旦有值，
// `face` 为空也是退 noface，不会回落到主 UP 主的 face。
func (a *App) ownerFace() string {
	if st, ok := a.collabOwner(); ok {
		if st.Face != "" {
			return st.Face
		}
		return noFaceAvatar
	}
	if a.Info != nil && a.Info.OwnerFace != "" {
		return a.Info.OwnerFace
	}
	return noFaceAvatar
}

func (a *App) partTitle() string {
	if a.Track != nil && a.Track.Part != "" {
		return a.Track.Part
	}
	// 没有分集标题（单 P 视频 / 详情还没回来）时返回空串，由调用方决定
	// 是否整行省略。这里的占位文案「无选集标题」以前会直接显示在标题下方，
	// 等于告诉用户「这里本该有东西，但我不告诉你是什么」。
	return ""
}

// coverBitmap 返回当前要显示的封面位图。新封面还没抓回来时沿用上一张
// （原版「先预载、后换源」），没旧图时返回 nil。
func (a *App) coverBitmap() *ui.Bitmap {
	if uri := a.coverURL(); uri != "" {
		if b := a.Images.Bitmap(uri, imagecache.WidthPlayer); b != nil {
			a.shownCoverBmp = b
			return b
		}
	}
	return a.shownCoverBmp
}

// coverURL 是圆盘上显示的封面：多 P 时优先当前分集的首帧，
// 再退详情封面，最后退列表卡片封面
// （对应原版的 graftingImage(pageFirstFrame || displayVideoInfo?.pic)）。
func (a *App) coverURL() string {
	if a.Track != nil && a.Track.FirstFrame != "" {
		return a.Track.FirstFrame
	}
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

// statText 是操作行里的计数文案（原版 formatCompactCount(stat?.like ?? 0)）：
// 原版这一格**始终有值** —— 没有视频、没有统计、计数为 0 时都是 "0"。
// 之前把 0 当成空串，后果不只是少画一个数字：投币数 0 的视频投币键会被
// 一起禁用（不能用）。
func statText(info *Info, pick func(*Info) int64) string {
	if info == nil {
		return "0"
	}
	return compactCount(pick(info))
}

// coverOrLogo 画封面，没有封面时退回 logo（原版 cover || "/logo.png"）。
// 兜底用的是白底的 LogoCover（见 logo.go），铺在深色盘面上不会透出黑底。
func (a *App) coverOrLogo(c *ui.Context) {
	if bmp := a.coverBitmap(); bmp != nil {
		ui.Image(c, bmp).Fill().Fit(ui.Cover)
	} else if lg := coverLogo(); lg != nil {
		ui.Image(c, lg).Fill().Fit(ui.Cover)
	}
}

// coverImage 画圆盘封面（或 logo）并返回图片元素，供旋转。
// 调用前先确认 coverBitmap() 或 coverLogo() 非空。
func (a *App) coverImage(c *ui.Context) ui.Element {
	if bmp := a.coverBitmap(); bmp != nil {
		return ui.Image(c, bmp).Fill().Fit(ui.Cover)
	}
	return ui.Image(c, coverLogo()).Fill().Fit(ui.Cover)
}
