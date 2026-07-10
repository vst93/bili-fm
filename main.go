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
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   30,
	MaxConnsPerHost:       50,
	IdleConnTimeout:       120 * time.Second,
	TLSHandshakeTimeout:  15 * time.Second,
	ExpectContinueTimeout: 2 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
	DisableKeepAlives:     false,
	ForceAttemptHTTP2:     false, // 禁用 HTTP/2，避免某些 CDN 问题
	DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, addr)
		}
		// 查缓存（读锁）
		dnsCacheMu.RLock()
		if cached, ok := dnsCache[host]; ok {
			dnsCacheMu.RUnlock()
			return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp4", net.JoinHostPort(cached, port))
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
			return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp4", net.JoinHostPort(ip, port))
		}
		return nil, fmt.Errorf("no addresses for %s", host)
	},
}

var proxyClient = &http.Client{
	Transport: proxyTransport,
	Timeout:   30 * time.Second,
}

// prewarmDNS 预解析 B站 CDN 域名，避免首次图片请求卡在 DNS
func prewarmDNS() {
	hosts := []string{
		"i0.hdslb.com",
		"i1.hdslb.com",
		"i2.hdslb.com",
		"i3.hdslb.com",
		"static.hdslb.com",
		"api.bilibili.com",
	}
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
		println("image-proxy: invalid URL:", imageURL, "error:", err.Error())
		http.Error(w, "Invalid image URL", http.StatusBadRequest)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	// 带重试的上游请求（最多 3 次），每次重试创建新 request 避免连接状态残留
	var resp *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			req, _ = http.NewRequest("GET", imageURL, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
			req.Header.Set("Referer", "https://www.bilibili.com/")
			req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
			req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
			time.Sleep(time.Duration(200*(attempt)) * time.Millisecond)
		}
		resp, err = proxyClient.Do(req)
		if err == nil {
			break
		}
		println("image-proxy: attempt", attempt+1, "failed for", imageURL, ":", err.Error())
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
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// 将图片数据写入响应
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		// 客户端断开连接等，只打印日志，不回写 error（headers 已发送）
		println("image-proxy: write error after", n, "bytes for", imageURL, ":", err.Error())
	}
}

func main() {
	// Windows: 允许 WebView2 加载 HTTP 混合内容（图片代理）
	if runtime.GOOS == "windows" {
		os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
			"--allow-running-insecure-content --disable-features=MixedContentAutoupgrade")
	}

	service.InitDb()

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
		aboutMenu.AddText("退出应用", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
			appMenu.CloseApp()
		})
	}

	// macOS: 点击叉按钮仅隐藏窗口（HideWindowOnClose=true）；CMD+Q、菜单退出、Alt+F4 真退出
	// Windows/Linux: 点击叉按钮直接退出

	// Windows: 读取辅助功能文本缩放比例，反向补偿 ZoomFactor，避免控件溢出窗口
	textScale := getTextScaleFactor()
	zoomFactor := 1.0
	if textScale > 100 {
		zoomFactor = 100.0 / float64(textScale)
		println(fmt.Sprintf("Windows text scale: %d%%, adjusted zoom factor: %.3f", textScale, zoomFactor))
	}

	err := wails.Run(&options.App{
		Title:  service.APP_NAME,
		Width:  800,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/image-proxy" {
						imageProxyHandler(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
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
			ZoomFactor:                        zoomFactor,
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

			// Windows: 初始化系统托盘 + 存储 context
			if runtime.GOOS == "windows" {
				setWailsContext(ctx)
				initTrayWindows(func() {
					// 显示窗口 - Wails runtime + Win32 双保险
					// wailsruntime.Show 在某些时序下可能不生效（窗口句柄未就绪），
					// 同时用 Win32 FindWindow + ShowWindow 直接操作窗口
					wailsruntime.Show(ctx)
					bringWindowToFront()
				}, func() {
					// 退出应用 - 强制退出进程
					removeTrayWindows()
					go func() {
						time.Sleep(100 * time.Millisecond)
						os.Exit(0)
					}()
				})
			}
		},
		OnBeforeClose: func(ctx context.Context) bool {
			// 正在退出中（菜单/托盘触发的 os.Exit），允许关闭
			if IsExiting() {
				return false
			}
			// macOS: HideWindowOnClose=true，点击叉按钮仅隐藏窗口，不会触发此回调
			// 只有 CMD+Q 这样的系统级退出才会触发此回调，直接退出
			// Windows/Linux: HideWindowOnClose=false，点击叉按钮或 Alt+F4 触发此回调，直接退出
			return false
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
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				// 第二实例启动时，显示已运行实例的主窗口
				showExistingWindow()
			},
		},
		CSSDragProperty:   "widows",
		CSSDragValue:      "1",
		HideWindowOnClose: isMacOS, // macOS: 叉按钮隐藏窗口；Windows/Linux: 叉按钮关闭窗口
		// Debug: options.Debug{
		// 	OpenInspectorOnStartup: true,
		// },
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
