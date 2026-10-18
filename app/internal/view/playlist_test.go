package view

import (
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
