package view

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// 桌面字幕悬浮窗：显示当前句 + 下一句，右上角齿轮切换到设置页。
// 整个窗口是原生 mygo UI，不是 WebView。

var iconLyricsGear = icon(`<circle cx="8" cy="8" r="2.2"/><path d="M8 1.8v1.4M8 12.8v1.4M1.8 8h1.4M12.8 8h1.4M3.6 3.6l1 1M11.4 11.4l1 1M12.4 3.6l-1 1M4.6 11.4l-1 1"/>`)

func (a *App) LyricsShell(c *ui.Context) {
	// 原生 UI 的根也要显式透明，否则窗口虽然 Transparent，根节点仍会铺一层
	// 主题底色 —— 这正是此前“文字周围一圈白色内边距”的来源。
	c.SetTheme(a.Theme.uiTheme())
	c.Root().Background(ui.Transparent)
	if a.LyricsConfig {
		a.lyricsConfigShell(c)
		return
	}
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
	colorHex := a.LyricsColor
	if len(colorHex) != 7 || colorHex[0] != '#' {
		colorHex = "#ffffff"
	}
	color := ui.Hex(colorHex)
	// 面板保持可拖拽；齿轮只在鼠标进入窗口时出现。整个窗口由面板铺满，
	// 不再留一圈会透出白色根底的外边距；边距只作用在文字内层。
	root := ui.Box(c).Fill().DragWindow()
	_, _, over := root.PointerPosition()
	root.Children(func() {
		panelColor := ui.Hex("#0f172a").Alpha(bgOpacity)
		panel := ui.Box(c).FillWidth().FillHeight().Radius(14).
			Background(panelColor).
			Padding(10, float32(a.LyricsMargin)).Center()
		if bgOpacity > 0 {
			panel.Border(1, ui.Hex("#ffffff").Alpha(0.12*min32(1, bgOpacity*2))).
				InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.16*min32(1, bgOpacity*2))).
				Shadow(0, 10, 30, 4, ui.Hex("#000000").Alpha(0.24*min32(1, bgOpacity*2)))
		}
		panel.Children(func() {
			ui.Column(c).FillWidth().Gap(2).Center().Children(func() {
				curColor := color.Alpha(textOpacity)
				ui.Text(c, cur).FontSize(size).FontWeight(700).
					TextColor(curColor).TextAlign(ui.Center).MaxLines(2)
				if next != "" {
					nextSize := max(float32(11), size*0.65)
					ui.Text(c, next).FontSize(nextSize).
						TextColor(color.Alpha(textOpacity * 0.62)).
						TextAlign(ui.Center).SingleLine()
				}
			})
			// 绝对定位齿轮：鼠标离开窗口即隐藏，不影响文字居中。
			if over {
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

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// lyricsConfigShell 是同一个窗口的设置页。控件全部是原生按钮/滑块。
func (a *App) lyricsConfigShell(c *ui.Context) {
	// 设置页不能用整页 DragWindow：它会吃掉滑块的拖动。只让标题行可拖。
	ui.Box(c).Fill().Children(func() {
		ui.Box(c).FillWidth().FillHeight().Radius(14).
			Background(ui.Hex("#0f172a").Alpha(0.94)).
			Border(1, ui.Hex("#ffffff").Alpha(0.16)).
			Padding(14, 18).Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Row(c).FillWidth().Height(26).AlignItems(ui.Center).DragWindow().Children(func() {
					ui.Text(c, "桌面字幕设置").FontSize(14).FontWeight(700).
						TextColor(ui.Hex("#ffffff"))
					ui.Box(c).Grow(1)
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

				// 颜色：浅色系为主，避免深色文字落在深色玻璃上看不清。
				ui.Row(c).Height(30).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "颜色").FontSize(12).TextColor(ui.Hex("#ffffff").Alpha(0.62)).Width(46).Shrink(0)
					for _, col := range []string{"#ffffff", "#e2e8f0", "#38bdf8", "#facc15", "#fb7185", "#a7f3d0"} {
						col := col
						sw := ui.ButtonBase(c.Key("lyrics-color-"+col)).Size(24, 24).Radius(6).Center().
							Label(col).Tooltip(col).BorderWidth(2).BorderColor(ui.Transparent)
						sw.Background(ui.Hex(col))
						if a.LyricsColor == col {
							sw.BorderColor(ui.Hex("#ffffff"))
						} else if sw.Hovered() {
							sw.BorderColor(ui.Hex("#ffffff").Alpha(0.7))
						}
						if sw.Clicked() {
							a.LyricsColor = col
							if a.Act.UpdateLyricsConfig != nil {
								a.Act.UpdateLyricsConfig()
							}
						}
					}
				})

				a.lyricsSliderRow(c, "字号", &a.LyricsFontSize, 14, 30, 1)
				a.lyricsSliderRow(c, "边距", &a.LyricsMargin, 4, 40, 1)
				a.lyricsSliderRow(c, "文字透明度", &a.LyricsTextOpacity, 0, 1, 0.05)
				a.lyricsSliderRow(c, "背景透明度", &a.LyricsBgOpacity, 0, 1, 0.05)

				ui.Text(c, "设置保存在本机，只影响桌面字幕窗口。").FontSize(11).
					TextColor(ui.Hex("#ffffff").Alpha(0.45))
			})
		})
	})
}

// lyricsSliderRow 是设置页的一行：标签 + 滑块 + 当前值。
func (a *App) lyricsSliderRow(c *ui.Context, label string, value *float64, lo, hi, step float64) {
	changed := false
	ui.Row(c).FillWidth().Height(34).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(ui.Hex("#ffffff").Alpha(0.62)).
			Width(46).Shrink(0)
		// SliderBase 没有外观（只有拖动行为），在设置页看起来像不存在；
		// 用带主题外观的 Slider，用户才能看到轨道/滑块并拖动。
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
