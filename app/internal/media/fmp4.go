package media

// fMP4（B 站 DASH 音轨）的流式解封装。
//
// 背景：B 站 360P 合流的音频在部分稿件里是 HE-AAC（mp4a.40.5），纯 Go 的
// go-aac 解不了，只能改播 DASH 里真正的 AAC-LC 音轨。DASH 音轨是 fMP4
// （ftyp/moov/sidx + 一串 moof/mdat），而 go-m4a 的 NewReader 为了建样本表
// 会把**每一个 moof 都扫一遍**：一小时 192kbps 的音频约 70MB，等于起播前
// 先把整条音轨下完，慢网下就是「一直起播中 / 解析失败」。
//
// 这里直接读 sidx 拿到分片索引，再按需解析 moof/mdat，起播只需要
// ftyp+moov+sidx（<10KB）和第一个分片（几十 KB），seek 也只定位到目标分片。
// 容器结构仍然是标准 ISO-BMFF：sidx 的每条 reference 是一段连续的子分片
// （moof+mdat），trun 里的 data_offset 相对 moof 起点，采样大小/时长来自
// trun 或 tfhd/trex 的默认值。
//
// 只有「有 sidx 的 fMP4」走这里；普通 MP4 / 没有 sidx 的流仍由调用方退回
// go-m4a 的完整解析路径（行为不变）。

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	m4a "github.com/tphakala/go-m4a"
)

// fmp4MaxBox 是解析初始化段时单个 box 的体积上限。moov/sidx 都是几 KB，
// 这个值只用于挡住损坏文件里的天文长度。
const fmp4MaxBox = 32 << 20

// fmp4Chunk 是 DASH 音轨的取块大小。B 站 512KB 的默认块在慢网下单次
// Range 请求可能超过 media.remote 的 30 秒超时（而 fMP4 起播本来只需要
// 前 10KB），所以这里用更小的块；播放时 remoteFile 仍会异步预取下一块，
// 不会因此多出同步往返。
const fmp4Chunk = 128 << 10

// segLenGuard 是单个 access unit 的体积上界；真正的边界在 nextFrame 里
// 按 segBuf 的长度再校验一次。
const segLenGuard = fmp4MaxBox

// errNoSIDX 表示这不是带 sidx 的 fMP4（普通 MP4、或无索引的 fMP4）。
// 调用方据此退回 go-m4a 的通用解封装，而不是把它当成真正的错误。
var errNoSIDX = errors.New("media: fMP4 没有 sidx 索引")

// errShortBox 是内部解析用的截断错误，最终会带上 KindUnsupported。
var errShortBox = errors.New("media: fMP4 box 被截断")

// fmp4Trex 是 moov/mvex/trex 里与音频采样有关的默认值。
type fmp4Trex struct {
	duration uint32
	size     uint32
}

// fmp4Segment 是 sidx 的一条 reference：一段连续的 moof+mdat 子分片。
// start/duration 的单位是媒体 timescale。
type fmp4Segment struct {
	off      int64
	size     int64
	start    uint64
	duration uint64
}

// fmp4Sample 是一个 AAC access unit：off 相对当前分片缓冲区（也就是
// moof 起点所在的那块内存），不是文件偏移。
type fmp4Sample struct {
	off  int32
	size uint32
	dur  uint32
}

// fmp4Init 是初始化段解析结果。ftyp/moov 只保留 body，sidxEnd 是 sidx
// box 之后的文件偏移（第一条 reference 的基准点）。
type fmp4Init struct {
	brand   string
	ftyp    []byte
	moov    []byte
	sidx    []byte
	sidxEnd int64
	trex    map[uint32]fmp4Trex
}

// readBoxHeader 解析 b 开头的 box 头。avail 是当前流剩余字节数，用于解释
// size=0（延伸到文件末尾）并拒绝越过文件末尾的 box。
func readBoxHeader(b []byte, avail int64) (typ string, size, hdr int64, err error) {
	if len(b) < 8 {
		return "", 0, 0, errShortBox
	}
	size = int64(binary.BigEndian.Uint32(b[:4]))
	typ = string(b[4:8])
	hdr = 8
	switch size {
	case 0:
		size = avail
	case 1:
		if len(b) < 16 {
			return "", 0, 0, errShortBox
		}
		size = int64(binary.BigEndian.Uint64(b[8:16]))
		hdr = 16
	}
	if size < hdr || size > avail {
		return "", 0, 0, fmt.Errorf("media: fMP4 box %q 长度 %d 非法（剩余 %d）", typ, size, avail)
	}
	return typ, size, hdr, nil
}

// walkBoxes 顺序遍历一段 box 体里的直接子 box。
func walkBoxes(b []byte, fn func(typ string, body []byte, off, size int64) error) error {
	avail := int64(len(b))
	var off int64
	for avail-off >= 8 {
		rem := b[off:]
		typ, size, hdr, err := readBoxHeader(rem, avail-off)
		if err != nil {
			return err
		}
		if err := fn(typ, rem[hdr:size], off, size); err != nil {
			return err
		}
		off += size
	}
	return nil
}

// readTopBoxHeader 从 rf 读取一个顶层 box 头。size=0 表示到文件末尾。
func readTopBoxHeader(rf *remoteFile, remaining int64) (typ string, size, hdr int64, err error) {
	var hdrBuf [16]byte
	if _, err = io.ReadFull(rf, hdrBuf[:8]); err != nil {
		err = WrapKind(KindNetwork, err)
		return
	}
	size = int64(binary.BigEndian.Uint32(hdrBuf[:4]))
	typ = string(hdrBuf[4:8])
	hdr = 8
	switch size {
	case 0:
		size = remaining
	case 1:
		if _, err = io.ReadFull(rf, hdrBuf[8:16]); err != nil {
			err = WrapKind(KindNetwork, err)
			return
		}
		size = int64(binary.BigEndian.Uint64(hdrBuf[8:16]))
		hdr = 16
	}
	if size < hdr || size > remaining {
		err = fmt.Errorf("media: fMP4 box %q 长度 %d 非法（剩余 %d）", typ, size, remaining)
	}
	return
}

// readBoxBody 按长度读一段 box 体。网络读失败标成 KindNetwork，
// 让 audioErrorMessage 仍然显示网络错误而不是「格式不支持」。
func readBoxBody(rf *remoteFile, n int64) ([]byte, error) {
	if n < 0 || n > fmp4MaxBox {
		return nil, fmt.Errorf("media: fMP4 box 体积异常: %d", n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(rf, b); err != nil {
		return nil, WrapKind(KindNetwork, err)
	}
	return b, nil
}

// readFMP4Init 从头顺序读取 ftyp/moov/sidx。只读到 sidx 或第一个 moof 为止，
// 绝不扫描 mdat 里的媒体数据。
func readFMP4Init(rf *remoteFile) (*fmp4Init, error) {
	if _, err := rf.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	total := rf.Size()
	if total < 16 {
		return nil, errNoSIDX
	}
	init := &fmp4Init{trex: map[uint32]fmp4Trex{}}
	var pos int64
	for pos+8 <= total {
		remaining := total - pos
		typ, size, hdr, err := readTopBoxHeader(rf, remaining)
		if err != nil {
			return nil, err
		}
		bodyLen := size - hdr
		switch typ {
		case "ftyp":
			body, err := readBoxBody(rf, bodyLen)
			if err != nil {
				return nil, err
			}
			init.ftyp = body
			if len(body) >= 4 {
				init.brand = string(body[:4])
			}
		case "moov":
			body, err := readBoxBody(rf, bodyLen)
			if err != nil {
				return nil, err
			}
			init.moov = body
			init.trex = parseTrex(body)
		case "sidx":
			body, err := readBoxBody(rf, bodyLen)
			if err != nil {
				return nil, err
			}
			init.sidx = body
			init.sidxEnd = pos + size
			if init.moov != nil {
				return init, nil
			}
		case "moof":
			// 到第一个媒体分片还看不到 sidx：没有索引可用。
			return init, errNoSIDX
		default:
			// mdat 可能很大，直接 seek 跳过，不读它的内容。
			if _, err := rf.Seek(bodyLen, io.SeekCurrent); err != nil {
				return nil, WrapKind(KindNetwork, err)
			}
		}
		pos += size
		if init.moov != nil && init.sidx != nil {
			return init, nil
		}
	}
	return init, errNoSIDX
}

// parseTrex 从 moov/mvex/trex 里取各轨道的默认采样时长/大小。
// 解析失败只影响没有显式 trun 字段的流，按空表处理即可。
func parseTrex(moov []byte) map[uint32]fmp4Trex {
	m := map[uint32]fmp4Trex{}
	_ = walkBoxes(moov, func(typ string, body []byte, _, _ int64) error {
		if typ != "mvex" {
			return nil
		}
		return walkBoxes(body, func(typ string, b []byte, _, _ int64) error {
			if typ != "trex" || len(b) < 24 {
				return nil
			}
			id := binary.BigEndian.Uint32(b[4:8])
			m[id] = fmp4Trex{
				duration: binary.BigEndian.Uint32(b[12:16]),
				size:     binary.BigEndian.Uint32(b[16:20]),
			}
			return nil
		})
	})
	return m
}

// parseSIDX 解析 sidx（version 0/1），返回每条 reference 的分片位置、
// 媒体 timescale 和总时长（timescale 单位）。
func parseSIDX(body []byte, boxEnd, fileSize int64) (segs []fmp4Segment, timescale uint32, total uint64, err error) {
	if len(body) < 4 {
		return nil, 0, 0, fmt.Errorf("%w: sidx 太短", errNoSIDX)
	}
	version := body[0]
	if version > 1 {
		return nil, 0, 0, fmt.Errorf("%w: sidx version %d 不支持", errNoSIDX, version)
	}
	p := 4
	if len(body) < p+8 {
		return nil, 0, 0, fmt.Errorf("%w: sidx 缺少 timescale", errNoSIDX)
	}
	p += 4 // reference_ID
	timescale = binary.BigEndian.Uint32(body[p:])
	p += 4
	if timescale == 0 {
		return nil, 0, 0, fmt.Errorf("%w: sidx timescale 为 0", errNoSIDX)
	}
	var firstOffset uint64
	if version == 0 {
		if len(body) < p+8+4 {
			return nil, 0, 0, fmt.Errorf("%w: sidx 头被截断", errNoSIDX)
		}
		p += 4 // earliest_presentation_time
		firstOffset = uint64(binary.BigEndian.Uint32(body[p:]))
		p += 4
	} else {
		if len(body) < p+16+4 {
			return nil, 0, 0, fmt.Errorf("%w: sidx 头被截断", errNoSIDX)
		}
		p += 8 // earliest_presentation_time
		firstOffset = binary.BigEndian.Uint64(body[p:])
		p += 8
	}
	if len(body) < p+2+2 {
		return nil, 0, 0, fmt.Errorf("%w: sidx 缺少 reference 数量", errNoSIDX)
	}
	p += 2 // reserved
	count := int(binary.BigEndian.Uint16(body[p:]))
	p += 2
	if count == 0 {
		return nil, 0, 0, fmt.Errorf("%w: sidx 没有 reference", errNoSIDX)
	}
	if int64(count) > int64(len(body)-p)/12 {
		return nil, 0, 0, fmt.Errorf("%w: sidx reference 表被截断", errNoSIDX)
	}
	if firstOffset > math.MaxInt64 || boxEnd < 0 || boxEnd > math.MaxInt64-int64(firstOffset) {
		return nil, 0, 0, fmt.Errorf("%w: sidx first_offset 越界", errNoSIDX)
	}
	cur := boxEnd + int64(firstOffset)
	segs = make([]fmp4Segment, 0, count)
	for i := 0; i < count; i++ {
		ref := binary.BigEndian.Uint32(body[p:])
		dur := binary.BigEndian.Uint32(body[p+4:])
		p += 12 // referenced_size + subsegment_duration + SAP
		if ref>>31 != 0 {
			// reference_type=1 是层级索引（指向另一个 sidx），这里不支持。
			return nil, 0, 0, fmt.Errorf("%w: sidx 是层级索引", errNoSIDX)
		}
		size := int64(ref & 0x7fffffff)
		if size <= 0 {
			return nil, 0, 0, fmt.Errorf("%w: sidx 子分片长度为 0", errNoSIDX)
		}
		if cur > math.MaxInt64-size {
			return nil, 0, 0, fmt.Errorf("%w: sidx 子分片偏移溢出", errNoSIDX)
		}
		if fileSize >= 0 && cur+size > fileSize {
			return nil, 0, 0, fmt.Errorf("%w: sidx 子分片越过文件末尾", errNoSIDX)
		}
		segs = append(segs, fmp4Segment{
			off:      cur,
			size:     size,
			start:    total,
			duration: uint64(dur),
		})
		cur += size
		total += uint64(dur)
	}
	return segs, timescale, total, nil
}

// fmp4InitInfo 用初始化段的 moov 取音频参数（采样率/声道/ASC）。
//
// go-m4a 的 NewReader 在「moov 里带 mvex」时会按分片输入处理，但这里只有
// 初始化段、没有 moof，会被当成空的分片流而报错；把 mvex 去掉后它就按
// 普通 moov 解析，得到格式信息（样本表为空，Duration/FrameCount 为 0，
// 这两个值下面会用 sidx 覆盖）。
func fmp4InitInfo(ftyp, moov []byte) (m4a.Info, error) {
	stripped := stripDirectBox(moov, "mvex")
	var buf bytes.Buffer
	if len(ftyp) > 0 {
		writeBox(&buf, "ftyp", ftyp)
	}
	if len(stripped) > 0 {
		writeBox(&buf, "moov", stripped)
	} else {
		writeBox(&buf, "moov", moov)
	}
	rd, err := m4a.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return m4a.Info{}, err
	}
	return rd.Info(), nil
}

// stripDirectBox 复制 b，并去掉其中类型为 drop 的直接子 box。
func stripDirectBox(b []byte, drop string) []byte {
	out := make([]byte, 0, len(b))
	avail := int64(len(b))
	var off int64
	for avail-off >= 8 {
		rem := b[off:]
		typ, size, _, err := readBoxHeader(rem, avail-off)
		if err != nil {
			break
		}
		if typ != drop {
			out = append(out, rem[:size]...)
		}
		off += size
	}
	return out
}

// writeBox 写一个 8 字节头的 box。
func writeBox(buf *bytes.Buffer, typ string, body []byte) {
	var hdr [8]byte
	binary.BigEndian.PutUint32(hdr[:4], uint32(len(body)+8))
	copy(hdr[4:], typ)
	buf.Write(hdr[:])
	buf.Write(body)
}

// ticksDuration 把 timescale 单位的时长换成 time.Duration。
func ticksDuration(ticks uint64, timescale uint32) time.Duration {
	if timescale == 0 {
		return 0
	}
	return time.Duration(float64(ticks) * float64(time.Second) / float64(timescale))
}

// durationTicks 把 time.Duration 换成 timescale 单位，四舍五入（与
// go-m4a 的 rescaleTicks 一致）。向下取整会让「正好一帧」的时间因
// 纳秒截断落到前一帧（1024/48000 秒算出来是 1023.999984 tick）。
func durationTicks(d time.Duration, timescale uint32) uint64 {
	if d <= 0 || timescale == 0 {
		return 0
	}
	return (uint64(d)*uint64(timescale) + uint64(time.Second)/2) / uint64(time.Second)
}

// fmp4Stream 实现 pipeline 需要的解封装接口。
type fmp4Stream struct {
	rf        *remoteFile
	info      m4a.Info
	timescale uint32
	segs      []fmp4Segment
	total     uint64
	trex      map[uint32]fmp4Trex

	// 当前分片与读游标。
	segIndex int
	segBuf   []byte
	samples  []fmp4Sample
	next     int
}

// newFMP4Stream 尝试把 rf 当成带 sidx 的 fMP4 打开。
// 不是这种流时返回 errNoSIDX，由调用方退回 go-m4a。
func newFMP4Stream(rf *remoteFile) (*fmp4Stream, error) {
	// 起播只需要前几 KB；把块调小，慢网下不会为了一次 512KB Range 等到超时。
	rf.chunk = fmp4Chunk
	init, err := readFMP4Init(rf)
	if err != nil {
		return nil, err
	}
	if len(init.sidx) == 0 || len(init.moov) == 0 {
		return nil, errNoSIDX
	}
	segs, timescale, total, err := parseSIDX(init.sidx, init.sidxEnd, rf.Size())
	if err != nil {
		return nil, err
	}
	info, err := fmp4InitInfo(init.ftyp, init.moov)
	if err != nil {
		return nil, WrapKind(KindUnsupported, fmt.Errorf("media: 解析 fMP4 初始化段失败: %w", err))
	}
	if info.SampleRate <= 0 || info.Channels <= 0 || len(info.ASC) == 0 {
		return nil, WrapKind(KindUnsupported, errors.New("media: fMP4 缺少音频配置"))
	}
	// Brand 在 B 站 DASH 里常见是 isom，但这条流确实是分片的；保留真实
	// brand 供诊断，seek 是否原地走由 pipeline 的 fragmented 标记决定。
	info.Brand = init.brand
	info.Duration = ticksDuration(total, timescale)
	info.FrameCount = 0
	return &fmp4Stream{
		rf:        rf,
		info:      info,
		timescale: timescale,
		segs:      segs,
		total:     total,
		trex:      init.trex,
		segIndex:  -1,
	}, nil
}

// Info 实现 demuxer。
func (s *fmp4Stream) Info() m4a.Info { return s.info }

// RawStream 返回按「2 字节大端长度 + AAC access unit」输出的流，
// 与 go-m4a 的 RawStream 格式一致。
func (s *fmp4Stream) RawStream() io.Reader { return &fmp4Raw{s: s} }

// SeekTime 定位到包含 d 的分片，并丢弃该分片内 d 之前的采样。
// 返回的 sample 下标对调用方没有意义（pipeline 只用它是成功信号），
// 真正的读游标已经在这里移动好。
func (s *fmp4Stream) SeekTime(d time.Duration) (int, int64, error) {
	if len(s.segs) == 0 {
		return 0, 0, WrapKind(KindUnsupported, errors.New("media: fMP4 没有分片索引"))
	}
	if d < 0 {
		d = 0
	}
	ticks := durationTicks(d, s.timescale)
	if s.total > 0 && ticks >= s.total {
		// 与 go-m4a 的边界一致：落在末尾时退到最后一 tick，避免选到 EOF。
		ticks = s.total - 1
	}
	idx := sort.Search(len(s.segs), func(i int) bool {
		return s.segs[i].start+s.segs[i].duration > ticks
	})
	if idx >= len(s.segs) {
		idx = len(s.segs) - 1
	}
	skip := ticks - s.segs[idx].start
	s.segBuf, s.samples, s.next, s.segIndex = nil, nil, 0, -1
	if err := s.loadSegment(idx, skip); err != nil {
		return 0, 0, err
	}
	return 0, 0, nil
}

// SeekSample 是 SeekTime 不可用时的兜底路径（pipeline 按平均帧长估算帧号）。
// 以 1024 采样/帧折算成时间再定位；AAC-LC 固定 1024，够用。
func (s *fmp4Stream) SeekSample(sample int) error {
	if sample <= 0 {
		_, _, err := s.SeekTime(0)
		return err
	}
	if s.info.SampleRate <= 0 {
		return WrapKind(KindUnsupported, errors.New("media: fMP4 采样率未知"))
	}
	sec := float64(sample) * 1024 / float64(s.info.SampleRate)
	_, _, err := s.SeekTime(time.Duration(sec * float64(time.Second)))
	return err
}

// loadSegment 读取并解析第 idx 个分片，skip 是要在该分片内丢弃的媒体 tick 数。
func (s *fmp4Stream) loadSegment(idx int, skip uint64) error {
	if idx < 0 || idx >= len(s.segs) {
		return io.EOF
	}
	seg := s.segs[idx]
	buf, err := s.readSegment(seg)
	if err != nil {
		return err
	}
	samples, err := parseMoofSamples(buf, seg.off, s.trex)
	if err != nil {
		return WrapKind(KindUnsupported, err)
	}
	s.segIndex = idx
	s.segBuf = buf
	s.samples = samples
	s.next = 0
	if skip == 0 {
		return nil
	}
	// 分片内按 trun 的采样时长前进；时长缺失时用 sidx 的平均值。
	var totalDur uint64
	for _, sm := range samples {
		totalDur += uint64(sm.dur)
	}
	var avg uint64
	if totalDur == 0 && seg.duration > 0 && len(samples) > 0 {
		avg = seg.duration / uint64(len(samples))
	}
	for s.next < len(samples) {
		d := uint64(samples[s.next].dur)
		if d == 0 {
			d = avg
		}
		if d == 0 || skip < d {
			break
		}
		skip -= d
		s.next++
	}
	if s.next >= len(samples) && len(samples) > 0 {
		// 目标正好落在分片末尾（或时长字段有轻微误差）时，宁可停在最后
		// 一个采样上，也不要让游标越界后立刻报 EOF。
		s.next = len(samples) - 1
	}
	return nil
}

// readSegment 把一段连续的子分片（styp?/moof/mdat）读进内存。
func (s *fmp4Stream) readSegment(seg fmp4Segment) ([]byte, error) {
	if seg.size <= 0 || seg.size > fmp4MaxBox {
		return nil, WrapKind(KindUnsupported, fmt.Errorf("media: fMP4 分片长度异常: %d", seg.size))
	}
	if _, err := s.rf.Seek(seg.off, io.SeekStart); err != nil {
		return nil, err
	}
	buf := make([]byte, seg.size)
	if _, err := io.ReadFull(s.rf, buf); err != nil {
		return nil, WrapKind(KindNetwork, err)
	}
	return buf, nil
}

// nextFrame 取下一个 AAC access unit。它只读当前分片缓冲区，不碰网络——
// 网络读发生在 loadSegment 里（由 pump 的 procMu 串行化）。
func (s *fmp4Stream) nextFrame() ([]byte, error) {
	for {
		if s.next < len(s.samples) {
			sm := s.samples[s.next]
			s.next++
			if sm.size == 0 {
				continue
			}
			end := int64(sm.off) + int64(sm.size)
			if sm.off < 0 || end > int64(len(s.segBuf)) {
				return nil, WrapKind(KindUnsupported, fmt.Errorf("media: fMP4 样本越界（off=%d size=%d len=%d）", sm.off, sm.size, len(s.segBuf)))
			}
			return s.segBuf[sm.off:end], nil
		}
		if s.segIndex < 0 {
			if err := s.loadSegment(0, 0); err != nil {
				return nil, err
			}
			continue
		}
		if s.segIndex+1 >= len(s.segs) {
			return nil, io.EOF
		}
		if err := s.loadSegment(s.segIndex+1, 0); err != nil {
			return nil, err
		}
	}
}

// fmp4Raw 把 access unit 序列转成 go-aac 认的「长度前缀」字节流。
type fmp4Raw struct {
	s        *fmp4Stream
	frame    []byte
	prefix   [2]byte
	pos      int
	inPrefix bool
	have     bool
}

func (r *fmp4Raw) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if !r.have {
			frame, err := r.s.nextFrame()
			if err != nil {
				return 0, err
			}
			if len(frame) > 0xffff {
				return 0, WrapKind(KindUnsupported, fmt.Errorf("media: fMP4 AAC 帧过大: %d", len(frame)))
			}
			r.frame = frame
			r.prefix = [2]byte{byte(len(frame) >> 8), byte(len(frame))}
			r.pos = 0
			r.inPrefix = true
			r.have = true
		}
		src := r.frame
		if r.inPrefix {
			src = r.prefix[:]
		}
		n := copy(p, src[r.pos:])
		r.pos += n
		if r.pos == len(src) {
			r.pos = 0
			if r.inPrefix {
				r.inPrefix = false
			} else {
				r.have = false
				r.frame = nil
			}
		}
		if n > 0 {
			return n, nil
		}
	}
}

// findMoof 在子分片里找到 moof 的起点。CMAF 分片有时以 styp 开头，
// sidx 的 reference 指向的是整个子分片；B 站自己的 m4s 则是 moof 打头。
func findMoof(seg []byte) (off, size int64, err error) {
	avail := int64(len(seg))
	for avail-off >= 8 {
		typ, sz, _, err := readBoxHeader(seg[off:], avail-off)
		if err != nil {
			return 0, 0, err
		}
		if typ == "moof" {
			return off, sz, nil
		}
		switch typ {
		case "styp", "prft", "emsg", "free", "sidx":
			off += sz
		default:
			return 0, 0, fmt.Errorf("media: fMP4 分片里找不到 moof（首 box 是 %q）", typ)
		}
	}
	return 0, 0, fmt.Errorf("media: fMP4 分片里找不到 moof")
}

// parseMoofSamples 解析一个 moof 里的第一个 traf（B 站音轨只有一个）并把
// trun 的 data_offset 换算成相对 segBuf 的偏移。
func parseMoofSamples(seg []byte, segStart int64, trex map[uint32]fmp4Trex) ([]fmp4Sample, error) {
	moofOff, moofSize, err := findMoof(seg)
	if err != nil {
		return nil, err
	}
	typ, _, hdr, err := readBoxHeader(seg[moofOff:], int64(len(seg))-moofOff)
	if err != nil || typ != "moof" {
		return nil, fmt.Errorf("media: fMP4 moof 头损坏: %v", err)
	}
	var (
		samples []fmp4Sample
		seen    bool
	)
	err = walkBoxes(seg[moofOff+hdr:moofOff+moofSize], func(typ string, body []byte, _, _ int64) error {
		if typ != "traf" || seen {
			return nil
		}
		seen = true
		var tr fmp4Traf
		if err := parseTraf(body, &tr); err != nil {
			return err
		}
		var err error
		samples, err = tr.samples(segStart, moofOff, moofSize, trex)
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(samples) == 0 {
		return nil, errors.New("media: fMP4 分片里没有音频样本")
	}
	return samples, nil
}

// fmp4Traf 是一个 traf 里与采样布局有关的原始字段。
type fmp4Traf struct {
	track      uint32
	flags      uint32
	baseOffset int64
	hasBase    bool
	defDur     uint32
	defSize    uint32
	trun       []byte
}

func parseTraf(body []byte, tr *fmp4Traf) error {
	return walkBoxes(body, func(typ string, b []byte, _, _ int64) error {
		switch typ {
		case "tfhd":
			if len(b) < 8 {
				return errShortBox
			}
			tr.flags = binary.BigEndian.Uint32(b[:4]) & 0x00ffffff
			tr.track = binary.BigEndian.Uint32(b[4:8])
			p := 8
			if tr.flags&0x000001 != 0 { // base-data-offset-present
				if len(b) < p+8 {
					return errShortBox
				}
				tr.baseOffset = int64(binary.BigEndian.Uint64(b[p:]))
				p += 8
				tr.hasBase = true
			}
			if tr.flags&0x000002 != 0 { // sample-description-index-present
				p += 4
			}
			if tr.flags&0x000008 != 0 { // default-sample-duration-present
				if len(b) < p+4 {
					return errShortBox
				}
				tr.defDur = binary.BigEndian.Uint32(b[p:])
				p += 4
			}
			if tr.flags&0x000010 != 0 { // default-sample-size-present
				if len(b) < p+4 {
					return errShortBox
				}
				tr.defSize = binary.BigEndian.Uint32(b[p:])
				p += 4
			}
		case "trun":
			tr.trun = b
		}
		return nil
	})
}

// samples 把 trun 展开成逐样本的偏移/大小/时长。
func (tr *fmp4Traf) samples(segStart, moofOff, moofSize int64, trex map[uint32]fmp4Trex) ([]fmp4Sample, error) {
	if len(tr.trun) < 8 {
		return nil, errShortBox
	}
	def := trex[tr.track]
	defDur, defSize := tr.defDur, tr.defSize
	if defDur == 0 {
		defDur = def.duration
	}
	if defSize == 0 {
		defSize = def.size
	}
	flags := binary.BigEndian.Uint32(tr.trun[:4]) & 0x00ffffff
	count := int(binary.BigEndian.Uint32(tr.trun[4:8]))
	p := 8
	var dataOffset int64
	dataOffsetPresent := flags&0x000001 != 0
	if dataOffsetPresent {
		if len(tr.trun) < p+4 {
			return nil, errShortBox
		}
		dataOffset = int64(int32(binary.BigEndian.Uint32(tr.trun[p:])))
		p += 4
	}
	if flags&0x000004 != 0 { // first-sample-flags-present
		p += 4
	}
	if count < 0 || count > (len(tr.trun)-p)/4 {
		return nil, fmt.Errorf("media: fMP4 trun 样本表被截断（count=%d）", count)
	}
	// 样本数据的基准偏移（相对当前分片缓冲区 seg）：
	//   base-data-offset-present → 文件绝对偏移，减去 segStart；
	//   否则 data_offset 相对 moof 起点；
	//   两者都没有 → mdat 负载从 moof 末尾 + 8 字节开始。
	base := moofOff
	switch {
	case tr.hasBase:
		base = tr.baseOffset - segStart + dataOffset
	case dataOffsetPresent:
		base = moofOff + dataOffset
	default:
		base = moofOff + moofSize + 8
	}
	pos := base
	out := make([]fmp4Sample, 0, count)
	for i := 0; i < count; i++ {
		var dur, size uint32
		if flags&0x000100 != 0 { // sample-duration-present
			if len(tr.trun) < p+4 {
				return nil, errShortBox
			}
			dur = binary.BigEndian.Uint32(tr.trun[p:])
			p += 4
		} else {
			dur = defDur
		}
		if flags&0x000200 != 0 { // sample-size-present
			if len(tr.trun) < p+4 {
				return nil, errShortBox
			}
			size = binary.BigEndian.Uint32(tr.trun[p:])
			p += 4
		} else {
			size = defSize
		}
		if flags&0x000400 != 0 { // sample-flags-present
			if len(tr.trun) < p+4 {
				return nil, errShortBox
			}
			p += 4
		}
		if flags&0x000800 != 0 { // sample-composition-time-offsets-present
			if len(tr.trun) < p+4 {
				return nil, errShortBox
			}
			p += 4
		}
		if size == 0 {
			return nil, fmt.Errorf("media: fMP4 样本 %d 长度为 0", i)
		}
		if pos < 0 || pos > math.MaxInt32 || int64(size) > int64(segLenGuard) {
			return nil, fmt.Errorf("media: fMP4 样本偏移/长度异常")
		}
		out = append(out, fmp4Sample{off: int32(pos), size: size, dur: dur})
		pos += int64(size)
	}
	return out, nil
}
