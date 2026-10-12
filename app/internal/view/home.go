package view

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/imagecache"
)

// ---------------------------------------------------------------- 搜索药丸

// searchBar 是居中的液态玻璃搜索药丸（原版 .home-searchbar）：
// max-width 520、高 46、圆角 13、内边距 4、子元素间距 8，
// 内容依次是「输入内腔 + 四个内容入口 + 头像」。
func (a *App) searchBar(c *ui.Context) {
	ui.Row(c).FillWidth().Height(SearchBarHeight).Shrink(0).Justify(ui.Center).Children(func() {
		// 原版 .home-searchbar 自己有 backdrop-filter（blur(20px) saturate(1.45)），
		// 是真玻璃，所以这里用 glass.Glass 而不是半透明填充。
		// 原版 .home-searchbar：1px 玻璃边、::before 是 blur(20px)+白 22%、
		// ::after 是 1px 内圈描光（上缘亮、下缘暗），外面两层淡投影；
		// 悬停时描边转亮、投影抬起。
		// 材质是框架玻璃，它自带边缘聚光与描光，所以不再叠 1px 边框/inset 内影
		//（那是原版手搓同一件事的做法，两层描边会显脏）。只保留外层投影。
		pill := ui.Row(c).Width(SearchBarWidth).Height(SearchBarHeight).Radius(SearchRadius).
			Padding(SearchPadding).Gap(8).AlignItems(ui.Center).
			Material(glassLayer(glass.Glass{})).
			Shadow(0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.07)).
			Shadow(0, 1, 4, 0, ui.Hex("#0f172a").Alpha(0.03))
		if pill.Hovered() {
			pill.Shadow(0, 16, 38, 0, ui.Hex("#0f172a").Alpha(0.11)).
				Shadow(0, 1, 4, 0, ui.Hex("#0f172a").Alpha(0.03))
		}
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
		if in.Submitted() {
			a.runSearch()
		}
		// 原版 .search-submit-btn：32×32、圆角 6、默认透明 + opacity .82、
		// 图标 #64748b；悬停 opacity 1、图标转 #0284c7、底白 50%。
		submit := ui.ButtonBase(c.Key("search-submit")).Size(ToolButton, ToolButton).
			Radius(ToolButtonR).Center().Label("搜索").Tooltip("搜索")
		if submit.Hovered() {
			submit.Background(ui.Hex("#ffffff").Alpha(0.50)).Opacity(1)
		} else {
			submit.Background(ui.Transparent).Opacity(0.82)
		}
		searching := a.list().Loading
		submit.Disabled(searching)
		submit.Children(func() {
			if searching {
				// 搜索中：用框架的 Spinner（原版是自己写 CSS 关键帧做
				// .search-submit-spinner）。v0.4.2 起 Spinner 会跟着
				// TextColor 上色，所以能直接染成主题蓝。
				ui.Spinner(c).Size(18, 18).TextColor(t.Blue)
			} else {
				ui.Icon(c, iconMagnifier).Size(18, 18).
					TextColor(pick(submit.Hovered(), ui.Hex("#0284c7"), ui.Hex("#64748b")))
			}
		})
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
	t := a.Theme
	b := ui.ButtonBase(c.Key("nav-"+key)).Size(ToolButton, ToolButton).Radius(ToolButtonR).
		Center().Label(label).Tooltip(label)
	glassOn(b, false, b.Hovered(), ui.Color{}, ui.Hex("#ffffff").Alpha(0.30))
	pressPose(b)
	b.Children(func() { ui.Icon(c, ic).Size(18, 18).TextColor(t.Muted) })
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
	// 圆形头像键：半透明填充 + 白边（原版 .home-user-button 无 backdrop-filter）。
	// 文案与原版一致：未登录是「登录账号」，已登录是「切换登录账号」（aria-label），
	// title 分别提示「点击登录账号」/「点击登录其他账号」。
	b := ui.ButtonBase(c.Key("user")).Size(AvatarSize, AvatarSize).Radius(RadiusPill).
		Center().Clip().Background(ui.Hex("#ffffff").Alpha(0.32)).
		Border(1, ui.Hex("#ffffff").Alpha(0.55)).
		Label(pick(a.LoggedIn, "切换登录账号", "登录账号")).
		Tooltip(pick(a.LoggedIn, "点击登录其他账号", "点击登录账号"))
	b.Children(func() {
		if bmp := a.Images.Bitmap(a.Face, imagecache.WidthAvatar); bmp != nil {
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

// coverDisc 是左栏的封面（原版 .cover-shell 292px，内部 #video-cover）：
//
//	disc   （默认）圆形唱片，播放时 22s 转一圈，点一下切换播放/暂停；
//	square 静态圆角方块（省 GPU），点一下也切换播放/暂停。
//
// 右下角三个开关（原版 .cover-mode-toggle）：切换碟片/封面、封面背景、高级质感。
func (a *App) coverDisc(c *ui.Context) {
	ui.Row(c).FillWidth().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		shell := ui.Box(c).Size(292, 292).Shrink(0)
		// 原版 .cover-mode-toggle 默认 opacity:0，.cover-shell:hover 才显形。
		shellHovered := shell.Hovered()
		shell.Children(func() {
			if a.CoverMode == "square" {
				sq := ui.ButtonBase(c.Key("disc")).Fill().Radius(14).Clip().
					Background(ui.Hex("#0f172a")).
					Border(1, ui.Hex("#ffffff").Alpha(0.20)).
					Label(pick(a.Playing, "暂停", "播放"))
				sq.Children(func() { a.coverOrLogo(c) })
				if sq.Clicked() && a.Act.TogglePlay != nil {
					a.Act.TogglePlay()
				}
			} else {
				// 外圈玻璃环（原版 .cover-shell::before）：inset 3% →
				// 292 × 0.94 ≈ 274，1px 白边 + 柔和投影；唱片 263 居中其内。
				ring := ui.Box(c).Size(274, 274).AlignSelf(ui.Center).Radius(RadiusPill).
					Border(1, ui.Hex("#ffffff").Alpha(0.20)).
					Shadow(0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.12)).Center()
				ring.Children(func() {
					// 唱片（原版 #video-cover）：90% 于外环 = 263，深底 + 1px
					// 白边 + 内侧 1px 白线（inset 0 0 0 1px rgba(255,255,255,.1)）。
					disc := ui.ButtonBase(c.Key("disc")).Size(263, 263).Radius(RadiusPill).Clip().
						Background(ui.Hex("#0f172a")).
						Border(1, ui.Hex("#ffffff").Alpha(0.20)).
						InsetShadow(0, 0, 0, 1, ui.Hex("#ffffff").Alpha(0.10)).
						Label(pick(a.Playing, "暂停", "播放"))
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
						if a.coverBitmap() != nil || Logo != nil {
							a.advanceDisc(time.Now())
							img := a.coverImage(c)
							if a.Playing {
								rate := float32(discSpinStep) / float32(discSpinPeriod) * 360
								img.AnimatePose(ui.Pose{Rotate: a.discAngle + rate},
									ui.Animation{Duration: discSpinStep, Ease: ui.Linear})
							} else {
								img.AnimatePose(ui.Pose{Rotate: a.discAngle},
									ui.Animation{Duration: discSpinStep, Ease: ui.Linear})
							}
						}
					})
					if disc.Clicked() && a.Act.TogglePlay != nil {
						a.Act.TogglePlay()
					}
				})
			}

			// 三个小开关：碟片/封面、封面背景、高级质感。
			// 原版底部右起 8px，彼此间距 6（28 + 6 = 34）；整组随封面悬停淡入。
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
				a.coverToggle(c, "ambient",
					pick(a.Ambient, "关闭封面背景", "开启封面背景"),
					iconHalo, func() {
						if a.Act.ToggleAmbient != nil {
							a.Act.ToggleAmbient()
						}
					})
				a.coverToggle(c, "premium",
					pick(a.Premium, "关闭高级质感（更省电）", "开启高级质感"),
					iconSparkles, func() {
						if a.Act.TogglePremium != nil {
							a.Act.TogglePremium()
						}
					})
			})
		})
	})
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
//（之前激活直接把图标提到 .95，是多余的偏差）。
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

// coverToggle 是封面上的一个小开关（原版 .cover-mode-toggle）。激活态由
// coverToggleActive 按 key 决定（只有封面背景 / 高级质感有 is-active）。
func (a *App) coverToggle(c *ui.Context, key, label string, ic *ui.SVG, fn func()) {
	b := ui.ButtonBase(c.Key("cover-"+key)).Size(28, 28).Radius(RadiusPill).
		Center().Label(label).Tooltip(label)
	// 这几个键浮在封面上：深色半透明填充 + 1px 白边（原版
	// .cover-mode-toggle 也没有 backdrop-filter，之前用 Clear 玻璃多算了
	// 三次 backdrop 重算）。
	active := coverToggleActive(key, a.Ambient, a.Premium)
	bg, border, ink := coverToggleStyle(active, b.Hovered())
	b.Background(bg).Border(1, border)
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

// videoInfo 是右栏（原版 #video-info，固定 366 高）：
// UP 主行 / 标题 / 简介 / 选集胶囊 / 操作行 / 工具条，自上而下。
func (a *App) videoInfo(c *ui.Context) {
	t := a.Theme
	ui.Column(c).Grow(1).MinWidth(0).AlignItems(ui.Start).Children(func() {
		a.videoInfoHead(c)
		// 原版 #video-title h2：22px / 800 / 2 行截断 / min-height 52。
		ui.Text(c, a.infoTitle()).FontSize(22).FontWeight(800).TextColor(t.Ink).
			MinHeight(52).MaxLines(2).Margin(8, 0, 0, 0).Selectable()
		// 原版 #video-desc（后者覆盖前者）：13px / var(--studio-muted) /
		// margin-top 14 / max-height 60 = 3 行（line-height 1.55）。
		ui.Text(c, a.infoDesc()).FontSize(13).TextColor(ui.Hex("#475569")).
			MaxLines(3).Margin(14, 0, 0, 0).Selectable()
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
		// 悬停放大 5%（原版 #video-owner-face 的 hover:scale-105）。
		// 用 AnimatePose：只重绘不重建视图，便宜。
		scale := float32(1)
		if face.Hovered() {
			scale = 1.05
		}
		face.AnimatePose(ui.Pose{Scale: scale}, ui.Snappy)
		face.Children(func() {
			if bmp := a.Images.Bitmap(a.ownerFace(), imagecache.WidthAvatar); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			}
		})
		if face.Clicked() {
			a.openUp()
		}

		ui.Column(c).MinWidth(0).Children(func() {
			ui.Text(c, "UP 主").FontSize(10).Bold().TextColor(ui.Hex("#8a95a6"))
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				name := ui.ButtonBase(c.Key("owner-name")).Padding(0).Label("UP 主空间")
				name.Background(ui.Transparent)
				name.Children(func() {
					// 原版 #video-owner-name：15px / 650 / hover #0284c7。
					ui.Text(c, a.ownerName()).FontSize(15).FontWeight(650).
						TextColor(pick(name.Hovered(), ui.Hex("#0284c7"), ui.Hex("#334155"))).
						SingleLine().MaxWidth(240)
				})
				if name.Clicked() {
					a.openUp()
				}
				// 合作视频角标（原版只在详情页有 staff 字段时显示）。
				if a.Info != nil && len(a.Info.Staff) > 1 {
					a.collabBadge(c)
				}
			})
		})
	})
}

// collabBadge 是合作视频角标：点击展开参与 UP 主列表，选一个进 TA 的空间
// （原版 videoInfo 的选择合作 UP 主菜单）。
func (a *App) collabBadge(c *ui.Context) {
	b := ui.Box(c).Height(20).Padding(0, 7).Radius(4).Center().
		Background(ui.Hex("#0284c7").Alpha(0.14)).
		Border(1, ui.Hex("#0284c7").Alpha(0.30)).
		Cursor(ui.CursorPointer).
		Label("选择合作 UP 主").Tooltip(fmt.Sprintf("选择合作 UP 主 (%d)", len(a.Info.Staff)))
	if hover := b.Hovered(); hover {
		b.Background(ui.Hex("#0284c7").Alpha(0.24))
	}
	b.Children(func() {
		ui.Text(c, fmt.Sprintf("合作 %d 人", len(a.Info.Staff))).FontSize(10).Bold().
			TextColor(ui.Hex("#0369a1"))
	})
	b.Menu(func(m *ui.Menu) {
		for _, st := range a.Info.Staff {
			st := st
			label := st.Name
			if st.Title != "" {
				label = st.Title + " · " + st.Name
			}
			if m.Item(label).Chosen() && a.Act.OpenUp != nil {
				a.Act.OpenUp(st.Mid, st.Name)
			}
		}
	})
}

// openUp 打开当前视频 UP 主的空间。没有 mid 就什么都不做。
func (a *App) openUp() {
	var mid int64
	if a.Info != nil {
		mid = a.Info.OwnerMid
	}
	if mid == 0 || a.Act.OpenUp == nil {
		a.NotifyType("warning", "该视频没有可用的 UP 主空间")
		return
	}
	a.Act.OpenUp(mid, a.ownerName())
}

// partPill 是选集胶囊（.video-part-pill）：小圆点 + 来源 + 分集标题。
// 从播放列表播入时前缀显示「播放列表」（原版 isPlaylistMode）。
func (a *App) partPill(c *ui.Context) {
	t := a.Theme
	blue := ui.Hex("#0369a1")
	// 播放列表模式的圆点是玫红（原版 .video-part-dot-playlist 用
	// --studio-rose），非播放列表才是蓝。
	inPlaylist := a.PlayingPlaylist != ""
	dotInk := pick(inPlaylist, t.Rose, ui.Hex("#0284c7"))
	source := "当前选集"
	if inPlaylist {
		source = "播放列表"
	}
	// 原版 .video-part-pill：min-height 36、margin-top 12、padding 5 11、
	// 薄玻璃底 rgba(255,255,255,.15) + 1px 白边（.38）、外侧 0 2px 10px
	// rgba(15,23,42,.035)、内侧顶部一线高光。
	// 半透明填充（原版 .video-part-pill 没有 backdrop-filter）：薄白底 +
	// 1px 白边 + 顶部一线内高光，见上方注释。
	ui.Row(c).MaxWidth(430).MinHeight(36).Margin(12, 0, 0, 0).Padding(5, 11).
		Radius(RadiusPill).
		Background(ui.Hex("#ffffff").Alpha(0.15)).
		Border(1, ui.Hex("#ffffff").Alpha(0.38)).
		InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.50)).
		Shadow(0, 2, 10, 0, ui.Hex("#0f172a").Alpha(0.035)).
		Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Shrink(0).Radius(RadiusPill).Background(dotInk)
		ui.Text(c, source).FontSize(13).Bold().TextColor(blue).Shrink(0)
		ui.Text(c, a.partTitle()).FontSize(13).Bold().TextColor(blue).SingleLine()
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
	var aid int64
	if a.Info != nil {
		bvid, aid = a.Info.Bvid, a.Info.Aid
	}
	// 原版 .video-content-actions: margin-top 14、padding 0 0 14px、gap 4。
	// 底部 14px 是给绝对定位的计数留的位置（这里用等高的计数行实现）。
	ui.Row(c).Margin(14, 0, 0, 0).Padding(0, 0, 14, 0).Gap(4).
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
			Icon: iconStar, Size: 20, Active: a.Faved,
			ActiveInk: ui.Hex("#eab308"), Count: a.statFav(),
			// 原版收藏键在「收藏状态还在查」时也禁用（isFavoriteUpdating），
			// 否则会用上一条视频的状态误操作。
			Off: aid == 0 || a.FavPending, OnClick: a.Act.Favorite,
		})
		a.contentIconButton(c, "browser", "浏览器打开", iconBrowser, 20, func() {
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
		a.contentIconButton(c, "video", "视频播放", iconVideo, 20, func() {
			if a.VideoOpen {
				if a.Act.CloseVideo != nil {
					a.Act.CloseVideo()
				}
			} else if a.Act.OpenVideo != nil {
				a.Act.OpenVideo()
			}
		})
		a.contentIconButton(c, "danmaku", "弹幕/评论", iconDanmaku, 20, a.openDanmaku)
	})
}

// contextDock 是工具条（.video-context-dock）：
// 搜索结果 / 选集 / 合集 / 播放列表（旧版四个）。
func (a *App) contextDock(c *ui.Context) {
	t := a.Theme
	ui.Row(c).Margin(12, 0, 0, 0).Padding(3).Radius(10).Gap(2).AlignItems(ui.Center).
		Background(ui.Hex("#0f172a").Alpha(0.03)).
		Border(1, ui.Hex("#64748b").Alpha(0.14)).
		InsetShadow(0, 1, 2, 0, ui.Hex("#0f172a").Alpha(0.05)).
		InsetShadow(0, -1, 0, 0, ui.Hex("#ffffff").Alpha(0.35)).Children(func() {
		// 搜索结果：没有搜索结果时禁用（原版 disabled={!searchResultsCount}）。
		sr := a.iconButton(c, "dock-search", "搜索结果", iconSearch, 20, func() {
			a.Drawer = DrawerSearch
		})
		disableNav(sr, len(a.ListFor(DrawerSearch).Cards) == 0)
		// 选集：非播放列表模式且有视频时图标是蓝色（原版 fill 逻辑）。
		partsInk := ui.Color{}
		if a.PlayingPlaylist == "" && a.Track != nil {
			partsInk = t.Blue
		}
		a.inkButton(c, "dock-parts", "选集", iconList, 20, partsInk, a.openParts)
		// 合集：还没选过合集时禁用。
		series := a.iconButton(c, "dock-series", "合集", iconSeries, 20, a.openSeries)
		disableNav(series, a.SeriesID == 0)
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
		a.inkButton(c, "dock-playlist", "播放列表", iconMusicList, 20,
			plistInk, func() {
				if a.Drawer == DrawerPlaylist {
					a.Drawer = ""
				} else {
					a.Drawer = DrawerPlaylist
				}
			})
		if n := a.playlistBadgeCount(); n > 0 {
			// 原版 .nav-badge：14 高、圆角 7、1px 白边 + 内高光 +
			// 一层玫红投影（合集来源才换紫底）。
			badge := ui.Text(c, a.playlistBadgeText()).FontSize(9).Bold().
				TextColor(ui.Hex("#ffffff")).
				Absolute().Right(-4).Top(-4).Padding(1, 4).Radius(RadiusPill).
				Background(a.playlistBadgeInk()).
				Border(1, ui.Hex("#ffffff").Alpha(0.55)).
				InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.50)).
				InsetShadow(0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.08)).
				Shadow(0, 2, 6, 0, t.Rose.Alpha(0.20))
			badge.Label("播放列表数量")
		}
	})
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
// 计数用「独立文本 + 固定行高」实现而不是 Absolute（原版用 absolute 是
// CSS 里避免计数撑高行高的手法；mygo 的 Absolute 需要父级显式定位，
// 在 Button 的 Center 布局里取不到整行宽，居中不了）。整列高度
// 整列高：按钮 32 + 间隔 1 + 计数 12。
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
	Off     bool
	OnClick func()
}

// statButton 是互动按钮（原版 .video-content-actions .nav-icon-btn）：
// 34×34 图标钮（圆角 8），9px 计数挂在按钮正下方；激活时图标按动作变色——
// 点赞红、投币黄、收藏黄。
func (a *App) statButton(c *ui.Context, spec statSpec) {
	col := ui.Column(c).Width(ContentIcon).Height(ContentIcon + 13).
		AlignItems(ui.Center).Shrink(0)
	col.Children(func() {
		b := ui.ButtonBase(c.Key("stat-"+spec.Key)).Size(ContentIcon, ContentIcon).
			Radius(ContentIconR).Center().Label(spec.Label).Tooltip(spec.Label)
		disableNav(b, spec.Off)
		// 半透明填充；激活（已赞/已投币/已收藏）用该状态的强调色做底色。
		switch {
		case spec.Off:
			b.Background(ui.Transparent)
		case spec.Active:
			glassOn(b, true, b.Hovered(), spec.ActiveInk.Alpha(0.26), spec.ActiveInk.Alpha(0.26))
		default:
			glassOn(b, false, b.Hovered(), ui.Color{}, ui.Hex("#ffffff").Alpha(0.30))
		}
		b.Children(func() {
			ui.Icon(c, spec.Icon).Size(spec.Size, spec.Size).
				TextColor(pick(spec.Active, spec.ActiveInk, ui.Hex("#475569")))
		})
		if b.Clicked() && spec.OnClick != nil {
			spec.OnClick()
		}
		// 原版这一格始终有值（没有视频时是 "0"，禁用态用 inherit 颜色）。
		if spec.Count != "" {
			ui.Text(c, spec.Count).FontSize(9).
				TextColor(pick(spec.Active, spec.ActiveInk, ui.Hex("#64748b"))).
				SingleLine().MaxWidth(ContentIcon)
		}
	})
}

// disableNav 把按钮设成原版 `.nav-icon-btn:disabled` 的样子：禁用 +
// opacity .3。mygo 的禁用态自己会乘 .5，这里再乘 .6 补到 .3
// （注意 IsDisabled 查不到「之后才调用 Disabled」的情况，所以显式传参）。
func disableNav(b ui.Element, off bool) ui.Element {
	b.Disabled(off)
	if off {
		b.Opacity(0.6)
	}
	return b
}

// iconButton 是 34×34 的纯图标按钮（.video-context-dock .nav-icon-btn）。
func (a *App) iconButton(c *ui.Context, key, label string, ic *ui.SVG, size float32, fn func()) ui.Element {
	return a.inkButton(c, key, label, ic, size, ui.Color{}, fn)
}

// contentIconButton 是内容操作行的图标钮（.video-content-actions .nav-icon-btn）：
// 34×34、圆角 8（比工具条的 7 大）。
func (a *App) contentIconButton(c *ui.Context, key, label string, ic *ui.SVG, size float32, fn func()) ui.Element {
	t := a.Theme
	b := ui.ButtonBase(c.Key("icon-"+key)).Size(ContentIcon, ContentIcon).Radius(ContentIconR).
		Center().Label(label).Tooltip(label)
	b.Background(pressFeedback(b, ui.Transparent, t.GlassHover, t.GlassActive))
	b.Children(func() { ui.Icon(c, ic).Size(size, size).TextColor(ui.Hex("#475569")) })
	if b.Clicked() && fn != nil {
		fn()
	}
	return b
}

// inkButton 同 iconButton，ink 非零时用它做图标颜色（原版部分按钮是蓝色）。
func (a *App) inkButton(c *ui.Context, key, label string, ic *ui.SVG, size float32, ink ui.Color, fn func()) ui.Element {
	t := a.Theme
	textInk := ui.Hex("#475569")
	if ink.A != 0 {
		textInk = ink
	}
	// 原版 .video-context-dock .nav-icon-btn：34×34、圆角 7、无边框、径向高光底。
	b := ui.ButtonBase(c.Key("icon-"+key)).Size(DockIcon, DockIcon).Radius(DockIconR).
		Center().Label(label).Tooltip(label)
	// 原版 .nav-icon-btn:hover 是玻璃底 + 上浮 1px；mygo 没有位移变换，
	// 悬停给玻璃底、按住给实一点的底（见 pressFeedback）。
	b.Background(pressFeedback(b, ui.Transparent, t.GlassHover, t.GlassActive))
	b.Children(func() { ui.Icon(c, ic).Size(size, size).TextColor(textInk) })
	if b.Clicked() && fn != nil {
		fn()
	}
	return b
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
func (a *App) coverOrLogo(c *ui.Context) {
	if bmp := a.coverBitmap(); bmp != nil {
		ui.Image(c, bmp).Fill().Fit(ui.Cover)
	} else if Logo != nil {
		ui.Image(c, Logo).Fill().Fit(ui.Cover)
	}
}

// coverImage 画圆盘封面（或 logo）并返回图片元素，供旋转。
// 调用前先确认 coverBitmap() 或 Logo 非空。
func (a *App) coverImage(c *ui.Context) ui.Element {
	if bmp := a.coverBitmap(); bmp != nil {
		return ui.Image(c, bmp).Fill().Fit(ui.Cover)
	}
	return ui.Image(c, Logo).Fill().Fit(ui.Cover)
}
