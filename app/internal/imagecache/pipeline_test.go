package imagecache

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestPipelineSlotHeldThroughDecode 钉住整条抓取管线共用一个有界名额：
// 第一张图已下载完、正卡在解码闸上时，第二个请求不得提前发 HTTP。
// 旧实现「网络名额下载完就释放、解码另排队」会让很多压缩字节堆在解码闸前。
func TestPipelineSlotHeldThroughDecode(t *testing.T) {
	oldFetches, oldDecodes := maxConcurrentFetches, maxConcurrentDecodes
	maxConcurrentFetches = 1
	maxConcurrentDecodes = 1

	data := coverPNG()
	firstServed := make(chan struct{}, 1)
	var secondStarted atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/second.png" {
			secondStarted.Store(1)
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
		if r.URL.Path == "/first.png" {
			select {
			case firstServed <- struct{}{}:
			default:
			}
		}
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.Client = srv.Client()
	t.Cleanup(func() {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) && c.Pending() > 0 {
			time.Sleep(2 * time.Millisecond)
		}
		maxConcurrentFetches = oldFetches
		maxConcurrentDecodes = oldDecodes
	})
	// 占满唯一的解码名额：第一张图拿到抓取名额、下载完后会停在这里。
	c.decodeSem <- struct{}{}

	u1 := srv.URL + "/first.png"
	u2 := srv.URL + "/second.png"
	c.Bitmap(u1, WidthCard)
	select {
	case <-firstServed:
	case <-time.After(3 * time.Second):
		<-c.decodeSem // 放行，避免测试退出时留下卡住的 goroutine
		t.Fatal("第一张图没有到达服务器")
	}

	// 第一张图此时应已拿到抓取名额并卡在解码闸前；再发第二张。
	time.Sleep(30 * time.Millisecond)
	c.Bitmap(u2, WidthCard)
	time.Sleep(300 * time.Millisecond)
	if secondStarted.Load() != 0 {
		<-c.decodeSem
		t.Fatal("第一张图还在管线中，第二个请求已提前发 HTTP（名额没有覆盖解码段）")
	}

	// 放开解码：第一张完成后第二张才能拿到名额。
	<-c.decodeSem
	waitIdle(t, c)
	if c.Get(u1, WidthCard) == nil || c.Get(u2, WidthCard) == nil {
		t.Fatalf("放行后两张图未就绪: u1=%v u2=%v", c.Get(u1, WidthCard) != nil, c.Get(u2, WidthCard) != nil)
	}
	if secondStarted.Load() == 0 {
		t.Fatal("放行后第二张图仍未发起")
	}
}
