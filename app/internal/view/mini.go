package view

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// miniWidth / miniHeight 是迷你模式的窗口尺寸，与原版一致（400×155）。
// 原版在 Linux 上禁用迷你模式，因为 webkit2gtk 的无边框窗口无法在运行时
// 调整大小；原生窗口没有这个限制，所以三个平台都能用。
const (
	miniWidth  = 400
	miniHeight = 155
)

// MiniShell 是迷你模式的界面：封面、标题、进度、一排紧凑控制。
func (a *App) MiniShell(c *ui.Context) {
	t := a.Theme
	c.Root().Background(t.Period.Sample(0.5))
	ui.Column(c).Fill().Children(func() { a.backdrop(c) })

	ui.Column(c).Absolute().Fill().Padding(8, 10).Gap(6).Children(func() {
		// 标题栏：可拖动 + 还原 / 关闭
		ui.Row(c).FillWidth().Height(MiniTitleBar).Shrink(0).AlignItems(ui.Center).
			Gap(6).DragWindow().Children(func() {
			ui.Text(c, "bili-FM").FontSize(11).Bold().TextColor(t.Ink)
			ui.Box(c).Grow(1)
			restore := ui.ButtonBase(c.Key("mini-restore")).Size(22, 18).Radius(4).Center().
				Label("还原").Tooltip("还原窗口")
			if restore.Hovered() {
				restore.Background(t.GlassHover)
			} else {
				restore.Background(t.Glass)
			}
			if restore.Clicked() && a.Act.SetMini != nil {
				a.Act.SetMini(false)
			}

			closeB := ui.ButtonBase(c.Key("mini-close")).Size(22, 18).Radius(4).Center().
				Label("关闭").Tooltip("关闭")
			if closeB.Hovered() {
				closeB.Background(t.Rose.Alpha(0.28))
			} else {
				closeB.Background(t.Glass)
			}
			if closeB.Clicked() && a.Act.Quit != nil {
				a.Act.Quit()
			}
		})

		// 主体：封面 + 标题 + UP 主
		ui.Row(c).FillWidth().Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(72, 72).Shrink(0).Radius(RadiusSmall).
				Background(t.CoverPlaceholder()).Clip().Children(func() {
				if a.Track != nil {
					if bmp := a.Images.Bitmap(a.Track.Cover); bmp != nil {
						ui.Image(c, bmp).Fill().Fit(ui.Cover)
					}
				}
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				title := "未在播放"
				up := "空格 暂停 / 播放"
				if a.Track != nil {
					title = a.Track.Title
					if a.Track.Up != "" {
						up = a.Track.Up
					}
				}
				ui.Text(c, title).FontSize(12).TextColor(t.Ink).MaxLines(2)
				ui.Text(c, up).FontSize(10).TextColor(t.Faint).SingleLine()
			})
			// 紧凑控制：上一集 / 播放 / 下一集 / 还原大窗
			ui.Row(c).Gap(4).Shrink(0).AlignItems(ui.Center).Children(func() {
				miniBtn(c, a, "mini-prev", iconPrev, func() {
					if a.Act.Prev != nil {
						a.Act.Prev()
					}
				})
				playIcon := iconPlay
				if a.Playing {
					playIcon = iconPause
				}
				miniBtn(c, a, "mini-play", playIcon, func() {
					if a.Act.TogglePlay != nil {
						a.Act.TogglePlay()
					}
				})
				miniBtn(c, a, "mini-next", iconNext, func() {
					if a.Act.Next != nil {
						a.Act.Next()
					}
				})
			})
		})

		// 进度条
		ui.Row(c).FillWidth().Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmtTime(a.Pos)).FontSize(9).TextColor(t.Faint)
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
			ui.Text(c, fmtTime(a.Dur)).FontSize(9).TextColor(t.Faint)
		})
	})
}

// miniBtn 是迷你模式里的小图标按钮。
func miniBtn(c *ui.Context, a *App, key string, ic *ui.SVG, onClick func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key(key)).Size(28, 24).Radius(RadiusSmall).Center().Label(key)
	if b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(t.Glass)
	}
	b.Children(func() { ui.Icon(c, ic).Size(12, 12).TextColor(t.Muted) })
	if b.Clicked() {
		onClick()
	}
}

// MiniSize 返回迷你模式的窗口尺寸，供装配层调整窗口用。
func MiniSize() (int, int) { return miniWidth, miniHeight }

// miniHint 用于日志：迷你模式的尺寸。
func miniHint() string { return fmt.Sprintf("%dx%d", miniWidth, miniHeight) }
