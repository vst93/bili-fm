package media

import (
	"bytes"
	"io"
	"testing"
	"time"

	m4a "github.com/tphakala/go-m4a"
)

// newTestPipeline 造一个不接解码器的流水线，只用来测就绪队列的行为。
func newTestPipeline(rate int) *pipeline {
	p := &pipeline{channels: 2, stop: make(chan struct{}), wake: make(chan struct{}, 1)}
	p.info.SampleRate = rate
	// 与 newPipeline 一致：起播时队列是空的，先处于等待状态。
	p.waiting.Store(true)
	return p
}

// TestPipelineReadFromQueue 就绪队列里有数据时，Read 立刻返回、不去碰网络。
func TestPipelineReadFromQueue(t *testing.T) {
	p := newTestPipeline(48000)
	p.ready = []byte{1, 2, 3, 4, 5, 6}
	buf := make([]byte, 4)
	n, err := p.Read(buf)
	if err != nil || n != 4 {
		t.Fatalf("应从队列取 4 字节，得到 n=%d err=%v", n, err)
	}
	if buf[0] != 1 || buf[3] != 4 {
		t.Fatalf("取出的字节不对：%v", buf)
	}
	// 再取剩下的 2 字节。
	n, err = p.Read(buf)
	if err != nil || n != 2 {
		t.Fatalf("应取到剩余 2 字节，得到 n=%d err=%v", n, err)
	}
	// 队列应已归零（读完即清）。
	if len(p.ready) != 0 || p.readyAt != 0 {
		t.Fatalf("队列读完应归零，得到 len=%d at=%d", len(p.ready), p.readyAt)
	}
}

// TestPipelineReadUnderrun 是「不能让声卡等网络」的核心保证：
// 队列空、生产者又没跟上时，Read 必须在有限时间内返回（补静音），
// 而不是无限等下去 —— 无限等就是爆音/卡顿的根源。
func TestPipelineReadUnderrun(t *testing.T) {
	p := newTestPipeline(48000)
	buf := make([]byte, 8192)
	start := time.Now()
	n, err := p.Read(buf)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("欠载不该报错（补静音继续），得到 %v", err)
	}
	if n != len(buf) {
		t.Fatalf("欠载应补满整块静音，得到 %d", n)
	}
	if elapsed > 2*readWait {
		t.Fatalf("等太久了：%v（应该到 readWait=%v 就返回）", elapsed, readWait)
	}
	for i, v := range buf {
		if v != 0 {
			t.Fatalf("第 %d 字节应为静音，得到 %d", i, v)
		}
	}
	if got := p.Underruns(); got != 1 {
		t.Fatalf("欠载计数应为 1，得到 %d", got)
	}
}

// TestPipelineReadEOFRead 流到头时 Read 报 io.EOF（让 oto 收尾），
// 且在有错误时优先报错误。
func TestPipelineReadEOFRead(t *testing.T) {
	p := newTestPipeline(48000)
	p.eof = true
	if _, err := p.Read(make([]byte, 16)); err != io.EOF {
		t.Fatalf("到头应返回 io.EOF，得到 %v", err)
	}
	p2 := newTestPipeline(48000)
	p2.eof, p2.err = true, io.ErrUnexpectedEOF
	if _, err := p2.Read(make([]byte, 16)); err != io.ErrUnexpectedEOF {
		t.Fatalf("应优先报错误，得到 %v", err)
	}
}

// TestPipelineTakeCompacts 长期运行时队列要压缩，底层数组不能无限增长。
func TestPipelineTakeCompacts(t *testing.T) {
	p := newTestPipeline(48000)
	p.ready = make([]byte, 1<<19)
	p.readyAt = 1<<18 + 1 // 超过压缩阈值
	p.take(make([]byte, 8))
	if p.readyAt != 0 {
		t.Fatalf("超过阈值应压缩队列，得到 readyAt=%d", p.readyAt)
	}
	if cap(p.ready) > 1<<19 {
		t.Fatalf("压缩后容量应回落，得到 cap=%d", cap(p.ready))
	}
}

// TestPipelineEnded 钉住「自然播完」的判据：Read 已经收到 EOF 后 done=true，
// 主窗据此在用户再点播放时走 Replay（而不是对已 EOF 的 oto player 调 Play）。
func TestPipelineEnded(t *testing.T) {
	p := newTestPipeline(48000)
	if p.ended() {
		t.Fatal("新流水线不应是已结束")
	}
	p.mu.Lock()
	p.done = true
	p.mu.Unlock()
	if !p.ended() {
		t.Fatal("done=true 后 ended 应为真")
	}
}

// TestPipelineCloseReleasesBuffers 钉住切歌时主动释放大缓冲：关闭后 raw /
// ready 必须清空，不能等整个 pipeline 被 GC 才还（Windows 私有工作集里
// 这些页会一直算进内存）。
func TestPipelineCloseReleasesBuffers(t *testing.T) {
	p := newTestPipeline(48000)
	p.raw = make([]float32, 1<<16)
	p.ready = make([]byte, 1<<16)
	p.src = make([]byte, 1<<16)
	p.close()
	if p.raw != nil || p.ready != nil || p.src != nil {
		t.Fatalf("close 后仍持有缓冲：raw=%d ready=%d src=%d", len(p.raw), len(p.ready), len(p.src))
	}
}

// TestPipelineWaitingSignal 钉住 issue #42 的运行中等待信号：
//   - 队列空、Read 等到 readWait 仍没有数据 → waiting=true（欠载）；
//   - 下一块真正的 PCM 被取走 → waiting=false。
//
// 这里**不按样本内容判断**：全零的 PCM 也是数据（见下一条）。
func TestPipelineWaitingSignal(t *testing.T) {
	p := newTestPipeline(48000)
	if !p.waitingForData() {
		t.Fatal("新流水线在拿到第一块 PCM 之前应处于等待")
	}
	if n, err := p.Read(make([]byte, 16)); err != nil || n != 16 {
		t.Fatalf("欠载应补静音并返回整块，得到 n=%d err=%v", n, err)
	}
	if !p.waitingForData() {
		t.Fatal("Read 等到 readWait 仍没有数据时应标记 waiting")
	}
	p.ready = []byte{1, 2, 3, 4}
	if n, err := p.Read(make([]byte, 4)); err != nil || n != 4 {
		t.Fatalf("有数据后应直接取走，得到 n=%d err=%v", n, err)
	}
	if p.waitingForData() {
		t.Fatal("取到真正的 PCM 后 waiting 应立刻清除")
	}
}

// TestPipelineZeroPCMIsData 是 issue #42 的判据反证：安静段落（全零样本）
// 是有效音频，不能因为「样本都是 0」就继续显示缓冲。
func TestPipelineZeroPCMIsData(t *testing.T) {
	p := newTestPipeline(48000)
	p.ready = make([]byte, 8)
	if n, err := p.Read(make([]byte, 8)); err != nil || n != 8 {
		t.Fatalf("全零 PCM 应正常取走，得到 n=%d err=%v", n, err)
	}
	if p.waitingForData() {
		t.Fatal("全零 PCM 也是数据：取走后不应再显示等待")
	}
}

// TestPipelineEOFErrorClosedNotWaiting 钉住「播完 / 出错 / 关闭都不是缓冲」。
// waitingForData 是只读谓词：观察 terminal 不会回写 waiting，terminal 复位后
// 残留的 waiting 立刻重新可见（否则并发 seek 的新信号会被读掉）。
func TestPipelineEOFErrorClosedNotWaiting(t *testing.T) {
	t.Run("EOF", func(t *testing.T) {
		p := newTestPipeline(48000)
		p.eof = true
		if _, err := p.Read(make([]byte, 16)); err != io.EOF {
			t.Fatalf("应返回 io.EOF，得到 %v", err)
		}
		if p.waitingForData() {
			t.Fatal("EOF 之后不应再显示缓冲")
		}
		// 即使 waiting 被别的路径重新置真，terminal 也要把它拦住 —— 但只读
		// 观察，不能把这个位清掉。
		p.waiting.Store(true)
		if p.waitingForData() {
			t.Fatal("EOF 的 terminal 闸门没拦住残留 waiting")
		}
		if !p.waiting.Load() {
			t.Fatal("读 waitingForData 清掉了 waiting 位（并发 seek 的新信号会丢）")
		}
		p.terminal.Store(false)
		if !p.waitingForData() {
			t.Fatal("terminal 复位后残留 waiting 应重新可见")
		}
	})
	t.Run("error", func(t *testing.T) {
		p := newTestPipeline(48000)
		p.eof, p.err = true, io.ErrUnexpectedEOF
		if _, err := p.Read(make([]byte, 16)); err != io.ErrUnexpectedEOF {
			t.Fatalf("应返回错误，得到 %v", err)
		}
		if p.waitingForData() {
			t.Fatal("出错之后不应再显示缓冲")
		}
		p.waiting.Store(true)
		if p.waitingForData() {
			t.Fatal("出错的 terminal 闸门没拦住残留 waiting")
		}
		if !p.waiting.Load() {
			t.Fatal("读 waitingForData 清掉了 waiting 位")
		}
	})
	t.Run("stopped", func(t *testing.T) {
		p := newTestPipeline(48000)
		close(p.stop) // 只停生产者，不经过 close()，单独钉 stopped 闸门
		p.waiting.Store(true)
		if p.waitingForData() {
			t.Fatal("stop 已关闭时等待位应为 false")
		}
		if !p.waiting.Load() {
			t.Fatal("读 waitingForData 清掉了 waiting 位")
		}
	})
	t.Run("closed", func(t *testing.T) {
		p := newTestPipeline(48000)
		p.close()
		if p.waitingForData() {
			t.Fatal("关闭后不应再显示缓冲")
		}
		p.waiting.Store(true)
		if p.waitingForData() {
			t.Fatal("关闭后的 stopped 闸门没拦住残留 waiting")
		}
		if !p.waiting.Load() {
			t.Fatal("读 waitingForData 清掉了 waiting 位")
		}
	})
}

// TestPipelineZeroLengthReadNotWaiting 钉住零长度 Read：直接返回，不白等
// 一个 readWait，也不把状态改成「在等数据」。
func TestPipelineZeroLengthReadNotWaiting(t *testing.T) {
	p := newTestPipeline(48000)
	p.waiting.Store(false)
	start := time.Now()
	n, err := p.Read(nil)
	if n != 0 || err != nil {
		t.Fatalf("零长度 Read 应返回 0,nil，得到 n=%d err=%v", n, err)
	}
	if time.Since(start) > readWait/2 {
		t.Fatalf("零长度 Read 不该等 readWait：%v", time.Since(start))
	}
	if p.waitingForData() {
		t.Fatal("零长度 Read 不是缓冲")
	}
}

// fakeSeekDemux 是 seek 测试用的最小解封装器：只提供空 RawStream 与可成功的
// SeekTime，让 pipeline.seek 走完重建路径（不碰网络）。
type fakeSeekDemux struct{ info m4a.Info }

func (f fakeSeekDemux) Info() m4a.Info       { return f.info }
func (f fakeSeekDemux) RawStream() io.Reader { return bytes.NewReader(nil) }
func (f fakeSeekDemux) SeekTime(time.Duration) (int, int64, error) {
	return 0, 0, nil
}
func (f fakeSeekDemux) SeekSample(int) error { return nil }

// TestPipelineSeekMarksWaiting 钉住 seek 后的等待：seek 会清空就绪队列，
// 在下一块 PCM 被取走之前 waiting=true；取到数据后立刻清除。
func TestPipelineSeekMarksWaiting(t *testing.T) {
	info := m4a.Info{SampleRate: 48000, Channels: 1, ASC: []byte{0x11, 0x88}, Duration: 10 * time.Second}
	p := &pipeline{
		channels:   1,
		info:       info,
		rd:         fakeSeekDemux{info: info},
		fragmented: true,
		ws:         NewWSOLA(1, 1, 0),
		eq:         NewCompressor(1, 48000),
		stop:       make(chan struct{}),
		wake:       make(chan struct{}, 1),
		eqWake:     make(chan struct{}, 1),
	}
	if err := p.seek(1); err != nil {
		t.Fatalf("seek 失败：%v", err)
	}
	if !p.waitingForData() {
		t.Fatal("seek 清空队列后应标记等待")
	}
	p.ready = []byte{1, 2}
	if n, err := p.Read(make([]byte, 2)); err != nil || n != 2 {
		t.Fatalf("seek 后取数据失败：n=%d err=%v", n, err)
	}
	if p.waitingForData() {
		t.Fatal("seek 后取到 PCM 应清除等待")
	}
}

// TestPipelineWaitingReadTakesNoIOLocks 钉住界面线程的读路径：拿住解码锁
// （procMu）与输出锁（outMu）时，等待位仍能立刻读到。违反这条就是 UI 卡死
// （PLAN 的锁模型：不许握着流水线锁等网络 I/O）。
func TestPipelineWaitingReadTakesNoIOLocks(t *testing.T) {
	p := newTestPipeline(48000)
	p.procMu.Lock()
	p.outMu.Lock()
	done := make(chan bool, 1)
	go func() { done <- p.waitingForData() }()
	select {
	case got := <-done:
		if !got {
			t.Fatal("等待位应保持 true")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("读等待位被 procMu/outMu 阻塞 —— 界面线程会卡在网络读上")
	}
	p.outMu.Unlock()
	p.procMu.Unlock()
}

// TestBufferingNow 钉住 Player.Buffering 的两道闸门：oto 必须在播，且当前
// 流水线必须在等数据。暂停（playing=false）与没有流水线都不忙。
func TestBufferingNow(t *testing.T) {
	p := newTestPipeline(48000)
	if !bufferingNow(true, p) {
		t.Fatal("在播 + 流水线等待 = 忙")
	}
	if bufferingNow(false, p) {
		t.Fatal("暂停不是缓冲")
	}
	if bufferingNow(true, nil) {
		t.Fatal("没有流水线不是缓冲")
	}
	p.waiting.Store(false)
	if bufferingNow(true, p) {
		t.Fatal("流水线已拿到数据就不再忙")
	}
}
