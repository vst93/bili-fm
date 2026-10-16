//go:build windows && (amd64 || arm64)

package main

import (
	"log"
	"sync"
	"syscall"
	"unsafe"
)

// Windows 任务栏缩略图工具栏的实现（ITaskbarList3::ThumbBarAddButtons /
// ThumbBarUpdateButtons + SetWindowSubclass 接管 THBN_CLICKED）。
//
// 结构：
//   - taskbarInit(hwnd)：建 ITaskbarList3、按当前 DPI 与系统主题光栅化 4 个
//     HICON（上一首 / 播放 / 暂停 / 下一首）、加 3 个按钮，并挂子类化过程。
//   - taskbarSetState(playing, canPrev, canNext)：换图标与禁用态；系统主题
//     切换时顺带重建图标。状态无变化时直接返回，不做无谓的 COM 调用。
//   - 点击 → taskbarMediaActions（taskbar.go 接到播放控制）。
//
// 图标不落盘：按字形 4×4 超采样光栅化成 32bpp DIB 再 CreateIconIndirect。
// 已知限制（同 Tauri 原版 v1）：explorer.exe 重启后缩略图工具栏需要重挂；
// 这里在窗口每次 OnShow 时重试 taskbarInit（幂等）兜住常见场景。

var (
	taskbarOle32    = syscall.NewLazyDLL("ole32.dll")
	taskbarUser32   = syscall.NewLazyDLL("user32.dll")
	taskbarGdi32    = syscall.NewLazyDLL("gdi32.dll")
	taskbarComctl32 = syscall.NewLazyDLL("comctl32.dll")
	taskbarAdvapi32 = syscall.NewLazyDLL("advapi32.dll")

	procCoCreateInstance   = taskbarOle32.NewProc("CoCreateInstance")
	procCreateDIBSection   = taskbarGdi32.NewProc("CreateDIBSection")
	procCreateBitmap       = taskbarGdi32.NewProc("CreateBitmap")
	procCreateIconIndirect = taskbarUser32.NewProc("CreateIconIndirect")
	procDeleteObject       = taskbarGdi32.NewProc("DeleteObject")
	procSetWindowSubclass  = taskbarComctl32.NewProc("SetWindowSubclass")
	procDefSubclassProc    = taskbarComctl32.NewProc("DefSubclassProc")
	procRegGetValueW       = taskbarAdvapi32.NewProc("RegGetValueW")
	procGetDpiForWindow    = taskbarUser32.NewProc("GetDpiForWindow")
)

// CLSID_TaskbarList / IID_ITaskbarList3。
var (
	taskbarCLSID    = guid{0x56fdf344, 0xfd6d, 0x11d0, [8]byte{0x95, 0x8a, 0x00, 0x60, 0x97, 0xc9, 0xa0, 0x90}}
	taskbarIIDList3 = guid{0xea1afb91, 0x9e28, 0x4b86, [8]byte{0x90, 0xe9, 0x9e, 0x9f, 0x8a, 0x5e, 0xee, 0x84}}
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// ITaskbarList3 的 vtable 索引（ShObjIdl_core.h；前 3 个是 IUnknown）。
const (
	taskbarIdxHrInit                     = 3
	taskbarIdxThumbBarAddButtons         = 15
	taskbarIdxThumbBarUpdButtons         = 16
	taskbarClsctxInprocServer            = 0x1
	taskbarSubclassID                    = 0x4249_4C46 // "BILF"
	taskbarWmCommand                     = 0x0111
	taskbarThbnClicked                   = 0x1800
	taskbarRrfRtRegDword                 = 0x10
	taskbarHkeyCurrentUser       uintptr = 0x80000001
)

// comCallTaskbar 调 COM 接口 obj 的第 index 个方法。转成 uintptr 的指针在
// 调用返回前保持有效（go:uintptrescapes）。
//
//go:uintptrescapes
func comCallTaskbar(obj uintptr, index int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(nativePtr(obj))
	fn := *(*uintptr)(unsafe.Add(nativePtr(vtbl), uintptr(index)*unsafe.Sizeof(uintptr(0))))
	var a [16]uintptr
	a[0] = obj
	n := copy(a[1:], args)
	r, _, _ := syscall.SyscallN(fn, a[:n+1]...)
	return r
}

// nativePtr 把原生地址当指针用。直接写 unsafe.Pointer(uintptr) 会触发 vet 的
// unsafeptr 检查；借一个局部变量中转是标准写法（同 mygo 的 native()）。
func nativePtr(addr uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&addr)) }

func taskbarFailedHR(hr uintptr) bool { return int32(uint32(hr)) < 0 }

type taskbarIcons struct {
	prev, play, pause, next uintptr
}

type taskbarThumbState struct {
	hwnd    uintptr
	taskbar uintptr
	icons   taskbarIcons
	size    int
	dark    bool
	playing bool
	canPrev bool
	canNext bool
	added   bool
}

var (
	taskbarMu    sync.Mutex
	taskbarCur   *taskbarThumbState
	taskbarSubcl uintptr
)

// taskbarInit 装缩略图工具栏。幂等：同一窗口已挂上就跳过；失败（窗口还没
// 显示、没有 comctl32 v6 等）保留重试机会（下一次 OnShow 再来）。
func taskbarInit(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	taskbarMu.Lock()
	defer taskbarMu.Unlock()
	if st := taskbarCur; st != nil && st.hwnd == hwnd && st.added {
		return
	}
	if taskbarSubcl == 0 {
		taskbarSubcl = syscall.NewCallback(taskbarSubclassProc)
	}
	taskbar, ok := taskbarCreate()
	if !ok {
		return
	}
	size := taskbarIconSize(hwnd)
	dark := taskbarSystemDark()
	icons := taskbarBuildIcons(size, dark)
	buttons := [3]taskbarThumbButton{
		taskbarMakeButton(taskbarBtnPrev, icons.prev, "上一首", true),
		taskbarMakeButton(taskbarBtnPlay, icons.play, "播放", true),
		taskbarMakeButton(taskbarBtnNext, icons.next, "下一首", true),
	}
	hr := comCallTaskbar(taskbar, taskbarIdxThumbBarAddButtons,
		hwnd, 3, uintptr(unsafe.Pointer(&buttons[0])))
	if taskbarFailedHR(hr) {
		log.Printf("任务栏缩略图工具栏：ThumbBarAddButtons 失败（HRESULT %#x）", uint32(hr))
		return
	}
	if r, _, _ := procSetWindowSubclass.Call(hwnd, taskbarSubcl, taskbarSubclassID, 0); r == 0 {
		log.Printf("任务栏缩略图工具栏：SetWindowSubclass 失败，按钮点击不会生效")
	}
	taskbarCur = &taskbarThumbState{
		hwnd: hwnd, taskbar: taskbar, icons: icons, size: size, dark: dark,
		canPrev: true, canNext: true, added: true,
	}
}

// taskbarSetState 同步任务栏按钮的播放/暂停图标与上一首/下一首可用性。
func taskbarSetState(playing, canPrev, canNext bool) {
	taskbarMu.Lock()
	defer taskbarMu.Unlock()
	st := taskbarCur
	if st == nil || !st.added {
		return
	}
	dark := taskbarSystemDark()
	themeChanged := dark != st.dark
	if themeChanged {
		st.dark = dark
		st.icons = taskbarBuildIcons(st.size, dark)
	}
	if !themeChanged && st.playing == playing && st.canPrev == canPrev && st.canNext == canNext {
		return
	}
	st.playing, st.canPrev, st.canNext = playing, canPrev, canNext
	playIcon, tip := st.icons.play, "播放"
	if playing {
		playIcon, tip = st.icons.pause, "暂停"
	}
	buttons := [3]taskbarThumbButton{
		taskbarMakeButton(taskbarBtnPrev, st.icons.prev, "上一首", canPrev),
		taskbarMakeButton(taskbarBtnPlay, playIcon, tip, true),
		taskbarMakeButton(taskbarBtnNext, st.icons.next, "下一首", canNext),
	}
	comCallTaskbar(st.taskbar, taskbarIdxThumbBarUpdButtons,
		st.hwnd, 3, uintptr(unsafe.Pointer(&buttons[0])))
}

// taskbarCreate 建一个 ITaskbarList3 并 HrInit。失败返回 ok=false。
func taskbarCreate() (uintptr, bool) {
	var tl uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&taskbarCLSID)), 0, taskbarClsctxInprocServer,
		uintptr(unsafe.Pointer(&taskbarIIDList3)), uintptr(unsafe.Pointer(&tl)))
	if taskbarFailedHR(hr) || tl == 0 {
		log.Printf("任务栏缩略图工具栏：CoCreateInstance 失败（HRESULT %#x）", uint32(hr))
		return 0, false
	}
	if hr = comCallTaskbar(tl, taskbarIdxHrInit); taskbarFailedHR(hr) {
		log.Printf("任务栏缩略图工具栏：HrInit 失败（HRESULT %#x）", uint32(hr))
		return 0, false
	}
	return tl, true
}

// taskbarSubclassProc 只接管缩略图工具栏按钮的 THBN_CLICKED（WM_COMMAND
// 高位是通知码、低位是按钮 id），其余消息透传给窗口原来的过程。
func taskbarSubclassProc(hwnd, msg, wparam, lparam, subclassID, refData uintptr) uintptr {
	if msg == taskbarWmCommand && (wparam>>16)&0xffff == taskbarThbnClicked {
		switch wparam & 0xffff {
		case taskbarBtnPrev:
			if fn := taskbarMediaActions.Prev; fn != nil {
				fn()
			}
			return 0
		case taskbarBtnPlay:
			if fn := taskbarMediaActions.PlayPause; fn != nil {
				fn()
			}
			return 0
		case taskbarBtnNext:
			if fn := taskbarMediaActions.Next; fn != nil {
				fn()
			}
			return 0
		}
	}
	r, _, _ := procDefSubclassProc.Call(hwnd, msg, wparam, lparam)
	return r
}

// taskbarIconSize 按窗口 DPI 缩放图标：16px @100%、24px @150%、32px @200%，
// 钳到 [16,48]（原版同值）。
func taskbarIconSize(hwnd uintptr) int {
	dpi := uintptr(96)
	if r, _, _ := procGetDpiForWindow.Call(hwnd); r > 0 {
		dpi = r
	}
	size := int(float64(16*dpi)/96.0 + 0.5)
	if size < 16 {
		size = 16
	}
	if size > 48 {
		size = 48
	}
	return size
}

// taskbarSystemDark 读 AppsUseLightTheme：1 = 浅色，0 = 深色；读不到按浅色。
func taskbarSystemDark() bool {
	key, err := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	if err != nil {
		return false
	}
	name, err := syscall.UTF16PtrFromString("AppsUseLightTheme")
	if err != nil {
		return false
	}
	var value uint32 = 1
	size := uint32(unsafe.Sizeof(value))
	r, _, _ := procRegGetValueW.Call(taskbarHkeyCurrentUser,
		uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(name)),
		taskbarRrfRtRegDword, 0,
		uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&size)))
	return taskbarThemeIsDark(value, r == 0)
}

func taskbarBuildIcons(size int, dark bool) taskbarIcons {
	return taskbarIcons{
		prev:  taskbarBuildIcon(size, taskbarGlyphPrev, dark),
		play:  taskbarBuildIcon(size, taskbarGlyphPlay, dark),
		pause: taskbarBuildIcon(size, taskbarGlyphPause, dark),
		next:  taskbarBuildIcon(size, taskbarGlyphNext, dark),
	}
}

type taskbarBitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type taskbarIconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

// taskbarBuildIcon 把一个字形光栅化成 32bpp（直通 alpha）的 HICON。
func taskbarBuildIcon(size int, g taskbarGlyph, dark bool) uintptr {
	px := rasterTaskbarGlyph(size, g, dark)
	if px == nil {
		return 0
	}
	bmi := struct {
		header taskbarBitmapInfoHeader
		colors [1]uint32
	}{}
	bmi.header.biSize = uint32(unsafe.Sizeof(taskbarBitmapInfoHeader{}))
	bmi.header.biWidth = int32(size)
	bmi.header.biHeight = -int32(size) // 负高度 = top-down DIB
	bmi.header.biPlanes = 1
	bmi.header.biBitCount = 32
	bmi.header.biCompression = 0 // BI_RGB

	var bits unsafe.Pointer
	color, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bmi)),
		0 /* DIB_RGB_COLORS */, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == nil {
		return 0
	}
	copy(unsafe.Slice((*byte)(bits), len(px)), px)

	// AND mask：32bpp 图标的透明由 alpha 决定，mask 全 0；1bpp 位图的行按
	// WORD 对齐。
	stride := ((size + 15) / 16) * 2
	maskBits := make([]byte, stride*size)
	mask, _, _ := procCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1,
		uintptr(unsafe.Pointer(&maskBits[0])))
	if mask == 0 {
		procDeleteObject.Call(color)
		return 0
	}
	info := taskbarIconInfo{fIcon: 1, hbmMask: mask, hbmColor: color}
	icon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	// HICON 已复制位图内容，原始位图立即释放；HICON 本身进程期内一直持有。
	procDeleteObject.Call(mask)
	procDeleteObject.Call(color)
	return icon
}
