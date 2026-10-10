//go:build windows && (amd64 || arm64)

package windows

import (
	"bytes"
	"runtime"
	"testing"
	"unsafe"

	"github.com/egoist/mygo/transfer"
)

func TestTransferDataStreamPosition(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dropOnce.Do(initDropTarget)

	format := uint16(registerClipboardFormat("application/vnd.mygo.stream-test"))
	for _, clipboard := range []bool{false, true} {
		for _, payload := range [][]byte{nil, {0, 1, 2, 0}, bytes.Repeat([]byte{1, 0}, 64<<10)} {
			checkTransferDataStream(t, format, clipboard, payload)
		}
	}
}

func checkTransferDataStream(t *testing.T, format uint16, clipboard bool, payload []byte) {
	t.Helper()
	data := transfer.New(transfer.NewItem(transfer.Bytes("application/vnd.mygo.stream-test", payload)))
	object := newTransferData(data, "", clipboard)
	defer release(object)
	f := nativeFormat(format)
	f.Tymed = tymedStream
	var medium stgMedium
	if hr := comCall(object, dataGetData, uintptr(unsafe.Pointer(&f)), uintptr(unsafe.Pointer(&medium))); failed(hr) {
		t.Fatal(hresultError("IDataObject::GetData", hr))
	}
	defer procReleaseStgMedium.Call(uintptr(unsafe.Pointer(&medium)))
	if medium.Tymed != tymedStream || medium.Handle == 0 {
		t.Fatalf("GetData returned an invalid stream: %+v", medium)
	}
	var position uint64
	if hr := comCall(medium.Handle, 5, 0, 1, uintptr(unsafe.Pointer(&position))); failed(hr) { // STREAM_SEEK_CUR
		t.Fatal(hresultError("IStream::Seek", hr))
	}
	if position != uint64(len(payload)) {
		t.Fatalf("clipboard=%t: GetData stream position %d, want payload length %d", clipboard, position, len(payload))
	}
	if got, err := readDragStream(medium.Handle); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("clipboard=%t: readDragStream returned %d bytes: %v; want %d", clipboard, len(got), err, len(payload))
	}
	if got, err := oleDataBytes(object, format); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("clipboard=%t: oleDataBytes returned %d bytes: %v; want %d", clipboard, len(got), err, len(payload))
	}
}
