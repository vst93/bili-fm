package view

import "testing"

// TestUpFollowSpec 钉住 UP 空间表头关注键的文案与转圈（原版 upVideoList）：
//
//	查关注状态 / 关注请求在途（pending）→ 转圈（文案仍是「关注」）
//	已关注                             → 「已关注」
//	否则                               → 「关注」
func TestUpFollowSpec(t *testing.T) {
	cases := []struct {
		pending, followed bool
		label             string
		spinner           bool
	}{
		{true, false, "关注", true},
		{true, true, "关注", true}, // 查询中即使已知已关注也先转圈
		{false, true, "已关注", false},
		{false, false, "关注", false},
	}
	for _, c := range cases {
		label, spinner := upFollowSpec(c.pending, c.followed)
		if label != c.label || spinner != c.spinner {
			t.Errorf("upFollowSpec(%v,%v) = (%q,%v), want (%q,%v)",
				c.pending, c.followed, label, spinner, c.label, c.spinner)
		}
	}
}

// TestUpFollowButtonDisabledWhilePending 钉住「关注状态查询 / 请求在途时关注键
// 禁用」：原版关注键在 isCheckingFollow / isFollowingLoading 时是禁用态，
// 连点不会重复发请求（否则 follow + unfollow 各发一次，状态来回跳）。
//
// 用无头 tester 真点一次：pending 时点不动（ToggleFollow 不触发），非 pending
// 时正常触发。
func TestUpFollowButtonDisabledWhilePending(t *testing.T) {
	render := func(pending bool) int {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Drawer = DrawerUp
		a.UpMid = 7
		a.UpName = "某UP"
		a.UpFollowPending = pending
		n := 0
		a.Act.ToggleFollow = func() { n++ }
		tt := shellTester(t, a)
		// 禁用键点不到（Click 返回错误），忽略返回值。
		_ = tt.Click("关注")
		tt.Frame()
		return n
	}

	if n := render(true); n != 0 {
		t.Fatalf("关注状态查询中点击关注键触发了请求：n=%d，想 0", n)
	}
	if n := render(false); n != 1 {
		t.Fatalf("非 pending 时点击关注键：n=%d，想 1", n)
	}
}
