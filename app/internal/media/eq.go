package media

import "math"

// Compressor 是前馈动态范围压缩器，对应旧版音量均衡用的 Web Audio
// DynamicsCompressorNode（player.tsx 的 getOrCreateAudioGraph）。
//
// 旧版参数：
//
//	关闭时 threshold 0 / knee 0 / ratio 1   —— 透明直通，但图保持稳定
//	开启时 threshold -50 / knee 40 / ratio 12 / attack 0 / release 0.25
//	切换时对 threshold/knee/ratio 做线性渐变（EQ_TRANSITION_SECONDS）
//
// 不同 B 站视频的默认音量差异很大，开启后安静的和响亮的视频能落在更接近
// 的响度上。
type Compressor struct {
	channels int
	rate     int

	// 当前值（会朝目标线性过渡）
	threshold, knee, ratio float64
	// 目标值
	tThreshold, tKnee, tRatio float64
	// 过渡进度：0 表示还需 transitionSamples 个采样走完
	transLeft  int
	transTotal int

	// 每个声道的包络（dB 域的增益衰减）
	envDB []float64
}

const eqTransitionSeconds = 0.12

// NewCompressor 建一个压缩器，初始为透明直通。
func NewCompressor(channels, rate int) *Compressor {
	return &Compressor{
		channels:   channels,
		rate:       rate,
		threshold:  0,
		knee:       0,
		ratio:      1,
		tThreshold: 0,
		tKnee:      0,
		tRatio:     1,
		envDB:      make([]float64, channels),
	}
}

// SetEnabled 打开或关闭均衡，参数按 eqTransitionSeconds 线性过渡，
// 避免切换时出现咔哒声。
func (c *Compressor) SetEnabled(on bool) {
	if on {
		c.tThreshold, c.tKnee, c.tRatio = -50, 40, 12
	} else {
		c.tThreshold, c.tKnee, c.tRatio = 0, 0, 1
	}
	c.transTotal = int(eqTransitionSeconds * float64(c.rate))
	c.transLeft = c.transTotal
}

// Enabled 报告当前是否朝「开启」过渡。
func (c *Compressor) Enabled() bool { return c.tRatio > 1 }

// Process 原地处理一段交错 float32 PCM。
func (c *Compressor) Process(buf []float32) {
	if len(buf) == 0 {
		return
	}
	ch := c.channels
	frames := len(buf) / ch

	// 过渡步长：把三个参数在本段内朝目标推进。
	step := 1.0
	if c.transLeft > 0 {
		if frames < c.transLeft {
			step = float64(frames) / float64(c.transTotal)
		} else {
			step = float64(c.transLeft) / float64(c.transTotal)
		}
	}

	for i := 0; i < frames; i++ {
		// 推进参数过渡
		if c.transLeft > 0 {
			c.threshold += (c.tThreshold - c.threshold) * math.Min(1, step)
			c.knee += (c.tKnee - c.knee) * math.Min(1, step)
			c.ratio += (c.tRatio - c.ratio) * math.Min(1, step)
			c.transLeft--
			if c.transLeft <= 0 {
				c.threshold, c.knee, c.ratio = c.tThreshold, c.tKnee, c.tRatio
			}
		}

		// 透明直通时完全不做运算，省 CPU。
		if c.ratio <= 1.0001 && c.threshold >= -0.001 {
			continue
		}

		base := i * ch
		// 立体声用两声道的最大值驱动，保持声像稳定。
		peak := 0.0
		for ch2 := 0; ch2 < ch; ch2++ {
			if v := math.Abs(float64(buf[base+ch2])); v > peak {
				peak = v
			}
		}
		xDB := -120.0
		if peak > 1e-7 {
			xDB = 20 * math.Log10(peak)
		}
		gainDB := c.gainReduction(xDB)
		// 释放平滑：旧版 release 0.25s，用单极点逼近。
		coef := math.Exp(-1.0 / (0.25 * float64(c.rate)))
		for ch2 := 0; ch2 < ch; ch2++ {
			c.envDB[ch2] = gainDB + (c.envDB[ch2]-gainDB)*coef
			g := math.Pow(10, c.envDB[ch2]/20)
			buf[base+ch2] = float32(float64(buf[base+ch2]) * g)
		}
	}
}

// gainReduction 返回该电平应施加的增益（dB，<=0），带软拐点。
func (c *Compressor) gainReduction(xDB float64) float64 {
	th, knee, ratio := c.threshold, c.knee, c.ratio
	lo := th - knee/2
	hi := th + knee/2

	var yDB float64
	switch {
	case xDB < lo:
		yDB = xDB
	case xDB > hi:
		yDB = th + (xDB-th)/ratio
	default:
		// 二次插值，与 Web Audio 的软拐点一致。
		t := xDB - lo
		yDB = xDB + (1/ratio-1)*t*t/(2*knee)
	}
	return yDB - xDB
}
