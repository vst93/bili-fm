package view

import (
	"github.com/egoist/mygo/ui"
	"image"
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

// TestStatButtonNoActiveBackground 钉住「赞/币/藏三键任何状态都不铺底色」。
//
// 原版 JSX 只把 svg 的 fill 在激活时换成动作色，globals.css 里没有任何
// `.nav-stat-btn[data-active]` / `.nav-icon-btn[aria-pressed]` 的面规则 ——
// 已赞/已投币/已收藏的按钮底仍是通用 `.nav-icon-btn` 的「常态透明、悬停
// 白 .38、按下白 .16」。原生版此前给激活态铺 `ActiveInk.Alpha(0.26)` 的着色面
// + 1px 白边（dd2cb91 的遗留），已赞是红底、已投币是琥珀底、已收藏是黄底加白框，
// 与只有图标变色的原版完全对不上。
//
// 无头实测：把三个键的 fav/like/coin 状态从「未激活」切到「已激活」，按钮
// 矩形内除图标笔画外应**没有任何**成片的底/边颜色出现 —— 即激活不该新增底色。
// 判据：三种激活态的按钮内像素与「未激活」同态几乎逐像素相同，只在图标颜色上
// 有别（图标墨迹只占按钮的小部分）。这里直接量「按钮底/边区域」（图标笔画之外
// 的环带）的像素差：激活态不应在环带里新增任何着色底。
func TestStatButtonNoActiveBackground(t *testing.T) {
	render := func(active bool) *image.RGBA {
		a := descApp()
		a.Info.Like, a.Info.Coin, a.Info.Favorite = 1234, 5678, 9012
		a.Liked, a.Coined, a.Faved = active, active, active
		return shellTesterSize(t, a, 1000, 700).Image()
	}

	off := render(false)
	on := render(true)

	// 三个按钮的矩形按已知布局几何定位：34 宽、圆角 8、间距 4，行首是「点赞」。
	// 标签（aria-label）随激活态切换（点赞↔取消点赞），Find 靠不住，所以用
	// 未激活态的「点赞」定锚、再按 gap 推出后两个。
	anchor := ui.NewTester(func(c *ui.Context) { descApp().Shell(c) }, 1000, 700)
	anchor.Frame()
	like, ok := anchor.Find("点赞")
	if !ok {
		t.Fatal("找不到点赞按钮")
	}
	btns := []ui.Rect{
		like,
		{X: like.X + ContentIcon + 4, Y: like.Y, W: ContentIcon, H: ContentIcon},
		{X: like.X + 2*(ContentIcon+4), Y: like.Y, W: ContentIcon, H: ContentIcon},
	}
	names := []string{"点赞", "投币", "收藏"}

	// 每个按钮取上缘 3px 环带（避开正中的图标笔画）：底 / 边框若有变化，环带里
	// 会大量显现；只换图标 fill 时环带（图标不触及）应几乎不变。
	for i, r := range btns {
		ring := ui.Rect{X: r.X, Y: r.Y, W: r.W, H: 3}
		diff, total := diffInRect(off, on, ring)
		t.Logf("%s 上缘环带：%d/%d 像素随激活变化", names[i], diff, total)
		if diff > total/4 {
			t.Errorf("%s 激活后在按钮上缘环带里新增了 %d/%d 像素 —— 激活态仍在铺底/画边框（原版只有图标变色）",
				names[i], diff, total)
		}
		// 计数条（按钮下方 13px）：原版计数色恒 #64748b，激活不改；此前原生版
		// 让激活态计数跟图标一起染动作色（红/琥珀/黄）。两态的计数数值相同
		//（1234/5678/9012），所以计数条应逐像素一致。
		strip := ui.Rect{X: r.X, Y: r.Y + r.H, W: r.W, H: 13}
		diffCount, totalCount := diffInRect(off, on, strip)
		t.Logf("%s 计数条：%d/%d 像素随激活变化", names[i], diffCount, totalCount)
		if diffCount > totalCount/8 {
			t.Errorf("%s 激活后计数条变了 %d/%d 像素 —— 激活态计数仍在染动作色（原版恒 #64748b）",
				names[i], diffCount, totalCount)
		}
	}
}
