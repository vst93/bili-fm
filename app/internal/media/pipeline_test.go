package media

import (
	"io"
	"testing"
	"time"
)

// newTestPipeline 造一个不接解码器的流水线，只用来测就绪队列的行为。
func newTestPipeline(rate int) *pipeline {
	p := &pipeline{channels: 2, stop: make(chan struct{}), wake: make(chan struct{}, 1)}
	p.info.SampleRate = rate
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
