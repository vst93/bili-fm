package meminfo

import (
	"os"
	"strconv"
	"strings"
)

// footprint 读 /proc/self/status 的 VmRSS（常驻）与 VmHWM 无关，Linux 上
// 「私有提交」没有与 Windows 完全等价的口径，这里只给 RSS。
func footprint() (rss, private uint64) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			return 0, 0
		}
		kb, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			return 0, 0
		}
		return kb << 10, 0
	}
	return 0, 0
}
