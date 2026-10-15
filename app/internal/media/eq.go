package media

import (
	"math"
	"sync"
)

// Compressor 是响度均衡器：用 RMS 滑动窗口检测信号响度，按软拐点
// 压缩曲线计算增益，让安静和响亮的内容落在更接近的响度上。
//
// 与旧版（纯峰值驱动 + 固定 makeup +45.8 dB）的区别：
//
//	旧版：阈值 -50 dBFS + 12:1 压缩 → makeup 让 0 dBFS→0 dBFS，
//	      安静内容被固定抬 +45.8 dB，响亮内容几乎不变——「感觉基本就是
//	      把小音量调大」。
//	新版：RMS 检测 + 目标响度归一化 — makeup 动态计算，让目标 RMS
//	      (-5 dBFS) 的输入原样输出，安静/响亮内容都向目标靠拢。
//
// 参数（与原版 Web Audio 对齐，threshold -40 / target -5 让
// 正常响度内容原样输出，安静/响亮内容都向目标靠拢）：
//
//	threshold -40 / knee 40 / ratio 12 / attack 0 / release 0.25s
//
// 不同 B 站视频的默认音量差异很大，开启后安静的和响亮的视频能落在
// 更接近的响度上。
type Compressor struct {
	// mu 保护下面这些控制字段：SetEnabled 在界面线程调，Process 在 oto
	// 取数据 goroutine 上调。
	mu       sync.Mutex
	channels int
	rate     int

	// 当前值（会朝目标线性过渡）
	threshold, knee, ratio float64
	// 目标值
	tThreshold, tKnee, tRatio float64
	// 过渡进度：0 表示还需 transitionSamples 个采样走完
	transLeft  int
	transTotal int

	// 每个声道的包络（dB 域的增益）
	envDB []float64
	// releaseCoef 是释放的单极点系数（0.25s）；起音是瞬时的，没有系数。
	releaseCoef float64

	// RMS 检测器：每声道维护 RMS² 的指数滑动平均
	rmsState []float64
	rmsAlpha float64

	// 目标响度（RMS，dBFS）——makeup 动态计算，让该电平的输入原样输出
	targetRMSDB float64
}

const (
	eqTransitionSeconds = 0.06
	eqRMSWindowSeconds = 0.2
	eqTargetRMSDB      = -5.0
)

// NewCompressor 建一个均衡器，初始为透明直通。
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
		// 释放 250ms（原版 release: 0.25；起音是瞬时的，见 Process）。
		releaseCoef: math.Exp(-1.0 / (0.25 * float64(rate))),
		// RMS 滑动窗口 200ms：足够平滑短瞬态，又能在音量变化时及时响应。
		rmsState:    make([]float64, channels),
		rmsAlpha:    1.0 - math.Exp(-1.0/(eqRMSWindowSeconds*float64(rate))),
		targetRMSDB: eqTargetRMSDB,
	}
}

// SetEnabled 打开或关闭均衡，参数按 eqTransitionSeconds 线性过渡，
// 避免切换时出现咔哒声。
func (c *Compressor) SetEnabled(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if on {
		c.tThreshold, c.tKnee, c.tRatio = -40, 40, 12
	} else {
		c.tThreshold, c.tKnee, c.tRatio = 0, 0, 1
	}
	c.transTotal = int(eqTransitionSeconds * float64(c.rate))
	c.transLeft = c.transTotal
}

// Enabled 报告当前是否朝「开启」过渡。
func (c *Compressor) Enabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tRatio > 1
}

// Process 原地处理一段交错 float32 PCM。
func (c *Compressor) Process(buf []float32) {
	if len(buf) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
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
		// RMS 检测：维护每声道的 RMS² 指数滑动平均。
		sumSq := 0.0
		for ch2 := 0; ch2 < ch; ch2++ {
			v := float64(buf[base+ch2])
			c.rmsState[ch2] = c.rmsState[ch2]*(1-c.rmsAlpha) + v*v*c.rmsAlpha
			sumSq += c.rmsState[ch2]
		}
		rms := math.Sqrt(sumSq / float64(ch))
		xDB := -120.0
		if rms > 1e-7 {
			xDB = 20 * math.Log10(rms)
		}

		gainDB := c.gainReduction(xDB) + c.makeupDB()

		// 包络：起音**瞬时**（原版 attack: 0）、释放 0.25s。
		// 起音必须瞬时，不能平滑：平滑会让包络在信号峰值处还停在"较低电平
		// 对应的高增益"上，于是满幅内容被抬到 1.0 以上。
		if gainDB < c.envDB[0] {
			for ch2 := 0; ch2 < ch; ch2++ {
				c.envDB[ch2] = gainDB
			}
		} else {
			for ch2 := 0; ch2 < ch; ch2++ {
				c.envDB[ch2] = gainDB + (c.envDB[ch2]-gainDB)*c.releaseCoef
			}
		}

		// 峰值限制器：防止 RMS 驱动的增益把瞬态峰值推过 1.0 造成削波。
		g := math.Pow(10, c.envDB[0]/20)
		peak := 0.0
		for ch2 := 0; ch2 < ch; ch2++ {
			if v := math.Abs(float64(buf[base+ch2])); v > peak {
				peak = v
			}
		}
		if peak > 0 && peak*g > 1.0 {
			g = 1.0 / peak
		}

		for ch2 := 0; ch2 < ch; ch2++ {
			buf[base+ch2] = float32(float64(buf[base+ch2]) * g)
		}
	}
}

// curveDB 是按当前参数压缩后的输出电平（dB），带软拐点。
func (c *Compressor) curveDB(xDB float64) float64 {
	th, knee, ratio := c.threshold, c.knee, c.ratio
	if knee <= 0 {
		if xDB <= th {
			return xDB
		}
		return th + (xDB-th)/ratio
	}
	lo := th - knee/2
	hi := th + knee/2
	switch {
	case xDB < lo:
		return xDB
	case xDB > hi:
		return th + (xDB-th)/ratio
	default:
		// 二次插值，与 Web Audio 的软拐点一致。
		t := xDB - lo
		return xDB + (1/ratio-1)*t*t/(2*knee)
	}
}

// gainReduction 返回该电平应施加的增益（dB，<=0）。
func (c *Compressor) gainReduction(xDB float64) float64 {
	return c.curveDB(xDB) - xDB
}

// makeupDB 是自动补偿增益：让目标 RMS 电平的输入原样输出。
//
// 旧版用 -curve(0)（让 0 dBFS→0 dBFS），导致 makeup 高达 +45.8 dB，
// 安静内容被大幅抬高。新版用 target - curve(target)，makeup 更温和，
// 安静/响亮内容都向目标响度靠拢。
func (c *Compressor) makeupDB() float64 {
	if c.ratio <= 1.0001 && c.threshold >= -0.001 {
		return 0
	}
	return c.targetRMSDB - c.curveDB(c.targetRMSDB)
}
