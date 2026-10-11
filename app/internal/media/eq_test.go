package media

import (
	"math"
	"testing"
)

// runComp 用一段正弦喂压缩器，返回稳态后的峰值。
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

// 实测电平对照（补上补偿增益之后）——这就是「音量均衡」应有的力度：
//
//	输入   0.0 dBFS → 开均衡  0.0 dBFS（提升  0.0 dB）
//	输入  -6.0 dBFS → 开均衡 -0.5 dBFS（提升  5.5 dB）
//	输入 -12.0 dBFS → 开均衡 -1.0 dBFS（提升 11.0 dB）
//	输入 -20.0 dBFS → 开均衡 -1.7 dBFS（提升 18.3 dB）
//	输入 -26.0 dBFS → 开均衡 -2.2 dBFS（提升 23.9 dB）
//	输入 -40.0 dBFS → 开均衡 -4.5 dBFS（提升 35.5 dB）
//
// 各种电平都收敛到 -1 ~ -4 dBFS 附近，且满幅不被压低。

// TestCompressorMakeup 钉住「音量均衡」的核心行为：
// 满幅内容不被压低，安静内容被抬上来（原版靠 Web Audio 压缩器的
// 自动补偿增益做到，少了它就是一开均衡全都变小）。
func TestCompressorMakeup(t *testing.T) {
	fullOn := runComp(t, true, 1.0)
	fullOff := runComp(t, false, 1.0)
	if math.Abs(fullOn-fullOff) > 0.05 {
		t.Errorf("满幅内容开均衡后不应被明显压低：关=%v 开=%v", fullOff, fullOn)
	}

	quietOn := runComp(t, true, 0.05)   // -26 dBFS
	quietOff := runComp(t, false, 0.05) // 原样
	if quietOn <= quietOff*1.5 {
		t.Errorf("安静内容开均衡后应被明显抬上来：关=%v 开=%v", quietOff, quietOn)
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
