package view

import (
	"fmt"
	"testing"
)

// browseVideo 造一条可辨认的视频详情：标题 / UP 主 / 分集标题都带 tag 前缀，
// 这样无头渲染里能一眼看出主区与选集抽屉各自读的是哪一条。
func browseVideo(tag string, parts int) *Info {
	ps := make([]Part, parts)
	for i := range ps {
		ps[i] = Part{
			Cid:  int64(1000 + i),
			Page: i + 1,
			Part: fmt.Sprintf("%s选集%d", tag, i),
		}
	}
	return &Info{
		Bvid:      "BV-" + tag,
		Aid:       1,
		Cid:       1000,
		Title:     tag + "视频标题",
		Pic:       tag + "/cover.png",
		OwnerName: tag + "UP",
		Parts:     ps,
	}
}

// TestBrowseFallsBackToInfo 钉住 Browse() 的回退：没有「浏览中」的视频时
// 退回正在播放的那条（「打开当前视频的选集」本就属于浏览的一种，此时两者
// 同一条），所以选集抽屉永远有内容。
func TestBrowseFallsBackToInfo(t *testing.T) {
	a := NewApp(func() {})
	if a.Browse() != nil {
		t.Fatal("还没有任何详情时 Browse() 应为 nil")
	}
	a.Info = browseVideo("播放", 2)
	if a.Browse() != a.Info {
		t.Fatal("没有浏览信息时应退回正在播放的那条（a.Info）")
	}
	browsed := browseVideo("浏览", 3)
	a.BrowseInfo = browsed
	if a.Browse() != browsed {
		t.Fatal("有浏览信息时应优先用它")
	}
}

// TestBrowseToKeepsDisplayInfo 钉住 BrowseTo 只写「浏览中」那一条：
// 原版 handleUrlJump 只 setVideoInfo + setShowPageList，主区显示跟的是
// playingInfo（displayVideoInfo），所以浏览另一条视频不能改 a.Info。
func TestBrowseToKeepsDisplayInfo(t *testing.T) {
	a := NewApp(func() {})
	playing := browseVideo("播放", 2)
	browsed := browseVideo("浏览", 5)
	a.Info = playing
	a.Track = &Track{Bvid: playing.Bvid, Cid: 1000, Title: playing.Title, Part: "P1", Up: "播放UP"}

	a.BrowseTo(browsed)

	if a.Info != playing {
		t.Error("BrowseTo 不该改主区显示用的 Info")
	}
	if a.BrowseInfo != browsed {
		t.Error("BrowseTo 应把详情记进 BrowseInfo")
	}
	if a.Drawer != DrawerParts {
		t.Errorf("BrowseTo 应打开选集抽屉，得到 %q", a.Drawer)
	}
	// 主区文案仍来自正在播放的那条。
	if got := a.infoTitle(); got != playing.Title {
		t.Errorf("主区标题 = %q, want %q（正在播放）", got, playing.Title)
	}
	if got := a.ownerName(); got != playing.OwnerName {
		t.Errorf("主区 UP 主 = %q, want %q（正在播放）", got, playing.OwnerName)
	}
}

// TestPartsDrawerReadsBrowseInfo 无头渲染选集抽屉：列表与表头计数都跟
// 「浏览中」的那条，而不是正在播放的那条。
func TestPartsDrawerReadsBrowseInfo(t *testing.T) {
	a := NewApp(func() {})
	a.Info = browseVideo("播放", 2)
	a.BrowseInfo = browseVideo("浏览", 5)
	a.Drawer = DrawerParts

	tt := shellTesterSize(t, a, 980, 720)
	if !tt.HasText("浏览选集0") {
		t.Fatal("选集抽屉应展示浏览中的那条的分集")
	}
	if tt.HasText("播放选集0") {
		t.Fatal("选集抽屉不该展示正在播放的那条的分集")
	}
	if !tt.HasText("选集(5)") {
		t.Error("表头计数应数浏览中的那条的分集")
	}
	if tt.HasText("选集(2)") {
		t.Error("表头计数不该数正在播放的那条")
	}
}

// TestMainStageKeepsPlayingInfoWhileBrowsing 无头渲染主区：浏览另一条视频
// （BrowseInfo 已写）时，主区标题仍是正在播放的那条 —— 这是原版
// displayVideoInfo 只跟 playingInfo 的用户可见差异。
func TestMainStageKeepsPlayingInfoWhileBrowsing(t *testing.T) {
	a := NewApp(func() {})
	playing := browseVideo("播放", 2)
	a.Info = playing
	a.Track = &Track{Bvid: playing.Bvid, Cid: 1000, Title: playing.Title, Part: "P1", Up: "播放UP"}
	a.BrowseInfo = browseVideo("浏览", 5)

	tt := shellTesterSize(t, a, 1200, 800)
	if !tt.HasText("播放视频标题") {
		t.Fatal("主区应显示正在播放的视频标题")
	}
	if tt.HasText("浏览视频标题") {
		t.Fatal("主区不该被浏览中的视频标题覆盖")
	}
}

// TestPartsLocateUsesBrowseInfo 钉住选集搜索也只认「浏览中」的那条：
// 关键词只出现在浏览那条的分集标题里时应命中，只出现在正在播放那条时不该命中。
func TestPartsLocateUsesBrowseInfo(t *testing.T) {
	a := NewApp(func() {})
	a.Info = browseVideo("播放", 3)
	a.BrowseInfo = browseVideo("浏览", 3)

	a.locatePart("浏览选集2")
	if !a.locateNow || a.locatePartIndex != 2 {
		t.Fatalf("应定位到浏览那条的第 3 集：locateNow=%v idx=%d", a.locateNow, a.locatePartIndex)
	}

	a.locateNow, a.locatePartIndex = false, -1
	a.locatePart("播放选集1")
	if a.locateNow || a.locatePartIndex != -1 {
		t.Fatalf("正在播放那条的分集不该被搜索命中：locateNow=%v idx=%d", a.locateNow, a.locatePartIndex)
	}
}
