// Package media 是 bili-FM 的原生音频引擎：解码、变速不变调、音量均衡与输出。
//
// 为什么不用 FFmpeg / 相位声码器：
//
//   - FFmpeg 要 cgo，会丢掉 mygo「CGO_ENABLED=0，任意机器交叉编译」这个前提，
//     包体也从 3MB 级涨到 15~90MB，还牵扯 LGPL 合规。
//   - 相位声码器（如 nanowarp）音质更好，但实测 3x 倍速要吃掉约 0.75 个核心，
//     而且它依赖 cgo 的 PFFFT。实时播放器要的不是「母带级」而是「够好且便宜」。
//
// 所以这里用 WSOLA（波形相似叠加）—— 浏览器与 mpv 的 scaletempo 走的就是
// 这一类算法：只做互相关搜索 + 叠加，开销比相位声码器低一个数量级，且变速
// 不变调。
package media

import (
	"math"
	"sync"
)

// WSOLA 是流式的波形相似叠加变速器：按 speed 改变播放速度而不改变音高。
//
// 算法（Verhelst & Roelands）：
//
//   - 合成端固定跳距 hop（帧长的一半），把加窗的片段叠加起来；
//   - 分析端的目标位置按 hop*speed 推进，这样输出长度就是输入的 1/speed；
//   - 为了让相邻片段在波形上接得上（否则会有周期性咔哒声），在目标位置
//     附近 ±search 内做互相关，挑出与「上一片段自然延续」最相似的位置。
//
// 帧长取 N、合成跳距取 N/2 且窗取 Hann 时，窗函数的叠加和恒为 1，所以
// 不需要额外归一化。
type WSOLA struct {
	// mu 保护全部可变字段：Push/Flush 跑在 oto 的取数据 goroutine 上，
	// SourcePosition/SetSpeed/Speed 跑在播放器的上报 goroutine 上，两者并发。
	mu     sync.Mutex
	ch     int
	n      int // 帧长（单声道采样数）
	hop    int // 合成跳距
	search int // 互相关搜索半径
	window []float32

	in   []float32 // 待处理输入（交错）
	skip int       // in 里已丢弃的采样数（相对流起点的绝对位置）

	// pos 是**标称分析游标**（绝对采样数）。它只按 hop×当帧速度**增量**
	// 前进，不从帧号重算 —— 这是倍速能中途改的前提：
	//   frame×step 的算法里 step 含 speed，改一次速度就会把「过去所有帧」
	//   也按新速度重算，游标一次性向前跳掉一大截（播 1 分钟后切 2x 就跳
	//   1 分钟的源音频），输入永远跟不上 → 直接停住不再产出，进度也跟着跳。
	//   （那个算法原本是为了防漂移：不能按 posPrev+step 递推，因为互相关
	//   每帧能挪 ±search。这里两者兼顾 —— 增量推进用标称游标，互相关只
	//   决定"实际取哪一段"，不进游标。）
	pos float64
	// lastTarget 是最近一帧的标称目标位置，SourcePosition 用它报告
	// 「已经消费到源音频的哪里」。
	lastTarget int
	// posPrev 是上一帧**实际选用**的位置（互相关结果），只作为相似度
	// 模板使用。
	posPrev int
	hasPrev bool

	ola    []float32 // 叠加缓冲（交错），长度 n
	out    []float32 // 已产出但未取走的输出（交错）
	speed  float64
	closed bool
}

// NewWSOLA 建一个变速器。channels 是声道数，speed 是播放速度
// （1 为原速，2 为两倍速）。
// NewWSOLA 建一个变速器。channels 是声道数，speed 是播放速度
// （1 为原速，2 为两倍速）；startSample 是这条流从源音频的第几个采样
// 开始（seek 重建流水线时传入目标位置，SourcePosition 才能接上，
// 不然播放进度会从 0 重算）。
func NewWSOLA(channels int, speed float64, startSample int) *WSOLA {
	if channels < 1 {
		channels = 1
	}
	if speed <= 0 {
		speed = 1
	}
	const (
		n      = 1024
		search = 240
	)
	w := &WSOLA{
		ch:     channels,
		n:      n,
		hop:    n / 2,
		search: search,
		window: hann(n),
		ola:    make([]float32, n*channels),
		speed:  speed,
		skip:   startSample,
		// 游标从 startSample 起（seek 重建时就是目标位置）。原来的实现从
		// 0 起、靠"空转若干帧直到 target 追上 skip"来对齐，既浪费又依赖
		// 边界行为。
		pos:        float64(startSample),
		lastTarget: startSample,
	}
	return w
}

// SetSpeed 改速度。分析游标按增量推进，所以这里只影响**之后**每帧前进的
// 距离，不会回头改写已经消费过的位置 —— 中途改速度是连续的。
func (w *WSOLA) SetSpeed(speed float64) {
	if speed <= 0 {
		return
	}
	w.mu.Lock()
	w.speed = speed
	w.mu.Unlock()
}

// Speed 返回当前速度。
func (w *WSOLA) Speed() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.speed
}

// SourcePosition 返回当前分析位置（相对流起点的采样帧数），
// 也就是「已经消费到源音频的哪里」。播放器用它换算播放进度。
func (w *WSOLA) SourcePosition() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hasPrev {
		return w.lastTarget
	}
	return w.lastTarget + w.n
}

// Push 送进一段交错 PCM（float32，-1..1），返回目前能产出的输出。
// 返回值是内部缓冲的切片，调用方应尽快消费或拷贝。
func (w *WSOLA) Push(pcm []float32) []float32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || len(pcm) == 0 {
		return nil
	}
	w.in = append(w.in, pcm...)
	w.produce()
	out := w.out
	w.out = nil
	return out
}

// Flush 结束输入，把缓冲里剩下的内容补零产出，返回尾部输出。
func (w *WSOLA) Flush() []float32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	// 补足一帧的零，让最后一帧也能叠加完成。
	w.in = append(w.in, make([]float32, w.n*w.ch)...)
	w.produce()
	w.closed = true
	out := w.out
	w.out = nil
	return out
}

// 每帧需要的输入采样数（交错）。
func (w *WSOLA) frameSamples() int { return w.n * w.ch }

// 可用输入帧数（交错采样数 / ch）。
func (w *WSOLA) available() int { return len(w.in)/w.ch + w.skip }

// produce 尽可能多地产出合成帧。
func (w *WSOLA) produce() {
	for {
		// 本帧的标称目标位置：直接取游标（增量推进，见 pos 的注释）。
		target := int(math.Round(w.pos))
		// 搜索需要 target+search+n 个采样可用。
		if w.available() < target+w.search+w.n {
			break
		}

		pos := w.choosePosition(target)
		w.emit(pos)
		w.posPrev = pos
		w.lastTarget = target
		w.hasPrev = true
		// 按**当前**速度推进游标：改速度只影响之后的前进量。
		w.pos += float64(w.hop) * w.speed
	}
	w.discard()
}

// choosePosition 在 target±search 内挑一个与上一帧自然延续最相似的位置。
func (w *WSOLA) choosePosition(target int) int {
	if !w.hasPrev {
		return target
	}
	// 相邻两帧只有重叠区会被叠加：上一帧的后半段 [posPrev+hop, posPrev+n)
	// 与新帧的前半段 [p, p+hop)。让这两段波形尽量一致，叠加才不会互相抵消
	// ——这就是「波形相似」的含义。比较新帧的后半段是错的：那样选出的位置
	// 相位不连贯，输出会有幅度起伏和轻微的频率偏移。
	template := w.slice(w.posPrev+w.hop, w.hop)
	if template == nil {
		return target
	}
	best, bestScore := target, math.Inf(-1)
	for d := -w.search; d <= w.search; d++ {
		cand := w.slice(target+d, w.hop)
		if cand == nil {
			continue
		}
		score := correlate(cand, template)
		if score > bestScore {
			bestScore, best = score, target+d
		}
	}
	return best
}

// emit 把位置 pos 处的一帧加窗叠加进 OLA 缓冲，并推出已定型的 hop 个采样。
func (w *WSOLA) emit(pos int) {
	seg := w.slice(pos, w.n)
	if seg == nil {
		return
	}
	ch := w.ch
	for i := 0; i < w.n; i++ {
		g := w.window[i]
		base := i * ch
		for c := 0; c < ch; c++ {
			w.ola[base+c] += seg[base+c] * g
		}
	}
	// 前 hop 个采样不再有新贡献，可以输出。
	w.out = append(w.out, w.ola[:w.hop*ch]...)
	// OLA 缓冲左移 hop，尾部补零。
	copy(w.ola, w.ola[w.hop*ch:])
	tail := w.ola[(w.n-w.hop)*ch:]
	for i := range tail {
		tail[i] = 0
	}
}

// slice 取绝对位置 start 起的 frames 帧（交错采样）。
func (w *WSOLA) slice(start, frames int) []float32 {
	if frames <= 0 {
		return nil
	}
	begin := (start - w.skip) * w.ch
	end := begin + frames*w.ch
	if begin < 0 || end > len(w.in) {
		return nil
	}
	return w.in[begin:end]
}

// discard 丢掉已经用不到的输入。保留范围要覆盖「上一帧分析位置 - 搜索半径」
// 之前的内容。
func (w *WSOLA) discard() {
	// 用标称游标算保留范围：实际位置总在它的 ±search 之内，取游标更稳。
	keepFrom := int(w.pos) - w.search - w.n
	if keepFrom <= w.skip {
		return
	}
	drop := (keepFrom - w.skip) * w.ch
	if drop > len(w.in) {
		drop = len(w.in)
	}
	w.in = append(w.in[:0], w.in[drop:]...)
	w.skip += drop / w.ch
}

// hann 是周期 Hann 窗：与 hop = n/2 的叠加和恒为 1。
func hann(n int) []float32 {
	w := make([]float32, n)
	for i := range w {
		w[i] = float32(0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n)))
	}
	return w
}

// correlate 是归一化互相关（余弦相似度），返回 -1..1。
// 用归一化而不是原始点积，避免选到单纯音量更大的位置。
func correlate(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}
