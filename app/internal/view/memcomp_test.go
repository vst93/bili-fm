//go:build perfmeasure

// 内存构成的**无头**量法（受控场景，不含 GPU / WebView2）：
//
//	go test -tags perfmeasure -count=1 -run TestMemComposition -v ./internal/view/
//
// 它量的是「应用代码自己持有多少」：Go 活堆（runtime.MemStats）与位图缓存
// （imagecache.Bytes）。实机日志（-memstats）里另外两项 —— 进程 RSS / 私有提交
// —— 由 internal/meminfo 读，无头环境拿不到真实图形栈，所以这里不量。
package view

import (
	"fmt"
	"runtime"
	"testing"
	"time"

)

func heapNow() (alloc, sys uint64) {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc, m.Sys
}

func TestMemComposition(t *testing.T) {
	a, tst, _ := perfApp(t, 300, 1, true)
	log := func(stage string) {
		alloc, sys := heapNow()
		ic, _ := a.Images.(interface{ Bytes() int64 })
		var icb int64
		if ic != nil {
			icb = ic.Bytes()
		}
		t.Logf("%-22s heapAlloc=%6.1fMB sys=%6.1fMB cacheBytes=%5.1fMB cacheLen=%d pending=%d",
			stage, float64(alloc)/(1<<20), float64(sys)/(1<<20), float64(icb)/(1<<20), a.Images.Pending(), a.Images.Pending())
	}
	log("start")
	waitCovers(t, a, tst)
	log("covers settled")
	for i := 0; i < 6; i++ {
		for j := 0; j < 10; j++ {
			tst.Scroll(400, 300, 0, 120)
		}
		waitCovers(t, a, tst)
	}
	log("scrolled 6 pages")
	// 模拟切视频：换 Track 封面（480w）+ 切 5 条
	for i := 0; i < 5; i++ {
		a.Track = &Track{Bvid: fmt.Sprintf("BVX%d", i), Cover: a.ListFor("feed").Cards[i].Cover, FirstFrame: a.ListFor("feed").Cards[i].Cover}
		tst.Frame()
	}
	log("switched 5 tracks")
	a.Drawer = ""
	tst.Frame()
	log("closed drawer")
	time.Sleep(10 * time.Millisecond)
	log("after sleep")
}
