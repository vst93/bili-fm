package view

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// speedOptions 是倍速可选项，与旧版的档位一致。
var speedOptions = []float64{0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 2.5, 3.0}

// playerBar 是底部播放栏：烟熏玻璃条，深色时段自动换令牌。
func (a *App) playerBar(c *ui.Context) {
	t := a.Theme
	ui.Column(c).FillWidth().Shrink(0).Padding(PlayerBarPad, 14, PlayerBarPad, 14).Children(func() {
		bar := ui.Column(c).FillWidth().Radius(Radius).Gap(6).Padding(8, 10).
			Background(t.PlayerSurface).Border(1, t.PlayerBorder)
		bar.Children(func() {
			a.playerTopRow(c)
			a.playerBottomRow(c)
		})
	})
}

// playerTopRow：封面、标题、上一首/播放/下一首、进度、时间。
func (a *App) playerTopRow(c *ui.Context) {
	t := a.Theme
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		// 封面
		ui.Box(c).Size(40, 40).Shrink(0).Radius(RadiusSmall).
			Background(t.CoverPlaceholder()).Clip().Children(func() {
			if a.Track != nil {
				if bmp := a.Images.Bitmap(a.Track.Cover); bmp != nil {
					ui.Image(c, bmp).Fill().Fit(ui.Cover)
				}
			}
		})

		// 标题 / UP 主
		ui.Column(c).Width(190).Shrink(0).Gap(2).Children(func() {
			title := "未在播放"
			if a.Track != nil {
				title = a.Track.Title
			}
			ui.Text(c, title).FontSize(12).TextColor(t.Ink).SingleLine()
			sub := "空格 暂停 / 播放　← → 上一集 / 下一集"
			if a.Track != nil && a.Track.Up != "" {
				sub = a.Track.Up
				if a.Track.Part != "" {
					sub += " · " + a.Track.Part
				}
			}
			ui.Text(c, sub).FontSize(10).TextColor(t.Faint).SingleLine()
		})

		a.transportButtons(c)

		// 进度
		ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmtTime(a.Pos)).FontSize(10).TextColor(t.Faint)
			hi := a.Dur
			if hi <= 0 {
				hi = 1
			}
			if !a.seeking {
				a.SeekValue = a.Pos
			}
			s := ui.Slider(c, &a.SeekValue, 0, hi).Grow(1).Label("播放进度")
			if s.Changed() {
				a.seeking = true
			}
			if s.Submitted() || (a.seeking && !s.Dragging()) {
				if a.seeking {
					a.seeking = false
					if a.Act.Seek != nil {
						a.Act.Seek(a.SeekValue)
					}
				}
			}
			ui.Text(c, fmtTime(a.Dur)).FontSize(10).TextColor(t.Faint)
		})
	})
}

// transportButtons 是上一首 / 播放暂停 / 下一首。
func (a *App) transportButtons(c *ui.Context) {
	t := a.Theme
	ui.Row(c).Gap(6).Shrink(0).AlignItems(ui.Center).Children(func() {
		prev := ui.ButtonBase(c.Key("prev")).Size(30, 28).Radius(RadiusSmall).Center().
			Label("上一集").Tooltip("上一集")
		if prev.Hovered() {
			prev.Background(t.GlassHover)
		} else {
			prev.Background(t.Glass)
		}
		prev.Children(func() { ui.Icon(c, iconPrev).Size(14, 14).TextColor(t.Muted) })
		if prev.Clicked() && a.Act.Prev != nil {
			a.Act.Prev()
		}

		play := ui.ButtonBase(c.Key("play")).Size(40, 28).Radius(RadiusSmall).Center().
			Label(pick(a.Playing, "暂停", "播放")).Tooltip(pick(a.Playing, "暂停", "播放"))
		play.Background(t.Blue.Alpha(0.28)).Border(1, t.Blue.Alpha(0.45))
		play.Children(func() {
			if a.Playing {
				ui.Icon(c, iconPause).Size(14, 14).TextColor(t.Ink)
			} else {
				ui.Icon(c, iconPlay).Size(14, 14).TextColor(t.Ink)
			}
		})
		if play.Clicked() && a.Act.TogglePlay != nil {
			a.Act.TogglePlay()
		}

		next := ui.ButtonBase(c.Key("next")).Size(30, 28).Radius(RadiusSmall).Center().
			Label("下一集").Tooltip("下一集")
		if next.Hovered() {
			next.Background(t.GlassHover)
		} else {
			next.Background(t.Glass)
		}
		next.Children(func() { ui.Icon(c, iconNext).Size(14, 14).TextColor(t.Muted) })
		if next.Clicked() && a.Act.Next != nil {
			a.Act.Next()
		}
	})
}

// playerBottomRow：倍速、均衡、跳过赞助、弹幕、分集、视频、音量。
func (a *App) playerBottomRow(c *ui.Context) {
	t := a.Theme
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(40).Shrink(0) // 与上一行的封面左对齐
		a.chip(c, "speed", fmt.Sprintf("%.2gx", a.Speed), iconSpeed, a.ShowSpeed, func() {
			a.ShowSpeed = !a.ShowSpeed
		})
		a.chip(c, "eq", "均衡", iconEQ, a.EQ, func() {
			if a.Act.ToggleEQ != nil {
				a.Act.ToggleEQ()
			}
		})
		a.chip(c, "sponsor", "跳过", iconSponsor, a.Sponsor, func() {
			if a.Act.ToggleSponsor != nil {
				a.Act.ToggleSponsor()
			}
		})
		a.chip(c, "danmaku", "弹幕", iconDanmaku, a.ShowDanmaku, func() {
			if a.Act.ToggleDanmaku != nil {
				a.Act.ToggleDanmaku()
			}
		})
		a.chip(c, "parts", "分集", iconList, a.ShowParts, func() {
			if a.Act.OpenParts != nil && a.Track != nil {
				a.Act.OpenParts(*a.Track)
			}
		})
		a.chip(c, "video", "视频", iconVideo, a.VideoOpen, func() {
			if a.VideoOpen {
				if a.Act.CloseVideo != nil {
					a.Act.CloseVideo()
				}
			} else if a.Act.OpenVideo != nil {
				a.Act.OpenVideo()
			}
		})

		ui.Box(c).Grow(1)

		// 音量
		ui.Row(c).Width(140).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconVolume).Size(13, 13).TextColor(t.Faint)
			v := a.Volume
			s := ui.Slider(c, &v, 0, 1).Grow(1).Label("音量")
			if s.Changed() {
				a.Volume = v
				if a.Act.SetVolume != nil {
					a.Act.SetVolume(v)
				}
			}
		})
	})

	// 倍速菜单：点「倍速」展开一排档位。
	if a.ShowSpeed {
		ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(40).Shrink(0)
			for _, opt := range speedOptions {
				active := abs(opt-a.Speed) < 0.001
				b := ui.ButtonBase(c.Key(fmt.Sprintf("rate-%v", opt))).Height(22).Padding(0, 10).
					Radius(RadiusSmall).Center().Label(fmt.Sprintf("%.2gx", opt))
				switch {
				case active:
					b.Background(t.Blue.Alpha(0.35)).Border(1, t.Blue.Alpha(0.55))
				case b.Hovered():
					b.Background(t.GlassHover)
				default:
					b.Background(t.Glass)
				}
				b.Children(func() {
					ui.Text(c, fmt.Sprintf("%.2gx", opt)).FontSize(10).
						TextColor(pick(active, t.Ink, t.Muted))
				})
				if b.Clicked() {
					a.Speed = opt
					a.ShowSpeed = false
					if a.Act.SetSpeed != nil {
						a.Act.SetSpeed(opt)
					}
				}
			}
		})
	}
}

// chip 是一个可切换的小按钮（图标 + 文字），激活时高亮。
func (a *App) chip(c *ui.Context, key, label string, ic *ui.SVG, active bool, onClick func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("chip-"+key)).Height(24).Padding(0, 9).Radius(RadiusSmall).
		Center().Label(label).Tooltip(label)
	switch {
	case active:
		b.Background(t.Blue.Alpha(0.30)).Border(1, t.Blue.Alpha(0.50))
	case b.Hovered():
		b.Background(t.GlassHover)
	default:
		b.Background(t.Glass)
	}
	b.Children(func() {
		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, ic).Size(12, 12).TextColor(pick(active, t.Ink, t.Muted))
			ui.Text(c, label).FontSize(11).TextColor(pick(active, t.Ink, t.Muted))
		})
	})
	if b.Clicked() {
		onClick()
	}
}

// fmtTime 把秒格式化成 m:ss / h:mm:ss。
func fmtTime(sec float64) string {
	if sec < 0 || sec != sec {
		sec = 0
	}
	s := int(sec)
	h, m, ss := s/3600, (s%3600)/60, s%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, ss)
	}
	return fmt.Sprintf("%d:%02d", m, ss)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

var _ = time.Second
