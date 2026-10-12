package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestDrawerSnap 钉住下滑手势的落点：过中点（或带速度投影后过中点）就落到
// 最底（等关闭），否则回弹到原位。
func TestDrawerSnap(t *testing.T) {
	const h float32 = 400
	snap := drawerSnap(h)
	cases := []struct {
		in   ui.Point
		want ui.Point
	}{
		{ui.Point{}, ui.Point{}},
		{ui.Point{Y: 100}, ui.Point{}},
		{ui.Point{Y: 200}, ui.Point{}}, // 正好中点：Nearest 取先出现的 {} 那一侧
		{ui.Point{Y: 201}, ui.Point{Y: h}},
		{ui.Point{Y: h}, ui.Point{Y: h}},
	}
	for _, c := range cases {
		if got := snap(c.in); got != c.want {
			t.Errorf("drawerSnap(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestDrawerDismissed 钉住「动画途中不关」：只有偏移到底且停稳才算关闭，
// 否则抽屉会在滑下去的中途就消失。
func TestDrawerDismissed(t *testing.T) {
	const h float32 = 400
	cases := []struct {
		offsetY       float32
		transitioning bool
		want          bool
	}{
		{0, false, false},
		{200, false, false},
		{h, true, false}, // 到底了但还在滑：不能关
		{h, false, true},
		{h + 20, false, true},
	}
	for _, c := range cases {
		if got := drawerDismissed(c.offsetY, h, c.transitioning); got != c.want {
			t.Errorf("drawerDismissed(%v, %v, %v) = %v, want %v",
				c.offsetY, h, c.transitioning, got, c.want)
		}
	}
	// 高度退化时不能误判为关闭。
	if drawerDismissed(0, 0, false) {
		t.Error("drawerDismissed(0, 0, false) = true, want false")
	}
}

// TestDrawerFade 钉住遮罩随下滑的透明度：贴底 1、到底 0、中间线性，
// 且越界时夹在 [0, 1]。
func TestDrawerFade(t *testing.T) {
	const h float32 = 400
	cases := []struct {
		offsetY float32
		want    float32
	}{
		{0, 1},
		{-10, 1}, // 上弹越界：夹回 1
		{200, 0.5},
		{h, 0},
		{h + 50, 0}, // 下甩越界：夹到 0
	}
	for _, c := range cases {
		if got := drawerFade(c.offsetY, h); got != c.want {
			t.Errorf("drawerFade(%v, %v) = %v, want %v", c.offsetY, h, got, c.want)
		}
	}
	// 高度退化（首帧还没布局）时不能除零，按不透明处理。
	if got := drawerFade(10, 0); got != 1 {
		t.Errorf("drawerFade(10, 0) = %v, want 1", got)
	}
}

func TestFieldStyle(t *testing.T) {
	// 列比例 / 伸展 / 对齐 / 隐藏阈值对齐原版 .card-meta-field.is-*。
	cases := []struct {
		kind  cardFieldKind
		basis float32
		grow  float32
		minW  float32
		align ui.Align
	}{
		{fieldAuthor, 40, 4, 0, ui.Start},
		{fieldViews, 30, 3, 150, ui.End},
		{fieldPubdate, 30, 4, 200, ui.End}, // 日期列 grow 4，可从富余里「借」宽
		{fieldDuration, 30, 3, 240, ui.End},
		{fieldExtra, 30, 3, 260, ui.End},
	}
	for _, c := range cases {
		basis, grow, minW, align := fieldStyle(c.kind)
		if basis != c.basis || grow != c.grow || minW != c.minW || align != c.align {
			t.Errorf("fieldStyle(%s) = (%v,%v,%v,%v), want (%v,%v,%v,%v)",
				c.kind, basis, grow, minW, align, c.basis, c.grow, c.minW, c.align)
		}
	}
}

func TestResolveFieldWidths(t *testing.T) {
	// 三列：basis 之和 = 100%，无富余，列宽就是各自比例。
	three := []cardField{
		{Kind: fieldAuthor, Text: "UP"},
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldPubdate, Text: "09-22"},
	}
	got := resolveFieldWidths(three, 230)
	want := []float32{92, 69, 69}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("三列宽度[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	// 两列：富余空间按 grow 比例分完（列仍填满整行）。
	two := []cardField{
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldDuration, Text: "12:30"},
	}
	got = resolveFieldWidths(two, 300)
	if got[0] != 150 || got[1] != 150 {
		t.Errorf("两列宽度 = %v, want [150 150]", got)
	}

	// 发布时间列 grow 4（大于播放量的 3），富余里能「借」到更宽。
	two = []cardField{
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldPubdate, Text: "2024-03-10"},
	}
	got = resolveFieldWidths(two, 300)
	if got[1] <= got[0] {
		t.Errorf("日期列应借到比播放量更宽: %v", got)
	}
	if diff := (got[0] + got[1]) - 300; diff > 0.01 || diff < -0.01 {
		t.Errorf("两列未填满整行: %v", got)
	}

	// 窄于阈值：整列隐藏（宽 0），富余只分给剩下的列。
	narrow := []cardField{
		{Kind: fieldAuthor, Text: "UP"},
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldExtra, Text: "已看 30%"},
	}
	got = resolveFieldWidths(narrow, 200) // extra 阈值 260 → 隐藏
	if got[2] != 0 {
		t.Errorf("窄卡 extra 列宽 = %v, want 0", got[2])
	}
	// author(40) + views(30) 基准 70% = 140，富余 60 按 grow 4:3 分。
	if diff := (got[0] + got[1]) - 200; diff > 0.01 || diff < -0.01 {
		t.Errorf("剩余两列未填满整行: %v", got)
	}

	// 四列：basis 之和 130% > 行宽，按比例收缩到恰好填满（原版 flex-shrink）。
	four := []cardField{
		{Kind: fieldAuthor, Text: "UP"},
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldPubdate, Text: "2024-03-10"},
		{Kind: fieldExtra, Text: "已看 30%"},
	}
	got = resolveFieldWidths(four, 300)
	var sum float32
	for _, w := range got {
		sum += w
	}
	if sum > 300.01 || sum < 299.99 {
		t.Errorf("四列总宽 = %v, want 300", sum)
	}
	if got[2] >= 90 {
		t.Errorf("四列时日期列应被压缩到 <90，got %v", got[2])
	}
}

func TestPubdateCandidates(t *testing.T) {
	now := time.Date(2025, 9, 20, 12, 0, 0, 0, time.UTC)

	// 完整年月日 → 具体到抽象阶梯：ymd → ym → N个月前 → N年前。
	got := pubdateCandidates("2024-03-10", now)
	want := []string{"2024-03-10", "2024-03", "18个月前", "1年前"}
	if len(got) != len(want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidates[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// 当月同日未到：月差少算一个月。
	got = pubdateCandidates("2024-03-25", now)
	if got[len(got)-2] != "17个月前" {
		t.Errorf("月差 = %q, want 17个月前", got[len(got)-2])
	}

	// 非完整年月日（相对形态 / MM-DD / 无法解析）原样作为唯一候选。
	for _, in := range []string{"昨天", "09-22", "2024-03", "不是日期"} {
		if c := pubdateCandidates(in, now); len(c) != 1 || c[0] != in {
			t.Errorf("pubdateCandidates(%q) = %v, want [%q]", in, c, in)
		}
	}
	if c := pubdateCandidates("  ", now); c != nil {
		t.Errorf("空白 = %v, want nil", c)
	}
}

func TestPickDateForm(t *testing.T) {
	cands := []string{"2024-03-10", "2024-03", "18个月前", "1年前"}
	// 放得下就取最具体的一档。
	if got := pickDateForm(cands, func(s string) bool { return len(s) <= 7 }); got != "2024-03" {
		t.Errorf("pickDateForm(width 7) = %q, want 2024-03", got)
	}
	// 一个都放不下：兜底取最后一档，绝不返回空。
	if got := pickDateForm(cands, func(string) bool { return false }); got != "1年前" {
		t.Errorf("pickDateForm(都不放) = %q, want 1年前", got)
	}
	if got := pickDateForm(nil, func(string) bool { return true }); got != "" {
		t.Errorf("pickDateForm(空) = %q, want empty", got)
	}
}
