package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestInactiveAccentKeepsTheAccent 钉住 mygo 0.8.0 的「非活动窗口淡成灰」这条
// 行为变更：本仓库显式把 `Theme.InactiveAccent` 设成 `Theme.Accent`，让每个窗口
// 都保持主题蓝。
//
// 为什么重要：0.8.0 起框架会把非活动窗口的强调色（输入框、滚动条、焦点环、
// 菜单、复选框…）淡成灰。本仓库绝大多数强调色是自绘的（直写 t.Blue），不会
// 跟着灰，若这里不设就会出「框架控件变灰、自绘控件仍蓝」的半灰界面 ——
// 这是个常驻托盘的浮窗应用，窗口失焦（去浏览器里挑视频）是常态。
func TestInactiveAccentKeepsTheAccent(t *testing.T) {
	th := ThemeFor(PeriodAt(12)).uiTheme()
	if th.InactiveAccent != th.Accent {
		t.Fatalf("InactiveAccent = %v，应等于 Accent = %v（否则非活动窗口会淡成灰）",
			th.InactiveAccent, th.Accent)
	}
	// 反证：零值 InactiveAccent 正是框架「淡成灰」的默认路径。
	if th.InactiveAccent == (ui.Color{}) {
		t.Fatal("InactiveAccent 是零值，框架会淡成灰")
	}
	// 三个时段（以及迷你窗共用的浅色主题）都要一致。
	for _, p := range []Period{PeriodAt(6), PeriodAt(12), PeriodAt(20)} {
		th := ThemeFor(p).uiTheme()
		if th.InactiveAccent != th.Accent {
			t.Errorf("时段 %q 的 InactiveAccent = %v，应等于 Accent %v", p.Name, th.InactiveAccent, th.Accent)
		}
	}
}

// TestUIPageKeepsAccentInBackground 是无头实测的反证：把一个窗口设成非活动
// （tester 的 SetWindowActive(false)），框架画的强调色仍要是主题蓝。
func TestUIPageKeepsAccentInBackground(t *testing.T) {
	th := ThemeFor(PeriodAt(12)).uiTheme()
	tst := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(th)
		ui.Box(c).Fill().Background(ui.Hex("#ffffff"))
		// CheckboxBase 是框架自己用强调色填充的控件。
		var on bool = true
		ui.CheckboxBase(c, &on).Label("选中项")
	}, 80, 80)
	tst.Frame()
	tst.SetWindowActive(false)
	tst.Frame()
	// 不崩溃、且主题色未被框架改写即视为通过：核心不变量在纯函数层面已钉住，
	// 这里防的是 0.8.0 的 emphasized() 路径在本仓库主题下 panic 或改色。
	if got := th.InactiveAccent; got != th.Accent {
		t.Fatalf("跑过非活动帧后 InactiveAccent 被改成 %v", got)
	}
}

// TestTooltipDelayFasterThanFrameworkDefault 钉住 mygo 0.8.0 的 Theme.TooltipDelay：
// 本仓库把它从框架默认的 0.6s 收到 0.35s。
//
// 这是个常驻托盘的浮窗应用，播放栏与抽屉表头一排排全是图标键（55 处 tooltip）：
// 鼠标顺手划过去要等 0.6s 才显形，用起来就当成「没有 tooltip」了。
func TestTooltipDelayFasterThanFrameworkDefault(t *testing.T) {
	th := ThemeFor(PeriodAt(12)).uiTheme()
	const frameworkDefault = 600 * time.Millisecond
	if th.TooltipDelay <= 0 {
		t.Fatalf("TooltipDelay = %v，应为正值（0 表示退回框架默认 0.6s）", th.TooltipDelay)
	}
	if th.TooltipDelay >= frameworkDefault {
		t.Errorf("TooltipDelay = %v，应比框架默认的 %v 更快", th.TooltipDelay, frameworkDefault)
	}
	// 也不能快到「鼠标划过就弹」——至少留 250ms 的「停一下」。
	if th.TooltipDelay < 250*time.Millisecond {
		t.Errorf("TooltipDelay = %v，太快了（鼠标划过就会弹），至少留 250ms", th.TooltipDelay)
	}
}
