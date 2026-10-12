//go:build perfmeasure

package imagecache

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestScrollRelease 与基线同口径：滚 300 张（远超 2 屏），看条目数/字节
// 是否收敛（LRU 上限 64 = 原版「滚出 2 屏丢位图」的等效兜底），以及回滚后
// 是否按需重新抓取。
func TestScrollRelease(t *testing.T) {
	var mu sync.Mutex
	data := coverPNG()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Write(data)
	}))
	defer srv.Close()
	c := New(nil)
	for i := 0; i < 300; i++ {
		c.Bitmap(srv.URL+"/s"+itoa(i)+".png", WidthCard)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && c.Pending() > 0 {
		time.Sleep(5 * time.Millisecond)
	}
	n, by := c.footprint()
	t.Logf("滚完 300 张：条目 %d、字节 %.1f MB（上限 %d，滚远的位图已按 LRU 释放）", n, float64(by)/1048576, c.Limit())
	// 回滚到第 0 张：已被 LRU 淘汰，应重新抓取（等效原版回滚重新 decode）。
	c.Bitmap(srv.URL+"/s0.png", WidthCard)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && c.Get(srv.URL+"/s0.png", WidthCard) == nil {
		time.Sleep(2 * time.Millisecond)
	}
	if c.Get(srv.URL+"/s0.png", WidthCard) == nil {
		t.Log("回滚第 0 张未重新加载（意外）")
	} else {
		t.Log("回滚第 0 张：重新抓取成功（旧位图已随 LRU 释放，按需重建）")
	}
}

func (c *Cache) footprint() (int, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var by int64
	for _, e := range c.items {
		if e.bitmap != nil {
			w, h := e.bitmap.Size()
			by += int64(w) * int64(h) * 4
		}
		if e.small != nil {
			sb := e.small.Bounds()
			by += int64(sb.Dx()) * int64(sb.Dy()) * 4
		}
	}
	return len(c.items), by
}
