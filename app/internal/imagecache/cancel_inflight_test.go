package imagecache

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestCancelAbortsInFlightFetch 钉住「滚出视口的封面不再占着抓取名额」：
//
// 之前 Cancel 只退还在排队的请求，已经拿到名额的下载会一直跑到 15 秒超时。
// 慢网 + 快速滚动时，16 个名额全被屏外旧封面占住，滚停后的可见图只能排队，
// 用户看到的就是「图片疯狂闪动加载」。现在 Cancel 直接 abort 在途的 HTTP
// 请求，让名额立刻回到可见图手里；取消不算失败、不写失败冷却。
func TestCancelAbortsInFlightFetch(t *testing.T) {
	started := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
		select {
		case canceled <- struct{}{}:
		default:
		}
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.Client = srv.Client()
	u := srv.URL + "/slow.png"
	if b := c.Bitmap(u, WidthCard); b != nil {
		t.Fatal("首次 Bitmap 应返回 nil")
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("请求一直没到服务器")
	}

	c.Cancel(u, WidthCard)
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("Cancel 没有 abort 在途请求（服务器的 context 没结束）")
	}

	// 条目应从缓存中清掉，Pending 归零，且这次取消不计为失败。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Pending() == 0 && c.State(u, WidthCard) == StateAbsent {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if got := c.State(u, WidthCard); got != StateAbsent {
		t.Fatalf("取消后条目状态 = %v，应为 Absent（滚回来可重新发起）", got)
	}
	if n := c.Pending(); n != 0 {
		t.Fatalf("取消后 Pending = %d，应为 0", n)
	}
	if st := c.Stats(); st.Failed != 0 {
		t.Fatalf("取消被记成失败（Failed=%d），会写失败冷却", st.Failed)
	}
}

// TestAcquireAfterCancelBeforeWaitDoesNotPoisonQueue 钉住 Cancel 在
// 「fetch 已设好 cancel、尚未 acquire」这段窗口里发生时，acquire 不能先给
// 已取消条目加 waitHi/waitLo 再退出：那些计数会永远留在队列上，优先队列
// 一旦被污染，后续普通抓取可能永远等不到名额。
func TestAcquireAfterCancelBeforeWaitDoesNotPoisonQueue(t *testing.T) {
	old := maxConcurrentFetches
	maxConcurrentFetches = 1
	t.Cleanup(func() { maxConcurrentFetches = old })

	srv, hits := countServer(t)
	c := New(nil)
	c.Client = srv.Client()
	u := srv.URL + "/cancelled-before-acquire.png"
	k := key{u, WidthCard}
	e := &entry{pending: true}
	c.mu.Lock()
	c.items[k] = e
	c.order = append(c.order, k)
	c.loading = 1
	c.mu.Unlock()

	// 模拟 Cancel 先于同一 fetch 的 acquire 执行：此时 e.waiting 仍是 false。
	c.Cancel(u, WidthCard)
	if release := c.acquire(e, true); release != nil {
		release()
		t.Fatal("已取消的条目不应拿到抓取名额")
	}
	c.mu.Lock()
	hi, lo, slots := c.waitHi, c.waitLo, c.slots
	c.mu.Unlock()
	if hi != 0 || lo != 0 || slots != 0 {
		t.Fatalf("已取消条目的 acquire 污染了队列：waitHi=%d waitLo=%d slots=%d", hi, lo, slots)
	}

	// 旧 fetch 的收尾路径：只删自己的 entry、只扣自己的 pending。
	c.dropPending(k, e)
	if p := c.Pending(); p != 0 {
		t.Fatalf("取消 + 收尾后 Pending = %d, want 0", p)
	}

	// 下一个普通（非 Touch）抓取必须能正常拿到名额并完成。
	next := srv.URL + "/after-cancel.png"
	c.Bitmap(next, WidthCard)
	waitIdle(t, c)
	if c.Get(next, WidthCard) == nil {
		t.Fatalf("Cancel-before-acquire 之后的普通抓取没有完成: %+v", c.Stats())
	}
	if n := hits("/after-cancel.png"); n != 1 {
		t.Fatalf("普通抓取请求数 = %d, want 1", n)
	}
}
