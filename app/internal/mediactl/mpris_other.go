//go:build !linux

package mediactl

// New 在非 Linux 平台暂时是 no-op：mygo 还没有 macOS Now Playing /
// Windows SMTC 的绑定。接口保持与 Linux 一致，接上层代码不用改。
func New(cb Callbacks) Controller { return noop{} }
