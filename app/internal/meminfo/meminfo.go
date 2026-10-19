// Package meminfo 报告进程内存构成，供「-memstats」诊断与内存问题的实机
// 定位使用。
//
// 为什么需要它：用户报「播放视频 200MB+、切几个视频 + 翻抽屉后 315MB」，
// 而只看任务管理器的一个数字分不清钱花在哪。这里把两侧拆开：
//
//   - Go 侧：runtime.MemStats（活堆 / 已保留 / 栈 / 总计），这是**应用代码**
//     真正持有的部分，也是我们唯一能主动压的部分；
//   - 进程侧：RSS / 私有提交字节，包含 Go 堆之外的一切（图形驱动、系统 DLL、
//     字体引擎、GPU 纹理的 CPU 侧映射等）。
//
// 两者之差就是「不由应用代码决定」的那部分。实测（Linux，无头 800×600、
// 300 张卡片 + 封面）：Go 活堆 ~16MB，进程 RSS ~163MB —— 差出来的 140MB 里
// 有 GTK/GL 驱动 ~95MB、Go 运行时保留页 ~20MB、图形栈的纹理/帧缓冲。
package meminfo

import (
	"fmt"
	"runtime"
)

// Snapshot 是一次内存快照。字节单位。
type Snapshot struct {
	// HeapAlloc 是 Go 活堆（本轮 GC 后仍可达的对象），应用代码真正的占用。
	HeapAlloc uint64
	// HeapSys 是 Go 向系统要来的堆地址空间（含空闲 span，未还给 OS 的部分）。
	HeapSys uint64
	// StackSys 是 goroutine 栈占的地址空间。
	StackSys uint64
	// Sys 是 Go 运行时从系统拿到的总量（堆 + 栈 + 运行时结构 + GC 元数据）。
	Sys uint64
	// RSS 是进程常驻物理内存（含与其它进程共享的页），拿不到时为 0。
	RSS uint64
	// Private 是进程私有提交字节（Windows 任务管理器「内存」列的口径），
	// 拿不到时为 0。
	Private uint64
}

// Now 读一次快照。会先跑一次 GC 吗？不会 —— 它必须便宜到能在帧循环旁边调；
// 需要「回收后的真实占用」时调用方自己先 runtime.GC()。
func Now() Snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	s := Snapshot{
		HeapAlloc: m.HeapAlloc,
		HeapSys:   m.HeapSys,
		StackSys:  m.StackSys,
		Sys:       m.Sys,
	}
	s.RSS, s.Private = footprint()
	return s
}

// MB 把字节换算成便于阅读的 MB。
func MB(n uint64) string { return fmt.Sprintf("%.1fMB", float64(n)/(1<<20)) }

// String 给出一行可读的构成描述（诊断日志用）。
func (s Snapshot) String() string {
	out := fmt.Sprintf("Go 堆 %s/%s(保留) 栈 %s 运行时 %s",
		MB(s.HeapAlloc), MB(s.HeapSys), MB(s.StackSys), MB(s.Sys))
	if s.RSS != 0 {
		out += " | 进程 RSS " + MB(s.RSS)
	}
	if s.Private != 0 {
		out += " | 私有提交 " + MB(s.Private)
	}
	return out
}

// NonGo 返回「不由 Go 堆决定」的那部分：进程 RSS 减 Go 已保留的地址空间。
// 拿不到 RSS 时返回 0。
func (s Snapshot) NonGo() uint64 {
	if s.RSS <= s.Sys {
		return 0
	}
	return s.RSS - s.Sys
}
