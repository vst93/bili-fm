package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（进度条悬停时间胶囊的两个层级 / 命中问题）测试。
//
// 用户报两件事：
//  1. 弹出音量 / 倍速弹层时，在弹层上做选择会触发下一层进度条的悬浮时间胶囊
//     （命中链 / 事件穿透）；
//  2. 时间胶囊向上伸出播放栏顶边 10px，栏顶那条 1px 边框画在胶囊之上，
//     把胶囊腰部切出一条线（层级）。
//
// 对应两处修复：seekBubble 的「是否悬停」改读命中链（s.Hovered()）而不是
// PointerPosition 的矩形包含；播放栏顶边从 Absolute 子盒改为 Draw（Draw 在
// 子元素之前绘制）。

// seekBubblePopoverOverlap 打开音量弹层，返回（App、tester、弹层与进度条
// 重叠区里的一个点）。断言两者确实重叠，否则测试本身没意义。
func seekBubblePopoverOverlap(t *testing.T) (*App, *ui.Tester, float32, float32) {
	t.Helper()
	a, tst := playerBarProbe(t, func(a *App) { a.ShowVolume = true })
	timeline, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("播放栏里找不到进度条")
	}
	vol, ok := tst.Find("音量")
	if !ok {
		t.Fatal("播放栏里找不到音量键")
	}
	panel := volumePopoverRect(vol, volumePopoverW, volumePopoverH, volumePopoverEdgeGap)
	// 弹层与进度条在水平方向的重叠区。
	x0 := panel.X
	if timeline.X > x0 {
		x0 = timeline.X
	}
	x1 := panel.X + panel.W
	if timeline.X+timeline.W < x1 {
		x1 = timeline.X + timeline.W
	}
	if x1-x0 < 8 {
		t.Fatalf("音量弹层与进度条没有足够重叠（弹层 x=[%v,%v] 进度条 x=[%v,%v]）",
			panel.X, panel.X+panel.W, timeline.X, timeline.X+timeline.W)
	}
	return a, tst, (x0 + x1) / 2, timeline.Y + timeline.H/2
}

// TestSeekBubbleIgnoresPopoverHover 钉住：指针在音量弹层上（弹层矩形与进度条
// 重叠）时，进度条的悬浮时间胶囊不应出现——弹层是更上层的命中目标，
// 「事件」不该穿透到下一层的进度条。
func TestSeekBubbleIgnoresPopoverHover(t *testing.T) {
	a, tst, x, y := seekBubblePopoverOverlap(t)

	tst.Move(x, y)
	tst.Frame()
	if a.seekBubbleLabel != "" {
		t.Errorf("指针在音量弹层上时时间胶囊被点亮（label=%q）：弹层交互穿透到了进度条",
			a.seekBubbleLabel)
	}

	// 反证：同一个点、关掉弹层后应正常点亮（证明这一点确实在进度条上，
	// 上面的断言不是「点在进度条外」的假通过）。
	a.ShowVolume = false
	tst.Frame()
	tst.Move(x, y) // 指针没动也要重推一帧命中链
	tst.Frame()
	if a.seekBubbleLabel == "" {
		t.Error("关掉弹层后同一个点仍不点亮时间胶囊：点位根本不在进度条上，测试前提有误")
	}
}

// TestSeekBubbleHoverUsesHitChain 钉住 seekBubble 读的是命中链而不是矩形包含：
// 直接把进度条自己的 Hovered() 与「指针在矩形内」解耦，断言胶囊文案跟着
// Hovered() 走。这里是源码链断言（命中链是框架内部状态，无头测试里靠行为
// 断言已由 TestSeekBubbleIgnoresPopoverHover 覆盖）。
func TestSeekBubbleHoverUsesHitChain(t *testing.T) {
	flat := flatten(t, "playerbar.go")
	if !strings.Contains(flat, "over:=s.Hovered()") {
		t.Error("seekBubble 的 over 没有改读命中链（s.Hovered()）")
	}
	if strings.Contains(flat, "px,_,over:=s.PointerPosition()") {
		t.Error("seekBubble 仍在用 PointerPosition 的矩形包含判定悬停")
	}
}

// TestPlayerBarTopBorderUnderChildren 钉住播放栏顶边 1px 画在子元素之前。
//
// 判法（高对比主题，PlayerBorder 是深 slate）：栏顶那一行在**胶囊之外**应比
// 下一行明显更暗（边框还在）；在**胶囊 x 范围内**则不应明显更暗（边框画在
// 胶囊之上会把胶囊腰部切出一条线——正是用户报的）。
func TestPlayerBarTopBorderUnderChildren(t *testing.T) {
	a, tst := playerBarProbe(t, nil)
	// 高对比主题下 --player-border 是不透明的深 slate（#334155 .52），栏顶那
	// 1px 在胶囊之上时会比胶囊内部明显暗——用它把「边框压过胶囊」放大出来。
	tst.SetPreferences(ui.Preferences{HighContrast: true})
	tst.Frame()
	if !a.Theme.HighContrast {
		t.Fatal("高对比偏好没有生效（Shell 里 Theme 被时段基态重置？）")
	}
	if a.Theme.PlayerBorder == ui.Hex("#ffffff").Alpha(0.72) {
		t.Fatal("高对比下 PlayerBorder 仍是浅色白 .72，测不出差异")
	}
	bar, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("播放栏里找不到进度条")
	}
	// 悬停进度条中段，生成时间胶囊。
	tst.Move(bar.X+bar.W*0.5, bar.Y+bar.H/2)
	tst.Frame()
	if a.seekBubbleLabel == "" {
		t.Fatal("悬停进度条后没有时间胶囊")
	}
	if a.seekBubbleWidth <= 0 {
		t.Fatal("时间胶囊宽度为 0")
	}

	// 播放栏顶边：窗口 1000×700、栏高 56 → y = 644（取 644 那一行像素）。
	const barTop = 700 - PlayerBarHeight
	img := tst.Image()
	luma := func(x int, y int) float64 {
		i := img.PixOffset(x, y)
		return (float64(img.Pix[i])*299 + float64(img.Pix[i+1])*587 + float64(img.Pix[i+2])*114) / 1000
	}
	// 胶囊范围内的最亮像素：避开时间字形（字是深墨，会比底色暗），
	// 取横带最大值就落在字形之间的胶囊底色上。栏顶边框如果画在胶囊之上，
	// 整条横带（含字形之间）都会被压暗，最大值就抬不起来。
	bubbleMax := func(y int) float64 {
		x0 := int(bar.X+a.seekBubbleLeft) + 3
		x1 := int(bar.X+a.seekBubbleLeft+a.seekBubbleWidth) - 3
		best := 0.0
		for x := x0; x < x1; x++ {
			if v := luma(x, y); v > best {
				best = v
			}
		}
		return best
	}
	// 边框样本：胶囊之外的进度条中段。
	outX := int(bar.X + 12)
	outTop, outBelow := luma(outX, barTop), luma(outX, barTop+1)
	if !(outTop < outBelow-8) {
		t.Errorf("栏顶边框缺失或太淡：顶行亮度 %.1f 未明显暗于下一行 %.1f", outTop, outBelow)
	}
	topMax, belowMax := bubbleMax(barTop), bubbleMax(barTop+2)
	// 阈值 40：胶囊底色是 rgba(255,255,255,.86)，底下那点东西会透出 14%，
	// 所以栏顶边框在胶囊之下时仍会有 ~10 的亮度差（与原版一致）；但如果边框
	// 画在胶囊之上（Absolute 子盒的老写法），深 slate 会把整条带压到 ~154，
	// 与下面相差 ~100。
	if topMax < belowMax-40 {
		t.Errorf("胶囊腰部仍被栏顶边框切过：胶囊内最亮 %.1f 明显暗于下一行 %.1f（left=%v w=%v）",
			topMax, belowMax, a.seekBubbleLeft, a.seekBubbleWidth)
	}
}
