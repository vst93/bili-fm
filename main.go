package main

import (
	"embed"
	"fmt"

	"net/http"
	"net/http/httputil"
	"net/url"

	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	// 设置代理处理函数
	// http.HandleFunc("/proxy/", func(w http.ResponseWriter, r *http.Request) {

	// 	// 解析图片URL
	// 	path := r.URL.Path
	// 	fmt.Println(path)
	// 	path = strings.TrimPrefix(path, "/proxy/http:/")

	// 	targetUrl, err := url.Parse("http://" + path)
	// 	fmt.Println(targetUrl)
	// 	if err != nil {
	// 		http.Error(w, err.Error(), http.StatusInternalServerError)
	// 		return
	// 	}

	// 	// 创建一个反向代理
	// 	proxy := httputil.NewSingleHostReverseProxy(targetUrl)

	// 	r.URL.Scheme = "http"
	// 	r.URL.Host = targetUrl.Host
	// 	r.URL.Path = targetUrl.Path
	// 	r.URL.RawQuery = targetUrl.RawQuery

	// 	// 使用代理服务请求图片
	// 	proxy.ServeHTTP(w, r)
	// })
	// go http.ListenAndServe(":4653", nil)
		
	
	// Create an instance of the app structure
	app := NewApp()
	bl := NewBL()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "bili-fm",
		Width:  800,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
			bl,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
