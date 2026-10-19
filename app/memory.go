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

// memoryWatch 每秒看一眼界面状态：换抽屉就主动回收一次；-memstats 打开时
// 每 15 秒打一行构成。
//
// 读界面状态必须走 Win.Update（界面线程），不能在这个 goroutine 里直接读
// app.Drawer —— 那是界面线程写的字段。
func (c *controller) memoryWatch() {
	if c.app == nil || c.app.Win == nil {
		return
	}
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		var lastDrawer string
		primed := false
		var lastReport time.Time
		for range t.C {
			c.app.Win.Update(func() {
				d := c.app.Drawer
				if !primed {
					lastDrawer, primed = d, true
					return
				}
				if d != lastDrawer {
					lastDrawer = d
					trimMemory()
				}
			})
			if memStats != nil && *memStats && time.Since(lastReport) >= 15*time.Second {
				lastReport = time.Now()
				log.Printf("内存 | %s", c.memReport())
			}
		}
	}()
}
