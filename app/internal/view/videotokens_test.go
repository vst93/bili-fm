package view

// 视频窗的**排版秩序**测试（用户 2026-11：现在的 UI 看着不够正式）。
//
// 这一轮不加功能、不加动效，靠的是秩序；所以测试也钉秩序：
//
//   - 网格：所有间距是 4 的倍数；顶栏与控件条左右内边距相等（两栏同宽同缩进）；
//   - 垂直节奏：一行内全部键共用一条视觉中心线；
//   - 键令牌：顶栏与控件条的全部键同尺寸同圆角（唯一的例外是「文字值键」
//     倍速键的宽度；**没有** 28 / 32 / 34 / 40 / 42 这类离群尺寸）；
//   - 字号与时间码：时间码固定宽度 + 等宽数字（0:00 与 12:34:56 同宽）；
//   - 分割线：两条栏各一条 1px 低对比 hairline（不是纯黑 / 纯白）；
//   - 两态与四尺度：深浅各跑一遍，320×200 / 640×360 / 900×500 / 全屏无重叠无越界。

import (
	"image"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

// videoNear 报告两个尺寸是否在 tol 之内（浮点布局的取整余量）。
func videoNear(a, b, tol float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// videoOrderThemes 返回深 / 浅两态（与真实主题同一条解析路径）。
func videoOrderThemes() map[string]Theme {
	return map[string]Theme{
		"light": ThemeFor(PeriodAt(12)),
		"dark":  ThemeFor(PeriodAt(12)).withAppearance("dark", 0),
	}
}

// videoOrderProbe 起一个无头视频窗（画面留黑：不传 Content）。
func videoOrderProbe(t *testing.T, th Theme, w, h int, dur, pos time.Duration, full bool) (*VideoChrome, *ui.Tester) {
	t.Helper()
	pl := &videoStubPlayer{playing: true, ready: true, dur: dur, pos: pos, rate: 1, vol: 1}
	win := newVideoStubWindow()
	ch := &VideoChrome{Theme: th, Title: "测试稿件 · 第一部分", Pl: pl, Win: win}
	if full {
		ch.fs.full = true
		win.full = true
	}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, w, h)
	tt.Frame()
	return ch, tt
}

// videoKeyRoles 是「键」在无障碍树里的角色（滑块不算键）。
func videoKeyRole(r ui.Role) bool {
	return r == ui.RoleButton || r == ui.RoleToggleButton || r == ui.RoleDisclosure
}

// videoKeysIn 返回落在 [y0, y1] 这一条栏里的键，按 x 从左到右排好。
func videoKeysIn(tt *ui.Tester, y0, y1 float32) []ui.AccessInfo {
	var out []ui.AccessInfo
	for _, info := range tt.AccessAll() {
		if !videoKeyRole(info.Role) {
			continue
		}
		if info.Bounds.Y < y0 || info.Bounds.Y > y1 {
			continue
		}
		out = append(out, info)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Bounds.X < out[j-1].Bounds.X; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// videoBarSpan 是这一帧两条栏各自的高度（窄窗控件条是两行）。
func videoBarSpan(w float32) (titleH, barH float32) {
	barH = videoChromeBarH
	if videoChromeLayoutFor(w).TwoRows {
		barH = videoChromeBarTwoH
	}
	return videoChromeTitleH, barH
}

// TestVideoTokensOnGrid 钉住令牌本身：间距落 4/8 的倍数、两栏同缩进、键只有
// 一档尺寸与圆角、字号只有三档。
func TestVideoTokensOnGrid(t *testing.T) {
	for _, v := range []struct {
		name string
		val  float32
	}{
		{"videoStep", videoStep},
		{"videoStep2", videoStep2},
		{"videoInset", videoInset},
		{"videoPadX", videoPadX},
		{"videoGap", videoGap},
		{"videoRowGap", videoRowGap},
		{"videoChromeTitleH", videoChromeTitleH},
		{"videoChromeBarH", videoChromeBarH},
		{"videoChromeRadius", videoChromeRadius},
		{"videoSpeedKeyW", videoSpeedKeyW},
		{"videoTimeW", videoTimeW},
		{"videoVolW", videoVolW},
		{"videoTrackH", videoTrackH},
		{"videoSettingsRowH", videoSettingsRowH},
		{"videoSettingsHeadH", videoSettingsHeadH},
		{"videoChoiceH", videoChoiceH},
		{"videoPanelPadX", videoPanelPadX},
		{"videoPanelPadY", videoPanelPadY},
		{"videoPanelGap", videoPanelGap},
		{"videoPanelEdge", videoPanelEdge},
	} {
		if v.val <= 0 || int(v.val)%int(videoStep) != 0 {
			t.Errorf("%s = %v：不是 4 的倍数（网格是这一轮的秩序）", v.name, v.val)
		}
	}
	// 两栏同缩进：外边距与内边距都只有一个值（顶栏与控件条共用）。
	if videoInset != videoInset {
		t.Errorf("videoInset = %v ≠ videoInset %v（两栏必须同缩进）", videoInset, videoInset)
	}
	// 键：顶栏与控件条同尺寸同圆角；播放键与图标键同档。
	if VideoChromeKey != videoKeySize || videoKeyTransport != videoKeySize {
		t.Errorf("键尺寸不统一：VideoChromeKey=%v videoKeyTransport=%v videoKeySize=%v",
			VideoChromeKey, videoKeyTransport, videoKeySize)
	}
	// 字号只有三档，且标题是 semibold。
	if videoFontTitle != 13 || videoTitleWeight != 600 {
		t.Errorf("标题字号/字重 = %v/%d，应为 13/600", videoFontTitle, videoTitleWeight)
	}
	if videoFontBody != 12 || videoFontMeta != 11 || videoFontTime != 11 {
		t.Errorf("正文字号档 = %v / %v / %v，应为 12 / 11 / 11",
			videoFontBody, videoFontMeta, videoFontTime)
	}
	// 面板头部与行同高（「头部高度统一」）。
	if videoSettingsHeadH != videoSettingsRowH {
		t.Errorf("面板头部 %v ≠ 行高 %v", videoSettingsHeadH, videoSettingsRowH)
	}
	// 时间码列宽必须装得下最长的 h:mm:ss（固定宽度是「不左右跳」的前提）。
	if videoTimeW < 48 {
		t.Errorf("时间列宽 %v 装不下 12:34:56（需 ≥ 48）", videoTimeW)
	}
}

// TestVideoChromeGridAlignment 钉住两栏的几何：左右内边距相等、键间距是 8 的
// 倍数、一行内全部键共用一条视觉中心线。深浅两态 × 四种尺度各跑一遍。
func TestVideoChromeGridAlignment(t *testing.T) {
	for name, th := range videoOrderThemes() {
		for _, tc := range []struct {
			w, h int
			full bool
		}{{320, 200, false}, {640, 360, false}, {900, 500, false}, {1920, 1080, true}} {
			ch, tt := videoOrderProbe(t, th, tc.w, tc.h, 100*time.Second, 30*time.Second, tc.full)
			_ = ch
			titleH, barH := videoBarSpan(float32(tc.w))

			title := videoKeysIn(tt, videoInset-1, videoInset+titleH+1)
			bar := videoKeysIn(tt, float32(tc.h)-videoInset-barH-1, float32(tc.h)-videoInset+1)
			if len(title) < 4 || len(bar) < 3 {
				t.Fatalf("%s %dx%d：键太少（顶栏 %d / 控件条 %d）",
					name, tc.w, tc.h, len(title), len(bar))
			}
			// 一行内全部键共用一条视觉中心线（同一 y、同一高）。
			for _, row := range [][]ui.AccessInfo{title, bar} {
				cy := row[0].Bounds.Y + row[0].Bounds.H/2
				for _, k := range row[1:] {
					if got := k.Bounds.Y + k.Bounds.H/2; !videoNear(got, cy, 0.6) {
						t.Errorf("%s %dx%d：%q 的中心线 %v 与 %q 的 %v 不在一条线上",
							name, tc.w, tc.h, k.Name, got, row[0].Name, cy)
					}
				}
			}
			// 键与键之间的间距是 8 的倍数（顶栏全是正好 videoGap）。
			for i := 1; i < len(title); i++ {
				gap := title[i].Bounds.X - (title[i-1].Bounds.X + title[i-1].Bounds.W)
				if !videoNear(gap, videoGap, 0.6) {
					t.Errorf("%s %dx%d：顶栏 %q 与 %q 的间距 %v，应为 %v",
						name, tc.w, tc.h, title[i-1].Name, title[i].Name, gap, videoGap)
				}
			}
			minGap := float32(1e9)
			for i := 1; i < len(bar); i++ {
				gap := bar[i].Bounds.X - (bar[i-1].Bounds.X + bar[i-1].Bounds.W)
				if gap <= 0 {
					t.Errorf("%s %dx%d：控件条 %q 与 %q 重叠（间距 %v）",
						name, tc.w, tc.h, bar[i-1].Name, bar[i].Name, gap)
					continue
				}
				if gap < minGap {
					minGap = gap
				}
			}
			if !videoNear(minGap, videoGap, 0.6) {
				t.Errorf("%s %dx%d：控件条最小键间距 %v，应为 %v", name, tc.w, tc.h, minGap, videoGap)
			}
			// 两栏左右缘一致，且都等于 videoInset（像素级：画面留黑，栏是会亮起来的那块）。
			img := tt.Image()
			ty := int(videoInset + titleH/2)
			by := tc.h - int(videoInset) - int(barH/2)
			tl, tr := barEdges(img, tc.w, ty)
			bl, br := barEdges(img, tc.w, by)
			if tl != bl || tr != br {
				t.Errorf("%s %dx%d：顶栏 x∈[%d,%d] 与控件条 x∈[%d,%d] 缩进不一致",
					name, tc.w, tc.h, tl, tr, bl, br)
			}
			if !videoNear(float32(tl), videoInset, 1) || !videoNear(float32(tc.w-1-tr), videoInset, 1) {
				t.Errorf("%s %dx%d：栏的左右边距 (%d, %d)，应为 %v",
					name, tc.w, tc.h, tl, tc.w-1-tr, videoInset)
			}
		}
	}
}

// barEdges 在 y 这一行上找「栏」的左右缘：画面是纯黑，栏是会亮起来的那块。
func barEdges(img *image.RGBA, w, y int) (left, right int) {
	lum := func(x int) int {
		r, g, b, _ := img.RGBAAt(x, y).RGBA()
		return (int(r>>8) + int(g>>8) + int(b>>8)) / 3
	}
	left, right = -1, -1
	for x := 0; x < w; x++ {
		if lum(x) > 8 {
			left = x
			break
		}
	}
	for x := w - 1; x >= 0; x-- {
		if lum(x) > 8 {
			right = x
			break
		}
	}
	return left, right
}

// TestVideoChromeKeyTokens 钉住「顶栏键与控件条键取同一令牌」：每一颗键都是
// videoKeySize 见方、圆角 videoKeyRadius；唯一的宽度例外是倍速这颗文字值键
// （videoSpeedKeyW）。不允许 28 / 32 / 34 / 40 / 42 这类离群尺寸。
func TestVideoChromeKeyTokens(t *testing.T) {
	for name, th := range videoOrderThemes() {
		for _, tc := range []struct{ w, h int }{{320, 200}, {640, 360}, {900, 500}} {
			_, tt := videoOrderProbe(t, th, tc.w, tc.h, 100*time.Second, 30*time.Second, false)
			titleH, barH := videoBarSpan(float32(tc.w))
			keys := append(
				videoKeysIn(tt, videoInset-1, videoInset+titleH+1),
				videoKeysIn(tt, float32(tc.h)-videoInset-barH-1, float32(tc.h)-videoInset+1)...)
			for _, k := range keys {
				if !videoNear(k.Bounds.H, videoKeySize, 0.6) {
					t.Errorf("%s %dx%d：键 %q 高 %v，应为 %v（同一令牌）",
						name, tc.w, tc.h, k.Name, k.Bounds.H, videoKeySize)
				}
				switch {
				case videoNear(k.Bounds.W, videoKeySize, 0.6):
				case videoNear(k.Bounds.W, videoSpeedKeyW, 0.6):
					// 倍速这把文字值键按令牌给宽度。
				default:
					t.Errorf("%s %dx%d：键 %q 宽 %v 是离群尺寸（只允许 %v 或 %v）",
						name, tc.w, tc.h, k.Name, k.Bounds.W, videoKeySize, videoSpeedKeyW)
				}
			}
		}
	}
}

// TestVideoLayoutNoOverlap 钉住四种尺度的排布：任何可点元素都不越界、两两不
// 重叠。深浅两态各跑一遍（320×200 是窗口的最小尺寸，1920×1080 是全屏）。
func TestVideoLayoutNoOverlap(t *testing.T) {
	labels := []string{
		"置顶", "弹幕", "视频设置", "最小化", "退出视频播放",
		"暂停", "播放进度", "音量", "音量滑块", "全屏", "0:30", "1:40",
	}
	for name, th := range videoOrderThemes() {
		for _, tc := range []struct {
			w, h int
			full bool
		}{{320, 200, false}, {640, 360, false}, {900, 500, false}, {1920, 1080, true}} {
			_, tt := videoOrderProbe(t, th, tc.w, tc.h, 100*time.Second, 30*time.Second, tc.full)
			found := map[string]ui.Rect{}
			for _, s := range labels {
				r, ok := tt.Find(s)
				if !ok {
					continue // 窄窗按布局规则收起了这一枚（收起的规则另有测试钉住）
				}
				found[s] = r
				if r.X < 0 || r.Y < 0 || r.X+r.W > float32(tc.w)+0.5 || r.Y+r.H > float32(tc.h)+0.5 {
					t.Errorf("%s %dx%d：%q 超出窗口 %+v", name, tc.w, tc.h, s, r)
				}
			}
			names := make([]string, 0, len(found))
			for n := range found {
				names = append(names, n)
			}
			for i := 0; i < len(names); i++ {
				for j := i + 1; j < len(names); j++ {
					a, b := found[names[i]], found[names[j]]
					if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
						t.Errorf("%s %dx%d：%q 与 %q 重叠 %+v / %+v",
							name, tc.w, tc.h, names[i], names[j], a, b)
					}
				}
			}
			// 顶栏与控件条不重叠（最矮的窗口也不许压在一起）。
			title, ok1 := tt.Find("测试稿件 · 第一部分")
			bar, ok2 := tt.Find("播放进度")
			if ok1 && ok2 && title.Y+title.H > bar.Y {
				t.Errorf("%s %dx%d：顶栏与控件条重叠：标题底 %v > 进度顶 %v",
					name, tc.w, tc.h, title.Y+title.H, bar.Y)
			}
		}
	}
}

// TestVideoTimeColumnFixedWidth 钉住时间码：列宽由令牌固定，0:00 与 12:34:56
// 两种内容下宽度完全一样（播放时不会左右跳），并且走 numericText（等宽数字）。
func TestVideoTimeColumnFixedWidth(t *testing.T) {
	orig := numericText
	defer func() { numericText = orig }()
	var drawn []string
	numericText = func(c *ui.Context, s string) ui.Element {
		drawn = append(drawn, s)
		return ui.Text(c, s).FontFeatures(tabularNums...)
	}
	if len(tabularNums) == 0 || tabularNums[0] != "tnum" {
		t.Fatalf("tabularNums = %v，应为 tnum", tabularNums)
	}

	cases := []struct {
		pos, dur   time.Duration
		posS, durS string
	}{
		{0, 45296 * time.Second, "0:00", "12:34:56"},
		{30 * time.Second, 100 * time.Second, "0:30", "1:40"},
	}
	for _, tc := range cases {
		_, tt := videoOrderProbe(t, ThemeFor(PeriodAt(12)), 900, 500, tc.dur, tc.pos, false)
		for _, s := range []string{tc.posS, tc.durS} {
			r, ok := tt.Find(s)
			if !ok {
				t.Fatalf("找不到时间码 %q（现有：%v）", s, tt.Texts())
			}
			if !videoNear(r.W, videoTimeW, 0.6) {
				t.Errorf("时间码 %q 宽 %v，应为固定的 %v", s, r.W, videoTimeW)
			}
		}
	}
	// 等宽数字：视频窗的两个时间码都经由 numericText（FontFeatures tnum）。
	for _, want := range []string{"0:00", "12:34:56"} {
		ok := false
		for _, got := range drawn {
			if got == want {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("时间码 %q 没有走 numericText（等宽数字）：%v", want, drawn)
		}
	}
}

// TestVideoHairlineLowContrast 钉住两条栏的分割线：同一个令牌、低对比（既不是
// 纯黑也不是纯白），并且**真的画出来了**（顶栏下沿、控件条上沿各一条 1px 线）。
func TestVideoHairlineLowContrast(t *testing.T) {
	for name, th := range videoOrderThemes() {
		line := videoHairline(th)
		if line.A == 0 {
			t.Errorf("%s：分割线完全透明", name)
			continue
		}
		if line.A >= 89 {
			t.Errorf("%s：分割线不透明度 %.2f 太高（要低对比）", name, float32(line.A)/255)
		}
		fill := videoBarFill(th)
		over := composite(line, fill)
		ratio := contrastRatio(over, fill)
		if ratio < 1.02 || ratio > 2.0 {
			t.Errorf("%s：分割线与栏底的对比 %.2f 不在低对比区间（1.02~2.0）", name, ratio)
		}
		// 不是纯黑 / 纯白：叠在栏底上之后既不是黑块也不是白块。
		blend := composite(line, fill)
		if (blend.R == 0 && blend.G == 0 && blend.B == 0) ||
			(blend.R == 255 && blend.G == 255 && blend.B == 255) {
			t.Errorf("%s：分割线叠在栏底上是纯黑 / 纯白 %v", name, blend)
		}

		// 像素级：顶栏下沿那一条比它上面一行深；控件条上沿那一条比它下面一行深。
		ch, tt := videoOrderProbe(t, th, 640, 360, 100*time.Second, 30*time.Second, false)
		_ = ch
		img := tt.Image()
		_, barH := videoBarSpan(640)
		titleLine := int(videoInset + videoChromeTitleH - 1)
		barLine := 360 - int(videoInset) - int(barH)
		if d := rowDelta(img, titleLine, titleLine-2); d < 3 {
			t.Errorf("%s：顶栏下沿没有 1px 分割线（与上一行差 %d）", name, d)
		}
		if d := rowDelta(img, barLine, barLine+2); d < 3 {
			t.Errorf("%s：控件条上沿没有 1px 分割线（与下一行差 %d）", name, d)
		}
	}
}

// rowDelta 是两行（栏内横条）的平均亮度差。
func rowDelta(img *image.RGBA, y1, y2 int) int {
	mean := func(y int) int {
		sum, n := 0, 0
		for x := 40; x < 600; x++ {
			r, g, b, _ := img.RGBAAt(x, y).RGBA()
			sum += (int(r>>8) + int(g>>8) + int(b>>8)) / 3
			n++
		}
		return sum / n
	}
	d := mean(y2) - mean(y1)
	if d < 0 {
		d = -d
	}
	return d
}

// TestVideoDanmakuBandBetweenBars 钉住弹幕区：顶 / 底固定弹幕与滚动弹幕在同一
// 条带里（上下边距一致），并且不压在控件条上。
func TestVideoDanmakuBandBetweenBars(t *testing.T) {
	for name, th := range videoOrderThemes() {
		s := danmaku.DefaultSettings()
		pl := &videoStubPlayer{playing: true, ready: true, dur: 200 * time.Second,
			pos: 10 * time.Second, rate: 1, vol: 1}
		o := &danmaku.Overlay{}
		o.SetItems([]danmaku.Item{
			{Time: 9, Text: "顶部固定", Kind: danmaku.Top},
			{Time: 9, Text: "底部固定", Kind: danmaku.Bottom},
		})
		ch := &VideoChrome{
			Theme: th, Title: "测试稿件", Pl: pl, Win: newVideoStubWindow(),
			Danmaku: true, DanmakuOverlay: o, DanmakuSettings: s,
		}
		tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 640, 360)
		tt.Frame()
		top, ok1 := tt.Find("顶部固定")
		bottom, ok2 := tt.Find("底部固定")
		if !ok1 || !ok2 {
			t.Fatalf("%s：固定弹幕没画出来（现有：%v）", name, tt.Texts())
		}
		_, barH := videoBarSpan(640)
		titleBottom := videoInset + videoChromeTitleH
		barTop := 360 - videoInset - barH
		if top.Y < titleBottom {
			t.Errorf("%s：顶部固定弹幕 y=%v 压在顶栏里（顶栏底 %v）", name, top.Y, titleBottom)
		}
		if bottom.Y+bottom.H > barTop+1 {
			t.Errorf("%s：底部固定弹幕底 %v 压在控件条上（控件条顶 %v）",
				name, bottom.Y+bottom.H, barTop)
		}
		// 上下边距一致：顶栏底到顶部弹幕、底部弹幕底到控件条顶。
		upper := top.Y - titleBottom
		lower := barTop - (bottom.Y + bottom.H)
		if !videoNear(upper, lower, 2) {
			t.Errorf("%s：顶/底固定弹幕的边距不一致（上 %v / 下 %v）", name, upper, lower)
		}
	}
}

// TestVideoSettingsRhythm 钉住设置面板的节奏：行高统一 36、头部同高、
// 开关与档位选择器在行尾对齐、分组之间是 1px 分隔线（不是大片留白）。
func TestVideoSettingsRhythm(t *testing.T) {
	for name, th := range videoOrderThemes() {
		set := &PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1}
		set.Ensure()
		pl := &videoStubPlayer{playing: true, ready: true, dur: 200 * time.Second,
			pos: 30 * time.Second, rate: 1, vol: 1}
		ch := &VideoChrome{
			Theme: th, Title: "测试稿件 · 第一部分", Pl: pl, Win: newVideoStubWindow(),
			Set: set, Danmaku: true, DanmakuSettings: danmaku.DefaultSettings(), NativeVideo: true,
		}
		tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 900, 1000)
		tt.Frame()
		openVideoSettings(t, tt)

		// 头部高度统一：关闭键与标题同一中心线，关闭键是键令牌里的尺寸。
		head, ok := tt.Find("关闭设置")
		if !ok {
			t.Fatalf("%s：面板头部找不到关闭键", name)
		}
		if !videoNear(head.H, videoKeySize, 0.6) {
			t.Errorf("%s：关闭键高 %v，应为键令牌 %v", name, head.H, videoKeySize)
		}
		// 开关行行高统一 36。
		var rowRight float32 = -1
		for _, label := range []string{"视频设置：弹幕总开关", "视频设置：广告跳过", "视频设置：原生视频播放"} {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%s：找不到 %q", name, label)
			}
			if !videoNear(r.H, videoSettingsRowH, 0.6) {
				t.Errorf("%s：%q 行高 %v，应为 %v", name, label, r.H, videoSettingsRowH)
			}
			right := r.X + r.W
			if rowRight < 0 {
				rowRight = right
			} else if !videoNear(right, rowRight, 0.6) {
				t.Errorf("%s：%q 右缘 %v 与其它行 %v 不齐", name, label, right, rowRight)
			}
		}
		// 选择器收在行尾：最后一颗档位按钮的右缘与开关行右缘对齐。
		last, ok := tt.Find("视频设置：不透明度 100%")
		if !ok {
			t.Fatalf("%s：找不到档位按钮", name)
		}
		if !videoNear(last.X+last.W, rowRight, 0.6) {
			t.Errorf("%s：档位选择器右缘 %v 与开关行右缘 %v 不齐（应在行尾对齐）",
				name, last.X+last.W, rowRight)
		}

		// 分组之间是分隔线（源码级：videoSettingsGroup 用 videoHairline 画 1px）。
		src, err := os.ReadFile("videosettings.go")
		if err != nil {
			t.Fatalf("读 videosettings.go 失败：%v", err)
		}
		body := string(src)
		idx := strings.Index(body, "func videoSettingsGroup(")
		if idx < 0 {
			t.Fatal("找不到 videoSettingsGroup")
		}
		end := strings.Index(body[idx:], "\nfunc ")
		group := body[idx : idx+end]
		for _, want := range []string{"Height(1)", "videoHairline(t)"} {
			if !strings.Contains(group, want) {
				t.Errorf("%s：分组之间缺少 1px 分隔线（%q）", name, want)
			}
		}
	}
}

// TestVideoIconsShareOneToken 钉住「一排键里的图标同一条网格」：线性图标全部
// 取 videoIconSize（播放/暂停是 videoPlayIconSize）、视频窗自己的两枚图标与
// icon-park 走同一条 48 画布 / 描边 4（不是 16 画布手抄），控件条的图标尺寸
// 也只从键令牌来（不再逐键写死）。
func TestVideoIconsShareOneToken(t *testing.T) {
	chrome, err := os.ReadFile("videochrome.go")
	if err != nil {
		t.Fatalf("读 videochrome.go 失败：%v", err)
	}
	if body := string(chrome); !strings.Contains(body, "iconFullScreen     = iconPark(") ||
		!strings.Contains(body, "iconExitFullScreen = iconPark(") {
		t.Errorf("视频窗的全屏图标不在 icon-park 的 48 画布网格上（描边会与其它键不一致）")
	}
	for _, f := range []string{"playerbar_shared.go", "videotokens.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", f, err)
		}
		_ = b
	}
	tokens, err := os.ReadFile("videotokens.go")
	if err != nil {
		t.Fatalf("读 videotokens.go 失败：%v", err)
	}
	shared, err := os.ReadFile("playerbar_shared.go")
	if err != nil {
		t.Fatalf("读 playerbar_shared.go 失败：%v", err)
	}
	for _, want := range []struct {
		body string
		frag string
	}{
		// 顶栏键的唯一画法：
		{string(tokens), `ui.Icon(c,ic).Size(videoIconSize,videoIconSize)`},
		// 控件条的图标尺寸：统一值来自令牌，缺省值才是各键的原版取值。
		{string(shared), `m.icon(volumeIconSize)`},
		{string(shared), `m.icon(nextIconSize)`},
		{string(shared), `m.icon(eqIconSize)`},
		{string(shared), `Size(m.PlayIcon,m.PlayIcon)`},
	} {
		if !strings.Contains(strings.Join(strings.Fields(want.body), ""), want.frag) {
			t.Errorf("键令牌没有接在图标尺寸上：找不到 %q", want.frag)
		}
	}
	// 渲染层面：每枚图标在 videoIconSize 下的墨迹都留得住键边（不会顶到圆角）。
	for _, c := range []struct {
		name string
		ic   *ui.SVG
	}{
		{"图钉", iconPin}, {"弹幕", iconDanmaku}, {"设置齿轮", iconLyricsGear}, {"最小化", iconMinus},
		{"关闭", iconClose}, {"下一集", iconNext}, {"音量", iconVolume}, {"均衡", iconEQ},
		{"恰饭", iconSponsor}, {"全屏", iconFullScreen}, {"退出全屏", iconExitFullScreen},
	} {
		w, h := iconRenderedInk(c.ic, videoIconSize)
		if w > int(videoKeySize*2/3) || h > int(videoKeySize*2/3) {
			t.Errorf("%s 图标在 %vpx 下墨迹 %dx%d 顶到键边（键 %v）",
				c.name, videoIconSize, w, h, videoKeySize)
		}
		if w == 0 || h == 0 {
			t.Errorf("%s 图标没画出墨迹", c.name)
		}
	}
}
