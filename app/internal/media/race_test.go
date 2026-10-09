package media

import (
	"sync"
	"testing"
)

// TestWSOLAConcurrentSpeedAndPosition 断言变速器的控制方法与产出方法可以并发
// 调用（oto 取数据与进度上报是两条 goroutine）。
func TestWSOLAConcurrentSpeedAndPosition(t *testing.T) {
	w := NewWSOLA(2, 1)
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			w.SetSpeed(2.0)
			_ = w.Speed()
			_ = w.SourcePosition()
		}
	}()

	pcm := make([]float32, 4096)
	for i := 0; i < 200; i++ {
		w.Push(pcm)
		_ = w.SourcePosition()
	}
	w.Flush()
	close(stop)
	wg.Wait()
}

// TestCompressorConcurrentToggle 断言压缩器开关与处理可以并发调用。
func TestCompressorConcurrentToggle(t *testing.T) {
	c := NewCompressor(2, 44100)
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			c.SetEnabled(true)
			c.SetEnabled(false)
			_ = c.Enabled()
		}
	}()

	buf := make([]float32, 4096)
	for i := 0; i < 500; i++ {
		for j := range buf {
			buf[j] = 0.3
		}
		c.Process(buf)
	}
	close(stop)
	wg.Wait()
}
