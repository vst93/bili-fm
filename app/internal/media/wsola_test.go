package media

import (
	"math"
	"testing"
	"time"
)

// dominantFreq 数上升沿过零。对纯正弦这是精确的，而且比频率扫描快几个
// 数量级（扫描式相关跑一次要几十秒，还会因为泄漏产生 ~2% 的系统偏差）。
func dominantFreq(sig []float32, rate int, lo, hi float64) float64 {
	// 取中段，避开首尾叠加未定型的部分。
	start, end := len(sig)/4, len(sig)*3/4
	if end-start < 4096 {
		start, end = 0, len(sig)
	}
	x := sig[start:end]

	first, last, cycles := -1, -1, 0
	for i := 1; i < len(x); i++ {
		if x[i-1] <= 0 && x[i] > 0 {
			if first < 0 {
				first = i
			} else {
				last = i
				cycles++
			}
		}
	}
	if cycles == 0 || last <= first {
		return 0
	}
	return float64(cycles) * float64(rate) / float64(last-first)
}

func sine(rate int, freq, seconds float64) []float32 {
	n := int(float64(rate) * seconds)
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(0.5 * math.Sin(2*math.Pi*freq*float64(i)/float64(rate)))
	}
	return out
}

// TestPitchPreserved 是 WSOLA 最核心的性质：变速之后音高不变。
// 朴素重采样（花栗鼠音）会让 440Hz 变成 880Hz，这里必须仍然是 440Hz。
func TestPitchPreserved(t *testing.T) {
	const (
		rate = 44100
		freq = 440.0
	)
	in := sine(rate, freq, 2.0)

	for _, speed := range []float64{0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 3.0} {
		w := NewWSOLA(1, speed)
		out := append(w.Push(in), w.Flush()...)

		got := dominantFreq(out, rate, 200, 1200)
		if math.Abs(got-freq) > 3 {
			t.Errorf("speed=%.2f: 主频 %.2f Hz，期望 %.2f Hz（音高变了）", speed, got, freq)
		}

		// 输出长度应当约等于输入 / speed。
		want := float64(len(in)) / speed
		gotLen := float64(len(out))
		if math.Abs(gotLen-want)/want > 0.08 {
			t.Errorf("speed=%.2f: 输出 %d 采样，期望约 %.0f（误差 %.1f%%）",
				speed, len(out), want, 100*math.Abs(gotLen-want)/want)
		}
	}
}

// TestStereoIndependent 确认立体声按声道独立处理（左右不串）。
func TestStereoIndependent(t *testing.T) {
	const rate = 44100
	// 左 440Hz，右 880Hz，交错。
	n := rate
	in := make([]float32, n*2)
	for i := 0; i < n; i++ {
		in[2*i] = float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/rate))
		in[2*i+1] = float32(0.5 * math.Sin(2*math.Pi*880*float64(i)/rate))
	}
	w := NewWSOLA(2, 1.5)
	out := append(w.Push(in), w.Flush()...)

	left := make([]float32, 0, len(out)/2)
	right := make([]float32, 0, len(out)/2)
	for i := 0; i+1 < len(out); i += 2 {
		left = append(left, out[i])
		right = append(right, out[i+1])
	}
	if f := dominantFreq(left, rate, 200, 1500); math.Abs(f-440) > 3 {
		t.Errorf("左声道主频 %.2f Hz，期望 440", f)
	}
	if f := dominantFreq(right, rate, 200, 1500); math.Abs(f-880) > 5 {
		t.Errorf("右声道主频 %.2f Hz，期望 880", f)
	}
}

// TestThroughput 量一下开销。实时播放要求远高于 1x：3 倍速播放时，
// 每 1 秒墙钟要产出 1 秒输出（输出长度已是输入的 1/3）。
func TestThroughput(t *testing.T) {
	const rate = 44100
	in := sine(rate, 440, 10.0) // 10 秒单声道

	for _, speed := range []float64{1.0, 2.0, 3.0} {
		w := NewWSOLA(2, speed)
		// 转成立体声输入
		st := make([]float32, len(in)*2)
		for i, v := range in {
			st[2*i], st[2*i+1] = v, v
		}
		t0 := time.Now()
		out := append(w.Push(st), w.Flush()...)
		el := time.Since(t0)
		outSeconds := float64(len(out)/2) / rate
		t.Logf("speed=%.1f: 输入 10.00s → 输出 %.2fs，耗时 %v，实时倍率 %.1fx",
			speed, outSeconds, el, outSeconds/el.Seconds())
		if outSeconds/el.Seconds() < 5 {
			t.Errorf("speed=%.1f: 只有 %.1fx 实时，太慢", speed, outSeconds/el.Seconds())
		}
	}
}
