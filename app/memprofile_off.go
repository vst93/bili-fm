//go:build !linux && !darwin

package main

// startHeapDump 在其它平台是空操作（没有 SIGUSR2）。
func startHeapDump() {}
