package view

// 原生视频窗「离开」语义的无头测试（微轮 4a）：顶栏三颗键、关闭只此一颗、
// 退出视频播放不退应用、最小化不暂停。
//
// 为什么这些断言要落在真实 Tester 的点击路径上：用户报的三条（重复关闭键、
// 右上角退出整个应用、最小化）都是**点击**出来的，只测 videoKeyFor 之类的
// 纯函数永远发现不了「两颗键接了同一条动作链」。

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// videoChromeCalls 记外壳三条动作链各被触发几次。
// 用原子量：OnQuit 走的是 `go v.OnQuit()`（键盘退出不阻塞界面帧），
// 普通 int 在这里就是数据竞争。
type videoChromeCalls struct {
	close, min, quit atomic.Int32
}

// waitFor 等一个计数到 want（最多 1s）。OnQuit 是 goroutine，点完那一帧
// 不保证已经跑过。
func (c *videoChromeCalls) waitFor(t *testing.T, name string, got *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got.Load() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Errorf("%s 调用 %d 次，想要 %d", name, got.Load(), want)
}

// videoChromeCloseProbe 起一个带动作桩的无头视频窗：播放中、100s 时长。
// 三个回调各记次数，方便断言「点的是哪一条动作链」。
func videoChromeCloseProbe(t *testing.T, w, h int) (*VideoChrome, *videoStubPlayer, *ui.Tester, *videoChromeCalls) {
	t.Helper()
	pl := &videoStubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second, rate: 1, vol: 1}
	calls := &videoChromeCalls{}
	ch := &VideoChrome{
		Theme:      ThemeFor(PeriodAt(12)),
		Title:      "测试稿件",
		Pl:         pl,
		Win:        newVideoStubWindow(),
		OnClose:    func() { calls.close.Add(1) },
		OnMinimize: func() { calls.min.Add(1) },
		OnQuit:     func() { calls.quit.Add(1) },
	}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, w, h)
	tt.Frame()
	return ch, pl, tt, calls
}

// countText 数一段文字在整窗里出现了几次（按钮的 Label 会出现在 Texts 里）。
func countText(tt *ui.Tester, want string) int {
	return strings.Count(strings.Join(tt.Texts(), "\x00"), want)
}

// TestVideoChromeTitleBarHasThreeKeys 钉住顶栏的构成：**弹幕 / 最小化 /
// 退出视频播放**，而且关闭键全窗只有一颗（用户报的「上下各有一个关闭按钮」）。
//
// 同时覆盖最窄窗口（320×200 是窗口的 MinWidth/MinHeight）：三颗键与标题
// 挤在一行时标题可以被省略号截断，键本身不能被挤掉。
func TestVideoChromeTitleBarHasThreeKeys(t *testing.T) {
	for _, size := range []struct{ w, h int }{{640, 360}, {320, 200}} {
		_, _, tt, _ := videoChromeCloseProbe(t, size.w, size.h)
		for _, want := range []string{"弹幕", "最小化", "退出视频播放"} {
			if n := countText(tt, want); n != 1 {
				t.Errorf("%dx%d：顶栏键 %q 出现 %d 次，想要 1（现有：%v）",
					size.w, size.h, want, n, tt.Texts())
			}
		}
		if n := countText(tt, "退出视频播放"); n != 1 {
			t.Errorf("%dx%d：关闭键出现 %d 次，想要 1（控件条不许再有一颗）", size.w, size.h, n)
		}
		if tt.HasText("关闭视频") {
			t.Errorf("%dx%d：旧文案「关闭视频」还在：%v", size.w, size.h, tt.Texts())
		}
	}
}

// TestVideoChromeExitVideoDoesNotQuit 钉住用户报的最严重那条：点视频窗的
// 关闭**绝不能**退应用。
//
// 用桩断言「点了哪条动作链」：OnClose（退出视频播放）必须恰好一次，
// OnQuit（退出应用）必须零次。Cmd+Q 那条仍要能走 OnQuit —— 否则这个测试
// 是空转的（把 OnQuit 整个摘掉它也会绿）。
func TestVideoChromeExitVideoDoesNotQuit(t *testing.T) {
	_, _, tt, calls := videoChromeCloseProbe(t, 640, 360)
	if err := tt.Click("退出视频播放"); err != nil {
		t.Fatalf("点「退出视频播放」失败：%v", err)
	}
	tt.Frame()
	if got := calls.close.Load(); got != 1 {
		t.Errorf("退出视频播放：OnClose 调用 %d 次，想要 1", got)
	}
	if got := calls.quit.Load(); got != 0 {
		t.Errorf("点视频窗的关闭触发了 OnQuit（%d 次）—— 这就是用户报的「退出整个应用」", got)
	}
	if got := calls.min.Load(); got != 0 {
		t.Errorf("退出视频播放误触了最小化（%d 次）", got)
	}

	// 键盘那条（系统级 Cmd/Ctrl+Q）仍在：它才是唯一的退出应用入口，
	// 按钮一律不碰。
	_, _, tt2, calls2 := videoChromeCloseProbe(t, 640, 360)
	tt2.Key(ui.Cmd, ui.KeyQ)
	tt2.Frame()
	calls2.waitFor(t, "Cmd+Q → OnQuit", &calls2.quit, 1)
	if got := calls2.close.Load(); got != 0 {
		t.Errorf("Cmd+Q 误走了「退出视频播放」（%d 次）", got)
	}
}

// TestVideoChromeMinimizeKeepsPlayback 钉住「最小化 ≠ 暂停」：最小化只动
// 窗口，播放器一个动作都不许收到（Pause / Play / Seek 全为零），几帧之后
// 状态也不许变 —— 声音与进度照常往前走。
func TestVideoChromeMinimizeKeepsPlayback(t *testing.T) {
	ch, pl, tt, calls := videoChromeCloseProbe(t, 640, 360)
	before := *pl
	if err := tt.Click("最小化"); err != nil {
		t.Fatalf("点最小化失败：%v", err)
	}
	tt.Frame()
	if got := calls.min.Load(); got != 1 {
		t.Errorf("最小化：OnMinimize 调用 %d 次，想要 1", got)
	}
	if c, q := calls.close.Load(), calls.quit.Load(); c != 0 || q != 0 {
		t.Errorf("最小化误触了关闭/退出：close=%d quit=%d", c, q)
	}
	if pl.playing != before.playing || pl.pauses != before.pauses || pl.plays != before.plays ||
		pl.pos != before.pos || len(pl.seeks) != len(before.seeks) {
		t.Errorf("最小化改动了播放器：before=%+v after=%+v", before, *pl)
	}
	// 「恢复」在无头窗口里就是再建几帧：状态必须原样接续。
	for i := 0; i < 3; i++ {
		tt.Advance(16 * time.Millisecond)
	}
	if !pl.playing || pl.pauses != before.pauses {
		t.Errorf("恢复后播放状态变了：playing=%v pauses=%d", pl.playing, pl.pauses)
	}
	if set := ch.settings(); set.Speed != 1 || set.Volume != 1 {
		t.Errorf("恢复后倍速/音量变了：%v / %v", set.Speed, set.Volume)
	}
}

// TestVideoChromeButtonsNeverReachQuit 是源码契约：外壳里 OnQuit 只能出现在
// `do(videoKeyQuit)` 这一条（键盘系统级退出），任何画键的代码（titleBar /
// controlBar / controlRow / controlRows）都不许碰它。
//
// 行为测试能证明「今天点的这一颗没退应用」，这条守住的是「以后新加的键
// 别又接到 quit 上」。
func TestVideoChromeButtonsNeverReachQuit(t *testing.T) {
	flat := flattenNoComments(t, "videochrome.go")
	// 一处 nil 判断 + 一处调用，都在 do(videoKeyQuit) 里。
	if got := strings.Count(flat, "v.OnQuit"); got != 2 {
		t.Fatalf("videochrome.go 里 v.OnQuit 出现 %d 次，想要 2（只在 do(videoKeyQuit) 里）", got)
	}
	if !strings.Contains(flat, "casevideoKeyQuit:ifv.OnQuit!=nil{gov.OnQuit()}") {
		t.Error("唯一的 OnQuit 调用点不是 do(videoKeyQuit)：键盘退出的接线被改坏了")
	}
	// 顶栏那颗必须走 close()（退出视频播放），不是 do(videoKeyQuit)。
	if !strings.Contains(flat, `"video-exit","退出视频播放","退出视频播放"`) {
		t.Error("顶栏第三颗键不是「退出视频播放」")
	}
	// 控件条不许再有第二颗关闭键。
	if strings.Contains(flat, `"video-close"`) {
		t.Error("控件条又出现了一颗关闭键（用户报的上下重复）")
	}
}
