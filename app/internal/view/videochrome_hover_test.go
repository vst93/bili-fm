package view

import (
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
func TestVideoOpenButtonHighlights(t *testing.T) {
	a := &App{}
	t.Setenv("HOME", t.TempDir())
	probe := func(videoOpen bool) (uint8, bool) {
		a.VideoOpen = videoOpen
		tt := ui.NewTester(func(c *ui.Context) {
			a.Theme = ThemeFor(PeriodAt(12))
			videoOpenHighlight(c, a, ui.ButtonBase(c.Key("icon-video")).Size(ContentIcon, ContentIcon))
		}, 60, 60)
		tt.Frame()
		return tt.Image().RGBAAt(int(ContentIcon/2), int(ContentIcon/2)).B, a.VideoOpen
	}
	off, _ := probe(false)
	on, _ := probe(true)
	if on == off {
		t.Fatalf("视频窗开/关时按钮看起来一样（B 通道 %d）", on)
	}
}
