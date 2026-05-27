package main

import (
	"bilifm/service"
	"context"
	"embed"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"runtime"

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

// 图片代理处理函数
func imageProxyHandler(w http.ResponseWriter, r *http.Request) {
	// 从查询参数中获取图片 URL
	imageURL := r.URL.Query().Get("url")
	if imageURL == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch image: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
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
			if err := http.ListenAndServe(":"+thePort, nil); err != nil {
				println("Error:", err.Error())
				service.IMAGE_PROXY_PROT++
				proxyListenTryNum++
				continue
			} else {
				println("Image proxy server started on port " + thePort)
				close(proxyReady)
				break
			}
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
		HideWindowOnClose: true,
		// Debug: options.Debug{
		// 	OpenInspectorOnStartup: true,
		// },
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
