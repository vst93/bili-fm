package view

// 视频窗（原生播放器外壳）的**视觉令牌**：网格、键、字号、hairline、材质。
//
// 为什么单独一份：视频窗的顶栏 / 控件条 / 设置面板此前各写各的数字 —— 顶栏
// 外边距 10、内边距 8、键 30×30 圆角 6、标题 12.5；控件条外边距 10、内边距
// 12、工具键 32×32 圆角 8、播放键 42 正圆、均衡列 34、时间列 56；面板行高
// 30、字号 12.5/10.5/10。同一块界面上出现 10 / 12 / 30 / 32 / 34 / 40 / 42
// 七套数字，读起来就是「自己拼的控件盒」。用户 2026-11 的要求是「视频播放的
// 界面还需要优化，现在的 UI 看着不够正式」——**不加花活、靠秩序**。
//
// 规则只有四条：
//
//  1. **网格**：所有间距是 4 的倍数（4 / 8 两档步进，12 一档内边距）。顶栏
//     与控件条共用**同一个**外边距与内边距，两栏的内容左缘因此严格对齐。
//  2. **垂直节奏**：顶栏 36、控件条 56（窄窗两行 66），一行内全部键
//     AlignItems(Center)，可点元素共用一条视觉中心线。
//  3. **字号只有三档**：标题 13 / 600、正文 12、次要信息 11；时间码用等宽
//     数字（tabular figures）并给**固定宽度**，播放时不会左右跳。
//  4. **分割**用 1px 低对比 hairline（顶栏下沿、控件条上沿各一条），不靠
//     「更厚的玻璃」建立层次。
//
// 深浅两态都只从这里取色：深色走主题的 Panel / Ink / Blue，浅色把「压在
// 白面板上的淡底」换成墨色低透明度（白压白看不见）。**不许**在视图里再写
// 字面量尺寸或颜色 —— 散落的魔法数字正是这轮要清掉的东西。

import (
	"github.com/egoist/mygo/ui"
)

// ---------------------------------------------------------------- 网格

// 间距只有两档步进：4（同一块内部的细缝）与 8（同一行内相邻元素）。
// 其余间距都由它们组合：12 = 8+4（栏与窗口边缘、栏内左右内边距）。
// **所有**间距都是 4 的倍数。
const (
	videoStep  float32 = 4
	videoStep2 float32 = 8

	// videoInset 是栏与窗口边缘的距离（左右与上下同一值）。顶栏与控件条都
	// 用它，所以两栏的左右缘完全对齐、与窗口边缘的距离也相等。
	videoInset float32 = 12
	// videoPadX 是栏内左右内边距。两栏同值 ⇒ 内容左缘 = videoInset+videoPadX
	// = 24，标题、第一颗键、进度条起点落在同一条竖线上。
	videoPadX float32 = 12
	// videoGap 是同一行内相邻元素之间的距离（键与键、列与列）。
	videoGap float32 = videoStep2
	// videoRowGap 是两行布局里进度条与键行之间的细缝。
	videoRowGap float32 = videoStep
)

// ---------------------------------------------------------------- 高度与圆角

const (
	// videoChromeTitleH 是顶栏高度（36 = 9×4）。键 30 在其中居中，上下各
	// 留 3 —— 键落在一行里，而不是顶着栏边。
	videoChromeTitleH float32 = 36
	// videoChromeBarH 是控件条单行高度：与主窗播放栏同一个 56（两处共用
	// 一份控件代码，高度跟着走，两栏的键才在同一档节奏上）。
	videoChromeBarH float32 = PlayerBarHeight
	// videoChromeBarTwoH 是窄窗两行控件条的高度：4 + 24（进度条）+ 4 +
	// 30（键行）+ 4 = 66。全部由网格与键尺寸推出，不再是手写的 76。
	videoChromeBarTwoH float32 = 3*videoStep + timelineH + videoKeySize
	// videoChromeRadius 是两条栏的圆角，videoPanelRadius 是设置面板的圆角
	// —— 同一档浮层，同一个值（面板挂在栏下面，圆角不一致会读成两块东西）。
	videoChromeRadius float32 = 12
	videoPanelRadius  float32 = videoChromeRadius
)

// ---------------------------------------------------------------- 键

const (
	// videoKeySize / videoKeyRadius 是**这一窗全部图标键**的尺寸与圆角：
	// 顶栏五颗（置顶 / 弹幕 / 设置 / 最小化 / 退出）与控件条里的播放、下一集、
	// 音量、倍速、均衡、跳过、全屏都是 30×30、圆角 6。此前顶栏 30、工具键
	// 32、均衡列 34、播放 42 —— 同一排里四种尺寸，视觉上就是「拼的」。
	//
	// VideoChromeKey 是它的导出名：主窗迷你模式的固定键也是 30×30，两处必须
	// 取同一个值（TestVideoPinKeyMatchesMini 钉住）。
	videoKeySize      float32 = 30
	videoKeyRadius    float32 = 6
	VideoChromeKey    float32 = videoKeySize
	videoKeyTransport float32 = videoKeySize // 播放 / 下一集与图标键同档

	// videoIconSize 是全部线性图标的统一尺寸（16）。icon-park 的 48 画布字形
	// 占 4..44：缩到 16 时墨迹约 13.3px、描边 1.33px；视频窗自己的两枚图标
	//（全屏 / 退出全屏）也画在同一条 48/4 网格上，所以一排键里每枚图标的
	// 描边与墨迹都在同一档。16 画布的自绘字形（iconEQ / iconLyricsGear）描边
	// 是 1.4px，与 1.33px 差 0.07px（不足一像素）—— 光学对齐靠的是**同一条
	// 网格**，不是逐枚挑几何尺寸。
	videoIconSize float32 = 16
	// videoPlayIconSize 是播放 / 暂停的**实心**字形尺寸：实心图形在同样的
	// 盒子里读起来比描边小一档，给它 18（光学对齐，不是几何对齐）。
	videoPlayIconSize float32 = 18
	// videoSpeedKeyW 是「文字值键」（倍速 1.25x）的宽度：它显示 5 个字形，
	// 30 放不下；高度与圆角仍与图标键同档。
	videoSpeedKeyW float32 = 40
	// videoTimeW 是时间码列的固定宽度（4 的倍数）：容纳最长的 h:mm:ss
	// （"12:34:56" 在 11px / 600 / tnum 下量得 45.9），且宽度固定 + 等宽
	// 数字 ⇒ 播放时时间码不会左右跳。
	videoTimeW float32 = 52
	// videoVolW 是宽窗内联音量滑块的宽度。
	videoVolW float32 = 68
	// videoTrackH 是进度条 / 音量条的**视觉**轨道高度（4）。命中区仍是
	// timelineH = 24，远大于要求的 16。
	videoTrackH float32 = 4
)

// ---------------------------------------------------------------- 字号

// 字号只有三档，全部为整数，避免 12.5 / 10.5 这种「看着差不多」的取值。
const (
	// videoFontTitle 是顶栏标题与面板标题：13 / 600。
	videoFontTitle   float32 = 13
	videoTitleWeight int     = 600
	// videoFontBody 是面板里的一行正文（开关名）。
	videoFontBody float32 = 12
	// videoFontMeta 是次要信息：分组标题、说明小字、档位按钮。
	videoFontMeta float32 = 11
	// videoFontTime 是时间码字号；videoTimeLineH 是它的行盒。字重沿用主窗
	// 播放栏那一档（playerTimeWeight = 600），不另开一个。
	videoFontTime  float32 = 11
	videoTimeLineH float32 = 18
)

// ---------------------------------------------------------------- 面板

const (
	// videoSettingsRowH 是面板里每一行的行高（36）：开关行、不可用行、档位
	// 行统一这一个值，行与行之间不靠留白拉开，靠分隔线。
	videoSettingsRowH float32 = 36
	// videoSettingsHeadH 是固定头部（标题 + 关闭键）的高度：与行高同档。
	videoSettingsHeadH float32 = videoSettingsRowH
	// videoChoiceH 是档位按钮的高度（28 = 36 − 上下各 4）。
	videoChoiceH float32 = 28
	// videoPanelPadX / videoPanelPadY 是面板内边距（12 / 12，与栏内边距同档）。
	videoPanelPadX float32 = 12
	videoPanelPadY float32 = 12
	// videoPanelGap 是面板内相邻块之间的距离。
	videoPanelGap float32 = videoGap
	// videoPanelEdge 是面板与窗口边缘的距离。
	videoPanelEdge float32 = videoInset
	// videoPanelW 是面板的目标宽度；窗口更窄时按窗口宽度收。
	videoPanelW float32 = 300
)

// videoSettingsMaxH 是面板这一帧的最大高度：标题栏下方到窗口底，上下各留
// videoPanelGap / videoPanelEdge。
//
// 320×200 的最小窗口下：top = 12+36+8 = 56，max = 200−56−12 = 132；减掉
// 头部 36、间距 8 与上下内边距 24，滚动区 64 —— 够开、够滚、够关。
func videoSettingsMaxH(h float32) float32 {
	top := videoInset + videoChromeTitleH + videoPanelGap
	floor := videoSettingsHeadH + videoPanelGap + videoSettingsRowH + 2*videoPanelPadY
	return maxFloat32(h-top-videoPanelEdge, floor)
}

// videoSettingsScrollH 是面板里滚动区能有的最大高度（面板高减去头部与内边距）。
func videoSettingsScrollH(h float32) float32 {
	return maxFloat32(videoSettingsMaxH(h)-videoSettingsHeadH-videoPanelGap-2*videoPanelPadY,
		videoSettingsRowH)
}

// ---------------------------------------------------------------- hairline

// videoHairline 是两条栏的分割线颜色：**低对比**的一条 1px 线，深浅两态都只
// 比栏底深一点点。它不画纯黑 / 纯白 —— 那种线会把栏切成两块色带，比「更厚的
// 玻璃」更破坏层次。
//
// 浅色是墨 .12（白面板上的一条浅灰线），深色是白 .10（深面板上的一条浅线）。
func videoHairline(t Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#ffffff").Alpha(0.10)
	}
	return t.Ink.Alpha(0.12)
}

// ---------------------------------------------------------------- 材质

// videoBar 给一条浮在画面上的控件条铺材质：应用的面板令牌 + 1px 线 + 投影。
//
// 深浅两态都用 Theme 的令牌：浅色外观是白面板（Ink 深墨），深色外观是深面板
// （Ink 浅墨）。视频画面背后可能是**任何一帧**（黑场与纯白高光都常见），所以
// 这两条栏要的是「厚玻璃」而不是播放栏那种薄玻璃 —— 用户 2026-11 报「还是太
// 透，字糊在视频上」，这里把底色的**不透明度下限**抬到 0.93 / 0.95（见
// videoBarFill）。没抬到 1.0：纯色块会把画面切成三条色带，层次感全失。
func (t Theme) videoBar(bar ui.Element) ui.Element {
	bar.Background(videoBarFill(t)).Border(1, t.Line).InsetShadow(0, 1, 1, 0, t.Highlight)
	if t.Dark {
		bar.Shadow(0, 10, 28, 0, shadowInk.Alpha(0.42))
		return bar
	}
	bar.Shadow(0, 8, 24, 0, shadowInk.Alpha(0.22))
	return bar
}

// videoTitleBar / videoControlBar 是「在对着画面的那条边补 hairline」的入口：
// 顶栏在下沿、控件条在上沿各一条 1px 低对比线。两条线都画在栏内缘（inset
// 阴影），所以跟着圆角走，不会像绝对定位的矩形那样在圆角处探出去。
func (t Theme) videoTitleBar(bar ui.Element) ui.Element {
	return t.videoBar(bar).InsetShadow(0, -1, 0, 0, videoHairline(t))
}

func (t Theme) videoControlBar(bar ui.Element) ui.Element {
	return t.videoBar(bar).InsetShadow(0, 1, 0, 0, videoHairline(t))
}

// 上下两条栏的底材质不透明度下限（浅 / 深）。Panel 令牌只抬下限，色相层次
// 照旧。原值：浅 #ffffff .88、深 #152238 .92。
const (
	videoBarAlpha     float32 = 0.93
	videoBarAlphaDark float32 = 0.95
)

// videoBarFill 是上下两条栏的底色：Panel 令牌 + 不透明度下限。
//
// 它**不**跟着窗口透明度设置变薄：Theme.withAppearance 会按透明度把 Panel 的
// alpha 降下来，这里按下限抬回去。理由：主窗的玻璃可以更透（背后是应用自己的
// 背景），视频窗的栏不行 —— 背后是画面本身。系统「降低透明度」仍然完全生效
// （Panel 在那里已经是 255，下限自然满足）。
func videoBarFill(t Theme) ui.Color {
	f := t.Panel
	floor := videoBarAlpha
	if t.Dark {
		floor = videoBarAlphaDark
	}
	if a := float32(f.A) / 255; a < floor {
		f.A = uint8(floor*255 + 0.5)
	}
	return f
}

// ---------------------------------------------------------------- 键的材质

// videoKeySurfaceColors 是视频窗键的四态颜色：层次明确但克制，每种状态只
// 有一层表达（不用发光、不用大范围阴影）。
//
//	常态    透明底 + Muted 墨
//	悬停    墨 6% 底 + 1px 线 + accent 墨
//	按下    墨 10% 底 + 1px 线 + accent 墨
//	激活    单一 accent：accent 底 + accent 墨（**不**再叠 accent 描边 + 彩色
//	        投影 —— 那正是用户说的「彩色图标 + 玻璃底 + 描边叠三种」）
//
// 深浅两态只差「墨」的方向：深色的 Ink 本身是浅色，同一套 alpha 就是一层亮
// 底，不需要第二套字面量。
func videoKeySurfaceColors(t Theme, active, hovered, pressed bool) (fill, border ui.Color) {
	fill, border = ui.Transparent, ui.Transparent
	switch {
	case active && hovered:
		fill = t.Blue.Alpha(0.20)
	case active:
		fill = t.Blue.Alpha(0.14)
	case pressed:
		fill, border = t.Ink.Alpha(0.10), t.Line
	case hovered:
		fill, border = t.Ink.Alpha(0.06), t.Line
	}
	return fill, border
}

// videoKeyInk 是键的墨色：激活与悬停都用**同一个** accent，常态是 Muted。
// 高对比偏好下换成播放栏那对不透明墨（与主窗键族同一套无障碍取值）。
func videoKeyInk(t Theme, active, hovered bool) ui.Color {
	if t.HighContrast {
		if active || hovered {
			return playerHCActiveInk
		}
		return playerHCInk
	}
	if active || hovered {
		return t.Blue
	}
	return t.Muted
}

// videoKeyShape 把一颗键的四态材质铺上去（底色与描边走 140ms 过渡），返回
// 元素本身。它是视频窗所有键（顶栏 + 控件条）唯一的材质入口。
func videoKeyShape(c *ui.Context, t Theme, b ui.Element, st headerState) ui.Element {
	fill, border := videoKeySurfaceColors(t, st.Selected, st.Hovered, st.Pressed)
	return b.Background(headerFade(c, b, "video-key-fill", fill, headerAnim)).
		Border(1, headerFade(c, b, "video-key-border", border, headerAnim))
}

// videoKeyTitle 是视频窗顶栏 / 控件条里**全部图标键**的唯一画法：30×30、圆角
// 6、图标 16（光学尺寸）、同一套四态材质、同一条 140ms 悬停过渡、按压 0.94、
// 同一圈键盘焦点环。
//
// 它替换了此前顶栏的 playerBarTitleKey 与控件条里的四颗工具键画法 —— 同一类
// 控件两套绘制正是这轮要清掉的重复。
func videoKeyTitle(c *ui.Context, t Theme, key, label, tip string, ic *ui.SVG, active bool) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(videoKeySize, videoKeySize).
		Radius(videoKeyRadius).Center().Label(label).Tooltip(tip)
	st := headerStateOf(b, active)
	b = videoKeyShape(c, t, b, st)
	if st.Hovered && !st.Pressed {
		b = b.InsetShadow(0, 1, 1, 0, t.Highlight)
	}
	pressScale(b)
	b.Children(func() {
		ui.Icon(c, ic).Size(videoIconSize, videoIconSize).
			TextColor(headerFade(c, b, "video-key-ink", videoKeyInk(t, st.Selected, st.Hovered), headerAnim))
	})
	focusOutline(b, videoKeyRadius, t.headerFocusRing())
	return b
}

// videoPlaySurface / videoPlayInk 是视频窗播放键的材质与图标色：播放键是主
// 操作，常态就带一层淡 accent 底（否则白面板上的白键认不出来），三态只在
// accent 的浓度上走 —— 不叠描边、不叠发光、不叠彩色投影。
func videoPlaySurface(t Theme, b ui.Element, hovered, pressed bool) ui.Element {
	fill := t.Blue.Alpha(0.14)
	switch {
	case pressed:
		fill = t.Blue.Alpha(0.10)
	case hovered:
		fill = t.Blue.Alpha(0.22)
	}
	return b.Background(fill).Border(1, ui.Transparent)
}

func videoPlayInk(t Theme, hovered bool) ui.Color {
	if t.HighContrast {
		return playerHCInk
	}
	if hovered {
		return ui.Hex("#0369a1")
	}
	return t.Blue
}

// videoPanelKeys 是视频窗控件条这一帧的键尺寸令牌（顶栏与控件条共用）。
func videoPanelKeys() PlayerKeyMetrics {
	return PlayerKeyMetrics{
		Key:  videoKeySize,
		KeyR: videoKeyRadius,
		Wide: videoKeySize,
		Play: videoKeyTransport, PlayR: videoKeyRadius,
		Icon: videoIconSize, PlayIcon: videoPlayIconSize,
		SpeedW: videoSpeedKeyW,
		// 列宽 = 键宽：键与键之间因此正好是 videoGap。
		ToolSlot: videoKeySize, SpeedSlot: videoSpeedKeyW, SponsorSlot: videoKeySize,
		TransportGap: videoGap,
		TimeW:        videoTimeW, TimeSize: videoFontTime,
		TrackH: videoTrackH,
	}
}

// videoBufferedFrom / videoBufferedTo 是视频窗进度条缓冲层的颜色：一对中性
// 灰蓝，与青→蓝的播放层一眼分色（浅色面板上再深一档，深色面板上再亮一档）。
func videoBufferedFrom(t Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#cbd5e1").Alpha(0.30)
	}
	return ui.Hex("#94a3b8").Alpha(0.42)
}

func videoBufferedTo(t Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#94a3b8").Alpha(0.26)
	}
	return ui.Hex("#64748b").Alpha(0.36)
}
