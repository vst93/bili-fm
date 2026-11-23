// Package dashhls 把 B 站 DASH 的音画分片直接当成本地 HLS 播出来。
//
// 为什么需要它：视频窗走平台原生播放器（GStreamer / Media Foundation /
// AVPlayer），源一直是 platform=html5 的 360P 合流 MP4。那种文件的 moov
// （索引）有几 MB 且在文件头，实测 BV1BFsA6TERt 要先把 6.5MB 索引读完才
// 解得出第一帧；1MiB/s 限速下首帧 6.57 秒。
//
// DASH 的每个分片本身就是自洽的 moof+mdat，sidx 里带精确的字节区间。所以
// 这里**不改写任何一个字节**：只生成一份本地 HLS 播放列表，把区间编进分片
// URL 的查询串，让播放器按需拉取。音画同步、拖动、倍速全部交给系统媒体框架
// —— 它本来就是干这个的。
//
// 为什么是 HLS 而不是「自己封装成一个 fMP4 文件」：后者要合并 moov、改
// tkhd/trex 的 track ID、重算 sidx、再生成 mfra/tfra，是几百行二进制改写，
// 出错的表现是静默花屏或音画漂移；而且 Linux 的 qtdemux 在 push 模式下
// **忽略**分片 MP4 的 seek，单文件方案在 Linux 上根本拖不动（需要自写
// GstBaseSrc 拉流源，进不了产品）。HLS 三个平台都原生支持，没有这些代价。
//
// 实测（同机同播放器，冷连接）：首帧 544/623/994ms、首帧前 CDN 字节
// 476-593KB；旧合流路径 1447ms、12.3MB。时长、seek、2 倍速、音频解码均正常。
package dashhls

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ua 与 proxy 里那条一致：B 站 CDN 认这个 UA，换别的会被 403。
const uaChrome131 = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// referer 同样必须带，否则 CDN 直接 403。
const referer = "https://www.bilibili.com/"

// Track 是 playurl 里的一条 DASH 轨道（data.dash.video[] / audio[]）。
//
// InitRange / IndexRange 就是接口给的 "0-933" / "934-13585" 这种闭区间；
// 分片区间由 sidx 推出来，不在这里。
type Track struct {
	BaseURL    string
	BackupURLs []string
	InitRange  string
	IndexRange string
	Codecs     string
	MimeType   string
	Bandwidth  int
	Width      int
	Height     int
}

// 分段拉取的上限。B 站 360P 单个分片最大约 370KB，留足余量即可；设上限是
// 为了防止一个坏区间把整条轨道拉下来（那正是我们要避免的浪费）。
const maxSegmentBytes = 8 << 20

// fetchTimeout 是单次上游请求的超时。分片本身很小，30s 足够，也给慢网留了
// 余量；不能设成 0（无限等），否则关窗时 goroutine 会挂住。
const fetchTimeout = 30 * time.Second

// 分片预取（read-ahead）。播放器按顺序往下播，下一个分片几乎总是紧接着
// 就要；在把当前分片回完之后顺手把后面几个也拉下来，上游 RTT 就不再落在
// 播放器身上 —— 3 倍速这种吃数据的速度下，这是「卡一下缓冲一下」的主要
// 来源之一。
//
// 缓存只放**媒体分片**（init 段很小，不值得走缓存路径），总量封顶 6MiB：
// B 站 360P 单分片约 100~370KB，够放十几个，回拖 / seek 回来也能命中。
const (
	prefetchSegments = 3
	maxPrefetchBytes = 6 << 20
)

// sidx 的一条 reference：分片字节数与时长（timescale 单位）。
type reference struct {
	size uint32
	dur  uint32
}

// track 是一条轨道的本地状态：init/sidx 解析结果 + 上游连接池。
type track struct {
	name   string // "video" / "audio"，只用于错误信息
	kind   byte   // 'v' / 'a'，编进分片 URL
	src    Track
	init   [2]int64 // init 区间
	index  [2]int64 // sidx 区间
	refs   []reference
	scale  uint32   // sidx timescale
	origin int64    // 第一个分片的字节偏移
	media  [2]int64 // 媒体数据区间 [start, end]（闭区间）
	dur    float64  // 这条轨道按 sidx 算出来的时长（秒）

	// starts 是每个分片在整条轨道里的字节起点，顺序与 refs 一致。预取要用
	// 它把「播放器请求的区间」映射回分片号（区间是原样编进 URL 的，不能反解）。
	// 生产路径在 load 里一次算好；直接构造 track 的测试靠 startsOnce 懒算。
	starts     []int64
	startsOnce sync.Once

	client *http.Client

	// 分片读缓存与预取状态（见 prefetchSegments）。cache 只被 serveSegment
	// 与预取 goroutine 访问，全部在 cacheMu 下。
	cacheMu     sync.Mutex
	cache       map[int64][]byte
	cacheOrder  []int64
	cacheBytes  int
	prefetching atomic.Bool
	// fetches 是这条轨道真正回上游的次数（预取与播放器请求都算）。缓存命中
	// 不加。测试用它钉「第二个请求不再走上游」，不会被另一条轨道的预取干扰
	// （两条轨道常常指向同一个上游地址、区间也一样）。
	fetches atomic.Int64
	// stop 在 Session.Close 时关闭，预取 goroutine 见到就退出（它可能在
	// 关窗后还挂在一次上游请求上，靠 fetchTimeout 收尾）。
	stop chan struct{}
}

// Session 是一个视频的本地 HLS 服务。用完必须 Close，否则监听端口和上游
// 连接会一直挂着。
type Session struct {
	srv  *http.Server
	ln   net.Listener
	url  string
	vid  *track
	aud  *track
	dur  float64
	once sync.Once
	err  error
}

// Start 起一个本地 HLS 服务：并发取两条轨道的 init+sidx，解析出分片表，
// 把两份播放列表算好放进内存，然后开始监听 127.0.0.1 的随机端口。
//
// duration 是接口给的秒数，只用来兜底（sidx 的时长更准，优先用它）。
func Start(video, audio Track, duration float64) (*Session, error) {
	v := &track{name: "video", kind: 'v', src: video, client: newClient(), stop: make(chan struct{})}
	a := &track{name: "audio", kind: 'a', src: audio, client: newClient(), stop: make(chan struct{})}

	// init 与 sidx 是两条独立的请求，并发取：串行时那一个 RTT 会直接落在
	// 起播延迟上（实测两次各 ~50ms，但慢网下差别更大）。
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, t := range []*track{v, a} {
		wg.Add(1)
		go func(i int, t *track) {
			defer wg.Done()
			errs[i] = t.load()
		}(i, t)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	dur := v.dur
	if a.dur > dur {
		dur = a.dur
	}
	if dur <= 0 {
		dur = duration
	}

	s := &Session{vid: v, aud: a, dur: dur}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("dashhls: 本地端口绑定失败: %w", err)
	}
	s.ln = ln
	s.url = fmt.Sprintf("http://%s/master.m3u8", ln.Addr().String())

	mux := http.NewServeMux()
	mux.HandleFunc("/master.m3u8", s.serveMaster)
	mux.HandleFunc("/video.m3u8", s.serveMedia('v'))
	mux.HandleFunc("/audio.m3u8", s.serveMedia('a'))
	mux.HandleFunc("/seg", s.serveSegment)
	s.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// 监听已关（Close 的正常路径）之外不该出错；记到字段里让测试能看。
			s.err = err
		}
	}()
	// 提前加载头两个分片：播放器解析完播放列表就要它们，等它来要再回上游
	// 就是一次白等的 RTT（首帧前那几百毫秒里最值钱的一段）。
	v.warm()
	a.warm()
	return s, nil
}

// URL 是给播放器的 master 播放列表地址。
func (s *Session) URL() string { return s.url }

// Duration 是解析出来的时长（秒），供调用方与接口值对账。
func (s *Session) Duration() float64 { return s.dur }

// Close 停服务、关上游连接。可以重复调用。
func (s *Session) Close() error {
	s.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if s.srv != nil {
			_ = s.srv.Shutdown(ctx)
		}
		if s.ln != nil {
			_ = s.ln.Close()
		}
		for _, t := range []*track{s.vid, s.aud} {
			if t != nil {
				if t.stop != nil {
					close(t.stop)
				}
				t.client.CloseIdleConnections()
			}
		}
	})
	return nil
}

// newClient 给一条轨道建 HTTP 客户端。
//
// MaxIdleConnsPerHost 必须 >=1 且 KeepAlive 打开：**每个分片新建一条 TLS
// 连接会让 HLS 比旧的合流路径还慢**（实测握手 3.5s 的那台 CDN 直接把首帧
// 推到 11s）。这条是本包性能的关键，别改。
//
// DisableCompression：媒体字节不能被 gzip，否则 Content-Length 与实际不符。
func newClient() *http.Client {
	return &http.Client{
		Timeout: fetchTimeout,
		Transport: &http.Transport{
			// B 站部分 CDN 的证书链不全，仓库里其他地方（bilibili / proxy）
			// 也是放行的。
			TLSClientConfig:       insecureTLS(),
			MaxIdleConns:          4,
			MaxIdleConnsPerHost:   2,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			DisableCompression:    true,
			ForceAttemptHTTP2:     true,
		},
	}
}

// load 取 init 与 sidx 并解析。init 本身不参与播放列表之外的计算，但
// #EXT-X-MAP 要用它的区间，所以只校验区间合法。
func (t *track) load() error {
	init, err := parseRange(t.src.InitRange)
	if err != nil {
		return fmt.Errorf("dashhls: %s 的 init 区间无效: %w", t.name, err)
	}
	idx, err := parseRange(t.src.IndexRange)
	if err != nil {
		return fmt.Errorf("dashhls: %s 的 index 区间无效: %w", t.name, err)
	}
	if t.src.BaseURL == "" {
		return fmt.Errorf("dashhls: %s 缺少地址", t.name)
	}
	t.init, t.index = init, idx

	body, err := t.fetch(idx[0], idx[1])
	if err != nil {
		return fmt.Errorf("dashhls: 取 %s 的 sidx 失败: %w", t.name, err)
	}
	scale, first, refs, err := parseSIDX(body)
	if err != nil {
		return fmt.Errorf("dashhls: 解析 %s 的 sidx 失败: %w", t.name, err)
	}
	if len(refs) == 0 {
		return fmt.Errorf("dashhls: %s 的 sidx 没有分片", t.name)
	}
	t.scale, t.refs = scale, refs

	// 第一个分片紧接在 sidx 之后（B 站 DASH 的 first_offset 恒为 0，但仍按
	// 标准加上，别的稿件若给了非零值也不会错位）。
	t.origin = idx[1] + 1 + int64(first)
	var total int64
	var ticks uint64
	for _, r := range refs {
		total += int64(r.size)
		ticks += uint64(r.dur)
	}
	t.media = [2]int64{t.origin, t.origin + total - 1}
	t.dur = float64(ticks) / float64(scale)
	// 分片起点表：mediaPlaylist 与预取共用同一份顺序，避免两处各算一遍
	// 漂开（预取错一个分片就是白白多下一段）。
	t.starts = make([]int64, len(refs))
	off := t.origin
	for i, r := range refs {
		t.starts[i] = off
		off += int64(r.size)
	}
	return nil
}

// fetch 取一个闭区间。先试主地址，失败再试备用地址（B 站通常给 3 条）。
// 错误里不带签名 URL：那是会过期的凭证，进了日志就是泄漏。
func (t *track) fetch(start, end int64) ([]byte, error) {
	if end < start {
		return nil, fmt.Errorf("区间 %d-%d 无效", start, end)
	}
	if end-start+1 > maxSegmentBytes {
		return nil, fmt.Errorf("区间 %d-%d 过大", start, end)
	}
	urls := append([]string{t.src.BaseURL}, t.src.BackupURLs...)
	var lastErr error
	for _, u := range urls {
		if u == "" {
			continue
		}
		body, err := t.fetchOne(u, start, end)
		if err == nil {
			t.fetches.Add(1)
			return body, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("没有可用地址")
	}
	return nil, lastErr
}

func (t *track) fetchOne(u string, start, end int64) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, errors.New("地址无效")
	}
	req.Header.Set("User-Agent", uaChrome131)
	req.Header.Set("Referer", referer)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, errors.New("请求失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	want := end - start + 1
	// 多读 1 字节：多出来说明上游忽略了 Range（回了整条），宁可报错也不要
	// 把整条轨道写进内存。
	body, err := io.ReadAll(io.LimitReader(resp.Body, want+1))
	if err != nil {
		return nil, errors.New("读取失败")
	}
	if int64(len(body)) != want {
		return nil, fmt.Errorf("上游字节数不符（想要 %d，得到 %d）", want, len(body))
	}
	return body, nil
}

// serveMaster 输出 master 播放列表：音轨单独一组，视频流引用它。
//
// CODECS 带上真实 codec：部分播放器（AVPlayer）据此挑解码器，缺了会直接
// 拒绝播放。
func (s *Session) serveMaster(w http.ResponseWriter, r *http.Request) {
	codecs := s.vid.src.Codecs
	if s.aud.src.Codecs != "" {
		codecs += "," + s.aud.src.Codecs
	}
	bw := s.vid.src.Bandwidth + s.aud.src.Bandwidth
	if bw <= 0 {
		bw = 300000
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n")
	b.WriteString("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"aud\",NAME=\"aac\",DEFAULT=YES,AUTOSELECT=YES,URI=\"audio.m3u8\"\n")
	// 只写**一条** variant：以前写了「有 RESOLUTION / 没有 RESOLUTION」两条
	// 指向同一个 video.m3u8，部分播放器会把它当成两个清晰度去挑，挑中哪条
	// 全看实现；对本地单码率源没有任何意义，只会增加解析歧义。
	fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d", bw)
	if s.vid.src.Width > 0 && s.vid.src.Height > 0 {
		fmt.Fprintf(&b, ",RESOLUTION=%dx%d", s.vid.src.Width, s.vid.src.Height)
	}
	fmt.Fprintf(&b, ",CODECS=\"%s\",AUDIO=\"aud\"\n", codecs)
	b.WriteString("video.m3u8\n")
	writePlaylist(w, b.String())
}

// serveMedia 输出某条轨道的分片列表。
func (s *Session) serveMedia(kind byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t := s.vid
		if kind == 'a' {
			t = s.aud
		}
		writePlaylist(w, mediaPlaylist(t))
	}
}

// segmentStarts 返回每个分片的字节起点。load 会把它算好放进 starts；
// 直接构造 track 的单元测试没有这一步，这里按 origin + refs 现算一份，
// 保证播放列表与预取用的是同一套顺序。
func (t *track) segmentStarts() []int64 {
	t.startsOnce.Do(func() {
		if len(t.starts) == len(t.refs) {
			return
		}
		out := make([]int64, len(t.refs))
		off := t.origin
		for i, r := range t.refs {
			out[i] = off
			off += int64(r.size)
		}
		t.starts = out
	})
	return t.starts
}

// indexOf 返回以 start 开头的分片序号；start 不是分片起点时返回 -1
// （init 段、播放器自己拼出来的区间都不进预取）。
func (t *track) indexOf(start int64) int {
	starts := t.segmentStarts()
	lo, hi := 0, len(starts)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if starts[mid] < start {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(starts) && starts[lo] == start {
		return lo
	}
	return -1
}

// cached 取一份已经预取 / 回过的分片。
func (t *track) cached(start int64) ([]byte, bool) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	body, ok := t.cache[start]
	return body, ok
}

// store 缓存一份分片；超过 maxPrefetchBytes 时按写入顺序淘汰最旧的
// （FIFO 足够：播放是顺序的，最旧的就是最不可能再回看的）。
func (t *track) store(start int64, body []byte) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	if _, ok := t.cache[start]; ok {
		return
	}
	if t.cache == nil {
		t.cache = map[int64][]byte{}
	}
	t.cache[start] = body
	t.cacheOrder = append(t.cacheOrder, start)
	t.cacheBytes += len(body)
	for t.cacheBytes > maxPrefetchBytes && len(t.cacheOrder) > 0 {
		old := t.cacheOrder[0]
		t.cacheOrder = t.cacheOrder[1:]
		if b, ok := t.cache[old]; ok {
			t.cacheBytes -= len(b)
			delete(t.cache, old)
		}
	}
}

// stopped 报告会话是否已经关闭。
func (t *track) stopped() bool {
	if t.stop == nil {
		return false
	}
	select {
	case <-t.stop:
		return true
	default:
		return false
	}
}

// warm 在起服务时就把头两个分片拉下来（见 Start）。与 maybePrefetch 共用
// 同一个 CAS：播放器已经开始要分片时不再重复预取。
func (t *track) warm() {
	if !t.prefetching.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer t.prefetching.Store(false)
		starts := t.segmentStarts()
		for i := 0; i < 2 && i < len(t.refs); i++ {
			if t.stopped() {
				return
			}
			s := starts[i]
			if _, ok := t.cached(s); ok {
				continue
			}
			e := s + int64(t.refs[i].size) - 1
			body, err := t.fetch(s, e)
			if err != nil {
				return
			}
			if t.stopped() {
				return
			}
			t.store(s, body)
		}
	}()
}

// maybePrefetch 在当前分片回完之后，后台把后面 prefetchSegments 个拉下来。
//
// 同一时刻每条轨道最多一个预取 goroutine（CAS）：播放器一秒可能要几个分片，
// 不能每个请求都再开一条。预取失败就停，不重试 —— 播放器自己马上会来要
// 这一段，到时的失败路径与没有预取完全一样。
func (t *track) maybePrefetch(start int64) {
	if !t.prefetching.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer t.prefetching.Store(false)
		starts := t.segmentStarts()
		idx := t.indexOf(start)
		if idx < 0 {
			return
		}
		for i := idx + 1; i <= idx+prefetchSegments && i < len(t.refs); i++ {
			if t.stopped() {
				return
			}
			s := starts[i]
			if _, ok := t.cached(s); ok {
				continue
			}
			e := s + int64(t.refs[i].size) - 1
			body, err := t.fetch(s, e)
			if err != nil {
				return
			}
			if t.stopped() {
				return
			}
			t.store(s, body)
		}
	}()
}

// mediaPlaylist 把一条轨道渲染成 HLS 媒体播放列表。
//
// 分片地址里直接带字节区间（"seg?k=v&r=13586-93585"）。**不能用
// #EXT-X-BYTERANGE**：GStreamer 的 hlsdemux 会忽略它，请求回来的是不带
// Range 的整条流（实测直接退化成把整个文件下完）。区间放查询串里，我们
// 自己按它去上游要那一段，三个平台的播放器都只看到普通 URL。
func mediaPlaylist(t *track) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n")
	// TARGETDURATION 必须 >= 最长分片，否则部分播放器会拒收。
	var longest float64
	for _, r := range t.refs {
		if d := float64(r.dur) / float64(t.scale); d > longest {
			longest = d
		}
	}
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", int(math.Ceil(longest)))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	fmt.Fprintf(&b, "#EXT-X-MAP:URI=\"seg?k=%c&r=%d-%d\"\n", t.kind, t.init[0], t.init[1])

	starts := t.segmentStarts()
	for i, r := range t.refs {
		off := starts[i]
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n", float64(r.dur)/float64(t.scale))
		fmt.Fprintf(&b, "seg?k=%c&r=%d-%d\n", t.kind, off, off+int64(r.size)-1)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

// serveSegment 按查询串里的区间去上游取那一段，原样回给播放器。
//
// 回 200 而不是 206：这是一段独立的分片（不是对整条资源的部分响应），
// 播放器拿到 Content-Length 就够。带 Content-Range 反而会让某些解复用器
// 以为自己在读一个更大的文件。
func (s *Session) serveSegment(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := q.Get("k")
	t := s.vid
	ctype := "video/mp4"
	if kind == "a" {
		t, ctype = s.aud, "audio/mp4"
	} else if kind != "v" {
		http.Error(w, "bad k", http.StatusBadRequest)
		return
	}
	rng, err := parseRange(q.Get("r"))
	if err != nil {
		http.Error(w, "bad r", http.StatusBadRequest)
		return
	}
	start, end := rng[0], rng[1]
	// 夹到这条轨道自己的媒体区间：播放器有时会多要一点（例如最后一个分片
	// 之后），越界就直接 416，不去上游白跑一趟。
	//
	// 但 #EXT-X-MAP（init 段）是**在 media 区间之前**的，必须放行：hlsdemux
	// 取 init 时用的就是这一条 URL，把它按分片越界拒掉会让整份播放列表起不来
	// （实测 GStreamer 报 "Requested Range Not Satisfiable (416)"）。
	inInit := start >= t.init[0] && end <= t.init[1]
	if !inInit && (start < t.media[0] || end > t.media[1] || end < start) {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", t.media[1]-t.media[0]+1))
		http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	// 只接受**整段命中**：播放器偶尔会要一段区间里的一部分，缓存里那份是
	// 完整分片，长度对不上就不能拿它冒充（Content-Length 会与请求不符）。
	body, ok := t.cached(start)
	if ok && int64(len(body)) != end-start+1 {
		ok = false
	}
	if !ok {
		var err error
		body, err = t.fetch(start, end)
		if err != nil {
			// 不把上游错误原文回给播放器（里面可能有签名 URL 的片段）。
			http.Error(w, "upstream failed", http.StatusBadGateway)
			return
		}
		if !inInit && t.indexOf(start) >= 0 {
			// 只缓存完整分片（见上：缓存按分片起点索引）。
			t.store(start, body)
		}
	}
	h := w.Header()
	h.Set("Content-Type", ctype)
	h.Set("Content-Length", strconv.Itoa(len(body)))
	// 分片在一次会话里是不可变的：允许播放器 / 中间层缓存，回拖与 seek
	// 回来就不用再走一趟上游。播放列表仍是 no-store（它带的是本次会话的
	// 签名地址）。
	h.Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
	if !inInit {
		// 回完这一段再预取后面几个：不占播放器这条请求的时间。
		t.maybePrefetch(start)
	}
}

// writePlaylist 输出一份 m3u8。
func writePlaylist(w http.ResponseWriter, body string) {
	h := w.Header()
	h.Set("Content-Type", "application/vnd.apple.mpegurl")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

// parseRange 解析 "0-933" 这种闭区间。空串、顺序反了、含非数字都算无效。
func parseRange(s string) ([2]int64, error) {
	var out [2]int64
	i := strings.IndexByte(s, '-')
	if i <= 0 || i == len(s)-1 {
		return out, fmt.Errorf("区间格式 %q 无效", s)
	}
	a, err := strconv.ParseInt(strings.TrimSpace(s[:i]), 10, 64)
	if err != nil {
		return out, fmt.Errorf("区间起点无效: %w", err)
	}
	b, err := strconv.ParseInt(strings.TrimSpace(s[i+1:]), 10, 64)
	if err != nil {
		return out, fmt.Errorf("区间终点无效: %w", err)
	}
	if a < 0 || b < a {
		return out, fmt.Errorf("区间 %q 无效", s)
	}
	out[0], out[1] = a, b
	return out, nil
}

// parseSIDX 解析 sidx 盒，返回 timescale、first_offset 与全部 reference。
//
// 只看第一个 sidx：B 站每条轨道只有一条音/视频轨，sidx 也就一个。
// reference_type=1 表示「分层引用」（引用的不是媒体而是别的 sidx），B 站
// 不用，遇到了直接报错而不是猜。
func parseSIDX(body []byte) (scale uint32, firstOffset uint64, refs []reference, err error) {
	off := 0
	for off+8 <= len(body) {
		size := int(be32(body, off))
		typ := string(body[off+4 : off+8])
		hdr := 8
		if size == 1 {
			if off+16 > len(body) {
				return 0, 0, nil, errors.New("盒长度被截断")
			}
			size = int(be64(body, off+8))
			hdr = 16
		} else if size == 0 {
			size = len(body) - off
		}
		if size < hdr || off+size > len(body) {
			return 0, 0, nil, errors.New("盒长度越界")
		}
		if typ == "sidx" {
			return parseSIDXBody(body[off+hdr : off+size])
		}
		off += size
	}
	return 0, 0, nil, errors.New("没有 sidx")
}

func parseSIDXBody(b []byte) (scale uint32, firstOffset uint64, refs []reference, err error) {
	if len(b) < 4 {
		return 0, 0, nil, errors.New("sidx 太短")
	}
	version := b[0]
	if version > 1 {
		return 0, 0, nil, fmt.Errorf("sidx version %d 不支持", version)
	}
	p := 4 // version + flags
	if len(b) < p+8 {
		return 0, 0, nil, errors.New("sidx 缺少 timescale")
	}
	p += 4 // reference_ID
	scale = be32(b, p)
	p += 4
	if scale == 0 {
		return 0, 0, nil, errors.New("sidx timescale 为 0")
	}
	if version == 0 {
		if len(b) < p+8 {
			return 0, 0, nil, errors.New("sidx 头被截断")
		}
		p += 4 // earliest_presentation_time
		firstOffset = uint64(be32(b, p))
		p += 4
	} else {
		if len(b) < p+16 {
			return 0, 0, nil, errors.New("sidx 头被截断")
		}
		p += 8 // earliest_presentation_time
		firstOffset = be64(b, p)
		p += 8
	}
	if len(b) < p+4 {
		return 0, 0, nil, errors.New("sidx 缺少 reference 数量")
	}
	p += 2 // reserved
	count := int(be16(b, p))
	p += 2
	if count == 0 {
		return 0, 0, nil, errors.New("sidx 没有 reference")
	}
	if len(b) < p+count*12 {
		return 0, 0, nil, errors.New("sidx reference 表被截断")
	}
	refs = make([]reference, 0, count)
	for i := 0; i < count; i++ {
		w0 := be32(b, p)
		w1 := be32(b, p+4)
		p += 12 // 第三字是 SAP 信息，用不到
		if w0&0x80000000 != 0 {
			return 0, 0, nil, errors.New("sidx 用了分层引用")
		}
		refs = append(refs, reference{size: w0 & 0x7fffffff, dur: w1})
	}
	return scale, firstOffset, refs, nil
}

func be16(b []byte, o int) uint16 {
	return uint16(b[o])<<8 | uint16(b[o+1])
}

func be32(b []byte, o int) uint32 {
	return uint32(b[o])<<24 | uint32(b[o+1])<<16 | uint32(b[o+2])<<8 | uint32(b[o+3])
}

func be64(b []byte, o int) uint64 {
	return uint64(be32(b, o))<<32 | uint64(be32(b, o+4))
}
