package view

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// infoPanel 是右侧「详情」抽屉：视频信息 + 互动 + 评论。
//
// 对应原版的 videoInfo（固定 366px 高的展示区）与评论抽屉。互动按钮的状态
// 由上层查询后回填（Liked / Coined / Faved / Followed）。
func (a *App) infoPanel(c *ui.Context) {
	t := a.Theme
	ui.Box(c).Absolute().Fill().Background(ui.Hex("#0f172a").Alpha(0.28)).Children(func() {
		ui.Column(c).Absolute().Right(0).Top(0).Bottom(0).Width(366).
			Background(t.Panel).Border(1, t.Line).Padding(12, 12).Gap(8).Children(func() {
			// 标题栏
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "详情").FontSize(13).Bold().TextColor(t.Ink)
				ui.Box(c).Grow(1)
				b := ui.ButtonBase(c.Key("info-close")).Size(24, 22).Radius(RadiusSmall).Center().Label("关闭")
				if b.Hovered() {
					b.Background(t.GlassHover)
				} else {
					b.Background(t.Glass)
				}
				b.Children(func() { ui.Icon(c, iconClose).Size(12, 12).TextColor(t.Muted) })
				if b.Clicked() {
					a.ShowInfo = false
				}
			})
			ui.Divider(c)

			if a.Track == nil {
				ui.Text(c, "还没有在播放的视频").FontSize(12).TextColor(t.Faint)
				return
			}

			// 视频信息
			ui.Text(c, a.Track.Title).FontSize(13).Bold().TextColor(t.Ink).MaxLines(3)
			if a.Track.Up != "" {
				ui.Text(c, a.Track.Up).FontSize(11).TextColor(t.Muted).SingleLine()
			}

			// 互动
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				a.actionChip(c, "like", "点赞", iconLike, a.Liked, a.Act.Like)
				a.actionChip(c, "coin", "投币", iconCoin, a.Coined, a.Act.Coin)
				a.actionChip(c, "fav", "收藏", iconStar, a.Faved, a.Act.Favorite)
				a.actionChip(c, "follow", "关注", iconLike, a.Followed, a.Act.Follow)
			})
			ui.Divider(c)

			// 评论
			ui.Text(c, fmt.Sprintf("评论 %d 条", len(a.Comments))).FontSize(12).Bold().TextColor(t.Ink)
			if len(a.Comments) == 0 {
				ui.Text(c, "还没有加载评论").FontSize(11).TextColor(t.Faint)
				return
			}
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(10).Children(func() {
					for i, cm := range a.Comments {
						ui.Column(c).Key(fmt.Sprintf("cm-%d", i)).FillWidth().Gap(3).Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, cm.User).FontSize(11).TextColor(t.Blue).SingleLine()
								ui.Text(c, cm.Time).FontSize(10).TextColor(t.Faint)
								if cm.Likes > 0 {
									ui.Text(c, fmt.Sprintf("赞 %d", cm.Likes)).FontSize(10).TextColor(t.Faint)
								}
							})
							ui.Text(c, cm.Content).FontSize(11).TextColor(t.Muted).MaxLines(6)
						})
					}
				})
			})
		})
	})
}

// actionChip 是互动按钮，激活时高亮。
func (a *App) actionChip(c *ui.Context, key, label string, ic *ui.SVG, active bool, fn func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("act-" + key)).Grow(1).Height(30).Radius(RadiusSmall).
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
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, ic).Size(12, 12).TextColor(pick(active, t.Ink, t.Muted))
			ui.Text(c, label).FontSize(11).TextColor(pick(active, t.Ink, t.Muted))
		})
	})
	if b.Clicked() && fn != nil {
		fn()
	}
}
