package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// shellTester 起一个 800×600 的无头窗口，跑**真实的** App.Shell，
// 用来验证全局快捷键（纯函数测试看不到按键路由，那正是这一轮修的东西）。
func shellTester(t *testing.T, a *App) *ui.Tester {
	t.Helper()
	return settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600))
}

// focusedInfo 给主区一份「正在播放的视频」，让点赞/投币/收藏这三个按钮可用
// （没有视频时它们是禁用态，禁用按钮不参与焦点）。
func focusedInfo(a *App) {
	a.Info = &Info{Bvid: "BV1test", Aid: 1, Cid: 2, Title: "标题", OwnerMid: 5, OwnerName: "UP"}
}

// TestSpaceBeatsFocusedButton 是本轮修的回归：
// 原版在 keydown 捕获阶段把空格从聚焦按钮手里抢回来（handleSpaceKeyIntercept），
// 空格永远只管播放/暂停。mygo 的默认规矩正好相反（聚焦的按钮保留空格），
// 于是「点过任意按钮之后再按空格」会变成再点一次那个按钮。
func TestSpaceBeatsFocusedButton(t *testing.T) {
	a := NewApp(func() {})
	focusedInfo(a)
	plays, likes := 0, 0
	a.Act.TogglePlay = func() { plays++ }
	a.Act.Like = func() { likes++ }

	tt := shellTester(t, a)
	if err := tt.Click("点赞"); err != nil {
		t.Fatalf("点不到点赞按钮：%v", err)
	}
	tt.Frame()
	if likes != 1 {
		t.Fatalf("鼠标点一下点赞：likes=%d，想 1", likes)
	}

	// 点赞按钮此刻有焦点。空格该切播放，不该再点一次点赞。
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if plays != 1 || likes != 1 {
		t.Errorf("聚焦按钮后按空格：plays=%d（想 1）、likes=%d（想 1）"+
			"—— 空格被聚焦的按钮吃掉了（原版会 preventDefault 抢回来）", plays, likes)
	}
}

// TestSpaceWithoutFocus 是窗口级那份注册的回归：什么都没聚焦时（刚启动、
// 点过空白处）空格也要能用 —— 此时焦点链是空的，找不到根元素的注册。
func TestSpaceWithoutFocus(t *testing.T) {
	a := NewApp(func() {})
	plays := 0
	a.Act.TogglePlay = func() { plays++ }

	tt := shellTester(t, a)
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if plays != 1 {
		t.Errorf("没有焦点时按空格：plays=%d，想 1", plays)
	}
}

// TestSpaceInSearchField 保证「抢按钮的空格」没有连输入框一起抢：
// 在搜索框里按空格应该只是打一个空格（原版也是排除 input/textarea 的）。
func TestSpaceInSearchField(t *testing.T) {
	a := NewApp(func() {})
	plays := 0
	a.Act.TogglePlay = func() { plays++ }

	tt := shellTester(t, a)
	// Tab 到搜索输入框（标题栏的键（设置 / 隐身模式）现在也可聚焦，Tab 次数不再
	// 固定；改成按下 Tab 直到光标进入输入框，最多试 8 次，这样新增可聚焦控件
	// 不会把测试变成「数 Tab 次数」的脆弱断言）。
	focused := false
	for i := 0; i < 8; i++ {
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		if _, ok := tt.TextCaret(); ok {
			focused = true
			break
		}
	}
	if !focused {
		t.Fatal("Tab 多次后焦点仍未落到搜索输入框上（找不到可编辑光标）")
	}
	tt.TypeKey(0, ui.KeySpace, " ")
	tt.Frame()
	if a.Query != " " {
		t.Errorf("搜索框里按空格：Query=%q，想 %q", a.Query, " ")
	}
	if plays != 0 {
		t.Errorf("搜索框里按空格不该切播放：plays=%d，想 0", plays)
	}
}

// TestArrowShortcuts 钉住方向键仍走窗口级：按钮不吃方向键，所以点过按钮
// 之后 ←/→ 照常上一集/下一集（只有空格需要根元素那份注册）。
func TestArrowShortcuts(t *testing.T) {
	a := NewApp(func() {})
	focusedInfo(a)
	prev, next, likes := 0, 0, 0
	a.Act.Prev = func() { prev++ }
	a.Act.Next = func() { next++ }
	a.Act.Like = func() { likes++ }

	tt := shellTester(t, a)
	if err := tt.Click("点赞"); err != nil {
		t.Fatalf("点不到点赞按钮：%v", err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if prev != 1 || next != 1 || likes != 1 {
		t.Errorf("聚焦按钮后按方向键：prev=%d next=%d likes=%d，想 1/1/1", prev, next, likes)
	}
}

// TestMinimizeQuitShortcuts 补上原版的 Ctrl/Cmd+W（最小化）与 Ctrl/Cmd+Q（退出）：
// 「快捷键」对话框里一直写着这两条，之前只有托盘/菜单入口。
func TestMinimizeQuitShortcuts(t *testing.T) {
	a := NewApp(func() {})
	min, quit := 0, 0
	a.Act.Minimize = func() { min++ }
	a.Act.Quit = func() { quit++ }

	tt := shellTester(t, a)
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyQ)
	tt.Frame()
	if min != 1 || quit != 1 {
		t.Errorf("Ctrl/Cmd+W→最小化 %d 次、Ctrl/Cmd+Q→退出 %d 次，想各 1 次", min, quit)
	}
}

// TestEscapeShortcutStillClosesDrawer 保证改注册方式没动 Esc 的窗口级行为。
func TestEscapeShortcutStillClosesDrawer(t *testing.T) {
	a := NewApp(func() {})
	a.Drawer = DrawerSearch

	tt := shellTester(t, a)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.Drawer != "" {
		t.Errorf("Esc 没关掉抽屉：Drawer=%q", a.Drawer)
	}
}

// TestVideoOpenBlocksTrackAndVolumeShortcuts 对齐原版的「播放视频时屏蔽快捷键」：
// index.tsx 的 handleKeyPress 在 isPlayVideo 分支里对 ← → ↑ ↓ 直接 return，
// 因为视频弹窗自带进度 / 音量控件，主窗再切集、调音量会互相打架。
// 原生版此前无条件响应这四颗键（弹窗开着、用户点回主窗时就会误触）。
func TestVideoOpenBlocksTrackAndVolumeShortcuts(t *testing.T) {
	a := NewApp(func() {})
	focusedInfo(a)
	prev, next, vol := 0, 0, 0
	a.Act.Prev = func() { prev++ }
	a.Act.Next = func() { next++ }
	a.Act.SetVolume = func(float64) { vol++ }
	a.VideoOpen = true

	tt := shellTester(t, a)
	for _, k := range []ui.Key{ui.KeyLeft, ui.KeyRight, ui.KeyUp, ui.KeyDown} {
		tt.Key(0, k)
		tt.Frame()
	}
	if prev != 0 || next != 0 || vol != 0 {
		t.Errorf("视频弹窗开着时主窗仍响应方向键：prev=%d next=%d vol=%d，想全 0"+
			"（原版 isPlayVideo 分支屏蔽这些快捷键）", prev, next, vol)
	}

	// 对照：弹窗关掉后这些键恢复。
	a.VideoOpen = false
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if prev != 1 || next != 1 || vol != 1 {
		t.Errorf("弹窗关掉后方向键应恢复：prev=%d next=%d vol=%d，想 1/1/1", prev, next, vol)
	}
}

// TestVideoOpenEscapeClosesAndSpaceToggles 保证屏蔽没有连 Esc / 空格一起屏蔽：
// 原版视频模式下 Esc 关浮层、空格切视频播放 / 暂停，正是这两颗键的语义变了。
func TestVideoOpenEscapeClosesAndSpaceToggles(t *testing.T) {
	a := NewApp(func() {})
	a.VideoOpen = true
	closed, plays := 0, 0
	a.Act.CloseVideo = func() { closed++ }
	a.Act.TogglePlay = func() { plays++ }

	tt := shellTester(t, a)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if closed != 1 {
		t.Errorf("视频弹窗开着时按 Esc：CloseVideo 调用 %d 次，想 1", closed)
	}
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if plays != 1 {
		t.Errorf("视频弹窗开着时按空格：TogglePlay 调用 %d 次，想 1", plays)
	}
}

// TestHeldKeySemantics 钉住 mygo 0.8.0 的 `Context.ShortcutRepeated` 用法：
// 开关/动作只吃首次按下，步进（音量）吃每一次自动重复。
//
// 此前所有快捷键都吃重复，于是按住空格会把播放/暂停来回切十几次、按住 ← 会
// 一路退回整个队列；而按住 ↑ ↓ 调音量反而更自然（与原生 range 一致）。
func TestHeldKeySemantics(t *testing.T) {
	a := NewApp(func() {})
	focusedInfo(a)
	plays, prev, next := 0, 0, 0
	vol := 0.0
	a.Volume = 0.5
	a.Act.TogglePlay = func() { plays++ }
	a.Act.Prev = func() { prev++ }
	a.Act.Next = func() { next++ }
	a.Act.SetVolume = func(v float64) { vol = v }

	tt := shellTester(t, a)

	// 空格：按下 + 两次自动重复 → 只切一次。
	tt.KeyDown(ui.KeySpace)
	tt.KeyRepeat(0, ui.KeySpace)
	tt.KeyRepeat(0, ui.KeySpace)
	tt.KeyUp(ui.KeySpace)
	tt.Frame()
	if plays != 1 {
		t.Errorf("按住空格：播放/暂停被切了 %d 次，想 1 次（自动重复应被 ShortcutRepeated 挡掉）", plays)
	}

	// ← →：按下 + 两次重复 → 各只跳一次。
	tt.KeyDown(ui.KeyLeft)
	tt.KeyRepeat(0, ui.KeyLeft)
	tt.KeyRepeat(0, ui.KeyLeft)
	tt.KeyUp(ui.KeyLeft)
	tt.Frame()
	tt.KeyDown(ui.KeyRight)
	tt.KeyRepeat(0, ui.KeyRight)
	tt.KeyUp(ui.KeyRight)
	tt.Frame()
	if prev != 1 || next != 1 {
		t.Errorf("按住方向键：prev=%d next=%d，想各 1 次（切集是动作，不吃重复）", prev, next)
	}

	// ↑：按下 + 两次重复 → 音量连续加三档（步进，吃重复）。
	tt.KeyDown(ui.KeyUp)
	tt.KeyRepeat(0, ui.KeyUp)
	tt.KeyRepeat(0, ui.KeyUp)
	tt.KeyUp(ui.KeyUp)
	tt.Frame()
	if want := 0.65; vol < want-0.001 || vol > want+0.001 {
		t.Errorf("按住 ↑ 三下：音量 = %v，想 %v（步进应吃自动重复）", vol, want)
	}
}
