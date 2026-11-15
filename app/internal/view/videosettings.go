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
// 组件：PopoverBase（与主窗设置菜单、合作 UP 主下拉同一个）。框架负责开合、
// 点面板外关、Esc 关与焦点归还；材质用同一块玻璃（模糊 + 面板底 + 静态洗光），
// 深浅两态都跟主窗的主题令牌走。
//
// 面板打开期间控件条**不自动隐藏**（见 VideoChrome.holdChrome）：用户正在一个
// 浮层里调设置，底下的控件条藏起来既没用又晃眼。关闭后立刻恢复原来的
// 「静止 2.5s 淡出」。面板高度受窗口高度限制，超出部分在内部滚动 ——
// 320×200 的最小窗口里也能开、能滚、能关（关闭键在面板固定头部）。

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

// 面板几何（DIP）。
const (
	// videoSettingsW 是面板的目标宽度；窗口更窄时按窗口宽度收（两边各留
	// videoSettingsEdge）。
	videoSettingsW float32 = 300
	// videoSettingsPadX / videoSettingsPadY 是面板内边距。
	videoSettingsPadX float32 = 12
	videoSettingsPadY float32 = 10
	// videoSettingsRadius 与标题栏 / 控件条同族（12），面板是同一套浮层。
	videoSettingsRadius float32 = 12
	// videoSettingsHeadH 是固定头部（标题 + 关闭键）的高度：与顶栏那几颗键
	// 同尺寸（30×30），头部不会被滚走 —— 小窗口里关闭键始终可达。
	videoSettingsHeadH float32 = videoSettingsRowH
	// videoSettingsGap 是面板与标题栏之间、以及面板各块之间的间距。
	videoSettingsGap float32 = 6
	// videoSettingsEdge 是面板与窗口下缘的距离。
	videoSettingsEdge float32 = 6
	// videoSettingsRowH 是开关行高度（与主窗设置菜单的 34 略有区别：视频窗
	// 窄，30 一行能多放一行）。
	videoSettingsRowH   float32 = 30
	videoSettingsRowPad float32 = 8
	// videoSettingsSubGap 是档位行标题与上一块之间的间距。
	videoSettingsSubGap float32 = 6
)

// videoSettingsMaxH 是面板这一帧的最大高度：标题栏下方到窗口底，上下各留
// videoSettingsGap / videoSettingsEdge。
//
// 320×200 的最小窗口下是 142（面板从 y=52 开始，到 194 收），减掉头部 30、
// 间距 6 与上下内边距 20，滚动区 86 —— 两行半，够开、够滚、够关。
func videoSettingsMaxH(h float32) float32 {
	top := videoChromePad + videoChromeTitleH + videoSettingsGap
	floor := videoSettingsHeadH + videoSettingsGap + videoSettingsRowH + 2*videoSettingsPadY
	return maxFloat32(h-top-videoSettingsEdge, floor)
}

// videoSettingsScrollH 是面板里滚动区能有的最大高度（面板高减去头部与内边距）。
func videoSettingsScrollH(h float32) float32 {
	return maxFloat32(videoSettingsMaxH(h)-videoSettingsHeadH-videoSettingsGap-2*videoSettingsPadY,
		videoSettingsRowH)
}

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
	panelW := minF(videoSettingsW, maxFloat32(w-2*videoSettingsEdge, 200))
	ui.PopoverBase(c, anchor, &v.showSettings, func(panel ui.Element) {
		panel.Width(panelW).MaxHeight(videoSettingsMaxH(h)).
			Margin(videoSettingsGap, 0, 0, 0).
			Padding(videoSettingsPadY, videoSettingsPadX).
			Radius(videoSettingsRadius).Gap(videoSettingsGap).
			// 材质与主窗设置下拉 / 合作 UP 主面板同一块玻璃：只模糊，再自铺
			// 一层面板底（深色外观是 #18263a .94，浅色是 Theme.Glass），最后
			// 铺静态洗光。视频画面明暗不可控，所以底比普通弹层更实一档。
			Material(blurLayer(glass.Blur{Radius: glassBlurSigma})).
			Border(1, t.GlassBorder).
			Draw(func(p *ui.Painter, r ui.Rect) {
				paintGlassPanelFill(p, r, videoSettingsRadius, t.surface(t.Glass, "#18263a", 0.94))
				paintBakedSheen(p, r, videoSettingsRadius, t.glassSheen())
			}).
			InsetShadow(0, 1, 1, 0, t.Highlight).
			InsetShadow(0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.06)).
			Shadow(0, 18, 44, 0, ui.Hex("#0f172a").Alpha(pick[float32](t.Dark, 0.42, 0.10)))
		panel.Children(func() {
			// 固定头部：标题 + 关闭键。滚动区在它下面，滚不到它。
			ui.Row(c).FillWidth().Height(videoSettingsHeadH).AlignItems(ui.Center).Gap(6).
				Children(func() {
					ui.Text(c, "视频设置").FontSize(12.5).FontWeight(650).
						TextColor(t.Ink).Grow(1).SingleLine().Ellipsis("…")
					b := playerBarTitleKey(c, t, "video-settings-close", "关闭设置", "关闭设置",
						iconClose, false)
					if b.Clicked() {
						v.showSettings = false
					}
				})
			ui.Scroll(c).FillWidth().MaxHeight(videoSettingsScrollH(h)).Children(func() {
				v.settingsBody(c, t)
			})
		})
	})
}

// settingsBody 是面板的滚动内容。
func (v *VideoChrome) settingsBody(c *ui.Context, t Theme) {
	set := v.settings()
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		// ---------------------------------------------------------- 弹幕
		videoSettingsGroup(c, t, "弹幕")
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
		videoSettingsGroup(c, t, "播放")
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
		videoSettingsGroup(c, t, "播放引擎")
		videoSettingsToggle(c, t, "video-settings-native", "原生视频播放",
			pick(v.NativeVideo, "原生视频播放 · 开（WebView 兜底仍在）", "原生视频播放 · 关（走 WebView）"),
			v.NativeVideo, v.toggleNativeVideo)
		videoSettingsNote(c, t,
			"本机不支持原生解码（缺媒体框架 / 格式不支持）时，起播会先询问你是否改用 WebView 播放，不会自己切。")
	})
}

// videoSettingsGroup / videoSettingsToggle / settingsChoices / videoSettingsNote
// 是面板里四种基本的行。它们与主窗设置菜单的行是同一套语言（开关行 + 等宽
// 档位按钮），只是底色按「压在视频上的玻璃面板」取值。

// videoSettingsGroup 是一组的分组标题。
func videoSettingsGroup(c *ui.Context, t Theme, label string) {
	ui.Text(c, label).FontSize(10).FontWeight(600).
		TextColor(t.text(settingsItemInk.Alpha(0.62))).
		Margin(videoSettingsSubGap, 0, 1, 0)
}

// videoSettingsToggle 是一行开关：左侧标签，右侧与主窗设置菜单同一个轨道
// （incognitoTrack，开启转青）。整行可点，点完整行翻转，面板不收起。
func videoSettingsToggle(c *ui.Context, t Theme, key, label, tip string, on bool, toggle func()) {
	row := ui.ButtonBase(c.Key(key)).FillWidth().Height(videoSettingsRowH).
		Padding(0, videoSettingsRowPad).Justify(ui.Start).Radius(6).
		Label("视频设置：" + label).Tooltip(tip).
		Role(ui.RoleSwitch).Checked(on)
	switch {
	case row.Pressed():
		row.Background(t.Blue.Alpha(0.18))
	case row.Hovered():
		row.Background(t.Blue.Alpha(0.08))
	}
	row.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, label).FontSize(12.5).TextColor(t.text(settingsItemInk)).
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
		Padding(0, videoSettingsRowPad).Justify(ui.Start).Radius(6).
		Label("视频设置：" + label).Tooltip(tip)
	setDisabledOpacity(row, true, playerButtonDisabledOpacity)
	row.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, label).FontSize(12.5).TextColor(t.text(settingsItemInk)).
				Grow(1).SingleLine().Ellipsis("…")
			ui.Text(c, "不适用").FontSize(10.5).Shrink(0).
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

// settingsChoices 画一行「标题 + 等宽档位按钮」：选中蓝底蓝字、悬停淡蓝底，
// 与主窗设置菜单的弹幕档位、外观预设同一套观感。
func settingsChoices(c *ui.Context, t Theme, key, title string, choices []settingsChoice) {
	ink := t.text(settingsItemInk)
	ui.Text(c, title).FontSize(10.5).TextColor(t.text(settingsItemInk.Alpha(0.78))).
		Margin(videoSettingsSubGap, 0, 2, 0)
	ui.Row(c).FillWidth().Gap(4).Children(func() {
		for _, ch := range choices {
			ch := ch
			b := ui.ButtonBase(c.Key(key + "-" + ch.Key)).
				Grow(1).Basis(0).Height(26).Radius(6).Center().
				Label("视频设置：" + title + " " + ch.Label).
				Tooltip(title + "：" + ch.Label).
				Role(ui.RoleToggleButton).Checked(ch.On)
			switch {
			case b.Pressed():
				b.Background(t.Blue.Alpha(0.20))
			case ch.On:
				b.Background(t.Blue.Alpha(0.16))
			case b.Hovered():
				b.Background(t.Blue.Alpha(0.08))
			}
			b.Children(func() {
				ui.Text(c, ch.Label).FontSize(11).TextColor(pick(ch.On, t.Blue, ink))
			})
			if b.Clicked() {
				ch.Pick()
			}
		}
	})
}

// videoSettingsNote 是面板里的一行小字说明。
func videoSettingsNote(c *ui.Context, t Theme, text string) {
	ui.Text(c, text).FontSize(10).MaxLines(3).
		TextColor(t.text(settingsItemInk.Alpha(0.60))).
		Margin(0, 2, 2, 2)
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
