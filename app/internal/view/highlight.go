package view

import "github.com/egoist/mygo/ui"

// NextHighlightPoint 返回 pos 之后第一个足够明显的弹幕热区时刻（秒）。
//
// 这条规则是「跳到下一个高能」的**唯一事实源**：控制器（player_extras.go 的
// nextHighlight，真正 seek）与进度条右键菜单（把目标时刻写进条目副标题）
// 都从这里取。此前只有控制器里那一份，若菜单再抄一份，两处的 1 秒提前量与
// 0.25 阈值迟早会漂开 —— 菜单显示的落点和实际跳的落点就不是同一个。
//
// points 按时间升序（buildDanmakuHeat 的顺序输出）；Value 已按 sqrt 归一化
// 到 0..1，0.25 是「明显高能」的门槛。
func NextHighlightPoint(points []HeatPoint, pos float64) (float64, bool) {
	for _, p := range points {
		if p.At > pos+1 && p.Value >= 0.25 {
			return p.At, true
		}
	}
	return 0, false
}

// highlightSubtitle 是「跳到下一个高能」菜单项的副标题文案。
//
// mygo 0.8.1 起 ui.MenuItem 有 Subtitle（macOS 14 画在标签下，其余平台跟在
// 标签后成「标签 — 副标题」，见 mygo 的 docs/ui/context-menu.md）。用它把
// **落点**写在条目上：用户点下去之前就知道会跳到哪，而不是点完才等到一句
// 「后面没有更多高能片段」。抽成纯函数是为了能直接断言文案（系统菜单在
// 无头测试里只暴露标签，读不到副标题）。
func highlightSubtitle(points []HeatPoint, pos float64) string {
	if at, ok := NextHighlightPoint(points, pos); ok {
		return fmtTime(at)
	}
	return "后面没有更多高能"
}

// nextHighlightMenu 给进度条挂「跳到下一个高能」右键菜单。原版没有这条菜单
// （热力条只负责「看见」），是本仓库的补充：不占播放栏的定宽列，也不改变
// 原版布局。主窗与迷你窗共用，保证两处的副标题与动作完全一致。
func (a *App) nextHighlightMenu(target ui.Element) {
	target.ContextMenu(func(m *ui.Menu) {
		item := m.Item("跳到下一个高能")
		item.Subtitle(highlightSubtitle(a.DanmakuHeat, a.Pos))
		if item.Chosen() && a.Act.NextHighlight != nil {
			a.Act.NextHighlight()
		}
	})
}
