package bilibili

// 应用常量（对应旧版 service/config.go）。名字带 Version 前缀是为了避开
// api.go 里已有的 type AppVersion。
const (
	Version   = "3.0.0"
	VersionNo = 300
	AppName   = "bili-FM"
)

// ImageProxyPort 是内嵌图片/音频代理监听的本地端口。与旧版保持一致，
// 这样从 Wails / Tauri 版本升级上来的用户，页面缓存里的 URL 依然有效。
const ImageProxyPort = 4654
