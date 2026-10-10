package ui

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"golang.org/x/image/bmp"
)

// stripedImage returns a w×h image of vertical stripes, black and white, each
// n pixels wide, shifted by off pixels.
func stripedImage(w, h, n, off int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := uint8(0)
			if (x+off)/n%2 == 1 {
				c = 255
			}
			img.Set(x, y, color.RGBA{c, c, c, 255})
		}
	}
	return img
}

func TestBitmapShowsSmallerSmoothly(t *testing.T) {
	// Stripes of 4 pixels, whose middles a GPU's samples, one in 8 pixels,
	// fall on: all white.
	bm := NewBitmap(stripedImage(512, 512, 4, 2))
	tt := coreNewTester(func(c *context) {
		coreImage(c, bm).Size(64, 64).Fit(FillBox)
	}, 100, 100)
	// Every pixel shows as many black stripes as white: gray, not stripes
	// of its own.
	img := tt.Image()
	lo, hi := uint8(255), uint8(0)
	for y := 4; y < 60; y++ {
		for x := 4; x < 60; x++ {
			v := img.RGBAAt(x, y).R
			lo, hi = min(lo, v), max(hi, v)
		}
	}
	if lo < 100 || hi > 160 {
		t.Errorf("the stripes, 8 times smaller, range from %d to %d", lo, hi)
	}
	if len(bm.levels) != 3 {
		t.Errorf("%d levels made", len(bm.levels))
	}
	// At its size, it is itself.
	if got := bm.smaller(1); got != bm.img {
		t.Error("a level for the bitmap at its size")
	}
}

func TestHalve(t *testing.T) {
	// 3×3 of values 0, 40, 80 by column: an odd column counts twice.
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := range 3 {
		for x := range 3 {
			img.Set(x, y, color.RGBA{uint8(40 * x), 0, 0, 255})
		}
	}
	h := halve(NewBitmap(img).img)
	if h.W != 2 || h.H != 2 {
		t.Fatalf("%d×%d", h.W, h.H)
	}
	if r0, r1 := h.Pix[0], h.Pix[4]; r0 != 20 || r1 != 80 {
		t.Errorf("columns %d and %d", r0, r1)
	}
}

// withOrientation returns a JPEG with an EXIF segment giving orientation
// o, in byte order bo.
func withOrientation(t *testing.T, img image.Image, o uint16, bo binary.ByteOrder) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	tiff := make([]byte, 8+2+12+4)
	if bo == binary.LittleEndian {
		copy(tiff, "II")
	} else {
		copy(tiff, "MM")
	}
	bo.PutUint16(tiff[2:], 42)
	bo.PutUint32(tiff[4:], 8)
	bo.PutUint16(tiff[8:], 1)       // one entry
	bo.PutUint16(tiff[10:], 0x0112) // Orientation
	bo.PutUint16(tiff[12:], 3)      // SHORT
	bo.PutUint32(tiff[14:], 1)
	bo.PutUint16(tiff[18:], o)
	app1 := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(2+len(app1)))
	data := buf.Bytes()
	return append(append(append([]byte{}, data[:2]...), append(seg, app1...)...), data[2:]...)
}

func TestDecodeBitmapTurnsPhotosUpright(t *testing.T) {
	// Stored on its side, 32×16: red on the left, blue on the right.
	img := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for y := range 16 {
		for x := range 32 {
			c := color.RGBA{255, 0, 0, 255}
			if x >= 16 {
				c = color.RGBA{0, 0, 255, 255}
			}
			img.Set(x, y, c)
		}
	}
	red := func(b *Bitmap, x, y int) bool {
		p := b.img.Pix[4*(y*b.w+x):]
		return p[0] > 200 && p[2] < 60
	}
	for _, c := range []struct {
		o          uint16
		bo         binary.ByteOrder
		w, h       int
		redX, redY int // a pixel that is red
	}{
		{1, binary.BigEndian, 32, 16, 4, 8},
		{3, binary.LittleEndian, 32, 16, 28, 8}, // upside down
		{6, binary.BigEndian, 16, 32, 8, 4},     // turned right: red on top
		{8, binary.LittleEndian, 16, 32, 8, 28}, // turned left: red at the bottom
	} {
		b, err := DecodeBitmap(withOrientation(t, img, c.o, c.bo))
		if err != nil {
			t.Fatal(err)
		}
		if b.w != c.w || b.h != c.h || !red(b, c.redX, c.redY) {
			t.Errorf("orientation %d: %d×%d, red at (%d, %d): %v", c.o, b.w, b.h, c.redX, c.redY, red(b, c.redX, c.redY))
		}
	}
	if exifOrientation([]byte("not a jpeg")) != 0 {
		t.Error("an orientation in what is no JPEG")
	}
}

func TestDecodeBitmapOfWebPAndBMP(t *testing.T) {
	webp, _ := base64.StdEncoding.DecodeString("UklGRiQAAABXRUJQVlA4IBgAAAAwAQCdASoBAAEAAwA0JaQAA3AA/vuUAAA=")
	b, err := DecodeBitmap(webp)
	if err != nil || b.w != 1 || b.h != 1 {
		t.Errorf("WebP: %v", err)
	}
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, stripedImage(6, 4, 1, 0)); err != nil {
		t.Fatal(err)
	}
	if b, err = DecodeBitmap(buf.Bytes()); err != nil || b.w != 6 || b.h != 4 || b.img.Pix[4] != 255 {
		t.Errorf("BMP: %v", err)
	}
}

// BenchmarkHalvePhoto halves a photo of 4000×3000 pixels, a step of
// drawing an image smaller than it is.
func BenchmarkHalvePhoto(b *testing.B) {
	img := NewBitmap(image.NewRGBA(image.Rect(0, 0, 4000, 3000))).img
	b.ResetTimer()
	for range b.N {
		halve(img)
	}
}
