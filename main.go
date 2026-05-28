package main

import (
	"bilifm/service"
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// proxyReady 用于同步：代理服务器启动完成后关闭此 channel
var proxyReady = make(chan struct{})

//go:embed all:frontend/dist
var assets embed.FS

type GithubRelease struct {
	TagName string `json:"tag_name"`
	HtmlUrl string `json:"html_url"`
	Assets  []struct {
		BrowserDownloadUrl string `json:"browser_download_url"`
	} `json:"assets"`
}

// dnsCache DNS 缓存，避免 Windows 下重复解析同一域名（Windows IPv6 DNS 超时 fallback 很慢）
var dnsCache = make(map[string]string)
var dnsCacheMu sync.RWMutex

// 自定义 HTTP Transport：连接池 + 超时 + keep-alive + DNS 缓存
var proxyTransport = &http.Transport{
	TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	MaxIdleConns:          50,
	MaxIdleConnsPerHost:   20,
	MaxConnsPerHost:       30,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:  10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, addr)
		}
		// 查缓存（读锁）
		dnsCacheMu.RLock()
		if cached, ok := dnsCache[host]; ok {
			dnsCacheMu.RUnlock()
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp4", net.JoinHostPort(cached, port))
		}
		dnsCacheMu.RUnlock()
		// 首次解析，使用 LookupIP 精确筛选 IPv4，避免 Windows IPv6 DNS 超时
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
		if err != nil {
			// fallback: 尝试全部地址
			ips, err = net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
		if err != nil {
			return nil, err
		}
		if len(ips) > 0 {
			ip := ips[0].String()
			dnsCacheMu.Lock()
			dnsCache[host] = ip
			dnsCacheMu.Unlock()
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp4", net.JoinHostPort(ip, port))
		}
		return nil, fmt.Errorf("no addresses for %s", host)
	},
}

var proxyClient = &http.Client{
	Transport: proxyTransport,
	Timeout:   15 * time.Second,
}

// prewarmDNS 预解析 B站 CDN 域名，避免首次图片请求卡在 DNS
func prewarmDNS() {
	hosts := []string{"i0.hdslb.com", "i1.hdslb.com", "i2.hdslb.com"}
	for _, h := range hosts {
		ips, err := net.DefaultResolver.LookupIP(context.Background(), "ip4", h)
		if err != nil {
			ips, _ = net.DefaultResolver.LookupIP(context.Background(), "ip", h)
		}
		if len(ips) > 0 {
			dnsCacheMu.Lock()
			dnsCache[h] = ips[0].String()
			dnsCacheMu.Unlock()
			println("DNS prewarm:", h, "->", ips[0].String())
		}
	}
}

// 图片代理处理函数
func imageProxyHandler(w http.ResponseWriter, r *http.Request) {
	// 从查询参数中获取图片 URL（前端使用 encodeURIComponent 编码）
	imageURL := r.URL.Query().Get("url")
	if imageURL == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	// 确保 URL 有协议前缀
	if len(imageURL) > 2 && imageURL[:2] == "//" {
		imageURL = "https:" + imageURL
	}

	// 创建带自定义 headers 的请求，避免被 B站 CDN 拒绝
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		http.Error(w, "Invalid image URL", http.StatusBadRequest)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")

	// 带重试的上游请求（最多 3 次），每次重试创建新 request 避免连接状态残留
	var resp *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			req, _ = http.NewRequest("GET", imageURL, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
			req.Header.Set("Referer", "https://www.bilibili.com/")
			req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
			time.Sleep(time.Duration(200*(attempt)) * time.Millisecond)
		}
		resp, err = proxyClient.Do(req)
		if err == nil {
			break
		}
		println("image-proxy: attempt", attempt+1, "failed:", err.Error())
	}
	if err != nil {
		println("image-proxy: all attempts failed for", imageURL, ":", err.Error())
		http.Error(w, "Failed to fetch image after retries", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		println("image-proxy: upstream returned", resp.StatusCode, "for", imageURL)
		http.Error(w, fmt.Sprintf("Upstream returned %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	// 将图片数据写入响应
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		// 客户端断开连接等，只打印日志，不回写 error（headers 已发送）
		println("image-proxy: write error:", err.Error())
	}
}

func main() {
	// 单实例检测（Windows 特定）
	isFirst, mutexHandle := checkSingleInstanceWindows()
	if !isFirst {
		// 已有实例运行，尝试恢复其窗口
		hwnd := findExistingWindow()
		if hwnd != 0 {
			restoreExistingWindow(hwnd)
		}
		fmt.Println("Another instance is already running, exiting...")
		return
	}
	defer closeMutex(mutexHandle)

	service.InitDb()

	// 注册图片代理处理函数
	http.HandleFunc("/image-proxy", imageProxyHandler)
	// 启动 HTTP 服务器
	go func() {
		proxyListenTryNum := 0
		for proxyListenTryNum < 10 {
			thePort := strconv.Itoa(service.IMAGE_PROXY_PROT)
			//检查端口是否使用
			_, err := net.Dial("tcp", "localhost:"+thePort)
			if err == nil {
				service.IMAGE_PROXY_PROT++
				proxyListenTryNum++
				continue
			}
			// 用 Listen + Serve 替代 ListenAndServe，以便在端口绑定后立即发信号
			ln, err := net.Listen("tcp", ":"+thePort)
			if err != nil {
				println("Error:", err.Error())
				service.IMAGE_PROXY_PROT++
				proxyListenTryNum++
				continue
			}
			println("Image proxy server started on port " + thePort)
			close(proxyReady) // 端口已绑定，通知主线程
			if err := http.Serve(ln, nil); err != nil {
				println("Image proxy serve error:", err.Error())
			}
			return
		}
		if proxyListenTryNum >= 10 {
			println("Image proxy: failed to start after 10 attempts")
			close(proxyReady)
		}
	}()

	// 等待代理服务器启动完成（最多 5 秒）
	select {
	case <-proxyReady:
	case <-time.After(5 * time.Second):
		println("Image proxy: startup timeout, continuing anyway")
	}

	// 预热 B站 CDN DNS，避免首次图片请求慢
	prewarmDNS()

	// 发送统计信息到
	go service.SendAppStats()

	// Create an instance of the app structure
	app := NewApp()
	appMenu := NewMenu()
	bl := service.NewBL()

	isMacOS := runtime.GOOS == "darwin"
	// isMacOS = false
	AppMenu := menu.NewMenu()
	if isMacOS {
		aboutMenu := AppMenu.AddSubmenu("设置")
		aboutMenu.AddText("关于应用", nil, func(_ *menu.CallbackData) {
			appMenu.ShowAbout()
		})
		aboutMenu.AddText("快捷键", nil, func(_ *menu.CallbackData) {
			appMenu.ShowKeyboardShortcuts()
		})
		aboutMenu.AddText("检查更新", nil, func(_ *menu.CallbackData) {
			appMenu.CheckForUpdates(true, "")
		})
		aboutMenu.AddText("退出应用", nil, func(_ *menu.CallbackData) {
			appMenu.CloseApp()
		})
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:  service.APP_NAME,
		Width:  800,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},

		BackgroundColour: options.NewRGBA(0, 0, 0, 0),
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title: fmt.Sprintf("%s %s", service.APP_NAME, service.APP_VERSION),
			},
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Appearance:           mac.NSAppearanceNameAqua,
			DisableZoom:          true,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableFramelessWindowDecorations: false,
			IsZoomControlEnabled:              false,
			ZoomFactor:                        1.0,
		},
		Linux: &linux.Options{
			ProgramName:         service.APP_NAME,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
			WindowIsTranslucent: true,
		},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			appMenu.SetAppContext(ctx)
			// 启动时检查更新
			appMenu.CheckForUpdates(false, "")

			// Windows: 初始化系统托盘
			if runtime.GOOS == "windows" {
				initTrayWindows(func() {
					// 显示窗口 - 使用 Wails runtime
					runtime.Show(ctx)
				}, func() {
					// 退出应用
					removeTrayWindows()
					runtime.Quit(ctx)
				})
			}
		},
		OnBeforeClose: func(ctx context.Context) bool {
			// Windows 下关闭窗口时隐藏到托盘，不真正退出
			if runtime.GOOS == "windows" {
				runtime.Hide(ctx)
				return true // 阻止默认的关闭行为
			}
			return false // macOS/Linux 允许关闭
		},
		OnShutdown: func(ctx context.Context) {
			// 应用退出时清理托盘
			if runtime.GOOS == "windows" {
				removeTrayWindows()
			}
		},
		Bind: []interface{}{
			app,
			bl,
			appMenu,
		},
		DisableResize: true,
		Fullscreen:    false,
		Menu:          AppMenu,
		Frameless:     !isMacOS,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "bili-fm",
		},
		CSSDragProperty:   "widows",
		CSSDragValue:      "1",
		HideWindowOnClose: false, // 由 OnBeforeClose 控制
		// Debug: options.Debug{
		// 	OpenInspectorOnStartup: true,
		// },
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
