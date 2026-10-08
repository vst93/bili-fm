package proxy

import "crypto/tls"

// insecureTLS 放宽证书校验。
//
// 旧版（Wails 的 main.go 与 Tauri 的 proxy.rs）都是这么做的：B 站图床在
// 部分网络环境下证书链不全，严格校验会让整批封面加载失败。这里只作用于
// 代理内部的出站请求，不涉及应用自身的其它连接。
func insecureTLS() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true} //nolint:gosec // 与旧版行为一致
}
