package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vst93/bili-fm/app/internal/imagecache"
	"github.com/vst93/bili-fm/app/internal/meminfo"
	"github.com/vst93/bili-fm/app/internal/view"
)

// TestMemInfoReportsGoHeap 钉住诊断口径：Go 活堆必须可读、且描述里带「Go 堆」。
func TestMemInfoReportsGoHeap(t *testing.T) {
	s := meminfo.Now()
	if s.HeapAlloc == 0 || s.Sys == 0 {
		t.Fatalf("Go 堆读数异常：%+v", s)
	}
	if !strings.Contains(s.String(), "Go 堆") {
		t.Fatalf("描述里没有 Go 堆：%q", s.String())
	}
	// 进程侧在 Linux/Windows 上必须拿得到（诊断的主体就是「Go 之外那部分」）。
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		if s.RSS == 0 {
			t.Fatalf("%s 上没读到进程常驻内存：%+v", runtime.GOOS, s)
		}
	}
}

// TestTrimMemoryCoalesces 钉住「冷却期内不重复回收」：切集时几个回收点连着来，
// FreeOSMemory 是同步全量 GC，连跑会把界面线程拖住。
func TestTrimMemoryCoalesces(t *testing.T) {
	trimGate.mu.Lock()
	trimGate.at, trimGate.running = time.Now(), false
	trimGate.mu.Unlock()
	trimMemory()
	trimGate.mu.Lock()
	running := trimGate.running
	trimGate.mu.Unlock()
	if running {
		t.Fatal("冷却期内的 trimMemory 不该再起一次 FreeOSMemory")
	}
	// 冷却过去后必须真的跑起来（反证：否则「回收」永远不发生）。
	trimGate.mu.Lock()
	trimGate.at = time.Now().Add(-2 * trimCooldown)
	trimGate.mu.Unlock()
	trimMemory()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		trimGate.mu.Lock()
		busy := trimGate.running
		trimGate.mu.Unlock()
		if busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("冷却过后 trimMemory 没有起 FreeJS 回收（FreeOSMemory）")
}

// TestMemReportIncludesBitmapCache 钉住报告里必须分得清「Go 堆」与「位图缓存」，
// 否则实机日志看不出内存花在哪。
func TestMemReportIncludesBitmapCache(t *testing.T) {
	cache := imagecache.New(nil)
	c := &controller{app: &view.App{Images: cache}}
	got := c.memReport()
	for _, want := range []string{"Go 堆", "位图缓存"} {
		if !strings.Contains(got, want) {
			t.Fatalf("报告缺 %q：%q", want, got)
		}
	}
}

// TestWebViewArgsShape 钉住给 WebView2 的参数：必须有 V8 老生代封顶（这是
// 「渲染进程 76MB 涨不上去」的那一条），且只在 Windows 上真的写进环境变量。
func TestWebViewArgsShape(t *testing.T) {
	if !strings.Contains(webviewMemoryArgs, "--max-old-space-size=") {
		t.Fatalf("WebView2 参数里没有 V8 老生代封顶：%q", webviewMemoryArgs)
	}
	const key = "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS"
	old, had := os.LookupEnv(key)
	t.Cleanup(func() {
		if had {
			os.Setenv(key, old)
			return
		}
		os.Unsetenv(key)
	})
	os.Unsetenv(key)
	applyWebViewArgs()
	got := os.Getenv(key)
	if runtime.GOOS == "windows" {
		if got != webviewMemoryArgs {
			t.Fatalf("Windows 上没把参数交给 WebView2 loader：%q", got)
		}
		return
	}
	if got != "" {
		t.Fatalf("非 Windows 不该设 WebView2 参数：%q", got)
	}
}

// TestMemoryTuningAppliesGCPercent 钉住 GC 收紧确实生效（用户显式设 GOGC 时不覆盖）。
func TestMemoryTuningAppliesGCPercent(t *testing.T) {
	if os.Getenv("GOGC") != "" {
		t.Skip("外部设了 GOGC，不覆盖是预期行为")
	}
	before := debugGCPercent()
	applyMemoryTuning()
	if got := debugGCPercent(); got != gcPercent {
		t.Fatalf("GOGC 没收到 %d，实际 %d（改前 %d）", gcPercent, got, before)
	}
}
