package media

import (
	"math"
	"testing"
)

// runComp 用一段正弦喂均衡器，返回稳态后的峰值。
func runComp(t *testing.T, on bool, amp float64) float64 {
	t.Helper()
	const rate = 48000
	c := NewCompressor(1, rate)
	if on {
		c.SetEnabled(true)
	}
	const total = rate // 1 秒，够过渡与包络都稳住
	buf := make([]float32, total)
	for i := range buf {
		buf[i] = float32(amp * math.Sin(2*math.Pi*440*float64(i)/rate))
	}
	c.Process(buf)
	peak := 0.0
	// 只看后半段（稳态）。
	for _, v := range buf[total/2:] {
		if a := math.Abs(float64(v)); a > peak {
			peak = a
		}
	}
	return peak
}

// TestCompressorMakeup 钉住「音量均衡」的核心行为（RMS 驱动 + 目标响度
// 归一化）：不同输入电平的输出动态范围应比输入窄，且安静内容被抬高。
//
//	输入峰值  1.0 ( 0 dBFS) → 输出 ≈ 0.81（压低，向目标 -5 dBFS 靠拢）
//	输入峰值  0.05(-26 dBFS) → 输出 ≈ 0.57（抬高 +21 dB）
//
//	输入动态范围 20× → 输出动态范围 ≈ 1.8×（均衡有效）
func TestCompressorMakeup(t *testing.T) {
	fullOn := runComp(t, true, 1.0)
	quietOn := runComp(t, true, 0.05)

	// 安静内容被抬高
	quietOff := runComp(t, false, 0.05)
	if quietOn <= quietOff*1.5 {
		t.Errorf("安静内容开均衡后应被明显抬上来：关=%v 开=%v", quietOff, quietOn)
	}

	// 输出动态范围比输入窄（均衡的核心效果）
	inputRatio := 1.0 / 0.05 // 20×
	outputRatio := fullOn / quietOn
	if outputRatio >= inputRatio*0.5 {
		t.Errorf("输出动态范围应比输入窄：输入 %.1f× 输出 %.2f×", inputRatio, outputRatio)
	}

	// 响亮内容不被抬高（压低或持平）
	if quietOn > fullOn*1.2 {
		t.Errorf("安静内容不应超过响亮内容太多：full=%v quiet=%v", fullOn, quietOn)
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

// TestCompressorLimitsPeak 任何输入都不该被抬过头（补偿增益不能造成削波）。
func TestCompressorLimitsPeak(t *testing.T) {
	for _, amp := range []float64{1.0, 0.5, 0.2} {
		if got := runComp(t, true, amp); got > 1.001 {
			t.Errorf("amp=%v 时输出峰值 %v 超过 1.0（会削波）", amp, got)
		}
	}
}
