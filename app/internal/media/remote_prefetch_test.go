package media

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// rangeServer 是一个支持 Range 的测试源，记录收到的请求数，并可让前 n 次
// 取块请求以 500 失败（探测请求不计入 failChunks）。
type rangeServer struct {
	*httptest.Server
	data      []byte
	requests  atomic.Int64
	failFirst atomic.Int64 // 前 N 次「非探测」请求返回 500
	probeSeen atomic.Bool
	delay     time.Duration // 每个非探测请求的固定延迟（模拟 CDN 抖动）
}

func newRangeServer(t *testing.T, size int) *rangeServer {
	t.Helper()
	rs := &rangeServer{data: make([]byte, size)}
	for i := range rs.data {
		rs.data[i] = byte(i * 7)
	}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.requests.Add(1)
		if r.Header.Get("Range") == "bytes=0-0" && !rs.probeSeen.Swap(true) {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(rs.data)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(rs.data[:1])
			return
		}
		if rs.delay > 0 {
			time.Sleep(rs.delay)
		}
		if n := rs.failFirst.Load(); n > 0 && rs.failFirst.CompareAndSwap(n, n-1) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		start, end := 0, len(rs.data)-1
		if rng := r.Header.Get("Range"); rng != "" {
			if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end); err != nil {
				start, end = 0, len(rs.data)-1
			}
		}
		if start > end || start >= len(rs.data) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		end = min(end, len(rs.data)-1)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(rs.data)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(rs.data[start : end+1])
	}))
	t.Cleanup(rs.Close)
	return rs
}

// waitPrefetch 等到预取块就绪（或超时），避免测试靠 sleep 猜时间。
func waitPrefetch(t *testing.T, f *remoteFile) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		f.pfMu.Lock()
		ready := f.pfBuf != nil
		f.pfMu.Unlock()
		if ready {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("预取块一直没就绪")
}

// TestRemoteFilePrefetchesNextChunk 钉住「跨块不再同步等网络」：
// 取到一块后，下一块在后台抓；读到块边界时直接换上，不再发请求。
func TestRemoteFilePrefetchesNextChunk(t *testing.T) {
	rs := newRangeServer(t, remoteChunk*4)
	rs.delay = 120 * time.Millisecond // 每次取块都要等一次「网络」
	f, err := newRemoteFile(rs.URL, nil)
	if err != nil {
		t.Fatalf("探测失败：%v", err)
	}
	buf := make([]byte, 16)
	if _, err := f.Read(buf); err != nil {
		t.Fatalf("首次读失败：%v", err)
	}
	waitPrefetch(t, f)

	// 读到第二块的开头：预取块已在内存里，应当立刻返回；若退回同步取块，
	// 这里会被服务端的 120ms 延迟拖住。
	if _, err := f.Seek(int64(remoteChunk), io.SeekStart); err != nil {
		t.Fatalf("Seek：%v", err)
	}
	start := time.Now()
	if _, err := f.Read(buf); err != nil {
		t.Fatalf("跨块读失败：%v", err)
	}
	if elapsed := time.Since(start); elapsed > 60*time.Millisecond {
		t.Errorf("跨块读用了 %v：预取没被用上，仍在同步等网络", elapsed)
	}
}

// TestRemoteFilePrefetchSurvivesSeekIntoIt 钉住「跳到预取块里不作废预取」：
// seek 到刚好是预取起点时，结果仍可用。
func TestRemoteFilePrefetchSurvivesSeekIntoIt(t *testing.T) {
	rs := newRangeServer(t, remoteChunk*2)
	f, err := newRemoteFile(rs.URL, nil)
	if err != nil {
		t.Fatalf("探测失败：%v", err)
	}
	if _, err := f.Read(make([]byte, 16)); err != nil {
		t.Fatalf("首次读失败：%v", err)
	}
	waitPrefetch(t, f)
	if _, err := f.Seek(int64(remoteChunk), io.SeekStart); err != nil {
		t.Fatalf("Seek：%v", err)
	}
	f.pfMu.Lock()
	ready := f.pfBuf != nil
	f.pfMu.Unlock()
	if !ready {
		t.Fatal("seek 到预取起点后预取块被丢掉了")
	}
}

// TestRemoteFileRetriesTransientChunkError 钉住「CDN 抖一下不至于整首报错」：
// 首块 500 一次后重试成功。
func TestRemoteFileRetriesTransientChunkError(t *testing.T) {
	rs := newRangeServer(t, remoteChunk*2)
	rs.failFirst.Store(1)
	f, err := newRemoteFile(rs.URL, nil)
	if err != nil {
		t.Fatalf("探测失败：%v", err)
	}
	if _, err := f.Read(make([]byte, 16)); err != nil {
		t.Fatalf("重试后应当读到数据：%v", err)
	}
	if got := rs.requests.Load(); got < 3 {
		t.Errorf("应至少 3 个请求（探测/失败/重试），得到 %d", got)
	}
}

// TestRemoteFileSeekInvalidatesPrefetch 钉住「跳远了以后旧预取不会串音」：
// seek 到别处后，读到的必须是新位置的数据。
func TestRemoteFileSeekInvalidatesPrefetch(t *testing.T) {
	rs := newRangeServer(t, remoteChunk*4)
	f, err := newRemoteFile(rs.URL, nil)
	if err != nil {
		t.Fatalf("探测失败：%v", err)
	}
	buf := make([]byte, 8)
	if _, err := f.Read(buf); err != nil {
		t.Fatalf("首次读失败：%v", err)
	}
	waitPrefetch(t, f)
	far := int64(remoteChunk * 3)
	if _, err := f.Seek(far, io.SeekStart); err != nil {
		t.Fatalf("Seek：%v", err)
	}
	got := make([]byte, 8)
	if _, err := f.Read(got); err != nil {
		t.Fatalf("远跳后读失败：%v", err)
	}
	for i := range got {
		if want := rs.data[far+int64(i)]; got[i] != want {
			t.Fatalf("远跳后读到旧预取数据：偏移 %d 得 %d，want %d", i, got[i], want)
		}
	}
}

// TestRemoteFileCloseReleasesChunks 钉住切歌时的主动释放：关闭后当前块、
// 预取块与在途请求都要马上放掉，不能等整首放完 / GC。
func TestRemoteFileCloseReleasesChunks(t *testing.T) {
	rs := newRangeServer(t, remoteChunk*4)
	f, err := newRemoteFile(rs.URL, nil)
	if err != nil {
		t.Fatalf("探测失败：%v", err)
	}
	if _, err := f.Read(make([]byte, 16)); err != nil {
		t.Fatalf("首次读失败：%v", err)
	}
	waitPrefetch(t, f)
	f.Close()
	f.mu.Lock()
	buf := f.buf
	f.mu.Unlock()
	f.pfMu.Lock()
	pfBuf, busy := f.pfBuf, f.pfBusy
	f.pfMu.Unlock()
	if buf != nil || pfBuf != nil || busy {
		t.Fatalf("Close 后仍持有块：buf=%d prefetch=%d busy=%v", len(buf), len(pfBuf), busy)
	}
	if f.ctx.Err() == nil {
		t.Fatal("Close 没有取消远端请求上下文")
	}
}
