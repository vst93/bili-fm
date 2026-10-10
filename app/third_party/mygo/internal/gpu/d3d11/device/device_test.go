//go:build windows

package device

import (
	"syscall"
	"testing"
	"unsafe"
)

// IID_IDXGIDevice
var iidIDXGIDevice = [16]byte{0xfa, 0x77, 0xec, 0x54, 0x77, 0x13, 0xe6, 0x44, 0x8c, 0x32, 0x88, 0xfd, 0x5f, 0x44, 0xc8, 0x4c}

// call calls method i of the COM interface obj.
func call(obj uintptr, i int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(*(*unsafe.Pointer)(unsafe.Pointer(&obj)))
	fn := *(*uintptr)(unsafe.Add(*(*unsafe.Pointer)(unsafe.Pointer(&vtbl)), i*int(unsafe.Sizeof(uintptr(0)))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

// TestNew checks that New gives a device and a context, and that the device
// is a DXGI device, which DirectComposition needs.
func TestNew(t *testing.T) {
	device, context, software, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer call(device, 2)
	defer call(context, 2)
	if device == 0 || context == 0 {
		t.Fatalf("device %#x, context %#x", device, context)
	}
	var dxgi uintptr
	if hr := call(device, 0, uintptr(unsafe.Pointer(&iidIDXGIDevice)), uintptr(unsafe.Pointer(&dxgi))); int32(uint32(hr)) < 0 {
		t.Fatalf("the device is no DXGI device: %#x", uint32(hr))
	}
	call(dxgi, 2)
	t.Logf("software (WARP): %v", software)
}
