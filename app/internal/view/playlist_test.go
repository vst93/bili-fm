package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestPlaylistRowPaintStates 钉住播放列表记录行的配色/字重，逐条对应原版
// globals.css 里 `[data-slot="wrapper"]` 作用域下的 .group / .group:hover /
// .playlist-current / .playlist-current:hover / [class*="bg-blue-50"] /
// :active 六组规则。这些是用户可感知的差异：原生版此前常态没有玻璃卡底、
// 悬停只有白 .38、文案色用了 Tailwind 原值（未被绘板的覆写），当前项也只
// 有两段渐变。
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
		if p.AccentW != 4 || p.Accent != ui.Hex("#0369a1").Alpha(0.78) {
			t.Errorf("强调条 = %v/%v, want 4px 3,105,161 .78", p.AccentW, p.Accent)
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
		if !p.Gradient || p.From != ui.Hex("#0ea5e9").Alpha(0.36) || p.To != ui.Hex(white).Alpha(0.62) {
			t.Errorf("当前项悬停渐变 = %v→%v (gradient=%v), want 天蓝 .36→白 .62",
				p.From, p.To, p.Gradient)
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
		if p.AccentW != 4 || p.Accent != ui.Hex("#0369a1").Alpha(0.68) {
			t.Errorf("拖拽强调条 = %v/%v, want 4px 3,105,161 .68", p.AccentW, p.Accent)
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
		if cur.AccentW != 4 {
			t.Error("当前项按下仍应保留左缘强调条")
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
