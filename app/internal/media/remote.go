package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// remoteFile 把一个支持 Range 的 HTTP 资源包成 io.ReadSeeker，
// 供 MP4 解封装器随机访问。
//
// B 站的音频是渐进式 MP4，解封装要先读 moov，再按需读 mdat；直接每次
// Read 都发一个请求会慢得没法用，所以这里按块缓冲：一次抓 512KB，
// 顺序读时几乎不产生额外请求。
//
// **跨块的下一块是异步预取的**：同步取块要等一次 HTTP 往返（B 站 CDN 抖动时
// 几百 ms 到几秒），一旦正好赶上就绪队列见底，听感就是「这一段播完了、下一段
// 还没来」的卡顿。现在每取到一块就顺手把紧随其后的那块在后台抓下来，读到块尾
// 时直接换上；网络抖动只影响「再下一块」，被 5 秒的解码水位吸收。
type remoteFile struct {
	client    *http.Client
	url       string
	headers   map[string]string
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once

	// noRange 为真表示上游不支持 Range，只能顺序读。
	noRange bool

	mu    sync.Mutex
	size  int64
	buf   []byte // 当前块
	start int64  // 当前块在资源里的起始偏移
	pos   int64  // 逻辑读位置
	err   error

	// 异步预取的「下一块」。pfGen 是代号：seek / 换块会让在途的预取结果作废。
	// 锁序：任何地方都先拿 f.mu 再拿 pfMu（预取 goroutine 只拿 pfMu）。
	pfMu    sync.Mutex
	pfStart int64
	pfBuf   []byte
	pfBusy  bool
	pfGen   uint64

	// readHigh 是「已经从远端取到的最高字节偏移」，给进度条的缓冲段用。
	// 用原子量而不是 f.mu：f.mu 在网络读期间是被 held 住的，界面线程
	// 不能为了读一个进度数字去等一次 HTTP 往返（见 PLAN 的「锁里做 I/O」）。
	readHigh atomic.Int64
}

// remoteChunk 是一块的大小。B 站音频 128kbps 时 512KB ≈ 32 秒，块边界很少；
// 顺序读时预取又让跨块不产生同步等待。
const remoteChunk = 512 << 10

// remoteChunkRetries 是同步/预取取块遇到网络错误或 5xx 时的重试次数
// （不含首次）。CDN 抖动是常态，一次失败就整首报错太脆。
const remoteChunkRetries = 2

// newRemoteFile 先发一个 Range: bytes=0-0 探出总长度与是否支持 Range。
func newRemoteFile(url string, headers map[string]string) (*remoteFile, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	f := &remoteFile{client: client, url: url, headers: headers, ctx: ctx, cancel: cancel, size: -1}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, WrapKind(KindNetwork, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	switch resp.StatusCode {
	case http.StatusPartialContent:
		cr := resp.Header.Get("Content-Range")
		if i := lastIndexByte(cr, '/'); i >= 0 {
			if n, err := strconv.ParseInt(cr[i+1:], 10, 64); err == nil {
				f.size = n
			}
		}
		if f.size < 0 {
			cancel()
			return nil, WrapKind(KindNetwork, errors.New("media: 上游 Range 响应缺少总长度"))
		}
	case http.StatusOK:
		// 上游不支持 Range：只能顺序读，长度取 Content-Length。
		if n, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil {
			f.size = n
		}
		f.noRange = true
	default:
		// 最常见的一类：CDN 给了 403/404（链接过期、需要 Referer）。
		cancel()
		return nil, WrapKind(KindNetwork, fmt.Errorf("media: 上游返回 %d", resp.StatusCode))
	}
	return f, nil
}

// Size 返回资源总长度（-1 表示未知）。
func (f *remoteFile) Size() int64 { return f.size }

// Close 取消这个文件所有在途 / 后续的 HTTP 请求，并清掉当前块与预取块。
//
// 切歌时旧 pipeline 会立刻被换掉；如果不取消，上一首的预取请求还会在后台
// 跑最多 30 秒，并各自押着 512KB 块。连续切歌时这些「已经没人听的数据」
// 会叠起来，正是用户观察到的「越播内存越高」的一部分（见 player.close）。
func (f *remoteFile) Close() {
	f.closeOnce.Do(func() {
		if f.cancel != nil {
			f.cancel()
		}
		f.mu.Lock()
		f.buf = nil
		f.err = context.Canceled
		f.mu.Unlock()
		f.pfMu.Lock()
		f.pfGen++
		f.pfBuf = nil
		f.pfStart = -1
		f.pfBusy = false
		f.pfMu.Unlock()
	})
}

func (f *remoteFile) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = f.pos + offset
	case io.SeekEnd:
		if f.size < 0 {
			return 0, errors.New("media: 长度未知，无法从末尾定位")
		}
		abs = f.size + offset
	default:
		return 0, errors.New("media: 非法的 whence")
	}
	if abs < 0 {
		return 0, errors.New("media: 定位到负偏移")
	}
	f.pos = abs
	// 缓冲读数要跟着走：跳到现存块之外时旧数据已经被丢掉，读数必须
	// 回退（否则进度条的缓冲段会显示一段其实已经没有的数据）。
	// 落在现存块内则保持不变——那块数据还在，缓冲确实延伸到它末尾。
	if f.buf == nil || abs < f.start || abs >= f.start+int64(len(f.buf)) {
		f.readHigh.Store(abs)
		// 跳到当前块之外：除非落点正好在「已经预取 / 正在预取」的那块里，
		// 否则在途的预取是给旧位置准备的，作废。
		if !f.prefetchCoversLocked(abs) {
			f.invalidatePrefetchLocked()
		}
	}
	return abs, nil
}

func (f *remoteFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return 0, f.err
	}
	if f.size >= 0 && f.pos >= f.size {
		return 0, io.EOF
	}
	// 命中当前块就直接从里面取。
	if f.buf != nil && f.pos >= f.start && f.pos < f.start+int64(len(f.buf)) {
		off := f.pos - f.start
		n := copy(p, f.buf[off:])
		f.pos += int64(n)
		return n, nil
	}
	if err := f.fillLocked(); err != nil {
		f.err = err
		return 0, err
	}
	if len(f.buf) == 0 {
		return 0, io.EOF
	}
	off := f.pos - f.start
	if off < 0 || off >= int64(len(f.buf)) {
		return 0, io.EOF
	}
	n := copy(p, f.buf[off:])
	f.pos += int64(n)
	return n, nil
}

// fetchChunk 从 start 起抓一块（带重试）。不碰 f 的锁，供同步取块与
// 后台预取共用。
func (f *remoteFile) fetchChunk(start int64) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= remoteChunkRetries; attempt++ {
		if f.ctx.Err() != nil {
			return nil, f.ctx.Err()
		}
		if attempt > 0 {
			// 150ms、400ms：给 CDN 一点恢复时间，又不至于把就绪队列拖穿。
			time.Sleep(time.Duration(150*(1<<uint(attempt-1))) * time.Millisecond)
		}
		data, retry, err := f.fetchChunkOnce(start)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, lastErr
}

// fetchChunkOnce 发一次 Range 请求。retry 表示这个错误值得重试
// （网络错误 / 5xx / 429）；4xx（链接过期 403、404）重试没有意义。
func (f *remoteFile) fetchChunkOnce(start int64) (data []byte, retry bool, err error) {
	req, err := http.NewRequestWithContext(f.ctx, "GET", f.url, nil)
	if err != nil {
		return nil, false, err
	}
	for k, v := range f.headers {
		req.Header.Set(k, v)
	}
	if !f.noRange {
		end := start + remoteChunk - 1
		if f.size > 0 && end >= f.size {
			end = f.size - 1
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, true, WrapKind(KindNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		retry := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusRequestTimeout
		return nil, retry, WrapKind(KindNetwork, fmt.Errorf("media: 取块失败，上游返回 %d", resp.StatusCode))
	}
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, WrapKind(KindNetwork, err)
	}
	return data, false, nil
}

// fillLocked 从 f.pos 起取一块：优先用后台预取好的那块（不产生同步等待），
// 否则同步取，取到之后再把下一块排进预取。
func (f *remoteFile) fillLocked() error {
	if data, ok := f.takePrefetchLocked(); ok {
		f.adoptLocked(data)
		f.prefetchNextLocked()
		return nil
	}
	// 预取没命中（首次、seek 之后、或预取还在路上）：作废在途结果，同步取。
	f.invalidatePrefetchLocked()
	data, err := f.fetchChunk(f.pos)
	if err != nil {
		return err
	}
	f.adoptLocked(data)
	f.prefetchNextLocked()
	return nil
}

// adoptLocked 把一块数据装成当前块（调用方持有 f.mu）。
func (f *remoteFile) adoptLocked(data []byte) {
	f.buf, f.start = data, f.pos
	if end := f.start + int64(len(data)); end > f.readHigh.Load() {
		f.readHigh.Store(end)
	}
}

// takePrefetchLocked 取走「正好覆盖 f.pos」的预取块（调用方持有 f.mu）。
func (f *remoteFile) takePrefetchLocked() ([]byte, bool) {
	f.pfMu.Lock()
	defer f.pfMu.Unlock()
	if f.pfBuf == nil || f.pfStart != f.pos {
		return nil, false
	}
	data := f.pfBuf
	f.pfBuf = nil
	f.pfStart = -1
	return data, true
}

// prefetchNextLocked 把当前块之后的那块排进后台抓取（调用方持有 f.mu）。
func (f *remoteFile) prefetchNextLocked() {
	if f.noRange || f.buf == nil {
		return
	}
	next := f.start + int64(len(f.buf))
	if f.size >= 0 && next >= f.size {
		return
	}
	f.pfMu.Lock()
	if f.pfStart == next && (f.pfBuf != nil || f.pfBusy) {
		f.pfMu.Unlock()
		return
	}
	f.pfGen++
	gen := f.pfGen
	f.pfStart, f.pfBuf, f.pfBusy = next, nil, true
	f.pfMu.Unlock()
	go func() {
		data, err := f.fetchChunk(next)
		if f.ctx.Err() != nil {
			return
		}
		f.pfMu.Lock()
		if gen == f.pfGen {
			f.pfBusy = false
			if err == nil {
				f.pfBuf = data
				if end := next + int64(len(data)); end > f.readHigh.Load() {
					f.readHigh.Store(end)
				}
			} else {
				f.pfStart = -1
			}
		}
		f.pfMu.Unlock()
	}()
}

// prefetchCoversLocked 报告 abs 是否落在预取（已完成或在途）的那块里。
// 调用方持有 f.mu。
func (f *remoteFile) prefetchCoversLocked(abs int64) bool {
	f.pfMu.Lock()
	defer f.pfMu.Unlock()
	if f.pfStart < 0 {
		return false
	}
	if f.pfBuf != nil {
		return abs >= f.pfStart && abs < f.pfStart+int64(len(f.pfBuf))
	}
	return f.pfBusy && abs >= f.pfStart && abs < f.pfStart+remoteChunk
}

// invalidatePrefetchLocked 让在途的预取结果作废（调用方持有 f.mu）。
func (f *remoteFile) invalidatePrefetchLocked() {
	f.pfMu.Lock()
	f.pfGen++
	f.pfBuf = nil
	f.pfStart = -1
	f.pfMu.Unlock()
}

// ReadAhead 返回已从远端取到的最高字节偏移（0..Size）。
// 只读原子量，不拿 f.mu，界面线程可以直接调。
func (f *remoteFile) ReadAhead() int64 { return f.readHigh.Load() }

func lastIndexByte(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}
