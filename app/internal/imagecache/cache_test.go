package imagecache

import (
	"fmt"
	"image"
	"testing"
	"time"
)

func TestThumbURL(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"", WidthCard, ""},
		{
			"https://i0.hdslb.com/bfs/archive/abc.jpg",
			WidthCard,
			"https://i0.hdslb.com/bfs/archive/abc.jpg@240w.webp",
		},
		{
			// 已经是处理过的地址，不再追加后缀。
			"https://i0.hdslb.com/bfs/archive/abc.jpg@480w.webp",
			WidthCard,
			"https://i0.hdslb.com/bfs/archive/abc.jpg@480w.webp",
		},
		{
			// 协议相对地址补 https。
			"//i1.hdslb.com/bfs/archive/x.png",
			WidthCard,
			"https://i1.hdslb.com/bfs/archive/x.png@240w.webp",
		},
		{
			// http 图床统一走 https。
			"http://i2.hdslb.com/bfs/archive/y.jpg",
			WidthCard,
			"https://i2.hdslb.com/bfs/archive/y.jpg@240w.webp",
		},
		{
			// 非 hdslb 图床原样返回。
			"https://archive.biliimg.com/bfs/archive/z.jpg",
			WidthCard,
			"https://archive.biliimg.com/bfs/archive/z.jpg",
		},
		// 各调用点宽度（对齐 Tauri 原版 graftingImage 的显式宽度）。
		{
			"https://i0.hdslb.com/bfs/archive/abc.jpg",
			WidthPlayer,
			"https://i0.hdslb.com/bfs/archive/abc.jpg@480w.webp",
		},
		{
			"https://i0.hdslb.com/bfs/archive/abc.jpg",
			WidthAmbient,
			"https://i0.hdslb.com/bfs/archive/abc.jpg@320w.webp",
		},
		{
			"https://i0.hdslb.com/bfs/archive/abc.jpg",
			WidthPlaylist,
			"https://i0.hdslb.com/bfs/archive/abc.jpg@192w.webp",
		},
		{
			"https://i0.hdslb.com/bfs/face/member/noface.jpg",
			WidthAvatar,
			"https://i0.hdslb.com/bfs/face/member/noface.jpg@96w.webp",
		},
	}
	for _, c := range cases {
		if got := ThumbURL(c.in, c.width); got != c.want {
			t.Errorf("ThumbURL(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
	}
}

// TestGradeAmbient 钉住原版滤镜（saturate .78 / brightness 1.14 / contrast .76）
// 的几个特征点，免得以后调色时把参数改错。
func TestGradeAmbient(t *testing.T) {
	mk := func(r, g, b uint8) *image.RGBA {
		im := image.NewRGBA(image.Rect(0, 0, 1, 1))
		im.Pix[0], im.Pix[1], im.Pix[2], im.Pix[3] = r, g, b, 255
		return im
	}
	// 中灰：饱和度不动它；亮度 1.14 抬到 0.572，再被对比度 0.76 压回
	// (0.572-0.5)*0.76+0.5 = 0.5547 → 141。
	got := gradeAmbient(mk(128, 128, 128), true)
	if got.Pix[0] != 141 {
		t.Errorf("中灰应为 141，得到 %d", got.Pix[0])
	}
	// 纯色：饱和度降低后应往灰靠（R 降、G 升），且不再是纯色。
	got = gradeAmbient(mk(255, 0, 0), true)
	if got.Pix[0] <= got.Pix[1] {
		t.Errorf("降饱和后红通道应仍高于绿通道，得到 %d/%d", got.Pix[0], got.Pix[1])
	}
	if got.Pix[1] == 0 {
		t.Error("降饱和后绿通道不应还是 0")
	}
	// 亮度提升 + 对比度压缩：纯黑提升到接近 0.1 而不是仍是 0。
	got = gradeAmbient(mk(0, 0, 0), true)
	if got.Pix[0] < 20 || got.Pix[0] > 40 {
		t.Errorf("纯黑应被抬到 20~40，得到 %d", got.Pix[0])
	}
}

// TestDownscale 检查降采样守住最长边上限，且颜色大致保留。
func TestDownscale(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = 200, 100, 50, 255
	}
	got := downscale(im, 64)
	if got.Bounds().Dx() != 64 || got.Bounds().Dy() != 32 {
		t.Fatalf("应缩到 64×32，得到 %v", got.Bounds())
	}
	if got.Pix[0] != 200 || got.Pix[1] != 100 || got.Pix[2] != 50 {
		t.Errorf("颜色应保留，得到 %d/%d/%d", got.Pix[0], got.Pix[1], got.Pix[2])
	}
	// 已经够小就不再缩。
	small := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if d := downscale(small, 64); d.Bounds().Dx() != 10 {
		t.Errorf("够小的图不应再缩，得到 %v", d.Bounds())
	}
}

// TestBlurRGBA 检查预烤进氛围光源图的模糊：硬边被混成灰、尺寸不变、
// 且不修改输入。氛围光不再走每帧 backdrop，模糊必须在这里正确完成。
func TestBlurRGBA(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			v := uint8(0)
			if x >= 4 {
				v = 255
			}
			i := im.PixOffset(x, y)
			im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = v, v, v, 255
		}
	}
	got := blurRGBA(im, 1)
	if got.Bounds() != im.Bounds() {
		t.Fatalf("尺寸不应改变，得到 %v", got.Bounds())
	}
	// 边界附近的像素应是黑白的混合，而不是仍然非黑即白。
	if v := got.Pix[got.PixOffset(3, 4)]; v == 0 || v == 255 {
		t.Errorf("硬边应被模糊成灰，得到 %d", v)
	}
	// 输入不能被改坏（函数应是纯的）。
	if v := im.Pix[im.PixOffset(0, 0)]; v != 0 {
		t.Errorf("输入被修改，左上角 = %d", v)
	}
}

// TestThumbURLLeavesNonBilibiliAlone 钉住：只有 B 站 hdslb 图床才被改写。
// 从前把**所有** http:// 都升级成 https://，把本地代理 / 测试服务器改坏。
func TestThumbURLLeavesNonBilibiliAlone(t *testing.T) {
	for _, in := range []string{
		"http://127.0.0.1:4654/image-proxy?url=x",
		"http://localhost:8080/cover.png",
		"http://example.com/bfs/archive/a.jpg",
	} {
		if got := ThumbURL(in, WidthCard); got != in {
			t.Errorf("ThumbURL(%q) = %q，非 B 站地址应原样返回", in, got)
		}
	}
}

// TestThumbURLKeepsWidthZeroDefault 钉住：宽度 <= 0 时退回卡片宽度，
// 不会拼出 `@0w.webp` 这种无意义后缀。
func TestThumbURLKeepsWidthZeroDefault(t *testing.T) {
	in := "https://i0.hdslb.com/bfs/archive/abc.jpg"
	if got, want := ThumbURL(in, 0), in+"@240w.webp"; got != want {
		t.Errorf("ThumbURL(%q, 0) = %q, want %q", in, got, want)
	}
}

// waitIdle 等到没有在抓的图，避免并发顺序让下面的断言不确定。
func waitIdle(t *testing.T, c *Cache) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if c.Pending() == 0 {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("缓存一直没停下来")
}

// TestCacheBoundedLRU 钉住缓存有上限、按 LRU 淘汰。
//
// 原版对齐点：Tauri 版 useViewportImageUnload 在卡片滚出 2 屏后主动把
// <img> 换成 1px 占位，促使引擎丢弃解码位图；mygo 的 ui.List 回收行后不会
// 通知缓存，所以用有界 LRU 等效兜底。没有这条上限，长列表从头滚到尾会把
// 每张封面位图都攒下来（300 张 240w ≈ 57MB）。
func TestCacheBoundedLRU(t *testing.T) {
	srv, hits := countServer(t)
	c := New(nil)
	limit := c.Limit()
	if limit <= 0 {
		t.Fatalf("缓存上限 = %d，必须为正", limit)
	}
	// 顺序加载超过上限的图，最旧的应被淘汰。
	for i := 0; i < limit+20; i++ {
		c.Bitmap(fmt.Sprintf("%s/c%d.png", srv.URL, i), WidthCard)
		waitIdle(t, c)
	}
	if got := c.Len(); got > limit {
		t.Fatalf("缓存位图数 = %d，超过上限 %d", got, limit)
	}
	// 最早的几张已淘汰：再取要重新发 HTTP。
	old := fmt.Sprintf("%s/c0.png", srv.URL)
	before := hits("/c0.png")
	c.Bitmap(old, WidthCard)
	waitIdle(t, c)
	if got := hits("/c0.png"); got != before+1 {
		t.Fatalf("被 LRU 淘汰的旧图再取应重发 HTTP（命中 %d 次，want %d）", got, before+1)
	}
	// 最近用过的仍在缓存：不再发 HTTP。
	newest := fmt.Sprintf("%s/c%d.png", srv.URL, limit+19)
	before = hits(fmt.Sprintf("/c%d.png", limit+19))
	c.Bitmap(newest, WidthCard)
	waitIdle(t, c)
	if got := hits(fmt.Sprintf("/c%d.png", limit+19)); got != before {
		t.Fatalf("最近用过的图不该重发 HTTP（命中 %d 次，want %d）", got, before)
	}
}

// TestCacheBoundedOnBurst 钉住一次涌入大量抓取请求（都处于 pending）时，
// 缓存也不会无界增长：就绪后必须被重新淘汰回上限内。
func TestCacheBoundedOnBurst(t *testing.T) {
	srv, _ := countServer(t)
	c := New(nil)
	limit := c.Limit()
	for i := 0; i < limit+50; i++ {
		c.Bitmap(fmt.Sprintf("%s/b%d.png", srv.URL, i), WidthCard)
	}
	waitIdle(t, c)
	if got := c.Len(); got > limit {
		t.Fatalf("突发请求后缓存位图数 = %d，超过上限 %d", got, limit)
	}
}

// TestCacheMemoryBoundedOverScroll 模拟滚过长列表：滚一遍 300 张卡片，
// 缓存字节数必须收敛在上限内，而不是随滚动无限累积。
func TestCacheMemoryBoundedOverScroll(t *testing.T) {
	srv, _ := countServer(t)
	c := New(nil)
	peak := int64(0)
	for i := 0; i < 300; i++ {
		c.Bitmap(fmt.Sprintf("%s/s%d.png", srv.URL, i), WidthCard)
		if i%15 == 14 {
			waitIdle(t, c)
			if b := c.Bytes(); b > peak {
				peak = b
			}
		}
	}
	waitIdle(t, c)
	if c.Len() > c.Limit() {
		t.Fatalf("滚动后位图数 %d 超过上限 %d", c.Len(), c.Limit())
	}
	// 峰值也不该超过「上限张数 × 单张上限」太多：这里只断言它是有界的，
	// 具体数字由 Bytes() 的估算给出，测试打印出来给报告用。
	t.Logf("滚动 300 张后：位图 %d 张、约 %d 字节、峰值约 %d 字节（上限 %d 张）",
		c.Len(), c.Bytes(), peak, c.Limit())
}

// TestCacheKeysByWidth 钉住宽度是缓存 key 的一部分：同一 URL 的不同宽度
// 各存一份、各自抓一次（原版各调用点 URL 后缀不同即不同请求）。
func TestCacheKeysByWidth(t *testing.T) {
	srv, hits := countServer(t)
	c := New(nil)
	u := srv.URL + "/one.png"
	c.Bitmap(u, WidthCard)
	waitIdle(t, c)
	c.Bitmap(u, WidthAvatar)
	waitIdle(t, c)
	if got := hits("/one.png"); got != 2 {
		t.Fatalf("同一 URL 两种宽度应各抓一次，共 %d 次", got)
	}
	if c.Get(u, WidthCard) == nil || c.Get(u, WidthAvatar) == nil {
		t.Fatal("两种宽度都应命中各自缓存")
	}
}
