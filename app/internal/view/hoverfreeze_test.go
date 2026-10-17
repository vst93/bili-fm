package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// hfRec 是一次构建 pass 的观测量：滚动锁是否按住、这一 pass 里有多少张卡 /
// 行铺了悬停态底色。
type hfRec struct {
	held  bool
	hover int64
}

// hoverProbe 起一个无头窗口跑真实 App.Shell，并逐 pass 记录锁定状态与
// 悬停底色的铺设次数。
func hoverProbe(t *testing.T, a *App) (*ui.Tester, *[]hfRec) {
	t.Helper()
	var recs []hfRec
	tt := ui.NewTester(func(c *ui.Context) {
		a.hoverChannels.Store(0)
		a.Shell(c)
		recs = append(recs, hfRec{held: a.scrollHoldHeld, hover: a.hoverChannels.Load()})
	}, 800, 600)
	tt.Advance(400 * time.Millisecond) // 推过抽屉入场（v0.5.5 冻结时钟）
	tt.Frame()
	return tt, &recs
}

// TestScrollHoldFreezesCardHover 端到端（无头）：指针停在网格里滚动时，
// 每一 pass 都不铺悬停态底色（滚动期间 hover 视觉被完全冻住，而不只是锁
// 上浮位移）——这正是用户报的「抽屉卡片滚动时选中抖动」的根因：卡片在指针
// 下滑过，hover 底/边/页脚洗色每帧翻转。停下后悬停态恢复。
func TestScrollHoldFreezesCardHover(t *testing.T) {
	a := feedApp(300)
	tt, recs := hoverProbe(t, a)
	tt.Move(400, 300)
	for i := 0; i < 10; i++ {
		tt.Frame()
	}
	// 静止时指针在卡片上：应当铺了悬停底色（证明样本点确实命中卡片）。
	*recs = (*recs)[:0]
	tt.Frame()
	if settled := (*recs)[len(*recs)-1]; settled.hover == 0 {
		t.Fatal("静止时指针下的卡片没有悬停底色——样本点没命中卡片，测试无效")
	}

	// 滚动周期：按住期间悬停底色必须为 0。
	*recs = (*recs)[:0]
	for i := 0; i < 4; i++ {
		tt.Scroll(400, 300, 0, 120)
	}
	var heldPasses, lockedHover int64
	for _, r := range *recs {
		if r.held {
			heldPasses++
			lockedHover += r.hover
		}
	}
	if heldPasses == 0 {
		t.Fatal("真实滚动没有触发滚动锁（holdListScroll 没接上 ui.List 的偏移）")
	}
	if lockedHover != 0 {
		t.Fatalf("滚动锁住期间仍铺了 %d 次悬停底色（hover 没被冻结）", lockedHover)
	}

	// 停下后再跑几帧：悬停底色恢复。
	*recs = (*recs)[:0]
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	var restHover int64
	for _, r := range *recs {
		restHover += r.hover
	}
	if restHover == 0 {
		t.Fatal("滚动停下后悬停底色没有恢复（视觉被误删）")
	}
	t.Logf("滚动周期：按住 %d pass、悬停底色 0 次 → 停下后恢复 %d 次", heldPasses, restHover)
}

// TestScrollHoldFreezesPlaylistRowHover 同理覆盖播放列表行（同一套滚动锁，
// 行在指针下滑过时底/边/删除键显隐也会翻转）。
func TestScrollHoldFreezesPlaylistRowHover(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	items := make([]PlayItem, 60)
	for i := range items {
		items[i] = PlayItem{ID: string(rune('a'+i%26)) + string(rune('0'+i/26)), Title: "T", Part: "P"}
	}
	a.Playlist = items
	a.PlaylistTab = ListUser
	a.Drawer = DrawerPlaylist
	tt, recs := hoverProbe(t, a)
	tt.Move(400, 300)
	for i := 0; i < 10; i++ {
		tt.Frame()
	}
	*recs = (*recs)[:0]
	for i := 0; i < 4; i++ {
		tt.Scroll(400, 300, 0, 120)
	}
	var heldPasses, lockedHover int64
	for _, r := range *recs {
		if r.held {
			heldPasses++
			lockedHover += r.hover
		}
	}
	if heldPasses == 0 {
		t.Fatal("播放列表滚动没有触发滚动锁")
	}
	if lockedHover != 0 {
		t.Fatalf("播放列表滚动锁住期间仍铺了 %d 次行悬停底色", lockedHover)
	}
}
