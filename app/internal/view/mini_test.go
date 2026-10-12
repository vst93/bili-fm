package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestTitleBarButtonStyle 钉住 .app-title-bar-btn / #switch-window-mode /
// .mini-window-controls button 的三态视觉（原版 globals.css）：
//
//	常态  background transparent; color --studio-muted; opacity .72
//	悬停  background-color --glass-bg-hover（白 .38）; color #0284c7; opacity 1
//	激活  （迷你窗图钉 aria-pressed=true）同悬停 + inset 内高光
//
// 之前迷你窗控制键的激活态误用了「玻璃 Tint 时代」的蓝 .28 底色 + 白描边，
// 与 --glass-bg-hover 的白 .38 不符；标题栏迷你入口也漏了常态 opacity .72 与
// 悬停图标转蓝。这个测试锁住修正后的取值。
func TestTitleBarButtonStyle(t *testing.T) {
	th := ThemeFor(PeriodAt(12))

	bg, ink, op, hi := titleBarButtonStyle(th, false, false)
	if bg != ui.Transparent {
		t.Errorf("常态底色 = %v，应为透明", bg)
	}
	if ink != th.Muted {
		t.Errorf("常态图标色 = %v，应为 --studio-muted %v", ink, th.Muted)
	}
	if op != 0.72 {
		t.Errorf("常态不透明度 = %v，应为 .72", op)
	}
	if hi {
		t.Error("常态不应有内高光")
	}

	for _, tc := range []struct {
		name            string
		active, hovered bool
	}{
		{"悬停", false, true},
		{"激活", true, false},
		{"激活+悬停", true, true},
	} {
		bg, ink, op, hi := titleBarButtonStyle(th, tc.active, tc.hovered)
		if bg != th.GlassHover {
			t.Errorf("%s 底色 = %v，应为 --glass-bg-hover %v", tc.name, bg, th.GlassHover)
		}
		if ink != th.Blue {
			t.Errorf("%s 图标色 = %v，应为 --studio-blue %v", tc.name, ink, th.Blue)
		}
		if op != 1 {
			t.Errorf("%s 不透明度 = %v，应为 1", tc.name, op)
		}
		if !hi {
			t.Errorf("%s 应叠内高光", tc.name)
		}
	}
}

// miniProbe 建一个 400×155 的迷你窗（固定 noon 时段，背景可复现），
// 带一条正在播放的曲目，便于采样封面/控制键区域。
func miniProbe(t *testing.T, pinned bool) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Mini = true
	a.Pinned = pinned
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()
	return tst
}

// TestMiniCoverShadow 是无头渲染实测：迷你窗封面（原版
// `#min-video-info-cover`）有一道 `0 10px 22px rgba(15,23,42,.16)` 的投影，
// 封面正下方应明显比同高度、远离封面的背景暗。
//
// 窗口背景是 145° 的浅色渐变（#fbfdff→#f3f8fb），整窗最大通道差 ≈8，所以
// 同 y 下 12 以上的差值只能来自投影。之前原生版漏了这道投影，封面看起来是
// 平贴在背景上（没有浮起感）。
func TestMiniCoverShadow(t *testing.T) {
	tst := miniProbe(t, false)
	img := tst.Image()
	sample := func(x, y int) (int, int, int) {
		i := img.PixOffset(x, y)
		return int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
	}
	// 封面 x 12..76、y 36..100（64px，居中于 88px 信息行）。取封面正下方
	// (44,104) 与同高度、远在投影范围外的 (350,104) 比较。
	br, bg, bb := sample(44, 104)
	rr, rg, rb := sample(350, 104)
	d := (rr - br + rg - bg + rb - bb) / 3
	t.Logf("封面下方=(%d,%d,%d) 参照=(%d,%d,%d) 差值=%d", br, bg, bb, rr, rg, rb, d)
	if d < 12 {
		t.Fatalf("封面下方只比背景暗 %d：封面投影缺失", d)
	}
}

// TestMiniPinButtonState 是无头渲染实测：迷你窗「窗口置顶」键（原版
// `.mini-window-controls button[aria-pressed="true"]`）置顶时图标转
// --studio-blue，底色是 --glass-bg-hover（白 .38，中性），**不是**蓝 Tint。
func TestMiniPinButtonState(t *testing.T) {
	off := miniProbe(t, false)
	on := miniProbe(t, true)

	bluePixels := func(tst *ui.Tester) int {
		img := tst.Image()
		n := 0
		// 置顶键：x 358..388、y 70..100（窗口控制列第二颗）。
		for y := 70; y < 100; y++ {
			for x := 358; x < 388; x++ {
				i := img.PixOffset(x, y)
				if int(img.Pix[i+2])-int(img.Pix[i]) > 80 {
					n++
				}
			}
		}
		return n
	}
	if n := bluePixels(off); n != 0 {
		t.Errorf("未置顶时置顶键出现 %d 个蓝色像素，应为 0", n)
	}
	if n := bluePixels(on); n == 0 {
		t.Error("置顶时置顶键没有蓝色图标像素")
	}

	// 底色应当是中性白（--glass-bg-hover），不是蓝 Tint：取按钮左上角一块
	// 背景像素，B-R 应很小（旧蓝 .28 底会到 ~65）。
	img := on.Image()
	i := img.PixOffset(373, 74)
	r, b := int(img.Pix[i]), int(img.Pix[i+2])
	if b-r > 20 {
		t.Errorf("置顶键底色偏蓝：(%d,...,%d) B-R=%d，应为中性的 --glass-bg-hover", r, b, b-r)
	}
}

// TestMiniVolumePopoverGeometry 钉住迷你窗音量弹层的挂法与几何（原版
// body.mini-mode #player .player-volume-popover：158×38、圆角 12，
// right calc(100% + 8px)、top 50% + translateY(-50%)）：右边贴着音量列
// 左侧 8px、与 43 高的 mini 栏垂直居中。
//
// 改成 AttachTo(音量列) 之后这份几何跟着锚点列走，不再是写死的窗口坐标
// （旧实现是 right 52 / bottom 2），所以断言从「音量键」的位置推出来。
func TestMiniVolumePopoverGeometry(t *testing.T) {
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.ShowVolume = true
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()

	vol, ok := tst.Find("音量") // 音量键贴在音量列左侧，可当锚点
	if !ok {
		t.Fatal("迷你窗里找不到音量键")
	}
	mute, ok := tst.Find("静音")
	if !ok {
		t.Fatal("音量弹层没打开（找不到静音键）")
	}
	// 弹层右边 = 音量列左边 - 8；静音键在弹层内再内缩 padding-left 8。
	if want := vol.X - miniVolumePopoverEdgeGap - 158 + volumePopoverPadL; mute.X != want {
		t.Errorf("静音键 X = %v，应为 %v（弹层右边贴音量列左侧 %v）",
			mute.X, want, miniVolumePopoverEdgeGap)
	}
	// 垂直居中于锚点列。mini 窗只有 155 高，框架的 4px 窗口边距会把弹层
	// 略微上推（构造上 over(p) > 0），所以留 4px 容差。
	if d := mute.Y + mute.H/2 - (vol.Y + vol.H/2); d > 4 || d < -4 {
		t.Errorf("静音键中心比音量列中心偏 %v，应基本居中", d)
	}
}
