package main

import (
	"embed"
	"fmt"

	"net/http"
	"net/http/httputil"
	"net/url"

	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS
var APP_DIR = ""
var APP_VERSION = "0.1.0"
var APP_VERSION_NO = 1

func main() {

	InitDb()

	// 设置代理处理函数
	http.HandleFunc("/proxy/", func(w http.ResponseWriter, r *http.Request) {

		// 解析图片URL
		path := r.URL.Path
		fmt.Println(path)
		path = strings.TrimPrefix(path, "/proxy/https:/")

		targetUrl, err := url.Parse("https://" + path)
		fmt.Println(targetUrl)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 创建一个反向代理
		proxy := httputil.NewSingleHostReverseProxy(targetUrl)

		// 修改请求头
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			// req.Header.Set("Origin", "https://www.bilibili.com")
			req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.110 Safari/537.36")
			// req.Header.Set("Refresh", "https://www.bilibili.com") // 5秒刷新一次
			// req.Header.Set("Host", "www.bilibili.com") // 防盗链
			req.Header.Set("Accept-Encoding", "gzip, deflate, br")
			req.Header.Set("Connection", "keep-alive")
			req.Header.Set("Accept", "*/*")
		}

		// 使用代理服务请求图片
		proxy.ServeHTTP(w, r)
	})
	go http.ListenAndServe(":4653", nil)

	// Create an instance of the app structure
	app := NewApp()
	bl := NewBL()

	AppMenu := menu.NewMenu()
	AppMenu.AddSubmenu("bili-FM")
	aboutMenu := AppMenu.AddSubmenu("Operation")
	aboutMenu.AddText("About", nil, func(_ *menu.CallbackData) {
		runtime.MessageDialog(app.ctx, runtime.MessageDialogOptions{
			Title:   "About",
			Message: "bili-FM is a simple and lightweight bilibili audio player built with Wails framework. https://github.com/vst93/bili-fm",
			Type:    "info",
			Buttons: []string{"OK"},
		})
	})
	// VersionMenu := AppMenu.AddSubmenu("Version")
	aboutMenu.AddText("Version", nil, func(_ *menu.CallbackData) {
		runtime.MessageDialog(app.ctx, runtime.MessageDialogOptions{
			Title:   "Version",
			Message: APP_VERSION,
			Type:    "info",
			Buttons: []string{"OK"},
		})
	})

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "bili-FM",
		Width:  800,
		Height: 580,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: options.NewRGB(235, 235, 235),
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
			bl,
		},
		DisableResize: true,
		Fullscreen:    false,
		Menu:          AppMenu,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
