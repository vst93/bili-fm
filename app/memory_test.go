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
	// 进程侧必须拿得到（诊断的主体就是「Go 之外那部分」）。macOS 走
	// proc_pid_rusage 的 phys_footprint，Linux 走 VmRSS，Windows 走
	// GetProcessMemoryInfo。
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		if s.RSS == 0 {
			t.Fatalf("%s 上没读到进程内存：%+v", runtime.GOOS, s)
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

// TestTrimMemoryOnBlurWired 钉住「窗口失焦」这个回收时刻真的接上了。
//
// 用户 2026-10-19 ① 要求的关键清理路径是：关闭视频、切换视频、翻页、抽屉开关、
// **窗口失焦**。此前 memory.go 里只接了四个时刻（关视频 / 收托盘 / 开视频 /
// 换抽屉），失焦那条没接 —— 用户 alt-tab 走开、最小化之后，那一屏封面位图和
// 抽屉里那几页列表留下的空页仍然挂在进程里，Windows 的私有工作集照算。
func TestTrimMemoryOnBlurWired(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go：%v", err)
	}
	s := string(src)
	if !strings.Contains(s, "OnBlur(trimMemory)") {
		t.Fatal("窗口失焦没有接 trimMemory（用户 10-19 ① 的关键路径之一）")
	}
	// 反证一：失焦回收必须走**同一个** trimMemory（冷却 + 单飞都在它里面），
	// 不许另起一份同步 FreeOSMemory —— 那会在 alt-tab 时把界面线程卡住。
	if strings.Contains(s, "debug.FreeOSMemory") {
		t.Error("main.go 里出现了第二份 FreeOSMemory，应统一走 trimMemory 的异步 + 冷却")
	}
	// 反证二：收托盘那条回收点仍在（失焦是新增，不是替换）。
	if !strings.Contains(s, "app.Win.Hide()\n\t\t\ttrimMemory()") {
		t.Error("收托盘时的 trimMemory 被删了")
	}
}

// TestBackgroundMemoryHooksWired 钉住「隐藏到托盘 / 最小化」这两个真正的后台
// 时刻：停掉界面构建、清图片缓存、FreeOSMemory。只在失焦时 trim 不够 ——
// 用户收托盘后仍看到内存不降，而且旋转封面会继续推进动画。
func TestBackgroundMemoryHooksWired(t *testing.T) {
	mainSrc := flattenCode(t, "main.go")
	for _, want := range []string{
		"app.Win.OnHide(func(){c.setBackground(true)})",
		"app.Win.OnMinimize(func(){c.setBackground(true)})",
		"app.Win.OnShow(func(){c.setBackground(false)})",
		"app.Win.OnRestore(func(){c.setBackground(false)})",
	} {
		if !strings.Contains(mainSrc, want) {
			t.Errorf("后台生命周期没接上：%s", want)
		}
	}
	memSrc := flattenCode(t, "memory.go")
	if !strings.Contains(memSrc, "c.app.SetBackground(on)") {
		t.Error("setBackground 没有把状态交给 view（Shell 不会停构建）")
	}
}

// TestRenderBackendLowMemoryOptIn 钉住渲染后端的取舍：默认 GPU；只有显式
// BILIFM_LOW_MEMORY=1 / BILIFM_GPU=0 才换 CPU（内存优先、帧时间变差），
// 外部已设 MYGO_GPU 时完全尊重。
//
// 实测依据（800×600、2x、连续重绘）：Metal p50 8.7ms / p90 10ms / 137MB；
// CPU p50 15ms / p90 33ms / 100MB。默认不能为了内存牺牲顺滑。
func TestRenderBackendLowMemoryOptIn(t *testing.T) {
	clear := func() {
		t.Setenv("MYGO_GPU", "")
		os.Unsetenv("MYGO_GPU")
		t.Setenv("BILIFM_GPU", "")
		os.Unsetenv("BILIFM_GPU")
		t.Setenv("BILIFM_LOW_MEMORY", "")
		os.Unsetenv("BILIFM_LOW_MEMORY")
	}

	// 默认：什么都不设，交给框架（macOS 上是 Metal）。
	clear()
	applyRenderBackend()
	if got := os.Getenv("MYGO_GPU"); got != "" {
		t.Fatalf("默认不该设 MYGO_GPU，实际 %q", got)
	}

	// 显式低内存：所有平台都切到 CPU 光栅化（用户主动选择，默认仍 GPU）。
	clear()
	t.Setenv("BILIFM_LOW_MEMORY", "1")
	applyRenderBackend()
	want := "0"
	if got := os.Getenv("MYGO_GPU"); got != want {
		t.Fatalf("BILIFM_LOW_MEMORY=1 时 MYGO_GPU=%q，want %q（%s）", got, want, runtime.GOOS)
	}

	// BILIFM_GPU=0 是同一件事的另一种写法。
	clear()
	t.Setenv("BILIFM_GPU", "0")
	applyRenderBackend()
	if got := os.Getenv("MYGO_GPU"); got != want {
		t.Fatalf("BILIFM_GPU=0 时 MYGO_GPU=%q，want %q", got, want)
	}

	// BILIFM_GPU=1：显式要 GPU，不设 MYGO_GPU。
	clear()
	t.Setenv("BILIFM_GPU", "1")
	applyRenderBackend()
	if got := os.Getenv("MYGO_GPU"); got != "" {
		t.Fatalf("BILIFM_GPU=1 时不该设 MYGO_GPU，实际 %q", got)
	}

	// 外部显式设了 MYGO_GPU：尊重它。
	clear()
	t.Setenv("MYGO_GPU", "1")
	applyRenderBackend()
	if got := os.Getenv("MYGO_GPU"); got != "1" {
		t.Fatalf("外部设的 MYGO_GPU 被覆盖成了 %q", got)
	}
}

// TestMemoryWatchDoesNotRequestFrames 钉住 memoryWatch 不再每秒请求一帧。
//
// 它是本轮内存问题的根因：每秒一次 Win.Update 读 Drawer，让渲染器永远
// 等不到「连续 2 秒没有帧」的 idle trim；macOS 上那份 Metal 工作集约 45MB，
// 于是「省内存的观察器」反而把内存按在高位。换抽屉的回收改由视图回调。
func TestMemoryWatchDoesNotRequestFrames(t *testing.T) {
	src := flattenCode(t, "memory.go")
	if strings.Contains(src, "time.NewTicker(time.Second)") {
		t.Fatal("memoryWatch 里还有每秒一次的 ticker（会不断请求帧、挡住 idle trim）")
	}
	if !strings.Contains(src, "time.NewTicker(15*time.Second)") {
		t.Fatal("memoryWatch 没有 15 秒的 -memstats 日志节奏")
	}
	if !strings.Contains(src, "ifmemStats==nil||!*memStats{") {
		t.Error("没开 -memstats 时 memoryWatch 应立即返回，不该起 goroutine")
	}
	if strings.Contains(src, "app.Drawer") {
		t.Error("memoryWatch 不该再读界面字段（那需要 Win.Update，会请求帧）")
	}
}

// TestDrawerChangeTrimWired 钉住「换抽屉主动回收」没有随着轮询一起消失：
// 视图在 Shell 里发现 Drawer 变化后回调，主程序把它接到 trimMemory。
func TestDrawerChangeTrimWired(t *testing.T) {
	mainSrc := flattenCode(t, "main.go")
	if !strings.Contains(mainSrc, "app.OnDrawerChange=trimMemory") {
		t.Fatal("main.go 没把 App.OnDrawerChange 接到 trimMemory")
	}
	viewSrc := flattenCode(t, "internal/view/shell.go")
	for _, want := range []string{
		"ifa.Drawer!=a.drawerWatched{",
		"a.OnDrawerChange()",
	} {
		if !strings.Contains(viewSrc, want) {
			t.Errorf("Shell 里的抽屉变化回调缺 %q", want)
		}
	}
}

// TestDMABUFRendererEnvOverride 钉住 mygo 0.8.0 的 `App.SetDMABUFRenderer` 用法：
// Linux 上默认交给框架判断（DMABUFRendererAuto），并支持 BILIFM_DMABUF 强制
// 关/开（排查「视频弹窗白屏」时最有用）。
//
// 背景：mygo 默认把 WebKitGTK 的 DMA-BUF 渲染器关掉（NVIDIA / 虚拟机 / 容器里
// 会画出空白窗口），而视频弹窗正是唯一用 WebKitGTK 的地方。能用的主机上开着它，
// 页面滚动帧率是关着的数倍、CPU 也更低。
//
// 这里只验「选哪个档」这条纯逻辑，不真的调框架（那需要运行中的应用）。
func TestDMABUFRendererEnvOverride(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want string // "off" / "on" / "auto"
	}{
		{"", "auto"},
		{"off", "off"},
		{"0", "off"},
		{"false", "off"},
		{"on", "on"},
		{"1", "on"},
		{"true", "on"},
		{"AUTO", "auto"},    // 大小写不敏感
		{" off ", "off"},    // 两端空白要吃掉
		{"garbage", "auto"}, // 未知值退回自动判断
	} {
		if got := dmabufRendererFor(tc.env); got != tc.want {
			t.Errorf("BILIFM_DMABUF=%q → %q，想 %q", tc.env, got, tc.want)
		}
	}
}

// TestApplyDMABUFRendererSkipsOtherPlatforms 保证非 Linux 上不碰这个开关
// （macOS / Windows 根本没有 WebKitGTK，也没有 DMA-BUF）。
func TestApplyDMABUFRendererSkipsOtherPlatforms(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux 上会真的设置框架选项，见下一条测试的用途")
	}
	// 非 Linux：函数是 no-op，调用不该 panic。
	applyDMABUFRenderer()
}

// TestDMABUFRendererSetBeforeRun 钉住**调用时机**：`applyDMABUFRenderer()` 必须在
// `mygo.App.Run()` 之前调用，而且不能落在 `WhenReady` 回调内部。
//
// 为什么必须测时机：框架把这项设置随 AppOptions 交给后端，Linux 后端在 Init 里
// 据此决定要不要设 WEBKIT_DISABLE_DMABUF_RENDERER=1（GTK 初始化前后的事），之后
// 再改字段就没有任何地方读了。第一版写在 WhenReady 里 —— 代码在、测试绿、能力
// 完全没生效（用户可见效果为零）。这类「静默失效」只能靠检查调用点发现。
func TestDMABUFRendererSetBeforeRun(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败: %v", err)
	}
	code := string(src)

	callIdx := strings.Index(code, "applyDMABUFRenderer()")
	if callIdx < 0 {
		t.Fatal("main.go 里没有调用 applyDMABUFRenderer()（Linux 视频弹窗的 DMA-BUF 没接上）")
	}
	runIdx := strings.Index(code, "mygo.App.Run()")
	if runIdx < 0 {
		t.Fatal("找不到 mygo.App.Run()")
	}
	if callIdx > runIdx {
		t.Error("applyDMABUFRenderer() 必须在 mygo.App.Run() 之前调用" +
			"（后端只在 Init 里读这项设置，之后再改就无效）")
	}

	// 反证：不许落在 WhenReady 的闭包**内部**（那是 Run 之后才跑的 ready 回调）。
	// 光比字符串位置不够 —— WhenReady(...) 在文件里出现得更早，所以要按大括号
	// 深度判：从回调起点数到调用点，深度 > 0 就是嵌在里面。
	readyIdx := strings.Index(code, "mygo.App.WhenReady(func() {")
	if readyIdx >= 0 && callIdx > readyIdx {
		depth := 0
		for i := readyIdx; i < callIdx; i++ {
			switch code[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		if depth > 0 {
			t.Error("applyDMABUFRenderer() 落在 WhenReady 回调内部 —— 那会在 Run 之后才执行，" +
				"框架早就按默认值把 WEBKIT_DISABLE_DMABUF_RENDERER 设好了，等于没设")
		}
	}
}
