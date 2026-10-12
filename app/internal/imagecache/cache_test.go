package imagecache

import (
	"image"
	"testing"
)

func TestThumbURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{
			"https://i0.hdslb.com/bfs/archive/abc.jpg",
			"https://i0.hdslb.com/bfs/archive/abc.jpg@240w.webp",
		},
		{
			// 已经是处理过的地址，不再追加后缀。
			"https://i0.hdslb.com/bfs/archive/abc.jpg@480w.webp",
			"https://i0.hdslb.com/bfs/archive/abc.jpg@480w.webp",
		},
		{
			// 协议相对地址补 https。
			"//i1.hdslb.com/bfs/archive/x.png",
			"https://i1.hdslb.com/bfs/archive/x.png@240w.webp",
		},
		{
			// http 图床统一走 https。
			"http://i2.hdslb.com/bfs/archive/y.jpg",
			"https://i2.hdslb.com/bfs/archive/y.jpg@240w.webp",
		},
		{
			// 非 hdslb 图床原样返回。
			"https://archive.biliimg.com/bfs/archive/z.jpg",
			"https://archive.biliimg.com/bfs/archive/z.jpg",
		},
	}
	for _, c := range cases {
		if got := ThumbURL(c.in); got != c.want {
			t.Errorf("ThumbURL(%q) = %q, want %q", c.in, got, c.want)
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
		if got := ThumbURL(in); got != in {
			t.Errorf("ThumbURL(%q) = %q，非 B 站地址应原样返回", in, got)
		}
	}
}
