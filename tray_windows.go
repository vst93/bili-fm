//go:build windows

package main

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	shell32          = syscall.NewLazyDLL("shell32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procShellNotify  = shell32.NewProc("Shell_NotifyIconW")
	procCreateWindow = user32.NewProc("CreateWindowExW")
	procDefWindowProc = user32.NewProc("DefWindowProcW")
	procGetMessage   = user32.NewProc("GetMessageW")
	procTranslateMsg = user32.NewProc("TranslateMessage")
	procDispatchMsg  = user32.NewProc("DispatchMessageW")
	procPostQuitMsg  = user32.NewProc("PostQuitMessage")
	procRegisterClass = user32.NewProc("RegisterClassExW")
	procShowWindow   = user32.NewProc("ShowWindow")
	procSetForeground = user32.NewProc("SetForegroundWindow")
	procCreateMutex  = kernel32.NewProc("CreateMutexW")
	procCloseHandle  = kernel32.NewProc("CloseHandle")
)

const (
	WM_USER            = 0x0400
	WM_TRAYICON        = WM_USER + 1
	WM_COMMAND         = 0x0111
	WM_LBUTTONDBLCLK   = 0x0203
	WM_RBUTTONUP       = 0x0205
	WS_EX_APPWINDOW    = 0x00040000
	WS_OVERLAPPED      = 0x00000000
	CW_USEDEFAULT      = 0x80000000
	SW_RESTORE         = 9
	SW_HIDE            = 0
	NIF_ICON           = 0x00000002
	NIF_MESSAGE        = 0x00000001
	NIF_TIP            = 0x00000004
	NIF_INFO           = 0x00000010
	NIM_ADD            = 0x00000000
	NIM_MODIFY         = 0x00000001
	NIM_DELETE         = 0x00000002
	IDI_APPLICATION    = 32512
	MF_STRING          = 0x00000000
	MF_SEPARATOR       = 0x00000800
	TPM_RIGHTBUTTON    = 0x0002
	TPM_BOTTOMALIGN    = 0x0020
	GWL_STYLE          = -16
	GWL_EXSTYLE        = -20
	IMAGE_ICON         = 1
	LR_LOADFROMFILE    = 0x00000010
	ERROR_ALREADY_EXISTS = 183
)

type NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeoutOrVersion uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type POINT struct {
	X, Y int32
}

var (
	trayWindow    uintptr
	trayIcon      uintptr
	onShowWindow  func()
	onExit        func()
	trayOnce      sync.Once
)

// CheckSingleInstance 检查是否已有实例运行
func CheckSingleInstance(uniqueId string) (bool, uintptr) {
	mutexName, _ := syscall.UTF16PtrFromString("Global\\" + uniqueId)
	handle, _, _ := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))

	// 检查 GetLastError
	errno := syscall.GetLastError()
	if errno == ERROR_ALREADY_EXISTS {
		return false, handle
	}
	return true, handle
}

// InitTray 初始化系统托盘
func InitTray(appName string, showFn func(), exitFn func()) {
	trayOnce.Do(func() {
		onShowWindow = showFn
		onExit = exitFn

		// 注册窗口类
		className, _ := syscall.UTF16PtrFromString("BiliFMTrayClass")
		wndClass := WNDCLASSEX{
			CbSize:      uint32(unsafe.Sizeof(WNDCLASSEX{})),
			LpfnWndProc: syscall.NewCallback(wndProc),
			HInstance:    0,
			LpszClassName: className,
		}
		procRegisterClass.Call(uintptr(unsafe.Pointer(&wndClass)))

		// 创建隐藏窗口用于接收消息
		windowName, _ := syscall.UTF16PtrFromString(appName)
		hwnd, _, _ := procCreateWindow.Call(
			WS_EX_APPWINDOW,
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(windowName)),
			WS_OVERLAPPED,
			0, 0, 0, 0,
			0, 0, 0, 0,
		)
		trayWindow = hwnd

		// 加载图标（使用默认应用图标）
		iconHandle, _, _ := user32.NewProc("LoadIconW").Call(0, uintptr(IDI_APPLICATION))
		trayIcon = iconHandle

		// 创建托盘图标
		tip, _ := syscall.UTF16PtrFromString(appName)
		nid := NOTIFYICONDATA{
			CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATA{})),
			HWnd:             trayWindow,
			UID:              1,
			UFlags:           NIF_ICON | NIF_MESSAGE | NIF_TIP,
			UCallbackMessage: WM_TRAYICON,
			HIcon:            trayIcon,
		}
		copy(nid.SzTip[:], tip)
		procShellNotify.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))

		// 消息循环（在后台运行）
		go func() {
			var msg MSG
			for {
				ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
				if ret == 0 {
					break
				}
				procTranslateMsg.Call(uintptr(unsafe.Pointer(&msg)))
				procDispatchMsg.Call(uintptr(unsafe.Pointer(&msg)))
			}
		}()
	})
}

// wndProc 窗口消息处理
func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAYICON:
		switch lParam {
		case WM_LBUTTONDBLCLK:
			// 双击托盘图标恢复窗口
			if onShowWindow != nil {
				onShowWindow()
			}
		case WM_RBUTTONUP:
			// 右键弹出菜单
			showTrayMenu()
		}
	case WM_COMMAND:
		switch wParam {
		case 1: // 显示窗口
			if onShowWindow != nil {
				onShowWindow()
			}
		case 2: // 退出
			if onExit != nil {
				onExit()
			}
		}
	default:
		ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	}
	return 0
}

// showTrayMenu 显示托盘右键菜单
func showTrayMenu() {
	menuHandle, _, _ := user32.NewProc("CreatePopupMenu").Call()

	showText, _ := syscall.UTF16PtrFromString("显示窗口")
	exitText, _ := syscall.UTF16PtrFromString("退出")

	user32.NewProc("AppendMenuW").Call(menuHandle, MF_STRING, 1, uintptr(unsafe.Pointer(showText)))
	user32.NewProc("AppendMenuW").Call(menuHandle, MF_SEPARATOR, 0, 0)
	user32.NewProc("AppendMenuW").Call(menuHandle, MF_STRING, 2, uintptr(unsafe.Pointer(exitText)))

	var pt POINT
	user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pt)))

	procSetForeground.Call(trayWindow)
	user32.NewProc("TrackPopupMenu").Call(
		menuHandle,
		TPM_RIGHTBUTTON|TPM_BOTTOMALIGN,
		uintptr(pt.X), uintptr(pt.Y),
		0, trayWindow, 0,
	)
	user32.NewProc("DestroyMenu").Call(menuHandle)
}

// RemoveTray 移除托盘图标
func RemoveTray() {
	if trayWindow != 0 {
		nid := NOTIFYICONDATA{
			CbSize: uint32(unsafe.Sizeof(NOTIFYICONDATA{})),
			HWnd:   trayWindow,
			UID:    1,
		}
		procShellNotify.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
	}
}

// FindExistingWindow 查找已存在的窗口
func FindExistingWindow(className, windowName string) uintptr {
	classPtr, _ := syscall.UTF16PtrFromString(className)
	namePtr, _ := syscall.UTF16PtrFromString(windowName)
	hwnd, _, _ := user32.NewProc("FindWindowW").Call(
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(namePtr)),
	)
	return hwnd
}

// RestoreExistingWindow 恢复已存在的窗口
func RestoreExistingWindow(hwnd uintptr) {
	procShowWindow.Call(hwnd, SW_RESTORE)
	procSetForeground.Call(hwnd)
}

func init() {
	fmt.Println("Windows tray module loaded")
	os.Setenv("BILI_FM_WINDOWS", "1")
}
