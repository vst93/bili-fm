package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/vst93/bili-fm/app/internal/view"
)

// TestDanmakuLoadBlocked 钉住弹幕拉取的连点闸门：同一条视频已经有一次在途时
// 忽略新的拉取请求。对应原版 isLoadingDanmaku 期间弹幕抽屉刷新键被禁用（点不动），
// 以及 loadDanmakuList 对同一 cid 的重复拉取跳过。目标不同的在途请求**不**挡
// ——起播预取 A 时打开 B 的抽屉应当照发（由 startDanmakuFetch 的请求代号
// 保证只有最新一次的结果落地）。
func TestDanmakuLoadBlocked(t *testing.T) {
	if danmakuLoadBlocked(0, 42) {
		t.Error("没有在途拉取时不该挡")
	}
	if !danmakuLoadBlocked(42, 42) {
		t.Error("同一条 cid 已有一次在途时应挡，否则连点会各起一整套弹幕请求")
	}
	if danmakuLoadBlocked(42, 43) {
		t.Error("在途的是别的 cid 时不该挡（预取 A 不该挡住打开 B 的抽屉）")
	}
}

// TestDanmakuTargetFollowsBrowse 钉住弹幕 / 评论抽屉的数据来源是「浏览中」那条
// （原版 loadDanmakuList / loadReplyList 读 videoInfo），不是正在播放的曲目。
// 没有浏览信息时回退到正在播放那条（Browse 的既有语义）。
func TestDanmakuTargetFollowsBrowse(t *testing.T) {
	playing := &view.Info{Bvid: "BVplaying", Aid: 2, Cid: 20}
	browsed := &view.Info{Bvid: "BVbrowsed", Aid: 7, Cid: 70}
	if got := danmakuTarget(&view.App{Info: playing, BrowseInfo: browsed}); got != browsed {
		t.Errorf("有浏览信息时应取浏览那条，得到 %+v", got)
	}
	if got := danmakuTarget(&view.App{Info: playing}); got != playing {
		t.Errorf("没有浏览信息时应回退到正在播放那条，得到 %+v", got)
	}
	if got := danmakuTarget(&view.App{}); got != nil {
		t.Errorf("两条都没有时应是 nil，得到 %+v", got)
	}
}

// TestNeedLoadDanmakuOnTargetChange 钉住「换一条视频后弹幕要重拉」。改前只判断
// len(a.Danmaku)==0，播放 A（已预取弹幕）后浏览 B，打开抽屉会继续显示 A 的弹幕；
// 原版 loadDanmakuList 比的是 danmakuCid，数据属于别的视频就重拉。
func TestNeedLoadDanmakuOnTargetChange(t *testing.T) {
	a := &view.App{
		Info:       &view.Info{Bvid: "BVplaying", Aid: 2, Cid: 20},
		BrowseInfo: &view.Info{Bvid: "BVbrowsed", Aid: 7, Cid: 70},
		Danmaku:    []view.Danmaku{{Time: 1, Text: "A 的弹幕"}},
		DanmakuCid: 20, // 这批弹幕是 A 的
	}
	if !needLoadDanmaku(a) {
		t.Error("目标换到浏览中的 B 后应重新拉弹幕")
	}
	// 已经拉过 B 的弹幕：不重拉。
	a.DanmakuCid = 70
	if needLoadDanmaku(a) {
		t.Error("已是 B 的弹幕，不该重复拉")
	}
	// 数据属于 B 但列表为空（比如上次拉取失败）：仍要拉。
	a.Danmaku = nil
	if !needLoadDanmaku(a) {
		t.Error("还没有弹幕数据时应拉")
	}
	// 目标没有 cid：不拉。
	a.DanmakuCid = 70
	a.BrowseInfo = &view.Info{Bvid: "BVbrowsed", Aid: 7}
	if needLoadDanmaku(a) {
		t.Error("目标没有 cid 时不该拉")
	}
}

// TestNeedLoadCommentsOnTargetChange 是弹幕那条的评论版：改前只判断
// len(a.Comments)==0，播放 A 拉过评论后再播放 / 浏览 B，打开评论 tab 会一直显示
// A 的评论（原版 loadReplyList 比 replyOid）。
func TestNeedLoadCommentsOnTargetChange(t *testing.T) {
	a := &view.App{
		Info:       &view.Info{Bvid: "BVplaying", Aid: 2, Cid: 20},
		BrowseInfo: &view.Info{Bvid: "BVbrowsed", Aid: 7, Cid: 70},
		Comments:   mkComments("A", 3),
		ReplyOid:   2, // 这批评论是 A 的
	}
	if len(a.Comments) == 0 {
		t.Fatal("前置：Comments 非空（旧的 len==0 判据会误判为已加载）")
	}
	if !needLoadComments(a) {
		t.Error("目标换到浏览中的 B 后应重新拉评论")
	}
	// 已经拉过 B 的评论：不重拉。
	a.ReplyOid = 7
	if needLoadComments(a) {
		t.Error("已是 B 的评论，不该重复拉")
	}
	// 浏览信息缺失时回退到正在播放那条（ReplyOid 就是它）：不重拉。
	a.BrowseInfo = nil
	a.ReplyOid = 2
	if needLoadComments(a) {
		t.Error("回退到 A（ReplyOid=A）后不该重拉")
	}
	// 列表为空时仍要拉。
	a.Comments = nil
	if !needLoadComments(a) {
		t.Error("还没有评论数据时应拉")
	}
}

// TestBrowseInfoFromPlayItem 钉住手动从播放列表点一条时写的「浏览中」信息
// （原版 handlePlaylistVideoSelect 的 setVideoInfo）：弹幕 / 评论抽屉的目标
// 要跟着它，否则手动播 B 时抽屉还停在之前浏览的那条上。
func TestBrowseInfoFromPlayItem(t *testing.T) {
	it := view.PlayItem{Bvid: "BVx", Aid: 9, Cid: 90, Title: "标题", Pic: "封面"}
	got := browseInfoFromPlayItem(it)
	if got == nil || got.Bvid != "BVx" || got.Aid != 9 || got.Cid != 90 || got.Title != "标题" || got.Pic != "封面" {
		t.Errorf("browseInfoFromPlayItem = %+v", got)
	}
	// 源码反证：只有手动（keepDrawer）才写，自动切歌不写。
	src, err := os.ReadFile("playlist.go")
	if err != nil {
		t.Fatalf("读 playlist.go 失败：%v", err)
	}
	if !bytes.Contains(src, []byte("if keepDrawer {\n\t\tit := items[index]\n\t\ta.BrowseInfo = browseInfoFromPlayItem(it)")) {
		t.Error("playPlaylist 应在手动（keepDrawer）时写 BrowseInfo")
	}
}

// TestDanmakuTargetWiring 源码反证：弹幕 / 评论的加载目标与「已加载属于谁」的
// 记录都接上了。这些路径要真实网络才能跑，无头测试退一步钉接线。
func TestDanmakuTargetWiring(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}
	for _, needle := range []string{
		"t := danmakuTarget(a)",  // loadComments / toggleDanmaku 取浏览中那条
		"if needLoadComments(a)", // 切 tab / 打开抽屉的重拉判据
		"if !needLoadDanmaku(a)",
		"c.startDanmakuFetch(cid)", // 起播预取
		"a.ReplyOid = aid",         // 评论拉到后记录 oid
		"c.app.DanmakuCid = cid",   // 弹幕拉到后记录 cid
	} {
		if !bytes.Contains(src, []byte(needle)) {
			t.Errorf("main.go 里找不到 %s（弹幕/评论目标没接上？）", needle)
		}
	}
	// 弹幕 / 评论的加载不能再以正在播放的曲目为目标。
	for _, bad := range []string{"t := a.Current()\n\tif t == nil || t.Cid == 0", "t := a.Current()\n\tif t == nil {\n\t\treturn\n\t}"} {
		if bytes.Contains(src, []byte(bad)) {
			t.Errorf("弹幕/评论加载仍以 a.Current() 为目标：%q", bad)
		}
	}
}
