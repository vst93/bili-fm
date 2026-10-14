package main

import "testing"

// TestDanmakuLoadBlocked 钉住弹幕拉取的连点闸门：已经有一次在途时忽略新的
// 拉取请求。对应原版 isLoadingDanmaku 期间弹幕抽屉刷新键被禁用（点不动），
// 以及 loadDanmakuList 对同一 cid 的重复拉取跳过。
func TestDanmakuLoadBlocked(t *testing.T) {
	if danmakuLoadBlocked(false) {
		t.Error("没有在途拉取时不该挡")
	}
	if !danmakuLoadBlocked(true) {
		t.Error("已有一次在途时应挡，否则连点会各起一整套弹幕请求")
	}
}
