package main

import "runtime/debug"

// debugGCPercent 读回当前 GOGC（debug.SetGCPercent 用 -1 探测后立刻恢复）。
func debugGCPercent() int {
	old := debug.SetGCPercent(-1)
	debug.SetGCPercent(old)
	return old
}
