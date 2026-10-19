package meminfo

import (
	"syscall"
	"unsafe"
)

// processMemoryCounters 对应 PROCESS_MEMORY_COUNTERS（psapi.h）。
// 字段全用 uintptr，32/64 位下布局都与头文件一致。
type processMemoryCounters struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

var (
	psapi                    = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
)

// footprint 取当前进程的 WorkingSetSize（任务管理器「内存」附近的口径）与
// PagefileUsage（私有提交字节，Windows 上更接近「这块内存真的归我」）。
// 取不到时返回 0，调用方会跳过这两项。
func footprint() (rss, private uint64) {
	h, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, 0
	}
	var pmc processMemoryCounters
	pmc.Cb = uint32(unsafe.Sizeof(pmc))
	ok, _, _ := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.Cb))
	if ok == 0 {
		return 0, 0
	}
	return uint64(pmc.WorkingSetSize), uint64(pmc.PagefileUsage)
}
