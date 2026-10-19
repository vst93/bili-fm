package media

import (
	"math"
	"testing"
)

// runComp 用一段正弦喂均衡器，返回**稳态**（后半段）的峰值。
func runComp(t *testing.T, on bool, amp float64) float64 {
	t.Helper()
	const rate = 48000
	c := NewCompressor(1, rate)
	if on {
		c.SetEnabled(true)
	}
	const total = rate // 1 秒：过渡 + 检测器都稳住
	buf := make([]float32, total)
	for i := range buf {
		buf[i] = float32(amp * math.Sin(2*math.Pi*440*float64(i)/rate))
	}
	c.Process(buf)
	peak := 0.0
	for _, v := range buf[total/2:] {
		if a := math.Abs(float64(v)); a > peak {
			peak = a
		}
	}
	return peak
}

// TestCompressorMatchesWebAudio 钉住「与原版（Web Audio DynamicsCompressorNode，
// threshold -50 / knee 40 / ratio 12）逐值一致」。
//
// 期望值由 Blink 的静态曲线算出（eq.go 的 saturate + 补偿增益）：
//
//	0 dBFS → 静态曲线 -20.75 dB × 补偿 +12.40 dB ≈ -8.3 dB (0.385)
//
// 注意它**不是**「0 dBFS 进、0 dBFS 出」：Web Audio 这个节点在 -50 dB 阈值、
// 12:1 下本来就压得很狠，只补回一部分。原版 Tauri 用的正是这组参数，
// 用户要的就是这个听感（响的被压、轻的被抬）。
func TestCompressorMatchesWebAudio(t *testing.T) {
	cases := []struct {
		amp    float64
		wantDB float64 // 相对输入的增益
	}{
		{1.0, -8.30},
		{0.1, 8.11},
		{0.01, 12.22},
	}
	for _, tc := range cases {
		on := runComp(t, true, tc.amp)
		gotDB := 20 * math.Log10(on/tc.amp)
		if math.Abs(gotDB-tc.wantDB) > 0.35 {
			t.Errorf("amp=%v 增益 %.2f dB，应为 %.2f dB（Blink 静态曲线）", tc.amp, gotDB, tc.wantDB)
		}
	}
}

// TestCompressorCompressesRange 核心效果：把 40dB 的输入动态范围压到一半左右，
// 且轻的内容被抬上来（原版「音量均衡」的听感）。
func TestCompressorCompressesRange(t *testing.T) {
	loudOn := runComp(t, true, 1.0)
	loudOff := runComp(t, false, 1.0)
	quietOn := runComp(t, true, 0.01)
	quietOff := runComp(t, false, 0.01)

	if loudOn >= loudOff {
		t.Errorf("响的内容应被压低：关=%v 开=%v", loudOff, loudOn)
	}
	if quietOn <= quietOff*2 {
		t.Errorf("轻的内容应被明显抬高：关=%v 开=%v", quietOff, quietOn)
	}
	inDB := 20 * math.Log10(1.0/0.01)            // 40 dB
	outDB := 20 * math.Log10(loudOn/quietOn)     // 压缩后的动态范围
	if outDB >= inDB*0.6 {
		t.Errorf("输出动态范围 %.1f dB，应明显小于输入的 %.1f dB", outDB, inDB)
	}
}

// TestCompressorPassthrough 关闭时必须完全透明（一个采样都不改）。
func TestCompressorPassthrough(t *testing.T) {
	c := NewCompressor(2, 48000)
	buf := []float32{0.3, -0.3, 0.7, -0.7, 0.1, -0.1}
	orig := append([]float32(nil), buf...)
	c.Process(buf)
	for i := range buf {
		if buf[i] != orig[i] {
			t.Fatalf("关闭时不该改动采样：%v != %v", buf, orig)
		}
	}
}

// TestCompressorSteadyStateNoClip 稳态不该削波（过冲/起音瞬态由 Blink 的
// attack 负责，这里只钉住「稳住之后」）。
func TestCompressorSteadyStateNoClip(t *testing.T) {
	for _, amp := range []float64{1.0, 0.5, 0.2} {
		if got := runComp(t, true, amp); got > 1.001 {
			t.Errorf("amp=%v 稳态输出峰值 %v 超过 1.0（会削波）", amp, got)
		}
	}
}

// TestCompressorStereoKeepsChannels 立体声：两个声道都要处理（而不是只改左声道），
// 且左右相同的信号保持相同。
func TestCompressorStereoKeepsChannels(t *testing.T) {
	const rate = 48000
	c := NewCompressor(2, rate)
	c.SetEnabled(true)
	buf := make([]float32, 2*rate)
	for i := 0; i < rate; i++ {
		v := float32(0.5 * math.Sin(2*math.Pi*220*float64(i)/rate))
		buf[2*i], buf[2*i+1] = v, v
	}
	c.Process(buf)
	for i := len(buf) / 2; i < len(buf); i += 2 {
		if buf[i] != buf[i+1] {
			t.Fatalf("左右声道处理结果不一致：%v != %v", buf[i], buf[i+1])
		}
		if math.Abs(float64(buf[i])) > 1.001 {
			t.Fatalf("立体声输出削波：%v", buf[i])
		}
	}
}
