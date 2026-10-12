package media

import (
	"io"
	"math"
	"testing"
	"time"
)

// TestRemoteFileReadAhead 钉住「已下载到哪」这个读数：它是进度条缓冲段的
// 唯一信号源，取的是原子量，界面线程不会因为一次网络读被挡住。
func TestRemoteFileReadAhead(t *testing.T) {
	f := &remoteFile{size: 1000}
	if got := f.ReadAhead(); got != 0 {
		t.Fatalf("初始应为 0，得到 %d", got)
	}
	// 模拟 fillLocked 抓到一块：读数应延伸到块尾。
	f.buf, f.start = make([]byte, 300), 100
	f.readHigh.Store(f.start + int64(len(f.buf)))
	if got := f.ReadAhead(); got != 400 {
		t.Fatalf("块尾应为 400，得到 %d", got)
	}
}

// TestRemoteFileReadAheadSeek 是缓冲段的正确性关键：跳到现存块之外时旧数据
// 已被丢弃，读数必须回退（否则缓冲条会显示一段其实已经没有的数据）；
// 落在现存块内则保持不变。
func TestRemoteFileReadAheadSeek(t *testing.T) {
	f := &remoteFile{size: 1000, buf: make([]byte, 300), start: 100}
	f.readHigh.Store(400)

	// 向前跳出块外。
	if _, err := f.Seek(700, io.SeekStart); err != nil {
		t.Fatalf("Seek 出错: %v", err)
	}
	if got := f.ReadAhead(); got != 700 {
		t.Fatalf("跳出块外应回退到 700，得到 %d", got)
	}

	// 落回现存块内：那块数据还在，读数保持。
	f.readHigh.Store(400)
	if _, err := f.Seek(150, io.SeekStart); err != nil {
		t.Fatalf("Seek 出错: %v", err)
	}
	if got := f.ReadAhead(); got != 400 {
		t.Fatalf("块内定位不应改动读数（400），得到 %d", got)
	}

	// 退到块之前：旧数据已失效。
	if _, err := f.Seek(10, io.SeekStart); err != nil {
		t.Fatalf("Seek 出错: %v", err)
	}
	if got := f.ReadAhead(); got != 10 {
		t.Fatalf("退到块之前应回到 10，得到 %d", got)
	}
}

// TestPipelineBuffered 钉住字节占比 → 媒体秒数的换算与夹取。
func TestPipelineBuffered(t *testing.T) {
	f := &remoteFile{size: 1000}
	p := &pipeline{rf: f}
	p.info.Duration = 40 * time.Second

	f.readHigh.Store(250)
	if got := p.buffered(); math.Abs(got-10) > 1e-9 {
		t.Fatalf("25%% 缓冲应为 10 秒，得到 %v", got)
	}
	// 读数超过总量时夹到时长。
	f.readHigh.Store(1500)
	if got := p.buffered(); math.Abs(got-40) > 1e-9 {
		t.Fatalf("超量应夹到 40 秒，得到 %v", got)
	}
	// 负读数（异常）当 0。
	f.readHigh.Store(-5)
	if got := p.buffered(); got != 0 {
		t.Fatalf("负读数应为 0，得到 %v", got)
	}
}

// TestPipelineBufferedNoSource 没有远端源（测试流水线）时不 panic、返回 0。
func TestPipelineBufferedNoSource(t *testing.T) {
	p := &pipeline{}
	p.info.Duration = 40 * time.Second
	if got := p.buffered(); got != 0 {
		t.Fatalf("无源应返回 0，得到 %v", got)
	}
}
