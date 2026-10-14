package view

import "testing"

// TestSeriesPlayAllSpec 钉住合集抽屉表头「播放全部」的两态（原版 seriesList）：
//
//	isDisabled={seriesVideos.length === 0}  列表为空 → 禁用
//	isLoading={isPlayingAll}                 拉取在途 → 转圈 + 禁用
//
// 原生版此前这两态都没有：空列表也点得动（点了只弹一句警告）、拉取中连点会
// 各起一套分页 + 详情请求。
func TestSeriesPlayAllSpec(t *testing.T) {
	cases := []struct {
		pending, hasVideos bool
		spinner, enabled   bool
	}{
		{false, true, false, true},   // 常态：播放图标 + 可点
		{true, true, true, false},    // 拉取中：转圈 + 禁用
		{true, false, true, false},   // 拉取中且列表已清空：仍先转圈
		{false, false, false, false}, // 空列表：Play 图标但禁用
	}
	for _, c := range cases {
		spinner, enabled := seriesPlayAllSpec(c.pending, c.hasVideos)
		if spinner != c.spinner || enabled != c.enabled {
			t.Errorf("seriesPlayAllSpec(%v,%v) = (spinner=%v, enabled=%v), 想 (%v,%v)",
				c.pending, c.hasVideos, spinner, enabled, c.spinner, c.enabled)
		}
	}
}

// TestSeriesPlayAllButtonDisabled 用无头 tester 真点一次「播放全部」：
// 空列表 / 拉取在途时点不动（Act.SeriesPlayAll 不触发），常态触发一次。
func TestSeriesPlayAllButtonDisabled(t *testing.T) {
	render := func(pending, hasVideos bool) int {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Drawer = DrawerSeries
		a.SeriesID = 3
		a.SeriesName = "某合集"
		a.SeriesPlayAllPending = pending
		if hasVideos {
			a.ListFor(DrawerSeries).Cards = []Card{{Bvid: "BV1", Title: "第一集"}}
		}
		n := 0
		a.Act.SeriesPlayAll = func() { n++ }
		tt := shellTester(t, a)
		// 禁用时点不到（Click 返回错误），忽略返回值。
		_ = tt.Click("播放全部")
		tt.Frame()
		return n
	}

	if n := render(false, false); n != 0 {
		t.Fatalf("合集列表为空时点击「播放全部」触发了加载：n=%d，想 0", n)
	}
	if n := render(true, true); n != 0 {
		t.Fatalf("拉取在途时点击「播放全部」重复触发了加载：n=%d，想 0", n)
	}
	if n := render(false, true); n != 1 {
		t.Fatalf("常态点击「播放全部」：n=%d，想 1", n)
	}
}
