package view

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// 播放栏（原版 #player）：固定底部、高 56、单行。
//
//	#player .player-controls {
//	    height: 56px; padding: 0 18px;
//	    grid-template-columns: 80px 56px minmax(0,1fr) 56px 34px 40px 34px 40px;
//	    gap: 8px; border-top: 1px solid var(--player-border);
//	    background-color: var(--player-surface);
//	}
//
// 列依次是：传输键（播放 42 + 下一集 32）/ 当前时间 / 进度 / 总时长 /
// 音量 / 倍速 / 均衡 / 跳过赞助。原版没有封面和标题 —— 那些在主区。
func (a *App) playerBar(c *ui.Context) {
	t := a.Theme
	ui.Column(c).FillWidth().Height(56).Shrink(0).
		Background(t.PlayerSurface).Children(func() {
		ui.Box(c).FillWidth().Height(1).Shrink(0).Background(t.PlayerBorder)
		ui.Row(c).FillWidth().Grow(1).Padding(0, 18).Gap(8).
			AlignItems(ui.Center).Children(func() {
			a.transport(c)
			a.timeText(c, a.Pos)
			a.progress(c)
			a.timeText(c, a.Dur)
			a.volumeButton(c)
			a.speedButton(c)
			a.eqButton(c)
			a.sponsorButton(c)
		})
	})

	// 倍速 / 音量弹层：从播放栏上方向上弹。
	if a.ShowSpeed {
		a.speedPopover(c)
	}
	if a.ShowVolume {
		a.volumePopover(c)
	}
}

// transport 是第 1 列（80px）：播放/暂停（42） + 下一集（32）。
func (a *App) transport(c *ui.Context) {
	t := a.Theme
	ui.Row(c).Width(80).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
		play := ui.ButtonBase(c.Key("play")).Size(42, 42).Radius(Radius).Center().
			Label(pick(a.Playing, "暂停", "播放")).Tooltip(pick(a.Playing, "暂停", "播放"))
		play.Background(t.Blue.Alpha(0.18)).Border(1, t.Blue.Alpha(0.35))
		play.Children(func() {
			if a.Playing {
				ui.Icon(c, iconPause).Size(17, 17).TextColor(ui.Hex("#0369a1"))
			} else {
				ui.Icon(c, iconPlay).Size(17, 17).TextColor(ui.Hex("#0369a1"))
			}
		})
		if play.Clicked() && a.Act.TogglePlay != nil {
			a.Act.TogglePlay()
		}

		next := ui.ButtonBase(c.Key("next")).Size(32, 32).Radius(Radius).Center().
			Label("下一集").Tooltip("下一集")
		next.Disabled(!a.canNavigate())
		if next.Hovered() {
			next.Background(t.GlassHover)
		} else {
			next.Background(ui.Transparent)
		}
		next.Border(1, t.GlassBorder)
		next.Children(func() { ui.Icon(c, iconNext).Size(15, 15).TextColor(ui.Hex("#334155")) })
		if next.Clicked() && a.Act.Next != nil {
			a.Act.Next()
		}
	})
}

// timeText 是第 2 / 第 4 列（56px）的时间文本。
func (a *App) timeText(c *ui.Context, sec float64) {
	ui.Text(c, fmtTime(sec)).FontSize(11).TextColor(ui.Hex("#334155").Alpha(0.72)).
		Width(56).Shrink(0).Center()
}

// progress 是第 3 列（1fr）的可拖动进度条，上面叠广告段标记。
func (a *App) progress(c *ui.Context) {
	hi := a.Dur
	if hi <= 0 {
		hi = 1
	}
	if !a.seeking {
		a.SeekValue = a.Pos
	}
	wrap := ui.Box(c).Grow(1).MinWidth(0)
	wrap.Children(func() {
		s := ui.Slider(c, &a.SeekValue, 0, hi).FillWidth().Label("播放进度")
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
		// 广告段标记：纯视觉，与「自动跳过」开关无关（原版 player-timeline-sponsor）。
		if a.Dur > 0 {
			for _, seg := range a.SponsorSegments {
				start := float32(seg.Start / a.Dur)
				end := float32(seg.End / a.Dur)
				if start < 0 {
					start = 0
				}
				if end > 1 {
					end = 1
				}
				if end <= start {
					continue
				}
				ui.Box(c).Absolute().BottomPercent(45).LeftPercent(start * 100).
					WidthPercent((end - start) * 100).Height(3).Radius(2).
					Background(ui.Hex("#ef4444").Alpha(0.55)).PassThrough()
			}
		}
	})
}

// volumeButton 是第 5 列（34px）：点开音量弹层。
func (a *App) volumeButton(c *ui.Context) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("volume")).Size(32, 32).Radius(Radius).Center().
		Label("音量").Tooltip("音量")
	if a.ShowVolume || b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(ui.Transparent)
	}
	b.Border(1, t.GlassBorder)
	// 原版：音量键图标随音量变化（0 视为静音）。
	muted := a.Muted || a.Volume <= 0
	b.Children(func() {
		ui.Icon(c, pick(muted, iconMute, iconVolume)).Size(15, 15).
			TextColor(pick(muted, t.Rose, ui.Hex("#334155")))
	})
	if b.Clicked() {
		a.ShowVolume = !a.ShowVolume
		a.ShowSpeed = false
	}
}

// speedButton 是第 6 列（40px）：显示当前倍速，点开档位菜单。
func (a *App) speedButton(c *ui.Context) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("speed")).Size(40, 32).Radius(Radius).Center().
		Label("倍速").Tooltip("倍速")
	if a.ShowSpeed || b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(ui.Transparent)
	}
	b.Border(1, t.GlassBorder)
	b.Children(func() {
		ui.Text(c, fmt.Sprintf("%.1fx", a.Speed)).FontSize(11).TextColor(ui.Hex("#334155"))
	})
	if b.Clicked() {
		a.ShowSpeed = !a.ShowSpeed
		a.ShowVolume = false
	}
}

// eqButton 是第 7 列（34px）：均衡开关。
func (a *App) eqButton(c *ui.Context) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("eq")).Size(32, 32).Radius(Radius).Center().
		Label("均衡").Tooltip("均衡")
	switch {
	case a.EQ:
		b.Background(t.Blue.Alpha(0.28))
	case b.Hovered():
		b.Background(t.GlassHover)
	default:
		b.Background(ui.Transparent)
	}
	b.Border(1, t.GlassBorder)
	b.Children(func() {
		ui.Icon(c, iconEQ).Size(15, 15).TextColor(pick(a.EQ, ui.Hex("#0369a1"), ui.Hex("#334155")))
	})
	if b.Clicked() && a.Act.ToggleEQ != nil {
		a.Act.ToggleEQ()
	}
}

// sponsorButton 是第 8 列（40px）：跳过赞助片段开关。
func (a *App) sponsorButton(c *ui.Context) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("sponsor")).Size(32, 32).Radius(Radius).Center().
		Label("跳过赞助").Tooltip("跳过赞助片段")
	switch {
	case a.Sponsor:
		b.Background(t.Blue.Alpha(0.28))
	case b.Hovered():
		b.Background(t.GlassHover)
	default:
		b.Background(ui.Transparent)
	}
	b.Border(1, t.GlassBorder)
	b.Children(func() {
		ui.Icon(c, iconSponsor).Size(15, 15).
			TextColor(pick(a.Sponsor, ui.Hex("#0369a1"), ui.Hex("#334155")))
		// 状态点（原版按钮右上角的小圆）：绿=有分段，黄=查询中，红=失败，灰=无/未启用。
		if a.Sponsor {
			ui.Box(c).Size(6, 6).Radius(RadiusPill).Absolute().Right(4).Top(4).
				Background(sponsorDotColor(a.SponsorStatus))
		}
	})
	if b.Clicked() && a.Act.ToggleSponsor != nil {
		a.Act.ToggleSponsor()
	}
}

// sponsorDotColor 把 SponsorBlock 状态映射成状态点颜色。
func sponsorDotColor(status string) ui.Color {
	switch status {
	case "loading":
		return ui.Hex("#f59e0b")
	case "ok":
		return ui.Hex("#22c55e")
	case "error":
		return ui.Hex("#ef4444")
	default:
		return ui.Hex("#94a3b8")
	}
}

// ---------------------------------------------------------------- 弹层

// speedPopover 是倍速档位菜单，从播放栏第 6 列上方弹出。
func (a *App) speedPopover(c *ui.Context) {
	t := a.Theme
	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Top(0).Children(func() {
		ui.Column(c).Absolute().Right(150).Bottom(60).Width(96).Padding(4).
			Radius(Radius).Background(t.Panel).Border(1, t.GlassBorder).
			Shadow(0, 8, 24, 0, shadowInk.Alpha(0.12)).Children(func() {
			for _, opt := range speedOptions {
				active := abs(opt-a.Speed) < 0.001
				b := ui.ButtonBase(c.Key(fmt.Sprintf("rate-%v", opt))).FillWidth().Height(26).
					Padding(0, 8).Radius(RadiusSmall).Label(fmt.Sprintf("%.1fx", opt))
				switch {
				case active:
					b.Background(t.Blue.Alpha(0.22))
				case b.Hovered():
					b.Background(t.GlassHover)
				}
				b.Children(func() {
					ui.Text(c, fmt.Sprintf("%.1fx", opt)).FontSize(11).
						TextColor(pick(active, t.Blue, t.Ink))
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
	})
}

// volumePopover 是音量滑块，从播放栏第 5 列上方弹出。
func (a *App) volumePopover(c *ui.Context) {
	a.volumePopoverAt(c, 190, 60)
}

// volumePopoverAt 同上，可指定右下角偏移（迷你模式的播放栏更矮）。
func (a *App) volumePopoverAt(c *ui.Context, right, bottom float32) {
	t := a.Theme
	ui.Box(c).Absolute().Fill().Children(func() {
		ui.Column(c).Absolute().Right(right).Bottom(bottom).Width(200).Padding(10).
			Radius(Radius).Background(t.Panel).Border(1, t.GlassBorder).
			Shadow(0, 8, 24, 0, shadowInk.Alpha(0.12)).Gap(8).Children(func() {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				// 静音键：独立的切换（原版 player-volume-mute-button）。
				mute := ui.ButtonBase(c.Key("volume-mute")).Size(26, 26).Radius(RadiusSmall).
					Center().Label(pick(a.Muted, "取消静音", "静音")).Tooltip(pick(a.Muted, "取消静音", "静音"))
				if a.Muted {
					mute.Background(t.Rose.Alpha(0.18))
				} else if mute.Hovered() {
					mute.Background(t.GlassHover)
				} else {
					mute.Background(ui.Transparent)
				}
				mute.Children(func() {
					ui.Icon(c, pick(a.Muted, iconMute, iconVolume)).Size(14, 14).
						TextColor(pick(a.Muted, t.Rose, t.Muted))
				})
				if mute.Clicked() && a.Act.ToggleMute != nil {
					a.Act.ToggleMute()
				}

				v := a.Volume
				s := ui.Slider(c, &v, 0, 1).Grow(1).Label("音量")
				if s.Changed() {
					a.Volume = v
					if a.Act.SetVolume != nil {
						a.Act.SetVolume(v)
					}
				}
				ui.Text(c, fmt.Sprintf("%d%%", int(a.Volume*100))).FontSize(10).TextColor(t.Faint)
			})
		})
	})
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

// speedOptions 是倍速可选项，与旧版的档位一致。
var speedOptions = []float64{0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 2.5, 3.0}
