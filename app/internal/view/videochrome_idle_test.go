package view

// 视频窗控件条自动隐藏的**手感**用例（§2.2「自动隐藏更跟手」）。
//
// 这一轮把规则从「静止 2.5s 就收」收紧成「静止 2.5s **且** 鼠标不在栏上、
// 不在拖进度、不暂停、设置面板关着才收」，并加了两条：指针一动当场浮出、
// 指针离开窗口当场淡出。这里的六个用例就是用户报的那六条手感，分两层钉：
//
//   - 纯状态机（videoChromeIdle）按 16ms 步进量淡入时长，不依赖渲染；
//     同一时刻的重建不能跳过动画；
//   - 真指针路径（ui.Tester）再走一遍「移动 1px 当场显示」「离开窗口立即
//     隐藏」「按键重置计时」，证明外壳每一帧确实把指针读进来了。

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// ---------------------------------------------------------------- 纯状态机

// videoIdleRun 是按固定步长驱动状态机的脚手架（模拟真机的帧节奏）。
type videoIdleRun struct {
	s      videoChromeIdle
	now    time.Time
	px, py float32
	over   bool
	hold   bool
	step   time.Duration
}

func newVideoIdleRun() *videoIdleRun {
	r := &videoIdleRun{
		now: time.Unix(1700000000, 0), px: 300, py: 200, over: true,
		step: 16 * time.Millisecond,
	}
	r.s.observe(r.now, r.px, r.py, r.over, r.hold) // 首帧：亮着
	return r
}

// run 推进 d（按 step 逐帧），返回最后一帧的结论。d=0 也推进一个刷新周期。
func (r *videoIdleRun) run(d time.Duration) videoChromeFrame {
	end := r.now.Add(d)
	var f videoChromeFrame
	for {
		r.now = r.now.Add(r.step)
		if d > 0 && r.now.After(end) {
			r.now = end
		}
		f = r.s.observe(r.now, r.px, r.py, r.over, r.hold)
		if !r.now.Before(end) {
			return f
		}
	}
}

// TestVideoChromeIdleFeel 是 §2.2 要求的六条手感用例。
func TestVideoChromeIdleFeel(t *testing.T) {
	t.Run("移动1px当场显示", func(t *testing.T) {
		r := newVideoIdleRun()
		r.run(videoChromeIdleDelay + videoChromeHideFade) // 先收起来
		if r.s.alpha != 0 {
			t.Fatalf("先决条件：控件条应已收起，alpha=%v", r.s.alpha)
		}
		r.px++ // 移动 1px
		f := r.run(0)
		if f.Alpha <= 0 || r.s.alpha <= 0 {
			t.Fatalf("移动 1px 没有当场浮出（alpha=%v）", r.s.alpha)
		}
		// 「当场」= 移动后的第一帧就开始淡入（不是先等一拍、也没有移动阈值）。
		if !f.Animating {
			t.Error("移动后的第一帧不在浮出过渡里")
		}
		// 淡入在 80~120ms 内收尾。
		spent := r.step
		for r.s.alpha < 1 && spent <= 200*time.Millisecond {
			r.run(0)
			spent += r.step
		}
		if r.s.alpha != 1 {
			t.Fatalf("%v 还没浮完（alpha=%v）", spent, r.s.alpha)
		}
		if spent < 80*time.Millisecond || spent > 120*time.Millisecond {
			t.Errorf("浮出用了 %v，应在 80~120ms（videoChromeShowFade=%v）", spent, videoChromeShowFade)
		}
	})

	t.Run("静止到点隐藏", func(t *testing.T) {
		r := newVideoIdleRun()
		r.run(videoChromeIdleDelay - 100*time.Millisecond)
		if r.s.alpha != 1 {
			t.Errorf("静止 2.4s 就淡出了（alpha=%v），应为 2.5s 之后", r.s.alpha)
		}
		r.run(100*time.Millisecond + videoChromeHideFade)
		if r.s.alpha != 0 {
			t.Errorf("静止超过 2.5s + 淡出时长后仍可见（alpha=%v）", r.s.alpha)
		}
	})

	t.Run("悬停栏上保持", func(t *testing.T) {
		r := newVideoIdleRun()
		r.hold = true // 指针停在控件条上（barHover）
		r.run(10 * time.Second)
		if r.s.alpha != 1 {
			t.Errorf("指针停在控件条上仍淡出了（alpha=%v）", r.s.alpha)
		}
	})

	t.Run("离开窗口立即隐藏", func(t *testing.T) {
		r := newVideoIdleRun()
		r.run(200 * time.Millisecond)
		r.over = false // 指针离开窗口（PointerLeave → over=false）
		r.run(videoChromeHideFade + r.step)
		if r.s.alpha != 0 {
			t.Errorf("离开窗口 %v 后控件条仍可见（alpha=%v）", videoChromeHideFade, r.s.alpha)
		}
		// 回到窗内：立刻浮出。
		r.over, r.px = true, r.px+20
		if f := r.run(0); f.Alpha <= 0 {
			t.Errorf("回到窗内没有浮出（alpha=%v）", r.s.alpha)
		}
	})

	t.Run("暂停态不隐藏", func(t *testing.T) {
		r := newVideoIdleRun()
		r.hold = true // 暂停（holdChrome 的 !Playing）
		r.run(30 * time.Second)
		if r.s.alpha != 1 {
			t.Errorf("暂停时控件条淡出了（alpha=%v）", r.s.alpha)
		}
	})

	t.Run("面板打开不隐藏", func(t *testing.T) {
		r := newVideoIdleRun()
		r.hold = true // 设置面板 / 音量 / 倍速弹层打开
		r.run(30 * time.Second)
		if r.s.alpha != 1 {
			t.Errorf("面板打开时控件条淡出了（alpha=%v）", r.s.alpha)
		}
	})
}

// TestVideoChromeIdleSampling 钉住兜底采样：收起之后每一帧都在按
// videoChromeSample 周期重看指针（16~33ms），亮着等静置时只按剩余时间排
// 一次唤醒（不把窗口锁在采样循环上）。
func TestVideoChromeIdleSampling(t *testing.T) {
	if videoChromeSample < 16*time.Millisecond || videoChromeSample > 33*time.Millisecond {
		t.Errorf("采样周期 %v 不在要求的 16~33ms", videoChromeSample)
	}
	if videoChromeShowFade < 80*time.Millisecond || videoChromeShowFade > 120*time.Millisecond {
		t.Errorf("浮出时长 %v 不在要求的 80~120ms", videoChromeShowFade)
	}
	r := newVideoIdleRun()
	f := r.run(200 * time.Millisecond)
	if f.Wake <= 0 || f.Wake > videoChromeIdleDelay {
		t.Errorf("亮着等静置时应在 2.5s 内唤醒一次，得到 %v", f.Wake)
	}
	r.run(videoChromeIdleDelay + videoChromeHideFade)
	f = r.run(0)
	if f.Wake != videoChromeSample {
		t.Errorf("收起后应按采样周期兜底（%v），得到 %v", videoChromeSample, f.Wake)
	}
	if f.Animating {
		t.Error("收起后不该还在过渡中")
	}
}

// TestVideoChromeIdleTouch 钉住「键盘也算有操作」：touch 重置静置计时。
func TestVideoChromeIdleTouch(t *testing.T) {
	r := newVideoIdleRun()
	r.run(videoChromeIdleDelay - 50*time.Millisecond)
	r.s.touch(r.now) // 按了一下键
	r.run(200 * time.Millisecond)
	if r.s.alpha != 1 {
		t.Errorf("按键后静置计时没有重置（alpha=%v）", r.s.alpha)
	}
	r.run(videoChromeIdleDelay + videoChromeHideFade)
	if r.s.alpha != 0 {
		t.Errorf("重置后再静止 2.5s 应淡出（alpha=%v）", r.s.alpha)
	}
	// 收起状态下按键：立刻浮出。
	r.s.touch(r.now)
	if f := r.run(0); f.Alpha <= 0 {
		t.Errorf("收起状态下按键没有浮出（alpha=%v）", r.s.alpha)
	}
}

// ---------------------------------------------------------------- 真指针路径

// TestVideoChromeWakeOnMove 钉住外壳真的每帧读了指针：收起之后移动 1px，
// 不需要额外的 Frame() 就当场浮出（0 帧等待）。
func TestVideoChromeWakeOnMove(t *testing.T) {
	ch, _, _, tt := videoChromeProbe(t, 640, 360)
	tt.Move(200, 200) // 先让指针进窗
	tt.Advance(4 * time.Second)
	if ch.idle.alpha != 0 {
		t.Fatalf("先决条件：控件条应收起，alpha=%v", ch.idle.alpha)
	}
	ch.idle.px, ch.idle.py = 200, 200
	tt.Move(201, 200) // 移动 1px，不额外调 Frame
	if ch.idle.alpha <= 0 {
		t.Fatalf("移动 1px 后控件条没有当场浮出（alpha=%v）", ch.idle.alpha)
	}
	tt.Advance(videoChromeShowFade)
	if ch.idle.alpha != 1 {
		t.Errorf("浮出 %v 后 alpha=%v，应为 1", videoChromeShowFade, ch.idle.alpha)
	}
}

// TestVideoChromeLeaveWindow 钉住「指针离开窗口立即淡出」：不等 2.5s 静置，
// 但也仍然守着「暂停 / 面板开着不淡出」那组条件。
func TestVideoChromeLeaveWindow(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 640, 360)
	tt.Move(200, 200)
	// 平台报 PointerLeave / 指针坐标跑到窗外：两种都落到 over=false。
	tt.Move(-40, 200)
	// 220ms 不一定正好落在显示刷新点，允许下一帧完成；不能依赖同时间戳
	// 的额外构建把剩余动画直接跳完。
	tt.Advance(videoChromeHideFade + time.Second/60)
	if ch.idle.alpha != 0 {
		t.Errorf("指针离开窗口 %v 后控件条仍可见（alpha=%v）", videoChromeHideFade, ch.idle.alpha)
	}
	// 回来：立刻浮出。
	tt.Move(200, 200)
	if ch.idle.alpha <= 0 {
		t.Errorf("指针回到窗内没有浮出（alpha=%v）", ch.idle.alpha)
	}
	// 暂停态下离开窗口：不淡出（用户正在看状态，随时可能要操作）。
	pl.Pause()
	tt.Advance(100 * time.Millisecond)
	tt.Move(-40, 200)
	tt.Advance(videoChromeHideFade * 2)
	if ch.idle.alpha != 1 {
		t.Errorf("暂停时离开窗口仍淡出了（alpha=%v）", ch.idle.alpha)
	}
}

// TestVideoChromeLeaveWindowFullScreen 钉住「全屏与窗口模式共用同一套规则」：
// 全屏下指针离开窗口同样立即淡出，回来同样立即浮出。
func TestVideoChromeLeaveWindowFullScreen(t *testing.T) {
	ch, _, win, tt := videoChromeProbe(t, 1280, 720)
	tt.Key(0, ui.KeyF)
	tt.Frame()
	if !win.full {
		t.Fatal("F 没有进入全屏")
	}
	tt.Move(600, 400)
	tt.Move(-40, 400)
	tt.Advance(videoChromeHideFade + videoChromeSample)
	if ch.idle.alpha != 0 {
		t.Errorf("全屏下指针离开窗口没有立即淡出（alpha=%v）", ch.idle.alpha)
	}
	tt.Move(600, 400)
	if ch.idle.alpha <= 0 {
		t.Errorf("全屏下指针回到窗内没有浮出（alpha=%v）", ch.idle.alpha)
	}
}

// TestVideoChromeSeekDragHolds 钉住「拖动进度条时不淡出」：指针被拖出两条栏
// 之外（栏上悬停已经不成立），静止 4s 仍要亮着。
//
// 松手之后这里不断言「2.5s 后淡出」：拖过一次进度条，那条滑块就拿到了键盘
// 焦点，而 holdChrome 把 bar.FocusWithin() 也算「还在跟控件打交道」（读屏 /
// 方向键用户不该因为 2.5s 没动就丢掉焦点环）。这是既有语义，不是这一轮新加的。
func TestVideoChromeSeekDragHolds(t *testing.T) {
	ch, _, _, tt := videoChromeProbe(t, 640, 360)
	pos, ok := tt.Find("播放进度")
	if !ok {
		t.Fatal("找不到进度条")
	}
	// 按住滑块，往窗口上方拖（离开两条栏）。
	tt.Press(pos.X+pos.W/2, pos.Y+pos.H/2)
	tt.Move(320, 150)
	tt.Frame()
	if !ch.seek.wasHeld {
		t.Fatalf("先决条件：应处于拖动中（seek.wasHeld=%v）", ch.seek.wasHeld)
	}
	tt.Advance(4 * time.Second)
	if ch.idle.alpha != 1 {
		t.Errorf("拖动进度条时控件条淡出了（alpha=%v）", ch.idle.alpha)
	}
	tt.Release(320, 150)
	tt.Frame()
	if ch.seek.wasHeld {
		t.Error("松手后应退出拖动状态")
	}
	if !ch.barHover {
		t.Error("指针虽然拖出了栏，但焦点还在栏里，barHover 应仍为真")
	}
}

// TestVideoChromeKeysResetIdle 钉住键盘操作也算「有操作」：收起后按方向键 /
// M / F 都会浮出，并且把静置计时的起点挪到现在。
func TestVideoChromeKeysResetIdle(t *testing.T) {
	for _, key := range []ui.Key{ui.KeyRight, ui.KeyM, ui.KeyUp} {
		ch, _, _, tt := videoChromeProbe(t, 640, 360)
		tt.Advance(4 * time.Second)
		if ch.idle.alpha != 0 {
			t.Fatalf("%v：先决条件不成立，控件条没收起（alpha=%v）", key, ch.idle.alpha)
		}
		tt.Key(0, key)
		// 快捷键可能在本帧的视图构建后处理；等一次显示刷新，不能要求
		// 同一时间戳的重建凭空消耗淡入时间。
		tt.Advance(time.Second / 60)
		if ch.idle.alpha <= 0 {
			t.Fatalf("%v：按键后控件条没有浮出（alpha=%v）", key, ch.idle.alpha)
		}
		// 计时起点是这一次按键：2.4s 后还在，越过 2.5s 才收。
		tt.Advance(videoChromeIdleDelay - 100*time.Millisecond)
		if ch.idle.alpha != 1 {
			t.Errorf("%v：按键后 2.4s 就淡出了（alpha=%v）", key, ch.idle.alpha)
		}
		tt.Advance(100*time.Millisecond + videoChromeHideFade)
		if ch.idle.alpha != 0 {
			t.Errorf("%v：按键后越过 2.5s 仍未淡出（alpha=%v）", key, ch.idle.alpha)
		}
	}
}
