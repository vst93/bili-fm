package view

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// 桌面字幕悬浮窗：显示当前句 + 下一句，右上角齿轮切换到设置页。
// 整个窗口是原生 mygo UI，不是 WebView。

var iconLyricsGear = icon(`<circle cx="8" cy="8" r="2.2"/><path d="M8 1.8v1.4M8 12.8v1.4M1.8 8h1.4M12.8 8h1.4M3.6 3.6l1 1M11.4 11.4l1 1M12.4 3.6l-1 1M4.6 11.4l-1 1"/>`)

func (a *App) LyricsShell(c *ui.Context) {
	if a.LyricsConfig {
		a.lyricsConfigShell(c)
		return
	}
	cur, next := a.LyricCurrent, a.LyricNext
	if cur == "" && next == "" {
		cur = "♪"
	}
	opacity := float32(a.LyricsOpacity)
	if opacity < 0.3 {
		opacity = 0.3
	}
	if opacity > 1 {
		opacity = 1
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
	// 面板保持可拖拽；齿轮是唯一交互元素，叠在右上角。
	ui.Box(c).Fill().Padding(6, float32(a.LyricsMargin)).DragWindow().Children(func() {
		ui.Box(c).FillWidth().FillHeight().Radius(16).
			Background(ui.Hex("#0f172a").Alpha(0.72*opacity)).
			Border(1, ui.Hex("#ffffff").Alpha(0.14)).
			InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.18)).
			Shadow(0, 10, 30, 4, ui.Hex("#000000").Alpha(0.24)).
			Padding(10, 16).Center().Children(func() {
			ui.Column(c).FillWidth().Gap(2).Center().Children(func() {
				ui.Text(c, cur).FontSize(size).FontWeight(700).
					TextColor(color).TextAlign(ui.Center).MaxLines(2)
				if next != "" {
					nextSize := max(float32(11), size*0.65)
					ui.Text(c, next).FontSize(nextSize).
						TextColor(color.Alpha(0.56 * opacity)).
						TextAlign(ui.Center).SingleLine()
				}
			})
			// 绝对定位齿轮：挂在面板内容上，不影响文字居中。
			ui.Box(c).Absolute().Right(4).Top(4).Children(func() {
				b := ui.ButtonBase(c.Key("lyrics-config")).Size(26, 26).Radius(7).Center().
					Label("桌面字幕设置").Tooltip("桌面字幕设置")
				b.Background(ui.Hex("#ffffff").Alpha(0.12))
				if b.Hovered() {
					b.Background(ui.Hex("#ffffff").Alpha(0.24))
				}
				b.Children(func() {
					ui.Icon(c, iconLyricsGear).Size(15, 15).TextColor(ui.Hex("#ffffff").Alpha(0.82))
				})
				if b.Clicked() && a.Act.ToggleLyricsConfig != nil {
					a.Act.ToggleLyricsConfig()
				}
			})
		})
	})
}

// lyricsConfigShell 是同一个窗口的设置页。控件全部是原生按钮/滑块。
func (a *App) lyricsConfigShell(c *ui.Context) {
	ui.Box(c).Fill().Padding(8).DragWindow().Children(func() {
		ui.Box(c).FillWidth().FillHeight().Radius(16).
			Background(ui.Hex("#0f172a").Alpha(0.94)).
			Border(1, ui.Hex("#ffffff").Alpha(0.16)).
			Padding(14, 18).Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Row(c).FillWidth().Height(26).AlignItems(ui.Center).Children(func() {
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
				a.lyricsSliderRow(c, "透明度", &a.LyricsOpacity, 0.3, 1, 0.05)

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
		s := ui.SliderBase(c, value, lo, hi).FillWidth().Grow(1).Step(step)
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
