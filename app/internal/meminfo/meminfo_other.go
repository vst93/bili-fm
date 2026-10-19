//go:build !linux && !windows

package meminfo

import "syscall"

// footprint 在 macOS / 其它 Unix 上用 getrusage 的最大常驻集（Maxrss）。
// 注意 macOS 的 Maxrss 单位是字节，Linux 是 KB —— 这个文件不覆盖 Linux。
func footprint() (rss, private uint64) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, 0
	}
	return uint64(ru.Maxrss), 0
}
