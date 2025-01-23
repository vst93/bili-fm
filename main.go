package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS
var APP_DIR = ""
var APP_VERSION = "1.1.1"
var APP_VERSION_NO = 4
var APP_NAME = "bili-FM"
var IMAGE_PROXY_PROT = 4654

type GithubRelease struct {
	TagName string `json:"tag_name"`
	HtmlUrl string `json:"html_url"`
	Assets  []struct {
		BrowserDownloadUrl string `json:"browser_download_url"`
	} `json:"assets"`
}

func checkForUpdates(ctx context.Context, isManualCheck bool) {
	resp, err := http.Get("https://api.github.com/repos/vst93/bili-fm/releases/latest")
	if err != nil {
		if isManualCheck {
			runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
				Title:   "检查更新失败",
				Message: "网络连接失败，请稍后重试",
				Type:    runtime.ErrorDialog,
				Buttons: []string{"确定"},
			})
		}
		return
	}
	defer resp.Body.Close()

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		if isManualCheck {
			runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
				Title:   "检查更新失败",
				Message: "解析版本信息失败，请稍后重试",
				Type:    runtime.ErrorDialog,
				Buttons: []string{"确定"},
			})
		}
		return
	}

	// 移除版本号前的 'v' 如果存在
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(APP_VERSION, "v")

	if latestVersion > currentVersion {
		choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Title:         "发现新版本",
			Message:       "发现新版本 " + latestVersion + "\n是否前往下载？",
			Type:          runtime.QuestionDialog,
			Buttons:       []string{"是", "否"},
			DefaultButton: "是",
			CancelButton:  "否",
		})

		if err == nil && choice == "是" {
			runtime.BrowserOpenURL(ctx, release.HtmlUrl)
		}
	} else if isManualCheck {
		runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Title:   "检查更新",
			Message: "当前已是最新版本",
			Type:    runtime.InfoDialog,
			Buttons: []string{"确定"},
		})
	}
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

	AppMenu := menu.NewMenu()
	AppMenu.AddSubmenu(APP_NAME)
	aboutMenu := AppMenu.AddSubmenu("设置")
	aboutMenu.AddText("关于", nil, func(_ *menu.CallbackData) {
		runtime.MessageDialog(app.ctx, runtime.MessageDialogOptions{
			Title:   "关于",
			Message: "通过音频来听B站节目，你可以把它作为一个音乐播放器，也可以用来作为知识学习的工具。\n\n项目开源地址：https://github.com/vst93/bili-fm",
			Type:    "info",
			Buttons: []string{"好的"},
		})
	})
	aboutMenu.AddText("版本", nil, func(_ *menu.CallbackData) {
		runtime.MessageDialog(app.ctx, runtime.MessageDialogOptions{
			Title:   "版本",
			Message: APP_VERSION,
			Type:    "info",
			Buttons: []string{"好的"},
		})
	})
	aboutMenu.AddText("检查更新", nil, func(_ *menu.CallbackData) {
		checkForUpdates(app.ctx, true)
	})

	// Create application with options
	err := wails.Run(&options.App{
		Title: APP_NAME,
		Width: 800,
		// Height: 580,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: options.NewRGBA(255, 255, 255, 0),
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title: fmt.Sprintf("%s %s", APP_NAME, APP_VERSION),
			},
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Appearance:           mac.NSAppearanceNameAqua,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableFramelessWindowDecorations: false,
		},
		Linux: &linux.Options{
			ProgramName:         APP_NAME,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
			WindowIsTranslucent: true,
		},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			// 启动时检查更新
			checkForUpdates(ctx, false)
		},
		Bind: []interface{}{
			app,
			bl,
		},
		DisableResize: true,
		Fullscreen:    false,
		Menu:          AppMenu,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "bili-fm",
		},
		// Frameless:       true,
		// CSSDragProperty: "widows",
		// CSSDragValue:    "1",
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
