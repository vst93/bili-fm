package view

import (
	"fmt"
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestCardLiftAnimationSnappy 钉住抽屉卡片悬停动画：原版是 160ms ease，
// 但用户反馈 native 版「不够干脆」；这里固定为 120ms ease-out，按下 90ms。
func TestCardLiftAnimationSnappy(t *testing.T) {
	if cardLiftAnim.Duration != 120*time.Millisecond {
		t.Errorf("上浮时长 = %v, want 120ms", cardLiftAnim.Duration)
	}
	if cardLiftAnim.Spring != (ui.SpringOptions{}) {
		t.Errorf("应为曲线动画，不是弹簧：%+v", cardLiftAnim.Spring)
	}
	if cardLiftAnim.Ease == nil {
		t.Fatal("缺少缓动曲线")
	}
	if cardPressAnim.Duration != 90*time.Millisecond {
		t.Errorf("按下时长 = %v, want 90ms", cardPressAnim.Duration)
	}
}

// TestDrawerCardCursorIsDefault 钉住「抽屉项悬停不变成手型」：原版
// globals.css:599 起的一条全局规则把所有 button/a/[role=button]/[draggable]/
// .cursor-pointer 的 cursor 都 `default !important`（App 观感），原生版此前
// 在卡片/播放列表行/弹幕分组行上显式设了 CursorPointer。
func TestDrawerCardCursorIsDefault(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.ListFor("feed").Cards = []Card{{Bvid: "BV1", Title: "标题", Up: "UP"}}
	a.Drawer = "feed"
	tt := shellTester(t, a)
	r, ok := tt.Find("标题")
	if !ok {
		t.Fatal("找不到卡片标题")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if got := tt.Cursor(); got != ui.CursorDefault {
		t.Errorf("抽屉卡片悬停光标 = %v, want 默认箭头（%v）", got, ui.CursorDefault)
	}
}

// TestPlaylistRowCursorIsDefault 同上，针对播放列表记录行。
func TestPlaylistRowCursorIsDefault(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Playlist = []PlayItem{{ID: "a", Part: "P0", Title: "T0"}}
	a.Drawer = DrawerPlaylist
	tt := shellTester(t, a)
	r, ok := tt.Find("P0")
	if !ok {
		t.Fatal("找不到播放列表分集名")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if got := tt.Cursor(); got != ui.CursorDefault {
		t.Errorf("播放列表行悬停光标 = %v, want 默认箭头（%v）", got, ui.CursorDefault)
	}
}

// TestCoverPlaceholderDrawnWhileLoading 钉住卡片封面的占位图（渐进式加载）：
// 真图还没好先画占位图（原版 RetryImg 的 /cover.png），解码完的下一帧再被
// 真图盖掉。这里把占位图临时换成纯红，量卡片封面区的红像素。
//
// 覆盖两种情况，因为原版 RetryImg 是 `src={src || fallbackSrc}`：
//   - 有地址、未解码；
//   - 地址为空（首帧还没填 cover / 本来就没封面）—— 从前只铺底色，用户报
//     「图片要等下载完才出现、之前是一片空」。
func TestCoverPlaceholderDrawnWhileLoading(t *testing.T) {
	old := placeholderCover
	defer func() { placeholderCover = old }()
	red := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := 0; i < len(red.Pix); i += 4 {
		red.Pix[i], red.Pix[i+3] = 255, 255
	}
	placeholderCover = ui.NewBitmap(red)

	for _, tc := range []struct{ name, cover string }{
		{"有地址未解码", "http://example.invalid/x.jpg"},
		{"地址为空", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.Images = newRecordingCache()
			spec := cardSpec{
				Key: "ph-card", Cover: tc.cover, Title: "标题",
				Lazy:   true, // 只读缓存：拿不到真图 → 走占位图分支
				Fields: []cardField{{Kind: fieldAuthor, Text: "UP主"}},
			}
			img := ui.Render(func(c *ui.Context) {
				c.Root().Background(ui.RGB(90, 90, 90))
				ui.Box(c).Absolute().Left(20).Top(20).Size(220, 170).Children(func() {
					a.renderCard(c, spec)
				})
			}, 300, 260, 1)

			reds := 0
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					i := img.PixOffset(x, y)
					if img.Pix[i] > 200 && img.Pix[i+1] < 60 && img.Pix[i+2] < 60 {
						reds++
					}
				}
			}
			if reds < 1000 {
				t.Fatalf("cover=%q：封面未就绪时应画占位图，红像素只有 %d", tc.cover, reds)
			}
		})
	}
}

// TestDrawerStaysOpenOnPartClick 钉住「视图层的选集卡片点击不自己关抽屉」：
// 关面板是控制器 playIndex 的职责（对齐原版 PageList.handleVideoSelect 的
// handleOpenChange(false)），视图层只发 Act.Play；自动续播/播完切集也要保留
// 面板，所以 partCard 不能硬写 a.Drawer = ""。控制器行为见
// TestPlayIndexClosesPartsDrawerOnSelect。
func TestDrawerStaysOpenOnPartClick(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Info = &Info{Bvid: "BV1", Aid: 1, Title: "标题", Parts: []Part{
		{Cid: 11, Page: 1, Part: "第一集"},
		{Cid: 12, Page: 2, Part: "第二集"},
	}}
	a.Drawer = DrawerParts
	played := -1
	a.Act.Play = func(i int) { played = i }
	tt := shellTester(t, a)
	if err := tt.Click("第一集"); err != nil {
		t.Fatalf("点不到选集卡片：%v", err)
	}
	tt.Frame()
	if played != 0 {
		t.Fatalf("点选集卡片应播放第 0 集，played=%d", played)
	}
	if a.Drawer != DrawerParts {
		t.Errorf("点选集后抽屉 = %q, want 保持 %q", a.Drawer, DrawerParts)
	}
}

// TestPlaylistTabWidthTracksCount 钉住表头 tab「快速动态」：标签宽度随条数
// 变化（不固定），且文字单行不折行。原版 header tab 是 `white-space: nowrap`
// 且 `flex: 0 0 auto`。
func TestPlaylistTabWidthTracksCount(t *testing.T) {
	width := func(n int) float32 {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Playlist = make([]PlayItem, n)
		for i := range a.Playlist {
			a.Playlist[i] = PlayItem{ID: fmt.Sprintf("x%d", i), Part: "P", Title: "T"}
		}
		a.Drawer = DrawerPlaylist
		tt := shellTester(t, a)
		label := tabLabelForTest(n)
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("找不到 tab %q", label)
		}
		if r.H > 32.5 {
			t.Errorf("tab 高 %v（>32），文字可能折行", r.H)
		}
		return r.W
	}
	small, large := width(1), width(1000)
	if large <= small {
		t.Errorf("tab 宽度没随条数变化：1 条 %v、1000 条 %v", small, large)
	}
}

// tabLabelForTest 拼出播放列表抽屉「我的列表」tab 的文案（与 playlistTabs 一致）。
func tabLabelForTest(n int) string {
	return fmt.Sprintf("我的列表(%d)", n)
}

// TestPlaylistRowsVirtualized 钉住播放列表记录行也走虚拟化 ui.List：上百条
// 记录时只构建可见的几行。之前 playlistBody 是 ui.Column 逐行全建，每行都
// 在构建期 Bitmap() 一次缩略图——上百张 192w 图每帧争抢 64 条 LRU、互相
// 挤出对方，用户报「左侧图标不显示」。改成虚拟化后屏外的行不构建，
// 缩略图也只按可见窗口预取（见 prefetchPlaylistCovers）。
func TestPlaylistRowsVirtualized(t *testing.T) {
	a := NewApp(func() {})
	a.Images = newRecordingCache()
	for i := 0; i < 200; i++ {
		a.Playlist = append(a.Playlist, PlayItem{
			ID:    fmt.Sprintf("p%d", i),
			Bvid:  fmt.Sprintf("BV%d", i),
			Part:  fmt.Sprintf("记录%d", i),
			Title: fmt.Sprintf("视频%d", i),
			Pic:   fmt.Sprintf("http://example.invalid/pl%d.png", i),
		})
	}
	a.Drawer = DrawerPlaylist
	a.PlaylistTab = ListUser

	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
	if !tst.HasText("记录0") {
		t.Fatal("首屏应构建第一条记录行")
	}
	if tst.HasText("记录199") {
		t.Fatal("屏外的记录行不该构建（说明又退回 ui.Column 每帧全建）")
	}

	// 缩略图只预取可见窗口：200 条里不该请求几十上百张。
	rec := a.Images.(*recordingCache)
	rec.mu.Lock()
	requested := len(rec.order)
	rec.mu.Unlock()
	if requested == 0 {
		t.Fatal("可见行的缩略图一张都没预取")
	}
	if requested > 40 {
		t.Fatalf("首屏预取了 %d 张缩略图（屏外记录也被请求，懒加载失效）", requested)
	}

	tst.Scroll(490, 400, 0, 100000)
	tst.Frame()
	if !tst.HasText("记录199") {
		t.Fatal("滚到底应构建末尾记录行")
	}
	if tst.HasText("记录0") {
		t.Fatal("滚走后顶部记录行应被虚拟化回收")
	}
}

// TestPlaylistCoverPrefetchVisibleOnly 用真实缓存服务器钉住「历史添加的作品
// 左侧图标」能显示：可见窗口内的缩略图真的被请求，屏外的不请求 —— 这正是
// 「左侧图标不显示」的根因（全建时上百张图互相挤出 LRU，可见图反而没缓存）。
func TestPlaylistCoverPrefetchVisibleOnly(t *testing.T) {
	cs := newCoverServer(t)
	a := NewApp(func() {})
	for i := 0; i < 120; i++ {
		a.Playlist = append(a.Playlist, PlayItem{
			ID:   fmt.Sprintf("p%d", i),
			Bvid: fmt.Sprintf("BV%d", i),
			Part: fmt.Sprintf("记录%d", i),
			Pic:  fmt.Sprintf("%s/pl%d.png", cs.URL, i),
		})
	}
	a.Drawer = DrawerPlaylist
	a.PlaylistTab = ListUser

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tst.Frame()
		if cs.count() > 0 && a.Images.Pending() == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if cs.count() == 0 {
		t.Fatal("首屏可见缩略图一张都没请求（左侧图标会一直空着）")
	}
	if !cs.requested("/pl0.png") {
		t.Fatal("第一条记录的缩略图应被请求")
	}
	if cs.requested("/pl119.png") {
		t.Fatal("最后一条在屏外，不该被请求")
	}
}

// TestDrawerCardHoverTransition 钉住原版卡片悬停的
// `transition: background-color .16s ease, border-color .16s ease,
// box-shadow .16s ease`（globals.css:3657，与 translateY 的上浮同一时长）。
//
// 改前：底色/蓝边/投影是**当帧硬切**，只有上浮在动 —— 指针一进卡片，
// 影子先跳到位、卡片本体再浮起来。判据用「左边缘的蓝度」(B-R)：常态描边是
// 白 55%（蓝度≈0），悬停描边是 #0369a1 60%（蓝度≈90+），插值期间必须严格
// 落在两者之间；移入**当帧**（elapsed=0）仍是常态值（旧写法在这里就跳满）。
//
// 量的时候把光晕那层换成空实现（radialPaint 计数的那条通道），否则 120ms 的
// 白色径向光晕会跟着一起变亮，蓝度读数混进第二个人为因素。
func TestDrawerCardHoverTransition(t *testing.T) {
	oldRadial := radialPaint
	t.Cleanup(func() { radialPaint = oldRadial })
	radialPaint = func(p *ui.Painter, bmp *ui.Bitmap, cx, cy, radius, opacity float32) {}

	tst, box := cardGlowProbe(t)
	// 左边缘描边的蓝度：只取竖直中段，避开四角圆弧与页脚分隔线。
	blueness := func() float32 {
		img := tst.Image()
		var sum float32
		var n int
		for y := int(box.Y) + 24; y < int(box.Y+box.H)-24; y++ {
			for x := int(box.X); x <= int(box.X)+1; x++ {
				i := img.PixOffset(x, y)
				sum += float32(int(img.Pix[i+2]) - int(img.Pix[i]))
				n++
			}
		}
		return sum / float32(n)
	}

	rest := blueness()
	tst.Move(box.X+box.W/2, box.Y+box.H/2)
	enter := blueness() // 移入当帧（elapsed = 0）
	tst.Advance(30 * time.Millisecond)
	mid := blueness()
	tst.Advance(200 * time.Millisecond)
	hover := blueness()
	t.Logf("左边缘蓝度 常态=%.1f 移入当帧=%.1f 60ms=%.1f 200ms=%.1f", rest, enter, mid, hover)

	if hover-rest < 20 {
		t.Fatalf("悬停后描边没有变蓝（常态 %.1f → 悬停 %.1f），采样点或取值有问题", rest, hover)
	}
	if diff := enter - rest; diff > 1 || diff < -1 {
		t.Errorf("移入当帧描边就跳到了 %.1f（常态 %.1f）—— 没有 160ms 过渡", enter, rest)
	}
	if !(mid > rest+2 && mid < hover-2) {
		t.Errorf("30ms 中点 %.1f 不在常态 %.1f 与悬停 %.1f 之间 —— 过渡没有中间态", mid, rest, hover)
	}
	// 回落同样走过渡：移出当帧不能立刻褪成常态。
	tst.Move(box.X-80, box.Y+box.H/2)
	leave := blueness()
	tst.Advance(300 * time.Millisecond)
	back := blueness()
	t.Logf("移出当帧=%.1f 300ms 后=%.1f", leave, back)
	if d := math.Abs(float64(leave - hover)); d > 2 {
		t.Errorf("移出当帧 %.1f 应仍接近悬停 %.1f（渐隐），差 %.1f", leave, hover, d)
	}
	if d := back - rest; d > 1 || d < -1 {
		t.Errorf("移出 300ms 后应回到常态 %.1f，得到 %.1f", rest, back)
	}
}
