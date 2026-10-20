//go:build darwin

package meminfo

import (
	"os"
	"unsafe"

	"github.com/ebitengine/purego"
)

// rusageInfoV4 是 sys/resource.h 的 rusage_info_v4，只声明到
// ri_phys_footprint 为止（后面的字段用不到，多留一倍空间也无妨）。
type rusageInfoV4 struct {
	UUID           [16]byte
	UserTime       uint64
	SystemTime     uint64
	PkgIdleWkups   uint64
	InterruptWkups uint64
	Pageins        uint64
	WiredSize      uint64
	ResidentSize   uint64
	PhysFootprint  uint64
	Rest           [512]byte
}

// RUSAGE_INFO_V4 是 proc_pid_rusage 的 flavor（sys/resource.h）。
const rusageInfoV4Flavor = 4

var procPidRusage func(pid int32, flavor int32, buffer unsafe.Pointer) int32

func init() {
	lib, err := purego.Dlopen("/usr/lib/libproc.dylib", purego.RTLD_LAZY)
	if err != nil {
		return
	}
	sym, err := purego.Dlsym(lib, "proc_pid_rusage")
	if err != nil {
		return
	}
	purego.RegisterFunc(&procPidRusage, sym)
}

// footprint 在 macOS 上读 proc_pid_rusage 的 phys_footprint —— 就是活动监视器
// 「内存」列的那个数，也是用户报「200MB」时看的数。
//
// 之前这里用 getrusage 的 Maxrss（峰值常驻）：它只涨不跌，空闲回收之后仍
// 显示历史峰值，和活动监视器对不上（本轮排查时就因此把「已经降下来」误判成
// 「没降」）。RSS 仍是 resident_size，private 在 macOS 上没有等价口径，返回 0。
func footprint() (rss, private uint64) {
	if procPidRusage == nil {
		return 0, 0
	}
	var ri rusageInfoV4
	if procPidRusage(int32(os.Getpid()), rusageInfoV4Flavor, unsafe.Pointer(&ri)) != 0 {
		return 0, 0
	}
	return ri.PhysFootprint, 0
}
