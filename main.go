package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS
var APP_DIR = ""
var APP_VERSION = "1.1.2"
var APP_VERSION_NO = 5
var APP_NAME = "bili-FM"
var IMAGE_PROXY_PROT = 4654

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

	// 发送 HTTP 请求获取图片
	resp, err := http.Get(imageURL)
	if err != nil {
		http.Error(w, "Failed to fetch image", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Failed to fetch image", http.StatusInternalServerError)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 将图片数据写入响应
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "Failed to write image data", http.StatusInternalServerError)
		return
	}
}

func main() {

	InitDb()

	// 注册图片代理处理函数
	http.HandleFunc("/image-proxy", imageProxyHandler)
	// 启动 HTTP 服务器
	go func() {
		proxyListenTryNum := 0
		for proxyListenTryNum < 10 {
			thePort := strconv.Itoa(IMAGE_PROXY_PROT)
			//检查端口是否使用
			_, err := net.Dial("tcp", "localhost:"+thePort)
			if err == nil {
				IMAGE_PROXY_PROT++
				proxyListenTryNum++
				continue
			}
			if err := http.ListenAndServe(":"+thePort, nil); err != nil {
				println("Error:", err.Error())
				IMAGE_PROXY_PROT++
				proxyListenTryNum++
				continue
			} else {
				println("Image proxy server started on port " + thePort)
				break
			}
		}
	}()

	// Create an instance of the app structure
	app := NewApp()
	bl := NewBL()
	appMenu := NewMenu()

	isMacOS := runtime.GOOS == "darwin"
	AppMenu := menu.NewMenu()
	if isMacOS {
		aboutMenu := AppMenu.AddSubmenu("设置")
		aboutMenu.AddText("关于", nil, func(_ *menu.CallbackData) {
			appMenu.ShowAbout()
		})
		aboutMenu.AddText("版本", nil, func(_ *menu.CallbackData) {
			appMenu.ShowVersion()
		})
		aboutMenu.AddText("检查更新", nil, func(_ *menu.CallbackData) {
			appMenu.CheckForUpdates(true)
		})
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:  APP_NAME,
		Width:  800,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},

		BackgroundColour: options.NewRGBA(0, 0, 0, 0),
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title: fmt.Sprintf("%s %s", APP_NAME, APP_VERSION),
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
		},
		Linux: &linux.Options{
			ProgramName:         APP_NAME,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
			WindowIsTranslucent: true,
		},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			appMenu.SetAppContext(ctx)
			// 启动时检查更新
			appMenu.CheckForUpdates(false)
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
		CSSDragProperty: "widows",
		CSSDragValue:    "1",
		// Debug: options.Debug{
		// 	OpenInspectorOnStartup: true,
		// },
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
