package view

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestNextHighlightPoint 钉住「下一个高能」的选点规则。控制器（真正 seek）与
// 进度条右键菜单（副标题写落点）共用这一个函数，所以规则的每一条都在这里钉。
func TestNextHighlightPoint(t *testing.T) {
	heat := []HeatPoint{{At: 10, Value: 0.9}, {At: 40, Value: 0.1}, {At: 70, Value: 0.3}}
	cases := []struct {
		pos  float64
		want float64
		ok   bool
	}{
		{0, 10, true},
		{8.5, 10, true}, // pos+1 = 9.5 < 10
		{9, 70, true},   // pos+1 = 10，At=10 不再是「之后」；40 的 Value 太低
		{50, 70, true},
		{70, 0, false},
		{100, 0, false},
	}
	for _, c := range cases {
		at, ok := NextHighlightPoint(heat, c.pos)
		if ok != c.ok || (ok && at != c.want) {
			t.Errorf("pos=%v: got (%v,%v), want (%v,%v)", c.pos, at, ok, c.want, c.ok)
		}
	}
	if _, ok := NextHighlightPoint(nil, 0); ok {
		t.Error("没有热力数据时不该报「有下一个高能」")
	}
	// 阈值边界：0.25 是门槛，0.24 不算。
	if _, ok := NextHighlightPoint([]HeatPoint{{At: 30, Value: 0.24}}, 0); ok {
		t.Error("Value 0.24 不该算明显高能")
	}
	if at, ok := NextHighlightPoint([]HeatPoint{{At: 30, Value: 0.25}}, 0); !ok || at != 30 {
		t.Errorf("Value 0.25 应算明显高能，得到 (%v,%v)", at, ok)
	}
}

// TestHighlightSubtitle 钉住菜单项副标题的两种文案：有落点写时刻（fmtTime 的
// m:ss / h:mm:ss），没有则明说 —— 不再等用户点完 toast 才告诉白点了。
func TestHighlightSubtitle(t *testing.T) {
	heat := []HeatPoint{{At: 65, Value: 0.6}, {At: 3725, Value: 1}}
	if got := highlightSubtitle(heat, 0); got != "1:05" {
		t.Errorf("副标题 = %q，应为 1:05", got)
	}
	if got := highlightSubtitle(heat, 100); got != "1:02:05" {
		t.Errorf("副标题 = %q，应为 1:02:05（超过一小时带 h:mm:ss）", got)
	}
	if got := highlightSubtitle(heat, 4000); got != "后面没有更多高能" {
		t.Errorf("无高能时副标题 = %q，应为「后面没有更多高能」", got)
	}
}

// TestProgressMenuJumpsToNextHighlight 无头点开进度条右键菜单：菜单里有
// 「跳到下一个高能」，选中会触发 Act.NextHighlight。副标题由 highlightSubtitle
// 给出（系统菜单在无头窗口里只暴露标签，读不到 Subtitle），这里钉的是接线：
// 菜单项存在、选项真的走到动作。
func TestProgressMenuJumpsToNextHighlight(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Dur = 600
	a.Pos = 100
	a.DanmakuHeat = []HeatPoint{{At: 200, Value: 0.9}}
	calls := 0
	a.Act.NextHighlight = func() { calls++ }
	tst := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.Theme.uiTheme())
		ui.Row(c).Fill().Padding(8, 16).Children(func() { a.progress(c) })
	}, 600, 120)
	tst.Frame()
	if err := tst.RightClick("播放进度"); err != nil {
		t.Fatalf("右键进度条失败：%v", err)
	}
	tst.Frame()
	if got := tst.Menu(); !slices.Contains(got, "跳到下一个高能") {
		t.Fatalf("进度条右键菜单 = %v，缺「跳到下一个高能」", got)
	}
	if err := tst.ChooseMenuItem("跳到下一个高能"); err != nil {
		t.Fatalf("选择菜单项失败：%v", err)
	}
	tst.Frame()
	if calls != 1 {
		t.Fatalf("Act.NextHighlight 调用 %d 次，应为 1", calls)
	}
}
