package media

import (
	"math"
	"testing"
)

// feed 用 440Hz 正弦喂 w，返回这次拿到的输出样本数与当前源位置。
func feedWSOLA(w *WSOLA, frames int, ch int) (int, int) {
	buf := make([]float32, frames*ch)
	for i := 0; i < frames; i++ {
		v := float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/48000))
		for c := 0; c < ch; c++ {
			buf[i*ch+c] = v
		}
	}
	return len(w.Push(buf)) / ch, w.SourcePosition()
}

// TestWSOLASpeedChangeMidStream 是「改倍速后卡死 / 进度乱跳」的回归测试。
//
// 旧实现用 frame×step 从帧号**重算**标称分析位置，而 step 里含 speed：
// 改一次速度会把过去所有帧也按新速度重算，游标一次性向前跳掉一大截
// （播 1 分钟后切 2x 就跳 1 分钟的源音频），输入跟不上就再也不产出。
// 现在的实现增量推进，改速度只影响之后每帧前进的距离。
func TestWSOLASpeedChangeMidStream(t *testing.T) {
	cases := []struct {
		name string
		to   float64
	}{
		{"加速到 2x", 2.0},
		{"加速到 3x", 3.0},
		{"减速到 0.5x", 0.5},
		{"减速到 0.75x", 0.75},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWSOLA(1, 1.0, 0)
			// 先按原速跑掉一大段（对应"播了一会儿之后再改速"）。
			const pre = 48000 * 30 // 30 秒
			produced, _ := feedWSOLA(w, pre, 1)
			if produced == 0 {
				t.Fatal("原速阶段就没有产出")
			}
			posBefore := w.SourcePosition()

			w.SetSpeed(tc.to)
			producedAfter := 0
			lastPos := posBefore
			// 再喂 5 秒（输入足够，之前卡死时这里会一直产出 0）。
			for i := 0; i < 10; i++ {
				n, pos := feedWSOLA(w, 48000/2, 1)
				producedAfter += n
				if pos < lastPos {
					t.Fatalf("源位置回退了：%d → %d", lastPos, pos)
				}
				// 单次喂入 0.5 秒输入，位置前进不应超过"输入量 + 一帧"。
				if jump := pos - lastPos; jump > 48000/2+1024 {
					t.Fatalf("位置跳变过大：一次喂入 0.5s 输入，位置前进了 %d 个采样", jump)
				}
				lastPos = pos
			}
			if producedAfter == 0 {
				t.Fatal("改速度之后完全没有产出 —— 卡死了")
			}
		})
	}
}

// TestWSOLASpeedRatio 检查稳态输出长度符合 1/speed（变速的核心契约）。
func TestWSOLASpeedRatio(t *testing.T) {
	for _, speed := range []float64{0.5, 1.0, 1.5, 2.0, 3.0} {
		w := NewWSOLA(1, speed, 0)
		const inFrames = 48000 * 10
		produced, _ := feedWSOLA(w, inFrames, 1)
		want := float64(inFrames) / speed
		// 允许一帧多一点的边界误差。
		if math.Abs(float64(produced)-want) > 2048 {
			t.Errorf("speed=%v：喂入 %d 帧产出 %d，期望约 %.0f", speed, inFrames, produced, want)
		}
	}
}

// TestWSOLAStartSample 检查从流中途开始时（seek 重建）游标接得上。
func TestWSOLAStartSample(t *testing.T) {
	const start = 48000 * 7
	w := NewWSOLA(1, 1.0, start)
	if got := w.SourcePosition(); got != start {
		t.Errorf("初始源位置应为 %d，得到 %d", start, got)
	}
	if n, _ := feedWSOLA(w, 48000, 1); n == 0 {
		t.Error("从 startSample 起应该立刻能产出（旧实现要先空转若干帧对齐 skip）")
	}
}

// TestWSOLAStartSampleOnlyShiftsPosition 钉住 WSOLA 的 skip/startSample 语义：
// 它只是 SourcePosition 的坐标系原点，不参与合成 —— 同样输入下两个不同
// startSample 的实例产出**逐采样相同**的音频，只是上报的源位置整体平移。
// 这正是 seek 用 startSample 接进度、而不用它丢音频的依据（见 seekStartSample）。
func TestWSOLAStartSampleOnlyShiftsPosition(t *testing.T) {
	const ch = 1
	in := make([]float32, 48000*ch)
	for i := 0; i < 48000; i++ {
		in[i] = float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/48000))
	}
	a := NewWSOLA(ch, 1.0, 0)
	b := NewWSOLA(ch, 1.0, 48000*7) // 模拟 seek 到 7s

	outA := append([]float32(nil), a.Push(in)...)
	outB := append([]float32(nil), b.Push(in)...)
	if len(outA) == 0 {
		t.Fatal("没有产出")
	}
	if len(outA) != len(outB) {
		t.Fatalf("产出长度不同：%d vs %d", len(outA), len(outB))
	}
	for i := range outA {
		if outA[i] != outB[i] {
			t.Fatalf("第 %d 个采样不同：%v vs %v（startSample 不应改变音频）", i, outA[i], outB[i])
		}
	}
	if d := b.SourcePosition() - a.SourcePosition(); d != 48000*7 {
		t.Errorf("源位置应相差 7s 的 %d，得到 %d", 48000*7, d)
	}
}

// TestWSOLAFlush 检查结束时尾部能吐干净、长度与稳态一致。
func TestWSOLAFlush(t *testing.T) {
	for _, speed := range []float64{0.5, 1.0, 2.0} {
		w := NewWSOLA(1, speed, 0)
		const inFrames = 48000 * 3
		produced, _ := feedWSOLA(w, inFrames, 1)
		produced += len(w.Flush()) / 1
		want := float64(inFrames) / speed
		if d := math.Abs(float64(produced) - want); d > 4096 {
			t.Errorf("speed=%v：含 Flush 共产出 %d，期望约 %.0f（差 %.0f）", speed, produced, want, d)
		}
	}
}

// TestWSOLAManySpeedChanges 连续多次改速度（用户连点档位）也不能卡。
func TestWSOLAManySpeedChanges(t *testing.T) {
	w := NewWSOLA(2, 1.0, 0)
	speeds := []float64{1.25, 1.5, 2.0, 3.0, 0.5, 0.75, 1.0, 2.0}
	lastPos := w.SourcePosition()
	for i, sp := range speeds {
		// 每次改速度前后各喂一点，模拟播放中随手切档。
		if n, _ := feedWSOLA(w, 48000/4, 2); n == 0 && i > 0 {
			t.Fatalf("第 %d 次改速前就没有产出", i)
		}
		w.SetSpeed(sp)
		n, pos := feedWSOLA(w, 48000/4, 2)
		if n == 0 {
			t.Fatalf("改成 %v 之后没有产出", sp)
		}
		if pos < lastPos {
			t.Fatalf("改成 %v 之后源位置回退：%d → %d", sp, lastPos, pos)
		}
		lastPos = pos
	}
}
