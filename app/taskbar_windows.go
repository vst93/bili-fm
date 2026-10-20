//go:build windows && (amd64 || arm64)

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
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
	procDestroyIcon        = taskbarUser32.NewProc("DestroyIcon")
	procDeleteObject       = taskbarGdi32.NewProc("DeleteObject")
	procSetWindowSubclass  = taskbarComctl32.NewProc("SetWindowSubclass")
	procDefSubclassProc    = taskbarComctl32.NewProc("DefSubclassProc")
	procRegGetValueW       = taskbarAdvapi32.NewProc("RegGetValueW")
	procGetDpiForWindow    = taskbarUser32.NewProc("GetDpiForWindow")
	procRegisterWindowMsgW = taskbarUser32.NewProc("RegisterWindowMessageW")
)

// ITaskbarList3 的 vtable 索引（ShObjIdl_core.h；前 3 个是 IUnknown）。
const (
	taskbarIdxRelease                    = 2
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

// taskbarReleaseIcons 释放一组 HICON。
func taskbarReleaseIcons(icons taskbarIcons) {
	for _, h := range []uintptr{icons.prev, icons.play, icons.pause, icons.next} {
		if h != 0 {
			procDestroyIcon.Call(h)
		}
	}
}

// taskbarButtonReady 报告是否已收到该窗口的 TaskbarButtonCreated。
func taskbarButtonReady(hwnd uintptr) bool {
	taskbarMu.Lock()
	defer taskbarMu.Unlock()
	return taskbarReadyHwnd == hwnd
}

// taskbarReleaseState 释放一个不再使用的 COM 对象与图标。
func taskbarReleaseState(st *taskbarThumbState) {
	if st == nil {
		return
	}
	taskbarReleaseIcons(st.icons)
	if st.taskbar != 0 {
		comCallTaskbar(st.taskbar, taskbarIdxRelease)
	}
}

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
	// taskbarWatchedHwnd 是已经装过子类化过程的窗口。同一个 HWND 只装一次：
	// SetWindowSubclass 用同一个 id 重复装会失败。
	taskbarWatchedHwnd uintptr
	// Explorer 的窗口消息。TaskbarButtonCreated 才是 ThumbBarAddButtons 的
	// 正确调用时机；首次显示前/Explorer 重启后它都会重新发一次。
	taskbarBtnCreatedMsg     uint32
	taskbarTaskbarCreatedMsg uint32
	// taskbarReadyHwnd 是已经收到过 TaskbarButtonCreated 的窗口。没收到时
	// syncTaskbar 用强制重挂，避免「Show 前/消息丢失」留下的假 added。
	taskbarReadyHwnd uintptr
)

// taskbarLogf 把任务栏工具栏的关键步骤写到
// %LOCALAPPDATA%\bili-FM\taskbar.log（同时保留 log.Printf）。
//
// 发布版是 windowsgui，没有控制台，任务栏按钮又只能在 Windows 实机验证；
// 如果用户仍看不到按钮，这个文件能直接说明：钩子装没装上、
// TaskbarButtonCreated 有没有到、AddButtons 返回了什么 HRESULT。
func taskbarLogf(format string, args ...any) {
	log.Printf(format, args...)
	dir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	path := filepath.Join(dir, "bili-FM", "taskbar.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(f, time.Now().Format("2006-01-02 15:04:05.000 ")+format+"\n", args...)
	_ = f.Close()
}

// taskbarRegisterMessages 注册 Explorer 的广播消息（进程内只做一次）。
func taskbarRegisterMessages() {
	if taskbarBtnCreatedMsg != 0 {
		return
	}
	reg := func(name string) uint32 {
		p, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			return 0
		}
		r, _, _ := procRegisterWindowMsgW.Call(uintptr(unsafe.Pointer(p)))
		return uint32(r)
	}
	taskbarBtnCreatedMsg = reg("TaskbarButtonCreated")
	taskbarTaskbarCreatedMsg = reg("TaskbarCreated")
}

// taskbarWatch 给窗口装子类化过程，接管 TaskbarButtonCreated / TaskbarCreated。
// 必须在 AddButtons 之前调用：Explorer 发来 TaskbarButtonCreated 时我们才能
// 在正确时机挂缩略图工具栏（只靠启动后的定时重试，Windows 11 上可能一直被
// 当成「任务栏按钮还没建好」而静默失败）。
func taskbarWatch(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	taskbarRegisterMessages()
	taskbarMu.Lock()
	if taskbarSubcl == 0 {
		taskbarSubcl = syscall.NewCallback(taskbarSubclassProc)
	}
	cb := taskbarSubcl
	watched := taskbarWatchedHwnd == hwnd
	taskbarMu.Unlock()
	if watched {
		return
	}
	if r, _, _ := procSetWindowSubclass.Call(hwnd, cb, taskbarSubclassID, 0); r == 0 {
		taskbarLogf("任务栏缩略图工具栏：SetWindowSubclass 失败，TaskbarButtonCreated 不会触发 hwnd=%#x", hwnd)
		return
	}
	taskbarMu.Lock()
	taskbarWatchedHwnd = hwnd
	taskbarMu.Unlock()
	taskbarLogf("任务栏缩略图工具栏：已装 TaskbarButtonCreated 钩子 hwnd=%#x", hwnd)
}

// taskbarInit 装缩略图工具栏。幂等：同一窗口已挂上就跳过；失败（窗口还没
// 显示、没有 comctl32 v6 等）保留重试机会（下一次 OnShow 再来）。
func taskbarInit(hwnd uintptr) { taskbarInitAt(hwnd, false) }

// taskbarReinit 无视 added 记录强制再走一次 ThumbBarAddButtons。
//
// 为什么需要：窗口在 Hidden 状态调用 ThumbBarAddButtons 时，Windows 可能
// 返回 S_OK 却因为任务栏按钮还没建好而**静默忽略**。此后 added=true 会让
// 所有定时重试 / OnShow 重挂全部跳过 —— 这正是用户连续四轮反馈「Windows
// 悬停任务栏没有播放快捷按钮」时盲重试失效的原因。启动后的几次重试改用
// 本函数强制添加：按钮已经真的存在时 Windows 会拒绝重复添加（保留原有
// 按钮），不会丢功能。
func taskbarReinit(hwnd uintptr) { taskbarInitAt(hwnd, true) }

func taskbarInitAt(hwnd uintptr, force bool) {
	if hwnd == 0 {
		return
	}
	taskbarWatch(hwnd)
	taskbarMu.Lock()
	defer taskbarMu.Unlock()
	st := taskbarCur
	same := st != nil && st.hwnd == hwnd
	if same && st.added && !force {
		return
	}
	taskbarLogf("任务栏缩略图工具栏：准备 AddButtons hwnd=%#x force=%v same=%v added=%v", hwnd, force, same, same && st.added)
	if taskbarSubcl == 0 {
		taskbarSubcl = syscall.NewCallback(taskbarSubclassProc)
	}
	// 强制重挂时复用同一窗口已经建好的 COM 对象 / 图标 / 状态，避免每次
	// 重试都泄漏一个 ITaskbarList3 与四个 HICON。
	taskbar, ok := uintptr(0), false
	if same {
		taskbar, ok = st.taskbar, st.taskbar != 0
	} else {
		taskbar, ok = taskbarCreate()
	}
	if !ok {
		return
	}
	size, dark, icons := taskbarIconSize(hwnd), taskbarSystemDark(), taskbarIcons{}
	playing, canPrev, canNext := false, true, true
	if same {
		size, dark, icons = st.size, st.dark, st.icons
		playing, canPrev, canNext = st.playing, st.canPrev, st.canNext
	} else {
		icons = taskbarBuildIcons(size, dark)
	}
	oldState := (*taskbarThumbState)(nil)
	if !same {
		oldState = taskbarCur
	}
	playIcon, playTip := icons.play, "播放"
	if playing {
		playIcon, playTip = icons.pause, "暂停"
	}
	buttons := [3]taskbarThumbButton{
		taskbarMakeButton(taskbarBtnPrev, icons.prev, "上一首", canPrev),
		taskbarMakeButton(taskbarBtnPlay, playIcon, playTip, true),
		taskbarMakeButton(taskbarBtnNext, icons.next, "下一首", canNext),
	}
	hr := comCallTaskbar(taskbar, taskbarIdxThumbBarAddButtons,
		hwnd, 3, uintptr(unsafe.Pointer(&buttons[0])))
	if taskbarFailedHR(hr) {
		taskbarLogf("任务栏缩略图工具栏：ThumbBarAddButtons 失败（HRESULT %#x，force=%v）", uint32(hr), force)
		// 已经有一份成功添加过的状态时保留它：重复添加被 Windows 拒绝
		// 往往说明按钮本来就在，不能让状态反而退回「未添加」，否则后续
		// 的图标 / 禁用态同步会全部失效。
		if !same {
			taskbarReleaseIcons(icons)
			if taskbar != 0 {
				comCallTaskbar(taskbar, taskbarIdxRelease)
			}
		}
		return
	}
	if oldState != nil {
		taskbarReleaseState(oldState)
	}
	if same {
		st.added = true
		taskbarLogf("任务栏缩略图工具栏：AddButtons 成功（复用状态） hwnd=%#x", hwnd)
		return
	}
	taskbarCur = &taskbarThumbState{
		hwnd: hwnd, taskbar: taskbar, icons: icons, size: size, dark: dark,
		canPrev: true, canNext: true, added: true,
	}
	taskbarLogf("任务栏缩略图工具栏：AddButtons 成功 hwnd=%#x buttons=3", hwnd)
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
	oldIcons := taskbarIcons{}
	if themeChanged {
		oldIcons = st.icons
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
	hr := comCallTaskbar(st.taskbar, taskbarIdxThumbBarUpdButtons,
		st.hwnd, 3, uintptr(unsafe.Pointer(&buttons[0])))
	if themeChanged && !taskbarFailedHR(hr) {
		// UpdateButtons 已经把新 HICON 交给任务栏；旧图标现在可以释放，
		// 否则每次系统深浅色切换都会泄漏 4 个 HICON。
		taskbarReleaseIcons(oldIcons)
	}
}

// taskbarCreate 建一个 ITaskbarList3 并 HrInit。失败返回 ok=false。
func taskbarCreate() (uintptr, bool) {
	var tl uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&taskbarCLSID)), 0, taskbarClsctxInprocServer,
		uintptr(unsafe.Pointer(&taskbarIIDList3)), uintptr(unsafe.Pointer(&tl)))
	if taskbarFailedHR(hr) || tl == 0 {
		taskbarLogf("任务栏缩略图工具栏：CoCreateInstance 失败（HRESULT %#x）", uint32(hr))
		return 0, false
	}
	if hr = comCallTaskbar(tl, taskbarIdxHrInit); taskbarFailedHR(hr) {
		taskbarLogf("任务栏缩略图工具栏：HrInit 失败（HRESULT %#x）", uint32(hr))
		return 0, false
	}
	return tl, true
}

// taskbarSubclassProc 只接管缩略图工具栏按钮的 THBN_CLICKED（WM_COMMAND
// 高位是通知码、低位是按钮 id），其余消息透传给窗口原来的过程。
func taskbarSubclassProc(hwnd, msg, wparam, lparam, subclassID, refData uintptr) uintptr {
	// Explorer 建好任务栏按钮（首次显示 / Explorer 重启后）会发这个消息。
	// 这是 ThumbBarAddButtons 的官方调用时机；处理完仍透传给 mygo 的窗口过程，
	// 让它自己的进度条状态也能应用。
	if taskbarBtnCreatedMsg != 0 && msg == uintptr(taskbarBtnCreatedMsg) {
		taskbarMu.Lock()
		taskbarReadyHwnd = hwnd
		taskbarMu.Unlock()
		taskbarLogf("任务栏缩略图工具栏：收到 TaskbarButtonCreated hwnd=%#x", hwnd)
		// 官方时机：无论启动时那次 AddButtons 返回什么，都在这里强制重挂。
		// Windows 在任务栏按钮还没建好时可能「接受」调用但不显示按钮；
		// 只按 added 判据会永远跳过，用户就看不到快捷控制。
		taskbarReinit(hwnd)
		r, _, _ := procDefSubclassProc.Call(hwnd, msg, wparam, lparam)
		return r
	}
	// Explorer 重启：旧的缩略图工具栏已经没了，清掉状态等下一次
	// TaskbarButtonCreated 重挂。
	if taskbarTaskbarCreatedMsg != 0 && msg == uintptr(taskbarTaskbarCreatedMsg) {
		taskbarLogf("任务栏缩略图工具栏：收到 TaskbarCreated（Explorer 重启）")
		taskbarMu.Lock()
		taskbarReadyHwnd = 0
		if st := taskbarCur; st != nil {
			st.added = false
		}
		taskbarMu.Unlock()
	}
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
