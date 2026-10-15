package view

import (
	"math"
	"testing"
)

// 本轮（操作行计数绝对定位）的测试。
//
// 原版 globals.css 把赞/币/藏的计数画成按钮的**绝对定位子元素**：
//
//	.nav-icon-btn > .nav-stat-value {
//	    position: absolute; top: calc(100% + 1px);
//	    left: 50%; transform: translateX(-50%);
//	    font-size: 9px; line-height: 12px; ...
//	}
//	.video-content-actions { padding: 0 0 14px; }  /* 给计数预留展示区 */
//
// 计数不占布局高度：一行按钮的行高只有 34（按钮）+ 14（padding-bottom），
// 有数字/没数字的按钮图标都在同一水平中轴线上。原生版此前把计数画成按钮
// 下方的等高列（Column 高 47），行高变成 47 + 14 = 61，把下面的工具条整体
// 顶下去 13px，也是右栏内容比固定盒高 366 多出 3px 的根因。

// statCountApp 造一个宽窗主界面，三个互动键的计数各不相同，便于按文案定位。
// 已收藏：内容操作行的收藏键文案变成「取消收藏」，与搜索栏的「收藏」入口区分开
// （两者都叫「收藏」时 Find 只能拿到第一个）。
func statCountApp() *App {
	a := descApp()
	a.Faved = true
	a.Info.Like = 1234
	a.Info.Coin = 5678
	a.Info.Favorite = 9012
	return a
}

// TestStatCountAttachedBelowButton 钉住计数的绝对定位：上缘 = 按钮下缘 + 1px
// （原版 top: calc(100% + 1px)），且水平居中（left 50% + translateX(-50%)）。
func TestStatCountAttachedBelowButton(t *testing.T) {
	tt := shellTesterSize(t, statCountApp(), 1000, 700)

	cases := []struct{ btn, count string }{
		{"点赞", "1234"},
		{"投币(2个)", "5678"},
		{"取消收藏", "9012"},
	}
	for _, tc := range cases {
		btn, ok := tt.Find(tc.btn)
		if !ok {
			t.Fatalf("找不到按钮 %q", tc.btn)
		}
		cnt, ok := tt.Find(tc.count)
		if !ok {
			t.Fatalf("找不到计数 %q", tc.count)
		}
		t.Logf("%s：按钮 %+v / 计数 %+v", tc.btn, btn, cnt)
		// 计数上缘 = 按钮下缘 + 1 + statCountShift（原版 top: calc(100% + 1px)；
		// statCountShift 是字形位移，为负值，把整个计数盒也一起上移，
		// 见 home.go 的 statCountBoxH/statCountShift）。
		if got, want := cnt.Y, btn.Y+btn.H+1+statCountShift; got != want {
			t.Errorf("%s 计数上缘 Y = %v, want %v（top: calc(100%% + 1px)，再叠字形位移 %v）",
				tc.btn, got, want, statCountShift)
		}
		bcx, ccx := btn.X+btn.W/2, cnt.X+cnt.W/2
		if math.Abs(float64(bcx-ccx)) > 0.5 {
			t.Errorf("%s 计数未水平居中：按钮中心 %v，计数中心 %v", tc.btn, bcx, ccx)
		}
	}
}

// TestStatCountDoesNotGrowActionsRow 是核心反证：计数若占布局高度，操作行会
// 从 34 + 14 撑到 47 + 14，把下方工具条顶下去 13px。这里量「点赞按钮下缘 →
// 工具条按钮上缘」的距离，应当只有 padding-bottom 14 + 工具条 margin-top 12 +
// 工具条 1px 边框 + 内边距 3 = 30。
func TestStatCountDoesNotGrowActionsRow(t *testing.T) {
	tt := shellTesterSize(t, statCountApp(), 1000, 700)

	like, ok := tt.Find("点赞")
	if !ok {
		t.Fatal("找不到点赞按钮")
	}
	dock, ok := tt.Find("搜索结果")
	if !ok {
		t.Fatal("找不到工具条按钮「搜索结果」")
	}
	gap := dock.Y - (like.Y + like.H)
	t.Logf("点赞按钮 %+v，工具条按钮 %+v，间距 %v", like, dock, gap)

	// 行高 = 按钮 34 + padding-bottom 14；工具条再 margin-top 12 + 边框 1 +
	// padding 3。计数是绝对定位，不参与这 30px 的构成。
	const want = 30
	if gap != want {
		t.Errorf("点赞按钮到工具条的间距 = %v, want %v（计数不占布局高时应为 34+14 之后的行距）",
			gap, want)
	}
	// 老实现（计数等高列，比按钮高 13px）会让间距变成 43；显式排除。
	if gap >= 40 {
		t.Errorf("间距 %v 偏大，像计数仍在撑高操作行", gap)
	}
}
