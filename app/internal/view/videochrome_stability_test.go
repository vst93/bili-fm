package view

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

// A frame can rebuild several times with the same timestamp (input/layout).
// Those passes must not consume the rest of a fade.
func TestVideoChromeFadeSameTimestamp(t *testing.T) {
	r := newVideoIdleRun()
	r.run(videoChromeIdleDelay + 40*time.Millisecond)
	if r.s.alpha <= 0 || r.s.alpha >= 1 {
		t.Fatalf("expected an unfinished fade, alpha=%v", r.s.alpha)
	}
	alpha := r.s.alpha
	for range 5 {
		f := r.s.observe(r.now, r.px, r.py, r.over, r.hold)
		if f.Alpha != alpha || !f.Animating {
			t.Fatalf("same-time rebuild consumed fade: %v -> %+v", alpha, f)
		}
	}
	// Reverse while the pointer is over the bar, still in the same frame.
	r.hold = true
	if f := r.s.observe(r.now, r.px, r.py, r.over, r.hold); f.Alpha != alpha {
		t.Fatalf("same-time reversal jumped: %v -> %v", alpha, f.Alpha)
	}
	r.run(videoChromeShowFade)
	if r.s.alpha != 1 {
		t.Fatalf("fade did not finish after time advanced: %v", r.s.alpha)
	}
}

func TestVideoChromeFadeAcrossRefreshRates(t *testing.T) {
	for _, hz := range []int{30, 60, 120, 144} {
		t.Run(fmt.Sprint(hz), func(t *testing.T) {
			r := newVideoIdleRun()
			r.step = time.Second / time.Duration(hz)
			r.over = false
			for elapsed := time.Duration(0); elapsed < videoChromeHideFade+r.step; elapsed += r.step {
				f := r.run(0)
				for range 3 {
					if next := r.s.observe(r.now, r.px, r.py, r.over, false); next.Alpha != f.Alpha {
						t.Fatalf("%d Hz: rebuild changed alpha %v -> %v", hz, f.Alpha, next.Alpha)
					}
				}
			}
			if r.s.alpha != 0 {
				t.Fatalf("fade did not finish at %d Hz: %v", hz, r.s.alpha)
			}
		})
	}
}

func TestVideoChromeDragSurvivesDanmakuChanges(t *testing.T) {
	ch, pl, overlay, tt := danmakuChrome(t, 10, true, nil, danmaku.DefaultSettings())
	ch.settings().SetSpeed(3)
	track, ok := tt.Find("播放进度")
	if !ok {
		t.Fatal("missing timeline")
	}
	x0, span, y := track.X+timelineInset, track.W-2*timelineInset, track.Y+track.H/2
	tt.Press(x0+span*0.3, y)
	if !ch.seek.wasHeld {
		t.Fatal("timeline was not captured")
	}
	// Change the visible sibling count while the physical pointer stays down.
	for i, items := range [][]danmaku.Item{
		{{Time: 9, Text: "一条弹幕"}},
		{{Time: 9, Text: "一条弹幕"}, {Time: 9.5, Text: "另一条弹幕"}},
		nil,
	} {
		overlay.SetItems(items)
		tt.Frame()
		if !ch.seek.wasHeld || len(pl.seeks) != 0 {
			t.Fatalf("comment change %d lost pointer capture: held=%v seeks=%v", i, ch.seek.wasHeld, pl.seeks)
		}
		if ch.DanmakuDrawn() != len(items) {
			t.Fatalf("fixture: expected %d visible comments, got %d", len(items), ch.DanmakuDrawn())
		}
		tt.Move(x0+span*float32(0.4+float64(i)*0.1), y)
		if !ch.seek.wasHeld || len(pl.seeks) != 0 {
			t.Fatalf("comment change %d lost pointer capture: held=%v seeks=%v", i, ch.seek.wasHeld, pl.seeks)
		}
	}
	landing := ch.seek.value
	tt.Release(x0+span*0.6, y)
	if len(pl.seeks) != 1 || math.Abs(pl.seeks[0].Seconds()-landing) > 0.1 {
		t.Fatalf("release should seek once to %.2f, got %v", landing, pl.seeks)
	}
}

func TestVideoChromeButtonSurvivesDanmakuChanges(t *testing.T) {
	_, pl, overlay, tt := danmakuChrome(t, 10, true, nil, danmaku.DefaultSettings())
	button, ok := tt.Find("暂停")
	if !ok {
		t.Fatal("missing pause button")
	}
	x, y := button.X+button.W/2, button.Y+button.H/2
	tt.Press(x, y)
	overlay.SetItems([]danmaku.Item{{Time: 9, Text: "按住按钮时出现"}})
	tt.Frame()
	tt.Release(x, y)
	if pl.pauses != 1 {
		t.Fatalf("comment arrival swallowed click: pauses=%d", pl.pauses)
	}
}

func TestVideoChromeHiddenSkipsControlsAndRestoresProgress(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 640, 360)
	contentFrames := 0
	ch.Content = func(c *ui.Context) ui.Element {
		contentFrames++
		return ui.Box(c).Fill()
	}
	tt.Advance(4 * time.Second)
	if ch.idle.alpha != 0 {
		t.Fatal("controls should be hidden")
	}
	for _, label := range []string{"播放进度", "暂停", "视频设置"} {
		if _, ok := tt.Find(label); ok {
			t.Errorf("hidden control still built: %s", label)
		}
	}
	before := contentFrames
	pl.pos = 65 * time.Second
	tt.Advance(videoChromeSample)
	if contentFrames <= before {
		t.Fatal("hiding controls stopped media content")
	}
	tt.Move(300, 180)
	tt.Advance(videoChromeShowFade + time.Second/60)
	if _, ok := tt.Find("播放进度"); !ok || ch.seek.value != 65 {
		t.Fatalf("restored timeline did not follow playback: present=%v value=%v", ok, ch.seek.value)
	}
	if err := tt.Click("暂停"); err != nil || pl.pauses != 1 {
		t.Fatalf("restored controls are not interactive: error=%v pauses=%d", err, pl.pauses)
	}
}

func BenchmarkVideoChromeFrame(b *testing.B) {
	for _, hidden := range []bool{false, true} {
		name := "visible"
		if hidden {
			name = "hidden"
		}
		b.Run(name, func(b *testing.B) {
			pl := &videoStubPlayer{playing: true, ready: true, dur: time.Hour, pos: time.Minute, rate: 3}
			ch := &VideoChrome{Pl: pl, Theme: ThemeFor(PeriodAt(12))}
			tt := ui.NewTester(ch.View, 960, 600)
			ch.settings().SetSpeed(3)
			tt.Frame()
			if hidden {
				tt.Advance(4 * time.Second)
			}
			b.Cleanup(ch.Detach)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				tt.Frame()
			}
		})
	}
}
