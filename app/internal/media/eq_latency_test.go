package media

import (
	"math"
	"testing"
	"time"
)

// eqTestPipeline 起一条只跑「均衡 → s16le」那一段的流水线（pump 手工喂 raw）。
func eqTestPipeline(t *testing.T) *pipeline {
	t.Helper()
	p := &pipeline{
		channels: 2,
		stop:     make(chan struct{}),
		eqWake:   make(chan struct{}, 1),
	}
	p.info.SampleRate = 48000
	p.eq = NewCompressor(2, 48000)
	// WSOLA 现在也在 finish 段（贴近声卡）推进，测试流水线要补上它；
	// 否则 finish 会因为 ws 为 nil 一直不产出。
	p.ws = NewWSOLA(2, 1, 0)
	go p.finish()
	t.Cleanup(func() { close(p.stop) })
	return p
}

// feedSine 往 raw 队列里塞 frames 帧的立体声正弦。
func feedSine(p *pipeline, frames int, amp float64) {
	buf := make([]float32, frames*p.channels)
	for i := 0; i < frames; i++ {
		v := float32(amp * math.Sin(2*math.Pi*440*float64(i)/48000))
		buf[2*i], buf[2*i+1] = v, v
	}
	p.mu.Lock()
	p.raw = append(p.raw, buf...)
	p.mu.Unlock()
	p.nudgeEQ()
}

// waitReady 等到 ready 队列攒到 at least 字节（或超时）。
func waitReady(t *testing.T, p *pipeline, atLeast int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		n := len(p.ready) - p.readyAt
		p.mu.Unlock()
		if n >= atLeast {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("ready 一直没攒到 %d 字节", atLeast)
}

// readyPeak 返回 ready 队列后半段的峰值（跳过起音瞬态）。
func readyPeak(p *pipeline) float64 {
	p.mu.Lock()
	data := append([]byte(nil), p.ready[p.readyAt:]...)
	p.mu.Unlock()
	peak := 0.0
	for i := len(data) / 2; i+1 < len(data); i += 2 {
		v := math.Abs(float64(int16(uint16(data[i]) | uint16(data[i+1])<<8)))
		if v > peak {
			peak = v
		}
	}
	return peak / 32767
}

// TestPipelineEQSwitchLatency 钉住「切均衡立刻生效」的机制：
//
// 均衡只作用在紧挨声卡的那一小段（eqTargetBytes ≈ 200ms）上，所以 ready 队列
// 永远不超过它 + 一个处理块；开关切换最多等这一小段播完。此前的实现把均衡
// 放在 5 秒大缓冲的生产端，切换要等 5 秒才听见（用户报的「有几秒延迟」）。
func TestPipelineEQSwitchLatency(t *testing.T) {
	p := eqTestPipeline(t)

	// 1) 关闭状态下喂 1 秒响信号：ready 只应攒到目标附近，不会把整秒都过完。
	feedSine(p, 48000, 0.9)
	waitReady(t, p, p.eqTargetBytes())
	time.Sleep(30 * time.Millisecond) // 给 finish 多余的时间把 raw 全过完（如果实现错了）
	p.mu.Lock()
	readyBytes := len(p.ready) - p.readyAt
	rawFrames := (len(p.raw) - p.rawAt) / p.channels
	p.mu.Unlock()
	maxReady := p.eqTargetBytes() + 1024*p.channels*2
	if readyBytes > maxReady {
		t.Fatalf("ready 攒了 %d 字节，超过目标+一块（%d）：均衡不再作用于小窗口，切换会变慢", readyBytes, maxReady)
	}
	if rawFrames < 30000 {
		t.Fatalf("raw 大缓冲应留在未均衡队列里（剩 %d 帧），否则切换要等它播完", rawFrames)
	}
	offPeak := readyPeak(p)

	// 2) 打开均衡，清掉已经均衡好的那段，再产出同一段信号：应当被压下来。
	p.eq.SetEnabled(true)
	p.mu.Lock()
	p.ready, p.readyAt = p.ready[:0], 0
	p.mu.Unlock()
	feedSine(p, 12000, 0.9) // 250ms
	waitReady(t, p, p.eqTargetBytes())
	onPeak := readyPeak(p)

	t.Logf("切换前后 ready 峰值：关 %.3f → 开 %.3f", offPeak, onPeak)
	if onPeak >= offPeak*0.95 {
		t.Errorf("打开均衡后新产出的音频没有被压缩：关 %v 开 %v", offPeak, onPeak)
	}
}
