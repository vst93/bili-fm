package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestHomeLayoutForWidth 钉住主区的宽度断点与取值（原版 globals.css 的
// `.home-stage` / `.home-searchbar` / `.home-now-playing` / `.cover-shell`
// 基线与 @media (max-width: 680px) 分支）。
func TestHomeLayoutForWidth(t *testing.T) {
	wide := homeLayoutFor(1000)
	if wide.Single {
		t.Error("1000px 宽不应是单列")
	}
	if wide.StagePadY != 20 || wide.StagePadX != 28 {
		t.Errorf("宽窗内边距 = %v/%v，应为 20/28", wide.StagePadY, wide.StagePadX)
	}
	if wide.CoverShell != 292 {
		t.Errorf("宽窗 .cover-shell = %v，应为 292", wide.CoverShell)
	}
	if wide.SearchWidth != 520 {
		t.Errorf("宽窗搜索药丸 = %v，应为 520（max-width 520）", wide.SearchWidth)
	}

	// 断点：<=680 单列，681 仍是两栏。
	if !homeLayoutFor(680).Single {
		t.Error("680px 应进入单列")
	}
	if homeLayoutFor(681).Single {
		t.Error("681px 不应单列")
	}

	// 400px：内边距 12/16；封面 min(196, 56vw=224)=196；药丸 = 400-32=368。
	n := homeLayoutFor(400)
	if !n.Single {
		t.Error("400px 应为单列")
	}
	if n.StagePadY != 12 || n.StagePadX != 16 {
		t.Errorf("窄窗内边距 = %v/%v，应为 12/16", n.StagePadY, n.StagePadX)
	}
	if n.CoverShell != 196 {
		t.Errorf("400px 封面 = %v，应为 196（min(196,56vw) 的上限）", n.CoverShell)
	}
	if n.SearchWidth != 368 {
		t.Errorf("400px 药丸 = %v，应为 368（100%% 于内边距内）", n.SearchWidth)
	}

	// 更窄时 56vw 生效（320 → 179.2），封面继续缩小而不是溢出。
	if got := homeLayoutFor(320).CoverShell; got < 179 || got > 180 {
		t.Errorf("320px 封面 = %v，应为 56vw ≈ 179.2", got)
	}
}

// TestTitleBarShowsBrand 钉住标题栏品牌在 <=600 隐藏（原版 titleBar.tsx 的
// `display: windowWidth <= 600 ? 'none' : ”`）。
func TestTitleBarShowsBrand(t *testing.T) {
	cases := map[float32]bool{
		1000: true,
		601:  true,
		600:  false, // <=600 隐藏（含 600）
		520:  false,
		400:  false,
	}
	for w, want := range cases {
		if got := titleBarShowsBrand(w); got != want {
			t.Errorf("titleBarShowsBrand(%v) = %v, want %v", w, got, want)
		}
	}
}

// shellTesterSize 起一个指定尺寸的无头窗口跑真实的 App.Shell。
func shellTesterSize(t *testing.T, a *App, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, w, h)
	tt.Frame()
	return tt
}

// narrowApp 造一个 400px 宽的主窗上下文：有标题/选集，封面模式下用一张确定的
// 深色假封面（碟片本身是 #0f172a 的实底，像素测量不依赖真实网络图）。
func narrowApp() *App {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.CoverMode = "disc"
	a.Track = &Track{Bvid: "BV1", Title: "标题", Part: "P1", Up: "UP"}
	giveDarkCover(a)
	return a
}

// coverNavyBox 在给定的 y 区间里找 #0f172a（唱片实底）的包围盒。用比正文更严的
// 阈值，把 #1e293b 的标题字、#334155 的时间字排除在外。
type navyBox struct{ minX, minY, maxX, maxY int }

func (b navyBox) w() int { return b.maxX - b.minX + 1 }
func (b navyBox) h() int { return b.maxY - b.minY + 1 }

func measureNavy(t *testing.T, tst *ui.Tester, x0, x1, y0, y1 int) navyBox {
	t.Helper()
	img := tst.Image()
	dark := func(x, y int) bool {
		i := img.PixOffset(x, y)
		return int(img.Pix[i]) < 28 && int(img.Pix[i+1]) < 36 && int(img.Pix[i+2]) < 62
	}
	box := navyBox{minY: -1}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if dark(x, y) {
				if box.minY < 0 {
					box.minY = y
				}
				box.maxY = y
				if box.minX == 0 || x < box.minX {
					box.minX = x
				}
				if x > box.maxX {
					box.maxX = x
				}
			}
		}
	}
	if box.minY < 0 {
		t.Fatalf("在 y∈[%d,%d) x∈[%d,%d) 里找不到封面深色区域", y0, y1, x0, x1)
	}
	return box
}

// TestNarrowWindowSingleColumn 是无头实测：400px 宽下主区应退化成单列 ——
// 封面按 min(196,56vw)=196 的 shell 缩放（碟片 = 90% ≈ 176），水平居中，
// 且整块落在视频信息（标题）之上。
func TestNarrowWindowSingleColumn(t *testing.T) {
	const w, h = 400, 700
	a := narrowApp()
	tt := shellTesterSize(t, a, w, h)

	title, ok := tt.Find("标题")
	if !ok {
		t.Fatal("找不到视频标题标签，无法确定封面/信息的分界")
	}
	t.Logf("标题 rect = %+v", title)
	if title.Y < 200 {
		t.Fatalf("标题 y=%v 太靠上：单列时封面应在标题之上", title.Y)
	}

	cover := measureNavy(t, tt, 0, w, 0, int(title.Y))
	t.Logf("封面深色盒 = %+v（%d×%d）", cover, cover.w(), cover.h())
	// 碟片宽 = shell(196) × 90% ≈ 176，允许几像素的抗锯齿/取整误差。
	if cover.w() < 170 || cover.w() > 182 {
		t.Errorf("窄窗碟片宽 = %d，应为 ~176（shell 196 × 90%%）", cover.w())
	}
	if d := cover.h() - cover.w(); d < -4 || d > 4 {
		t.Errorf("窄窗碟片 %d×%d 不像正圆", cover.w(), cover.h())
	}
	// 单列里封面在 .home-now-playing 的 1fr 列里居中 → 中心 ≈ 窗口中心。
	cx := (cover.minX + cover.maxX) / 2
	if cx < w/2-6 || cx > w/2+6 {
		t.Errorf("窄窗封面中心 x=%d，应 ≈ 窗口中心 %d", cx, w/2)
	}
	// 封面底边必须在标题之上（单列顺序：封面 → UP 主行 → 标题）。
	if cover.maxY >= int(title.Y) {
		t.Errorf("封面底边 %d 未在标题 %v 之上", cover.maxY, title.Y)
	}
}

// TestNarrowWindowSearchBarFits 钉住搜索药丸在窄窗不溢出：此前药丸宽度写死
// 520，窗口窄于 576 时四个内容入口与头像会被挤出窗口（原版是 max-width:100%
// 的流式宽度）。这里用 400px 窗口实测四个入口键都落在窗口内。
func TestNarrowWindowSearchBarFits(t *testing.T) {
	const w, h = 400, 700
	a := narrowApp()
	tt := shellTesterSize(t, a, w, h)

	for _, label := range []string{"动态", "热门与推荐", "收藏", "历史", "登录账号"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("找不到 %q", label)
		}
		if r.X < 0 || r.X+r.W > w {
			t.Errorf("%q 溢出窄窗：rect=%+v（窗口宽 %d）", label, r, w)
		}
	}
}

// TestTitleBarHidesBrandOnNarrowWindow 是无头实测：<=600 时品牌（bili-FM）
// 不再绘制；>600 时正常显示。
func TestTitleBarHidesBrandOnNarrowWindow(t *testing.T) {
	narrow := narrowApp()
	ttNarrow := shellTesterSize(t, narrow, 500, 700)
	if ttNarrow.HasText("bili-FM") {
		t.Error("500px 宽仍显示了标题栏品牌（原版 <=600 应隐藏）")
	}

	wide := narrowApp()
	ttWide := shellTesterSize(t, wide, 900, 700)
	if !ttWide.HasText("bili-FM") {
		t.Error("900px 宽没有显示标题栏品牌")
	}
}

// TestDrawerNarrowLayouts 钉住抽屉里此前还没接的两条宽度媒体查询
// （原版 globals.css）：隐身开关 ≤520 收窄并隐藏文字、弹幕分组 ≤420 收窄左列。
func TestDrawerNarrowLayouts(t *testing.T) {
	// 隐身开关：断点 520（含 520 走窄分支）。
	for _, c := range []struct {
		w     float32
		minW  float32
		label bool
	}{
		{980, 92, true},
		{521, 92, true},
		{520, 66, false},
		{400, 66, false},
	} {
		minW, label := incognitoLayout(c.w)
		if minW != c.minW || label != c.label {
			t.Errorf("incognitoLayout(%v) = (%v, %v), want (%v, %v)",
				c.w, minW, label, c.minW, c.label)
		}
	}

	// 弹幕分组：断点 420（含 420 走窄分支）。
	for _, c := range []struct{ w, col, gap, pad float32 }{
		{980, 64, 10, 12},
		{421, 64, 10, 12},
		{420, 54, 8, 10},
		{320, 54, 8, 10},
	} {
		l := danmakuRowLayoutFor(c.w)
		if l.Col != c.col || l.Gap != c.gap || l.PadInline != c.pad {
			t.Errorf("danmakuRowLayoutFor(%v) = %+v, want col=%v gap=%v pad=%v",
				c.w, l, c.col, c.gap, c.pad)
		}
	}
}

// historyDrawerApp 造一个打开历史抽屉（观看历史 tab）的 App，用来量隐身开关。
func historyDrawerApp() *App {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = "history"
	a.HistTab = HistHistory
	a.ListFor("history").Cards = []Card{
		{Bvid: "BV1", Title: "看过的视频", Up: "UP主", Views: "1.2万", Pubdate: "09-22"},
	}
	return a
}

// TestIncognitoSwitchNarrowWidth 是无头实测：>520 时隐身开关是「图标 + 文字 +
// 轨道」（min-width 92），≤520 时原版隐藏文字标签并把 min-width 收到 66 ——
// 此前原生版没有这条媒体查询，窄窗里开关一直占着 92px 把表头挤爆。
func TestIncognitoSwitchNarrowWidth(t *testing.T) {
	wide := historyDrawerApp()
	tw := shellTesterSize(t, wide, 600, 700)
	rw, ok := tw.Find("隐身")
	if !ok {
		t.Fatal("历史抽屉表头找不到隐身开关")
	}
	if rw.W < 88 {
		t.Errorf(">520 时隐身开关宽 = %v，应为 92（图标 + 文字 + 轨道）", rw.W)
	}

	narrow := historyDrawerApp()
	tn := shellTesterSize(t, narrow, 500, 700)
	rn, ok := tn.Find("隐身")
	if !ok {
		t.Fatal("窄窗历史抽屉表头找不到隐身开关")
	}
	if rn.W > 70 {
		t.Errorf("<=520 时隐身开关宽 = %v，应为 66（文字隐藏、min-width 收到 66）", rn.W)
	}
	// 反证：两态必须有可观测差异，否则这条断点等于没接。
	if !(rn.W < rw.W) {
		t.Errorf("窄窗开关宽 %v 未小于宽窗 %v", rn.W, rw.W)
	}
}

// danmakuDrawerApp 造一个打开弹幕抽屉的 App。
func danmakuDrawerApp() *App {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Pos = 0 // 不自动跟随，停在列表顶部
	a.Drawer = DrawerDanmaku
	a.DanmakuTab = TabDanmaku
	a.Danmaku = []Danmaku{
		{Time: 1, Text: "第一条弹幕"},
		{Time: 5, Text: "第二条弹幕"},
	}
	return a
}

// TestDanmakuGroupNarrowColumn 是无头实测：弹幕分组的消息列左缘随窗口宽度
// 收窄（原版 .danmaku-time-group 的 ≤420：左列 64→54、gap 10→8、
// padding-inline 12→10，共内收 8px）。这里量第一条弹幕文字的左缘：
// 宽窗 ≈ 10(list pad) + 12 + 64 + 10 + 9(chip pad) = 105，窄窗 ≈ 91。
func TestDanmakuGroupNarrowColumn(t *testing.T) {
	wide := danmakuDrawerApp()
	tw := shellTesterSize(t, wide, 980, 700)
	rw, ok := tw.Find("第一条弹幕")
	if !ok {
		t.Fatal("宽窗弹幕抽屉里找不到第一条弹幕")
	}
	if rw.X < 103 || rw.X > 107 {
		t.Errorf("宽窗弹幕文字左缘 x = %v，应 ≈ 105（64/10/12 基线）", rw.X)
	}

	narrow := danmakuDrawerApp()
	tn := shellTesterSize(t, narrow, 400, 700)
	rn, ok := tn.Find("第一条弹幕")
	if !ok {
		t.Fatal("窄窗弹幕抽屉里找不到第一条弹幕")
	}
	if rn.X < 89 || rn.X > 93 {
		t.Errorf("窄窗弹幕文字左缘 x = %v，应 ≈ 91（≤420 的 54/8/10 分支）", rn.X)
	}
	if !(rn.X < rw.X) {
		t.Errorf("窄窗消息列左缘 %v 未内收于宽窗 %v", rn.X, rw.X)
	}
}
