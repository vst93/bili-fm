//go:build windows && (amd64 || arm64)

package windows

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// Vtable indices of the WebView2 interfaces used here, from WebView2.h
// (SDK 1.0.4191.47). Each interface extends the previous version, so the
// indices stay valid for newer runtimes.
const (
	// ICoreWebView2Environment
	envCreateController          = 3
	envCreateWebResourceResponse = 4

	// ICoreWebView2Controller, ICoreWebView2Controller2
	ctlPutIsVisible                      = 4
	ctlPutBounds                         = 6
	ctlGetZoomFactor                     = 7
	ctlPutZoomFactor                     = 8
	ctlMoveFocus                         = 12
	ctlAddAcceleratorKeyPressed          = 19
	ctlNotifyParentWindowPositionChanged = 23
	ctlClose                             = 24
	ctlGetCoreWebView2                   = 25
	ctl2PutDefaultBackgroundColor        = 27

	// ICoreWebView2
	wvGetSettings                         = 3
	wvGetSource                           = 4
	wvNavigate                            = 5
	wvNavigateToString                    = 6
	wvAddNavigationStarting               = 7
	wvAddContentLoading                   = 9
	wvAddPermissionRequested              = 23
	wvAddNavigationCompleted              = 15
	wvAddProcessFailed                    = 25
	wvAddScriptToExecuteOnDocumentCreated = 27
	wvExecuteScript                       = 29
	wvCapturePreview                      = 30
	wvReload                              = 31
	wvAddWebMessageReceived               = 34
	wvCallDevToolsProtocolMethod          = 36
	wvGetCanGoBack                        = 38
	wvGetCanGoForward                     = 39
	wvGoBack                              = 40
	wvGoForward                           = 41
	wvStop                                = 43
	wvAddNewWindowRequested               = 44
	wvAddDocumentTitleChanged             = 46
	wvGetDocumentTitle                    = 48
	wvOpenDevToolsWindow                  = 51
	wvAddWebResourceRequested             = 55
	wvAddWebResourceRequestedFilter       = 57
	wvRemoveWebResourceRequestedFilter    = 58
	wvAddWindowCloseRequested             = 59

	// ICoreWebView2Settings, ICoreWebView2Settings2
	setPutIsScriptEnabled                = 4
	setPutIsWebMessageEnabled            = 6
	setPutAreDefaultScriptDialogsEnabled = 8
	setPutIsStatusBarEnabled             = 10
	setPutAreDevToolsEnabled             = 12
	setPutAreDefaultContextMenusEnabled  = 14
	setPutIsZoomControlEnabled           = 18
	set2GetUserAgent                     = 21
	set2PutUserAgent                     = 22

	// Event arguments and helpers.
	navStartingGetURI             = 3
	navStartingGetIsUserInitiated = 4
	navStartingPutCancel          = 8
	navCompletedGetIsSuccess      = 3
	navCompletedGetWebErrorStatus = 4
	msgTryGetWebMessageAsString   = 5
	msg2GetAdditionalObjects      = 6
	objectsGetCount               = 3
	objectsGetValueAtIndex        = 4
	fileGetPath                   = 3
	newWinGetURI                  = 3
	newWinPutHandled              = 6
	newWinGetWindowFeatures       = 10
	featGetHasPosition            = 3
	featGetHasSize                = 4
	featGetLeft                   = 5
	featGetTop                    = 6
	featGetHeight                 = 7
	featGetWidth                  = 8
	resReqGetRequest              = 3
	resReqPutResponse             = 5
	resReqGetDeferral             = 6
	resReqGetResourceContext      = 7
	reqGetURI                     = 3
	reqGetMethod                  = 5
	reqGetContent                 = 7
	reqGetHeaders                 = 9
	headersGetIterator            = 8
	iterGetCurrentHeader          = 3
	iterGetHasCurrentHeader       = 4
	iterMoveNext                  = 5
	deferralComplete              = 3
	procFailedGetKind             = 3
	wv4AddDownloadStarting        = 75  // ICoreWebView2_4
	wv13GetProfile                = 105 // ICoreWebView2_13
	profile2ClearBrowsingDataAll  = 12  // ICoreWebView2Profile2
	dlStartGetOperation           = 3
	dlStartPutCancel              = 5
	dlStartGetResultFilePath      = 6
	dlStartPutResultFilePath      = 7
	dlStartPutHandled             = 9
	dlOpAddStateChanged           = 7
	dlOpGetURI                    = 9
	dlOpGetState                  = 16
	dlOpGetInterruptReason        = 17
	permGetURI                    = 3
	permGetKind                   = 4
	permPutState                  = 7
	accelGetKeyEventKind          = 3
	accelGetVirtualKey            = 4
	accelPutHandled               = 8
)

var (
	iidICoreWebView2Controller2                  = guid("c979903e-d4ca-4228-92eb-47ee3fa96eab")
	iidICoreWebView2Settings2                    = guid("ee9a0f68-f46c-4e32-ac23-ef8cac224d2a")
	iidICoreWebView2WebMessageReceivedEventArgs2 = guid("06fc7ab7-c90c-4297-9389-33ca01cf6d5e")
	iidICoreWebView2File                         = guid("f2c19559-6bc1-4583-a757-90021be9afec")
	iidICoreWebView2_4                           = guid("20d02d59-6df2-42dc-bd06-f98a694b1302")
	iidICoreWebView2_13                          = guid("f75f09a8-667e-4983-88d6-c8773f315e84")
	iidICoreWebView2Profile2                     = guid("fa740d4b-5eae-4344-a8ad-74be31925397")
)

// The WebView2 Runtime channels, most stable first.
var webView2Channels = []string{
	"{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}", // Stable (Evergreen)
	"{2CD8A007-E189-409D-A2C8-9AF4EF3C72AA}", // Beta
	"{0D50BFEC-CD6A-4F9A-964C-C7416E3ACB10}", // Dev
	"{65C35B14-6C1D-4122-AC46-7148CC9D6497}", // Canary
}

const (
	hkeyCurrentUser  = 0x80000001
	hkeyLocalMachine = 0x80000002
	keyQueryValue    = 0x0001
	keyWow6432Key    = 0x0200
)

// errNoWebView2 explains how to get the runtime.
var errNoWebView2 = errors.New("mygo: the Microsoft Edge WebView2 Runtime is not installed; get it from https://go.microsoft.com/fwlink/p/?LinkId=2124703")

// createEnvironment starts creating the WebView2 environment; done runs on
// the main thread with it, or with an error.
func createEnvironment(userData string, done func(env uintptr, err error)) error {
	completed := func(hr, env uintptr) {
		if failed(hr) || env == 0 {
			done(0, hresultError("creating the WebView2 environment", hr))
			return
		}
		addRef(env)
		done(env, nil)
	}
	dataDir := u16(userData)

	// An official loader next to the executable wins.
	if exe, err := os.Executable(); err == nil {
		loader := filepath.Join(filepath.Dir(exe), "WebView2Loader.dll")
		if _, err := os.Stat(loader); err == nil {
			proc := syscall.NewLazyDLL(loader).NewProc("CreateCoreWebView2EnvironmentWithOptions")
			if proc.Find() == nil {
				hr := withHandler(completed, func(h uintptr) uintptr {
					r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(dataDir)), 0, h)
					return r
				})
				if failed(hr) {
					return hresultError("CreateCoreWebView2EnvironmentWithOptions", hr)
				}
				return nil
			}
		}
	}

	// Otherwise the installed runtime's client library, as the loader
	// does: a fixed version runtime named by
	// WEBVIEW2_BROWSER_EXECUTABLE_FOLDER, else the Evergreen runtime.
	client, runtimeType := "", uintptr(0)
	if dir := os.Getenv("WEBVIEW2_BROWSER_EXECUTABLE_FOLDER"); dir != "" {
		client, runtimeType = clientDLL(dir), 1
	} else {
		client = installedClientDLL()
	}
	if client == "" {
		return errNoWebView2
	}
	dll := syscall.NewLazyDLL(client)
	proc := dll.NewProc("CreateWebViewEnvironmentWithOptionsInternal")
	if err := proc.Find(); err != nil {
		return fmt.Errorf("mygo: unsupported WebView2 Runtime at %s: %w", client, err)
	}
	hr := withHandler(completed, func(h uintptr) uintptr {
		r, _, _ := proc.Call(1, runtimeType, uintptr(unsafe.Pointer(dataDir)), 0, h)
		return r
	})
	if failed(hr) {
		return hresultError("creating the WebView2 environment", hr)
	}
	return nil
}

func clientDLL(runtimeDir string) string {
	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	p := filepath.Join(runtimeDir, "EBWebView", arch, "EmbeddedBrowserWebView.dll")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// installedClientDLL finds the newest installed runtime, per machine or
// per user, in the channel order.
func installedClientDLL() string {
	for _, channel := range webView2Channels {
		best, bestVersion := "", []int(nil)
		for _, root := range []uintptr{hkeyLocalMachine, hkeyCurrentUser} {
			dir := regString(root, `SOFTWARE\Microsoft\EdgeUpdate\ClientState\`+channel, "EBWebView")
			if dir == "" {
				continue
			}
			if p := clientDLL(dir); p != "" {
				if v := parseVersion(filepath.Base(dir)); best == "" || compareVersions(v, bestVersion) > 0 {
					best, bestVersion = p, v
				}
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}

func parseVersion(s string) []int {
	var v []int
	for _, part := range strings.Split(s, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		v = append(v, n)
	}
	return v
}

func compareVersions(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// regString reads a string value from the 32-bit view of the registry,
// where EdgeUpdate records its clients.
func regString(root uintptr, path, name string) string {
	var key uintptr
	if r, _, _ := procRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(u16(path))), 0, keyQueryValue|keyWow6432Key, uintptr(unsafe.Pointer(&key))); r != 0 {
		return ""
	}
	defer procRegCloseKey.Call(key)
	var typ, size uint32
	if r, _, _ := procRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(u16(name))), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size))); r != 0 || size == 0 {
		return ""
	}
	buf := make([]uint16, size/2+1)
	if r, _, _ := procRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(u16(name))), 0, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// regDWORD reads a DWORD value, reporting whether it exists.
func regDWORD(root uintptr, path, name string) (uint32, bool) {
	var key uintptr
	if r, _, _ := procRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(u16(path))), 0, keyQueryValue, uintptr(unsafe.Pointer(&key))); r != 0 {
		return 0, false
	}
	defer procRegCloseKey.Call(key)
	var typ, v uint32
	size := uint32(4)
	if r, _, _ := procRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(u16(name))), 0, uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&size))); r != 0 {
		return 0, false
	}
	return v, true
}
