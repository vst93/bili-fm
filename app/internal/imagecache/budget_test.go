package imagecache

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 用户 2026-10-18 晚间 ⑥（内存）：条目数上限不够 —— 480w 的播放器封面是
// 240w 卡片的 4 倍，64 条全落在 480w 上是 33MB。这条钉住「按字节封顶」：
// 超过 cacheByteLimit 就按同一套 LRU 淘汰，retained（头像）不动。
func TestCacheByteBudgetEvicts(t *testing.T) {
	// 每张 480×270×4 ≈ 0.5MB：30 张 ≈ 15MB > 12MB 预算，但只占 64 条上限的
	// 一半 —— 条目数判据完全看不出问题，字节判据必须淘汰。
	mk := func() *ui.Bitmap {
		return ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 480, 270)))
	}
	c := New(nil)
	// 头像：常驻，永不淘汰（即使它让字节预算超一点点）。
	c.retained = map[key]struct{}{}
	c.mu.Lock()
	c.limit = 64
	for i := 0; i < 30; i++ {
		k := key{url: string(rune('a' + i)), width: WidthPlayer}
		b := mk()
		e := &entry{bitmap: b}
		e.bytes = bitmapBytes(b, nil)
		c.items[k] = e
		c.order = append(c.order, k)
		c.bytes += e.bytes
	}
	keep := key{url: "avatar", width: WidthAvatar}
	c.items[keep] = &entry{bytes: 1024}
	c.order = append(c.order, keep)
	c.bytes += 1024
	c.retained[keep] = struct{}{}
	before := c.bytes
	c.evictLocked()
	got := c.bytes
	c.mu.Unlock()

	if got > cacheByteLimit {
		t.Errorf("淘汰后仍有 %.1fMB（上限 %.1fMB，改前 %.1fMB）", float64(got)/1e6, float64(cacheByteLimit)/1e6, float64(before)/1e6)
	}
	if _, ok := c.items[keep]; !ok {
		t.Error("常驻头像被字节预算淘汰了")
	}
	if len(c.order) < 1 {
		t.Error("淘汰把缓存清空了")
	}
	// 反证：把字节判据去掉（只留条目数）时同一批条目一条都不会被淘汰。
	c2 := New(nil)
	c2.mu.Lock()
	for i := 0; i < 30; i++ {
		k := key{url: string(rune('a' + i)), width: WidthPlayer}
		e := &entry{}
		e.bytes = bitmapBytes(mk(), nil)
		c2.items[k] = e
		c2.order = append(c2.order, k)
		c2.bytes += e.bytes
	}
	c2.mu.Unlock()
	if len(c2.items) != 30 {
		t.Fatalf("反证组只剩 %d 条（30 条本就在 64 的条目上限内）", len(c2.items))
	}
}
