package media

import (
	"math"
	"sync"
)

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
//
// **关键：补偿增益（makeup gain）**。Web Audio 的 DynamicsCompressorNode
// 会自动加这一项 —— 它按压缩曲线反算，让 0 dBFS 输入仍是 0 dBFS。少了
// 它，-50 dB 阈值 + 12:1 会把**所有**内容压到极小（实测就是"一开均衡
// 声音全变得很小"）。加上之后：满幅信号保持满幅，安静的内容被抬上来，
// 这才是「音量均衡」。
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

	// 每个声道的包络（dB 域的增益，含补偿增益）
	envDB []float64
	// releaseCoef 是释放的单极点系数（0.25s）；起音是瞬时的，没有系数。
	releaseCoef float64
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
		// 释放 250ms（原版 release: 0.25；起音是瞬时的，见 Process）。
		releaseCoef: math.Exp(-1.0 / (0.25 * float64(rate))),
	}
}

// SetEnabled 打开或关闭均衡，参数按 eqTransitionSeconds 线性过渡，
// 避免切换时出现咔哒声。
func (c *Compressor) SetEnabled(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if on {
		c.tThreshold, c.tKnee, c.tRatio = -50, 40, 12
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
		gainDB := c.gainReduction(xDB) + c.makeupDB()
		// 峰值检测：起音**瞬时**（原版 attack: 0）、释放 0.25s。
		//
		// 起音必须瞬时，不能平滑：平滑会让包络在信号峰值处还停在"较低电平
		// 对应的高增益"上，于是满幅内容被抬到 1.0 以上（实测 1.039，会被
		// 后面转 s16 时硬削）。瞬时起音后，任意时刻的增益就是该电平在
		// 压缩曲线上的值，而 curve(0)+makeup=0，所以输出不会超过输入。
		if gainDB < c.envDB[0] {
			for ch2 := 0; ch2 < ch; ch2++ {
				c.envDB[ch2] = gainDB
			}
		} else {
			for ch2 := 0; ch2 < ch; ch2++ {
				c.envDB[ch2] = gainDB + (c.envDB[ch2]-gainDB)*c.releaseCoef
			}
		}
		for ch2 := 0; ch2 < ch; ch2++ {
			g := math.Pow(10, c.envDB[ch2]/20)
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

// makeupDB 是自动补偿增益：让 0 dBFS 输入仍输出 0 dBFS。
//
// 这是 Web Audio DynamicsCompressorNode 自带的行为，也是「音量均衡」能
// 把小音量抬上来的关键 —— 只有压缩没有补偿，就只是把所有声音变小。
func (c *Compressor) makeupDB() float64 {
	if c.ratio <= 1.0001 && c.threshold >= -0.001 {
		return 0
	}
	return -c.curveDB(0)
}
