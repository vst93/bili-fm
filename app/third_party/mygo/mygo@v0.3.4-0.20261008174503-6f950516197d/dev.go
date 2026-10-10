package mygo

import (
	"net"
	"os"
	"sync"
	"time"
)

// production is set to "1" by `mygo build` through
// -ldflags "-X github.com/egoist/mygo.production=1".
var production string

// IsDev reports whether the app runs in development: true unless it was
// built for production with `mygo build`, or MYGO_ENV=production is set.
// Development builds enable the web inspector by default.
func IsDev() bool {
	if production == "1" {
		return false
	}
	return os.Getenv("MYGO_ENV") != "production"
}

// devReady reports to `mygo dev` that a build has started, so that a reload
// only stops the previous build once the new one is up. mygo dev passes a
// Unix socket in MYGO_READY_SOCKET; the app connects to it when its first
// window is ready to show, or right away when it opens no window.
var devReady = struct {
	once   sync.Once
	socket string
}{socket: readySocket()}

func readySocket() string {
	path := os.Getenv("MYGO_READY_SOCKET")
	// Not meant for child processes.
	os.Unsetenv("MYGO_READY_SOCKET")
	if production == "1" {
		return ""
	}
	return path
}

// launchedByDev reports whether `mygo dev` launched the app.
func launchedByDev() bool { return devReady.socket != "" }

func signalDevReady() {
	if devReady.socket == "" {
		return
	}
	devReady.once.Do(func() {
		go func() {
			conn, err := net.DialTimeout("unix", devReady.socket, time.Second)
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("ready\n"))
			conn.Close()
		}()
	})
}

// devReadyAfterLaunch runs once the ready listeners have returned.
func devReadyAfterLaunch() {
	if !launchedByDev() {
		return
	}
	if len(Windows()) == 0 {
		signalDevReady()
		return
	}
	// A window that never loads a page must not hold up reloads.
	time.AfterFunc(5*time.Second, signalDevReady)
}
