package view

// 视频窗控件条 == 主窗播放栏（+ 一颗全屏键）的断言。
//
// 用户 2026-11：「视频和音频的播放栏没有完全统一，热力条这些也要同步。同时
// 现在的播放和全屏按钮鼠标悬浮时图标会闪烁……倍速、音量这些控制组件也要和
// 音频播放栏保持一致逻辑，不要平铺出来，现在新写的还有拖动的 bug，直接和原有
// 的保持一样逻辑就行了，也就是只多一个全屏按钮。」
//
// 这个文件钉住「同一份实现」的五个可观测面：
//
//  1. 控件集合：视频窗控件条的键（类型 + 顺序）== 主窗播放栏 + 全屏键；
//  2. 键的尺寸：每一颗都与主窗同名键同宽同高（同一套 grid 与图标档）；
//  3. 图标：只有一份定义（state.go 的 icon-park 集），控件条不自带 SVG；
//  4. 单层图标：面板上的键只画一层图标（不再叠玻璃那层白压印副本），
//     hover 只换材质 —— 悬浮时图标不再闪；
//  5. 弹幕热力：与主窗同一个 painter，矩形 = 进度条轨道矩形（宽/左/右 ≤1px）。
//
// 每一条都带反证：把视频窗改回自写控件 / 恢复两层图标 / 不绑进度条句柄，
// 对应断言就会变红（见各测试的「反证」注释）。

import (
	"image"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// videoSyncProbe 起一个 w×h 的无头视频窗（900×500 是完整一档：两枚时间 +
// 全部键）。返回的 ctx 在最后一帧构建时被记下（热力几何要用它量句柄）。
func videoSyncProbe(t *testing.T, ch *VideoChrome, w, h int) (*ui.Tester, **ui.Context) {
	t.Helper()
	var ctx *ui.Context
	tt := ui.NewTester(func(c *ui.Context) {
		ctx = c
		ch.View(c)
	}, w, h)
	tt.Frame()
	// 两帧：热力的轨道矩形读的是已提交那一帧的布局。
	tt.Frame()
	return tt, &ctx
}

// newSyncChrome 是一条播放中的视频窗（与 videoChromeProbe 同参数，但可注入
// 热力数据）。
func newSyncChrome(t *testing.T) *VideoChrome {
	t.Helper()
	return &VideoChrome{
		Theme: ThemeFor(PeriodAt(12)),
		Title: "测试稿件 · 第一部分",
		Pl: &videoStubPlayer{playing: true, ready: true,
			dur: 200 * time.Second, pos: 60 * time.Second, rate: 1, vol: 1},
		Win: newVideoStubWindow(),
	}
}

// keyName 返回一颗键在「控件集合」比较里用的身份。视频窗的下一集没有目标，
// label 因此不同 —— 它们仍是**同一颗键**（同一个 c.Key、同一列、同一种
// role），归一后再比。视频窗**没有**均衡键（视频的声音不经过本仓库的 DSP
// 管线），所以这里也不再需要均衡键的归一。
func keyName(info ui.AccessInfo) string {
	switch info.Name {
	case "播放", "暂停":
		// 播放键的身份与这一帧的播放状态无关（两窗可能一个在放一个在停）。
		return "播放键"
	case "视频窗没有换集：回主窗用播放列表":
		return "下一个视频"
	}
	return info.Name
}

func keyNames(keys []ui.AccessInfo) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = keyName(k)
	}
	return out
}

// TestVideoBarControlSetEqualsMainBar 钉住「视频窗控件条 == 主窗播放栏 +
// 全屏键 − 均衡键」：键的类型与顺序完全一致，多出来的是最后那颗全屏键，
// 少掉的是均衡键（视频窗的均衡键是死入口，用户 2026-11 裁决删掉）。
//
// 反证：把视频窗改回自写控件（少一颗、多一颗、换顺序），或把均衡键加回
// 视频窗，这条立刻变红。
func TestVideoBarControlSetEqualsMainBar(t *testing.T) {
	_, mainTT := playerBarProbe(t, nil)
	mainKeys := videoKeysIn(mainTT, float32(700-PlayerBarHeight), 700)
	if len(mainKeys) < 6 {
		t.Fatalf("主窗播放栏键太少：%v", keyNames(mainKeys))
	}

	ch := newSyncChrome(t)
	vidTT, _ := videoSyncProbe(t, ch, 900, 500)
	vidKeys := videoKeysIn(vidTT, videoBarYFor(900), 500)
	// 视频窗 = 主窗 − 均衡 + 全屏，数量与主窗相同。
	if len(vidKeys) != len(mainKeys) {
		t.Fatalf("视频窗控件条键数 %d，应为「主窗 %d − 均衡 + 全屏」= %d：%v",
			len(vidKeys), len(mainKeys), len(mainKeys), keyNames(vidKeys))
	}

	mainNames := keyNames(mainKeys)
	eqName := eqTip(false)
	var want []string
	eqs := 0
	for _, n := range mainNames {
		if n == eqName || n == eqTip(true) {
			eqs++
			continue
		}
		want = append(want, n)
	}
	if eqs != 1 {
		t.Fatalf("主窗播放栏应有且只有一颗均衡键，实际 %d 颗：%v", eqs, mainNames)
	}
	want = append(want, "全屏")
	vidNames := keyNames(vidKeys)
	if strings.Join(vidNames, " | ") != strings.Join(want, " | ") {
		t.Fatalf("视频窗控件条的键与「主窗 + 全屏 − 均衡」不一致：\n 视频 = %v\n 想要 = %v", vidNames, want)
	}
	// 视频窗里不许再出现均衡键（不是置灰，是没有）。
	for _, k := range vidKeys {
		if k.Name == eqTip(false) || k.Name == eqTip(true) || strings.Contains(k.Name, "音量均衡") {
			t.Errorf("视频窗不该有均衡键，出现了 %q", k.Name)
		}
	}
	// 全屏键是**唯一**多出来的那一颗，且排在最后。
	if last := vidKeys[len(vidKeys)-1]; last.Name != "全屏" {
		t.Errorf("最后一颗键应是全屏，得到 %q", last.Name)
	}
	for _, k := range vidKeys[:len(vidKeys)-1] {
		if k.Name == "全屏" || k.Name == "退出全屏" {
			t.Errorf("全屏键只能有一颗，这里出现了 %q", k.Name)
		}
	}
}

// TestVideoBarControlSizesMatchMainBar 钉住同一套 grid：视频窗的每一颗键都与
// 主窗同名键同宽同高（播放 42 正圆、工具键 32、均衡/恰饭 34、倍速 40、时间列
// 由共享令牌给）。上一轮视频窗把键统一收成 30×30，播放键因此比主窗小一圈，
// 下一集的图标也从 18 掉到 16 —— 用户说的「下一首的图标被改了」。
func TestVideoBarControlSizesMatchMainBar(t *testing.T) {
	_, mainTT := playerBarProbe(t, nil)
	mainKeys := videoKeysIn(mainTT, float32(700-PlayerBarHeight), 700)
	mainSize := map[string]ui.Rect{}
	for _, k := range mainKeys {
		mainSize[keyName(k)] = k.Bounds
	}

	ch := newSyncChrome(t)
	vidTT, _ := videoSyncProbe(t, ch, 900, 500)
	vidKeys := videoKeysIn(vidTT, videoBarYFor(900), 500)
	if len(vidKeys) == 0 {
		t.Fatal("视频窗控件条里没有键")
	}
	for _, k := range vidKeys {
		name := keyName(k)
		if name == "全屏" {
			// 全屏键是主窗没有的那一颗：尺寸仍是工具键那一档。
			if !videoNear(k.Bounds.W, playerBarKeyMetrics.Key, 0.6) ||
				!videoNear(k.Bounds.H, playerBarKeyMetrics.Key, 0.6) {
				t.Errorf("全屏键 %v×%v，应为工具键 %v×%v",
					k.Bounds.W, k.Bounds.H, playerBarKeyMetrics.Key, playerBarKeyMetrics.Key)
			}
			continue
		}
		want, ok := mainSize[name]
		if !ok {
			t.Fatalf("视频窗多出一颗主窗没有的键 %q", name)
		}
		if !videoNear(k.Bounds.W, want.W, 0.6) || !videoNear(k.Bounds.H, want.H, 0.6) {
			t.Errorf("键 %q 尺寸 %v×%v，主窗同名键是 %v×%v（同一套 grid）",
				name, k.Bounds.W, k.Bounds.H, want.W, want.H)
		}
	}
	// 时间码：视频窗与主窗同一列宽（共享令牌 56），不再是视频窗自己的 52。
	if playerBarKeyMetrics.TimeW != 56 || playerBarKeyMetrics.TimeSize != 11 {
		t.Errorf("时间列令牌 = %v / %v，应为 56 / 11", playerBarKeyMetrics.TimeW, playerBarKeyMetrics.TimeSize)
	}
}

// videoBarYFor 用宽度推控件条高度（与 videoBarSpan 同一份规则），返回控件条
// 的顶边；底部用窗口高（本文件全部视频 probe 都是 500 高）。
func videoBarYFor(w int) float32 {
	_, barH := videoBarSpan(float32(w))
	return float32(500) - videoInset - barH
}

// TestVideoBarIconsFromMainSet 钉住图标同源：控件条不再自带一份 SVG，两枚
// 全屏图标定义在 state.go 的 icon-park 图标集里；共享代码用的就是主窗那几个
// 图标尺寸常量（不留 icon-park 48/4 之外的第二套尺寸）。
//
// 反证：在 videochrome.go 里再写一枚自绘 SVG / 把控件条图标尺寸改回统一 16，
// 这里变红。
func TestVideoBarIconsFromMainSet(t *testing.T) {
	chrome, err := os.ReadFile("videochrome.go")
	if err != nil {
		t.Fatalf("读 videochrome.go 失败：%v", err)
	}
	body := string(chrome)
	for _, bad := range []string{"iconPark(", "<path d=", "videoIconSize", "videoPlayIconSize"} {
		if strings.Contains(body, bad) {
			t.Errorf("videochrome.go 里还有控件条图标的第二套定义（%q）—— 图标应只来自主窗那套", bad)
		}
	}
	if strings.Contains(body, "videoPanelKeys") || strings.Contains(body, "volumeSlider(") {
		t.Errorf("videochrome.go 里还留着自写的键令牌 / 内联音量条")
	}

	state, err := os.ReadFile("state.go")
	if err != nil {
		t.Fatalf("读 state.go 失败：%v", err)
	}
	for _, want := range []string{
		"iconFullScreen     = iconPark(",
		"iconExitFullScreen = iconPark(",
	} {
		if !strings.Contains(string(state), want) {
			t.Errorf("全屏图标不在 state.go 的 icon-park 图标集里：%q", want)
		}
	}

	shared, err := os.ReadFile("playerbar_shared.go")
	if err != nil {
		t.Fatalf("读 playerbar_shared.go 失败：%v", err)
	}
	sharedBody := string(shared)
	flat := strings.Join(strings.Fields(sharedBody), "")
	for _, want := range []string{
		"Size(playIconSize,playIconSize)",
		"Size(nextIconSize,nextIconSize)",
		"Size(volumeIconSize,volumeIconSize)",
		"Size(eqIconSize,eqIconSize)",
		"Size(fullScreenIconSize,fullScreenIconSize)",
		".Size(m.Play,m.Play)",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("共享播放栏没有用主窗的图标尺寸常量：找不到 %q", want)
		}
	}
	for _, bad := range []string{"Metrics*", "m.Icon(", "m.PlayIcon"} {
		if strings.Contains(strings.ReplaceAll(sharedBody, " ", ""), bad) {
			t.Errorf("共享播放栏还留着第二套键令牌（%q）", bad)
		}
	}

	// 渲染层面：同一颗键的图标墨迹在两窗同尺寸（同一图标源 + 同一视觉尺寸）。
	iconMaskGeometry(t)
}

// iconMaskGeometry 量出视频窗控件条那几枚图标的墨迹轮廓，与**主窗图标尺寸
// 常量**渲染出来的同一枚图标比对：包围盒一致、像素数在容差内。上一轮视频窗
// 把图标统一成 16（播放 18），这条会因轮廓变小而变红。
func iconMaskGeometry(t *testing.T) {
	t.Helper()
	ch := newSyncChrome(t)
	vidTT, _ := videoSyncProbe(t, ch, 900, 500)
	keys := videoKeysIn(vidTT, videoBarYFor(900), 500)
	want := map[string]struct {
		ic   *ui.SVG
		size float32
	}{
		"播放键":   {iconPause, playIconSize},
		"下一个视频": {iconNext, nextIconSize},
		"音量":    {iconVolume, volumeIconSize},
	}
	for _, k := range keys {
		w, ok := want[keyName(k)]
		if !ok {
			continue
		}
		// 只取键中央的一块（图标 + 2px 余量）：键的边缘有描边 / 聚光边 /
		// 内外阴影，它们会被覆盖率的判据误当成墨。
		side := w.size + 4
		crop := ui.Rect{
			X: k.Bounds.X + (k.Bounds.W-side)/2,
			Y: k.Bounds.Y + (k.Bounds.H-side)/2,
			W: side, H: side,
		}
		got := iconSilhouette(vidTT.Image(), crop)
		ref := refIconSilhouette(t, w.ic, w.size)
		gx0, gy0, gx1, gy1 := inkBoundsIn(got)
		rx0, ry0, rx1, ry1 := inkBoundsIn(ref)
		// 键里的图标常常落在半像素上（播放键还有 .5 的 translate），墨迹
		// 包围盒允许 1px 差 —— 差一档尺寸（16 vs 24）会差 3px 以上。
		gw, gh, rw, rh := gx1-gx0+1, gy1-gy0+1, rx1-rx0+1, ry1-ry0+1
		if d := gw - rw; d < -1 || d > 1 {
			t.Errorf("%q 的图标墨迹宽 %d，主窗 %vpx 档的同一枚图标是 %d",
				keyName(k), gw, w.size, rw)
		}
		if d := gh - rh; d < -1 || d > 1 {
			t.Errorf("%q 的图标墨迹高 %d，主窗 %vpx 档的同一枚图标是 %d",
				keyName(k), gh, w.size, rh)
		}
	}
}

// refIconSilhouette 在视频窗 Panel 的底色上渲染一枚图标（主窗的图标尺寸常量），
// 作为「同一图标源 + 同一视觉尺寸」的参照物。
func refIconSilhouette(t *testing.T, ic *ui.SVG, size float32) silhouette {
	t.Helper()
	th := ThemeFor(PeriodAt(12))
	under := ui.RGB(0, 0, 0) // 控件条压在画面上：画面留黑
	panel := composite(videoBarFill(th), under)
	box := int(size) + 4
	var boxRect ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(th.uiTheme())
		ui.Box(c).Fill().Background(panel).Children(func() {
			boxRect = ui.Rect{X: 0, Y: 0, W: float32(box), H: float32(box)}
			ui.Icon(c, ic).Size(size, size).TextColor(th.Muted)
		})
	}, box, box)
	tt.Frame()
	return iconSilhouette(tt.Image(), boxRect)
}

// iconSilhouette 把一颗键里的图标轮廓取成布尔掩码：以键内左上角为背景色、
// 键内最暗像素为墨色，把每个像素的「覆盖率」> 0.5 记为有墨。
func iconSilhouette(img *image.RGBA, r ui.Rect) silhouette {
	out := silhouette{w: int(r.W), h: int(r.H), on: make([]bool, int(r.W)*int(r.H))}
	if r.W <= 0 || r.H <= 0 {
		return out
	}
	c := img.RGBAAt(int(r.X)+2, int(r.Y)+2)
	bg := lum01(ui.RGB(c.R, c.G, c.B))
	ink := bg
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			cc := img.RGBAAt(x, y)
			if v := lum01(ui.RGB(cc.R, cc.G, cc.B)); v < ink {
				ink = v
			}
		}
	}
	if bg == ink {
		return out
	}
	for y := 0; y < int(r.H); y++ {
		for x := 0; x < int(r.W); x++ {
			cc := img.RGBAAt(int(r.X)+x, int(r.Y)+y)
			v := lum01(ui.RGB(cc.R, cc.G, cc.B))
			out.on[y*int(r.W)+x] = (bg-v)/(bg-ink) > 0.5
		}
	}
	return out
}

// silhouette 是一颗键里图标的覆盖率轮廓（on 为 w×h 的布尔掩码）。
type silhouette struct {
	w, h int
	on   []bool
}

func lum01(c ui.Color) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
}

func inkBoundsIn(m silhouette) (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = 1<<30, 1<<30, -1, -1
	for i, on := range m.on {
		if !on {
			continue
		}
		x, y := i%m.w, i/m.w
		if x < x0 {
			x0 = x
		}
		if y < y0 {
			y0 = y
		}
		if x > x1 {
			x1 = x
		}
		if y > y1 {
			y1 = y
		}
	}
	return
}

// TestVideoBarSingleIconLayerOnHover 钉住「一颗键只画一层图标」：视频窗控件条
// 压在 Panel 上，不叠玻璃播放栏那层白色 drop-shadow 副本；hover 只换材质，
// 图标轮廓不动、也不逐帧重画（两帧一致）。
//
// 反证：把 bind 的 OnPanel 去掉（视频窗改回玻璃那套画法）→ 压印计数 > 0，
// 这条变红。
func TestVideoBarSingleIconLayerOnHover(t *testing.T) {
	orig := playerIconShadowFn
	defer func() { playerIconShadowFn = orig }()

	// —— 视频窗：一层都不许画。
	calls := 0
	playerIconShadowFn = func(b ui.Element, svg *ui.SVG, dx, dy, opacity float32) ui.Element {
		calls++
		return b
	}
	ch := newSyncChrome(t)
	_, _ = videoSyncProbe(t, ch, 900, 500)
	if calls != 0 {
		t.Errorf("视频窗控件条画了 %d 层图标压印（每颗键应只有一层图标）", calls)
	}

	// —— 反证：同一份共享代码，把 OnPanel 关掉就会画第二层（玻璃播放栏的行为）。
	vidTT2 := ui.NewTester(func(c *ui.Context) {
		b := ch.videoPlayerBind(c, ch.Theme, videoChromeLayoutFor(900), false)
		b.OnPanel = false
		ui.Row(c).Padding(0, 12).Gap(8).Children(func() {
			playerTransportItems(c, b)
			var v, s ui.Element
			playerSettingsItems(c, b, &v, &s)
		})
	}, 900, 120)
	calls = 0
	vidTT2.Frame()
	if calls == 0 {
		t.Error("反证失效：OnPanel=false 时也没有画第二层图标（判据抓不住两层叠绘）")
	}

	// —— hover 只换材质：轮廓不变、两帧一致。
	playerIconShadowFn = orig
	vidTT, _ := videoSyncProbe(t, ch, 900, 500)
	keys := videoKeysIn(vidTT, videoBarYFor(900), 500)
	var play, full ui.Rect
	for _, k := range keys {
		switch k.Name {
		case "暂停":
			play = k.Bounds
		case "全屏":
			full = k.Bounds
		}
	}
	if play.W == 0 || full.W == 0 {
		t.Fatalf("找不到播放 / 全屏键（现有：%v）", keyNames(keys))
	}
	rest := iconSilhouette(vidTT.Image(), play)
	vidTT.Move(play.X+play.W/2, play.Y+play.H/2)
	vidTT.Advance(300 * time.Millisecond)
	hov := iconSilhouette(vidTT.Image(), play)
	// hover 只换材质与墨色（原版 `transition: color`），字形几何必须不变：
	// 多画一层图标（并排两个）才会把包围盒撑宽。
	rx, ry, rw, rh := silhouetteBounds(rest)
	hx, hy, hw, hh := silhouetteBounds(hov)
	if absInt(rw-hw) > 1 || absInt(rh-hh) > 1 || absInt(rx-hx) > 1 || absInt(ry-hy) > 1 {
		t.Errorf("播放键 hover 时图标几何变了：静息 %d,%d %d×%d → 悬停 %d,%d %d×%d（只允许换材质与墨色）",
			rx, ry, rw, rh, hx, hy, hw, hh)
	}
	first := iconSilhouette(vidTT.Image(), play)
	img1 := vidTT.Image()
	vidTT.Advance(16 * time.Millisecond)
	img2 := vidTT.Image()
	if n := pixelDiff(img1, img2, play); n != 0 {
		t.Errorf("悬停稳定后连续两帧的播放键仍差 %d 个像素（图标在逐帧重画 / 抖动）", n)
	}
	second := iconSilhouette(vidTT.Image(), play)
	if silhouetteDiff(first, second) != 0 {
		t.Errorf("悬停稳定后图标轮廓仍在变（%d 个像素）", silhouetteDiff(first, second))
	}

	// 全屏键同理：hover 后轮廓不变、两帧一致。
	restFull := iconSilhouette(vidTT.Image(), full)
	vidTT.Move(full.X+full.W/2, full.Y+full.H/2)
	vidTT.Advance(300 * time.Millisecond)
	hovFull := iconSilhouette(vidTT.Image(), full)
	fx, fy, fw, fh := silhouetteBounds(restFull)
	ux, uy, uw, uh := silhouetteBounds(hovFull)
	if absInt(fw-uw) > 1 || absInt(fh-uh) > 1 || absInt(fx-ux) > 1 || absInt(fy-uy) > 1 {
		t.Errorf("全屏键 hover 时图标几何变了：静息 %d,%d %d×%d → 悬停 %d,%d %d×%d（只允许换材质）",
			fx, fy, fw, fh, ux, uy, uw, uh)
	}
	f1 := vidTT.Image()
	vidTT.Advance(16 * time.Millisecond)
	f2 := vidTT.Image()
	if n := pixelDiff(f1, f2, full); n != 0 {
		t.Errorf("悬停稳定后连续两帧的全屏键仍差 %d 个像素（图标在闪）", n)
	}
}

// silhouetteDiff 数两个轮廓不同的像素数。
// silhouetteBounds 是轮廓里墨迹的包围盒。用它判「图标层的字形几何有没有
// 变」比逐像素轮廓稳：图标颜色在 hover 时按原版 `transition: color` 过渡，
// 归一化亮度会把抗锯齿边缘翻过 0.5 的阈值（几十个像素），但字形本身就那么
// 大、就那么居中 —— 多画一层（并排两个图标）才会把包围盒撑宽。
func silhouetteBounds(s silhouette) (x, y, w, h int) {
	minX, minY, maxX, maxY := s.w, s.h, -1, -1
	for i, on := range s.on {
		if !on {
			continue
		}
		cx, cy := i%s.w, i/s.w
		if cx < minX {
			minX = cx
		}
		if cy < minY {
			minY = cy
		}
		if cx > maxX {
			maxX = cx
		}
		if cy > maxY {
			maxY = cy
		}
	}
	if maxX < 0 {
		return 0, 0, 0, 0
	}
	return minX, minY, maxX - minX + 1, maxY - minY + 1
}

func silhouetteDiff(a, b silhouette) int {
	if a.w != b.w || a.h != b.h {
		return 1 << 20
	}
	n := 0
	for i := range a.on {
		if a.on[i] != b.on[i] {
			n++
		}
	}
	return n
}

func pixelDiff(a, b *image.RGBA, r ui.Rect) int {
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := y*a.Stride + x*4
			if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
				n++
			}
		}
	}
	return n
}

// TestVideoBarHeatMatchesProgressTrack 钉住弹幕热力：数据与主窗同一份、绘制
// 同一个 painter、矩形 == 进度条**轨道**矩形（宽 / 左 / 右 ≤1px，320 窄窗
// 再验一次）。热力不许被拉成整条栏宽。
//
// 反证：不绑 progressHandle（热力只能退回整条栏宽 / 干脆不画）时这条变红。
func TestVideoBarHeatMatchesProgressTrack(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{900, 500}, {320, 200}} {
		ch := newSyncChrome(t)
		ch.HeatOn = true
		ch.DanmakuHeat = []HeatPoint{
			{At: 0, Value: 0.4}, {At: 50, Value: 1}, {At: 100, Value: 0.6},
			{At: 150, Value: 0.9}, {At: 200, Value: 0.3},
		}
		tt, ctxp := videoSyncProbe(t, ch, tc.w, tc.h)
		ctx := *ctxp
		bar := ch.progressHandle.Bounds(ctx)
		if bar.W <= 0 {
			t.Fatalf("%dx%d：进度条句柄没有绑上（Bounds 为空）—— 热力没有轨道可对齐", tc.w, tc.h)
		}
		track := rangeTrackRect(bar, timelineGeom)
		got := ch.heatTrack
		if got.W <= 0 {
			t.Fatalf("%dx%d：热力没有画出来（heatTrack 为空，轨道 %v）", tc.w, tc.h, track)
		}
		if d := absF(got.W - track.W); d > 1 {
			t.Errorf("%dx%d：热力宽 %.1f，进度条轨道宽 %.1f（差 %.1f > 1）", tc.w, tc.h, got.W, track.W, d)
		}
		if d := absF(got.X - track.X); d > 1 {
			t.Errorf("%dx%d：热力左边界 %.1f，轨道左边界 %.1f（差 %.1f > 1）", tc.w, tc.h, got.X, track.X, d)
		}
		if d := absF((got.X + got.W) - (track.X + track.W)); d > 1 {
			t.Errorf("%dx%d：热力右边界 %.1f，轨道右边界 %.1f（差 %.1f > 1）",
				tc.w, tc.h, got.X+got.W, track.X+track.W, d)
		}
		// 反证：轨道比整条栏窄（跟着轨道 != 跟着栏）。
		if track.W >= bar.W {
			t.Fatalf("%dx%d：轨道宽 %.1f 不小于滑块盒宽 %.1f，判据失效", tc.w, tc.h, track.W, bar.W)
		}
		if got.W > bar.W+1 {
			t.Errorf("%dx%d：热力宽 %.1f 超过滑块盒宽 %.1f（被拉成整条栏）", tc.w, tc.h, got.W, bar.W)
		}

		// 像素级：热力真的画在控件条上（不是只记了个矩形）。与「关掉热力」的
		// 同一窗口比，轨道那一段底部必须出现差异。
		off := newSyncChrome(t)
		offTT, _ := videoSyncProbe(t, off, tc.w, tc.h)
		diff := 0
		band := ui.Rect{X: got.X, Y: got.Y + got.H - 18, W: got.W, H: 16}
		onImg, offImg := tt.Image(), offTT.Image()
		for y := int(band.Y); y < int(band.Y+band.H); y++ {
			for x := int(band.X); x < int(band.X+band.W); x++ {
				if y < 0 || y >= onImg.Rect.Dy() || x < 0 || x >= onImg.Rect.Dx() {
					continue
				}
				i := y*onImg.Stride + x*4
				if onImg.Pix[i] != offImg.Pix[i] || onImg.Pix[i+1] != offImg.Pix[i+1] ||
					onImg.Pix[i+2] != offImg.Pix[i+2] {
					diff++
				}
			}
		}
		if diff < 50 {
			t.Errorf("%dx%d：开热力与关热力的像素只差 %d 个（热力没有画到控件条上）", tc.w, tc.h, diff)
		}
	}
}

// TestVideoBarVolumeIsPopover 钉住「弹出式而非平铺」：音量/倍速在视频窗与主窗
// 一样是点开弹层；任何宽度都不把音量条摊平在控件条里。
//
// 反证：把内联音量条加回来（900 宽就会出现第二个「音量滑块」），这条变红。
func TestVideoBarVolumeIsPopover(t *testing.T) {
	for _, w := range []int{900, 320} {
		ch := newSyncChrome(t)
		tt, _ := videoSyncProbe(t, ch, w, 500)
		if tt.HasText("音量滑块") {
			t.Fatalf("%d 宽：不点音量键就出现了「音量滑块」（控件条里摊平了音量条）", w)
		}
		if err := tt.Click("音量"); err != nil {
			t.Fatalf("%d 宽：点不到音量键：%v", w, err)
		}
		tt.Frame()
		if !ch.showVolume {
			t.Errorf("%d 宽：点音量键没有进入弹出态", w)
		}
		if !tt.HasText("音量滑块") {
			t.Errorf("%d 宽：音量弹层里没有滑杆（现有：%v）", w, tt.Texts())
		}
		// 同一个弹层里还有静音键 —— 与主窗那颗同一份实现。
		if !tt.HasText("静音") && !tt.HasText("取消静音") {
			t.Errorf("%d 宽：音量弹层里缺静音键", w)
		}
		// 倍速：点开是弹出态（7 个档位），不是平铺的行内数字。
		if err := tt.Click("音量"); err != nil { // 关掉音量弹层
			t.Fatalf("%d 宽：关闭音量弹层失败：%v", w, err)
		}
		tt.Frame()
		if err := tt.Click("1.0x"); err != nil {
			t.Fatalf("%d 宽：点不到倍速键：%v", w, err)
		}
		tt.Frame()
		if !ch.showSpeed || !tt.HasText("0.75x") || !tt.HasText("2.0x") {
			t.Errorf("%d 宽：倍速没有弹出档位（showSpeed=%v，现有：%v）", w, ch.showSpeed, tt.Texts())
		}
	}
}

// TestVideoBarDragUsesSharedSlider 钉住拖动走主窗那套 rangeslider：按下 → 移动
// → 松手，值连续单调、松手落在落点值、不回跳。
//
// 反证：视频窗再包一层自写拖动（松手恢复成播放位置 / 中间跳值）时，这条红。
func TestVideoBarDragUsesSharedSlider(t *testing.T) {
	ch := newSyncChrome(t)
	pl := ch.Pl.(*videoStubPlayer)
	pl.dur, pl.pos = 200*time.Second, 20*time.Second
	tt, _ := videoSyncProbe(t, ch, 900, 500)

	track, ok := tt.Find("播放进度")
	if !ok {
		t.Fatalf("找不到进度条（现有：%v）", tt.Texts())
	}
	x0 := track.X + timelineInset
	span := track.W - 2*timelineInset
	y := track.Y + track.H/2
	step := func(frac float32) float64 { return float64(frac) * pl.dur.Seconds() }

	tt.Press(x0+span*0.30, y)
	tt.Frame()
	prev := ch.seek.value
	if prev <= 0 {
		t.Fatalf("按下后滑块值仍是 0（没有接到共享滑块的输入路径）")
	}
	var moved []float64
	for _, frac := range []float32{0.45, 0.60, 0.75} {
		tt.Move(x0+span*frac, y)
		tt.Frame()
		got := ch.seek.value
		if got < prev-0.5 {
			t.Errorf("拖动过程中值回退：%.2f → %.2f（应单调向前）", prev, got)
		}
		if got <= prev {
			t.Errorf("拖动没有更新值：%.2f → %.2f", prev, got)
		}
		moved = append(moved, got)
		prev = got
	}
	landing := ch.seek.value
	tt.Release(x0+span*0.75, y)
	tt.Frame()
	last := pl.seeks[len(pl.seeks)-1].Seconds()
	if absF32(float32(last-landing)) > 1 {
		t.Errorf("松手 seek 到 %.2f，滑块停在 %.2f（应落在同一个值）", last, landing)
	}
	if absF32(float32(ch.seek.value-landing)) > 1 {
		t.Errorf("松手后滑块跳回 %.2f（落点 %.2f）", ch.seek.value, landing)
	}
	// 期望值：落点约等于 75% 的总时长（轨道内缩已由 x0/span 扣掉）。
	if want := step(0.75); absF32(float32(landing-want)) > float32(pl.dur.Seconds())*0.04 {
		t.Errorf("落点 %.1fs，期望约 %.1fs（0.75 × 时长）", landing, want)
	}
	// 松手后再走两帧：不回跳（跟随播放位置，而播放位置已经被 seek 到落点）。
	tt.Advance(50 * time.Millisecond)
	tt.Frame()
	if absF32(float32(ch.seek.value-landing)) > 1 {
		t.Errorf("松手两帧后滑块漂到 %.2f（落点 %.2f）—— 拖动后回跳", ch.seek.value, landing)
	}
	if len(moved) != 3 {
		t.Errorf("移动次数 = %d", len(moved))
	}

	// 音量：同一个弹层里的滑杆同样走共享滑块。先把音量放到 0.2，向右拖才有
	// 余量（默认是 1.0，往右拖不会变）。
	set := ch.settings()
	set.SetVolume(0.2)
	tt.Frame()
	before := set.Volume
	tt.Move(track.X+track.W/2, track.Y+track.H/2)
	tt.Frame()
	if err := tt.Click("音量"); err != nil {
		t.Fatalf("点不到音量键：%v", err)
	}
	tt.Frame()
	vol, ok := tt.Find("音量滑块")
	if !ok {
		t.Fatalf("音量弹层里没有滑杆（现有：%v）", tt.Texts())
	}
	vy := vol.Y + vol.H/2
	vx := vol.X + timelineInset
	vspan := vol.W - 2*timelineInset
	tt.Press(vx+vspan*0.2, vy)
	tt.Frame()
	pv := set.Volume
	for _, frac := range []float32{0.4, 0.6, 0.8} {
		tt.Move(vx+vspan*frac, vy)
		tt.Frame()
		if set.Volume < pv-0.001 {
			t.Errorf("音量拖动回退：%.3f → %.3f", pv, set.Volume)
		}
		if set.Volume <= pv {
			t.Errorf("音量拖动没有更新值：%.3f → %.3f", pv, set.Volume)
		}
		pv = set.Volume
	}
	land := set.Volume
	tt.Release(vx+vspan*0.8, vy)
	tt.Frame()
	if absF32(float32(set.Volume-land)) > 0.01 {
		t.Errorf("音量松手后跳到 %.3f（落点 %.3f）", set.Volume, land)
	}
	if land <= before {
		t.Errorf("音量拖动后没有变大：之前 %.3f，落点 %.3f", before, land)
	}
}

// absF32 是 float32 的绝对值（absF 是 float64 版，测试里两处都要用）。
func absF32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestSilhouetteBoundsCatchesTwoIcons 验证 silhouetteBounds 这把这个判据
// 真的能抓住「并排两个图标」：同样两个图标分开摆，包围盒必须明显宽于一个。
// 它是 TestVideoBarSingleIconLayerOnHover 那条「图标几何不变」断言的灵敏度
// 依据 —— 判据不灵敏的话，那条断言等于没写。
func TestSilhouetteBoundsCatchesTwoIcons(t *testing.T) {
	one := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(40, 30).Children(func() {
			ui.Icon(c, iconPause).Size(16, 16).TextColor(ui.Hex("#0f172a"))
		})
	}, 40, 30)
	one.Frame()
	_, _, w1, h1 := silhouetteBounds(iconSilhouette(one.Image(), ui.Rect{X: 0, Y: 0, W: 40, H: 30}))

	two := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Size(40, 30).Children(func() {
			ui.Icon(c, iconPause).Size(16, 16).TextColor(ui.Hex("#0f172a"))
			ui.Icon(c, iconPause).Size(16, 16).TextColor(ui.Hex("#0f172a"))
		})
	}, 40, 30)
	two.Frame()
	_, _, w2, _ := silhouetteBounds(iconSilhouette(two.Image(), ui.Rect{X: 0, Y: 0, W: 40, H: 30}))

	if w1 == 0 || w2 == 0 {
		t.Fatalf("没量到图标墨迹：一个 %d×%d，两个 %d×%d", w1, h1, w2, h1)
	}
	if w2 <= w1 {
		t.Errorf("并排两个图标的包围盒宽 %d 不大于单个的 %d —— 判据抓不住两层图标", w2, w1)
	}
	t.Logf("单个 %d×%d，并排两个宽 %d", w1, h1, w2)
}
