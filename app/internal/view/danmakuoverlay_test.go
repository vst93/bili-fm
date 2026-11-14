package view

// 视频窗弹幕覆盖层的无头测试：渲染点计数、开关、暂停 / seek、设置即时生效、
// 设置菜单子面板的真实点击。
//
// 播放器与窗口仍是纯逻辑桩（见 videochrome_test.go），弹幕排版在
// internal/danmaku 里已有纯逻辑测试；这里只钉「view 把 Sprites 画出来 / 不画」
// 与「设置从菜单到窗口的这一段接线」。

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

func danmakuChrome(t *testing.T, pos float64, playing bool, items []danmaku.Item, s danmaku.Settings) (*VideoChrome, *videoStubPlayer, *danmaku.Overlay, *ui.Tester) {
	t.Helper()
	pl := &videoStubPlayer{playing: playing, ready: true, dur: 200 * time.Second,
		pos: time.Duration(pos * float64(time.Second)), rate: 1, vol: 1}
	win := newVideoStubWindow()
	o := &danmaku.Overlay{}
	o.SetItems(items)
	ch := &VideoChrome{
		Theme:           ThemeFor(PeriodAt(12)),
		Title:           "测试稿件",
		Pl:              pl,
		Win:             win,
		Danmaku:         s.On,
		DanmakuOverlay:  o,
		DanmakuSettings: s,
	}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 640, 360)
	tt.Frame()
	return ch, pl, o, tt
}

// TestVideoDanmakuToggle 钉住「视频窗内那颗弹幕键 = 总开关」：开时画出文字
// 元素，点一下之后渲染点为 0（不是只把透明度调低），再点回来恢复。
func TestVideoDanmakuToggle(t *testing.T) {
	s := danmaku.DefaultSettings()
	items := []danmaku.Item{
		{Time: 9, Text: "第一条测试弹幕"},
		{Time: 9.5, Text: "第二条测试弹幕"},
	}
	ch, _, o, tt := danmakuChrome(t, 10, true, items, s)
	var toggles []bool
	ch.OnDanmakuToggle = func(on bool) { toggles = append(toggles, on) }
	if ch.DanmakuDrawn() == 0 {
		t.Fatalf("弹幕开着却没有渲染点（同屏 %d）", o.Active())
	}
	if !tt.HasText("第一条测试弹幕") {
		t.Errorf("弹幕文字没有出现在画面上：%v", tt.Texts())
	}

	if err := tt.Click("弹幕"); err != nil {
		t.Fatalf("点弹幕键失败：%v", err)
	}
	tt.Frame()
	if ch.Danmaku {
		t.Fatal("点一下后开关仍为开")
	}
	if len(toggles) != 1 || toggles[0] {
		t.Errorf("视频窗开关没有把关闭动作回传主窗：%v", toggles)
	}
	if ch.DanmakuDrawn() != 0 {
		t.Errorf("关掉后仍渲染了 %d 个弹幕元素", ch.DanmakuDrawn())
	}
	if o.Active() != 0 {
		t.Errorf("关掉后可视集没有清空：%d 条", o.Active())
	}
	if tt.HasText("第一条测试弹幕") {
		t.Error("关掉后弹幕文字仍在画面上")
	}

	if err := tt.Click("弹幕"); err != nil {
		t.Fatalf("再次点弹幕键失败：%v", err)
	}
	tt.Frame()
	if !ch.Danmaku || ch.DanmakuDrawn() == 0 {
		t.Errorf("重新打开后没有恢复渲染：on=%v drawn=%d", ch.Danmaku, ch.DanmakuDrawn())
	}
}

// TestVideoDanmakuPauseAndSeek 钉住时间轴：暂停时 X 不动；seek 到 100s 后
// 只画 100s 附近的条目。
func TestVideoDanmakuPauseAndSeek(t *testing.T) {
	s := danmaku.DefaultSettings()
	items := []danmaku.Item{
		{Time: 9, Text: "十秒处"},
		{Time: 100, Text: "一百秒处"},
	}
	_, pl, o, tt := danmakuChrome(t, 10, true, items, s)
	sprites := o.Sprites()
	if len(sprites) == 0 {
		t.Fatal("10s 处没有弹幕")
	}
	x0 := sprites[0].X

	// 暂停：播放器位置不动，弹幕也不动（即使视图还在走帧）。
	pl.playing = false
	for i := 0; i < 30; i++ {
		tt.Frame()
	}
	sprites = o.Sprites()
	if len(sprites) == 0 {
		t.Fatal("暂停后弹幕消失了")
	}
	if sprites[0].X != x0 {
		t.Errorf("暂停时弹幕移动了：x %.1f → %.1f", x0, sprites[0].X)
	}

	// seek 到 100.5s（条目在 100s 入场，已滑入画面）：旧条目全部回收，
	// 新条目出现。
	pl.pos = 100500 * time.Millisecond
	pl.playing = true
	tt.Frame()
	for _, sp := range o.Sprites() {
		if sp.Time < 99 {
			t.Errorf("seek 后仍显示旧弹幕 %q（time=%.1f）", sp.Text, sp.Time)
		}
	}
	if !tt.HasText("一百秒处") {
		t.Errorf("seek 后没有显示 100s 的弹幕：%v", tt.Texts())
	}
	if tt.HasText("十秒处") {
		t.Error("seek 后仍画着 10s 的弹幕")
	}
}

// TestVideoDanmakuEndedClears 钉住「播完清空」：自然播完后一帧不再有弹幕
// 元素（重播时 Ended 复位，按新位置重建）。
func TestVideoDanmakuEndedClears(t *testing.T) {
	s := danmaku.DefaultSettings()
	ch, pl, o, tt := danmakuChrome(t, 9.5, true, []danmaku.Item{{Time: 9, Text: "播完前的弹幕"}}, s)
	if ch.DanmakuDrawn() == 0 {
		t.Fatal("播完前应有弹幕")
	}
	pl.playing = false
	pl.ended = true
	tt.Frame()
	if ch.DanmakuDrawn() != 0 || o.Active() != 0 {
		t.Errorf("播完后仍有弹幕：drawn=%d active=%d", ch.DanmakuDrawn(), o.Active())
	}
	pl.ended = false
	pl.pos = 0
	pl.playing = true
	tt.Frame()
	// 0 秒处没有条目，但重建本身不应再报错 / 挂住。
	if ch.DanmakuDrawn() != 0 {
		t.Errorf("重播到 0 秒仍画着 %d 条", ch.DanmakuDrawn())
	}
}

// TestVideoDanmakuSettingsApplyImmediately 钉住「改完立即生效」：同一帧
// 之后把显示区域收到上方一半、上限降到 30、字号放大，下一次构建就按新
// 设置排版（不重开窗口）。
func TestVideoDanmakuSettingsApplyImmediately(t *testing.T) {
	s := danmaku.DefaultSettings()
	items := make([]danmaku.Item, 0, 200)
	for i := 0; i < 200; i++ {
		items = append(items, danmaku.Item{Time: 9.5 + float64(i)*0.01, Text: "密集弹幕"})
	}
	ch, _, o, tt := danmakuChrome(t, 10, true, items, s)
	if o.Active() > s.Max {
		t.Fatalf("同屏 %d 条超过默认上限 %d", o.Active(), s.Max)
	}
	for _, sp := range o.Sprites() {
		if sp.Y > 360 {
			t.Fatalf("整屏模式下弹幕跑到画面外：y=%.1f", sp.Y)
		}
	}

	s.Area = 0.5
	s.Max = 30
	s.Size = 2
	ch.DanmakuSettings = s
	tt.Frame()
	if o.Active() > 30 {
		t.Errorf("上限改成 30 后同屏 %d 条", o.Active())
	}
	half := float32(360 * 0.5)
	for _, sp := range o.Sprites() {
		if sp.Y >= half {
			t.Errorf("显示区域收到上方 1/2 后弹幕仍在 y=%.1f（区域高 %.0f）", sp.Y, half)
		}
	}
	if ch.DanmakuDrawn() > 30 {
		t.Errorf("渲染点 %d 超过上限 30", ch.DanmakuDrawn())
	}
}

// TestDanmakuSettingsPanelClicks 走真实 tester：设置菜单 → 弹幕设置子面板 →
// 逐个点档位，断言 App 状态与上层回调（落盘 + 同步视频窗的入口）都拿到值。
func TestDanmakuSettingsPanelClicks(t *testing.T) {
	a, tt := settingsProbe(t, 12)
	var got []danmaku.Settings
	a.Act.SetDanmakuStyle = func(s danmaku.Settings) { got = append(got, s) }

	if err := tt.Click("设置"); err != nil {
		t.Fatalf("点设置键失败：%v", err)
	}
	tt.Frame()
	if err := tt.Click("弹幕设置"); err != nil {
		t.Fatalf("点「弹幕设置」失败：%v", err)
	}
	tt.Frame()
	if !a.SettingsDanmaku {
		t.Fatal("没有进入弹幕设置子面板")
	}

	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatalf("点 %q 失败：%v", label, err)
		}
		tt.Frame()
	}
	click("弹幕设置：不透明度 50%")
	if a.DanmakuStyle.Opacity != 0.5 {
		t.Errorf("不透明度 = %v，想要 0.5", a.DanmakuStyle.Opacity)
	}
	click("弹幕设置：字号 大")
	if a.DanmakuStyle.Size != 2 {
		t.Errorf("字号档 = %d，想要 2（大）", a.DanmakuStyle.Size)
	}
	click("弹幕设置：显示区域 上方 1/2")
	if a.DanmakuStyle.Area != 0.5 {
		t.Errorf("显示区域 = %v，想要 0.5", a.DanmakuStyle.Area)
	}
	click("弹幕设置：同屏上限 30")
	if a.DanmakuStyle.Max != 30 {
		t.Errorf("同屏上限 = %d，想要 30", a.DanmakuStyle.Max)
	}
	if !a.ShowSettings || !a.SettingsDanmaku {
		t.Errorf("调设置不该收起菜单：open=%v sub=%v", a.ShowSettings, a.SettingsDanmaku)
	}
	if len(got) < 4 {
		t.Fatalf("上层只收到 %d 次设置更新，想要 4", len(got))
	}
	last := got[len(got)-1]
	if !last.On || last.Opacity != 0.5 || last.Size != 2 || last.Area != 0.5 || last.Max != 30 {
		t.Errorf("最后一次下发的设置不对：%+v", last)
	}

	// 返回主菜单，用主菜单里的同名开关关掉弹幕：子面板状态要复位，
	// 开关仍留在展开的菜单里（方便连续调）。
	click("返回设置")
	if a.SettingsDanmaku {
		t.Error("返回后子面板没有关闭")
	}
	click("设置：弹幕")
	if a.DanmakuStyle.On {
		t.Error("主菜单的弹幕开关没有关掉弹幕")
	}
	if !a.ShowSettings {
		t.Error("切换弹幕开关后设置菜单不应收起")
	}
	tt.ClickAt(500, 500)
	tt.Frame()
	if a.ShowSettings {
		t.Fatal("点面板外没有收起设置菜单")
	}
	if a.SettingsDanmaku {
		t.Error("菜单收起后子面板状态没有复位")
	}
}

// TestDanmakuRenderCost 是渲染开销的量化证据：同样的视频窗、同样的密集
// 弹幕，开与关各渲染 30 帧，打印每帧耗时（默认上限 60 与上限 100 两档）。
// 弹幕层每帧只建可见的 Sprite（≤ 上限），排版本身在 internal/danmaku 的
// 滑窗测试里另测。
func TestDanmakuRenderCost(t *testing.T) {
	measure := func(max int) (active int, on, off time.Duration) {
		s := danmaku.DefaultSettings()
		s.Max = max
		items := make([]danmaku.Item, 0, 500)
		for i := 0; i < 500; i++ {
			items = append(items, danmaku.Item{Time: 15 + float64(i)*0.01, Text: "短"})
		}
		ch, pl, o, tt := danmakuChrome(t, 20, true, items, s)
		// 冻结时钟：只测「画 100 条」的构建/光栅化成本，不让条目在测量
		// 期间过期（Tester 的时钟不动，dt=0）。
		pl.playing = false
		active = o.Active()
		const frames = 60
		run := func() time.Duration {
			start := time.Now()
			for i := 0; i < frames; i++ {
				// SetSize 同尺寸也会标脏，保证这一帧真的重建 + 重绘
				//（Tester.Frame 在没有任何变化时会跳过绘制）。
				tt.SetSize(640, 360)
				tt.Frame()
			}
			return time.Since(start) / frames
		}
		on = run()
		ch.Danmaku = false
		off = run()
		return active, on, off
	}
	for _, max := range []int{60, 100} {
		active, on, off := measure(max)
		t.Logf("640×360、上限 %d（同屏 %d 条）：开弹幕 %.2fms/帧，关弹幕 %.2fms/帧（净增 %.2fms）",
			max, active, float64(on.Microseconds())/1000, float64(off.Microseconds())/1000,
			float64(on-off)/float64(time.Millisecond))
		if on > 200*time.Millisecond {
			t.Errorf("上限 %d：开弹幕单帧 %.1fms，超过 200ms", max, float64(on.Microseconds())/1000)
		}
	}
}
