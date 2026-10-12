package imagecache

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// coverPNG 是一张很小的封面图。
func coverPNG() []byte {
	im := image.NewRGBA(image.Rect(0, 0, 24, 15))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = 200, 100, 50, 255
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}

// countServer 记录每个路径被请求的次数。
func countServer(t *testing.T) (*httptest.Server, func(path string) int) {
	t.Helper()
	var mu sync.Mutex
	hits := map[string]int{}
	data := coverPNG()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return hits[path]
	}
}

// TestGetDoesNotFetch 钉住 Get 是纯读缓存：没加载过的 URL 不会发起抓取。
// 懒加载的构建路径靠它保证「屏外绝不 decode」。
func TestGetDoesNotFetch(t *testing.T) {
	srv, hits := countServer(t)
	c := New(nil)
	if b := c.Get(srv.URL+"/never.png", WidthCard); b != nil {
		t.Fatal("未加载的 URL，Get 应返回 nil")
	}
	time.Sleep(50 * time.Millisecond)
	if n := hits("/never.png"); n != 0 {
		t.Fatalf("Get 不应发起抓取，却请求了 %d 次", n)
	}
}

// TestBitmapAsyncDoesNotBlockThenCaches 钉住两件事：
//  1. Bitmap 第一次调用立刻返回 nil（异步抓取），绝不阻塞渲染帧；
//  2. 同一 URL 只抓取 / decode 一次，之后命中缓存返回同一张位图。
func TestBitmapAsyncDoesNotBlockThenCaches(t *testing.T) {
	srv, hits := countServer(t)
	notified := atomic.Int32{}
	c := New(func() { notified.Add(1) })
	u := srv.URL + "/a.png"

	start := time.Now()
	if b := c.Bitmap(u, WidthCard); b != nil {
		t.Fatal("首次 Bitmap 应返回 nil（未就绪，异步抓取）")
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Fatalf("Bitmap 阻塞了 %v，渲染帧里不能同步抓取", d)
	}
	// 请求中：Pending 应计上。
	if p := c.Pending(); p != 1 {
		t.Fatalf("抓取中 Pending = %d, want 1", p)
	}

	// 等它就绪。
	deadline := time.Now().Add(3 * time.Second)
	var bmp *ui.Bitmap
	for time.Now().Before(deadline) {
		if b := c.Bitmap(u, WidthCard); b != nil {
			bmp = b
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if bmp == nil {
		t.Fatal("封面一直没加载好")
	}
	if w, h := bmp.Size(); w != 24 || h != 15 {
		t.Fatalf("位图尺寸 = %dx%d, want 24x15", w, h)
	}
	if p := c.Pending(); p != 0 {
		t.Fatalf("加载完 Pending = %d, want 0", p)
	}
	// 再请求几次：必须命中缓存，不再发 HTTP。
	first := c.Get(u, WidthCard)
	for i := 0; i < 5; i++ {
		if got := c.Bitmap(u, WidthCard); got != first {
			t.Fatal("同一 URL 应返回缓存的同一张位图")
		}
	}
	if n := hits("/a.png"); n != 1 {
		t.Fatalf("同一 URL 被抓取了 %d 次，want 1", n)
	}
	if n := notified.Load(); n == 0 {
		t.Fatal("图片就绪应触发重绘通知")
	}
}

// TestConcurrentSameURLOnceNoPanic 并发请求同一 URL 只抓一次，且不 panic。
func TestConcurrentSameURLOnceNoPanic(t *testing.T) {
	srv, hits := countServer(t)
	c := New(nil)
	u := srv.URL + "/shared.png"
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.Bitmap(u, WidthCard) }()
	}
	wg.Wait()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && c.Get(u, WidthCard) == nil {
		time.Sleep(5 * time.Millisecond)
	}
	if c.Get(u, WidthCard) == nil {
		t.Fatal("并发请求后仍未加载")
	}
	if n := hits("/shared.png"); n != 1 {
		t.Fatalf("并发同一 URL 抓取了 %d 次，want 1", n)
	}
}

// TestAmbientFetchesOwnWidth 钉住：氛围光源图按**自己的宽度**（320）抓取，
// 与播放器封面的 480 是两个缓存条目、两次请求（原版是独立的两张图）。
// 曾经 Ambient 只读 (url, width) 的源缓存，而播放器按 480 取图，(url, 320)
// 永远不存在 → 氛围光调色层永远返回 nil、悄悄退回原图，观感回退到无滤镜。
func TestAmbientFetchesOwnWidth(t *testing.T) {
	srv, hits := countServer(t)
	c := New(nil)
	u := srv.URL + "/cover.png"

	// 播放器封面先按 480 就位（coverBitmap 的行为）。
	c.Bitmap(u, WidthPlayer)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && c.Get(u, WidthPlayer) == nil {
		time.Sleep(2 * time.Millisecond)
	}
	if c.Get(u, WidthPlayer) == nil {
		t.Fatal("480 源图未就绪")
	}
	before := hits("/cover.png") // 测试服务器非 hdslb，无宽度后缀；480 已抓 1 次

	// 氛围光第一次调用：320 源还没抓，返回 nil 但**必须**发起 320 抓取。
	if b := c.Ambient(u, WidthAmbient, false); b != nil {
		t.Fatal("320 源图未就绪时 Ambient 应返回 nil")
	}
	deadline = time.Now().Add(3 * time.Second)
	var got *ui.Bitmap
	for time.Now().Before(deadline) {
		if got = c.Ambient(u, WidthAmbient, false); got != nil {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if got == nil {
		t.Fatal("320 源就绪后 Ambient 仍返回 nil")
	}
	if n := hits("/cover.png"); n != before+1 {
		t.Fatalf("氛围光应自抓 320 源图（总请求 %d 次，want %d：480 一次 + 320 一次）", n, before+1)
	}
	// 再调用：调色结果命中缓存，不再重复抓取/派生。
	if c.Ambient(u, WidthAmbient, false) != got {
		t.Fatal("同 URL 同参数的 Ambient 应命中同一张调色位图")
	}
}
