package view

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestPlaylistRowPaintStates 钉住播放列表记录行的配色/字重，逐条对应原版
// globals.css 里 `[data-slot="wrapper"]` 作用域下的 .group / .group:hover /
// .playlist-current / .playlist-current:hover / [class*="bg-blue-50"] /
// :active 六组规则。这些是用户可感知的差异：原生版此前常态没有玻璃卡底、
// 悬停只有白 .38、文案色用了 Tailwind 原值（未被绘板的覆写），当前项也只
// 有两段渐变（原版悬停是三段，见 playlistCurrentHoverStops）。
func TestPlaylistRowPaintStates(t *testing.T) {
	const white = "#ffffff"

	t.Run("常态", func(t *testing.T) {
		p := playlistRowPaint(false, false, false, false)
		if p.From != ui.Hex(white).Alpha(0.42) || p.Gradient {
			t.Errorf("底色 = %v (gradient=%v), want 白 .42 纯色", p.From, p.Gradient)
		}
		if p.Border != ui.Hex(white).Alpha(0.28) {
			t.Errorf("边 = %v, want 白 .28", p.Border)
		}
		if p.InsetTop != ui.Hex(white).Alpha(0.42) {
			t.Errorf("内高光 = %v, want 白 .42", p.InsetTop)
		}
		if p.Shadow != ui.Hex("#0f172a").Alpha(0.04) || p.ShadowY != 2 || p.ShadowBlur != 8 {
			t.Errorf("外投影 = %v y=%v blur=%v, want 15,23,42 .04 y2/blur8",
				p.Shadow, p.ShadowY, p.ShadowBlur)
		}
		if p.AccentW != 0 || p.Ring {
			t.Errorf("常态不该有强调条/环：accentW=%v ring=%v", p.AccentW, p.Ring)
		}
		if p.PartInk != ui.Hex("#1e293b") || p.PartWeight != 600 {
			t.Errorf("分集名 = %v/%d, want #1e293b/600", p.PartInk, p.PartWeight)
		}
		if p.TitleInk != ui.Hex("#64748b") || p.TitleWeight != 400 {
			t.Errorf("视频名 = %v/%d, want #64748b/400", p.TitleInk, p.TitleWeight)
		}
	})

	t.Run("悬停", func(t *testing.T) {
		p := playlistRowPaint(false, true, false, false)
		if p.From != ui.Hex(white).Alpha(0.55) || p.Gradient {
			t.Errorf("悬停底 = %v (gradient=%v), want 白 .55（.hover\\:bg-gray-100:hover 最后生效）", p.From, p.Gradient)
		}
		if p.Border != ui.Hex("#0ea5e9").Alpha(0.34) {
			t.Errorf("悬停边 = %v, want 天蓝 .34", p.Border)
		}
		if p.InsetTop != ui.Hex(white).Alpha(0.70) {
			t.Errorf("悬停内高光 = %v, want 白 .70", p.InsetTop)
		}
		if p.Shadow != ui.Hex("#0f172a").Alpha(0.10) || p.ShadowY != 8 || p.ShadowBlur != 22 {
			t.Errorf("悬停外投影 = %v y=%v blur=%v, want 15,23,42 .10 y8/blur22",
				p.Shadow, p.ShadowY, p.ShadowBlur)
		}
		if !p.Ring {
			t.Error("悬停该有 1px 天蓝环")
		}
	})

	t.Run("当前项", func(t *testing.T) {
		p := playlistRowPaint(true, false, false, false)
		// 非悬停时 .bg-blue-100 的平铺色（比 .playlist-current 的渐变靠后）生效。
		if p.Gradient || p.From != ui.Hex("#0284c7").Alpha(0.16) {
			t.Errorf("当前项常态底 = %v (gradient=%v), want 平铺 2,132,199 .16", p.From, p.Gradient)
		}
		if p.Border != ui.Hex("#0284c7").Alpha(0.46) {
			t.Errorf("当前项边 = %v, want 2,132,199 .46", p.Border)
		}
		if p.AccentW != 0 || p.Accent != ui.Transparent {
			t.Errorf("当前项不该再有左缘强调条：accentW=%v accent=%v", p.AccentW, p.Accent)
		}
		if p.InsetTop != ui.Hex(white).Alpha(0.64) || p.InsetBot != ui.Hex("#0284c7").Alpha(0.16) {
			t.Errorf("内影 = %v/%v, want 白 .64 / 2,132,199 .16", p.InsetTop, p.InsetBot)
		}
		if p.Shadow != ui.Hex("#0284c7").Alpha(0.14) || p.ShadowY != 8 || p.ShadowBlur != 22 {
			t.Errorf("当前项外投影 = %v y=%v blur=%v, want 2,132,199 .14 y8/blur22",
				p.Shadow, p.ShadowY, p.ShadowBlur)
		}
		if p.PartInk != ui.Hex("#064e7a") || p.PartWeight != 800 {
			t.Errorf("当前项分集名 = %v/%d, want #064e7a/800", p.PartInk, p.PartWeight)
		}
		if p.TitleInk != ui.Hex("#334155") || p.TitleWeight != 600 {
			t.Errorf("当前项视频名 = %v/%d, want #334155/600", p.TitleInk, p.TitleWeight)
		}
	})

	t.Run("当前项悬停", func(t *testing.T) {
		p := playlistRowPaint(true, true, false, false)
		if !p.Gradient {
			t.Fatal("当前项悬停应是渐变底")
		}
		// 原版是**三段**渐变（天蓝 .36 → .20@42% → 白 .62）；两段近似在 42%
		// 处的停靠色不是 .20 天蓝（按预乘插值混成浅一档）。逐停靠点钉住。
		want := []ui.GradientStop{
			{Color: ui.Hex("#0ea5e9").Alpha(0.36), At: 0},
			{Color: ui.Hex("#0ea5e9").Alpha(0.20), At: 0.42},
			{Color: ui.Hex(white).Alpha(0.62), At: 1},
		}
		if len(p.Stops) != len(want) {
			t.Fatalf("当前项悬停停靠点 %d 个，want %d（原版三段渐变）", len(p.Stops), len(want))
		}
		for i := range want {
			if p.Stops[i] != want[i] {
				t.Errorf("停靠点 %d = %+v，want %+v", i, p.Stops[i], want[i])
			}
		}
		if p.Border != ui.Hex("#0284c7").Alpha(0.58) {
			t.Errorf("当前项悬停边 = %v, want 2,132,199 .58", p.Border)
		}
	})

	t.Run("拖拽悬停", func(t *testing.T) {
		p := playlistRowPaint(false, false, true, false)
		if !p.Gradient || p.From != ui.Hex("#0ea5e9").Alpha(0.24) || p.To != ui.Hex(white).Alpha(0.56) {
			t.Errorf("拖拽渐变 = %v→%v, want 天蓝 .24→白 .56", p.From, p.To)
		}
		if p.AccentW != 0 || p.Accent != ui.Transparent {
			t.Errorf("拖拽不该有左缘强调条：accentW=%v accent=%v", p.AccentW, p.Accent)
		}
		if p.InsetTop != ui.Hex(white).Alpha(0.60) {
			t.Errorf("拖拽内高光 = %v, want 白 .60", p.InsetTop)
		}
		if p.Shadow != ui.Hex("#0284c7").Alpha(0.12) || p.ShadowY != 4 || p.ShadowBlur != 14 {
			t.Errorf("拖拽外投影 = %v y=%v blur=%v, want 2,132,199 .12 y4/blur14",
				p.Shadow, p.ShadowY, p.ShadowBlur)
		}
	})

	t.Run("按下只改底色", func(t *testing.T) {
		base := playlistRowPaint(false, true, false, true)
		if base.Gradient || base.From != ui.Hex(white).Alpha(0.30) {
			t.Errorf("常态按下底 = %v (gradient=%v), want 白 .30 纯色", base.From, base.Gradient)
		}
		if base.Ring {
			t.Error("按下不该保留 hover 环（:active 覆盖背景，环随 hover 仍在；这里只要求背景语义正确）")
		}
		cur := playlistRowPaint(true, true, false, true)
		if cur.Gradient || cur.From != ui.Hex("#0284c7").Alpha(0.10) {
			t.Errorf("当前项按下底 = %v (gradient=%v), want 2,132,199 .10 纯色",
				cur.From, cur.Gradient)
		}
		if cur.AccentW != 0 {
			t.Error("当前项按下也不该再有左缘强调条")
		}
	})
}

// TestPlaylistRowRendered 无头实测：把行画出来，验证当前项的分集名/视频名
// 确实用上了绘板覆写色（比 Tailwind 原值更暗）。
func TestPlaylistRowRendered(t *testing.T) {
	a := NewApp(func() {})
	a.Playlist = []PlayItem{
		{ID: "a", Part: "P0", Title: "T0"},
		{ID: "b", Part: "P1", Title: "T1"},
	}
	a.PlayingPlaylist = ListUser
	a.Index = 0
	a.Drawer = DrawerPlaylist
	tt := shellTester(t, a)

	r0, ok := tt.Find("P0")
	if !ok {
		t.Fatal("找不到当前项的分集名 P0")
	}
	r1, ok := tt.Find("P1")
	if !ok {
		t.Fatal("找不到常态项的分集名 P1")
	}
	t0, ok := tt.Find("T0")
	if !ok {
		t.Fatal("找不到当前项的视频名 T0")
	}
	t1, ok := tt.Find("T1")
	if !ok {
		t.Fatal("找不到常态项的视频名 T1")
	}

	img := tt.Image()
	pick := func(r ui.Rect) [3]int {
		return darkestIn(img, int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
	}
	curPart, basePart := pick(r0), pick(r1)
	curTitle, baseTitle := pick(t0), pick(t1)
	t.Logf("分集名 当前=%v 常态=%v；视频名 当前=%v 常态=%v", curPart, basePart, curTitle, baseTitle)

	// 当前项分集名 #064e7a（R6 G78 B122）：B-R ≈ 116。
	// 旧值 #0284c7（R2 G132 B199）B-R ≈ 197。取 < 160 即可区分。
	if d := curPart[2] - curPart[0]; d >= 160 {
		t.Errorf("当前项分集名偏亮/偏鲜（B-R=%d），应接近 #064e7a", d)
	}
	lum := func(c [3]int) int { return (c[0]*299 + c[1]*587 + c[2]*114) / 1000 }
	// 当前项视频名 #334155（亮度 ≈ 66）；旧值 #9ca3af 亮度 ≈ 163。
	if lum(curTitle) > 130 {
		t.Errorf("当前项视频名太淡（亮度=%d），应接近 #334155", lum(curTitle))
	}
	// 常态视频名 #64748b（亮度 ≈ 117）；旧值 #9ca3af 亮度 ≈ 163。
	if lum(baseTitle) > 150 {
		t.Errorf("常态视频名太淡（亮度=%d），应接近 #64748b", lum(baseTitle))
	}
}

// TestPlaylistDeleteAlwaysReserved 无头实测：记录行的删除键**非悬停时也在布局里**
// （原版是 `opacity-0`，只切不透明度、不切存在），所以悬停前后文字列宽度必须一致。
// 原生版此前把「是否存在」挂在 `row.Hovered()` 上，悬停一落上文字列就被抽窄
// 26+8=34px，长标题当场重新截断、整行文字跳一下。
//
// 反证：把 `if editable` 换回 `if editable && row.Hovered()` → 静息文字列 660、
// 悬停 626，本测试红。
func TestPlaylistDeleteAlwaysReserved(t *testing.T) {
	long := strings.Repeat("很长分集名称", 20)
	a := NewApp(func() {})
	a.Playlist = []PlayItem{
		{ID: "a", Part: "P0", Title: "T0"},
		{ID: "b", Part: long, Title: "T1"},
	}
	a.PlayingPlaylist = ListUser
	a.Index = 0
	a.Drawer = DrawerPlaylist
	tt := shellTester(t, a)

	if _, ok := tt.Find("删除"); !ok {
		t.Fatal("静息时删除键不在布局里 —— 原版它始终占位（opacity-0）")
	}
	rest, ok := tt.Find(long)
	if !ok {
		t.Fatal("找不到长分集名")
	}
	tt.Move(rest.X+rest.W/2, rest.Y+rest.H/2)
	tt.Frame()
	hover, ok := tt.Find(long)
	if !ok {
		t.Fatal("悬停后找不到长分集名")
	}
	t.Logf("文字列宽 静息=%v 悬停=%v", rest.W, hover.W)
	if rest.W != hover.W {
		t.Fatalf("悬停让文字列宽从 %v 变到 %v：删除键没有常驻占位", rest.W, hover.W)
	}
}

// TestPlaylistDeleteFadeTokens 钉住删除键淡入的时长与两端目标：原版
// `transition-opacity` 是 150ms + cubic-bezier(.4, 0, .2, 1)，常态 0 / 行悬停 1。
func TestPlaylistDeleteFadeTokens(t *testing.T) {
	if playlistDeleteFadeAnim.Duration != 150*time.Millisecond {
		t.Errorf("淡入时长 = %v，应为 150ms（Tailwind transition-opacity 默认）",
			playlistDeleteFadeAnim.Duration)
	}
	if playlistDeleteFadeAnim.Ease == nil {
		t.Error("淡入没有缓动曲线")
	}
	if got := playlistDeleteOpacity(false); got != 0 {
		t.Errorf("静息不透明度 = %v，应为 0", got)
	}
	if got := playlistDeleteOpacity(true); got != 1 {
		t.Errorf("悬停不透明度 = %v，应为 1", got)
	}
}

// TestPlaylistDeleteFadesIn 无头实测：静息时删除键不可见（opacity 0，图标不画），
// 行悬停后淡入显形。反证：把 `rowIconButton` 里的 `Opacity(AnimateTo(...))` 去掉
// （或恒置 1）→ 静息也能采到 #999 的图标像素，本测试红。
func TestPlaylistDeleteFadesIn(t *testing.T) {
	long := strings.Repeat("很长分集名称", 20)
	a := NewApp(func() {})
	a.Playlist = []PlayItem{
		{ID: "a", Part: "P0", Title: "T0"},
		{ID: "b", Part: long, Title: "T1"},
	}
	a.PlayingPlaylist = ListUser
	a.Index = 0
	a.Drawer = DrawerPlaylist
	tt := shellTester(t, a)

	rest, ok := tt.Find(long)
	if !ok {
		t.Fatal("找不到长分集名")
	}
	// 行 b 的删除键在文字列右侧 8px，与文字行垂直居中。
	bx, by := int(rest.X+rest.W+8), int(rest.Y+rest.H/2-13)
	darkest := func() int {
		c := darkestIn(tt.Image(), bx, by, bx+26, by+26)
		return (c[0]*299 + c[1]*587 + c[2]*114) / 1000
	}
	restLum := darkest()
	tt.Move(rest.X+rest.W/2, rest.Y+rest.H/2)
	tt.Advance(300 * time.Millisecond) // 推过 150ms 删除键淡入（v0.5.5 冻结时钟）
	tt.Frame()
	hoverLum := darkest()
	t.Logf("删除键区域亮度 静息=%d 悬停=%d", restLum, hoverLum)
	if restLum < 220 {
		t.Fatalf("静息时删除键已可见（亮度=%d），应停在 opacity 0", restLum)
	}
	if hoverLum > 200 {
		t.Fatalf("悬停后删除键仍不可见（亮度=%d），淡入没接上", hoverLum)
	}
}

// TestPlaylistRowDragTokens 钉住拖拽态的两个纯函数：
//   - playlistRowOpacity：被拖的源行带 `opacity-40`（0.4），其余 1；
//   - playlistDragOver：原版 isDragOver 只在 dragIndex !== index 时置位，
//     所以被拖的源行即使指针回到自己身上也不切成 bg-blue-50。
func TestPlaylistRowDragTokens(t *testing.T) {
	if got := playlistRowOpacity(true); got != 0.4 {
		t.Errorf("拖拽中的行不透明度 = %v，应为 0.4（Tailwind opacity-40）", got)
	}
	if got := playlistRowOpacity(false); got != 1 {
		t.Errorf("静息行不透明度 = %v，应为 1", got)
	}
	if playlistDragOver(true, true) {
		t.Error("被拖的源行不该带拖拽悬停高亮（原版 isDragOver 排除 dragIndex）")
	}
	if !playlistDragOver(false, true) {
		t.Error("别的行被悬停时应带拖拽悬停高亮")
	}
	if playlistDragOver(false, false) {
		t.Error("没有拖拽悬停时不该高亮")
	}
}

// TestPlaylistRowDimsWhileDragging 无头实测：拖起某一行后，源行整行降到 40%
// 不透明度（分集名从深色变淡），而没被拖的行保持深色。
//
// 反证：把 `row.Opacity(playlistRowOpacity(selfDragging))` 去掉 → 拖拽中源行的
// 分集名亮度仍是静息值，本测试红。
func TestPlaylistRowDimsWhileDragging(t *testing.T) {
	a := NewApp(func() {})
	a.Playlist = []PlayItem{
		{ID: "a", Part: "P0", Title: "T0"},
		{ID: "b", Part: "P1", Title: "T1"},
		{ID: "c", Part: "P2", Title: "T2"},
	}
	a.PlayingPlaylist = ListUser
	a.Index = -1 // 无当前项，排除强调条/当前项配色的干扰
	a.Drawer = DrawerPlaylist
	tt := shellTester(t, a)

	p1, ok := tt.Find("P1")
	if !ok {
		t.Fatal("找不到分集名 P1")
	}
	t1, ok := tt.Find("T1")
	if !ok {
		t.Fatal("找不到视频名 T1")
	}
	p0, ok := tt.Find("P0")
	if !ok {
		t.Fatal("找不到分集名 P0")
	}
	lum := func(r ui.Rect) int {
		c := darkestIn(tt.Image(), int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
		return (c[0]*299 + c[1]*587 + c[2]*114) / 1000
	}
	rest := lum(p1)
	// 起拖：按在封面（非可选文本）上，横向拖 >4 DIP。拖到窗口右缘，避开
	// 跟着指针的拖拽副本对测量区域的覆盖。
	tt.Press(p1.X-40, (p1.Y+t1.Y+t1.H)/2)
	tt.Move(790, (p1.Y+t1.Y+t1.H)/2)
	tt.Frame()
	p1b, ok := tt.Find("P1")
	if !ok {
		t.Fatal("拖拽中找不到分集名 P1")
	}
	dragging := lum(p1b)
	other := lum(p0)
	t.Logf("分集名亮度 静息=%d 拖拽中=%d；未拖行=%d", rest, dragging, other)
	if rest > 90 {
		t.Fatalf("静息分集名不够深（亮度=%d），测试前提不成立", rest)
	}
	if other > 90 {
		t.Fatalf("未拖的行也变淡了（亮度=%d），不该受影响", other)
	}
	if dragging < 120 {
		t.Fatalf("拖拽中源行没变淡（亮度=%d），opacity-40 没接上", dragging)
	}
}

// TestPlayModeIconInk 逐值钉住播放模式键图标色（原版 playlist.tsx）：
// 顺序 Order fill="#888"；单曲循环 LoopOnce 与随机 Shuffle 都是 fill="#3b82f6"。
func TestPlayModeIconInk(t *testing.T) {
	if got := playModeIconInk(PlayModeSequence); got != ui.Hex("#888888") {
		t.Errorf("顺序图标色 = %v，应为 #888888", got)
	}
	if got := playModeIconInk(PlayModeSingle); got != ui.Hex("#3b82f6") {
		t.Errorf("单曲循环图标色 = %v，应为 #3b82f6", got)
	}
	if got := playModeIconInk(PlayModeShuffle); got != ui.Hex("#3b82f6") {
		t.Errorf("随机图标色 = %v，应为 #3b82f6", got)
	}
}

// TestPlaylistNoAccentBar 钉住本轮改动：当前项 / 拖拽态都不再画左缘强调条。
// 用户反馈那条 4px 纯蓝竖线太突兀（行本身已有蓝底 + 蓝边 + 蓝投影，再叠一条
// 竖条反而脏），所以 AccentW 一律保持 0，渲染分支不会画。
func TestPlaylistNoAccentBar(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		current, hover, drag bool
	}{
		{"当前项", true, false, false},
		{"当前项悬停", true, true, false},
		{"拖拽", false, false, true},
		{"常态", false, false, false},
	} {
		p := playlistRowPaint(tc.current, tc.hover, tc.drag, false)
		if p.AccentW != 0 || p.Accent != ui.Transparent {
			t.Errorf("%s仍有左缘强调条：accentW=%v accent=%v", tc.name, p.AccentW, p.Accent)
		}
	}
}

// TestPlaylistCurrentRowNotFlush 钉住「当前播放项的缩略图不贴行左缘」。
//
// 回归点：当前项曾把左内边距收成 0（为了用「强调条 4 + gap 8」凑出原版的
// 12px 内容起点），强调条后来删掉后内边距却没收回，缩略图直接贴在行左缘
// （用户报「当前播放项的左侧图片贴边了」）。
//
// 判法：当前项在分集名前多画一颗 12px 的播放图标 + 4px gap，所以当前项的
// 分集名 x 应比普通项大 16；若当前项的左内边距又被收成 0，差值会掉到 8。
func TestPlaylistCurrentRowNotFlush(t *testing.T) {
	if playlistRowPadL <= 0 {
		t.Fatalf("左内边距 = %v，应 >0（当前项也一样）", playlistRowPadL)
	}
	render := func(idx int) ui.Rect {
		a := NewApp(func() {})
		a.Playlist = []PlayItem{
			{ID: "a", Part: "P0", Title: "T0"},
			{ID: "b", Part: "P1", Title: "T1"},
		}
		a.PlayingPlaylist = ListUser
		a.Index = idx
		a.Drawer = DrawerPlaylist
		tt := shellTester(t, a)
		p0, ok := tt.Find("P0")
		if !ok {
			t.Fatal("找不到分集名 P0")
		}
		return p0
	}
	normal := render(-1)
	current := render(0)
	delta := current.X - normal.X
	if delta < 15.5 || delta > 16.5 {
		t.Errorf("当前项分集名 x 比普通项大 %v，应为 16（播放图标 12 + gap 4）；"+
			"偏小说明当前项左内边距没收回、缩略图又贴边了", delta)
	}
}

// playlistCurrentProbe 起一个抽屉停在播放列表 tab 的 800×600 无头窗口，
// 当前项是第 0 条（a.Index=0 + a.PlayingPlaylist=ListUser → 该行 isCurrent）。
func playlistCurrentProbe(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = DrawerPlaylist
	a.PlaylistTab = ListUser
	a.PlayingPlaylist = ListUser
	a.Index = 0
	a.Playlist = []PlayItem{
		{ID: "a", Part: "第一集", Title: "T0"},
		{ID: "b", Part: "第二集", Title: "T1"},
	}
	return a, shellTester(t, a)
}

// TestPlaylistPartEmbossValues 钉住当前项分集名那条白色压印的参数，逐值对应
// 原版 globals.css:3948 的
//
//	[data-slot="wrapper"] .playlist-current .text-blue-600,
//	[data-slot="wrapper"] .playlist-current p:first-of-type {
//	    text-shadow: 0 1px 0 rgba(255, 255, 255, 0.65);
//	}
//
// 说明为什么不是 :3942 的 `.52`：那条 `.text-blue-600` 规则只被 playlist.tsx
// 的当前项首行用到，而它一定同时带 `.playlist-current`，后者特异度更高且更靠后，
// 所以生效值恒为 .65（见 playlist.go 的注释）。
func TestPlaylistPartEmbossValues(t *testing.T) {
	if playlistPartShadowY != 1 {
		t.Errorf("压印下移 = %v，应为 1（text-shadow 的 y 偏移）", playlistPartShadowY)
	}
	if playlistPartShadowInk != 0.65 {
		t.Errorf("压印白 = %v，应为 .65（.playlist-current 覆写后的值）", playlistPartShadowInk)
	}
}

// TestPlaylistPartEmbossOnlyCurrent 无头反证：压印只挂在**当前项首行**，
// 且用的是首行的字体与色值；常态行、次行都不该出现压印。
func TestPlaylistPartEmbossOnlyCurrent(t *testing.T) {
	orig := playlistPartEmboss
	defer func() { playlistPartEmboss = orig }()
	var calls []string
	playlistPartEmboss = func(b ui.Element, s string, font ui.Font, ink ui.Color) ui.Element {
		calls = append(calls, s)
		if font.Size != 14 || font.Weight != 800 {
			t.Errorf("压印字体 = %v/%d，应为 14/800（当前项首行 text-sm + 覆写 800）",
				font.Size, font.Weight)
		}
		if want := ui.Hex("#ffffff").Alpha(playlistPartShadowInk); ink != want {
			t.Errorf("压印色 = %v，应为 %v", ink, want)
		}
		return orig(b, s, font, ink)
	}
	_, tst := playlistCurrentProbe(t)
	tst.Frame()
	if len(calls) == 0 {
		t.Fatal("当前项首行没挂压印（playlistPartEmboss 没被调用）")
	}
	for _, s := range calls {
		if s != "第一集" {
			t.Errorf("压印挂到了 %q 上；只有当前项首行（第一集）该有压印", s)
		}
	}
}

// TestPlaylistPartEmbossAddsWhite 无头实测：当前项首行区域的亮度在挂了压印后
// 变高（下移 1px 的白色字形副本从深色字形的下缘露出），反证：把
// playlistPartEmboss 换成直通桩，亮度应回落。这与 CSS 的 text-shadow 同义，
// 不是被用户否掉的「方形白底」——量的是字形轮廓里的像素，整块底色的亮度不变。
func TestPlaylistPartEmbossAddsWhite(t *testing.T) {
	boxLuma := func(img *image.RGBA, r ui.Rect) int {
		sum := 0
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				sum += lumaSum(rgbAt(img, x, y))
			}
		}
		return sum
	}

	_, on := playlistCurrentProbe(t)
	r, ok := on.Find("第一集")
	if !ok {
		t.Fatal("找不到当前项分集名「第一集」")
	}
	with := boxLuma(on.Image(), r)

	orig := playlistPartEmboss
	defer func() { playlistPartEmboss = orig }()
	playlistPartEmboss = func(b ui.Element, s string, font ui.Font, ink ui.Color) ui.Element { return b }
	_, off := playlistCurrentProbe(t)
	if _, ok := off.Find("第一集"); !ok {
		t.Fatal("反证组找不到分集名")
	}
	without := boxLuma(off.Image(), r)

	t.Logf("首行区域亮度：有压印 %d / 无压印 %d（+%.2f%%）",
		with, without, 100*float64(with-without)/float64(without))
	if with <= without {
		t.Errorf("压印没有让首行更亮：有压印 %d / 无压印 %d（反证应更暗）", with, without)
	}
}

// TestPlaylistRowHoverTransition 钉住原版记录行悬停的
// `transition: background 0.16s ease, border-color 0.16s ease,
// box-shadow 0.16s ease`（globals.css:3902）。
//
// 改前：底色 / 蓝边 / 投影都是**当帧硬切**，只有按压缩放在动 —— 鼠标沿播放
// 列表上下滑时每行都在闪，与上一轮修掉的抽屉卡片悬停是同一族问题。
//
// 判据取行左缘那 1px 描边的蓝度（B−R）：常态是白 28%（蓝度≈0），悬停是
// #0ea5e9 34%（蓝度≈60+），插值期间必须严格落在两者之间；移入**当帧**
// （elapsed = 0）仍是常态值（旧写法在这里就跳满）。
func TestPlaylistRowHoverTransition(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = DrawerPlaylist
	a.PlaylistTab = ListUser
	a.PlayingPlaylist = ListUser
	a.Index = 0 // 第一行是当前项，测第二行（纯色底 → 纯色底，可插值）
	a.Playlist = []PlayItem{
		{ID: "a", Part: "第一集", Title: "T0"},
		{ID: "b", Part: "第二集", Title: "T1"},
	}
	tst := shellTester(t, a)
	part, ok := tst.Find("第二集")
	if !ok {
		t.Fatal("找不到分集名「第二集」")
	}
	// 行盒左缘 = 文字列左缘 −（1px 边框 + p-2 8 + 封面 64 + gap-2 8）。
	rowX := int(part.X) - 81
	blueness := func() float32 {
		img := tst.Image()
		var sum float32
		var n int
		for y := int(part.Y) + 2; y < int(part.Y+part.H)-2; y++ {
			for x := rowX; x <= rowX+1; x++ {
				i := img.PixOffset(x, y)
				sum += float32(int(img.Pix[i+2]) - int(img.Pix[i]))
				n++
			}
		}
		return sum / float32(n)
	}

	rest := blueness()
	tst.Move(part.X+part.W/2, part.Y+part.H/2)
	enter := blueness() // 移入当帧（elapsed = 0）
	tst.Advance(60 * time.Millisecond)
	mid := blueness()
	tst.Advance(200 * time.Millisecond)
	hover := blueness()
	t.Logf("左缘描边蓝度 常态=%.1f 移入当帧=%.1f 60ms=%.1f 200ms=%.1f",
		rest, enter, mid, hover)

	if rest > 12 {
		t.Fatalf("常态描边不够中性（蓝度=%.1f）—— 采样列 %d 可能没落在行左缘", rest, rowX)
	}
	if hover-rest < 20 {
		t.Fatalf("悬停后描边没变蓝（常态 %.1f → 悬停 %.1f）", rest, hover)
	}
	if diff := enter - rest; diff > 1 || diff < -1 {
		t.Errorf("移入当帧描边就跳到了 %.1f（常态 %.1f）—— 没有 160ms 过渡", enter, rest)
	}
	if !(mid > rest+2 && mid < hover-2) {
		t.Errorf("60ms 中点 %.1f 不在常态 %.1f 与悬停 %.1f 之间 —— 过渡没走完一半", mid, rest, hover)
	}
	// 回落同样走过渡。
	tst.Move(part.X+part.W/2, part.Y-120)
	leave := blueness()
	tst.Advance(300 * time.Millisecond)
	back := blueness()
	t.Logf("移出当帧=%.1f 300ms 后=%.1f", leave, back)
	if d := leave - hover; d > 2 || d < -2 {
		t.Errorf("移出当帧 %.1f 应仍接近悬停 %.1f（渐隐），差 %.1f", leave, hover, d)
	}
	if d := back - rest; d > 1 || d < -1 {
		t.Errorf("移出 300ms 后应回到常态 %.1f，得到 %.1f", rest, back)
	}
}

// TestPlaylistRowHoverAnimTokens 钉住过渡参数本身：原版 `.group` 的
// `transition: … 0.16s ease`（ease = cubic-bezier(.25,.1,.25,1)）。
func TestPlaylistRowHoverAnimTokens(t *testing.T) {
	if playlistRowHoverAnim.Duration != 160*time.Millisecond {
		t.Errorf("悬停过渡时长 = %v，应为 160ms（原版 .group 的 transition）",
			playlistRowHoverAnim.Duration)
	}
	if playlistRowHoverAnim.Ease == nil {
		t.Error("悬停过渡没有缓动曲线（原版是 ease）")
	}
	rest := playlistRowPaint(false, false, false, false)
	hover := playlistRowPaint(false, true, false, false)
	mid := playlistRowPaintAt(rest, hover, 0.5)
	if mid.From != rest.From.Mix(hover.From, 0.5) {
		t.Errorf("k=0.5 的底色 = %v，应为两态中点 %v", mid.From, rest.From.Mix(hover.From, 0.5))
	}
	if mid.Border != rest.Border.Mix(hover.Border, 0.5) {
		t.Errorf("k=0.5 的描边 = %v，应为两态中点 %v", mid.Border, rest.Border.Mix(hover.Border, 0.5))
	}
	if mid.ShadowY != 5 || mid.ShadowBlur != 15 {
		t.Errorf("k=0.5 的投影 y/blur = %v/%v，应为 5/15（2→8、8→22 的中点）",
			mid.ShadowY, mid.ShadowBlur)
	}
	if got := playlistRowPaintAt(rest, hover, 0); got.From != rest.From || got.Ring {
		t.Errorf("k=0 应逐值等于常态，得到 %v（ring=%v）", got.From, got.Ring)
	}
	if got := playlistRowPaintAt(rest, hover, 1); got.From != hover.From || !got.Ring {
		t.Errorf("k=1 应逐值等于悬停，得到 %v（ring=%v）", got.From, got.Ring)
	}
}
