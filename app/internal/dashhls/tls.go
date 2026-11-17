package dashhls

import "crypto/tls"

// insecureTLS 放宽证书校验。
//
// 与 internal/proxy 的同名函数同样的理由：B 站部分 CDN / 图床在有些网络
// 环境下证书链不全，严格校验会让整批分片拉取失败。只作用于本包出站请求。
func insecureTLS() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true} //nolint:gosec // 与 proxy / bilibili 既有行为一致
}
