package media

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	m4a "github.com/tphakala/go-m4a"
)

// ascMono48k 是 AAC-LC 48 kHz mono 的 2-byte AudioSpecificConfig。
var ascMono48k = []byte{0x11, 0x88}

// writeTestM4A 用 go-m4a Writer 构造一个 n 帧的 AAC-LC M4A 文件并返回其字节。
// encoderDelay 传 m4a.NoEdit 表示不写 edit list。
func writeTestM4A(t *testing.T, n int, encoderDelay int) []byte {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "seektest*.m4a")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())

	cfg := m4a.WriterConfig{
		SampleRate:   48000,
		Channels:     1,
		ASC:          ascMono48k,
		EncoderDelay: encoderDelay,
	}
	w, err := m4a.NewWriter(f, cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	au := make([]byte, 32) // dummy AAC access unit，容器层不解析内容
	for i := 0; i < n; i++ {
		if err := w.WriteFrame(au); err != nil {
			t.Fatalf("WriteFrame %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	f.Close()
	return data
}

// mustReader 从字节构造 m4a.Reader，失败即 t.Fatal。
func mustReader(t *testing.T, data []byte) *m4a.Reader {
	t.Helper()
	rd, err := m4a.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	return rd
}

// TestSeekFrameSamples 钉住回退用的帧长估算：无信息时默认 1024，有信息时取
// 媒体总采样数（presented + priming）的平均值。
func TestSeekFrameSamples(t *testing.T) {
	info := m4a.Info{SampleRate: 48000, Channels: 1}
	if got := seekFrameSamples(info); got != 1024 {
		t.Errorf("无 Duration/FrameCount 时应默认 1024，得到 %d", got)
	}

	info.Duration = 10 * time.Second
	info.FrameCount = 100 // 48000*10/100 = 4800
	if got := seekFrameSamples(info); got != 4800 {
		t.Errorf("无 priming 时应取平均 4800，得到 %d", got)
	}

	// 加上 priming：媒体总采样数 = presented + EncoderDelay，帧长随之变大。
	info.EncoderDelay = 1024 // (480000+1024)/100 = 4810.24 → 4810
	if got := seekFrameSamples(info); got != 4810 {
		t.Errorf("含 priming 时应取平均 4810，得到 %d", got)
	}
}

// TestSeekFrameWithEditList 验证有 edit list 时目标帧按**媒体**时间选，
// 即把 EncoderDelay priming 算进去 —— 旧的手算逻辑会选到早一帧。
//
// 300 帧 × 1024 = 307200 采样 = 6.4 s @ 48 kHz。
// edit list: media_time=1024, presented=306176。
// SeekTime(5s) → 媒体 1024 + 240000 = 241024 = 235×1024 + 384 → 帧 235。
func TestSeekFrameWithEditList(t *testing.T) {
	data := writeTestM4A(t, 300, m4a.DefaultEncoderDelay)
	rd := mustReader(t, data)
	info := rd.Info()

	if info.FrameCount != 300 {
		t.Fatalf("FrameCount = %d，应为 300", info.FrameCount)
	}

	const seconds = 5.0
	fs := seekFrameSamples(info)
	frame, ok := seekFrame(rd, seconds, fs)
	if !ok {
		t.Fatal("有 edit list 的文件 SeekTime 应成功")
	}
	if frame != 235 {
		t.Errorf("帧 = %d，应为 235（含 %d priming）", frame, m4a.DefaultEncoderDelay)
	}
	// 目标必须落在选中帧覆盖的媒体区间内。
	target := int64(seconds*48000) + info.EncoderDelay
	if !(int64(frame)*1024 <= target && target < int64(frame+1)*1024) {
		t.Errorf("媒体目标 %d 不在帧 %d 的 [%d,%d) 内", target, frame, int64(frame)*1024, int64(frame+1)*1024)
	}
	// 旧手算逻辑（presented / 帧长）会选到 234，比正确帧早一帧。
	if naive := int(seconds*48000) / 1024; naive != 234 {
		t.Fatalf("测试前提变了：naive 帧应为 234，得到 %d", naive)
	}
}

// TestSeekFrameNoEditList 验证无 edit list 时目标帧同样落在目标所在帧。
//
// 300 帧 × 1024 = 307200 采样 = 6.4 s。SeekTime(5s) → 240000 = 234×1024+384。
func TestSeekFrameNoEditList(t *testing.T) {
	data := writeTestM4A(t, 300, m4a.NoEdit)
	rd := mustReader(t, data)

	const seconds = 5.0
	frame, ok := seekFrame(rd, seconds, seekFrameSamples(rd.Info()))
	if !ok {
		t.Fatal("无 edit list 但有 stts 的文件 SeekTime 应成功")
	}
	if frame != 234 {
		t.Errorf("帧 = %d，应为 234", frame)
	}
	target := int64(seconds * 48000)
	if !(int64(frame)*1024 <= target && target < int64(frame+1)*1024) {
		t.Errorf("目标 %d 不在帧 %d 的 [%d,%d) 内", target, frame, int64(frame)*1024, int64(frame+1)*1024)
	}
}

// TestSeekFrameOutOfRange 验证超出时长时 SeekTime 失败，回退按平均帧长估算。
func TestSeekFrameOutOfRange(t *testing.T) {
	data := writeTestM4A(t, 300, m4a.NoEdit) // 6.4 s
	rd := mustReader(t, data)

	frame, ok := seekFrame(rd, 10.0, 1024)
	if ok {
		t.Fatal("超出时长时 SeekTime 应失败")
	}
	if want := int(10.0*48000) / 1024; frame != want { // 468
		t.Errorf("回退帧 = %d，应为 %d", frame, want)
	}
}

// TestSeekStartSampleIsPresentation 钉住 WSOLA 起始位置是**呈现采样**：
// 直接 seconds × 采样率，不加 EncoderDelay。加了进度条会比原版 currentTime
// 快一个 priming（AAC-LC 约 21ms），seek 落点也会偏后。
func TestSeekStartSampleIsPresentation(t *testing.T) {
	if got := seekStartSample(5, 48000); got != 240000 {
		t.Errorf("seekStartSample(5s, 48k) = %d，应为 240000（不含 priming）", got)
	}
	if got := seekStartSample(0, 48000); got != 0 {
		t.Errorf("seekStartSample(0) = %d，应为 0", got)
	}
	if got := seekStartSample(1.5, 44100); got != 66150 {
		t.Errorf("seekStartSample(1.5s, 44.1k) = %d，应为 66150", got)
	}
}

// TestSeekTimeMatchesSequentialRead 端到端验证：SeekTime 定位后 ReadFrame
// 返回的帧与顺序读第 sample 帧一致。
func TestSeekTimeMatchesSequentialRead(t *testing.T) {
	data := writeTestM4A(t, 50, m4a.NoEdit)

	// 顺序读，收集所有帧。
	seq := mustReader(t, data)
	var frames [][]byte
	for {
		au, err := seq.ReadFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadFrame: %v", err)
		}
		frames = append(frames, au)
	}

	// SeekTime 定位到 1s（48000 采样 = 46.875 帧 → sample 46）。
	rd := mustReader(t, data)
	const seconds = 1.0
	sample, skip, err := rd.SeekTime(time.Duration(seconds * float64(time.Second)))
	if err != nil {
		t.Fatalf("SeekTime: %v", err)
	}
	if sample != 46 {
		t.Errorf("SeekTime(1s) sample = %d，应为 46", sample)
	}
	if skip != int64(48000-46*1024) { // 48000 - 47104 = 896
		t.Errorf("SeekTime(1s) skip = %d，应为 896", skip)
	}

	// 读出的帧应与顺序读的第 sample 帧一致。
	au, err := rd.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame after seek: %v", err)
	}
	if !bytes.Equal(au, frames[sample]) {
		t.Error("seek 后读出的帧与顺序读不一致")
	}
}
