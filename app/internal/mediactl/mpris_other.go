//go:build !linux && !darwin && !windows

package mediactl

// New 在没有实现系统媒体中心的平台是 no-op。
// 接口保持与 Linux 一致，接上层代码不用改。
func New(cb Callbacks) Controller { return noop{} }
