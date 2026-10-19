package main

import (
	"os"
	"strings"
	"testing"
)

// TestSeriesSkipMessage 钉住「合集播放全部」里单条拉详情失败的 warning 文案。
//
// 原版 main:src/pages/index.tsx:1733：
//
//	toast({ type: "warning", content: `视频 ${video.title} 加载失败，已跳过` })
//
// 此前原生版这条是**静默 return**（`if len(vi.Pages) == 0 { return }`）：用户点
// 「播放全部」后只会觉得合集里少了几集，没有任何线索。
func TestSeriesSkipMessage(t *testing.T) {
	got := seriesSkipMessage("第一集", "BV1xx411c7mD")
	want := "视频 第一集 加载失败，已跳过"
	if got != want {
		t.Fatalf("文案与原版不一致：got %q, want %q", got, want)
	}
	// 标题缺失时退 bvid，而不是渲染出「视频  加载失败」（原版会渲染 undefined）。
	if got := seriesSkipMessage("", "BV1xx411c7mD"); got != "视频 BV1xx411c7mD 加载失败，已跳过" {
		t.Fatalf("标题缺失的兜底不对：%q", got)
	}
	// 反证：文案里不许出现原版没有的英文/标点差异（这条断言在有人把「，」改成
	// 半角「,」或把「已跳过」写成「skipped」时会红）。
	if strings.Contains(got, ",") || strings.Contains(got, "skipped") {
		t.Fatalf("文案被改动过：%q", got)
	}
}

// TestSeriesPlayAllToastWording 逐条对照原版 index.tsx 的三条固定文案。
//
// 这些串是**源码级**契约（没有可注入的 stub BL，起播路径要真网络），所以直接
// 扫 playlist.go，确保文案与本轮对照结果一致；反证见下一条子测试。
func TestSeriesPlayAllToastWording(t *testing.T) {
	src, err := os.ReadFile("playlist.go")
	if err != nil {
		t.Fatalf("读 playlist.go 失败: %v", err)
	}
	code := string(src)
	for _, want := range []string{
		"请先点击UP主头像或昵称，选择一个合集", // index.tsx:2383
		"正在加载合集...",             // index.tsx:1661（三个 ASCII 点）
		"合集中的视频暂时都无法播放",        // index.tsx:1745
		"已加载 %d 集到播放列表",         // index.tsx:1757
	} {
		if !strings.Contains(code, want) {
			t.Errorf("playlist.go 里缺少原版文案 %q", want)
		}
	}
	// 反证：上一版自己编的三种写法必须已经消失。
	for _, bad := range []string{
		"请先到 UP 空间选择一个合集",
		"正在加载合集…",
		"合集里的视频暂时都无法播放",
	} {
		if strings.Contains(code, bad) {
			t.Errorf("旧文案 %q 还在（应换成原版写法）", bad)
		}
	}
}

// TestSeriesPlayAllBrowsesFirstItem 钉住「播放全部」后「浏览中」跟着换到第一集。
//
// 原版 index.tsx:1750 走 handlePlaylistVideoSelect(0, items, "series")，而它
// 在 preserveUi=false（手动选、非自动切歌）时会 setVideoInfo(第一集)。
// 原生版 playPlaylist 的 keepDrawer 同时控制「抽屉是否留开」与「是否写
// BrowseInfo」，而播放全部传的是 false —— 所以这一笔必须由 seriesPlayAll 自己补，
// 否则弹幕/评论与选集抽屉会继续用上一次浏览的那条视频。
func TestSeriesPlayAllBrowsesFirstItem(t *testing.T) {
	src, err := os.ReadFile("playlist.go")
	if err != nil {
		t.Fatalf("读 playlist.go 失败: %v", err)
	}
	code := string(src)
	if !strings.Contains(code, "a.BrowseInfo = browseInfoFromPlayItem(out[0])") {
		t.Fatal("seriesPlayAll 没有把「浏览中」换到第一集（BrowseInfo 会保持陈旧）")
	}
	// 反证：自动切歌那条路（playPlaylist 的 keepDrawer=false）不许写 BrowseInfo。
	if strings.Contains(code, "BrowseInfo = browseInfoFromPlayItem(it)") &&
		!strings.Contains(code, "if keepDrawer {\n\t\tit := items[index]") {
		t.Fatal("playPlaylist 的 BrowseInfo 写入丢了 keepDrawer 守卫")
	}
}
