package view

import "github.com/egoist/mygo/ui"

// LyricsShell 是桌面歌词悬浮窗的内容：一行当前字幕 + 一行下一句。
// 窗口本身透明，只画一块深色玻璃；没有按钮，整块区域可拖动。
func (a *App) LyricsShell(c *ui.Context) {
	cur, next := a.LyricCurrent, a.LyricNext
	if cur == "" && next == "" {
		cur = "♪"
	}
	ui.Box(c).Fill().Padding(8).DragWindow().Children(func() {
		ui.Box(c).FillWidth().FillHeight().Radius(16).
			Background(ui.Hex("#0f172a").Alpha(0.72)).
			Border(1, ui.Hex("#ffffff").Alpha(0.14)).
			InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.18)).
			Shadow(0, 10, 30, 4, ui.Hex("#000000").Alpha(0.24)).
			Padding(10, 16).Center().Children(func() {
			ui.Column(c).FillWidth().Gap(2).Center().Children(func() {
				ui.Text(c, cur).FontSize(20).FontWeight(700).
					TextColor(ui.Hex("#ffffff")).TextAlign(ui.Center).MaxLines(2)
				if next != "" {
					ui.Text(c, next).FontSize(13).
						TextColor(ui.Hex("#ffffff").Alpha(0.56)).
						TextAlign(ui.Center).SingleLine()
				}
			})
		})
	})
}
