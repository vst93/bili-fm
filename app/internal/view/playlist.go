package view

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// 播放列表抽屉（原版 playlist.tsx）：表头是「我的列表 / 合集列表」两个 tab +
// 定位当前 + 播放模式 + 清空；正文是记录行（缩略图 + 分集名 + 视频名 + 删除）。
//
// 与旧版的取舍：拖拽排序改成了每行 hover 时出现的上移/下移键 —— 原生界面
// 做跨行拖拽收益不高，上/下移一样能完成排序。

// playlistTabs 是播放列表抽屉的表头。
func (a *App) playlistTabs(c *ui.Context) {
	tabs := []tabItem{{Key: ListUser, Label: fmt.Sprintf("我的列表(%d)", len(a.Playlist))}}
	if len(a.SeriesPlaylist) > 0 {
		tabs = append(tabs, tabItem{Key: ListSeries, Label: fmt.Sprintf("合集列表(%d)", len(a.SeriesPlaylist))})
	}
	a.tabs(c, "playlist", tabs, a.PlaylistTab, func(k string) {
		a.PlaylistTab = k
		if a.Act.SwitchPlaylistTab != nil {
			a.Act.SwitchPlaylistTab(k)
		}
	})

	// 定位到当前播放的记录：只在当前 tab 就是正在播放的来源时有效。
	if a.PlayingPlaylist == a.PlaylistTab {
		if items := a.playlistActive(a.PlaylistTab); a.Index >= 0 && a.Index < len(items) {
			a.headerButton(c, "playlist-locate", "定位到当前播放的记录", iconLocate, func() {
				a.locateNow = true
			})
		}
	}

	a.playModeButton(c)

	if a.PlaylistTab == ListUser && len(a.Playlist) > 0 {
		a.headerButton(c, "playlist-clear", "清空播放列表", iconDelete, func() {
			if a.Act.ClearPlaylist != nil {
				a.Act.ClearPlaylist(ListUser)
			}
		})
	}
}

// playModeButton 是播放模式开关（顺序 → 单曲循环 → 随机 → 顺序）。
func (a *App) playModeButton(c *ui.Context) {
	t := a.Theme
	ic := iconOrder
	label := "当前：顺序播放"
	switch a.PlayMode {
	case PlayModeSingle:
		ic, label = iconLoopOne, "当前：单曲循环"
	case PlayModeShuffle:
		ic, label = iconShuffle, "当前：随机播放"
	}
	b := ui.ButtonBase(c.Key("play-mode")).Size(32, 32).Radius(Radius).Center().
		Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(t.GlassActive)
	}
	b.Children(func() {
		ui.Icon(c, ic).Size(17, 17).
			TextColor(pick(a.PlayMode == PlayModeSequence, ui.Hex("#475569"), t.Blue))
	})
	if b.Clicked() && a.Act.CyclePlayMode != nil {
		a.Act.CyclePlayMode()
	}
}

// playlistBody 画记录列表。
func (a *App) playlistBody(c *ui.Context) {
	t := a.Theme
	items := a.playlistActive(a.PlaylistTab)
	if len(items) == 0 {
		if a.PlaylistTab == ListSeries {
			ui.Text(c, "合集列表为空（在合集里点「播放全部」加载）").FontSize(12).TextColor(t.Faint)
		} else {
			ui.Text(c, "播放列表为空（在选集里点卡片右上角的 + 添加）").FontSize(12).TextColor(t.Faint)
		}
		return
	}
	ui.Column(c).Gap(2).Children(func() {
		for i := range items {
			a.playlistRow(c, i, items[i])
		}
	})
	// 定位标志只在这一次构建里生效。
	a.locateNow = false
}

// playlistRow 是一条记录。
func (a *App) playlistRow(c *ui.Context, index int, item PlayItem) {
	t := a.Theme
	current := a.PlayingPlaylist == a.PlaylistTab && a.Index == index
	row := ui.Row(c).Key("pl-"+item.ID).FillWidth().Padding(6, 8).
		Radius(RadiusSmall).Gap(10).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	switch {
	case current:
		row.Background(t.Blue.Alpha(0.18))
	case row.Hovered():
		row.Background(t.GlassHover)
	}
	if current && a.locateNow {
		row.ScrollIntoView()
	}

	row.Children(func() {
		// 缩略图 64×36。
		ui.Box(c).Width(64).Height(36).Shrink(0).Radius(4).
			Background(t.CoverPlaceholder()).Clip().Children(func() {
			src := item.FirstFrame
			if src == "" {
				src = item.Pic
			}
			if bmp := a.Images.Bitmap(src); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			}
		})

		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				if current {
					ui.Icon(c, iconPlay).Size(11, 11).TextColor(t.Blue)
				}
				ui.Text(c, item.Part).FontSize(13).
					TextColor(pick(current, t.Blue, ui.Hex("#1f2937"))).SingleLine()
			})
			ui.Text(c, item.Title).FontSize(11).TextColor(t.Faint).SingleLine()
		})

		// 我的列表才允许编辑：上移 / 下移 / 删除。
		if a.PlaylistTab == ListUser {
			if index > 0 {
				a.rowIconButton(c, fmt.Sprintf("pl-up-%s", item.ID), "上移", iconUp, ui.Hex("#94a3b8"), func() {
					if a.Act.ReorderPlaylist != nil {
						a.Act.ReorderPlaylist(ListUser, index, index-1)
					}
				})
			}
			if index < len(a.Playlist)-1 {
				a.rowIconButton(c, fmt.Sprintf("pl-down-%s", item.ID), "下移", iconDown, ui.Hex("#94a3b8"), func() {
					if a.Act.ReorderPlaylist != nil {
						a.Act.ReorderPlaylist(ListUser, index, index+1)
					}
				})
			}
			a.rowIconButton(c, fmt.Sprintf("pl-del-%s", item.ID), "删除", iconClose, ui.Hex("#94a3b8"), func() {
				if a.Act.DeletePlaylistItem != nil {
					a.Act.DeletePlaylistItem(ListUser, item.ID)
				}
			})
		}
	})
	if row.Clicked() && a.Act.PlayPlaylist != nil {
		a.Act.PlayPlaylist(a.PlaylistTab, index)
	}
}

// rowIconButton 是记录行右侧的小图标键（row 的子元素，不冒泡触发播放）。
func (a *App) rowIconButton(c *ui.Context, key, label string, ic *ui.SVG, ink ui.Color, fn func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key(key)).Size(26, 26).Radius(RadiusSmall).Center().
		Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.GlassActive)
	} else {
		b.Background(ui.Transparent)
	}
	b.Children(func() { ui.Icon(c, ic).Size(14, 14).TextColor(ink) })
	if b.Clicked() {
		fn()
	}
}

// PlayItemFromPart 把当前详情的某一集转成播放列表记录。
func PlayItemFromPart(info *Info, bvid string, aid int64, part Part) PlayItem {
	title, pic := "", ""
	if info != nil {
		title, pic = info.Title, info.Pic
	}
	return PlayItem{
		ID:         fmt.Sprintf("%s-%d", bvid, part.Cid),
		Bvid:       bvid,
		Aid:        aid,
		Cid:        part.Cid,
		Part:       part.Part,
		FirstFrame: "",
		Title:      title,
		Pic:        pic,
	}
}
