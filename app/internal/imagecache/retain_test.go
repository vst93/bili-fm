package imagecache

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRetainSurvivesEviction 钉住「常驻图不被 LRU 挤走」：头像（登录/UP）一直
// 挂在界面上，列表封面一轮抓取（一屏十几张、缓存上限 64）就会把它挤出去 ——
// 下一帧 Bitmap 未命中又重抓，头像于是「闪一下」。Retain 每帧重申一次后，
// 头像必须一直在缓存里；没 Retain 的普通图照旧被淘汰（上限仍然生效）。
func TestRetainSurvivesEviction(t *testing.T) {
	data := coverPNG()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.Client = srv.Client()
	c.limit = 8

	avatar := srv.URL + "/face.png"
	c.Bitmap(avatar, WidthAvatar)
	waitPending(t, c)
	if c.Get(avatar, WidthAvatar) == nil {
		t.Fatal("头像没抓到")
	}
	c.Retain(avatar, WidthAvatar)

	// 抓 20 张列表封面（远超上限 8）：普通图会被 LRU 淘汰，常驻头像不能。
	//
	// 注意：淘汰是按 order 从前往后跳过 pending 的，所以早抓的图若在淘汰
	// 那一刻仍在下载中，先完成的后续图反而会先被丢 —— 最终留下来的具体是
	// 哪几张不确定。这里只钉住两条确定的事实：缓存被扣到上限（不无界增长）、
	// 且确实发生了淘汰；头像则必须一直在。
	for i := 0; i < 20; i++ {
		c.Bitmap(srv.URL+"/c"+itoa(i)+".png", WidthCard)
	}
	waitPending(t, c)

	if c.Get(avatar, WidthAvatar) == nil {
		t.Fatalf("常驻头像被 LRU 淘汰了（len=%d limit=%d）", c.Len(), c.Limit())
	}
	if n := c.Len(); n > c.limit {
		t.Errorf("缓存没被淘汰到上限：Len=%d limit=%d", n, c.limit)
	}
	kept := 0
	for i := 0; i < 20; i++ {
		if c.Get(srv.URL+"/c"+itoa(i)+".png", WidthCard) != nil {
			kept++
		}
	}
	if kept >= 20 {
		t.Errorf("20 张封面一张都没被淘汰（kept=%d），LRU 上限失效", kept)
	}
}

// waitPending 等缓存里所有抓取结束。
func waitPending(t *testing.T, c *Cache) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && c.Pending() > 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if c.Pending() != 0 {
		t.Fatalf("抓取没完成：pending=%d", c.Pending())
	}
}

// TestPurgeReleasesBitmaps 钉住「隐藏到后台时清空缓存」：已解码位图、氛围光
// 小图、Retain 头像与在途请求都要释放；滚动/窗口恢复后同一 URL 能重新抓取。
func TestPurgeReleasesBitmaps(t *testing.T) {
	data := coverPNG()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.Client = srv.Client()
	u := srv.URL + "/cover.png"
	c.Bitmap(u, WidthCard)
	waitPending(t, c)
	if c.Get(u, WidthCard) == nil {
		t.Fatal("封面没抓到")
	}
	c.Retain(u, WidthCard)
	if c.Len() == 0 || c.Bytes() == 0 {
		t.Fatal("Purge 前缓存应有内容")
	}

	c.Purge()
	if c.Len() != 0 || c.Bytes() != 0 || c.Pending() != 0 {
		t.Fatalf("Purge 后仍有缓存：len=%d bytes=%d pending=%d", c.Len(), c.Bytes(), c.Pending())
	}
	if c.Get(u, WidthCard) != nil {
		t.Fatal("Purge 后 Get 不应命中旧位图")
	}

	// 恢复后重新发起抓取（Purge 不是永久禁用）。
	c.Bitmap(u, WidthCard)
	waitPending(t, c)
	if c.Get(u, WidthCard) == nil {
		t.Fatal("Purge 后同一 URL 无法重新抓取")
	}
}
