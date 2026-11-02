package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"

	m4a "github.com/tphakala/go-m4a"
)

// makeSIDX 造一个只含媒体 reference（reference_type=0）的 sidx，供解析测试用。
func makeSIDX(timescale uint32, refs [][2]uint32) []byte {
	body := make([]byte, 0, 24+12*len(refs))
	body = append(body, 0, 0, 0, 0) // version + flags
	body = binary.BigEndian.AppendUint32(body, 1)
	body = binary.BigEndian.AppendUint32(body, timescale)
	body = binary.BigEndian.AppendUint32(body, 0) // earliest_presentation_time
	body = binary.BigEndian.AppendUint32(body, 0) // first_offset
	body = append(body, 0, 0)                     // reserved
	body = binary.BigEndian.AppendUint16(body, uint16(len(refs)))
	for _, ref := range refs {
		body = binary.BigEndian.AppendUint32(body, ref[0]&0x7fffffff)
		body = binary.BigEndian.AppendUint32(body, ref[1])
		body = binary.BigEndian.AppendUint32(body, 0x90000000) // starts_with_SAP
	}
	out := make([]byte, 8)
	binary.BigEndian.PutUint32(out, uint32(8+len(body)))
	out[4], out[5], out[6], out[7] = 's', 'i', 'd', 'x'
	return append(out, body...)
}

func TestParseSIDX(t *testing.T) {
	box := makeSIDX(48000, [][2]uint32{{100, 48000}, {250, 96000}})
	body := box[8:]
	segs, ts, total, err := parseSIDX(body, 1000, 10000)
	if err != nil {
		t.Fatalf("parseSIDX: %v", err)
	}
	if ts != 48000 {
		t.Errorf("timescale = %d，want 48000", ts)
	}
	if total != 144000 {
		t.Errorf("total = %d，want 144000", total)
	}
	if len(segs) != 2 {
		t.Fatalf("分片数 = %d，want 2", len(segs))
	}
	if segs[0].off != 1000 || segs[0].size != 100 || segs[0].start != 0 {
		t.Errorf("分片 0 = %+v", segs[0])
	}
	if segs[1].off != 1100 || segs[1].size != 250 || segs[1].start != 48000 {
		t.Errorf("分片 1 = %+v", segs[1])
	}
}

func TestParseSIDXRejectsBroken(t *testing.T) {
	if _, _, _, err := parseSIDX([]byte{0, 0, 0, 0}, 0, 100); err == nil {
		t.Error("截断的 sidx 应报错")
	}
	box := makeSIDX(48000, [][2]uint32{{100, 100}})
	// 把 reference_type 改成 1（层级索引）
	box[len(box)-12] |= 0x80
	if _, _, _, err := parseSIDX(box[8:], 0, 10000); err == nil {
		t.Error("层级 sidx 应报错")
	}
}

// 构造一个最小的 moof：traf/tfhd + trun（带 data_offset / 逐样本 size）。
func makeMoof(sampleSizes []uint32, dataOffset uint32) []byte {
	var trun []byte
	trun = append(trun, 0, 0, 0x02, 0x01) // version + data-offset + sample-size
	trun = binary.BigEndian.AppendUint32(trun, uint32(len(sampleSizes)))
	trun = binary.BigEndian.AppendUint32(trun, dataOffset)
	for _, size := range sampleSizes {
		trun = binary.BigEndian.AppendUint32(trun, size)
	}
	trunBox := append(boxHeaderBytes("trun", len(trun)), trun...)
	var tfhd []byte
	// flags 0x2a = default-sample-flags + default-sample-duration +
	// sample-description-index；三者都在下面按顺序写入。
	tfhd = append(tfhd, 0, 0x00, 0x00, 0x2a)
	tfhd = binary.BigEndian.AppendUint32(tfhd, 1)    // track_ID
	tfhd = binary.BigEndian.AppendUint32(tfhd, 1)    // sample_description_index
	tfhd = binary.BigEndian.AppendUint32(tfhd, 1024) // default_sample_duration
	tfhd = binary.BigEndian.AppendUint32(tfhd, 0)    // default_sample_flags
	tfhdBox := append(boxHeaderBytes("tfhd", len(tfhd)), tfhd...)
	traf := append(append(boxHeaderBytes("traf", len(tfhdBox)+len(trunBox)), tfhdBox...), trunBox...)
	return append(boxHeaderBytes("moof", len(traf)), traf...)
}

func boxHeaderBytes(typ string, bodyLen int) []byte {
	var b [8]byte
	binary.BigEndian.PutUint32(b[:4], uint32(bodyLen+8))
	copy(b[4:], typ)
	return b[:]
}

func TestParseMoofSamples(t *testing.T) {
	sizes := []uint32{3, 4, 5}
	moof := makeMoof(sizes, 0x20)
	seg := append(append([]byte(nil), moof...), make([]byte, 0x40)...)
	samples, err := parseMoofSamples(seg, 100, map[uint32]fmp4Trex{})
	if err != nil {
		t.Fatalf("parseMoofSamples: %v", err)
	}
	if len(samples) != len(sizes) {
		t.Fatalf("样本数 = %d，want %d", len(samples), len(sizes))
	}
	var wantOff int32 = 0x20
	for i, sm := range samples {
		if sm.off != wantOff || sm.size != sizes[i] || sm.dur != 1024 {
			t.Errorf("样本 %d = %+v，want off=%d size=%d dur=1024", i, sm, wantOff, sizes[i])
		}
		wantOff += int32(sizes[i])
	}
}

func TestFindMoofSkipsStyp(t *testing.T) {
	moof := makeMoof([]uint32{2}, 0x20)
	seg := append(boxHeaderBytes("styp", 0), moof...)
	off, size, err := findMoof(seg)
	if err != nil {
		t.Fatalf("findMoof: %v", err)
	}
	if off != 8 || size != int64(len(moof)) {
		t.Errorf("moof off=%d size=%d，want 8/%d", off, size, len(moof))
	}
}

// 用 go-m4a 的 FragmentWriter 生成一个真实结构的初始化段和一个分片，
// 再手写 sidx 把它们串成 fMP4，验证流式读取不需要整条下载。
func TestFMP4StreamReadsGeneratedFragment(t *testing.T) {
	cfg := m4a.WriterConfig{
		SampleRate: 48000,
		Channels:   2,
		ASC:        []byte{0x11, 0x90},
	}
	init, err := m4a.InitSegment(cfg)
	if err != nil {
		t.Fatalf("InitSegment: %v", err)
	}
	fw, err := m4a.NewFragmentWriter(cfg)
	if err != nil {
		t.Fatalf("NewFragmentWriter: %v", err)
	}
	frames := [][]byte{{1, 2, 3}, {4, 5, 6, 7}, {8, 9, 10, 11, 12}}
	for _, f := range frames {
		if err := fw.WriteFrame(f); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}
	seg, err := fw.AppendSegment(nil)
	if err != nil {
		t.Fatalf("AppendSegment: %v", err)
	}
	sidx := makeSIDX(48000, [][2]uint32{{uint32(len(seg)), 1024 * uint32(len(frames))}})
	data := append(append(append([]byte(nil), init...), sidx...), seg...)

	rf := &remoteFile{
		size:  int64(len(data)),
		buf:   data,
		start: 0,
		ctx:   context.Background(),
	}
	st, err := newFMP4Stream(rf)
	if err != nil {
		t.Fatalf("newFMP4Stream: %v", err)
	}
	info := st.Info()
	if info.SampleRate != 48000 || info.Channels != 2 {
		t.Errorf("Info 音频参数 = %dHz/%dch", info.SampleRate, info.Channels)
	}
	if info.Duration != time.Duration(len(frames))*1024*time.Second/48000 {
		t.Errorf("Duration = %v", info.Duration)
	}

	raw, err := io.ReadAll(st.RawStream())
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	var got [][]byte
	for len(raw) > 0 {
		n := int(raw[0])<<8 | int(raw[1])
		raw = raw[2:]
		got = append(got, raw[:n])
		raw = raw[n:]
	}
	if len(got) != len(frames) {
		t.Fatalf("读到 %d 帧，want %d", len(got), len(frames))
	}
	for i := range frames {
		if !bytes.Equal(got[i], frames[i]) {
			t.Errorf("帧 %d = %x，want %x", i, got[i], frames[i])
		}
	}

	// seek 到中间一帧所在的时间点，确认分片内跳过生效。
	if _, _, err := st.SeekTime(time.Duration(1024) * time.Second / 48000); err != nil {
		t.Fatalf("SeekTime: %v", err)
	}
	one, err := io.ReadAll(st.RawStream())
	if err != nil {
		t.Fatalf("seek 后 ReadAll: %v", err)
	}
	if len(one) < 2 {
		t.Fatalf("seek 后没有读到帧")
	}
	n := int(one[0])<<8 | int(one[1])
	if n > len(one)-2 || !bytes.Equal(one[2:2+n], frames[1]) {
		t.Errorf("seek 后第一帧不正确")
	}
}

func TestNewFMP4StreamNoSIDX(t *testing.T) {
	data := boxHeaderBytes("ftyp", 4)
	data = append(data, boxHeaderBytes("moov", 0)...)
	rf := &remoteFile{size: int64(len(data)), buf: data, ctx: context.Background()}
	_, err := newFMP4Stream(rf)
	if !errors.Is(err, errNoSIDX) {
		t.Fatalf("没有 sidx 应返回 errNoSIDX，得到 %v", err)
	}
}
