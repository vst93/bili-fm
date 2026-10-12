package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestGroupDanmaku(t *testing.T) {
	list := []Danmaku{
		{Time: 3.2, Text: "哈哈"},
		{Time: 3.8, Text: "哈哈"},   // 同一秒、同文本 → 计数 2
		{Time: 3.5, Text: " 哈哈 "}, // 归一化后同上 → 计数 3
		{Time: 3.9, Text: "厉害"},   // 同一秒、不同文本
		{Time: 12.1, Text: "前方高能"},
	}
	groups := groupDanmaku(list)
	if len(groups) != 2 {
		t.Fatalf("组数 = %d, want 2", len(groups))
	}
	if groups[0].Second != 3 || groups[1].Second != 12 {
		t.Fatalf("组的秒序 = %d,%d, want 3,12", groups[0].Second, groups[1].Second)
	}
	g := groups[0]
	if g.TotalCount != 4 {
		t.Errorf("第一组总条数 = %d, want 4", g.TotalCount)
	}
	if len(g.Entries) != 2 {
		t.Fatalf("第一组去重后 = %d 条, want 2", len(g.Entries))
	}
	// 「哈哈」出现 3 次，「厉害」1 次 → 排在前。
	if g.Entries[0].Text != "哈哈" || g.Entries[0].Count != 3 {
		t.Errorf("组内排序/计数 = %+v", g.Entries)
	}
	if g.Entries[1].Text != "厉害" || g.Entries[1].Count != 1 {
		t.Errorf("组内第二条 = %+v", g.Entries[1])
	}
}

func TestGroupDanmakuEmpty(t *testing.T) {
	if groups := groupDanmaku(nil); len(groups) != 0 {
		t.Errorf("空输入应得 0 组，得 %d", len(groups))
	}
}

func TestDanmakuInk(t *testing.T) {
	// 默认色（0 / 未提供）用正文色。
	if got := danmakuInk(0); got != ui.Hex("#1e293b") {
		t.Errorf("danmakuInk(0) = %v", got)
	}
	// 白色太亮 → 深灰对比色。
	if got := danmakuInk(0xffffff); got != ui.Hex("#1a1a1a") {
		t.Errorf("danmakuInk(白) = %v", got)
	}
	// 中等亮度（红 #ff0000 → 亮度 76）应保留原色。
	if got := danmakuInk(0xff0000); got != ui.RGB(255, 0, 0) {
		t.Errorf("danmakuInk(红) = %v", got)
	}
}

func TestNearestGroup(t *testing.T) {
	groups := []danmakuGroup{{Second: 0}, {Second: 4}, {Second: 10}}
	// 3.9 更近 4 而不是 0（原版 currentIndex 取最近的一组）。
	if got := nearestGroup(groups, 3.9); got != 1 {
		t.Errorf("nearestGroup(3.9) = %d, want 1", got)
	}
	// 2.0 与 0 / 4 等距，取前一组。
	if got := nearestGroup(groups, 2.0); got != 0 {
		t.Errorf("nearestGroup(2.0) = %d, want 0", got)
	}
	// 在全部组之前 / 之后各钳到首尾。
	if got := nearestGroup(groups, -5); got != 0 {
		t.Errorf("nearestGroup(-5) = %d, want 0", got)
	}
	if got := nearestGroup(groups, 99); got != 2 {
		t.Errorf("nearestGroup(99) = %d, want 2", got)
	}
	if got := nearestGroup(nil, 3); got != -1 {
		t.Errorf("nearestGroup(空) = %d, want -1", got)
	}
}

func TestFmtClock(t *testing.T) {
	cases := map[float64]string{0: "00:00", 5: "00:05", 75: "01:15", 3725: "62:05", -3: "00:00"}
	for in, want := range cases {
		if got := fmtClock(in); got != want {
			t.Errorf("fmtClock(%v) = %q, want %q", in, got, want)
		}
	}
}
