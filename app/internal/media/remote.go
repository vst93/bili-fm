package media

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// remoteFile 把一个支持 Range 的 HTTP 资源包成 io.ReadSeeker，
// 供 MP4 解封装器随机访问。
//
// B 站的音频是渐进式 MP4，解封装要先读 moov，再按需读 mdat；直接每次
// Read 都发一个请求会慢得没法用，所以这里按块缓冲：一次抓 256KB，
// 顺序读时几乎不产生额外请求。
type remoteFile struct {
	client  *http.Client
	url     string
	headers map[string]string

	// noRange 为真表示上游不支持 Range，只能顺序读。
	noRange bool

	mu    sync.Mutex
	size  int64
	buf   []byte // 当前块
	start int64  // 当前块在资源里的起始偏移
	pos   int64  // 逻辑读位置
	err   error
}

const remoteChunk = 256 << 10

// newRemoteFile 先发一个 Range: bytes=0-0 探出总长度与是否支持 Range。
func newRemoteFile(url string, headers map[string]string) (*remoteFile, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	f := &remoteFile{client: client, url: url, headers: headers, size: -1}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
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
			return nil, errors.New("media: 上游 Range 响应缺少总长度")
		}
	case http.StatusOK:
		// 上游不支持 Range：只能顺序读，长度取 Content-Length。
		if n, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil {
			f.size = n
		}
		f.noRange = true
	default:
		return nil, fmt.Errorf("media: 上游返回 %d", resp.StatusCode)
	}
	return f, nil
}

// Size 返回资源总长度（-1 表示未知）。
func (f *remoteFile) Size() int64 { return f.size }

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

// fillLocked 从 f.pos 起抓一块。
func (f *remoteFile) fillLocked() error {
	req, err := http.NewRequest("GET", f.url, nil)
	if err != nil {
		return err
	}
	for k, v := range f.headers {
		req.Header.Set(k, v)
	}
	if !f.noRange {
		end := f.pos + remoteChunk - 1
		if f.size > 0 && end >= f.size {
			end = f.size - 1
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", f.pos, end))
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("media: 取块失败，上游返回 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	f.buf, f.start = data, f.pos
	return nil
}

func lastIndexByte(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}
