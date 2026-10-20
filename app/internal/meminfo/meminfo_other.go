//go:build !linux && !windows && !darwin

package meminfo

import "syscall"

// footprint 在其它 Unix（非 Linux / Windows / macOS）上用 getrusage 的
// 最大常驻集（Maxrss）。Linux 走 VmRSS，macOS 走 meminfo_darwin.go 的
// phys_footprint（活动监视器的口径）。
func footprint() (rss, private uint64) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, 0
	}
	return uint64(ru.Maxrss), 0
}
