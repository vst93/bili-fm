package media

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
	aacpcm "github.com/tphakala/go-aac/pcm"
	m4a "github.com/tphakala/go-m4a"
)

// Player 是 bili-FM 的音频播放器：HTTP 流 → AAC 解码 → WSOLA 变速 →
// 音量均衡 → 声卡输出。全程纯 Go，CGO_ENABLED=0 也能交叉编译。
type Player struct {
	mu sync.Mutex

	ctx     *oto.Context
	ctxRate int
	ctxCh   int

	op  *oto.Player
	cur *pipeline

	speed float64
	eqOn  bool
	vol   float64

	onProgress func(pos, dur float64)
	onEnded    func()
	onError    func(error)

	stopProgress chan struct{}
}

// NewPlayer 建一个播放器。
func NewPlayer() *Player {
	return &Player{speed: 1, vol: 1}
}

// OnProgress 设置进度回调，约每 500ms 一次（pos/dur 单位秒）。
func (p *Player) OnProgress(fn func(pos, dur float64)) { p.onProgress = fn }

// OnEnded 设置播放结束回调。
func (p *Player) OnEnded(fn func()) { p.onEnded = fn }

// OnError 设置错误回调。
func (p *Player) OnError(fn func(error)) { p.onError = fn }

// Play 开始播放一个音频地址。headers 会附加到每个请求上（B 站 CDN 需要
// Referer 与 User-Agent）。
func (p *Player) Play(url string, headers map[string]string) error {
	p.Stop()

	rf, err := newRemoteFile(url, headers)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	pipe, err := newPipeline(rf, p.ctxRate, p.speed, p.eqOn, p.ensureContext)
	if err != nil {
		return err
	}
	if p.ctx == nil {
		return errors.New("media: 音频上下文未建立")
	}
	p.cur = pipe
	p.op = p.ctx.NewPlayer(pipe)
	p.op.SetVolume(p.vol)
	p.op.Play()

	p.stopProgress = make(chan struct{})
	go p.reportLoop(p.stopProgress)
	return nil
}

// ensureContext 保证有一个 oto 上下文，返回输出采样率。
//
// oto 明确不支持创建多个 context，所以上下文只在第一次播放时按当时流的
// 采样率建立；之后采样率不同的流由 pipeline 内的重采样器转换。
func (p *Player) ensureContext(rate, channels int) (int, error) {
	if p.ctx != nil {
		if channels != p.ctxCh {
			return 0, fmt.Errorf("media: 声道数从 %d 变成 %d，暂不支持", p.ctxCh, channels)
		}
		return p.ctxRate, nil
	}
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   rate,
		ChannelCount: channels,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return 0, fmt.Errorf("media: 打开音频输出失败: %w", err)
	}
	<-ready
	p.ctx, p.ctxRate, p.ctxCh = ctx, rate, channels
	return rate, nil
}

// Pause 暂停。
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.op != nil {
		p.op.Pause()
	}
}

// Resume 继续。
func (p *Player) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.op != nil {
		p.op.Play()
	}
}

// Playing 报告是否正在播放。
func (p *Player) Playing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.op != nil && p.op.IsPlaying()
}

// Stop 停止并释放当前流。
func (p *Player) Stop() {
	p.mu.Lock()
	stop := p.stopProgress
	p.stopProgress = nil
	op, cur := p.op, p.cur
	p.op, p.cur = nil, nil
	p.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if op != nil {
		op.Close()
	}
	if cur != nil {
		cur.close()
	}
}

// Close 停止播放。oto 的 context 不能关闭，随进程结束回收。
func (p *Player) Close() { p.Stop() }

// SetSpeed 设置播放速度（0.5 ~ 3.0），变速不变调。
func (p *Player) SetSpeed(v float64) {
	if v <= 0 {
		return
	}
	p.mu.Lock()
	p.speed = v
	cur := p.cur
	p.mu.Unlock()
	if cur != nil {
		cur.ws.SetSpeed(v)
	}
}

// Speed 返回当前速度。
func (p *Player) Speed() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.speed
}

// SetEQ 开关音量均衡。
func (p *Player) SetEQ(on bool) {
	p.mu.Lock()
	p.eqOn = on
	cur := p.cur
	p.mu.Unlock()
	if cur != nil {
		cur.setEQ(on)
	}
}

// EQ 报告音量均衡是否开启。
func (p *Player) EQ() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.eqOn
}

// SetVolume 设置音量 0..1。
func (p *Player) SetVolume(v float64) {
	p.mu.Lock()
	p.vol = math.Max(0, math.Min(1, v))
	op := p.op
	p.mu.Unlock()
	if op != nil {
		op.SetVolume(p.vol)
	}
}

// Position 返回当前播放位置（源音频的秒数）。
func (p *Player) Position() float64 {
	p.mu.Lock()
	cur := p.cur
	p.mu.Unlock()
	if cur == nil {
		return 0
	}
	return cur.position()
}

// Duration 返回总时长（秒）。
func (p *Player) Duration() float64 {
	p.mu.Lock()
	cur := p.cur
	p.mu.Unlock()
	if cur == nil {
		return 0
	}
	return cur.duration()
}

// Seek 跳到指定秒数。
func (p *Player) Seek(seconds float64) error {
	p.mu.Lock()
	cur := p.cur
	p.mu.Unlock()
	if cur == nil {
		return errors.New("media: 当前没有在播放")
	}
	return cur.seek(seconds)
}

func (p *Player) reportLoop(stop chan struct{}) {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			p.mu.Lock()
			cur, ended, err := p.cur, false, error(nil)
			if cur != nil {
				ended, err = cur.state()
			}
			p.mu.Unlock()
			if err != nil && p.onError != nil {
				p.onError(err)
				return
			}
			if cur == nil {
				continue
			}
			if p.onProgress != nil {
				p.onProgress(cur.position(), cur.duration())
			}
			if ended {
				if p.onEnded != nil {
					p.onEnded()
				}
				return
			}
		}
	}
}

// pipeline 是「解码 → 变速 → 均衡 → s16le 字节」的流水线，
// 实现 io.Reader 供 oto 拉取。
type pipeline struct {
	mu sync.Mutex

	rf   *remoteFile
	rd   *m4a.Reader
	dec  *aacpcm.Decoder
	info m4a.Info

	channels int
	ws       *WSOLA
	eq       *Compressor
	// rs 在输出采样率与流的采样率不同时做转换，nil 表示不需要。
	rs *resampler

	src   []byte
	flt   []float32
	out   []byte
	outAt int

	eof  bool
	err  error
	done bool
}

// newPipeline 建流水线。outRate 是音频上下文的采样率（0 表示还没建立，
// 此时用流的采样率）；ensure 负责在需要时建立上下文并返回它用的采样率。
func newPipeline(rf *remoteFile, outRate int, speed float64, eqOn bool, ensure func(rate, ch int) (int, error)) (*pipeline, error) {
	rd, err := m4a.NewReader(rf)
	if err != nil {
		return nil, fmt.Errorf("media: 解析 MP4 失败: %w", err)
	}
	info := rd.Info()
	dec, err := aacpcm.NewDecoder(rd.RawStream(), aacpcm.WithRawStream(info.ASC))
	if err != nil {
		return nil, fmt.Errorf("media: 打开 AAC 解码器失败: %w", err)
	}
	rate, err := ensure(info.SampleRate, info.Channels)
	if err != nil {
		return nil, err
	}
	p := &pipeline{
		rf: rf, rd: rd, dec: dec, info: info,
		channels: info.Channels,
		ws:       NewWSOLA(info.Channels, speed, 0),
		eq:       NewCompressor(info.Channels, info.SampleRate),
	}
	if info.SampleRate != rate {
		p.rs = newResampler(info.Channels, info.SampleRate, rate)
	}
	if eqOn {
		p.eq.SetEnabled(true)
	}
	return p, nil
}

func (p *pipeline) close() {
	if p.dec != nil {
		_ = p.dec.Reset(nil)
	}
}

func (p *pipeline) duration() float64 {
	return p.info.Duration.Seconds()
}

func (p *pipeline) position() float64 {
	p.mu.Lock()
	ws := p.ws
	p.mu.Unlock()
	if ws == nil {
		return 0
	}
	return float64(ws.SourcePosition()) / float64(p.info.SampleRate)
}

func (p *pipeline) state() (ended bool, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.done, p.err
}

// seek 跳到指定秒数：重建解封装器，用 ReadFrame 跳过前面若干帧
// （只定位不decoding，比解码丢弃快得多），再重建解码器。
// setEQ 在流水线锁内切换均衡，避免与 Read（oto goroutine）里的
// eq.Process 并发写同一个压缩器的控制状态。
func (p *pipeline) setEQ(on bool) {
	p.mu.Lock()
	p.eq.SetEnabled(on)
	p.mu.Unlock()
}

func (p *pipeline) seek(seconds float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if seconds < 0 {
		seconds = 0
	}
	dur := p.info.Duration.Seconds()
	if dur > 0 && seconds > dur {
		seconds = dur
	}

	// 时间 → 帧号：每帧的采样数固定，据此把秒换成帧下标。
	frameSamples := 1024
	if p.info.FrameCount > 0 && dur > 0 {
		frameSamples = int(math.Round(float64(p.info.SampleRate) * dur / float64(p.info.FrameCount)))
	}
	skip := int(seconds * float64(p.info.SampleRate) / float64(frameSamples))

	if _, err := p.rf.Seek(0, io.SeekStart); err != nil {
		return err
	}
	rd, err := m4a.NewReader(p.rf)
	if err != nil {
		return fmt.Errorf("media: 重新解析失败: %w", err)
	}
	// 直接把游标定位到目标帧（库按 sample table 算出字节偏移，
	// 只发几个 Range 请求）——之前是逐帧 ReadFrame 跳过，跳一小时
	// 要发 ~15 万个请求，seek 卡死就卡在这。
	if skip > 0 {
		if err := rd.SeekSample(skip); err != nil {
			log.Printf("seek 定位到帧 %d 失败（退回顺序读）: %v", skip, err)
		}
	}
	dec, err := aacpcm.NewDecoder(rd.RawStream(), aacpcm.WithRawStream(p.info.ASC))
	if err != nil {
		return fmt.Errorf("media: 重新打开解码器失败: %w", err)
	}

	p.rd, p.dec = rd, dec
	// 起始采样按**源音频**的采样数算（不是 WSOLA 输出的采样数）：
	// SourcePosition 用源采样率换算秒。seek 只重定位不解码，前面
	// 的帧没经过变速，所以直接用 seconds × 源采样率。
	startSample := int(seconds * float64(p.info.SampleRate))
	p.ws = NewWSOLA(p.channels, p.ws.Speed(), startSample)
	if p.eq.Enabled() {
		p.eq.SetEnabled(true)
	}
	p.out, p.outAt = nil, 0
	p.eof, p.done, p.err = false, false, nil
	return nil
}

// Read 实现 io.Reader，供 oto 拉取 s16le 字节。
func (p *pipeline) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for p.outAt >= len(p.out) {
		if p.eof {
			p.done = true
			return 0, io.EOF
		}
		if err := p.fillLocked(); err != nil {
			p.err = err
			return 0, err
		}
	}
	n := copy(b, p.out[p.outAt:])
	p.outAt += n
	return n, nil
}

// fillLocked 解码一块、变速、均衡，产出 s16le 字节到 p.out。
func (p *pipeline) fillLocked() error {
	const want = 4096 // 每次解码的采样帧数
	if len(p.src) < want*p.channels*2 {
		p.src = make([]byte, want*p.channels*2)
	}
	n, err := io.ReadFull(p.dec, p.src)
	if n == 0 && err != nil {
		// 输入结束：让 WSOLA 把尾巴吐出来。
		if len(p.flt) > 0 {
			p.flt = nil
		}
		tail := p.ws.Flush()
		p.eq.Process(tail)
		if p.rs != nil {
			tail = p.rs.Push(tail)
		}
		p.out = appendS16(p.out[:0], tail)
		p.outAt = 0
		p.eof = true
		return nil
	}
	frames := n / (p.channels * 2)
	if frames == 0 {
		return nil
	}

	if cap(p.flt) < frames*p.channels {
		p.flt = make([]float32, frames*p.channels)
	}
	p.flt = p.flt[:frames*p.channels]
	for i := 0; i < frames*p.channels; i++ {
		p.flt[i] = float32(int16(uint16(p.src[2*i])|uint16(p.src[2*i+1])<<8)) / 32768
	}

	scaled := p.ws.Push(p.flt)
	p.eq.Process(scaled)
	if p.rs != nil {
		scaled = p.rs.Push(scaled)
	}
	p.out = appendS16(p.out[:0], scaled)
	p.outAt = 0
	if len(p.out) == 0 {
		return p.fillLocked() // WSOLA 还没攒够一帧，继续读
	}
	return nil
}

// appendS16 把交错 float32 转成 s16le 字节。
func appendS16(dst []byte, src []float32) []byte {
	for _, v := range src {
		x := v * 32767
		if x > 32767 {
			x = 32767
		} else if x < -32768 {
			x = -32768
		}
		u := uint16(int16(x))
		dst = append(dst, byte(u), byte(u>>8))
	}
	return dst
}
