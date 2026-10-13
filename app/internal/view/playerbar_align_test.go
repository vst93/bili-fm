package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 播放栏对齐 Tauri 原版最后版本（main:src/components/player.tsx +
// main:src/styles/globals.css 的 #player 段）的回归测试。
//
// 本轮逐项复刻了四类此前对不上的取值：
//   1. 图标尺寸：播放 18→24、下一集 15→18（原版 icon-park 的 size 属性）；
//   2. 工具键材质：玻璃边从通用白 .55 收敛到原版 tool 规则的 slate .2
//      （悬停 slate .32），内高光白 .55→.44；
//   3. 工具键配色：常态 #526174、悬停/激活转 --studio-blue #0284c7；
//   4. 倍速文字字重 400→700。
// 取值全部抽成纯函数（playerToolFill/Border/Inset/Ink、playerPlayFill/
// Border/Ink），这里逐态钉死。

// TestPlayerBarIconSizes 钉住播放栏图标尺寸（原版 icon-park size）。
func TestPlayerBarIconSizes(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"播放/暂停", playIconSize, 24},
		{"下一集", nextIconSize, 18},
		{"音量", volumeIconSize, 20},
		{"均衡/恰饭", eqIconSize, 17},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s 图标 = %d，应为 %d", c.name, c.got, c.want)
		}
	}
}

// TestPlayerToolPaint 钉住工具键三态材质（原版 tool 规则 + :hover + 激活）。
func TestPlayerToolPaint(t *testing.T) {
	// 底色：常态 --liquid-bg 白 .22、悬停白 .34、激活 rgba(186,230,253,.30)。
	if got := playerToolFill(false, false); got != ui.Hex("#ffffff").Alpha(0.22) {
		t.Errorf("工具键常态底 = %v，应为白 .22", got)
	}
	if got := playerToolFill(false, true); got != ui.Hex("#ffffff").Alpha(0.34) {
		t.Errorf("工具键悬停底 = %v，应为白 .34", got)
	}
	if got := playerToolFill(true, false); got != ui.Hex("#bae6fd").Alpha(0.30) {
		t.Errorf("工具键激活底 = %v，应为 #bae6fd .30", got)
	}
	// 描边：常态 slate .2（**不是**通用 .player-button 的白 .55）。
	if got := playerToolBorder(false, false); got != ui.Hex("#94a3b8").Alpha(0.20) {
		t.Errorf("工具键常态边 = %v，应为 slate .2", got)
	}
	if got := playerToolBorder(false, true); got != ui.Hex("#94a3b8").Alpha(0.32) {
		t.Errorf("工具键悬停边 = %v，应为 slate .32", got)
	}
	if got := playerToolBorder(true, false); got != ui.Hex("#0284c7").Alpha(0.28) {
		t.Errorf("工具键激活边 = %v，应为蓝 .28", got)
	}
	// 内高光：常态白 .44、悬停白 .56。
	if got := playerToolInset(false); got != ui.Hex("#ffffff").Alpha(0.44) {
		t.Errorf("工具键常态内高光 = %v，应为白 .44", got)
	}
	if got := playerToolInset(true); got != ui.Hex("#ffffff").Alpha(0.56) {
		t.Errorf("工具键悬停内高光 = %v，应为白 .56", got)
	}
	// 图标色：常态 #526174、悬停/激活 #0284c7、激活且悬停 #0369a1。
	if got := playerToolInk(false, false); got != ui.Hex("#526174") {
		t.Errorf("工具键常态图标 = %v，应为 #526174", got)
	}
	if got := playerToolInk(false, true); got != ui.Hex("#0284c7") {
		t.Errorf("工具键悬停图标 = %v，应为 #0284c7", got)
	}
	if got := playerToolInk(true, false); got != ui.Hex("#0284c7") {
		t.Errorf("工具键激活图标 = %v，应为 #0284c7", got)
	}
	if got := playerToolInk(true, true); got != ui.Hex("#0369a1") {
		t.Errorf("工具键激活悬停图标 = %v，应为 #0369a1", got)
	}
}

// TestPlayerPlayPaint 钉住播放键三态材质（原版 [data-playing] / :not(...) + hover）。
func TestPlayerPlayPaint(t *testing.T) {
	// 暂停常态：底白 .22、边 slate .22、图标 #475569。
	if got := playerPlayFill(false, false); got != ui.Hex("#ffffff").Alpha(0.22) {
		t.Errorf("暂停键常态底 = %v，应为白 .22", got)
	}
	if got := playerPlayBorder(false, false); got != ui.Hex("#94a3b8").Alpha(0.22) {
		t.Errorf("暂停键常态边 = %v，应为 slate .22", got)
	}
	if got := playerPlayInk(false, false); got != ui.Hex("#475569") {
		t.Errorf("暂停键常态图标 = %v，应为 #475569", got)
	}
	// 暂停悬停：底 rgba(186,230,253,.5)、边蓝 .28、图标 #0284c7。
	if got := playerPlayFill(false, true); got != ui.Hex("#bae6fd").Alpha(0.50) {
		t.Errorf("暂停键悬停底 = %v，应为 #bae6fd .5", got)
	}
	if got := playerPlayBorder(false, true); got != ui.Hex("#0284c7").Alpha(0.28) {
		t.Errorf("暂停键悬停边 = %v，应为蓝 .28", got)
	}
	if got := playerPlayInk(false, true); got != ui.Hex("#0284c7") {
		t.Errorf("暂停键悬停图标 = %v，应为 #0284c7", got)
	}
	// 播放中：底 rgba(125,211,252,.16)、边蓝 .30、图标 #0369a1；悬停加深。
	if got := playerPlayFill(true, false); got != ui.Hex("#7dd3fc").Alpha(0.16) {
		t.Errorf("播放键常态底 = %v，应为 #7dd3fc .16", got)
	}
	if got := playerPlayBorder(true, false); got != ui.Hex("#0284c7").Alpha(0.30) {
		t.Errorf("播放键常态边 = %v，应为蓝 .30", got)
	}
	if got := playerPlayInk(true, false); got != ui.Hex("#0369a1") {
		t.Errorf("播放键常态图标 = %v，应为 #0369a1", got)
	}
	if got := playerPlayFill(true, true); got != ui.Hex("#7dd3fc").Alpha(0.22) {
		t.Errorf("播放键悬停底 = %v，应为 #7dd3fc .22", got)
	}
	if got := playerPlayBorder(true, true); got != ui.Hex("#0284c7").Alpha(0.40) {
		t.Errorf("播放键悬停边 = %v，应为蓝 .40", got)
	}
}

// TestPlayerBarHoverTurnsIconBlue 无头实测：把指针移到「下一集」键上，键内应
// 出现 --studio-blue 的蓝（原版 tool 规则 :hover color: var(--studio-blue)）。
// 反证：把 playerToolInk 的 hovered 分支去掉（只留 #526174）→ 蓝通道不再抬高。
func TestPlayerBarHoverTurnsIconBlue(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Queue = []Track{{Bvid: "BV1", Cid: 1, Title: "t"}, {Bvid: "BV2", Cid: 2, Title: "t2"}}
	a.Index = 0
	a.Track = &a.Queue[0]
	tst := ui.NewTester(func(c *ui.Context) { a.playerBar(c) }, 800, 200)
	tst.Frame()
	r, ok := tst.Find("下一个视频")
	if !ok {
		t.Fatal("找不到「下一个视频」键")
	}
	blueness := func() int {
		img := tst.Image()
		best := 0
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				c := img.RGBAAt(x, y)
				// 蓝调：蓝通道明显高于红通道（#0284c7 的 b-r ≈ 197）。
				if d := int(c.B) - int(c.R); d > best {
					best = d
				}
			}
		}
		return best
	}
	rest := blueness()
	tst.Move(r.X+r.W/2, r.Y+r.H/2)
	tst.Frame()
	hover := blueness()
	t.Logf("「下一集」键内最大蓝调（B−R）静息=%d 悬停=%d", rest, hover)
	if hover <= rest+40 {
		t.Fatalf("悬停后图标没有转蓝：静息 B−R=%d 悬停 B−R=%d", rest, hover)
	}
}
