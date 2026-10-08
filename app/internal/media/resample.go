package media

import "math"

// resampler 做采样率转换，用 Catmull-Rom 三次插值。
//
// 为什么需要它：oto 的文档写明「创建多个 context 是不支持的」，所以音频
// 输出上下文只能用一种采样率。B 站的 html5 音频有 44100 和 48000 两种，
// 上下文按第一首的采样率建立，之后遇到不同采样率的曲子就在这里转换。
//
// 三次插值比线性插值的高频失真小得多，而两者开销相当（每输出采样几个
// 乘加），对播放来说可以忽略。
type resampler struct {
	ch      int
	inRate  int
	outRate int
	ratio   float64 // 每个输出采样对应多少个输入采样

	// hist 保存上一块的最后几帧，供跨块插值使用。
	hist []float32
	// pos 是输入域上的位置（帧），相对 hist 起点。
	pos float64
	// primed 表示 hist 里已经有足够的前置数据。
	primed bool
}

// newResampler 建一个从 inRate 到 outRate 的转换器。
func newResampler(channels, inRate, outRate int) *resampler {
	return &resampler{
		ch:      channels,
		inRate:  inRate,
		outRate: outRate,
		ratio:   float64(inRate) / float64(outRate),
		hist:    make([]float32, 0, 8*channels),
	}
}

// Push 送进一段交错 PCM，返回转换后的交错 PCM。
func (r *resampler) Push(in []float32) []float32 {
	if r.inRate == r.outRate || len(in) == 0 {
		return in
	}
	ch := r.ch

	// 拼上历史，保证插值需要的 3 个前置点可用。
	buf := make([]float32, 0, len(r.hist)+len(in))
	buf = append(buf, r.hist...)
	buf = append(buf, in...)
	base := 0.0 // buf[0] 对应的输入帧号

	nIn := len(buf) / ch
	if nIn < 4 {
		r.hist = append(r.hist[:0], buf...)
		return nil
	}
	// 能安全输出的上限：需要 i+1 帧存在。
	maxPos := float64(nIn - 2)

	outFrames := 0
	if r.primed {
		outFrames = int(math.Floor((maxPos - r.pos) / r.ratio))
	} else {
		// 第一块：从第 1 帧开始，前面留一帧做插值。
		r.pos = 1
		r.primed = true
		outFrames = int(math.Floor((maxPos - r.pos) / r.ratio))
	}
	if outFrames <= 0 {
		r.hist = append(r.hist[:0], buf...)
		return nil
	}

	out := make([]float32, outFrames*ch)
	p := r.pos
	for i := 0; i < outFrames; i++ {
		i0 := int(p)
		frac := float32(p - float64(i0))
		for c := 0; c < ch; c++ {
			y0 := buf[(i0-1)*ch+c]
			y1 := buf[i0*ch+c]
			y2 := buf[(i0+1)*ch+c]
			y3 := buf[(i0+2)*ch+c]
			out[i*ch+c] = catmullRom(y0, y1, y2, y3, frac)
		}
		p += r.ratio
	}
	_ = base

	// 保留尾部：下一块的插值需要回看几帧。
	consumed := int(p) - 2
	if consumed < 0 {
		consumed = 0
	}
	if consumed*ch > len(buf) {
		consumed = len(buf) / ch
	}
	r.hist = append(r.hist[:0], buf[consumed*ch:]...)
	r.pos = p - float64(consumed)
	return out
}

// catmullRom 在 y1 与 y2 之间按 t（0..1）插值。
func catmullRom(y0, y1, y2, y3, t float32) float32 {
	a := -0.5*y0 + 1.5*y1 - 1.5*y2 + 0.5*y3
	b := y0 - 2.5*y1 + 2*y2 - 0.5*y3
	c := -0.5*y0 + 0.5*y2
	return ((a*t+b)*t+c)*t + y1
}
