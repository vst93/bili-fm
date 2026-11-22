package view

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestVideoChromeHoverKeepsFramesComing 钉住「悬停期间帧要一直来」。
//
// 悬停 / 按压的材质色是构建期算的（Hovered + AnimateColor 的 140ms 过渡），
// 而视频窗的帧大多由视频驱动、只重绘不重建：不主动排帧，过渡就只在偶发的
// 重建帧里跳一格，画面在新旧两色之间闪（用户报的「悬停闪烁」）。
//
// 用 After 排「过渡走完」那一拍，而不是无限 AnimationFrame —— 后者会让
// Tester.Frame() 永远有待处理帧（整包测试卡死过）。
func TestVideoChromeHoverKeepsFramesComing(t *testing.T) {
	ch, _, _, tt := videoChromeProbe(t, 640, 360)
	r, ok := tt.Find("视频设置")
	if !ok {
		t.Fatalf("找不到设置键")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if ch.hoverAt.IsZero() {
		t.Fatal("指针停在按钮上，没有记下悬停时刻（过渡不会被推进）")
	}
	// 指针离开控件条：尾巴（140ms）内仍要有唤醒排队。
	tt.Move(320, 200)
	tt.Frame()
	if !tt.FramePending() {
		t.Error("指针刚离开，过渡尾巴内没有排下一次唤醒（过渡会停在半路）")
	}
	// 尾巴走完后不再空转。
	tt.Advance(videoHoverTail + 100*time.Millisecond)
	tt.Frame()
	if tt.FramePending() {
		t.Error("悬停尾巴已过，仍在排队帧（白烧 CPU）")
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
// 松手（并确认过）才生效」—— 与主窗 progressPlan 同一条语义。
//
// 长视频上一次 seek 要几十毫秒（AVPlayer 从关键帧解码），拖动中逐位移下发
// 会把绘制线程堵死；而 press 抖一帧就提交会让视频在用户没松手时跳走。
func TestVideoChromeSeekCommitsOnReleaseOnly(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)

	if commit, _ := f(PlayerTimelineInput{Held: true}); commit {
		t.Fatal("按下时不该提交 seek")
	}
	for i := 0; i < 60; i++ {
		v.seek.value = 100 + float64(i)*20
		if commit, next := f(PlayerTimelineInput{Held: true, Changed: true}); commit {
			t.Fatalf("第 %d 次位移就提交了 seek（拖动中不该下发）", i)
		} else if next != v.seek.value {
			t.Fatalf("第 %d 次位移滑块值不是用户拖到的 %v（得到 %v）", i, v.seek.value, next)
		}
	}
	landing := v.seek.value
	// 松手那一帧：进入确认窗口，还不提交。
	if commit, _ := f(PlayerTimelineInput{Held: false}); commit {
		t.Fatal("松手当帧就提交了（press 抖一帧会误判成松手）")
	}
	// 确认窗口过后：提交一次，落点是最终值。
	time.Sleep(videoSeekConfirm + 30*time.Millisecond)
	commit, next := f(PlayerTimelineInput{Held: false})
	if !commit || next != landing {
		t.Fatalf("确认松手后应提交落点 %v：commit=%v next=%v", landing, commit, next)
	}
	for i := 0; i < 5; i++ {
		if commit, _ := f(PlayerTimelineInput{Held: false}); commit {
			t.Fatalf("提交后第 %d 帧又提交了一次 seek", i)
		}
	}
}

// TestVideoChromeSeekPressFlickerDoesNotJump 钉住用户实测的 bug：press 抖
// 一帧（用户没松手）时视频当场跳走。抖动要在确认窗口内被作废。
func TestVideoChromeSeekPressFlickerDoesNotJump(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	f(PlayerTimelineInput{Held: true})
	v.seek.value = 900
	f(PlayerTimelineInput{Held: true, Changed: true})
	// press 掉一帧（用户没松手）。
	if commit, _ := f(PlayerTimelineInput{Held: false}); commit {
		t.Fatal("press 掉帧当帧就提交了")
	}
	// 立刻又按回来：那次松手必须作废。
	f(PlayerTimelineInput{Held: true})
	for i := 0; i < 6; i++ {
		time.Sleep(videoSeekConfirm / 3)
		if commit, next := f(PlayerTimelineInput{Held: true}); commit {
			t.Fatalf("又按回来之后仍提交了 seek（第 %d 帧）", i)
		} else if next != 900 {
			t.Fatalf("又按回来之后滑块值被打回 %v（应为 900）", next)
		}
	}
}

// TestVideoChromeSeekTinyTravelIgnored 钉住「没真的拖动就不提交」：点一下
// 进度条、或抖动后值被回填到播放位置附近时，提交一次 seek 就是一次没头没脑
// 的跳转。
func TestVideoChromeSeekTinyTravelIgnored(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	f(PlayerTimelineInput{Held: true})
	v.seek.value = 100.5 // 与播放位置差 0.5s，远小于 videoSeekMinTravel
	f(PlayerTimelineInput{Held: true, Changed: true})
	f(PlayerTimelineInput{Held: false})
	time.Sleep(videoSeekConfirm + 30*time.Millisecond)
	if commit, _ := f(PlayerTimelineInput{Held: false}); commit {
		t.Error("只挪了 0.5s 也提交了 seek（应视为没拖动）")
	}
}

// TestVideoChromeSeekClickFarStillCommits 钉住「点进度条远处」仍然生效。
func TestVideoChromeSeekClickFarStillCommits(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	f(PlayerTimelineInput{Held: true})
	v.seek.value = 1800 // 点在中点
	f(PlayerTimelineInput{Held: true, Changed: true})
	f(PlayerTimelineInput{Held: false})
	time.Sleep(videoSeekConfirm + 30*time.Millisecond)
	if commit, next := f(PlayerTimelineInput{Held: false}); !commit || next != 1800 {
		t.Fatalf("点远处应提交 1800：commit=%v next=%v", commit, next)
	}
}

// TestVideoChromeSeekWaitsForPlayer 钉住「seek 在路上时滑块不能被旧位置拽回」。
func TestVideoChromeSeekWaitsForPlayer(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 3600 * time.Second, pos: 100 * time.Second, rate: 1, vol: 1}
	v := &VideoChrome{Pl: pl}
	f := v.seekFrame(3600)
	f(PlayerTimelineInput{Held: true})
	v.seek.value = 900
	f(PlayerTimelineInput{Held: true, Changed: true})
	f(PlayerTimelineInput{Held: false})
	time.Sleep(videoSeekConfirm + 30*time.Millisecond)
	if commit, next := f(PlayerTimelineInput{Held: false}); !commit || next != 900 {
		t.Fatalf("确认松手应提交 900：commit=%v next=%v", commit, next)
	}
	// 播放器还没追上（位置仍是 100）：滑块必须停在 900。
	for i := 0; i < 3; i++ {
		if _, next := f(PlayerTimelineInput{Held: false}); next != 900 {
			t.Fatalf("seek 还没落地，第 %d 帧滑块被打回 %v（应为 900）", i, next)
		}
		time.Sleep(10 * time.Millisecond)
	}
	pl.pos = 900 * time.Second
	if _, next := f(PlayerTimelineInput{Held: false}); next != 900 {
		t.Fatalf("追上后应显示 900，实际 %v", next)
	}
	if v.seek.hasTarget {
		t.Error("追上后仍挂着未落地的目标")
	}
	pl.pos = 905 * time.Second
	if _, next := f(PlayerTimelineInput{Held: false}); next != 905 {
		t.Errorf("追上后应跟随播放位置 905，实际 %v", next)
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
	time.Sleep(videoSeekConfirm + 30*time.Millisecond)
	f(PlayerTimelineInput{Held: false})
	v.seek.releasedAt = time.Now().Add(-videoSeekWait - time.Second)
	if _, next := f(PlayerTimelineInput{Held: false}); next != 100 {
		t.Errorf("等太久之后应交还控制权（播放位置 100），实际 %v", next)
	}
}

// TestVideoChromeSeekCancelFallsBack 钉住「系统拿走指针」时不提交。
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
