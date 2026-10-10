//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// TestKillSparesWebView2 has kill end a build and the processes it started
// but WebView2's, which the next build may use. The test binary plays the
// build (see TestMain), and a copy of it WebView2's browser.
func TestKillSparesWebView2(t *testing.T) {
	defer func(name string) { webviewProcess = name }(webviewProcess)
	webviewProcess = "fakewebview2.exe"
	webview := filepath.Join(t.TempDir(), webviewProcess)
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(webview, data, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "MYGO_FAKE_APP=tree", "MYGO_FAKE_WEBVIEW="+webview)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// The processes found are held by handle, so that the test never ends
	// another process that got one of their ids.
	var handles []syscall.Handle
	t.Cleanup(func() {
		for _, h := range handles {
			_ = syscall.TerminateProcess(h, 1)
			_, _ = syscall.WaitForSingleObject(h, 5000) // before the copy is removed
			_ = syscall.CloseHandle(h)
		}
	})
	find := func(parent uint32, name string) (uint32, syscall.Handle) {
		for pid, n := range childProcesses(parent) {
			if !strings.EqualFold(n, name) {
				continue
			}
			if h, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE|syscall.SYNCHRONIZE, false, pid); err == nil {
				handles = append(handles, h)
				return pid, h
			}
		}
		return 0, 0
	}
	build := uint32(cmd.Process.Pid)
	var helper, browser, renderer syscall.Handle
	var browserPID uint32
	for deadline := time.Now().Add(10 * time.Second); helper == 0 || renderer == 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			kill(cmd)
			_ = cmd.Wait()
			t.Fatal("the fake build's processes did not start")
		}
		if helper == 0 {
			_, helper = find(build, filepath.Base(os.Args[0]))
		}
		if browser == 0 {
			browserPID, browser = find(build, webviewProcess)
		}
		if browser != 0 && renderer == 0 {
			_, renderer = find(browserPID, webviewProcess)
		}
	}

	kill(cmd)
	_ = cmd.Wait()
	if !exits(helper, 5*time.Second) {
		t.Error("the build's own child still runs")
	}
	if exits(browser, 500*time.Millisecond) || exits(renderer, 0) {
		t.Error("WebView2's processes were ended")
	}
}

// exits reports whether a process exits within d.
func exits(h syscall.Handle, d time.Duration) bool {
	ev, err := syscall.WaitForSingleObject(h, uint32(d.Milliseconds()))
	return err == nil && ev == syscall.WAIT_OBJECT_0
}

// childProcesses returns the processes whose parent is pid, by name.
func childProcesses(pid uint32) map[uint32]string {
	out := map[uint32]string{}
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return out
	}
	defer syscall.CloseHandle(snap)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err := syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		if e.ParentProcessID == pid {
			out[e.ProcessID] = syscall.UTF16ToString(e.ExeFile[:])
		}
	}
	return out
}
