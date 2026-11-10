package main

import (
	"strings"
	"testing"
)

// 生产磁盘缓存必须只在装配层显式开启：view.NewApp 保持纯 RAM（测试 / 无头
// 不碰用户缓存目录），main 在原生窗口创建之前换成 NewPersistent。源码级
// 契约，防止以后只改了构造函数却忘了接线（或反过来把磁盘带进测试默认值）。
func TestProductionImageCacheUsesPersistentDisk(t *testing.T) {
	mainSrc := flattenCode(t, "main.go")
	for _, want := range []string{
		"diskImages:=imagecache.NewPersistent(repaint)",
		"app.Images=diskImages",
		"diskImages.DiskInfo()",
	} {
		if !strings.Contains(mainSrc, want) {
			t.Errorf("main.go 缺少生产磁盘缓存接线：%s", want)
		}
	}

	viewSrc := flattenCode(t, "internal/view/state.go")
	if !strings.Contains(viewSrc, "Images:imagecache.New(repaint)") {
		t.Error("view.NewApp 应保持纯 RAM 的 imagecache.New")
	}
	if strings.Contains(viewSrc, "imagecache.NewPersistent") {
		t.Error("view.NewApp 不应隐式开启磁盘缓存（测试 / 无头会写用户目录）")
	}
}
