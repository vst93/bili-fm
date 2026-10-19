package media

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"sync"
	"sync/atomic"
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

	// gen 是起播代号，见 Play。
	gen atomic.Uint64
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
	// 给这次起播一个代号。建流水线要发 HTTP（几秒级），期间用户完全可能
	// 又切了一首 —— 那时这一次的结果必须作废，否则两个 oto player 会
	// **同时**在拉数据（两首歌叠着放），而且先前那个永远关不掉。
	gen := p.gen.Add(1)

	rf, err := newRemoteFile(url, headers)
	if err != nil {
		return err
	}

	// 建流水线要发 HTTP 请求解析 MP4 头（几秒级），**绝不能握着 p.mu**：
	// UI 线程上的 SetSpeed/SetVolume 会一起干等（实测倍速切换卡过 22 秒）。
	// 先在锁外建好，再进锁只做一次交换。
	p.mu.Lock()
	rate, speed, eqOn := p.ctxRate, p.speed, p.eqOn
	p.mu.Unlock()

	pipe, err := newPipeline(rf, rate, speed, eqOn, p.ensureContext)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctx == nil || p.gen.Load() != gen {
		// 上下文没建好，或期间已有更新的起播：关掉这次建好的，
		// 什么都不装。
		p.mu.Unlock()
		pipe.close()
		p.mu.Lock()
		return ErrSuperseded
	}
	// 安装前先关掉上一个（并发的起播可能在这之前装上了自己的）。
	if p.op != nil {
		p.op.Close()
		p.op = nil
	}
	if p.cur != nil {
		p.cur.close()
		p.cur = nil
	}
	if p.stopProgress != nil {
		close(p.stopProgress)
		p.stopProgress = nil
	}
	// 建流水线期间用户可能改过倍速/均衡，以**当前**值为准，
	// 不然这几秒里的改动会被建的时候那份快照吃掉。
	pipe.ws.SetSpeed(p.speed)
	pipe.eq.SetEnabled(p.eqOn)
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
	// Play 现在在锁外建流水线，所以这里自己加锁读已有的上下文。
	// oto.NewContext（开音频设备）本身也在锁外做，不然又会卡住 UI 线程。
	p.mu.Lock()
	have, ctxRate, ctxCh := p.ctx != nil, p.ctxRate, p.ctxCh
	p.mu.Unlock()
	if have {
		if channels != ctxCh {
			return 0, WrapKind(KindUnsupported, fmt.Errorf("media: 声道数从 %d 变成 %d，暂不支持", ctxCh, channels))
		}
		return ctxRate, nil
	}
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   rate,
		ChannelCount: channels,
		Format:       oto.FormatSignedInt16LE,
		// 设备缓冲给足 150ms：底层驱动默认只有几十毫秒，网络抖一下就见底。
		// 我们的预读队列（1.5s）负责"不出现欠载"，这个缓冲负责"万一欠载了
		// 也不至于立刻爆音"。150ms 的延迟对音乐播放器无感知。
		BufferSize: 150 * time.Millisecond,
	})
	if err != nil {
		return 0, WrapKind(KindOutput, fmt.Errorf("media: 打开音频输出失败: %w", err))
	}
	<-ready
	p.mu.Lock()
	// 两个 goroutine 同时第一次起播时，先到的赢，后来者用已建好的。
	if p.ctx == nil {
		p.ctx, p.ctxRate, p.ctxCh = ctx, rate, channels
	} else {
		rate = p.ctxRate
	}
	p.mu.Unlock()
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

// Buffered 返回当前曲目已预读到本地的媒体秒数（0 表示未知或还没开始）。
// 对应旧版 Web Audio 的 audio.buffered 末段，用于进度条上的缓冲段。
// 只读原子量，不会因为一次网络读而阻塞调用方。
func (p *Player) Buffered() float64 {
	p.mu.Lock()
	cur := p.cur
	p.mu.Unlock()
	if cur == nil {
		return 0
	}
	return cur.buffered()
}

// Underruns 返回当前曲目的欠载次数：就绪队列被声卡吃空、只能补静音的
// 次数。诊断「播着播着停一下」用（-memstats 会打出来）。
func (p *Player) Underruns() int {
	p.mu.Lock()
	cur := p.cur
	p.mu.Unlock()
	if cur == nil {
		return 0
	}
	return cur.Underruns()
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
			// 只握着 p.mu 取指针就放开：cur.state()/cur.position() 要拿
			// 流水线的锁，而那把锁会被 otor 取数据的 goroutine 在网络读时
			// 长期持有 —— 在 p.mu 里等它，UI 线程的 SetSpeed/SetVolume
			// 又会一起卡住（和 Play 那次是同一类问题）。
			p.mu.Lock()
			cur := p.cur
			p.mu.Unlock()
			ended, err := false, error(nil)
			if cur != nil {
				ended, err = cur.state()
			}
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
	// ── 并发模型（这是播放不卡的关键）──────────────────────────
	//
	//   后台生产者 goroutine ──► 就绪队列（s16le 字节）──► oto 的取数回调
	//        拿 procMu，做网络 I/O      只拿 mu            只拿 mu，永不碰网络
	//
	// 取数回调是声卡饿了才来叫的，**绝不能让它碰网络**：一碰，网络抖动就
	// 直接变成爆音/卡顿（oto 的设备缓冲只有几十毫秒，一次 HTTP 往返就够
	// 让它见底）。所以解码提前在后台做，队列里始终留着约 1.5 秒的数据，
	// 回调只负责把字节搬走。
	//
	// 三把锁各管一块，互不嵌套等待：
	//   procMu —— 「解码→变速→均衡→重采样」这条链与它的缓冲，还有 seek 重建
	//   mu     —— 就绪队列与对外状态（done/err/欠载计数）
	//   wsMu   —— 只有 ws 指针（position() 要读它，不能等 procMu）
	procMu sync.Mutex

	rf   *remoteFile
	rd   *m4a.Reader
	dec  *aacpcm.Decoder
	info m4a.Info

	channels int
	wsMu     sync.Mutex
	ws       *WSOLA
	eq       *Compressor
	// rs 在输出采样率与流的采样率不同时做转换，nil 表示不需要。
	rs *resampler

	// 处理链的临时缓冲（只在 procMu 内用）。
	src []byte
	flt []float32
	enc []byte
	// procEOF 表示输入已到末端（生产者在 procMu 内读写）。
	procEOF bool

	mu    sync.Mutex
	ready []byte
	readyAt int

	// raw 是「解码 → 变速 → 重采样」之后、**还没过均衡**的 float32 队列（大缓冲，
	// 5 秒）。均衡放在紧挨声卡的一小段（eqTargetBytes ≈ 200ms）上做 —— 开关切换
	// 只影响这 200ms，才会「立刻听见」；如果连 5 秒的大缓冲一起过均衡，切换要等
	// 缓冲播完才生效（用户报的「切均衡有几秒延迟」正是这个）。
	raw    []float32
	rawAt  int
	rawEOF bool
	eqWake chan struct{}

	started bool
	eof     bool
	err     error
	done    bool
	under   int

	stop chan struct{}
	// wake 用来把生产者从「队列已满」的等待里叫醒（seek 之后要立刻续产）。
	wake chan struct{}
	once sync.Once
}

// newPipeline 建流水线。outRate 是音频上下文的采样率（0 表示还没建立，
// 此时用流的采样率）；ensure 负责在需要时建立上下文并返回它用的采样率。
func newPipeline(rf *remoteFile, outRate int, speed float64, eqOn bool, ensure func(rate, ch int) (int, error)) (*pipeline, error) {
	rd, err := m4a.NewReader(rf)
	if err != nil {
		return nil, WrapKind(KindUnsupported, fmt.Errorf("media: 解析 MP4 失败: %w", err))
	}
	info := rd.Info()
	dec, err := aacpcm.NewDecoder(rd.RawStream(), aacpcm.WithRawStream(info.ASC))
	if err != nil {
		return nil, WrapKind(KindDecode, fmt.Errorf("media: 打开 AAC 解码器失败: %w", err))
	}
	rate, err := ensure(info.SampleRate, info.Channels)
	if err != nil {
		return nil, err
	}
	p := &pipeline{
		rf: rf, rd: rd, dec: dec, info: info,
		channels: info.Channels,
		ws:       NewWSOLA(info.Channels, speed, 0),
		// 均衡在重采样之后跑，采样率用**输出**采样率。
		eq:     NewCompressor(info.Channels, rate),
		eqWake: make(chan struct{}, 1),
	}
	if info.SampleRate != rate {
		p.rs = newResampler(info.Channels, info.SampleRate, rate)
	}
	if eqOn {
		p.eq.SetEnabled(true)
	}
	p.stop, p.wake = make(chan struct{}), make(chan struct{}, 1)
	go p.pump()
	go p.finish()
	return p, nil
}

// readAheadFrames 是 raw 队列（未过均衡）维持的水位：约 5 秒的采样帧。
//
// 原来是 1.5 秒，用户报「这一段播完了、下一段还没加载」的停顿：跨 512KB
// 块边界时要等一次 HTTP 往返，CDN 抖一下（几百 ms 到几秒）就够把缓冲吃空，
// oto 只好补静音。5 秒 × 48kHz × 2ch × 4B ≈ 1.9MB，代价可以忽略；
// seek/切歌会重建流水线，不会因此变钝。
func (p *pipeline) readAheadFrames() int {
	return p.eqRate() * 5
}

// eqRate 是输出采样率（均衡/重采样之后的采样率）。
func (p *pipeline) eqRate() int {
	if p.rs != nil {
		return p.rs.outRate
	}
	return p.info.SampleRate
}

// eqTargetBytes 是「已过均衡、等着被声卡搬走」的 s16le 字节数：约 200ms。
// 均衡开关只对这段生效，所以切换延迟 ≈ 200ms（原版 Web Audio 是实时图，
// 0 延迟；200ms 以内听感上就是「立刻」）。
func (p *pipeline) eqTargetBytes() int {
	return p.eqRate() * p.channels * 2 / 5
}

// pump 是后台生产者：把 raw 队列（未过均衡的 float32）填到水位，锁外做
// 耗时的解码与网络读。均衡由 finish 在输出前的小段上做（见 eqTargetBytes）。
func (p *pipeline) pump() {
	water := p.readAheadFrames()
	for {
		p.mu.Lock()
		pending := (len(p.raw) - p.rawAt) / p.channels
		idle := pending >= water || p.rawEOF
		p.mu.Unlock()
		if p.stopped() {
			return
		}
		if idle {
			select {
			case <-p.stop:
				return
			case <-p.wake:
			case <-time.After(20 * time.Millisecond):
			}
			continue
		}
		chunk, ended, err := p.produce()
		if err != nil {
			p.mu.Lock()
			// 没带类别的都当解码失败：网络类的错误在 remote.go 里已带
			// KindNetwork（WrapKind 会保留根因，不会被这里盖掉）。
			p.err, p.rawEOF = WrapKind(KindDecode, err), true
			p.mu.Unlock()
			p.nudgeEQ()
			return
		}
		if len(chunk) > 0 {
			p.mu.Lock()
			p.raw = append(p.raw, chunk...)
			p.mu.Unlock()
			p.nudgeEQ()
		}
		if ended {
			// 流真的到头了（不是"这块还没攒够"——那种情况 chunk 也是空的，
			// 但后面还有数据）。之后进 idle 分支等 seek 唤醒。
			p.mu.Lock()
			p.rawEOF = true
			p.mu.Unlock()
			p.nudgeEQ()
		}
	}
}

// finish 是「均衡 → s16le」这一段：从 raw 队列取一小段，过均衡后放进 ready。
// 它保持 ready 只有 eqTargetBytes 那么多，所以均衡开关只影响这 200ms。
func (p *pipeline) finish() {
	const maxFrames = 1024
	for {
		p.mu.Lock()
		pending := (len(p.raw) - p.rawAt) / p.channels
		readyBytes := len(p.ready) - p.readyAt
		if pending == 0 || readyBytes >= p.eqTargetBytes() {
			if pending == 0 && p.rawEOF {
				p.eof = true
			}
			p.mu.Unlock()
			if p.stopped() {
				return
			}
			select {
			case <-p.stop:
				return
			case <-p.eqWake:
			case <-time.After(10 * time.Millisecond):
			}
			continue
		}
		// 只取到「补满 ready 目标」为止，绝不把大缓冲一次全过完。
		wantFrames := (p.eqTargetBytes() - readyBytes) / (p.channels * 2)
		if wantFrames < 1 {
			wantFrames = 1
		}
		if wantFrames > pending {
			wantFrames = pending
		}
		if wantFrames > maxFrames {
			wantFrames = maxFrames
		}
		n := wantFrames * p.channels
		chunk := make([]float32, n)
		copy(chunk, p.raw[p.rawAt:p.rawAt+n])
		p.rawAt += n
		if p.rawAt >= len(p.raw) {
			p.raw, p.rawAt = p.raw[:0], 0
		} else if p.rawAt > 1<<19 {
			p.raw = append(p.raw[:0], p.raw[p.rawAt:]...)
			p.rawAt = 0
		}
		p.mu.Unlock()

		p.eq.Process(chunk)

		p.mu.Lock()
		p.ready = append(p.ready, appendS16(nil, chunk)...)
		p.started = true
		p.mu.Unlock()
	}
}

// nudgeEQ 把 finish 从等待里叫醒。
func (p *pipeline) nudgeEQ() {
	select {
	case p.eqWake <- struct{}{}:
	default:
	}
}

// stopped 报告 stop 是否已关闭。
func (p *pipeline) stopped() bool {
	select {
	case <-p.stop:
		return true
	default:
		return false
	}
}

// nudge 把生产者从等待里叫醒（seek 之后要立刻接着产）。
func (p *pipeline) nudge() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// produce 解码一块、变速、重采样，返回 float32 采样（**还没过均衡**）。
// 返回空切片**不代表结束** —— WSOLA 还没攒够一帧时就是空的（倍速越高、
// 流式源每次给的越少，这种情况越多）。是否到头由 ended 单独报告。
func (p *pipeline) produce() ([]float32, bool, error) {
	p.procMu.Lock()
	defer p.procMu.Unlock()
	return p.produceLocked()
}

func (p *pipeline) close() {
	// 先叫停生产者：它可能在网络读里，等它把 procMu 让出来再动解码器。
	p.once.Do(func() { close(p.stop) })
	p.procMu.Lock()
	defer p.procMu.Unlock()
	if p.dec != nil {
		_ = p.dec.Reset(nil)
	}
}

func (p *pipeline) duration() float64 {
	return p.info.Duration.Seconds()
}

// buffered 返回已预读到本地的媒体秒数：远端下载到的字节占比 × 总时长。
// B 站音频是 AAC（近似恒定码率），字节占比与时长占比基本一致，用来画
// 进度条上的缓冲段足够（原版 Web Audio 的 buffered 也是按块跳的）。
// 只读原子量与不可变字段，不碰 procMu/mu，界面线程可以直接调。
func (p *pipeline) buffered() float64 {
	if p.rf == nil {
		return 0
	}
	dur := p.info.Duration.Seconds()
	total := p.rf.Size()
	if dur <= 0 || total <= 0 {
		return 0
	}
	end := p.rf.ReadAhead()
	if end < 0 {
		end = 0
	} else if end > total {
		end = total
	}
	return dur * float64(end) / float64(total)
}

// wsNow 取当前的变速器指针（不阻塞在流水线锁上）。
func (p *pipeline) wsNow() *WSOLA {
	p.wsMu.Lock()
	ws := p.ws
	p.wsMu.Unlock()
	return ws
}

// setWS 换变速器（seek 重建时）。
func (p *pipeline) setWS(ws *WSOLA) {
	p.wsMu.Lock()
	p.ws = ws
	p.wsMu.Unlock()
}

func (p *pipeline) position() float64 {
	ws := p.wsNow()
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
// setEQ 切换均衡。
//
// **不要拿流水线的锁**：Compressor 自己就有锁（SetEnabled/Process 都走它），
// 而流水线锁会被 Read（oto 取数据的 goroutine）在网络读时长期持有 —— 在它
// 上面等，UI 线程会一起卡住（实测 toggleEQ 卡过 22 秒）。
func (p *pipeline) setEQ(on bool) {
	p.eq.SetEnabled(on)
}

// seekFrameSamples 估算每帧的源采样数，仅用于 SeekTime 不可用时的回退定位。
// 用**媒体总采样数**（presented Duration + EncoderDelay priming）除以帧数：
// 只除以 presented 会在有 edit list 时偏小，按平均帧长折算帧号就会系统性偏移。
// AAC-LC 帧长固定 1024，这里只是兜底。
func seekFrameSamples(info m4a.Info) int {
	frameSamples := 1024
	if info.FrameCount > 0 {
		mediaSamples := info.Duration.Seconds()*float64(info.SampleRate) + float64(info.EncoderDelay)
		frameSamples = int(math.Round(mediaSamples / float64(info.FrameCount)))
	}
	return frameSamples
}

// seekFrame 选 seek 目标所在的源帧下标。
//
// 优先用 go-m4a 的 SeekTime：它按 presentation time 在 stts 的**精确帧长**上
// 定位（不是平均帧长），并且感知 edit list 的 priming —— UI 的 0 秒对应媒体里
// EncoderDelay 之后的位置，所以有 priming 时目标帧会比「秒 × 采样率 / 帧长」
// 晚一帧，正好落在目标真正所在的那一帧。SeekTime 成功时它内部已经把读游标
// 移到该帧，调用方不用再 SeekSample；ok=false 表示 SeekTime 不可用（无 timing /
// edit list 形状不规整 / 时间越界），frame 是按平均帧长估算的，调用方需自行
// SeekSample 定位。
func seekFrame(rd *m4a.Reader, seconds float64, frameSamples int) (frame int, ok bool) {
	if sample, _, err := rd.SeekTime(time.Duration(seconds * float64(time.Second))); err == nil {
		return sample, true
	}
	return int(seconds * float64(rd.Info().SampleRate) / float64(frameSamples)), false
}

// seekStartSample 返回 WSOLA 的起始 SourcePosition，单位是**呈现采样**。
//
// WSOLA 的 skip 只是「输入流第 0 个采样对应的绝对位置」这个坐标系原点，改它
// 不会丢/补任何音频（produce 永远从输入的第 0 个采样开始合成），只影响
// SourcePosition 报出来的进度。SourcePosition 除以采样率就是播放进度，而原版
// （HTMLAudio 的 currentTime）报的是呈现时间，所以这里用 seconds × 采样率、
// **不加** EncoderDelay —— 加了进度条会比原版快一个 priming（AAC-LC 约 21ms），
// seek 落点也会比点击位置偏后。
func seekStartSample(seconds float64, sampleRate int) int {
	return int(seconds * float64(sampleRate))
}

func (p *pipeline) seek(seconds float64) error {
	// 拿 procMu 而不是 mu：既要与生产者互斥（它在解码），又不能挡住
	// 取数回调（它只碰 mu）。生产者最多在做一块，等它做完即可。
	p.procMu.Lock()
	defer p.procMu.Unlock()

	if seconds < 0 {
		seconds = 0
	}
	dur := p.info.Duration.Seconds()
	if dur > 0 && seconds > dur {
		seconds = dur
	}

	frameSamples := seekFrameSamples(p.info)

	if _, err := p.rf.Seek(0, io.SeekStart); err != nil {
		return err
	}
	rd, err := m4a.NewReader(p.rf)
	if err != nil {
		return WrapKind(KindUnsupported, fmt.Errorf("media: 重新解析失败: %w", err))
	}
	// 先把读游标定位到目标所在帧。优先 SeekTime（内部已 SeekSample，且按
	// presentation time 感知 priming），不可用时退回按平均帧长估算 + SeekSample
	// ——之前是逐帧 ReadFrame 跳过，跳一小时要发 ~15 万个请求，seek 卡死就卡在这。
	frame, ok := seekFrame(rd, seconds, frameSamples)
	if !ok {
		log.Printf("SeekTime 不可用（退回按平均帧长估算）")
		if frame > 0 {
			if err := rd.SeekSample(frame); err != nil {
				log.Printf("seek 定位到帧 %d 失败（退回顺序读）: %v", frame, err)
			}
		}
	}
	dec, err := aacpcm.NewDecoder(rd.RawStream(), aacpcm.WithRawStream(p.info.ASC))
	if err != nil {
		return WrapKind(KindDecode, fmt.Errorf("media: 重新打开解码器失败: %w", err))
	}

	p.rd, p.dec = rd, dec
	p.procEOF = false
	p.setWS(NewWSOLA(p.channels, p.wsNow().Speed(), seekStartSample(seconds, p.info.SampleRate)))
	if p.eq.Enabled() {
		p.eq.SetEnabled(true)
	}
	// 丢掉跳转前解码/均衡好的数据：不清的话会先把旧位置的声音放出来。
	p.mu.Lock()
	p.ready, p.readyAt = p.ready[:0], 0
	p.raw, p.rawAt = p.raw[:0], 0
	p.eof, p.done, p.err, p.rawEOF = false, false, nil, false
	p.mu.Unlock()
	p.nudge()
	p.nudgeEQ()
	return nil
}

// Read 实现 io.Reader，供 oto 拉取 s16le 字节。
//
// 用循环而不是让 fillLocked 自我递归：WSOLA 可能一次读不满一帧（倍速越高、
// 每次要的输入越多；流式源每次只给一块也很常见），递归会在这种时候一直
// 压栈。这里循环到有产出、或读到流末端为止。
// readWait 是队列空时取数回调最多等多久。到此为止就是欠载 ——
// 宁可补一小段静音让声卡继续走，也不要让设备缓冲彻底见底（那是爆音）。
const readWait = 120 * time.Millisecond

// Read 实现 io.Reader，供 oto 拉取。**只碰就绪队列，绝不做 I/O。**
func (p *pipeline) Read(b []byte) (int, error) {
	if n := p.take(b); n > 0 {
		return n, nil
	}
	deadline := time.Now().Add(readWait)
	for {
		p.mu.Lock()
		eof, err := p.eof, p.err
		p.mu.Unlock()
		if eof {
			p.mu.Lock()
			p.done = true
			p.mu.Unlock()
			if err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		if n := p.take(b); n > 0 {
			return n, nil
		}
		if !time.Now().Before(deadline) {
			break
		}
		select {
		case <-p.stop:
			return 0, io.EOF
		case <-time.After(4 * time.Millisecond):
		}
	}
	// 欠载：补静音。这样声卡的节奏不被打断，网络恢复后声音自然接上。
	for i := range b {
		b[i] = 0
	}
	p.mu.Lock()
	p.under++
	p.mu.Unlock()
	return len(b), nil
}

// take 从就绪队列搬走至多 len(b) 字节。
func (p *pipeline) take(b []byte) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := copy(b, p.ready[p.readyAt:])
	p.readyAt += n
	// 队列读完就归零；长期运行则偶尔压缩一次，避免底层数组无限增长。
	switch {
	case p.readyAt >= len(p.ready):
		p.ready, p.readyAt = p.ready[:0], 0
	case p.readyAt > 1<<18:
		p.ready = append(p.ready[:0], p.ready[p.readyAt:]...)
		p.readyAt = 0
	}
	return n
}

// Underruns 报告欠载次数（诊断用：数字大说明预读水位不够或网络太差）。
func (p *pipeline) Underruns() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.under
}

// produceLocked 解码一块、变速、均衡、重采样，产出 s16le 字节。
// 调用方必须持有 procMu。ended 表示输入已到末端。
func (p *pipeline) produceLocked() ([]float32, bool, error) {
	const want = 4096 // 每次解码的采样帧数
	if p.procEOF {
		return nil, true, nil
	}
	if len(p.src) < want*p.channels*2 {
		p.src = make([]byte, want*p.channels*2)
	}
	n, err := io.ReadFull(p.dec, p.src)
	if n == 0 && err != nil {
		// 输入结束：让 WSOLA 把尾巴吐出来（只做一次）。
		p.procEOF = true
		p.flt = nil
		tail := p.wsNow().Flush()
		if p.rs != nil {
			tail = p.rs.Push(tail)
		}
		return tail, true, nil
	}
	frames := n / (p.channels * 2)
	if frames == 0 {
		return nil, false, nil
	}

	if cap(p.flt) < frames*p.channels {
		p.flt = make([]float32, frames*p.channels)
	}
	p.flt = p.flt[:frames*p.channels]
	for i := 0; i < frames*p.channels; i++ {
		p.flt[i] = float32(int16(uint16(p.src[2*i])|uint16(p.src[2*i+1])<<8)) / 32768
	}

	scaled := p.wsNow().Push(p.flt)
	if p.rs != nil {
		scaled = p.rs.Push(scaled)
	}
	return scaled, false, nil
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
