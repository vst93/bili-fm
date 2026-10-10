package ui

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/gif"  // DecodeBitmap
	_ "image/jpeg" // DecodeBitmap
	_ "image/png"  // DecodeBitmap
	"math"

	_ "golang.org/x/image/bmp"  // DecodeBitmap
	_ "golang.org/x/image/webp" // DecodeBitmap

	"github.com/egoist/mygo/internal/scene"
)

// Bitmap is an image to show with Image. Create it once: converting an
// image is not free.
type Bitmap struct {
	img  *scene.Image
	w, h int
	// levels are the bitmap halved again and again, made as it shows
	// smaller, for drawing it smoothly: levels[k] is 2^(k+1) times
	// smaller.
	levels []*scene.Image
}

// NewBitmap converts img.
func NewBitmap(img image.Image) *Bitmap {
	s := scene.NewImage(img)
	return &Bitmap{img: s, w: s.W, h: s.H}
}

// DecodeBitmap decodes a PNG, JPEG, GIF (its first frame), WebP or BMP
// image. A JPEG photo shows upright, as its camera's EXIF orientation
// says.
func DecodeBitmap(data []byte) (*Bitmap, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := NewBitmap(img)
	if format == "jpeg" {
		if o := exifOrientation(data); o > 1 {
			b.img = orient(b.img, o)
			b.w, b.h = b.img.W, b.img.H
		}
	}
	return b, nil
}

// Size returns the bitmap's size in pixels, which Image shows as DIPs.
func (b *Bitmap) Size() (w, h int) { return b.w, b.h }

func (b *Bitmap) imageSize() (float32, float32) {
	if b == nil {
		return 0, 0
	}
	return float32(b.w), float32(b.h)
}

// smaller returns the smallest of the bitmap and its levels that is no
// smaller than shrink times smaller, for drawing it at that size: a GPU
// sampling a bitmap far larger than it shows sees only some of its
// pixels, which shimmer, where a level averages them all.
func (b *Bitmap) smaller(shrink float32) *scene.Image {
	if shrink < 2 {
		return b.img
	}
	k := int(math.Log2(float64(shrink))) // halvings that stay as large
	img := b.img
	for i := 0; i < k; i++ {
		if i == len(b.levels) {
			if img.W < 2 && img.H < 2 {
				break
			}
			b.levels = append(b.levels, halve(img))
		}
		img = b.levels[i]
	}
	return img
}

// halve returns img half as large, each pixel the average of the 2×2 it
// covers, of premultiplied colors; an odd last row or column counts twice.
func halve(img *scene.Image) *scene.Image {
	w, h := max(1, (img.W+1)/2), max(1, (img.H+1)/2)
	out := make([]byte, 4*w*h)
	src, stride := img.Pix, 4*img.W
	for y := range h {
		y0, y1 := 2*y, min(2*y+1, img.H-1)
		r0, r1 := src[y0*stride:], src[y1*stride:]
		o := out[4*w*y:]
		for x := range w {
			x0, x1 := 4*2*x, 4*min(2*x+1, img.W-1)
			for c := range 4 {
				o[4*x+c] = uint8((uint32(r0[x0+c]) + uint32(r0[x1+c]) + uint32(r1[x0+c]) + uint32(r1[x1+c]) + 2) / 4)
			}
		}
	}
	return scene.NewImageRGBA(w, h, out)
}

// exifOrientation returns the orientation of a JPEG's EXIF data, 1 to 8,
// or 0 without one.
func exifOrientation(jpg []byte) int {
	if len(jpg) < 4 || jpg[0] != 0xff || jpg[1] != 0xd8 {
		return 0
	}
	for i := 2; i+4 <= len(jpg); {
		if jpg[i] != 0xff {
			return 0
		}
		marker, size := jpg[i+1], int(binary.BigEndian.Uint16(jpg[i+2:]))
		if marker == 0xda || size < 2 || i+2+size > len(jpg) { // the image data starts
			return 0
		}
		seg := jpg[i+4 : i+2+size]
		if marker == 0xe1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 0
}

// tiffOrientation reads the Orientation tag of the first IFD of TIFF data.
func tiffOrientation(t []byte) int {
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 0
	}
	n := int(bo.Uint16(t[ifd:]))
	for e := ifd + 2; e+12 <= len(t) && n > 0; e, n = e+12, n-1 {
		if bo.Uint16(t[e:]) == 0x0112 && bo.Uint16(t[e+2:]) == 3 { // Orientation, a SHORT
			if o := int(bo.Uint16(t[e+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 0
		}
	}
	return 0
}

// orient turns and flips img as EXIF orientation o says it is to show.
func orient(img *scene.Image, o int) *scene.Image {
	w, h := img.W, img.H
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	out := make([]byte, 4*dw*dh)
	for y := range dh {
		for x := range dw {
			var sx, sy int
			switch o {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			default:
				sx, sy = x, y
			}
			copy(out[4*(y*dw+x):4*(y*dw+x)+4], img.Pix[4*(sy*w+sx):])
		}
	}
	return scene.NewImageRGBA(dw, dh, out)
}
