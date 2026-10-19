package media

import (
	"math"
	"sync"
)

// Compressor 是音量均衡器：**逐样本复刻 Web Audio 的 DynamicsCompressorNode**
//（Chromium 的 `DynamicsCompressor`，third_party/blink/renderer/platform/audio/
// dynamics_compressor.cc）。
//
// 为什么是它：原版 Tauri（main:src/components/player.tsx:562-570）就是
//
//	source → DynamicsCompressorNode(threshold -50, knee 40, ratio 12,
//	                                attack 0, release 0.25) → destination
//
// 用户参考的项目（bailitaotao/bili-loudness-equalizer，油猴脚本）也是同一张图。
// 原生版此前两次自创过曲线（固定 makeup + RMS 检测；再改成目标响度反算 makeup
// + 硬限幅），听感都与 Tauri 版不同。这一版不再自创：算法、常量、补偿增益
// 逐行对照 Blink 的实现。
//
// 要点（也是之前跑偏的地方）：
//
//   - **检测器是峰值**（逐样本取各声道 |x| 的最大值），不是 RMS；
//   - 压缩器**自带补偿增益** `linear_post_gain = (1/saturate(1,k))^0.6`，让
//     0 dBFS 输入基本仍是 0 dBFS 输出 —— 所以「响的段落被压下去、轻的段落被
//     相对抬上来」，而不是单纯把小音量调大；
//   - 起音 `max(1ms, attack)`，释放按压缩量自适应（4 阶多项式）；
//   - 有 6ms 的前瞻（pre-delay），压缩量按**未延迟**的信号算；
//   - 参数切换按 Web Audio 的 `linearRampToValueAtTime` 在 60ms 内线性过渡
//     （原版 EQ_TRANSITION_SECONDS = 0.06）。
type Compressor struct {
	// mu 保护控制字段：SetEnabled 在界面线程调，Process 在 oto 取数据
	// goroutine 上调。
	mu       sync.Mutex
	channels int
	rate     int

	// 曲线参数（dB / 比值），过渡中是插值后的当前值。
	threshold, knee, ratio float64
	// 过渡目标与起点。
	tThreshold, tKnee, tRatio          float64
	startThreshold, startKnee, startRatio float64
	transLeft, transTotal              int

	// 静态曲线（Blink 的 UpdateStaticCurveParameters / KAtSlope）。
	curveDirty       bool
	curveAge         int
	linearThreshold  float64 // 10^(threshold/20)
	kneeThreshold    float64 // 10^((threshold+knee)/20)
	dbKneeThreshold  float64 // threshold + knee
	dbYKneeThreshold float64
	slope            float64 // 1/ratio
	k, postGain      float64

	// 检测器与增益（Blink 的 detector_average_ / compressor_gain_）。
	detector, gain  float64
	dbMaxAttackDiff float64

	// 6ms 前瞻缓冲（每声道一条环形缓冲）。
	delay      [][]float64
	delayRead  int
	delayWrite int

	// 每块复用的帧数常量。
	attackFrames      float64
	releaseFrames     float64
	satReleaseFrames  float64
	releaseA, releaseB, releaseC, releaseD, releaseE float64
}

const (
	// 原版 EQ_TRANSITION_SECONDS（main:src/components/player.tsx）。
	eqTransitionSeconds = 0.06
	// Blink 的前瞻时间与检测器释放时间（秒）。
	eqPreDelaySeconds     = 0.006
	eqSatReleaseSeconds   = 0.0025
	eqMinAttackSeconds    = 0.001
	// Web Audio 的 DynamicsCompressor 默认参数（Blink InitializeParameters）。
	eqDefaultThreshold = -24.0
	eqDefaultKnee      = 30.0
	eqDefaultRatio     = 12.0
	eqDefaultAttack    = 0.003
	eqDefaultRelease   = 0.25
	// 开启时的参数：原版 player.tsx 的三条 ramp 目标。
	eqOnThreshold = -50.0
	eqOnKnee      = 40.0
	eqOnRatio     = 12.0
	// 关闭时回到「透明」：threshold 0 / knee 0 / ratio 1。
	eqOffThreshold = 0.0
	eqOffKnee      = 0.0
	eqOffRatio     = 1.0
	// 释放曲线的四个分区（Blink 的 kReleaseZone1..4）。
	eqReleaseZone1 = 0.09
	eqReleaseZone2 = 0.16
	eqReleaseZone3 = 0.42
	eqReleaseZone4 = 0.98
)

// NewCompressor 建一个均衡器，初始为透明直通（与 Web Audio 的默认值一致：
// 参数先按「关闭」的 0/0/1 起步，避免起播时突然压缩）。
func NewCompressor(channels, rate int) *Compressor {
	if channels < 1 {
		channels = 1
	}
	if rate < 1 {
		rate = 48000
	}
	delayFrames := int(eqPreDelaySeconds*float64(rate)) + 1
	c := &Compressor{
		channels:   channels,
		rate:       rate,
		threshold:  eqOffThreshold,
		knee:       eqOffKnee,
		ratio:      eqOffRatio,
		tThreshold: eqOffThreshold,
		tKnee:      eqOffKnee,
		tRatio:     eqOffRatio,
		detector:   0,
		gain:       1,
		// Blink 的 db_max_attack_compression_diff_ 初值是 -1（未初始化哨兵）。
		dbMaxAttackDiff: -1,
		delay:           make([][]float64, channels),
	}
	for i := range c.delay {
		c.delay[i] = make([]float64, delayFrames)
	}
	c.delayWrite = delayFrames - 1 // read 落后 write 一个预延迟长度
	c.delayRead = 0
	c.updateFrameConstants()
	c.curveDirty = true
	c.updateCurve()
	return c
}

// updateFrameConstants 按当前采样率算好 Blink 的三个帧数常量与释放多项式系数
// （这些只随采样率/释放时间变化）。
func (c *Compressor) updateFrameConstants() {
	sr := float64(c.rate)
	// 原版把 attack 设成 0，Blink 取 max(1ms, attack) = 1ms。
	c.attackFrames = eqMinAttackSeconds * sr
	c.releaseFrames = sr * eqDefaultRelease
	c.satReleaseFrames = eqSatReleaseSeconds * sr
	// 4 阶多项式：y = a + b·x + c·x² + d·x³ + e·x⁴，x∈[0,3] 对应 -15..0 dB。
	z1, z2, z3, z4 := eqReleaseZone1, eqReleaseZone2, eqReleaseZone3, eqReleaseZone4
	c.releaseA = 0.9999999999999998*z1 + 1.8432219684323923e-16*z2 - 1.9373394351676423e-16*z3 + 8.824516011816245e-18*z4
	c.releaseB = -1.5788320352845888*z1 + 2.3305837032074286*z2 - 0.9141194204840429*z3 + 0.1623677525612032*z4
	c.releaseC = 0.5334142869106424*z1 - 1.272736789213631*z2 + 0.9258856042207512*z3 - 0.18656310191776226*z4
	c.releaseD = 0.08783463138207234*z1 - 0.1694162967925622*z2 + 0.08588057951595272*z3 - 0.00429891410546283*z4
	c.releaseE = -0.042416883008123074*z1 + 0.1115693827987602*z2 - 0.09764676325265872*z3 + 0.028494263462021576*z4
	c.releaseA *= c.releaseFrames
	c.releaseB *= c.releaseFrames
	c.releaseC *= c.releaseFrames
	c.releaseD *= c.releaseFrames
	c.releaseE *= c.releaseFrames
}

// SetEnabled 打开或关闭均衡，参数按 60ms 线性过渡（与原版
// `linearRampToValueAtTime` 一致），避免切换时咔哒。
func (c *Compressor) SetEnabled(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.startThreshold, c.startKnee, c.startRatio = c.threshold, c.knee, c.ratio
	if on {
		c.tThreshold, c.tKnee, c.tRatio = eqOnThreshold, eqOnKnee, eqOnRatio
	} else {
		c.tThreshold, c.tKnee, c.tRatio = eqOffThreshold, eqOffKnee, eqOffRatio
	}
	c.transTotal = int(eqTransitionSeconds * float64(c.rate))
	if c.transTotal < 1 {
		c.transTotal = 1
	}
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
	if frames == 0 {
		return
	}

	// 完全关闭且过渡已经走完：透明直通（把状态复位成「没有压缩」）。
	if c.transLeft == 0 && c.ratio <= 1.0000001 && c.threshold >= -0.0000001 && c.knee >= -0.0000001 {
		c.detector, c.gain, c.dbMaxAttackDiff = 0, 1, -1
		return
	}

	delayLen := len(c.delay[0])

	for i := 0; i < frames; i++ {
		// 参数过渡：逐样本线性 ramp。
		if c.transLeft > 0 {
			c.transLeft--
			k := 1.0
			if c.transTotal > 0 {
				k = 1 - float64(c.transLeft)/float64(c.transTotal)
			}
			c.threshold = c.startThreshold + (c.tThreshold-c.startThreshold)*k
			c.knee = c.startKnee + (c.tKnee-c.startKnee)*k
			c.ratio = c.startRatio + (c.tRatio-c.startRatio)*k
			if c.transLeft == 0 {
				c.threshold, c.knee, c.ratio = c.tThreshold, c.tKnee, c.tRatio
			}
			c.curveDirty = true
		}
		// 曲线重算很贵（15 次二分 × 每声道 dB 转换）：过渡期间每 64 帧一次，
		// 相当于 Blink 每个 128 帧渲染量子更新一次。
		if c.curveDirty && c.curveAge%64 == 0 {
			c.updateCurve()
		}
		c.curveAge++

		// 前瞻：先把未延迟的样本写进环形缓冲，压缩量按未延迟信号算。
		base := i * ch
		var peak float64
		for j := 0; j < ch; j++ {
			v := float64(buf[base+j])
			c.delay[j][c.delayWrite] = v
			if a := math.Abs(v); a > peak {
				peak = a
			}
		}

		// 整形曲线 → 衰减 → 检测器（起音瞬时、释放按 Blink 的 sat release）。
		attenuation := 1.0
		if peak > 0.0001 {
			attenuation = c.saturate(peak) / peak
		}
		dbAttenuation := math.Max(2.0, -linearToDecibels(attenuation))
		satReleaseRate := decibelsToLinear(dbAttenuation/c.satReleaseFrames) - 1
		rate := 1.0
		if attenuation > c.detector {
			rate = satReleaseRate
		}
		c.detector += (attenuation - c.detector) * rate
		if c.detector > 1 {
			c.detector = 1
		}
		if c.detector < 0 || math.IsNaN(c.detector) {
			c.detector = 1
		}

		// 期望增益（pre-warp，让 sin() 之后刚好等于 detector）。
		desired := c.detector
		if desired < 0 {
			desired = 0
		}
		if desired > 1 {
			desired = 1
		}
		scaled := math.Asin(desired) / (math.Pi / 2)

		// 包络：起音/释放两条速率，与 Blink 相同。
		releasing := scaled > c.gain
		diff := 0.0
		if scaled == 0 {
			if releasing {
				diff = -1
			} else {
				diff = 1
			}
		} else {
			diff = linearToDecibels(c.gain / scaled)
		}
		envelopeRate := 0.0
		if releasing {
			c.dbMaxAttackDiff = -1
			if !isFinite(diff) {
				diff = -1
			}
			x := clampFloat(diff, -12, 0)
			x = 0.25 * (x + 12)
			x2 := x * x
			calcReleaseFrames := c.releaseA + c.releaseB*x + c.releaseC*x2 +
				c.releaseD*x2*x + c.releaseE*x2*x2
			if calcReleaseFrames < 1 {
				calcReleaseFrames = 1
			}
			envelopeRate = decibelsToLinear(5.0 / calcReleaseFrames)
		} else {
			if !isFinite(diff) {
				diff = 1
			}
			if c.dbMaxAttackDiff == -1 || c.dbMaxAttackDiff < diff {
				c.dbMaxAttackDiff = diff
			}
			dbEff := math.Max(0.5, c.dbMaxAttackDiff)
			envelopeRate = 1 - math.Pow(0.25/dbEff, 1/c.attackFrames)
		}
		if envelopeRate < 1 {
			c.gain += (scaled - c.gain) * envelopeRate
		} else {
			c.gain *= envelopeRate
			if c.gain > 1 {
				c.gain = 1
			}
		}
		if math.IsNaN(c.gain) || math.IsInf(c.gain, 0) {
			c.gain = 1
		}

		// sin 整形 + 补偿增益（Blink 的 post_warp_compressor_gain / total_gain）。
		totalGain := c.postGain * math.Sin((math.Pi/2)*c.gain)

		// 输出延迟后的样本。
		for j := 0; j < ch; j++ {
			buf[base+j] = float32(c.delay[j][c.delayRead] * totalGain)
		}
		c.delayRead++
		if c.delayRead >= delayLen {
			c.delayRead = 0
		}
		c.delayWrite++
		if c.delayWrite >= delayLen {
			c.delayWrite = 0
		}
	}
}

// updateCurve 复刻 Blink 的 UpdateStaticCurveParameters：按 threshold/knee/ratio
// 求 k（15 次二分）与各阈值，再算补偿增益。
func (c *Compressor) updateCurve() {
	c.curveAge = 0
	c.curveDirty = false
	c.linearThreshold = decibelsToLinear(c.threshold)
	c.dbKneeThreshold = c.threshold + c.knee
	c.kneeThreshold = decibelsToLinear(c.dbKneeThreshold)
	c.slope = 1 / c.ratio
	c.k = c.kAtSlope(c.slope)
	c.dbYKneeThreshold = linearToDecibels(c.kneeCurve(c.kneeThreshold, c.k))
	// post_gain = (1/saturate(1,k))^0.6；saturate 在 k=0 以下不会发生。
	s := c.saturate(1)
	if s < 1e-6 {
		s = 1e-6
	}
	c.postGain = math.Pow(1/s, 0.6)
}

// kneeCurve 是阈值以上的指数软拐点（Blink 的 KneeCurve）。
func (c *Compressor) kneeCurve(x, k float64) float64 {
	if x < c.linearThreshold {
		return x
	}
	return c.linearThreshold + (1-math.Exp(-k*(x-c.linearThreshold)))/k
}

// saturate 是完整压缩曲线（Blink 的 Saturate）：膝盖以下走 KneeCurve，以上按
// 固定 ratio 的直线。
func (c *Compressor) saturate(x float64) float64 {
	if x < c.kneeThreshold {
		return c.kneeCurve(x, c.k)
	}
	dbX := linearToDecibels(x)
	dbY := c.dbYKneeThreshold + c.slope*(dbX-c.dbKneeThreshold)
	return decibelsToLinear(dbY)
}

// kAtSlope 用 15 次二分逼近「让拐点处斜率为 desiredSlope」的 k
// （Blink 的 KAtSlope）。
func (c *Compressor) kAtSlope(desiredSlope float64) float64 {
	dbX := c.threshold + c.knee
	x := decibelsToLinear(dbX)
	x2 := 1.0
	dbX2 := 0.0
	if !(x < c.linearThreshold) {
		x2 = x * 1.001
		dbX2 = linearToDecibels(x2)
	}
	minK, maxK, k := 0.1, 10000.0, 5.0
	for i := 0; i < 15; i++ {
		slope := 1.0
		if !(x < c.linearThreshold) {
			dbY := linearToDecibels(c.kneeCurve(x, k))
			dbY2 := linearToDecibels(c.kneeCurve(x2, k))
			if dbX2 != dbX {
				slope = (dbY2 - dbY) / (dbX2 - dbX)
			}
		}
		if slope < desiredSlope {
			maxK = k
		} else {
			minK = k
		}
		k = math.Sqrt(minK * maxK)
	}
	return k
}

// linearToDecibels 与 Blink 的 audio_utilities 相同：0 → -1000 dB。
func linearToDecibels(x float64) float64 {
	if x <= 0 {
		return -1000
	}
	return 20 * math.Log10(x)
}

// decibelsToLinear 是 linearToDecibels 的逆。
func decibelsToLinear(db float64) float64 {
	return math.Pow(10, db/20)
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
