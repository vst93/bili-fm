//go:build linux || darwin

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
)

// startHeapDump 让 SIGUSR2（`kill -USR2 <pid>`）写一份堆快照到
// /tmp/bili-fm-heap-<pid>.pb.gz，用于定位「活堆花在哪」：
//
//	go tool pprof -inuse_space <binary> /tmp/bili-fm-heap-<pid>.pb.gz
//
// 只在 linux/darwin 有信号；与 -memstats 的定时摘要互补（那个看趋势，这个看构成）。
func startHeapDump() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR2)
	go func() {
		for range ch {
			path := fmt.Sprintf("/tmp/bili-fm-heap-%d.pb.gz", os.Getpid())
			f, err := os.Create(path)
			if err != nil {
				log.Printf("堆快照创建失败: %v", err)
				continue
			}
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Printf("堆快照写入失败: %v", err)
			}
			_ = f.Close()
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			log.Printf("堆快照 %s：HeapAlloc=%dMB HeapInuse=%dMB HeapObjects=%d",
				path, ms.HeapAlloc>>20, ms.HeapInuse>>20, ms.HeapObjects)
		}
	}()
}
