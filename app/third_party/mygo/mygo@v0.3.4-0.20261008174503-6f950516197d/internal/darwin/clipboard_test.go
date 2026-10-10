//go:build darwin

package darwin

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/egoist/mygo/transfer"
)

func TestClipboardNativeAliasesAndFileList(t *testing.T) {
	load()
	withPool(func() {
		item := autorelease(send(send(class("NSPasteboardItem"), "alloc"), "init"))
		const utf16 = "public.utf16-external-plain-text"
		send(item, "setString:forType:", uintptr(nsString("hello 日本語")), uintptr(nsString(utf16)))
		files := []byte("file:///tmp/a%20%23.txt\r\nfile:///tmp/b.txt")
		send(item, "setData:forType:", uintptr(nsData([]byte("file:///tmp/a%20%23.txt"))), uintptr(nsString("public.file-url")))
		send(item, "setData:forType:", uintptr(nsData(files)), uintptr(nsString("dev.mygo.file-list")))
		var encoded bytes.Buffer
		_ = png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2, 3)))
		rep := send(class("NSBitmapImageRep"), "imageRepWithData:", uintptr(nsData(encoded.Bytes())))
		tiff := send(rep, "representationUsingType:properties:", 0, uintptr(send(class("NSDictionary"), "dictionary")))
		send(item, "setData:forType:", uintptr(tiff), uintptr(nsString(utTIFF)))
		reps, err := macClipboardReadItem(item, []transfer.Format{transfer.Text, transfer.FileList, transfer.URIList, transfer.PNG, transfer.Text})
		if err != nil {
			t.Fatal(err)
		}
		d := transfer.New(transfer.NewItem(reps...))
		if b, _ := d.Read(transfer.Text); string(b) != "hello 日本語" {
			t.Fatalf("UTF-16 conversion %q", b)
		}
		for _, f := range []transfer.Format{transfer.FileList, transfer.URIList} {
			if b, _ := d.Read(f); !bytes.Equal(b, files) {
				t.Fatalf("complete %s: %q", f, b)
			}
		}
		b, _ := d.Read(transfer.PNG)
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil || img.Bounds() != image.Rect(0, 0, 2, 3) {
			t.Fatalf("TIFF conversion: %v", err)
		}
	})
}
