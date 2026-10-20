package main

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vst93/bili-fm/app/internal/view"
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
		"正在加载合集...",           // index.tsx:1661（三个 ASCII 点）
		"合集中的视频暂时都无法播放",       // index.tsx:1745
		"已加载 %d 集到播放列表",       // index.tsx:1757
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

// TestSeriesSeedArchives 钉住「播放全部」的起点：用抽屉里**已加载**的合集卡片，
// 只从「已加载到的下一页」往后翻（原版 index.tsx:1675 的
// `let nextPage = seriesVideosPage + 1`）。
//
// 此前原生版无条件 `for page := 1; page <= 50` 从第 1 页重拉：滚过 5 页的合集
// 白拉 5 次分页，而且不认抽屉里已有的内容。
func TestSeriesSeedArchives(t *testing.T) {
	list := &view.List{
		Page: 3,
		Cards: []view.Card{
			{Bvid: "BV1", Cover: "c1.jpg", Title: "第一集"},
			{Bvid: "BV2", Cover: "c2.jpg", Title: "第二集"},
			{Bvid: "BV2", Cover: "dup.jpg", Title: "重复"},
			{Bvid: "BV3", Cover: "c3.jpg", Title: "第三集"},
		},
	}
	archives, seen, nextPage := seriesSeedArchives(list)
	if nextPage != 4 {
		t.Errorf("起始页 = %d, want 4（seriesVideosPage+1，不许从第 1 页重拉）", nextPage)
	}
	if len(archives) != 3 {
		t.Fatalf("种子条目 = %d, want 3（BV2 重复要去重）", len(archives))
	}
	if archives[0].bvid != "BV1" || archives[0].pic != "c1.jpg" || archives[0].title != "第一集" {
		t.Errorf("首条 = %+v，want BV1/c1.jpg/第一集（封面与标题要一起带上）", archives[0])
	}
	if !seen["BV2"] || seen["BV9"] {
		t.Errorf("seen = %v，want 只含种子里的 bvid", seen)
	}

	// 头部曾被释放（Dropped）：手里只剩尾部窗口，不能当全集用，退回第 1 页。
	dropped := &view.List{Page: 9, Dropped: true, Cards: []view.Card{{Bvid: "BV9"}}}
	if _, _, np := seriesSeedArchives(dropped); np != 1 {
		t.Errorf("Dropped 列表的起始页 = %d, want 1", np)
	}
	// 还没加载过 / 空列表：同样从第 1 页起。
	if _, _, np := seriesSeedArchives(&view.List{}); np != 1 {
		t.Errorf("空列表的起始页 = %d, want 1", np)
	}
	if _, _, np := seriesSeedArchives(nil); np != 1 {
		t.Errorf("nil 列表的起始页 = %d, want 1", np)
	}

	// 接线上：seriesPlayAll 必须走 seriesSeedArchives，且不再有「1..50」硬循环。
	src, err := os.ReadFile("playlist.go")
	if err != nil {
		t.Fatalf("读 playlist.go 失败: %v", err)
	}
	code := string(src)
	if !strings.Contains(code, "seriesSeedArchives(drawerList)") {
		t.Fatal("seriesPlayAll 没有接上 seriesSeedArchives（仍从第 1 页重拉）")
	}
	if strings.Contains(code, "page <= 50") {
		t.Fatal("「播放全部」还留着 50 页硬上限（原版是翻到空页为止）")
	}
}

// TestSetCardsMarksDropped 钉住「头部被释放」的记账：`setCards` 超过上限从头部
// 裁掉卡片时必须把 List.Dropped 置真，否则「播放全部」会把只剩尾部窗口的列表
// 当成「已加载的全集」，从 Page+1 接着翻 —— 前面那几页的剧集就永远进不了队列。
func TestSetCardsMarksDropped(t *testing.T) {
	small := &view.List{}
	cards := make([]view.Card, 10)
	for i := range cards {
		cards[i] = view.Card{Bvid: "BV" + strconv.Itoa(i)}
	}
	setCards(small, cards, true)
	if small.Dropped {
		t.Error("10 条没超上限，Dropped 不该为真")
	}

	big := &view.List{}
	cards = make([]view.Card, maxRetainedCards+5)
	for i := range cards {
		cards[i] = view.Card{Bvid: "BV" + strconv.Itoa(i)}
	}
	setCards(big, cards, true)
	if !big.Dropped {
		t.Fatal("超上限裁掉头部后 Dropped 仍为假（播放全部会漏掉最早的几页）")
	}
	// 裁头量向上对齐到 cardRetentionAlign（2/3 列网格的公倍数 6），
	// 这样旧行首在新列表里仍是行首，虚拟列表的 Bvid 锚点不会丢。
	if len(big.Cards) != 127 {
		t.Errorf("保留 %d 条, want 127（溢出 5 条向上对齐到 6）", len(big.Cards))
	}
	if got := big.Cards[0].Bvid; got != "BV6" {
		t.Errorf("首条 = %q, want BV6（从头部释放 6 条）", got)
	}
	if got := big.Cards[len(big.Cards)-1].Bvid; got != "BV132" {
		t.Errorf("末条 = %q, want BV132（最新一条必须留着）", got)
	}
}
