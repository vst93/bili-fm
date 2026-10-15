package view

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"
)

func decodeEmbedded(t *testing.T, raw []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("解码内嵌 PNG 失败: %v", err)
	}
	return img
}

// TestCoverLogoHasOpaqueWhiteBackground 钉住封面兜底图是**白底**（原版
// public/logo.png）：整张图完全不透明，边角是白。透明底会让盘面的深色
// （home.go 里 `#0f172a`）透出来，用户看到的就是「默认封面黑底」。
func TestCoverLogoHasOpaqueWhiteBackground(t *testing.T) {
	img := decodeEmbedded(t, logoCoverPNG)
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a != 0xffff {
				t.Fatalf("白底兜底图 (%d,%d) 半透明 a=%d，应为不透明", x, y, a)
			}
			_ = r
			_ = g
			_ = bl
		}
	}
	r, g, bl, _ := img.At(b.Min.X, b.Min.Y).RGBA()
	if r < 0xf000 || g < 0xf000 || bl < 0xf000 {
		t.Errorf("兜底图左上角 = (%d,%d,%d)，应为白", r>>8, g>>8, bl>>8)
	}
}

// TestTitleLogoStaysTransparent 钉住标题栏那张仍是**透明底**（原版
// public/logo-transparent.png）：换封面兜底图时不能把标题栏也换成白底方块。
func TestTitleLogoStaysTransparent(t *testing.T) {
	img := decodeEmbedded(t, logoPNG)
	b := img.Bounds()
	transparent := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a == 0 {
				transparent++
			}
		}
	}
	if transparent == 0 {
		t.Fatal("标题栏 logo 没有任何透明像素——它被换成了白底图")
	}
}

// TestCoverLogoPrefersWhiteBackground 钉住 coverLogo() 用白底那张。
func TestCoverLogoPrefersWhiteBackground(t *testing.T) {
	if LogoCover == nil {
		t.Fatal("LogoCover 解码失败")
	}
	if coverLogo() != LogoCover {
		t.Fatal("coverLogo() 应优先返回白底 LogoCover")
	}
}

// TestDefaultCoverIsWhiteNotBlack 是无头渲染实测：没有封面时，盘面铺的是白底
// 兜底图，不是从透明 logo 透出的黑底。把盘面左侧（logo 图形之外、纯底色处）
// 采样，亮度应接近白。
func TestDefaultCoverIsWhiteNotBlack(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"} // 无 Cover
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	r, ok := tst.Find("播放")
	if !ok {
		t.Fatalf("找不到盘面：%v", tst.Texts())
	}
	// 盘面左侧偏上，避开居中的台标图形。
	x, y := int(r.X+r.W*0.18), int(r.Y+r.H*0.5)
	img := tst.Image()
	i := img.PixOffset(x, y)
	rr, gg, bb := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
	l := (rr*299 + gg*587 + bb*114) / 1000
	t.Logf("默认封面盘面 (%d,%d) = (%d,%d,%d) 亮度 %d", x, y, rr, gg, bb, l)
	if l < 170 {
		t.Fatalf("默认封面盘面亮度 %d 偏暗——透明 logo 透出了深色底", l)
	}
}
