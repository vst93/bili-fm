// 内存策略：GC 目标、关键路径的主动回收、WebView2 的浏览器参数、诊断日志。
//
// 背景（用户 2026-10-18 晚间 ⑥ / 10-19 凌晨 ①）：Windows 上播放视频 200+MB、
// 切几个视频 + 翻几个抽屉后 315MB，与原版 Tauri 版持平甚至更高。前几轮的实测
// 已经把构成拆清了（见 .pending-bugs.md 的内存小节）：
//
//	Go 活堆           ~8–16MB   ← 应用代码真正持有的部分
//	位图缓存（封顶）    ≤12MB     ← imagecache，CPU 侧；GPU 另有一份镜像
//	Go 运行时保留页    ~20MB     ← 未还给 OS 的空页
//	系统库 / 图形驱动  ~95MB     ← GTK+DLL / D3D11+DWrite+CRT，改不动
//	WebView2 进程组    100–220MB ← 只在「打开视频」时起，关掉即落
//
// 所以「只优化图片缓存」不可能把 315MB 拉下来：本文件做的是**能控制的那三块**
// —— 收紧 GC 目标（活堆不变、保留页少一倍）、在「刚丢掉一大堆东西」的时刻主动
// 把页还给 OS、以及给视频弹窗的 WebView2 加内存向的浏览器参数（V8 老生代封顶，
// 视频页只有一个 <video>，用不到 1GB 的默认老生代）。
//
// macOS（2026-10-20 实测，800×600 主窗、Retina 2x）：
//
//	Go 活堆           ~8–11MB   ← 应用代码真正持有的部分
//	位图缓存（封顶）    ≤12MB     ← imagecache，CPU 侧
//	AppKit / CoreText  ~30MB     ← 每个窗口都有，与内容无关
//	Metal GPU 渲染器   45MB +    ← 只要帧持续到来就一直押着（见下）
//	drawable          7.5MB/个   ← 默认 3 个
//
// 那 45MB 是 Metal 侧的工作集（命令缓冲 / 管线 / 驱动页），**只在停止绘制
// 2 秒后被回收**；播放时唱片每秒推进、进度每 500ms 更新一次，帧永远不断，
// 于是它一直算在进程足迹里。最小窗口（360×260）也同样是这 45MB，与窗口
// 内容和尺寸无关 —— 应用代码压不动。
//
// 框架的 CPU 光栅化（MYGO_GPU=0）没有这份 45MB，连续重绘时 ~100MB；但它的
// present 是整窗位图上传：实测 2x 800×600 下 p50 15ms / p90 33ms，而 Metal
// 路径 p50 8.7ms / p90 10ms。也就是说「<100MB」与「60fps 顺滑」在当前
// mygo 的 macOS 渲染器上不能同时成立，所以 CPU 只作为**显式低内存模式**
// （BILIFM_LOW_MEMORY=1）提供，默认仍走 GPU。真正的解法是上游降低 Metal
// 工作集（或让 CPU present 支持脏矩形/更小的位图），不是应用层能改的。
// Windows / Linux 不受影响，仍走 GPU。
package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/vst93/bili-fm/app/internal/imagecache"
	"github.com/vst93/bili-fm/app/internal/meminfo"
)

// memStats 对应 -memstats（在 main 里注册）：打开后 memoryWatch 每 15 秒打
// 一行内存构成。包级变量是因为 memoryWatch 在另一个文件里读它。
var memStats *bool

// gcPercent 是收紧后的 GOGC。默认 100 意味着「活堆翻倍才回收」：活堆 16MB
// 时堆可以长到 32MB 才开始收，Windows 的私有工作集按提交页算，那多出来的
// 一截就是用户看到的数字。50 把天花板砍到 1.5 倍，代价是 GC 次数翻倍
// （活堆 16MB 的一次 GC 是亚毫秒级，界面线程无感）。用户显式设了 GOGC 就
// 不覆盖。
const gcPercent = 50

// memoryLimit 是 Go 堆的软上限：活堆正常只有几十 MB，这个值只用来兜住
// 「解码一批封面 + 一次性塞进几百条列表数据」时的尖峰 —— 逼近它时 GC 会
// 提前跑，而不是先把堆吹大再收。低于它时完全不影响调度。
const memoryLimit = 192 << 20

// applyMemoryTuning 在创建任何窗口之前调一次。
func applyMemoryTuning() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(gcPercent)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(memoryLimit)
	}
}

// webviewMemoryArgs 是给视频弹窗的 WebView2 加的参数（Chromium 开关）。
//
// 视频弹窗只是一个 <video> 页面（internal/video 的 pageURL 生成的静态页），
// 但 WebView2 默认给渲染进程 ~1–2GB 的 V8 老生代预算，堆可以一直长；实测
// 渲染进程 76MB 里有一半是这种「没人用但没到上限就不收」的页。
//
//   - --js-flags=--max-old-space-size=64：把 V8 老生代封到 64MB。页面只存
//     一个播放状态对象，64MB 绰绰有余；到上限就回收，不涨到几百 MB。
//   - --disable-features=msWebOOUI,msPdfOOUI：去掉内置的下载/PDF 工具栏
//     （视频页永远用不到，微软自己的 WebView2 示例也这么关）。
//
// 只在 Windows 生效（WebView2 的 loader 读这个环境变量）；其它平台没有
// WebView2。用户可用 BILIFM_WEBVIEW_ARGS 覆盖，设成 "off" 表示不加任何参数。
var webviewMemoryArgs = "--js-flags=--max-old-space-size=64 --disable-features=msWebOOUI,msPdfOOUI"

// applyWebViewArgs 在**第一次创建 webview 之前**把浏览器参数交给 WebView2。
//
// 框架（mygo v0.5.5）的 environment options 把 put_AdditionalBrowserArguments
// 写成了 ignore，没有 Go 侧入口；但 WebView2 的 loader 本来就认
// WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS 这个环境变量（微软文档列出的三个
// loader 环境变量之一），且只在创建 environment 时读一次 —— 主窗是原生 UI、
// 不创建 webview，所以进程启动时设上就一定赶在视频弹窗之前。
func applyWebViewArgs() {
	if runtime.GOOS != "windows" {
		return
	}
	if os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS") != "" {
		return // 用户/宿主已显式指定，尊重它
	}
	args := webviewMemoryArgs
	if v, ok := os.LookupEnv("BILIFM_WEBVIEW_ARGS"); ok {
		args = v
	}
	if args == "" || args == "off" {
		return
	}
	_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", args)
}

// renderBackendEnv 是给 mygo 的渲染后端开关：MYGO_GPU=0 表示用 CPU 光栅化。
// 框架在**第一次画帧**时读它（ui/window.go 的 makeGPU），所以只要在
// mygo.NewWindow / Show 之前设置就一定生效。
const renderBackendEnv = "MYGO_GPU"

// applyRenderBackend 处理 macOS 的显式「低内存模式」。
//
// 默认仍走 GPU：实测（800×600、2x、播放列表 + 连续唱片自转）
//
//	Metal：p50 8.7ms / p90 10.0ms，进程足迹 ~137MB
//	CPU  ：p50 15.0ms / p90 33.0ms / p99 62ms，进程足迹 ~100MB
//
// CPU 少的那 ~45MB 是 Metal 的工作集（只有停止绘制 2 秒才释放，播放时帧
// 不断，所以一直押着）。换成 CPU 能把内存压到 100MB 以内，但 present 是
// 整窗位图上传，帧时间明显变差 —— 所以只给显式要求的人用，不能默认开。
//
// 开关（仅 macOS）：
//   - 默认：GPU；
//   - BILIFM_LOW_MEMORY=1（或 BILIFM_GPU=0）：CPU 光栅化，内存优先；
//   - 外部已设 MYGO_GPU：完全尊重，不覆盖；
//   - 其它平台：什么都不做。
func applyRenderBackend() {
	if runtime.GOOS != "darwin" {
		return
	}
	if os.Getenv(renderBackendEnv) != "" {
		return // 用户 / 宿主已经显式指定后端
	}
	lowMemory := strings.ToLower(strings.TrimSpace(os.Getenv("BILIFM_LOW_MEMORY")))
	gpu := strings.ToLower(strings.TrimSpace(os.Getenv("BILIFM_GPU")))
	switch {
	case gpu == "0", gpu == "false", gpu == "off", gpu == "cpu":
	case lowMemory == "1", lowMemory == "true", lowMemory == "on", lowMemory == "yes":
	default:
		return // 默认 GPU
	}
	_ = os.Setenv(renderBackendEnv, "0")
}

// trimCooldown 是两次主动回收之间的最短间隔。切视频 + 翻抽屉会连着触发好几个
// 回收点，FreeOSMemory 是同步全量 GC，连跑几次会把界面线程拖住。
const trimCooldown = 400 * time.Millisecond

var trimGate struct {
	mu      sync.Mutex
	at      time.Time
	running bool
}

// trimMemory 把 Go 的堆立刻还给操作系统。
//
// 为什么需要它：Go 的堆只涨到「下次 GC 的目标」才回收，而被丢弃的位图/列表
// 数据留下的空页在 Windows 上仍然算进进程的私有工作集（任务管理器看到的数字
// 就是它）。用户实测「多切几个视频、多翻几个抽屉后 315MB」里，有一份是这种
// 「已经没人引用、但还没还给系统」的页。值得主动还的时刻：
//
//   - 视频弹窗关闭：刚丢掉整窗的渲染缓存与视频调度的临时缓冲；
//   - 窗口隐藏到托盘：用户已经不看界面了，早还晚还都一样；
//   - 打开视频弹窗：马上要多背一整套 WebView2，先把原生侧的闲页还掉；
//   - 换抽屉：上一个抽屉那一屏的封面位图已经不在界面上。
//
// 异步跑（FreeOSMemory 会同步做一次完整 GC），并带冷却 —— 上面这些时刻在
// 「切集」时往往连着来。
func trimMemory() {
	trimGate.mu.Lock()
	if trimGate.running || time.Since(trimGate.at) < trimCooldown {
		trimGate.mu.Unlock()
		return
	}
	trimGate.running, trimGate.at = true, time.Now()
	trimGate.mu.Unlock()
	go func() {
		debug.FreeOSMemory()
		trimGate.mu.Lock()
		trimGate.running = false
		trimGate.mu.Unlock()
	}()
}

// setBackground 处理「窗口隐藏到托盘 / 最小化」：停止界面构建、清掉界面持有
// 的封面位图与图片缓存（包括 480w 唱片封面和旋转动画的纹理），再把 Go 堆还给
// 系统。用户实测后台内存不降、且怀疑旋转封面时，这条就是主动收口点。
func (c *controller) setBackground(on bool) {
	if c.app == nil {
		return
	}
	c.app.SetBackground(on)
	if c.app.Win != nil {
		// 隐藏时用空界面重建一次，释放上一帧留下的位图/纹理引用；显示时也
		// 补一帧，避免第一帧是空白。
		c.app.Win.Update(func() {})
	}
	if on {
		trimMemory()
	}
}

// memReport 拼一行内存构成：Go 侧 + 进程侧 + 位图缓存。
func (c *controller) memReport() string {
	s := meminfo.Now()
	parts := []string{s.String()}
	if c.mp != nil {
		parts = append(parts, fmt.Sprintf("欠载 %d", c.mp.Underruns()))
	}
	if c.app != nil && c.app.Images != nil {
		if cache, ok := c.app.Images.(*imagecache.Cache); ok {
			parts = append(parts, fmt.Sprintf("位图缓存 %d 张 / %s（在途 %d）",
				cache.Len(), meminfo.MB(uint64(cache.Bytes())), cache.Pending()))
		}
	}
	return strings.Join(parts, " | ")
}

// memoryWatch 是 -memstats 的诊断日志：每 15 秒打一行内存构成。
//
// 它**不再轮询界面状态**：此前每秒一次 Win.Update 读 app.Drawer，而那等于
// 每秒请求一帧 —— 渲染器的 idle trim 要求连续 2 秒没有帧，于是永远不触发，
// 45MB 的 Metal 工作集就被一直押着。换抽屉的回收改由视图在 Shell 里通过
// App.OnDrawerChange 通知（见 state.go / shell.go），只在真的换抽屉时发生。
//
// 日志只读 Go 堆 / 进程足迹 / 位图缓存这些线程安全的口径，不碰界面字段，
// 所以不需要（也不能）走 Win.Update。
func (c *controller) memoryWatch() {
	if c.app == nil || c.app.Win == nil {
		return
	}
	if memStats == nil || !*memStats {
		return
	}
	go func() {
		// 首帧 / 首屏封面与字体图集还没就绪：等 5 秒再打第一行，免得报一个
		// 「界面还没画」的低值。
		time.Sleep(5 * time.Second)
		log.Printf("内存 | %s", c.memReport())
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for range t.C {
			log.Printf("内存 | %s", c.memReport())
		}
	}()
}
