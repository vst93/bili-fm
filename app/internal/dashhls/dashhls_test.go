package dashhls

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- 构造测试用 sidx ---

// sidxBox 拼一个 v0 的 sidx 盒。refs 是 (size, dur) 列表。
func sidxBox(scale uint32, firstOffset uint32, refs [][2]uint32) []byte {
	body := make([]byte, 0, 12+12*len(refs))
	body = append(body, 0, 0, 0, 0) // version 0 + flags
	body = binary.BigEndian.AppendUint32(body, 1)
	body = binary.BigEndian.AppendUint32(body, scale)
	body = binary.BigEndian.AppendUint32(body, 0) // earliest_presentation_time
	body = binary.BigEndian.AppendUint32(body, firstOffset)
	body = append(body, 0, 0) // reserved
	body = binary.BigEndian.AppendUint16(body, uint16(len(refs)))
	for _, r := range refs {
		body = binary.BigEndian.AppendUint32(body, r[0]&0x7fffffff)
		body = binary.BigEndian.AppendUint32(body, r[1])
		body = binary.BigEndian.AppendUint32(body, 0x90000000) // SAP
	}
	out := binary.BigEndian.AppendUint32(nil, uint32(8+len(body)))
	out = append(out, 's', 'i', 'd', 'x')
	return append(out, body...)
}

// sidxBoxV1 拼一个 v1 的 sidx（64 位时间/偏移）。
func sidxBoxV1(scale uint32, firstOffset uint64, refs [][2]uint32) []byte {
	body := make([]byte, 0, 20+12*len(refs))
	body = append(body, 1, 0, 0, 0)
	body = binary.BigEndian.AppendUint32(body, 1)
	body = binary.BigEndian.AppendUint32(body, scale)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, firstOffset)
	body = append(body, 0, 0)
	body = binary.BigEndian.AppendUint16(body, uint16(len(refs)))
	for _, r := range refs {
		body = binary.BigEndian.AppendUint32(body, r[0]&0x7fffffff)
		body = binary.BigEndian.AppendUint32(body, r[1])
		body = binary.BigEndian.AppendUint32(body, 0x90000000)
	}
	out := binary.BigEndian.AppendUint32(nil, uint32(8+len(body)))
	out = append(out, 's', 'i', 'd', 'x')
	return append(out, body...)
}

// --- sidx 解析 ---

func TestParseSIDXVersion0(t *testing.T) {
	box := sidxBox(16000, 0, [][2]uint32{{100, 80000}, {200, 80000}})
	scale, first, refs, err := parseSIDX(box)
	if err != nil {
		t.Fatal(err)
	}
	if scale != 16000 || first != 0 {
		t.Fatalf("scale=%d first=%d", scale, first)
	}
	if len(refs) != 2 || refs[0].size != 100 || refs[1].dur != 80000 {
		t.Fatalf("refs=%+v", refs)
	}
}

func TestParseSIDXVersion1(t *testing.T) {
	box := sidxBoxV1(44100, 12345, [][2]uint32{{7, 1024}})
	scale, first, refs, err := parseSIDX(box)
	if err != nil {
		t.Fatal(err)
	}
	if scale != 44100 || first != 12345 || len(refs) != 1 || refs[0].size != 7 {
		t.Fatalf("scale=%d first=%d refs=%+v", scale, first, refs)
	}
}

func TestParseSIDXRejectsBadInput(t *testing.T) {
	t.Run("没有 sidx", func(t *testing.T) {
		box := binary.BigEndian.AppendUint32(nil, 8)
		box = append(box, 'm', 'o', 'o', 'v')
		if _, _, _, err := parseSIDX(box); err == nil {
			t.Fatal("应当报错")
		}
	})
	t.Run("分层引用", func(t *testing.T) {
		box := sidxBox(16000, 0, [][2]uint32{{100, 80000}})
		// 把第一条 reference 的 reference_type 置 1。
		box[len(box)-12] |= 0x80
		if _, _, _, err := parseSIDX(box); err == nil {
			t.Fatal("分层引用应当报错")
		}
	})
	t.Run("timescale 为 0", func(t *testing.T) {
		box := sidxBox(0, 0, [][2]uint32{{1, 1}})
		if _, _, _, err := parseSIDX(box); err == nil {
			t.Fatal("timescale=0 应当报错")
		}
	})
	t.Run("reference 表被截断", func(t *testing.T) {
		box := sidxBox(16000, 0, [][2]uint32{{100, 80000}, {200, 80000}})
		box = box[:len(box)-8]
		// 修回盒长度，让它成为一个「合法但短」的盒，走到 reference 表检查。
		binary.BigEndian.PutUint32(box, uint32(len(box)))
		if _, _, _, err := parseSIDX(box); err == nil {
			t.Fatal("截断应当报错")
		}
	})
	t.Run("空 buffer", func(t *testing.T) {
		if _, _, _, err := parseSIDX(nil); err == nil {
			t.Fatal("空 buffer 应当报错")
		}
	})
	t.Run("version 2", func(t *testing.T) {
		box := sidxBox(16000, 0, [][2]uint32{{1, 1}})
		box[8] = 2
		if _, _, _, err := parseSIDX(box); err == nil {
			t.Fatal("version 2 应当报错")
		}
	})
}

// --- 区间解析 ---

func TestParseRange(t *testing.T) {
	ok := []struct {
		in   string
		want [2]int64
	}{
		{"0-933", [2]int64{0, 933}},
		{"934-13585", [2]int64{934, 13585}},
		{"0-0", [2]int64{0, 0}},
	}
	for _, c := range ok {
		got, err := parseRange(c.in)
		if err != nil || got != c.want {
			t.Errorf("parseRange(%q)=%v,%v 想 %v", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"", "-", "1", "1-", "-5", "5-1", "a-b", "1-2-3"} {
		if _, err := parseRange(in); err == nil {
			t.Errorf("parseRange(%q) 应当报错", in)
		}
	}
}

// --- 播放列表 ---

func testTrack(kind byte) *track {
	return &track{
		kind:   kind,
		scale:  16000,
		init:   [2]int64{0, 933},
		origin: 13586,
		media:  [2]int64{13586, 13586 + 100 + 200 - 1},
		refs:   []reference{{size: 100, dur: 80000}, {size: 200, dur: 80000}},
		src:    Track{Codecs: "avc1.640033", Bandwidth: 202553, Width: 640, Height: 360},
	}
}

func TestMediaPlaylist(t *testing.T) {
	got := mediaPlaylist(testTrack('v'))
	for _, want := range []string{
		"#EXTM3U",
		"#EXT-X-VERSION:7",
		"#EXT-X-TARGETDURATION:5", // 80000/16000 = 5s
		"#EXT-X-PLAYLIST-TYPE:VOD",
		`#EXT-X-MAP:URI="seg?k=v&r=0-933"`,
		"#EXTINF:5.000000,",
		"seg?k=v&r=13586-13685", // 第一片：origin .. origin+99
		"seg?k=v&r=13686-13885", // 第二片：接上
		"#EXT-X-ENDLIST",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("播放列表缺少 %q\n---\n%s", want, got)
		}
	}
	// 绝不能出现 EXT-X-BYTERANGE：hlsdemux 会忽略它，请求会退化成整条下载。
	if strings.Contains(got, "BYTERANGE") {
		t.Error("不能用 EXT-X-BYTERANGE：hlsdemux 会忽略它")
	}
}

func TestMediaPlaylistTargetDurationRoundsUp(t *testing.T) {
	tr := testTrack('v')
	tr.refs = []reference{{size: 10, dur: 80001}} // 5.0000625s
	got := mediaPlaylist(tr)
	if !strings.Contains(got, "#EXT-X-TARGETDURATION:6") {
		t.Errorf("TARGETDURATION 必须向上取整到 6\n%s", got)
	}
}

// --- 本地 HTTP 服务（用 httptest 替上游） ---

// upstream 是一个假的 CDN：按 Range 返回填充字节，并记录收到的请求。
type upstream struct {
	srv      *httptest.Server
	mu       sync.Mutex
	ranges   []string
	newConns int32
	fail     int32 // 剩余失败次数（回 500）
	body     []byte
	// closeAfter 为真时每个响应都带 Connection: close，用来证明我们不是靠
	// 连接复用才碰巧对的。
	closeAfter bool
}

func newUpstream(t *testing.T, size int) *upstream {
	t.Helper()
	u := &upstream{body: make([]byte, size)}
	for i := range u.body {
		u.body[i] = byte(i % 251)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		u.ranges = append(u.ranges, r.Header.Get("Range"))
		u.mu.Unlock()
		if atomic.AddInt32(&u.fail, -1) >= 0 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		if r.Header.Get("User-Agent") != uaChrome131 {
			t.Errorf("缺少 UA：%q", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("Referer") != referer {
			t.Errorf("缺少 Referer：%q", r.Header.Get("Referer"))
		}
		rng := r.Header.Get("Range")
		start, end := int64(0), int64(len(u.body)-1)
		if rng != "" {
			var a, b int64
			if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &a, &b); err == nil {
				start, end = a, b
			}
		}
		if start > int64(len(u.body)-1) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end > int64(len(u.body)-1) {
			end = int64(len(u.body) - 1)
		}
		if u.closeAfter {
			w.Header().Set("Connection", "close")
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(u.body[start : end+1])
	})
	u.srv = httptest.NewServer(mux)
	t.Cleanup(u.srv.Close)
	return u
}

func (u *upstream) rangeCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.ranges)
}

// startWith 用给定的上游起一个 Session。init 区间放在 body 开头，sidx 紧随
// 其后，分片数据在最后，这样 media 区间是真实存在的。
func startWith(t *testing.T, up *upstream, scale uint32, refs [][2]uint32) *Session {
	t.Helper()
	sidx := sidxBox(scale, 0, refs)
	// 布局：[init 0-99][sidx 100-...][media ...]
	initEnd := 99
	sidxStart := int64(initEnd + 1)
	sidxEnd := sidxStart + int64(len(sidx)) - 1
	total := 0
	for _, r := range refs {
		total += int(r[0])
	}
	if need := int(sidxEnd) + 1 + total; need > len(up.body) {
		up.body = append(up.body, make([]byte, need-len(up.body))...)
	}
	copy(up.body[sidxStart:], sidx)

	mk := func(kind byte, url string, bw int) Track {
		return Track{
			BaseURL:    url + "/",
			InitRange:  "0-99",
			IndexRange: fmt.Sprintf("%d-%d", sidxStart, sidxEnd),
			Codecs:     "avc1.640033",
			Bandwidth:  bw,
			Width:      640,
			Height:     360,
		}
	}
	s, err := Start(mk('v', up.srv.URL, 200000), mk('a', up.srv.URL, 80000), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSessionPlaylistsAndSegments(t *testing.T) {
	up := newUpstream(t, 64*1024)
	s := startWith(t, up, 16000, [][2]uint32{{1000, 80000}, {2000, 80000}})

	if !strings.HasSuffix(s.URL(), "/master.m3u8") {
		t.Fatalf("URL=%q", s.URL())
	}
	if got := s.Duration(); got != 10 {
		t.Errorf("duration=%v，想 10", got)
	}

	master := get(t, s.URL())
	for _, want := range []string{
		"#EXTM3U",
		`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="aud"`,
		`URI="audio.m3u8"`,
		`AUDIO="aud"`,
		"video.m3u8",
		"RESOLUTION=640x360",
		"CODECS=",
	} {
		if !strings.Contains(master, want) {
			t.Errorf("master 缺少 %q\n%s", want, master)
		}
	}

	base := strings.TrimSuffix(s.URL(), "master.m3u8")
	video := get(t, base+"video.m3u8")
	audio := get(t, base+"audio.m3u8")
	if !strings.Contains(video, "k=v") {
		t.Error("视频播放列表的分片应带 k=v")
	}
	if !strings.Contains(audio, "k=a") {
		t.Error("音频播放列表的分片应带 k=a")
	}

	// 分片：按播放列表里的区间逐个取，必须拿到精确长度且内容与上游一致。
	rngs := segmentRanges(video)
	if len(rngs) != 2 {
		t.Fatalf("分片数=%d，想 2\n%s", len(rngs), video)
	}
	for _, r := range rngs {
		body := getBytes(t, base+"seg?k=v&r="+r)
		if len(body) == 0 {
			t.Fatalf("分片 %s 是空的", r)
		}
	}
}

func TestSegmentHonoursUpstreamRangeAndKeepsConnection(t *testing.T) {
	up := newUpstream(t, 64*1024)
	s := startWith(t, up, 16000, [][2]uint32{{1000, 80000}, {2000, 80000}})
	base := strings.TrimSuffix(s.URL(), "master.m3u8")

	// 记录我们自己的 client → upstream 之间新建了几条连接。
	var newConns int32
	s.vid.client.Transport.(*http.Transport).DialContext = tracedDial(&newConns)
	s.aud.client.Transport.(*http.Transport).DialContext = tracedDial(&newConns)
	before := atomic.LoadInt32(&newConns)

	rngs := segmentRanges(get(t, base+"video.m3u8"))
	for i := 0; i < 3; i++ { // 多取几轮，确保连接被复用
		for _, r := range rngs {
			getBytes(t, base+"seg?k=v&r="+r)
		}
	}
	// 第一次取分片时可能已经有一条空闲连接（Start 时取 sidx 建的），所以
	// 允许 +1；关键是不能每个分片都新建一条。
	if got := atomic.LoadInt32(&newConns) - before; got > 1 {
		t.Errorf("取 %d 个分片新建了 %d 条连接；必须复用 keep-alive", len(rngs)*3, got)
	}

	// 上游收到的必须是精确区间，不能是整条。
	up.mu.Lock()
	got := append([]string(nil), up.ranges...)
	up.mu.Unlock()
	for _, r := range got {
		if r == "" {
			t.Error("有请求没带 Range（会把整条轨道拉下来）")
		}
	}
}

func TestSegmentRangeOutOfBounds(t *testing.T) {
	up := newUpstream(t, 64*1024)
	s := startWith(t, up, 16000, [][2]uint32{{1000, 80000}})
	base := strings.TrimSuffix(s.URL(), "master.m3u8")

	resp, err := http.Get(base + "seg?k=v&r=999999-1000000")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("越界应回 416，得到 %d", resp.StatusCode)
	}

	bad, err := http.Get(base + "seg?k=zzz&r=0-1")
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("k 非法应回 400，得到 %d", bad.StatusCode)
	}
}

func TestSegmentFallsBackToBackupURL(t *testing.T) {
	good := newUpstream(t, 64*1024)
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer bad.Close()

	sidx := sidxBox(16000, 0, [][2]uint32{{1000, 80000}})
	sidxStart := int64(100)
	sidxEnd := sidxStart + int64(len(sidx)) - 1
	copy(good.body[sidxStart:], sidx)

	tr := Track{
		BaseURL:    bad.URL + "/",
		BackupURLs: []string{good.srv.URL + "/"},
		InitRange:  "0-99",
		IndexRange: fmt.Sprintf("%d-%d", sidxStart, sidxEnd),
		Bandwidth:  1,
	}
	// 主地址全挂：sidx 也会失败 → Start 直接报错。所以让主地址只在 sidx 之后挂。
	var served int32
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&served, 1) == 1 {
			// 第一条（sidx）照常，让 Start 成功。
			rng := r.Header.Get("Range")
			var a, b int64
			fmt.Sscanf(rng, "bytes=%d-%d", &a, &b)
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(good.body[a : b+1])
			return
		}
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer failing.Close()
	tr.BaseURL = failing.URL + "/"

	s, err := Start(tr, tr, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := strings.TrimSuffix(s.URL(), "master.m3u8")
	rngs := segmentRanges(get(t, base+"video.m3u8"))
	body := getBytes(t, base+"seg?k=v&r="+rngs[0])
	if len(body) == 0 {
		t.Fatal("备用地址没生效：分片是空的")
	}
}

func TestCloseIsIdempotentAndStopsServing(t *testing.T) {
	up := newUpstream(t, 64*1024)
	s := startWith(t, up, 16000, [][2]uint32{{100, 80000}})
	url := s.URL()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("重复 Close 应无害：%v", err)
	}
	if _, err := http.Get(url); err == nil {
		t.Error("Close 之后不该还能访问")
	}
}

func TestStartRejectsBadTracks(t *testing.T) {
	up := newUpstream(t, 4096)
	cases := []struct {
		name string
		v, a Track
	}{
		{"缺少地址", Track{InitRange: "0-99", IndexRange: "100-200"}, Track{InitRange: "0-99", IndexRange: "100-200"}},
		{"区间非法", Track{BaseURL: up.srv.URL, InitRange: "x", IndexRange: "100-200"},
			Track{BaseURL: up.srv.URL, InitRange: "0-99", IndexRange: "100-200"}},
	}
	for _, c := range cases {
		if _, err := Start(c.v, c.a, 0); err == nil {
			t.Errorf("%s：应当报错", c.name)
		}
	}
}

// --- 小工具 ---

// tracedDial 包一层 DialContext，数一数新建了几条 TCP 连接（keep-alive 的
// 反面证据）。
func tracedDial(newConns *int32) func(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		atomic.AddInt32(newConns, 1)
		return d.DialContext(ctx, network, addr)
	}
}

func get(t *testing.T, url string) string {
	t.Helper()
	return string(getBytes(t, url))
}

func getBytes(t *testing.T, url string) []byte {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d %s", url, resp.StatusCode, body)
	}
	return body
}

// segmentRanges 从媒体播放列表里抽出所有分片区间（跳过 EXT-X-MAP）。
func segmentRanges(pl string) []string {
	var out []string
	for _, line := range strings.Split(pl, "\n") {
		if !strings.HasPrefix(line, "seg?") || strings.Contains(line, "EXT-X-MAP") {
			continue
		}
		if i := strings.Index(line, "r="); i >= 0 {
			out = append(out, line[i+2:])
		}
	}
	return out
}

var _ = time.Second
