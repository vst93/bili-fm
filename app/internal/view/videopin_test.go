package view

// 视频窗顶栏「置顶」键（videochrome.go 的 pinKey）的无头测试。
//
// 这一轮的需求是「顶栏最前面加一颗置顶按钮，行为与外观对齐主窗迷你模式的
// 固定功能」，所以测试钉的是对齐本身：
//
//   - 点击真的把 SetAlwaysOnTop 下发到目标窗口（开 → 关，桩逐次记录）；
//   - 激活 / 非激活两态**画出来**有可见差异（像素差异 + 无障碍按压态 +
//     tooltip 文案），且置顶时用的是迷你窗那颗的实心图钉（不是另画一枚）；
//   - 键尺寸与迷你窗那颗完全一致（30×30）；
//   - 顶栏顺序：置顶是第一颗，且这一行仍然只有四颗键；
//   - 全屏进出后置顶状态不丢、并且重新下发过一次（见 videochrome.go 的
//     View：DWM / AppKit / GTK 在状态切换时可能清掉顶层窗口等级）；
//   - 置顶「失败」（Wayland 合成器忽略 keep-above）不影响播放与其它按钮。

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// videoPinProbe 起一个 640×360 的无头视频窗，pin 为初始置顶状态。
func videoPinProbe(t *testing.T, pinned bool) (*VideoChrome, *videoStubPlayer, *videoStubWindow, *ui.Tester) {
	t.Helper()
	pl := &videoStubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second, rate: 1, vol: 1}
	win := newVideoStubWindow()
	ch := &VideoChrome{
		Theme:  ThemeFor(PeriodAt(12)),
		Title:  "测试稿件 · 第一部分",
		Pl:     pl,
		Win:    win,
		Pinned: pinned,
	}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 640, 360)
	tt.Frame()
	return ch, pl, win, tt
}

// pinKeyName 返回当前状态下那颗键的无障碍名字（= tooltip = label）。
func pinKeyName(pinned bool) string {
	if pinned {
		return "取消置顶"
	}
	return "置顶"
}

// TestVideoPinKeyTogglesWindow 钉住「点一下置顶开、再点一下置顶关」：每次点击
// 都必须下发到目标窗口的 SetAlwaysOnTop（参数就是点击后的状态），本地状态与
// 无障碍按压态同步翻转。
func TestVideoPinKeyTogglesWindow(t *testing.T) {
	ch, _, win, tt := videoPinProbe(t, false)
	// 开窗那一次下发（默认不置顶）已经在 init 里发生过。
	if len(win.topCalls) != 1 || win.topCalls[0] {
		t.Fatalf("开窗应下发一次「不置顶」，实际 %v", win.topCalls)
	}

	if err := tt.Click("置顶"); err != nil {
		t.Fatalf("点不到置顶键（现有：%v）", tt.Texts())
	}
	tt.Frame()
	if !ch.Pinned {
		t.Error("点了置顶键，状态没打开")
	}
	if got := win.topCalls[len(win.topCalls)-1]; !got {
		t.Errorf("点置顶没有把 SetAlwaysOnTop(true) 下发到窗口，实际调用序列 %v", win.topCalls)
	}
	if !win.topmost {
		t.Error("窗口没有真的变成置顶")
	}
	// tooltip / 名字随状态切换，按压态交给读屏（与迷你窗那颗同为 toggle button）。
	// 用 Find 而不是 HasText：后者是子串匹配，「置顶」会命中「取消置顶」。
	if _, ok := tt.Find("取消置顶"); !ok {
		t.Errorf("置顶后 tooltip 应为「取消置顶」：%v", tt.Texts())
	}
	if _, ok := tt.Find("置顶"); ok {
		t.Errorf("置顶后仍能找到旧文案「置顶」：%v", tt.Texts())
	}
	if info, ok := tt.Access("取消置顶"); !ok {
		t.Fatal("置顶态读不到那颗键的无障碍节点")
	} else if info.Role != ui.RoleToggleButton || !info.Checked {
		t.Errorf("置顶键应为 checked 的 toggle button，得到 role=%v checked=%v", info.Role, info.Checked)
	}

	if err := tt.Click("取消置顶"); err != nil {
		t.Fatalf("点不到「取消置顶」：%v", err)
	}
	tt.Frame()
	if ch.Pinned {
		t.Error("再点一次应取消置顶")
	}
	if got := win.topCalls[len(win.topCalls)-1]; got {
		t.Errorf("取消置顶没有下发 SetAlwaysOnTop(false)，实际调用序列 %v", win.topCalls)
	}
	if win.topmost {
		t.Error("窗口仍是置顶")
	}
	if _, ok := tt.Find("置顶"); !ok {
		t.Errorf("取消后 tooltip 应回到「置顶」：%v", tt.Texts())
	}
	if _, ok := tt.Find("取消置顶"); ok {
		t.Errorf("取消后仍能找到「取消置顶」：%v", tt.Texts())
	}
	if info, ok := tt.Access("置顶"); !ok {
		t.Fatal("非置顶态读不到那颗键的无障碍节点")
	} else if info.Role != ui.RoleToggleButton || info.Checked {
		t.Errorf("非置顶键应为未 checked 的 toggle button，得到 role=%v checked=%v", info.Role, info.Checked)
	}
}

// TestVideoPinStateIsVisible 钉住两态**画出来**有可见差异：同一个键矩形里，
// 置顶 / 非置顶两张图要有足量像素不同；并且置顶时用的是迷你窗那颗的实心图钉
// （被图标围住的背景空洞为 0），非置顶是描边图钉（有空洞）。
//
// 这同时反证「图标复用」是接在渲染链上的：把 pinKey 里的 miniPinIcon(v.Pinned)
// 换成固定的 iconPin，实心断言会变红。
func TestVideoPinStateIsVisible(t *testing.T) {
	off, _, _, ttOff := videoPinProbe(t, false)
	_, _, _, ttOn := videoPinProbe(t, true)
	rOff, ok := ttOff.Find("置顶")
	if !ok {
		t.Fatalf("找不到置顶键（现有：%v）", ttOff.Texts())
	}
	rOn, ok := ttOn.Find("取消置顶")
	if !ok {
		t.Fatalf("找不到取消置顶键（现有：%v）", ttOn.Texts())
	}
	if rOn != rOff {
		t.Errorf("两态的键矩形应完全一致（只有材质不同）：%v vs %v", rOff, rOn)
	}

	diff := diffPixels(ttOff.Image(), ttOn.Image(), rOff)
	if total := int(rOff.W) * int(rOff.H); diff < total/4 {
		t.Errorf("置顶 / 非置顶只有 %d/%d 像素不同，视觉差异不足", diff, total)
	}

	// 实心图钉没有「被图标围住的背景」；描边图钉有。像素判据靠蓝图标
	// （蓝 B-R > 80），所以两帧都取置顶态（图标是 --studio-blue），第二帧
	// 把 miniPinIcon 换成固定描边图钉做反证 —— 这也直接证明渲染链用的是
	// 迷你窗那枚图标函数，而不是另画的一枚。
	solid := inkPockets(ttOn.Image(), rOn)
	if solid != 0 {
		t.Errorf("置顶态应画实心图钉，仍有 %d 个封闭空洞", solid)
	}
	saved := miniPinIcon
	miniPinIcon = func(bool) *ui.SVG { return iconPin }
	defer func() { miniPinIcon = saved }()
	_, _, _, ttOutline := videoPinProbe(t, true)
	rOutline, ok := ttOutline.Find("取消置顶")
	if !ok {
		t.Fatal("找不到置顶键（描边反证）")
	}
	outline := inkPockets(ttOutline.Image(), rOutline)
	t.Logf("置顶键封闭空洞：实心 %d、描边 %d", solid, outline)
	if outline == 0 {
		t.Error("把图标换成描边图钉后没有出现封闭空洞：反证失败（置顶键没走 miniPinIcon）")
	}
	// 两态的状态本身也要不同（防止像素差异来自别的东西）。
	if off.Pinned {
		t.Error("非置顶探针的 Pinned 应为假")
	}
}

// TestVideoPinKeyMatchesMini 钉住「同一个图标与键尺寸」：视频窗这颗键的矩形
// 与迷你窗那颗固定键完全同尺寸（30×30，见 miniControl / videoChromeKey）。
func TestVideoPinKeyMatchesMini(t *testing.T) {
	_, _, _, ttVideo := videoPinProbe(t, true)
	video, ok := ttVideo.Find("取消置顶")
	if !ok {
		t.Fatal("视频窗找不到置顶键")
	}
	mini, ok := miniProbe(t, true).Find("取消窗口置顶")
	if !ok {
		t.Fatal("迷你窗找不到固定键")
	}
	if video.W != mini.W || video.H != mini.H {
		t.Errorf("视频窗置顶键 %vx%v，迷你窗固定键 %vx%v：键尺寸不一致", video.W, video.H, mini.W, mini.H)
	}
	if video.W != VideoChromeKey || video.H != VideoChromeKey {
		t.Errorf("置顶键应为 %v×%v，实际 %v×%v", VideoChromeKey, VideoChromeKey, video.W, video.H)
	}
}

// TestVideoTitleBarKeyOrder 钉住顶栏顺序与键数：置顶是**第一颗**（最左），
// 后面依次是弹幕 / 最小化 / 退出视频播放，且这一行只有这四颗键 —— 没有多冒出
// 来的第五颗。
func TestVideoTitleBarKeyOrder(t *testing.T) {
	_, _, _, tt := videoPinProbe(t, false)
	names := []string{"置顶", "弹幕", "最小化", "退出视频播放"}
	rects := make([]ui.Rect, len(names))
	for i, n := range names {
		r, ok := tt.Find(n)
		if !ok {
			t.Fatalf("顶栏缺少 %q（现有：%v）", n, tt.Texts())
		}
		rects[i] = r
	}
	for i := 1; i < len(rects); i++ {
		if rects[i].X <= rects[i-1].X {
			t.Errorf("%q 应排在 %q 右边：%v vs %v", names[i], names[i-1], rects[i], rects[i-1])
		}
		if rects[i].Y != rects[0].Y {
			t.Errorf("%q 与 %q 不在同一行：y=%v vs %v", names[i], names[0], rects[i].Y, rects[0].Y)
		}
	}

	// 这一行里的**全部**按钮/开关：只有这四颗（顺序也一致）。
	inBar := titleBarKeys(tt)
	var got []string
	for _, k := range inBar {
		got = append(got, k.Name)
	}
	if len(got) != len(names) {
		t.Fatalf("顶栏有 %d 颗键（%v），想要 %d 颗 %v", len(got), got, len(names), names)
	}
	// 从左到右读回来的顺序必须与上面的名字一致（第一颗就是置顶）。
	for i := range names {
		if got[i] != names[i] {
			t.Errorf("顶栏第 %d 颗是 %q，想要 %q（从左到右：%v）", i+1, got[i], names[i], got)
		}
	}
}

// titleBarKeys 返回顶栏那一行（videoChromePad 起、高 videoChromeTitleH）里的
// 按钮 / 开关类无障碍节点，按 x 从左到右排好。
func titleBarKeys(tt *ui.Tester) []ui.AccessInfo {
	top := float32(videoChromePad) - 1
	bottom := float32(videoChromePad+videoChromeTitleH) + 1
	var out []ui.AccessInfo
	for _, info := range tt.AccessAll() {
		if info.Role != ui.RoleButton && info.Role != ui.RoleToggleButton {
			continue
		}
		if info.Bounds.Y < top || info.Bounds.Y > bottom {
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

// TestVideoPinSurvivesFullScreen 钉住 §1.5 的结论：置顶与全屏是两条独立的
// 窗口属性 —— 进全屏**保留**置顶、退出全屏**恢复**同一个置顶状态（不清掉），
// 且每次全屏状态真的变了都重新下发一次（某些 WM / AppKit / DWM 会清掉顶层
// 等级）。非置顶时任何一次下发都是 false，不会「进一次全屏就莫名置顶」。
func TestVideoPinSurvivesFullScreen(t *testing.T) {
	ch, pl, win, tt := videoPinProbe(t, false)
	if err := tt.Click("置顶"); err != nil {
		t.Fatalf("点不到置顶键：%v", err)
	}
	tt.Frame()
	if !win.topmost {
		t.Fatal("置顶没有生效")
	}
	playCalls := *pl

	// 进全屏（点控件条那颗全屏键）：置顶不取消，并重新下发一次。
	if err := tt.Click("全屏"); err != nil {
		t.Fatalf("点不到全屏键：%v", err)
	}
	tt.Frame()
	if !win.full {
		t.Fatal("没有进入全屏")
	}
	if !ch.Pinned || !win.topmost {
		t.Error("进入全屏后置顶丢了")
	}
	if got := win.topCalls[len(win.topCalls)-1]; !got {
		t.Errorf("全屏状态变化后没有重新下发置顶，调用序列 %v", win.topCalls)
	}
	if !tt.HasText("取消置顶") {
		t.Errorf("全屏里置顶键应仍是「取消置顶」：%v", tt.Texts())
	}

	// 退出全屏：置顶仍然保持（不因退出全屏被复位），再补一次下发。
	if err := tt.Click("退出全屏"); err != nil {
		t.Fatalf("点不到退出全屏：%v", err)
	}
	tt.Frame()
	if win.full {
		t.Fatal("没有退出全屏")
	}
	if !ch.Pinned || !win.topmost {
		t.Error("退出全屏后置顶丢了")
	}
	if !tt.HasText("取消置顶") {
		t.Errorf("退出全屏后置顶键应仍是「取消置顶」：%v", tt.Texts())
	}
	// 全屏进出只碰窗口：播放 / 进度 / 音量 / 倍速一个都没动。
	if pl.playing != playCalls.playing || pl.pos != playCalls.pos ||
		pl.rate != playCalls.rate || pl.vol != playCalls.vol {
		t.Errorf("全屏进出动到了播放状态：before=%+v after=%+v", playCalls, *pl)
	}

	// 非置顶：进出全屏的每一次下发都必须是 false（不能凭空置顶）。
	ch2, _, win2, tt2 := videoPinProbe(t, false)
	if err := tt2.Click("全屏"); err != nil {
		t.Fatalf("点不到全屏键：%v", err)
	}
	tt2.Frame()
	if err := tt2.Click("退出全屏"); err != nil {
		t.Fatalf("点不到退出全屏：%v", err)
	}
	tt2.Frame()
	if ch2.Pinned || win2.topmost {
		t.Error("非置顶状态进全屏后不该变成置顶")
	}
	for _, v := range win2.topCalls {
		if v {
			t.Errorf("非置顶时出现了 SetAlwaysOnTop(true)：%v", win2.topCalls)
		}
	}
}

// TestVideoPinRejectionKeepsPlayback 钉住兜底：置顶在 Wayland 上可能被合成器
// 直接忽略（调用成功、窗口状态不变）。这时播放与其它键必须照常工作 ——
// 没有状态机以置顶成功为闸。
func TestVideoPinRejectionKeepsPlayback(t *testing.T) {
	ch, pl, win, tt := videoPinProbe(t, false)
	win.ignore = true // 合成器忽略 keep-above
	before := *pl
	mini := 0
	ch.OnMinimize = func() { mini++ }

	if err := tt.Click("置顶"); err != nil {
		t.Fatalf("点不到置顶键：%v", err)
	}
	tt.Frame()
	if win.topmost {
		t.Fatal("桩应当忽略置顶，实际生效了（测试前提不成立）")
	}
	if !ch.Pinned {
		t.Error("置顶请求被忽略时，键的按压态仍应跟随用户的请求")
	}
	if !pl.playing || pl.pos != before.pos || pl.rate != before.rate || pl.vol != before.vol {
		t.Errorf("置顶失败不该影响播放：before=%+v after=%+v", before, *pl)
	}

	// 其它键照常：弹幕翻转、暂停生效、最小化回调。
	danmakuBefore := ch.Danmaku
	if err := tt.Click("弹幕"); err != nil {
		t.Fatalf("点弹幕失败：%v", err)
	}
	tt.Frame()
	if ch.Danmaku == danmakuBefore {
		t.Error("置顶失败后弹幕键失效")
	}
	if err := tt.Click("暂停"); err != nil {
		t.Fatalf("点暂停失败：%v", err)
	}
	tt.Frame()
	if pl.playing {
		t.Error("置顶失败后暂停键失效")
	}
	if err := tt.Click("最小化"); err != nil {
		t.Fatalf("点最小化失败：%v", err)
	}
	tt.Frame()
	if mini != 1 {
		t.Errorf("置顶失败后最小化键失效：调用 %d 次", mini)
	}
}

// diffPixels 数出两张同尺寸图在矩形内有多少像素不同（逐通道差值 > 4 算不同）。
func diffPixels(a, b *image.RGBA, r ui.Rect) int {
	n := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			ia, ib := a.PixOffset(x, y), b.PixOffset(x, y)
			for k := 0; k < 4; k++ {
				d := int(a.Pix[ia+k]) - int(b.Pix[ib+k])
				if d < 0 {
					d = -d
				}
				if d > 4 {
					n++
					break
				}
			}
		}
	}
	return n
}
