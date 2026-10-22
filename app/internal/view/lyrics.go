package view

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// 桌面字幕悬浮窗：显示当前句 + 下一句，右上角齿轮切换到设置页。
// 整个窗口是原生 mygo UI，不是 WebView。
//
// 设置页不是把显示页替换掉：上方固定保留 100px 的实时预览，下方才是
// 颜色 / 字号 / 宽度 / 透明度 / 微调控件，拖动滑块能立即看到效果。

var iconLyricsGear = icon(`<circle cx="8" cy="8" r="2.2"/><path d="M8 1.8v1.4M8 12.8v1.4M1.8 8h1.4M12.8 8h1.4M3.6 3.6l1 1M11.4 11.4l1 1M12.4 3.6l-1 1M4.6 11.4l-1 1"/>`)

func (a *App) LyricsShell(c *ui.Context) {
	// 原生 UI 的根也要显式透明，否则窗口虽然 Transparent，根节点仍会铺一层
	// 主题底色 —— 这正是此前“文字周围一圈白色内边距”的来源。
	c.SetTheme(a.Theme.uiTheme())
	c.Root().Background(ui.Transparent)
	if a.LyricsConfig {
		ui.Column(c).Fill().Children(func() {
			// 设置面板默认在预览下方；窗口贴近屏幕底部时由 controller 向上
			// 展开，并把面板画到预览上方，保证面板始终在屏幕内。
			if a.LyricsConfigAbove {
				a.lyricsConfigPanel(c)
				a.lyricsPreview(c, 100, false)
			} else {
				a.lyricsPreview(c, 100, false)
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
	size := float32(a.LyricsFontSize)
	if size < 14 {
		size = 20
	}
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
				Padding(10, 8).Center()
			if bgOpacity > 0 {
				// 常态不画描边/内高光：Wayland 下这圈白线会被合成器衬成一圈
				// 灰边，看起来像未选中的默认边框。只在鼠标进入时画选中蓝边。
				panel.Shadow(0, 10, 30, 4, ui.Hex("#000000").Alpha(0.24*min32(1, bgOpacity*2)))
			}
			if over {
				// 鼠标进入窗口时给一圈选中边框；背景透明度调到 0 时也能看出
				// 字幕窗的位置和可设置区域。
				panel.Border(1, ui.Hex("#7dd3fc").Alpha(0.62)).
					InsetShadow(0, 0, 0, 1, ui.Hex("#7dd3fc").Alpha(0.28))
			}
			panel.Children(func() {
				ui.Column(c).FillWidth().Gap(2).Center().Children(func() {
					ui.Text(c, cur).FontSize(size).FontWeight(700).
						TextColor(textColor.Alpha(textOpacity)).TextAlign(ui.Center).MaxLines(2)
					if next != "" {
						nextSize := max(float32(11), size*0.65)
						ui.Text(c, next).FontSize(nextSize).
							TextColor(textColor.Alpha(textOpacity * 0.62)).
							TextAlign(ui.Center).SingleLine()
					}
				})
				if gear && over {
					ui.Box(c).Absolute().Right(4).Top(4).Children(func() {
						b := ui.ButtonBase(c.Key("lyrics-config")).Size(26, 26).Radius(7).Center().
							Label("桌面字幕设置").Tooltip("桌面字幕设置")
						b.Background(ui.Hex("#ffffff").Alpha(0.18))
						if b.Hovered() {
							b.Background(ui.Hex("#ffffff").Alpha(0.30))
						}
						b.Children(func() {
							ui.Icon(c, iconLyricsGear).Size(15, 15).TextColor(ui.Hex("#ffffff").Alpha(0.9))
						})
						if b.Clicked() && a.Act.ToggleLyricsConfig != nil {
							a.Act.ToggleLyricsConfig()
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
