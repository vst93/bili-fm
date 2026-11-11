package view

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

// 桌面字幕悬浮窗：显示当前句 + 下一句，右上角的设置键切换到设置页。
// 整个窗口是原生 mygo UI，不是 WebView。
//
// 设置页不是把显示页替换掉：上方保留一块与显示窗等高的实时预览，下方才是
// 颜色 / 字号 / 宽度 / 透明度 / 微调控件，拖动滑块能立即看到效果。预览高度
// 随字号自适应（见 LyricsDisplayHeight），不再是固定 100px。

var iconLyricsGear = icon(`<circle cx="8" cy="8" r="2.2"/><path d="M8 1.8v1.4M8 12.8v1.4M1.8 8h1.4M12.8 8h1.4M3.6 3.6l1 1M11.4 11.4l1 1M12.4 3.6l-1 1M4.6 11.4l-1 1"/>`)

// 桌面字幕的布局常量。显示窗高度和设置页预览高度都由 LyricsDisplayHeight
// 按字号算出来，这里把内边距 / 行间距 / 字号范围集中在一处，避免窗口尺寸和
// 实际绘制各写一份而漂移。
const (
	lyricsPanelPadY        = float32(10)
	lyricsPanelPadX        = float32(8)
	lyricsLineGap          = float32(2)
	lyricsMaxCurrentLines  = 2
	lyricsMinFontSize      = float32(14)
	lyricsMaxFontSize      = float32(30)
	lyricsFallbackFontSize = float32(20)
	lyricsNextFontRatio    = float32(0.65)
	lyricsNextMinFontSize  = float32(11)
)

// LyricsGearInset / LyricsGearSize 是桌面字幕右上角**设置键**的位置与边长：
// 离窗口右上角的内边距 / 键的边长。（名字沿用 LyricsGear*，但它画的是设置图标，
// 不是齿轮 —— 用户 2026-11-11 指出过这一点，界面文案一律叫「设置键」。）
//
// **app 侧的鼠标穿透热区读同一组数**（app/lyrics.go 的 lyricsGearShapeRect）：
// 穿透打开时这块区域仍然接收指针，设置键永远可点（用户 2026-11-11 的要求）。
// 这两处必须同源，别再各写一份魔数。
const (
	LyricsGearInset float32 = 4
	LyricsGearSize  float32 = 26
)

// lyricsDragStart 是「按住设置键拖动」的起步阈值（DIP）。与 mygo 内部的
// dragStart（4）同一档：手指/鼠标轻微抖动不该把窗口拖走，超过它才算拖动。
const lyricsDragStart float32 = 4

// lyricsDragExceeded 报告这次按下是否已经算「拖动」（而不是点一下）。
// 抽成纯函数便于单测：设置键既是按钮又是拖动手柄，阈值是两者唯一的分界。
func lyricsDragExceeded(dx, dy float32) bool {
	return dx*dx+dy*dy >= lyricsDragStart*lyricsDragStart
}

// LyricsConfigPanelHeight 是设置面板本身需要的高度（不含预览）。控制器用它
// 算设置窗高度；面板是固定的 9 行控件 + 标题 + 说明，改布局时要一起改。
//
// 逐行算：标题 26 + 9×30（颜色 2 行 / 字号 / 宽度 / 透明度 2 行 / 微调 /
// 鼠标穿透 / 记住微调）+ 9×8 间距 + 说明行 ≈ 420（含 14px 上下内边距），
// 所以 440 留了 ~20px 余量。加行时同步加高，否则最后一行会被窗口裁掉。
const LyricsConfigPanelHeight = 440

// lyricsLayoutFontSize 把配置里的字号夹到实际可用的范围。0 / 未设置时用默认
// 20，与 lyricsPreview 的兜底一致；设置页与控制器都用它，窗口高度才不会和
// 绘制用的字号对不上。
func lyricsLayoutFontSize(v float64) float32 {
	size := float32(v)
	if size < lyricsMinFontSize {
		return lyricsFallbackFontSize
	}
	if size > lyricsMaxFontSize {
		return lyricsMaxFontSize
	}
	return size
}

// lyricsNextFontSize 是下一句的字号：当前句的 65%，但不小于 11。
func lyricsNextFontSize(size float32) float32 {
	next := size * lyricsNextFontRatio
	if next < lyricsNextMinFontSize {
		return lyricsNextMinFontSize
	}
	return next
}

// LyricsDisplayHeight 返回某个字号下桌面字幕显示页应有的高度。显示页最多
// 同时画「当前句两行 + 下一句一行」，固定 100px 在大字号时会把第二行裁掉，
// 小字号时又空得难看，所以窗口高度要跟着字号走。
//
// 算式与 lyricsPreview 的布局同源：上下内边距 + 两行当前句 + 行间距 + 一行
// 下一句。自然行盒取框架的字体度量（不同平台可能不同），向上取整避免裁字。
func LyricsDisplayHeight(fontSize float64) int {
	size := lyricsLayoutFontSize(fontSize)
	next := lyricsNextFontSize(size)
	h := 2*lyricsPanelPadY + float32(lyricsMaxCurrentLines)*cjkLineBox(size) +
		lyricsLineGap + cjkLineBox(next)
	if h < 1 {
		return 1
	}
	return int(math.Ceil(float64(h)))
}

func (a *App) LyricsShell(c *ui.Context) {
	// 原生 UI 的根也要显式透明，否则窗口虽然 Transparent，根节点仍会铺一层
	// 主题底色 —— 这正是此前“文字周围一圈白色内边距”的来源。
	c.SetTheme(a.Theme.uiTheme())
	c.Root().Background(ui.Transparent)
	// 预览高度与真实显示窗高度同源：字号越大，预览和窗口一起长高。
	displayH := float32(LyricsDisplayHeight(a.LyricsFontSize))
	if a.LyricsConfig {
		ui.Column(c).Fill().Children(func() {
			// 设置面板默认在预览下方；窗口贴近屏幕底部时由 controller 向上
			// 展开，并把面板画到预览上方，保证面板始终在屏幕内。
			if a.LyricsConfigAbove {
				a.lyricsConfigPanel(c)
				a.lyricsPreview(c, displayH, false)
			} else {
				a.lyricsPreview(c, displayH, false)
				a.lyricsConfigPanel(c)
			}
		})
		return
	}
	a.lyricsPreview(c, 0, true)
}

// lyricsPreview 画显示区。height>0 时用于设置页的固定高度预览；否则铺满
// 整个显示窗口。gear 只在显示模式且鼠标进入窗口时出现。
func (a *App) lyricsPreview(c *ui.Context, height float32, gear bool) {
	cur, next := a.LyricCurrent, a.LyricNext
	if cur == "" && next == "" {
		cur = "♪"
	}
	bgOpacity := float32(a.LyricsBgOpacity)
	if bgOpacity < 0 {
		bgOpacity = 0
	}
	if bgOpacity > 1 {
		bgOpacity = 1
	}
	textOpacity := float32(a.LyricsTextOpacity)
	if textOpacity < 0 {
		textOpacity = 0
	}
	if textOpacity > 1 {
		textOpacity = 1
	}
	size := lyricsLayoutFontSize(a.LyricsFontSize)
	textColor := ui.Hex("#ffffff")
	if len(a.LyricsColor) == 7 && a.LyricsColor[0] == '#' {
		textColor = ui.Hex(a.LyricsColor)
	}
	bgColor := ui.Hex("#0f172a")
	if len(a.LyricsBgColor) == 7 && a.LyricsBgColor[0] == '#' {
		bgColor = ui.Hex(a.LyricsBgColor)
	}
	// 显示模式的根铺满窗口；设置页的预览固定 100px。根可拖动，面板也
	// 可拖动（面板铺满窗口时根本身没有剩余命中区）。
	draw := func(root ui.Element) {
		_, _, over := root.PointerPosition()
		root.Children(func() {
			panel := ui.Box(c).FillWidth().FillHeight().Radius(14).
				Background(bgColor.Alpha(bgOpacity)).
				DragWindow().
				Padding(lyricsPanelPadY, lyricsPanelPadX).Center()
			// 面板铺满整个窗口，**不能给它画外投影**：投影只能在窗口矩形内绘制，
			// 被裁掉的那一半全落在窗口内侧 —— 四边各出现一圈由深到浅的暗边，
			// 背景透明度调到 0 时尤其明显（用户报的「最外围还有边框和阴影感」）。
			// 窗口本身的系统投影由控制器按背景透明度开关（Windows 上的 DWM 边框
			// 也由 lyricsTuneWindowFrame 关掉），内容层这里一律不画。
			//
			// 常态也不画描边/内高光：Wayland 下这圈白线会被合成器衬成一圈灰边，
			// 看起来像未选中的默认边框。只在鼠标进入时画选中蓝边（提示字幕窗的
			// 位置和可设置区域，背景透明时也能看见）。
			if over {
				// 鼠标进入窗口时给一圈选中边框；背景透明度调到 0 时也能看出
				// 字幕窗的位置和可设置区域。
				panel.Border(1, ui.Hex("#7dd3fc").Alpha(0.62)).
					InsetShadow(0, 0, 0, 1, ui.Hex("#7dd3fc").Alpha(0.28))
			}
			panel.Children(func() {
				ui.Column(c).FillWidth().Gap(lyricsLineGap).Center().Children(func() {
					ui.Text(c, cur).FontSize(size).FontWeight(700).
						TextColor(textColor.Alpha(textOpacity)).TextAlign(ui.Center).MaxLines(lyricsMaxCurrentLines).
						TextShadow(0, 1, lyricsTextShadowBlur, lyricsTextShadowInk(textColor, bgOpacity, textOpacity))
					if next != "" {
						nextSize := lyricsNextFontSize(size)
						ui.Text(c, next).FontSize(nextSize).
							TextColor(textColor.Alpha(textOpacity*0.62)).
							TextAlign(ui.Center).SingleLine().
							TextShadow(0, 1, lyricsTextShadowBlur, lyricsTextShadowInk(textColor, bgOpacity, textOpacity*0.62))
					}
				})
				// 设置键：指针在窗内（或指针正压在设置键热区上）时出现。
				//
				// 穿透打开时 `over` 读不到（窗口收不到鼠标事件），所以另有一条
				// `LyricsGearHot`：整窗穿透的平台由 app 侧的指针轮询维护它；
				// Linux 用输入区域，指针一进那块区域窗口就重新收到事件、`over`
				// 自然为真。两条都只表示「指针在设置键附近」，**不会**让键常显
				// —— 用户 2026-11-11 报「开启穿透后设置键不再自动隐藏」。
				if gear && (over || a.LyricsGearHot) {
					ui.Box(c).Absolute().Right(LyricsGearInset).Top(LyricsGearInset).Children(func() {
						b := ui.ButtonBase(c.Key("lyrics-config")).Size(LyricsGearSize, LyricsGearSize).Radius(7).Center().
							Label("桌面字幕设置").Tooltip("桌面字幕设置（可按住拖动整个字幕窗）")
						// 按住拖动 = 拖动整个字幕窗：设置键本身是按钮（可点击进设置页），
						// 拖动由 app 侧在 `Dragged()` 报「移动超过阈值」时调
						// `Window.StartDrag` 交给合成器 —— mygo 的 `DragWindow()`
						// 只在元素**没有**其它交互标志时生效（input.go 的
						// `interactive&^flagDragWindow == 0`），按钮拿不到它。
						b.Background(ui.Hex("#ffffff").Alpha(0.18))
						if b.Hovered() || b.Pressed() {
							b.Background(ui.Hex("#ffffff").Alpha(0.30))
						}
						b.Children(func() {
							ui.Icon(c, iconLyricsGear).Size(15, 15).TextColor(ui.Hex("#ffffff").Alpha(0.9))
						})
						if b.Clicked() && a.Act.ToggleLyricsConfig != nil {
							a.Act.ToggleLyricsConfig()
						}
						// 按住拖动 = 拖动整个字幕窗。Dragged 报的是**累积**位移
						// （mygo 每帧 `s.dragX += dx`，forgetInput 才清零），所以
						// 交给 app 的是「自上次以来新增的那段」，逐帧移动窗口才不
						// 会按累积量叠加成飞出去。
						if dx, dy, held := b.Dragged(); held {
							if lyricsDragExceeded(dx, dy) && a.Act.DragLyricsWindow != nil {
								a.Act.DragLyricsWindow(dx-a.lyricsDragDX, dy-a.lyricsDragDY)
								a.lyricsDragDX, a.lyricsDragDY = dx, dy
							}
						} else {
							// 松手（或没按）：清掉增量基准与「已交给合成器」的闩锁。
							a.lyricsDragDX, a.lyricsDragDY = 0, 0
							if a.Act.EndLyricsDrag != nil {
								a.Act.EndLyricsDrag()
							}
						}
					})
				}
			})
		})
	}
	if height > 0 {
		winW, _ := c.Size()
		actual := float32(a.LyricsPixelWidth)
		if actual <= 0 {
			actual = 560
		}
		// 预览必须能表达真实宽度：设置页固定 560 时，按比例缩小整个预览，
		// 而不是把宽度截到 560。这样配置大宽度也能看到完整字幕框。
		scale := float32(1)
		if avail := winW - 8; winW > 8 && actual > avail {
			scale = avail / actual
		}
		outer := ui.Box(c).FillWidth().Height(height).Clip()
		outer.Children(func() {
			left := (winW - actual*scale) / 2
			top := (height - height*scale) / 2
			root := ui.Box(c).Absolute().Left(left).Top(top).
				Width(actual).Height(height).Scale(scale, scale).TransformOrigin(0, 0).
				DragWindow()
			draw(root)
		})
		return
	}
	root := ui.Box(c).Fill().DragWindow()
	draw(root)
}

// lyricsTextShadowBlur 是桌面字幕文字投影的模糊半径（DIP）。字幕窗背景
// 透明度可以调到 0（纯文字浮在任意画面上），那时白字压在浅色内容上几乎看不见；
// 一层很淡的深色投影把字形从背景里托起来。
const lyricsTextShadowBlur float32 = 4

// lyricsTextShadowInk 是字幕文字投影的颜色。
//
// 只在**需要**的时候画：
//   - 面板底色够实（bgOpacity ≥ .55）时不需要 —— 字幕本来就有底，再画投影
//     只会让字变糊（这是原版的做法：面板不透明时文字就是纯色）。
//   - 文字颜色本身很深（灰阶 ≤ .5）时不需要 —— 深色字压在浅色画面上已经够
//     清楚，再加深色投影反而糊成一团。
//
// 其余情况（透明/半透明面板 + 浅色字）给深色投影，alpha 随文字透明度与
// 「面板有多透明」一起走：文字越淡、面板越透，投影越实一点才托得住。
func lyricsTextShadowInk(text ui.Color, bgOpacity, textOpacity float32) ui.Color {
	if bgOpacity >= 0.55 || lyricsTextIsDark(text) || textOpacity <= 0 {
		return ui.Transparent
	}
	// 面板越透明、文字越淡 → 投影越实。上限 .55（再重就成了描边）。
	alpha := 0.55 * (1 - bgOpacity) * textOpacity
	if alpha > 0.55 {
		alpha = 0.55
	}
	return ui.Hex("#000000").Alpha(alpha)
}

// lyricsTextIsDark 报告字幕文字颜色是不是深色（感知亮度 ≤ 50%）。
// 用与本仓库其它地方一致的 YIQ 系数（见 lum / lumaSum），不依赖框架内部实现。
func lyricsTextIsDark(c ui.Color) bool {
	return (int(c.R)*299+int(c.G)*587+int(c.B)*114)/1000 <= 127
}

// lyricsConfigPanel 是设置页的下半部分，和上方预览同时存在。
func (a *App) lyricsConfigPanel(c *ui.Context) {
	ui.Box(c).FillWidth().Grow(1).Children(func() {
		ui.Box(c).FillWidth().FillHeight().Radius(14).
			Background(ui.Hex("#0f172a").Alpha(0.94)).
			Border(1, ui.Hex("#ffffff").Alpha(0.16)).
			Padding(14, 18).Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Row(c).FillWidth().Height(26).AlignItems(ui.Center).DragWindow().Children(func() {
					ui.Text(c, "桌面字幕设置").FontSize(14).FontWeight(700).
						TextColor(ui.Hex("#ffffff"))
					ui.Box(c).Grow(1)
					reset := ui.ButtonBase(c.Key("lyrics-config-reset")).Height(26).Padding(0, 8).
						Radius(7).Center().Label("恢复默认").Tooltip("恢复桌面字幕全部默认值")
					reset.Background(ui.Hex("#ffffff").Alpha(0.08))
					if reset.Hovered() {
						reset.Background(ui.Hex("#ffffff").Alpha(0.18))
					}
					reset.Children(func() { ui.Text(c, "恢复默认").FontSize(12).TextColor(ui.Hex("#ffffff").Alpha(0.78)) })
					if reset.Clicked() && a.Act.ResetLyricsConfig != nil {
						a.Act.ResetLyricsConfig()
					}
					done := ui.ButtonBase(c.Key("lyrics-config-done")).Height(26).Padding(0, 10).
						Radius(7).Center().Label("完成").Tooltip("完成")
					done.Background(ui.Hex("#ffffff").Alpha(0.12))
					if done.Hovered() {
						done.Background(ui.Hex("#ffffff").Alpha(0.24))
					}
					done.Children(func() { ui.Text(c, "完成").FontSize(12).TextColor(ui.Hex("#ffffff")) })
					if done.Clicked() && a.Act.ToggleLyricsConfig != nil {
						a.Act.ToggleLyricsConfig()
					}
				})
				a.lyricsColorRow(c, "文字", DefaultLyricsColor, &a.LyricsColor, []string{
					"#ffffff", "#e2e8f0", "#38bdf8", "#facc15", "#fb7185", "#a7f3d0", "#0f172a",
				})
				a.lyricsColorRow(c, "背景", DefaultLyricsBgColor, &a.LyricsBgColor, []string{
					"#0f172a", "#1e293b", "#111827", "#000000", "#ffffff", "#f8fafc", "#1d4ed8",
				})
				a.lyricsSliderRow(c, "字号", fmt.Sprintf("%.0f", DefaultLyricsFontSize),
					&a.LyricsFontSize, 14, 30, 1)
				a.lyricsWidthRow(c)
				a.lyricsSliderRow(c, "文字透明度", fmt.Sprintf("%.1f", DefaultLyricsTextOpacity),
					&a.LyricsTextOpacity, 0, 1, 0.05)
				a.lyricsSliderRow(c, "背景透明度", fmt.Sprintf("%.2f", DefaultLyricsBgOpacity),
					&a.LyricsBgOpacity, 0, 1, 0.05)
				a.lyricsOffsetRow(c)
				a.lyricsClickThroughRow(c)
				a.lyricsRememberRow(c)
				ui.Text(c, "正数提前、负数延后；上方预览会实时更新。").FontSize(11).
					TextColor(ui.Hex("#ffffff").Alpha(0.45))
			})
		})
	})
}

// lyricsColorRow 是一排颜色预设。选中的色块用蓝色外环 + 勾，白色选中时
// 也不会看不出来。
func (a *App) lyricsColorRow(c *ui.Context, label, def string, current *string, colors []string) {
	ui.Row(c).Height(30).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label+" · "+def).FontSize(11).TextColor(ui.Hex("#ffffff").Alpha(0.54)).Width(112).Shrink(0)
		for _, col := range colors {
			col := col
			sw := ui.ButtonBase(c.Key("lyrics-color-"+label+"-"+col)).Size(24, 24).Radius(6).Center().
				Label(col).Tooltip(col).BorderWidth(2).BorderColor(ui.Transparent)
			sw.Background(ui.Hex(col))
			selected := *current == col
			if selected {
				sw.BorderColor(ui.Hex("#0ea5e9"))
			} else if sw.Hovered() {
				sw.BorderColor(ui.Hex("#ffffff").Alpha(0.72))
			}
			sw.Children(func() {
				if selected {
					// 勾用对比色：白底用深色勾，深底用白勾。
					ink := ui.Hex("#ffffff")
					if col == "#ffffff" || col == "#f8fafc" || col == "#e2e8f0" || col == "#a7f3d0" {
						ink = ui.Hex("#0f172a")
					}
					ui.Icon(c, iconCheck).Size(13, 13).TextColor(ink)
				}
			})
			if sw.Clicked() {
				*current = col
				if a.Act.UpdateLyricsConfig != nil {
					a.Act.UpdateLyricsConfig()
				}
			}
		}
	})
}

// lyricsWidthRow 直接配置字幕窗宽度占屏幕工作区的比例。此前叫「边距」，
// 实际只是把面板向内缩，用户看起来更像在调宽度；现在改成真正的宽度。
func (a *App) lyricsWidthRow(c *ui.Context) {
	percent := a.LyricsWidthRatio * 100
	changed := false
	ui.Row(c).FillWidth().Height(30).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "宽度 · "+fmt.Sprintf("%.0f%%", DefaultLyricsWidthRatio*100)).
			FontSize(11).TextColor(ui.Hex("#ffffff").Alpha(0.54)).
			Width(112).Shrink(0)
		s := ui.Slider(c, &percent, 20, 80).FillWidth().Grow(1).Step(1)
		if s.Changed() {
			changed = true
		}
		ui.Text(c, fmt.Sprintf("%.0f%%", percent)).FontSize(11).
			TextColor(ui.Hex("#ffffff").Alpha(0.62)).Width(50).Shrink(0).
			TextAlign(ui.Center)
	})
	if changed {
		a.LyricsWidthRatio = percent / 100
		if a.Act.UpdateLyricsConfig != nil {
			a.Act.UpdateLyricsConfig()
		}
	}
}

// lyricsOffsetRow 是桌面字幕整体时间微调：正数让字幕提前，负数延后。
func (a *App) lyricsOffsetRow(c *ui.Context) {
	changed := false
	ui.Row(c).FillWidth().Height(30).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "微调 · "+lyricsOffsetLabel(DefaultLyricsOffset)).
			FontSize(11).TextColor(ui.Hex("#ffffff").Alpha(0.54)).
			Width(112).Shrink(0)
		s := ui.Slider(c, &a.LyricsOffset, -10, 10).FillWidth().Grow(1).Step(0.1)
		if s.Changed() {
			changed = true
		}
		ui.Text(c, lyricsOffsetLabel(a.LyricsOffset)).FontSize(11).
			TextColor(ui.Hex("#ffffff").Alpha(0.62)).Width(50).Shrink(0).
			TextAlign(ui.Center)
	})
	if changed && a.Act.UpdateLyricsConfig != nil {
		a.Act.UpdateLyricsConfig()
	}
}

// lyricsOffsetLabel 把微调值写成 +1.2s / -1.2s / 0.0s。
func lyricsOffsetLabel(v float64) string {
	if v > 0 {
		return fmt.Sprintf("+%.1fs", v)
	}
	if v < 0 {
		return fmt.Sprintf("-%.1fs", -v)
	}
	return "0.0s"
}

// lyricsRememberRow 是「记住微调」开关。默认不记住：未勾选时切换视频会
// 归零；勾选后跨视频、重启应用都保留。
func (a *App) lyricsRememberRow(c *ui.Context) {
	b := ui.ButtonBase(c.Key("lyrics-offset-remember")).FillWidth().Height(30).
		Radius(6).Role(ui.RoleSwitch).Label("记住微调").Tooltip("记住微调").
		Checked(a.LyricsOffsetRemember)
	if b.Hovered() {
		b.Background(ui.Hex("#ffffff").Alpha(0.10))
	}
	b.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "记住微调 · 默认关").FontSize(11).
				TextColor(ui.Hex("#ffffff").Alpha(0.54)).Width(112).Shrink(0)
			ui.Text(c, "跨视频和重启保留").FontSize(11).
				TextColor(ui.Hex("#ffffff").Alpha(0.38)).Grow(1)
			incognitoTrack(c, a.LyricsOffsetRemember)
		})
	})
	if b.Clicked() {
		a.LyricsOffsetRemember = !a.LyricsOffsetRemember
		if a.Act.UpdateLyricsConfig != nil {
			a.Act.UpdateLyricsConfig()
		}
	}
}

// lyricsClickThroughRow 是设置页里的「鼠标穿透」开关（用户 2026-11-11 要求
// 「穿透设置在它的设置页面也要有」）。
//
// 行为与主窗设置菜单 / macOS 原生菜单里的那一项同源（同一个
// Act.ToggleLyricsClickThrough）：穿透打开时**设置键仍然可点**（见 app/lyrics.go
// 的 lyricsGearShapeRect 与输入区域 / 轮询），所以从字幕窗本身就能把它关掉。
func (a *App) lyricsClickThroughRow(c *ui.Context) {
	b := ui.ButtonBase(c.Key("lyrics-click-through")).FillWidth().Height(30).
		Radius(6).Role(ui.RoleSwitch).Label("鼠标穿透").Tooltip("鼠标穿透").
		Checked(a.LyricsClickThrough)
	if b.Hovered() {
		b.Background(ui.Hex("#ffffff").Alpha(0.10))
	}
	b.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "鼠标穿透 · 默认关").FontSize(11).
				TextColor(ui.Hex("#ffffff").Alpha(0.54)).Width(112).Shrink(0)
			ui.Text(c, "打开后字幕窗不吃鼠标，设置键仍可点").FontSize(11).
				TextColor(ui.Hex("#ffffff").Alpha(0.38)).Grow(1)
			incognitoTrack(c, a.LyricsClickThrough)
		})
	})
	if b.Clicked() && a.Act.ToggleLyricsClickThrough != nil {
		a.Act.ToggleLyricsClickThrough()
	}
}

// lyricsSliderRow 是设置页的一行：标签 + 可见的原生 Slider + 当前值。
func (a *App) lyricsSliderRow(c *ui.Context, label, def string, value *float64, lo, hi, step float64) {
	changed := false
	ui.Row(c).FillWidth().Height(30).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label+" · "+def).FontSize(11).TextColor(ui.Hex("#ffffff").Alpha(0.54)).
			Width(112).Shrink(0)
		// SliderBase 没有外观（只有拖动行为），设置页看起来像不能配置；
		// 用带主题外观的原生 Slider。
		s := ui.Slider(c, value, lo, hi).FillWidth().Grow(1).Step(step)
		if s.Changed() {
			changed = true
		}
		ui.Text(c, fmt.Sprintf("%.1f", *value)).FontSize(11).
			TextColor(ui.Hex("#ffffff").Alpha(0.62)).Width(42).Shrink(0).
			TextAlign(ui.Center)
	})
	if changed && a.Act.UpdateLyricsConfig != nil {
		a.Act.UpdateLyricsConfig()
	}
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
