//go:build windows && (amd64 || arm64)

package windows

import (
	"encoding/binary"
	"image"
	"testing"
)

func TestClipboardDIBRejectsInvalidGeometry(t *testing.T) {
	for _, dimensions := range [][2]uint32{{0, 1}, {1, 0}, {0x7fffffff, 0x7fffffff}, {2, 0x80000000}, {1 << 20, 1 << 20}} {
		dib := make([]byte, 40)
		binary.LittleEndian.PutUint32(dib, 40)
		binary.LittleEndian.PutUint32(dib[4:], dimensions[0])
		binary.LittleEndian.PutUint32(dib[8:], dimensions[1])
		binary.LittleEndian.PutUint16(dib[12:], 1)
		binary.LittleEndian.PutUint16(dib[14:], 32)
		if img := dibToImage(dib); img != nil {
			t.Fatalf("malformed dimensions accepted: %v", dimensions)
		}
	}
	dib := imageToDIB(image.NewNRGBA(image.Rect(0, 0, 2, 3)))
	if img := dibToImage(dib); img == nil || img.Bounds() != image.Rect(0, 0, 2, 3) {
		t.Fatal("valid top-down DIB rejected")
	}
	binary.LittleEndian.PutUint32(dib, 0)
	if dibToImage(dib) != nil {
		t.Fatal("invalid bitmap header accepted")
	}
}
