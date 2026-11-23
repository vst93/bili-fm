package view

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestVideoChromeHoverDoesNotStealTimer 钉住「悬停不排帧」这条纪律。
//
// mygo 一个窗口只有一个 After 定时器（先到先得）。悬停过渡如果在 View 里
// 排 After，就会把控件条自动隐藏的计时反复顶掉：alpha 一抖，整片 UI 都在
// 闪（用户实测「之前闪烁的问题又出来了」）。所以这里要求：指针停在按钮上
// 时不排任何帧，控件条该淡出就照常淡出。
func TestVideoChromeHoverDoesNotStealTimer(t *testing.T) {
	ch, _, _, tt := videoChromeProbe(t, 640, 360)
	r, ok := tt.Find("视频设置")
	if !ok {
		t.Fatalf("找不到设置键")
	}
	// 指针停在按钮上（悬停材质过渡靠别的帧带着推进，这里不排帧）。
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	// 指针离开控件条、静止超过阈值：控件条必须能正常淡出到底。
	tt.Move(320, 120)
	tt.Frame()
	tt.Advance(videoChromeIdleDelay + videoChromeHideFade + 100*time.Millisecond)
	tt.Frame()
	if ch.idle.alpha != 0 {
		t.Errorf("指针离开后控件条没有淡出到底（alpha=%v）—— 定时器被顶掉了", ch.idle.alpha)
	}
}

// TestVideoChromeChromeNeverStarvesPointer 钉住「alpha 与命中测试不能互相
// 拽着抖」：淡出途中指针回到栏上，alpha 必须立刻回升并停住，不能掉到 0
// （掉到 0 的瞬间栏 IgnoreInput，按钮点不动、进度条拖不动 —— Linux 上实测
// 到的整片 UI 闪烁 + 控件失效）。
func TestVideoChromeChromeNeverStarvesPointer(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 640, 360)
	// 让淡出开始（指针离开、静止超时），但只走一小段。
	tt.Move(320, 120)
	tt.Advance(videoChromeIdleDelay + 40*time.Millisecond)
	tt.Frame()
	if ch.idle.alpha >= 1 {
		t.Fatalf("淡出没开始（alpha=%v）", ch.idle.alpha)
	}
	mid := ch.idle.alpha
	// 淡出途中把指针移回控件条并停住。
	tt.Move(320, 360-videoInset-videoChromeBarH/2)
	low := mid
	for i := 0; i < 20; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
		if ch.idle.alpha < low {
			low = ch.idle.alpha
		}
	}
	if low < mid-0.05 {
		t.Errorf("指针已在栏上，alpha 仍继续下降：%v → %v", mid, low)
	}
	if ch.idle.alpha != 1 {
		t.Errorf("指针停在栏上，alpha 没回到 1：%v", ch.idle.alpha)
	}
	// 这条栏必须真的能点：点播放键要切播放（stub 是 playing，点它是暂停）。
	key, ok := tt.Find("暂停")
	if !ok {
		t.Fatalf("找不到播放键")
	}
	before := pl.pauses
	tt.ClickAt(key.X+key.W/2, key.Y+key.H/2)
	tt.Frame()
	if pl.pauses == before {
		t.Error("栏可见时点播放键没有反应（输入被饿死）")
	}
}

// TestVideoChromeFadeKeepsHoverChain 钉住「淡出途中栏仍在指针下时，alpha
// 不许掉到 0.01 以下」—— 0.01 是栏开始 IgnoreInput 的阈值，一旦跌破，
// 命中测试立刻失效，按钮与滑块整片失灵（Linux 实测）。
func TestVideoChromeFadeKeepsHoverChain(t *testing.T) {
	ch, _, _, tt := videoChromeProbe(t, 640, 360)
	bar := float32(360) - videoInset - videoChromeBarH/2
	// 指针停在栏上，把时间推过静置阈值：hold 应当兜住，alpha 不掉。
	tt.Move(320, bar)
	tt.Frame()
	for i := 0; i < 40; i++ {
		tt.Advance(50 * time.Millisecond)
		tt.Frame()
		if ch.idle.alpha < 0.02 {
			t.Fatalf("指针在栏上，alpha 掉到 %v（栏已 IgnoreInput，控件会失灵）", ch.idle.alpha)
		}
	}
	// 指针仍在栏上，横跨静置阈值也不该掉。
	if ch.idle.alpha != 1 {
		t.Errorf("指针一直在栏上，alpha 却不是 1：%v", ch.idle.alpha)
	}
}

// TestVideoChromeIdleHoldsWhilePointerInside 直接钉 observe 的语义：只要
// 指针在栏的几何范围内（hold 为真），alpha 必须停在 1，静置计时不推进。
// 这是「不靠命中测试也能 hold」的那一半，与 pointerInChrome 一起断开
// alpha ↔ Hovered 的正反馈环。
func TestVideoChromeIdleHoldsWhilePointerInside(t *testing.T) {
	var s videoChromeIdle
	now := time.Unix(0, 0)
	// 首帧。
	s.observe(now, 320, 340, true, true)
	// 指针一动不动地停在栏上 10 秒，每 16ms 一帧。
	for i := 0; i < 625; i++ {
		now = now.Add(16 * time.Millisecond)
		f := s.observe(now, 320, 340, true, true)
		if f.Alpha != 1 {
			t.Fatalf("hold 为真时 alpha 掉到 %v（第 %d 帧）", f.Alpha, i)
		}
	}
	// hold 解除后应当开始淡出（不是永不隐藏）。
	now = now.Add(16 * time.Millisecond)
	if f := s.observe(now, 320, 120, true, false); f.Alpha != 1 {
		t.Fatalf("刚解除 hold 就跳变：%v", f.Alpha)
	}
	for i := 0; i < 400; i++ {
		now = now.Add(16 * time.Millisecond)
		s.observe(now, 320, 120, true, false)
	}
	if s.alpha > 0.02 {
		t.Errorf("指针离开栏且静止后 alpha 应收到 0，实际 %v", s.alpha)
	}
}

// TestVideoOpenButtonHighlights 钉住主页「视频播放」键的状态可读性：
// 视频窗开着时它是高亮（GlassActive 底 + 蓝色图标 + Checked），关掉后
// 回到常态灰。判据是同一份 App.VideoOpen，不另立状态。
//
// 同时钉住**只画一个图标**：高亮态早期实现是在键上再 Children 一次补一个
// 蓝图标，结果并排出现两个（用户实测到的那条）。这里数「蓝色墨迹在水平
// 方向有几团」，多画一个立刻红。
func TestVideoOpenButtonHighlights(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	probe := func(videoOpen bool) (*image.RGBA, int) {
		a := &App{Theme: ThemeFor(PeriodAt(12))}
		a.VideoOpen = videoOpen
		tt := ui.NewTester(func(c *ui.Context) {
			a.contentIconButtonInk(c, "video", "视频播放", iconVideo, iconOpticalSize,
				pick(videoOpen, ui.Hex("#3b82f6"), ui.Color{}), videoOpen, false, nil)
		}, 60, 60)
		tt.Frame()
		return tt.Image(), blueClusters(tt.Image())
	}
	_, offBlue := probe(false)
	_, onBlue := probe(true)
	if offBlue != 0 {
		t.Errorf("常态键上不该有蓝色墨迹（%d 团）", offBlue)
	}
	if onBlue != 1 {
		t.Errorf("高亮态应有且只有 1 团蓝色图标墨迹，实际 %d 团", onBlue)
	}
}

// blueClusters 数「蓝色墨迹」在水平方向的连续团数。
func blueClusters(img *image.RGBA) int {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	isBlue := func(x int) bool {
		for y := 0; y < h; y++ {
			c := img.RGBAAt(x, y)
			if c.B > 150 && int(c.B)-int(c.R) > 40 { // 明显偏蓝
				return true
			}
		}
		return false
	}
	n, in := 0, false
	for x := 0; x < w; x++ {
		if isBlue(x) {
			if !in {
				n++
				in = true
			}
		} else {
			in = false
		}
	}
	return n
}

// TestVideoChromeSeekCommitsOnReleaseOnly 钉住「拖动中一次 seek 都不发，
// 松手当帧提交一次」—— 与主窗 progressPlan **同一条**语义（用户 2026-11 的
// 要求：视频窗的进度条要和主页音频播放器一致）。
//
// 长视频上一次 seek 要几十毫秒（AVPlayer 从关键帧解码），拖动中逐位移下发
// 会把绘制线程堵死；而松手后的确认窗口 / 最小位移阈值都是视频窗私有的规则，
// 主窗没有，这里也不许有。
func TestVideoChromeSeekCommitsOnReleaseOnly(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)

	f(PlayerTimelineInput{Held: true})
	for i := 0; i < 60; i++ {
		v.seek.value = 100 + float64(i)*20
		if commit, next := f(PlayerTimelineInput{Held: true, Changed: true}); commit {
			t.Fatalf("第 %d 次位移就提交了 seek（拖动中不该下发）", i)
		} else if next != v.seek.value {
			t.Fatalf("第 %d 次位移滑块值不是用户拖到的 %v（得到 %v）", i, v.seek.value, next)
		}
	}
	landing := v.seek.value
	// 松手当帧提交，落点就是最终值（没有确认窗口）。
	commit, next := f(PlayerTimelineInput{Held: false})
	if !commit || next != landing {
		t.Fatalf("松手当帧应提交落点 %v：commit=%v next=%v", landing, commit, next)
	}
	for i := 0; i < 5; i++ {
		if commit, _ := f(PlayerTimelineInput{Held: false}); commit {
			t.Fatalf("提交后第 %d 帧又提交了一次 seek", i)
		}
	}
}

// TestVideoChromeSeekTinyTravelCommits 钉住「只要松手就提交」：主窗对一次
// 0.5s 的微调也提交，视频窗不能自己加一个 2 秒阈把它吞掉（那是「拖了没反应」
// 的来源之一）。
func TestVideoChromeSeekTinyTravelCommits(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	f(PlayerTimelineInput{Held: true})
	v.seek.value = 100.5
	f(PlayerTimelineInput{Held: true, Changed: true})
	if commit, next := f(PlayerTimelineInput{Held: false}); !commit || next != 100.5 {
		t.Fatalf("0.5s 的拖动也应提交 100.5：commit=%v next=%v", commit, next)
	}
}

// TestVideoChromeSeekClickFarCommits 钉住「点进度条远处」仍然生效。
func TestVideoChromeSeekClickFarCommits(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	f(PlayerTimelineInput{Held: true})
	v.seek.value = 1800 // 点在中点
	f(PlayerTimelineInput{Held: true, Changed: true})
	if commit, next := f(PlayerTimelineInput{Held: false}); !commit || next != 1800 {
		t.Fatalf("点远处应提交 1800：commit=%v next=%v", commit, next)
	}
}

// TestVideoChromeSeekWaitsForPlayer 钉住「seek 在路上时滑块不能被旧位置拽回」，
// 以及追上之后要跟随播放位置（对应主窗的 SeekBusy + SeekStableAt）。
func TestVideoChromeSeekWaitsForPlayer(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	f(PlayerTimelineInput{Held: true})
	v.seek.value = 900
	f(PlayerTimelineInput{Held: true, Changed: true})
	if commit, next := f(PlayerTimelineInput{Held: false}); !commit || next != 900 {
		t.Fatalf("松手应提交 900：commit=%v next=%v", commit, next)
	}
	// 播放器还没追上（位置仍是 100）：滑块必须停在 900。
	for i := 0; i < 3; i++ {
		if _, next := f(PlayerTimelineInput{Held: false}); next != 900 {
			t.Fatalf("seek 还没落地，第 %d 帧滑块被打回 %v（应为 900）", i, next)
		}
		time.Sleep(10 * time.Millisecond)
	}
	pl.pos = 900 * time.Second
	f(PlayerTimelineInput{Held: false})
	if v.seek.hasTarget {
		t.Error("追上后仍挂着未落地的目标")
	}
	// 交接点之后要过 seekSettle 才恢复跟随（与主窗同一条）。
	time.Sleep(seekSettle + 20*time.Millisecond)
	pl.pos = 905 * time.Second
	if _, next := f(PlayerTimelineInput{Held: false}); next != 905 {
		t.Errorf("追上并过 settle 后应跟随播放位置 905，实际 %v", next)
	}
}

// TestVideoChromeSeekGivesUpEventually 钉住「seek 一直被丢弃时不会永久卡住」。
func TestVideoChromeSeekGivesUpEventually(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	f(PlayerTimelineInput{Held: true})
	v.seek.value = 900
	f(PlayerTimelineInput{Held: true, Changed: true})
	f(PlayerTimelineInput{Held: false})
	// 超时：下一帧 observe 收掉 busy 并记一个交接点（这一帧滑块仍停在
	// 目标值），再过 seekSettle 才恢复跟随 —— 与主窗 seek 完成后的语义一致。
	v.seek.seekAt = time.Now().Add(-videoSeekWait - time.Second)
	if _, next := f(PlayerTimelineInput{Held: false}); next != 900 {
		t.Fatalf("收掉 busy 的那一帧应先停在目标 900，实际 %v", next)
	}
	if v.seek.busy || v.seek.hasTarget {
		t.Fatalf("超时后 busy 没有收掉：busy=%v hasTarget=%v", v.seek.busy, v.seek.hasTarget)
	}
	time.Sleep(seekSettle + 20*time.Millisecond)
	if _, next := f(PlayerTimelineInput{Held: false}); next != 100 {
		t.Errorf("等太久之后应交还控制权（播放位置 100），实际 %v", next)
	}
}

// TestVideoChromeSeekCancelFallsBack 钉住「系统拿走指针」时不提交，且回落到
// 真实播放位置（与主窗 progressPlan 的 cancel 分支一致）。
func TestVideoChromeSeekCancelFallsBack(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	f(PlayerTimelineInput{Held: true})
	v.seek.value = 900
	f(PlayerTimelineInput{Held: true, Changed: true})
	if commit, next := f(PlayerTimelineInput{Cancel: true}); commit || next != 100 {
		t.Fatalf("取消不该提交，且应回落到播放位置 100：commit=%v next=%v", commit, next)
	}
}

// TestVideoChromeSeekKeyboardNudgeHolds 钉住键盘 ← / →：seek 后滑块停在目标，
// 不被旧播放位置拽回（与拖动松手后的等待同一条）。
func TestVideoChromeSeekKeyboardNudgeHolds(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	v.nudge(10) // 播放位置 100 → 110
	if !v.seek.busy || !v.seek.hasTarget || v.seek.value != 110 {
		t.Fatalf("键盘 seek 没挂上等待：%+v", v.seek)
	}
	if _, next := f(PlayerTimelineInput{Held: false}); next != 110 {
		t.Fatalf("键盘 seek 后滑块应停在 110，实际 %v", next)
	}
}
