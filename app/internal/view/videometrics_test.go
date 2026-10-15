package view

import (
	"testing"
)

// 本轮（videoInfo 右栏盒高 / 标题行高按原版钉住）的测试。
//
// 原版 globals.css 对右栏这两处显式给了盒高与内边距，mygo 不显式给就会退回
// 框架默认，把整列撑高：
//
//	#video-info       { height: 366px; padding: 2px 0 2px 3px }
//	#video-title h2   { min-height: 52px; max-height: 58px; line-height: 1.18 }
//
// 简介块（#video-desc）的 min / max 高已由 videodesc_test.go 单独钉住。

// TestVideoTitleBoxHeight 钉住标题的盒高：原版 `#video-title h2` 是
// `min-height: 52px; max-height: 58px; line-height: 1.18` —— 两行内容
// 2 × 22 × 1.18 ≈ 51.92 落在 min-height 52 上，所以**一行与两行盒高都是 52**
// （max-height 58 只是上限，2 行截断下永远够不到）。
//
// 此前原生版写成 `MinHeight(52).MaxHeight(58)`：框架把每行高夹在
// `max(指定行高, 字体 ascent+descent)` 之上（22px 中文的自然行盒 ≈ 33px/行），
// 两行内容 ≈ 66 > 58，于是盒高被 `MaxHeight(58)` 截在 58 —— 比原版高 6px，把
// 下方简介 / 胶囊 / 操作行 / 工具条整体压下 6px。现在改成固定 `Height(52)`
// （见 home.go 的 videoTitleBoxH）。
func TestVideoTitleBoxHeight(t *testing.T) {
	long := ""
	for i := 0; i < 6; i++ {
		long += "这是一个很长的视频标题用于测试两行截断"
	}
	a := descApp()
	a.Info.Title = long
	a.Track.Title = long
	tt := shellTesterSize(t, a, 1000, 700)
	r, ok := tt.Find(long)
	if !ok {
		t.Fatal("找不到长标题")
	}
	t.Logf("长标题 rect = %+v", r)
	if r.H != 52 {
		t.Errorf("两行标题盒高 %v，want 52（#video-title h2 min-height 52；两行 1.18 行高正好落在上面）", r.H)
	}

	b := descApp()
	b.Info.Title = "短标题"
	b.Track.Title = "短标题"
	t2 := shellTesterSize(t, b, 1000, 700)
	r2, ok := t2.Find("短标题")
	if !ok {
		t.Fatal("找不到短标题")
	}
	t.Logf("短标题 rect = %+v", r2)
	if r2.H != 52 {
		t.Errorf("单行标题盒高 %v，want 52（#video-title h2 min-height）", r2.H)
	}
}

// TestVideoTitleBoxHeightWired 证明固定盒高真的接上了布局：把 videoTitleBoxH
// 换成 58（此前错误口径），两行标题盒高应随之变 58，下方的工具条也跟着下移 6px。
// 这样即使以后有人把 Height(videoTitleBoxH) 删掉、退回按内容撑高，这条也会红。
func TestVideoTitleBoxHeightWired(t *testing.T) {
	long := ""
	for i := 0; i < 6; i++ {
		long += "这是一个很长的视频标题用于测试两行截断"
	}
	measure := func() (titleH, dockY float32) {
		a := descApp()
		a.Info.Title = long
		a.Track.Title = long
		tt := shellTesterSize(t, a, 1000, 700)
		r, ok := tt.Find(long)
		if !ok {
			t.Fatal("找不到长标题")
		}
		d, ok := tt.Find("搜索结果")
		if !ok {
			t.Fatal("找不到工具条")
		}
		return r.H, d.Y
	}
	if videoTitleBoxH != 52 {
		t.Fatalf("videoTitleBoxH = %v，应为原版 #video-title h2 的 min-height 52", videoTitleBoxH)
	}
	withFix, dockFix := measure()
	old := videoTitleBoxH
	videoTitleBoxH = 58
	defer func() { videoTitleBoxH = old }()
	without, dockWithout := measure()
	t.Logf("两行标题盒高：固定 52 → %v（工具条 %v），改回 58 → %v（工具条 %v）",
		withFix, dockFix, without, dockWithout)
	if withFix != 52 {
		t.Errorf("固定盒高未生效：两行标题 %v，want 52", withFix)
	}
	if without != 58 {
		t.Errorf("换上 58 后盒高 %v，want 58（说明盒高确实由 videoTitleBoxH 驱动）", without)
	}
	if d := dockWithout - dockFix; d != 6 {
		t.Errorf("盒高差应让工具条下移 6px，实测 %v", d)
	}
}

// TestVideoInfoPadShiftsContent 钉住右栏内边距：原版 `#video-info` 的
// `padding: 2px 0 2px 3px`。把 padding 归零，内容应整体上移 2px、左移 3px。
func TestVideoInfoPadShiftsContent(t *testing.T) {
	want := [4]float32{2, 0, 2, 3}
	if videoInfoPad != want {
		t.Fatalf("videoInfoPad = %v，应为原版 #video-info 的 padding 2px 0 2px 3px", videoInfoPad)
	}
	measure := func() (x, y float32) {
		tt := shellTesterSize(t, descApp(), 1000, 700)
		r, ok := tt.Find("UP 主")
		if !ok {
			t.Fatal("找不到「UP 主」小标")
		}
		return r.X, r.Y
	}
	withPadX, withPadY := measure()
	old := videoInfoPad
	videoInfoPad = [4]float32{}
	defer func() { videoInfoPad = old }()
	noPadX, noPadY := measure()
	t.Logf("「UP 主」位置：有 padding (%v,%v)，归零 (%v,%v)", withPadX, withPadY, noPadX, noPadY)
	if d := withPadX - noPadX; d != 3 {
		t.Errorf("内边距左边距差 %v，want 3", d)
	}
	if d := withPadY - noPadY; d != 2 {
		t.Errorf("内边距上边距差 %v，want 2", d)
	}
}

// TestVideoTitleBoxHeightNarrow 钉住窄窗（@media ≤680px）的标题盒高：原版把
// `#video-title h2` 的 `min-height` 放开成 auto，`max-height: 58px` 与
// line-height 1.18 仍在 —— 盒高随折行数，单行 1.18 × 22 ≈ 25.96、两行
// ≈ 51.92（都不是框架自然行盒的 33 / 66）。
//
// 此前原生版窄窗用 `ui.Text + MaxHeight(52)`：两行被框架按 1.5em/行量到 66
// 再截到 52（比原版高 0.08px，可忽略），但**单行是 33**（自然行盒压不下去），
// 比原版高 7px，把简介 / 胶囊 / 操作行整体顶下去一截。本轮改自绘逐行落点
// （与宽窗同路），盒高 = 折行数 × infoTitlePitch，两种折行数都对上原版。
func TestVideoTitleBoxHeightNarrow(t *testing.T) {
	long := ""
	for i := 0; i < 6; i++ {
		long += "这是一个很长的视频标题用于测试两行截断"
	}
	a := descApp()
	a.Info.Title = long
	a.Track.Title = long
	tt := shellTesterSize(t, a, 400, 900)
	r, ok := tt.Find(long)
	if !ok {
		t.Fatal("窄窗找不到长标题")
	}
	t.Logf("窄窗两行标题 rect = %+v（infoTitlePitch=%v）", r, infoTitlePitch)
	want := 2 * infoTitlePitch
	if d := abs32(r.H - want); d > 0.5 {
		t.Errorf("窄窗两行标题盒高 %v，want %v（2 × line-height 1.18 × 22）", r.H, want)
	}

	b := descApp()
	b.Info.Title = "短标题"
	b.Track.Title = "短标题"
	t2 := shellTesterSize(t, b, 400, 900)
	r2, ok := t2.Find("短标题")
	if !ok {
		t.Fatal("窄窗找不到短标题")
	}
	t.Logf("窄窗单行标题 rect = %+v（infoTitlePitch=%v）", r2, infoTitlePitch)
	if d := abs32(r2.H - infoTitlePitch); d > 0.5 {
		t.Errorf("窄窗单行标题盒高 %v，want %v（min-height auto ⇒ line-height 1.18 × 22；框架自然行盒 33 是在此之前的口径）", r2.H, infoTitlePitch)
	}
	if r2.H >= 52 {
		t.Errorf("窄窗单行标题盒高 %v，应随内容（min-height auto，小于 52）", r2.H)
	}
}

// TestVideoTitleBoxHeightNarrowWired 是换桩反证：窄窗标题盒高真的由
// `infoTitlePitch` 与折行数驱动 —— 把 infoTitlePitch 换回框架自然行盒（33），
// 单行盒高应随之变 33、两行变 66（旧 `MaxHeight` 口径是 52）。
func TestVideoTitleBoxHeightNarrowWired(t *testing.T) {
	long := ""
	for i := 0; i < 6; i++ {
		long += "这是一个很长的视频标题用于测试两行截断"
	}
	measure := func(title string) float32 {
		a := descApp()
		a.Info.Title = title
		a.Track.Title = title
		tt := shellTesterSize(t, a, 400, 900)
		r, ok := tt.Find(title)
		if !ok {
			t.Fatal("窄窗找不到标题")
		}
		return r.H
	}
	oneLine, twoLine := measure("短标题"), measure(long)
	old := infoTitlePitch
	infoTitlePitch = naturalLineBox(infoTitleSize)
	defer func() { infoTitlePitch = old }()
	oneNatural, twoNatural := measure("短标题"), measure(long)
	t.Logf("单行盒高：自绘 %v → 自然行盒桩 %v；两行：自绘 %v → %v", oneLine, oneNatural, twoLine, twoNatural)
	if d := abs32(oneNatural - naturalLineBox(infoTitleSize)); d > 0.5 {
		t.Errorf("换桩后单行盒高 %v，want 自然行盒 %v（说明盒高确实由 infoTitlePitch 驱动）", oneNatural, naturalLineBox(infoTitleSize))
	}
	if !(oneLine < oneNatural) {
		t.Errorf("自绘单行盒高 %v 没有比自然行盒 %v 矮，行距没修复", oneLine, oneNatural)
	}
	if abs32(twoLine-twoNatural) < 0.5 {
		t.Errorf("自绘两行盒高 %v 与自然行盒桩 %v 相同，盒高没随折行数变", twoLine, twoNatural)
	}
}

// TestVideoTitleTwoLineNotClipped 是像素反证：固定 Height(52) 不能让自绘的
// 第二行裁掉或重新居中。扫标题盒下方竖条里的「有墨迹的行」，应当看到
// 两行标题墨迹（原版行距 1.18 × 22 ≈ 26，第二行墨迹顶在盒顶约 29px 处，见
// multilinetext_test.go）与随后的简介首行，且第二行底与简介首行顶之间仍有
// 一条空隙（≥4px），不叠字。
func TestVideoTitleTwoLineNotClipped(t *testing.T) {
	title := "这是一个相当长的视频标题用来测试两行截断的效果另一个很长的视频标题"
	a := descApp()
	a.Info.Title = title
	a.Track.Title = title
	a.Info.Desc = "这是视频简介的第一行文字用来观察间距"
	tt := shellTesterSize(t, a, 800, 600)
	tr, ok := tt.Find(title)
	if !ok {
		t.Fatal("找不到长标题")
	}
	if tr.H != 52 {
		t.Fatalf("两行标题盒高 %v，want 52", tr.H)
	}
	img := tt.Image()
	x0, x1 := int(tr.X)-2, int(tr.X+tr.W)+2
	if x0 < 0 {
		x0 = 0
	}
	if x1 > 800 {
		x1 = 800
	}
	// 有墨迹的行：任一像素灰度 < 140。
	var runs [][2]int
	start := -1
	for y := int(tr.Y); y < int(tr.Y)+140; y++ {
		dark := false
		for x := x0; x < x1; x++ {
			if grayAt(img, x, y) < 140 {
				dark = true
				break
			}
		}
		if dark && start < 0 {
			start = y
		}
		if !dark && start >= 0 {
			runs = append(runs, [2]int{start, y - 1})
			start = -1
		}
	}
	t.Logf("标题盒 %v 下方有墨迹的行：%v", tr, runs)
	if len(runs) < 3 {
		t.Fatalf("墨迹行只有 %d 段（%v），看不清两行标题 + 简介首行", len(runs), runs)
	}
	line1, line2, desc := runs[0], runs[1], runs[2]
	// 第二行必须在第一行之下（证明没被裁掉 / 重新居中）。
	if line2[0] <= line1[1] {
		t.Errorf("第二行墨迹 %v 没在第一行 %v 之下", line2, line1)
	}
	if line2[0] < int(tr.Y)+29 {
		t.Errorf("第二行墨迹顶 %d 太靠上，不像两行", line2[0])
	}
	if gap := desc[0] - line2[1]; gap < 4 {
		t.Errorf("第二行标题底 %d 与简介首行顶 %d 只隔 %dpx，会叠字", line2[1], desc[0], gap)
	}
}
