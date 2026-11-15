package view

// 视频窗的**内置设置面板**：顶栏齿轮键点开，弹出在标题栏下方。
//
// 为什么在视频窗里（用户 2026-11 的要求）：「播放页面的设置不要放在菜单栏下，
// 统一在播放窗口下增加一个设置按钮，打开之后再弹出，这样功能交互上比较直观」。
// 此前弹幕那 5 项（总开关 / 不透明度 / 字号 / 显示区域 / 同屏上限）藏在主窗
// 标题栏的「设置」下拉里 —— 用户正看着视频时要去主窗找一个二级菜单，路径
// 既长、又不直观。现在这些设置跟着画面走：**视频窗里改，视频窗里立刻看见**。
//
// 面板分成三组（顺序 = 使用频率）：
//
//	弹幕      总开关 / 不透明度 / 字号 / 显示区域 / 同屏上限
//	          （这 5 项从主窗设置菜单迁来，同一份事实源：App.DanmakuStyle）
//	播放      倍速 / 音量均衡（原生视频不适用，置灰说明）/ 广告跳过
//	          （共享设置：与主窗播放栏是**同一个 PlayerSettings 指针**，
//	          改任一侧另一侧下一帧就是新值，文案用主窗那几条 speedTip /
//	          sponsorTip；音量均衡那一行不是开关，见 eqVideoUnavailableTip）
//	播放引擎  原生视频播放（保留用户的自主选择权）
//
// 主窗设置菜单里这 8 项全部移除（用户明确不要两处入口），「原生视频播放」
// 的入口也只剩这里 —— 它是**视频怎么播**的开关，属于这一页。
//
// 这一轮的**排版秩序**（用户：现在的 UI 看着不够正式）：几何全部来自
// videotokens.go —— 行高统一 36（头部同高）、档位按钮 28、面板内边距 12、
// 块间距 8；分组之间用 1px 分隔线，不再靠大片留白；开关与档位选择器都收在
// 行尾，右缘与面板内容右缘对齐。字号只有三档：标题 13/600、正文 12、次要 11。
//
// 组件：PopoverBase（与主窗设置菜单、合作 UP 主下拉同一个）。框架负责开合、
// 点面板外关、Esc 关与焦点归还；材质用同一块玻璃（模糊 + 面板底 + 静态洗光），
// 深浅两态都跟主窗的主题令牌走。
//
// 面板打开期间控件条**不自动隐藏**（见 VideoChrome.holdChrome）：用户正在一个
// 浮层里调设置，底下的控件条藏起来既没用又晃眼。关闭后立刻恢复原来的
// 「静止 2.5s 淡出」。面板高度受窗口高度限制，超出部分在内部滚动 ——
// 320×200 的最小窗口里也能开、能滚、能关（关闭键在固定头部）。

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

// videoSettingsBlockH 是档位行的高度：标题一行（行盒 18）+ 一个细缝（4）+
// 档位按钮（28）。开关行统一 36，档位行统一这一个值 —— 两种行各自一致。
const videoSettingsBlockH float32 = videoTimeLineH + videoStep + videoChoiceH

// settingsPanel 弹出设置面板。anchor 是这一帧的齿轮键（弹出位置由它定）。
//
// 面板宽度在窗口内夹取：320 宽的小窗口里是 300，位置由框架按 anchor 摆平
// （放不下时左右挪进窗口，不裁切）。高度按 videoSettingsMaxH 封顶，超出
// 的内容交给内部的 ui.Scroll。
func (v *VideoChrome) settingsPanel(c *ui.Context, t Theme, anchor ui.Element) {
	if !v.showSettings || !anchor.Valid() {
		return
	}
	w, h := c.Size()
	panelW := minF(videoPanelW, maxFloat32(w-2*videoPanelEdge, 200))
	ui.PopoverBase(c, anchor, &v.showSettings, func(panel ui.Element) {
		panel.Width(panelW).MaxHeight(videoSettingsMaxH(h)).
			Margin(videoPanelGap, 0, 0, 0).
			Padding(videoPanelPadY, videoPanelPadX).
			Radius(videoPanelRadius).Gap(videoPanelGap).
			// 材质与主窗设置下拉 / 合作 UP 主面板同一块玻璃：只模糊，再自铺
			// 一层面板底（深色外观是 #18263a .94，浅色是 Theme.Glass），最后
			// 铺静态洗光。视频画面明暗不可控，所以底比普通弹层更实一档。
			Material(blurLayer(glass.Blur{Radius: glassBlurSigma})).
			Border(1, t.GlassBorder).
			Draw(func(p *ui.Painter, r ui.Rect) {
				paintGlassPanelFill(p, r, videoPanelRadius, t.surface(t.Glass, "#18263a", 0.94))
				paintBakedSheen(p, r, videoPanelRadius, t.glassSheen())
			}).
			InsetShadow(0, 1, 1, 0, t.Highlight).
			InsetShadow(0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.06)).
			Shadow(0, 18, 44, 0, ui.Hex("#0f172a").Alpha(pick[float32](t.Dark, 0.42, 0.10)))
		panel.Children(func() {
			// 固定头部：标题 + 关闭键。滚动区在它下面，滚不到它。
			ui.Row(c).FillWidth().Height(videoSettingsHeadH).AlignItems(ui.Center).Gap(videoGap).
				Children(func() {
					ui.Text(c, "视频设置").FontSize(videoFontTitle).FontWeight(videoTitleWeight).
						TextColor(t.Ink).Grow(1).SingleLine().Ellipsis("…")
					b := videoKeyTitle(c, t, "video-settings-close", "关闭设置", "关闭设置",
						iconClose, false)
					if b.Clicked() {
						v.showSettings = false
					}
				})
			// 头部与内容之间一条 1px 低对比 hairline（与两条栏同一档）。
			ui.Box(c).FillWidth().Height(1).Shrink(0).Background(videoHairline(t))
			ui.Scroll(c).FillWidth().MaxHeight(videoSettingsScrollH(h)).Children(func() {
				v.settingsBody(c, t)
			})
		})
	})
}

// settingsBody 是面板的滚动内容。
func (v *VideoChrome) settingsBody(c *ui.Context, t Theme) {
	set := v.settings()
	ui.Column(c).FillWidth().Children(func() {
		// ---------------------------------------------------------- 弹幕
		videoSettingsGroup(c, t, "弹幕", true)
		videoSettingsToggle(c, t, "video-settings-danmaku", "弹幕总开关",
			pick(v.Danmaku, "弹幕 · 已开", "弹幕 · 已关"), v.Danmaku, v.toggleDanmaku)

		s := v.DanmakuSettings
		settingsChoices(c, t, "video-opacity", "不透明度", []settingsChoice{
			{Key: "opacity-30", Label: "30%", On: abs(s.Opacity-0.3) < 0.01,
				Pick: func() { s.Opacity = 0.3; v.applyDanmakuSettings(s) }},
			{Key: "opacity-50", Label: "50%", On: abs(s.Opacity-0.5) < 0.01,
				Pick: func() { s.Opacity = 0.5; v.applyDanmakuSettings(s) }},
			{Key: "opacity-70", Label: "70%", On: abs(s.Opacity-0.7) < 0.01,
				Pick: func() { s.Opacity = 0.7; v.applyDanmakuSettings(s) }},
			{Key: "opacity-100", Label: "100%", On: abs(s.Opacity-1) < 0.01,
				Pick: func() { s.Opacity = 1; v.applyDanmakuSettings(s) }},
		})
		settingsChoices(c, t, "video-size", "字号", []settingsChoice{
			{Key: "size-small", Label: "小", On: s.Size == 0,
				Pick: func() { s.Size = 0; v.applyDanmakuSettings(s) }},
			{Key: "size-medium", Label: "中", On: s.Size == 1,
				Pick: func() { s.Size = 1; v.applyDanmakuSettings(s) }},
			{Key: "size-large", Label: "大", On: s.Size == 2,
				Pick: func() { s.Size = 2; v.applyDanmakuSettings(s) }},
		})
		settingsChoices(c, t, "video-area", "显示区域", []settingsChoice{
			{Key: "area-full", Label: "铺满整屏", On: s.Area > 0.75,
				Pick: func() { s.Area = 1; v.applyDanmakuSettings(s) }},
			{Key: "area-half", Label: "上方 1/2", On: s.Area <= 0.75,
				Pick: func() { s.Area = 0.5; v.applyDanmakuSettings(s) }},
		})
		settingsChoices(c, t, "video-max", "同屏上限", []settingsChoice{
			{Key: "max-30", Label: "30", On: s.Max == danmaku.MaxOnScreen(0),
				Pick: func() { s.Max = danmaku.MaxOnScreen(0); v.applyDanmakuSettings(s) }},
			{Key: "max-60", Label: "60", On: s.Max == danmaku.MaxOnScreen(1),
				Pick: func() { s.Max = danmaku.MaxOnScreen(1); v.applyDanmakuSettings(s) }},
			{Key: "max-100", Label: "100", On: s.Max == danmaku.MaxOnScreen(2),
				Pick: func() { s.Max = danmaku.MaxOnScreen(2); v.applyDanmakuSettings(s) }},
		})
		videoSettingsNote(c, t, "同屏上限也是性能护栏：超出后新弹幕丢弃，不重排已有弹幕。")

		// ---------------------------------------------------------- 播放
		// 这三项是**共享设置**：与主窗播放栏同一个 PlayerSettings 指针，
		// 任一侧改动另一侧立刻看见（主窗的监听负责落盘与音频引擎）。
		videoSettingsGroup(c, t, "播放", false)
		speeds := make([]settingsChoice, 0, len(speedOptions))
		for _, opt := range speedOptions {
			opt := opt
			speeds = append(speeds, settingsChoice{
				Key: "speed-" + speedLabel(opt), Label: speedLabel(opt),
				On:   abs(opt-set.Speed) < 0.001,
				Pick: func() { set.SetSpeed(opt) },
			})
		}
		settingsChoices(c, t, "video-speed", "倍速", speeds)
		// 音量均衡对原生视频**不适用**：PCM 在系统媒体框架内部就送去声卡，
		// 本仓库的均衡器拿不到那路采样（见 eqVideoUnavailableTip）。这一行
		// 只说明、不给开关 —— 留一个能点的轨道就是伪造「点了会生效」。
		videoSettingsUnavailable(c, t, "video-settings-eq", "音量均衡",
			eqVideoUnavailableTip(set.EQ))
		videoSettingsNote(c, t,
			"音量均衡只作用于主窗的音频引擎；原生视频的声音由系统媒体框架直接输出，本窗口不适用。")
		videoSettingsToggle(c, t, "video-settings-sponsor", "广告跳过", sponsorTip(),
			set.Sponsor, func() { set.SetSponsor(!set.Sponsor) })

		// ------------------------------------------------------ 播放引擎
		videoSettingsGroup(c, t, "播放引擎", false)
		videoSettingsToggle(c, t, "video-settings-native", "原生视频播放",
			pick(v.NativeVideo, "原生视频播放 · 开（WebView 兜底仍在）", "原生视频播放 · 关（走 WebView）"),
			v.NativeVideo, v.toggleNativeVideo)
		videoSettingsNote(c, t,
			"本机不支持原生解码（缺媒体框架 / 格式不支持）时，起播会先询问你是否改用 WebView 播放，不会自己切。")
	})
}

// videoSettingsGroup 是一组的分隔线 + 分组标题。第一组不打线（面板头部
// 下面已经有一条 hairline），其余每组前面一条 1px 低对比线 —— 分组之间靠
// 线，不靠留白。
func videoSettingsGroup(c *ui.Context, t Theme, label string, first bool) {
	if !first {
		ui.Box(c).FillWidth().Height(1).Shrink(0).
			Margin(videoPanelGap, 0, videoPanelGap, 0).
			Background(videoHairline(t))
	}
	ui.Text(c, label).FontSize(videoFontMeta).FontWeight(videoTitleWeight).
		TextColor(t.text(settingsItemInk.Alpha(0.62))).
		Margin(0, 0, videoStep, 0)
}

// videoSettingsToggle 是一行开关：左侧标签，右侧与主窗设置菜单同一个轨道
// （incognitoTrack，开启转青）。整行可点，点完整行翻转，面板不收起。
//
// 行高统一 videoSettingsRowH（36），开关收在行尾（右缘与面板内容右缘对齐）。
func videoSettingsToggle(c *ui.Context, t Theme, key, label, tip string, on bool, toggle func()) {
	row := ui.ButtonBase(c.Key(key)).FillWidth().Height(videoSettingsRowH).
		Padding(0).Justify(ui.Start).Radius(videoKeyRadius).
		Label("视频设置：" + label).Tooltip(tip).
		Role(ui.RoleSwitch).Checked(on)
	switch {
	case row.Pressed():
		row.Background(t.Ink.Alpha(0.08))
	case row.Hovered():
		row.Background(t.Ink.Alpha(0.04))
	}
	row.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(videoGap).Children(func() {
			ui.Text(c, label).FontSize(videoFontBody).TextColor(t.text(settingsItemInk)).
				Grow(1).SingleLine().Ellipsis("…")
			incognitoTrack(c, on)
		})
	})
	if row.Clicked() {
		toggle()
	}
}

// videoSettingsUnavailable 是一行「本窗口不支持」的项：标签在左、右侧写
// 「不适用」而**不画开关轨道**，整行禁用并给 tooltip。
//
// 为什么不画轨道：轨道会让人以为它现在只是关着、点一下就能开 —— 这正是
// 用户要求避免的「伪造状态」。禁用的 ButtonBase 不接收点击（ui 的 disabled
// 节点不会进入命中），所以这里不需要额外的闸。
func videoSettingsUnavailable(c *ui.Context, t Theme, key, label, tip string) {
	row := ui.ButtonBase(c.Key(key)).FillWidth().Height(videoSettingsRowH).
		Padding(0).Justify(ui.Start).Radius(videoKeyRadius).
		Label("视频设置：" + label).Tooltip(tip)
	setDisabledOpacity(row, true, playerButtonDisabledOpacity)
	row.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(videoGap).Children(func() {
			ui.Text(c, label).FontSize(videoFontBody).TextColor(t.text(settingsItemInk)).
				Grow(1).SingleLine().Ellipsis("…")
			ui.Text(c, "不适用").FontSize(videoFontMeta).Shrink(0).
				TextColor(t.text(settingsItemInk.Alpha(0.62)))
		})
	})
}

// settingsChoice 是档位行里的一颗按钮。
type settingsChoice struct {
	Key   string
	Label string
	On    bool
	Pick  func()
}

// settingsChoices 画一行「标题 + 等宽档位按钮」：标题一行（次要信息 11），
// 下面一排等高按钮（28），整行占 videoSettingsBlockH。按钮选中蓝底蓝字、
// 悬停淡墨底 —— 激活态只用 accent 一层，不叠描边。
//
// 按钮等分整行宽度（Grow(1).Basis(0)），所以最后一颗的右缘正好落在面板内容
// 右缘：与开关行的轨道右缘对齐（「开关与选择器在行尾对齐」）。
func settingsChoices(c *ui.Context, t Theme, key, title string, choices []settingsChoice) {
	ink := t.text(settingsItemInk)
	ui.Column(c).FillWidth().Height(videoSettingsBlockH).Gap(videoStep).Children(func() {
		ui.Text(c, title).FontSize(videoFontMeta).TextColor(t.text(settingsItemInk.Alpha(0.78))).
			Height(videoTimeLineH)
		ui.Row(c).FillWidth().Grow(1).Gap(videoStep).Children(func() {
			for _, ch := range choices {
				ch := ch
				b := ui.ButtonBase(c.Key(key + "-" + ch.Key)).
					Grow(1).Basis(0).Height(videoChoiceH).Radius(videoKeyRadius).Center().
					Label("视频设置：" + title + " " + ch.Label).
					Tooltip(title + "：" + ch.Label).
					Role(ui.RoleToggleButton).Checked(ch.On)
				switch {
				case ch.On && b.Hovered():
					b.Background(t.Blue.Alpha(0.22))
				case ch.On:
					b.Background(t.Blue.Alpha(0.14))
				case b.Pressed():
					b.Background(t.Ink.Alpha(0.10))
				case b.Hovered():
					b.Background(t.Ink.Alpha(0.06))
				}
				b.Children(func() {
					ui.Text(c, ch.Label).FontSize(videoFontMeta).TextColor(pick(ch.On, t.Blue, ink))
				})
				if b.Clicked() {
					ch.Pick()
				}
			}
		})
	})
}

// videoSettingsNote 是面板里的一行小字说明（次要信息 11）。
func videoSettingsNote(c *ui.Context, t Theme, text string) {
	ui.Text(c, text).FontSize(videoFontMeta).MaxLines(3).
		TextColor(t.text(settingsItemInk.Alpha(0.60))).
		Margin(videoStep, 0, videoStep, 0)
}

// ---------------------------------------------------------------- 动作

// toggleDanmaku 是面板里的弹幕总开关：先翻本地状态（这一帧就见效），再把新值
// 交给主窗落盘 + 同步所有入口。顶栏那颗键走同一个方法。
func (v *VideoChrome) toggleDanmaku() {
	v.Danmaku = !v.Danmaku
	if v.OnDanmakuToggle != nil {
		v.OnDanmakuToggle(v.Danmaku)
	}
}

// applyDanmakuSettings 先落本地（这一帧就按新值排版 / 画字），再交给主窗
// 落盘 + 同步。与主窗 App.applyDanmakuStyle 是同一条路的两端。
func (v *VideoChrome) applyDanmakuSettings(s danmaku.Settings) {
	v.DanmakuSettings = s.Normalize()
	if v.OnDanmakuSettings != nil {
		v.OnDanmakuSettings(v.DanmakuSettings)
	}
}

// toggleNativeVideo 是面板里的「原生视频播放」开关：翻本地状态并回主窗落盘
// （KV 键仍是既有的 nativeVideoPlayer），Manager 的开关随即生效。
func (v *VideoChrome) toggleNativeVideo() {
	v.NativeVideo = !v.NativeVideo
	if v.OnNativeToggle != nil {
		v.OnNativeToggle(v.NativeVideo)
	}
}
