package view

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// partsPanel 是分集列表面板（多 P 视频）。
func (a *App) partsPanel(c *ui.Context) {
	t := a.Theme
	ui.Box(c).Absolute().Fill().Background(ui.Hex("#0f172a").Alpha(0.28)).Children(func() {
		ui.Column(c).Absolute().Right(0).Top(0).Bottom(0).Width(320).
			Background(t.Panel).Border(1, t.Line).Padding(12, 12).Gap(8).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "分集").FontSize(13).Bold().TextColor(t.Ink)
				ui.Box(c).Grow(1)
				b := ui.ButtonBase(c.Key("parts-close")).Size(24, 22).Radius(RadiusSmall).Center().Label("关闭")
				if b.Hovered() {
					b.Background(t.GlassHover)
				} else {
					b.Background(t.Glass)
				}
				b.Children(func() { ui.Icon(c, iconClose).Size(12, 12).TextColor(t.Muted) })
				if b.Clicked() {
					a.ShowParts = false
				}
			})
			ui.Divider(c)
			if len(a.Parts) == 0 {
				ui.Text(c, "没有分集信息").FontSize(12).TextColor(t.Faint)
				return
			}
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					for i, p := range a.Parts {
						active := a.Track != nil && a.Track.Cid == p.Cid
						row := ui.Row(c).Key(fmt.Sprintf("part-%d", p.Cid)).FillWidth().
							Padding(6, 8).Radius(RadiusSmall).Gap(8).AlignItems(ui.Center).
							Cursor(ui.CursorPointer)
						switch {
						case active:
							row.Background(t.Blue.Alpha(0.22))
						case row.Hovered():
							row.Background(t.GlassHover)
						}
						row.Children(func() {
							ui.Text(c, fmt.Sprintf("P%d", p.Page)).FontSize(10).TextColor(t.Faint).Width(26)
							ui.Text(c, p.Part).FontSize(12).TextColor(t.Ink).Grow(1).SingleLine()
						})
						if row.Clicked() {
							idx := i
							a.ShowParts = false
							if a.Act.Play != nil {
								a.Act.Play(idx)
							}
						}
					}
				})
			})
		})
	})
}

// danmakuPanel 是弹幕列表：按时间排，点一条跳到那一刻。
func (a *App) danmakuPanel(c *ui.Context) {
	t := a.Theme
	ui.Box(c).Absolute().Fill().Background(ui.Hex("#0f172a").Alpha(0.28)).Children(func() {
		ui.Column(c).Absolute().Right(0).Top(0).Bottom(0).Width(320).
			Background(t.Panel).Border(1, t.Line).Padding(12, 12).Gap(8).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "弹幕").FontSize(13).Bold().TextColor(t.Ink)
				ui.Text(c, fmt.Sprintf("共 %d 条", len(a.Danmaku))).FontSize(11).TextColor(t.Faint)
				ui.Box(c).Grow(1)
				b := ui.ButtonBase(c.Key("dm-close")).Size(24, 22).Radius(RadiusSmall).Center().Label("关闭")
				if b.Hovered() {
					b.Background(t.GlassHover)
				} else {
					b.Background(t.Glass)
				}
				b.Children(func() { ui.Icon(c, iconClose).Size(12, 12).TextColor(t.Muted) })
				if b.Clicked() {
					a.ShowDanmaku = false
				}
			})
			ui.Divider(c)
			if len(a.Danmaku) == 0 {
				ui.Text(c, "还没有弹幕").FontSize(12).TextColor(t.Faint)
				return
			}
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					for i, d := range a.Danmaku {
						row := ui.Row(c).Key(fmt.Sprintf("dm-%d", i)).FillWidth().
							Padding(4, 8).Radius(RadiusSmall).Gap(8).AlignItems(ui.Center).
							Cursor(ui.CursorPointer)
						if row.Hovered() {
							row.Background(t.GlassHover)
						}
						row.Children(func() {
							ui.Text(c, fmtTime(d.Time)).FontSize(10).TextColor(t.Faint).Width(44)
							ui.Text(c, d.Text).FontSize(12).TextColor(t.Ink).Grow(1).SingleLine()
						})
						if row.Clicked() && a.Act.Seek != nil {
							a.Act.Seek(d.Time)
						}
					}
				})
			})
		})
	})
}
