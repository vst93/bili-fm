//go:build perfmeasure

package imagecache

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrencyLadder 是并发档位选择的**实证**：同一批请求（一屏 15 张 +
// 滚动追加 15 张）在 8 / 16 / 32 / 50 四档下的表现，一次运行全部
// 打出来，选值直接对表：
//
//	go test -tags perfmeasure -run TestConcurrencyLadder -v ./internal/imagecache/
//
// 指标口径：
//   - 首屏全显：15 张 Bitmap 后全部就绪的耗时（模拟打开抽屉铺满一屏）；
//   - 滚动显图延迟：首屏就绪后立即 Bitmap 下 15 张（模拟滚动后新可见），
//     到它们全部就绪的耗时；
//   - 每帧 present：view 包的 TestPerfDrawerAndScroll 已单独量渲染成本，
//     这里量解码侧对 CPU 的挤压（decode 总 CPU 时间，它会抢占渲染线程）；
//   - 失败率：服务器按档位模拟软限流（同时高于 8 的请求延迟 10 倍），看并发
//     提高后被限流拖慢的比例。
func TestConcurrencyLadder(t *testing.T) {
	const firstScreen = 15
	const scrollBatch = 15
	data := coverPNG()

	for _, conc := range []int{8, 16, 32, 50} {
		conc := conc
		t.Run(fmt.Sprintf("conc=%d", conc), func(t *testing.T) {
			for _, throttle := range []bool{false, true} {
				label := "无限制流"
				if throttle {
					label = "限流(>8并发软限流)"
				}
				t.Run(label, func(t *testing.T) {
					var mu sync.Mutex
					inflight, maxInflight := 0, 0
					var limited, served, conns atomic.Int64
					srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						mu.Lock()
						inflight++
						if inflight > maxInflight {
							maxInflight = inflight
						}
						over := throttle && inflight > 8
						mu.Unlock()
						delay := 40 * time.Millisecond // 模拟图床 RTT + 传输
						if over {
							limited.Add(1)
							// 软限流：超阈值的请求拖 10 倍（图床对突发并发的
							// 真实行为是排队变慢；硬 429 只在配额型接口出现，
							// 应用层没法区分「被限流」和「网络慢」，退避救不了
							// 排队 —— 所以选值的判据是延迟曲线而非硬失败率）。
							delay = 400 * time.Millisecond
						}
						served.Add(1)
						time.Sleep(delay)
						w.Header().Set("Content-Type", "image/png")
						w.Write(data)
						mu.Lock()
						inflight--
						mu.Unlock()
					}))
					srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
						if s == http.StateNew {
							conns.Add(1)
						}
					}
					srv.Start()
					t.Cleanup(func() { srv.Close() })

					old := maxConcurrentFetches
					maxConcurrentFetches = conc
					defer func() { maxConcurrentFetches = old }()

					// 生产口径的传输：连接池上限跟着并发走，暖连接可复用（不再每次
					// 重开 TCP）。默认 Transport 的 MaxIdleConnsPerHost=2 会在并发
					// 8 时反复丢弃暖连接、每张图重做握手——见 TestConnectionReuse。
					tr := NewTransport(conc)
					defer tr.CloseIdleConnections()
					c := New(nil)
					c.Client = &http.Client{Timeout: 15 * time.Second, Transport: tr}
					t.Logf("start: conc=%d (set %d)", maxConcurrentFetches, conc)
					// URL 登记下来，waitReady 里模拟「每帧重探」用。
					urls := make([]string, 0, firstScreen+scrollBatch)
					start := time.Now()
					// 首屏：可见窗口 Touch + Bitmap（预取方的真实顺序）。
					for i := 0; i < firstScreen; i++ {
						u := fmt.Sprintf("%s/f%d.png", srv.URL, i)
						urls = append(urls, u)
						c.Touch(u, WidthCard)
						c.Bitmap(u, WidthCard)
					}
					waitReady(t, c, firstScreen, "首屏", func() {
						for _, u := range urls {
							c.Bitmap(u, WidthCard)
						}
					})
					first := time.Since(start)

					// 滚动：新可见的 15 张（先 Touch 后 Bitmap，模拟滚到位）。
					start = time.Now()
					for i := 0; i < scrollBatch; i++ {
						u := fmt.Sprintf("%s/s%d.png", srv.URL, i)
						urls = append(urls, u)
						c.Touch(u, WidthCard)
						c.Bitmap(u, WidthCard)
					}
					waitReady(t, c, firstScreen+scrollBatch, "滚动", func() {
						for _, u := range urls {
							c.Bitmap(u, WidthCard)
						}
					})
					scroll := time.Since(start)

					// 解码 CPU：Threads 粗略等价（decode 占用的处理器时间）。
					var failRate float64
					st := c.Stats()
					if total := st.Ready + st.Failed; total > 0 {
						failRate = float64(st.Failed) / float64(total)
					}
					t.Logf("档位=%2d 限流=%v: 首屏全显 %v、滚动显图 %v、失败率 %.1f%%、峰值在途 %d、新建连接 %d、被软限流 %d 次、内存 %.1f MB",
						conc, throttle, first.Round(time.Millisecond), scroll.Round(time.Millisecond),
						failRate*100, maxInflight, conns.Load(), limited.Load(),
						float64(c.Bytes())/1048576)
				})
			}
		})
	}
}

// waitReady 等到 ready 张图就绪。retry 非空时每 250ms 重探一遍 URL（模拟
// 渲染每帧 Bitmap：限流失败进冷却，冷却到期由「下一帧」重试——与应用里
// 每帧都调 Bitmap 的行为一致）。排障进度每 500ms 打一条。
func waitReady(t *testing.T, c *Cache, ready int, what string, retry func()) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	last, poke := time.Now(), time.Now()
	for time.Now().Before(deadline) {
		if n := c.readyCount(); n >= ready {
			return
		}
		if retry != nil && time.Since(poke) > 250*time.Millisecond {
			retry()
			poke = time.Now()
		}
		if time.Since(last) > 500*time.Millisecond {
			t.Logf("%s: %v pending=%d ready=%d stats=%+v", what, time.Since(deadline.Add(-15*time.Second)).Round(time.Millisecond), c.Pending(), c.readyCount(), c.Stats())
			last = time.Now()
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("%s: %.0fs 内只就绪 %d/%d 张", what, 15*time.Second.Seconds(), c.readyCount(), ready)
}

func (c *Cache) readyCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, e := range c.items {
		if e.bitmap != nil {
			n++
		}
	}
	return n
}

// TestConnectionReuse 证明「连接池上限跟着并发走」这件事：连续两批各 8 个
// 并发请求打到同一个主机，默认 Transport（MaxIdleConnsPerHost=2）第一批开
// 8 条连接、完成后只留 2 条空闲，第二批又要重开 6 条；NewTransport(8) 两批
// 复用同一批 8 条。生产是 HTTPS，每条新连接还多一次 TLS 握手 —— 这就是并发
// 8 时「图片加载慢」里非网络 RTT 的那部分。
//
//	go test -tags perfmeasure -run TestConnectionReuse -v ./internal/imagecache/
func TestConnectionReuse(t *testing.T) {
	data := coverPNG()
	newServer := func() (*httptest.Server, *atomic.Int64) {
		var conns atomic.Int64
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write(data)
		}))
		srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
			if s == http.StateNew {
				conns.Add(1)
			}
		}
		srv.Start()
		t.Cleanup(srv.Close)
		return srv, &conns
	}
	burst := func(t *testing.T, cl *http.Client, url string) {
		t.Helper()
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := cl.Get(url)
				if err != nil {
					t.Errorf("GET: %v", err)
					return
				}
				resp.Body.Close()
			}()
		}
		wg.Wait()
	}
	measure := func(name string, tr *http.Transport) {
		srv, conns := newServer()
		cl := &http.Client{Timeout: 5 * time.Second, Transport: tr}
		defer tr.CloseIdleConnections()
		burst(t, cl, srv.URL+"/a.png")
		burst(t, cl, srv.URL+"/b.png")
		t.Logf("%s: 两批 8 并发共新建 %d 条连接", name, conns.Load())
	}
	measure("默认 Transport", &http.Transport{})
	measure("NewTransport(8)", NewTransport(8))
}
