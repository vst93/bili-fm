package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 播放栏按钮（#player .player-button）的按压手感回归。之前六颗键都错用了
// `.nav-icon-btn` 的 pressPose：悬停凭空上浮 1px、按下缩到 .94 且曲线是
// 120ms/EaseOut。原版这些键：
//
//	transition: ... transform 120ms cubic-bezier(0.23, 1, 0.32, 1);
//	:hover     （无 transform）
//	:active    { transform: scale(0.93); box-shadow: inset 0 1px 3px rgba(15,23,42,.1) }
//	:active::before { background-color: --liquid-bg-active /* 白 .16 */ }
func TestPlayerButtonPressTokens(t *testing.T) {
	if got := playerButtonPressPose(false); got != (ui.Pose{}) {
		t.Errorf("未按下 = %+v，应为零位移（常态/悬停都不动）", got)
	}
	if got := playerButtonPressPose(true); got != (ui.Pose{Scale: 0.93}) {
		t.Errorf("按下 = %+v，应为 scale .93", got)
	}
	// 与 `.nav-icon-btn` 的 .94 区分开（两族缩放值不同）。
	if playerButtonPressPose(true).Scale == navIconPoseFor(true, false).Scale {
		t.Errorf("按下缩放 .93 与 .nav-icon-btn 的 .94 相同了")
	}

	if playerButtonPressAnim.Duration != 120*time.Millisecond {
		t.Errorf("位移时长 = %v，应为 120ms（原版 transform 120ms）", playerButtonPressAnim.Duration)
	}
	if playerButtonPressAnim.Spring != (ui.SpringOptions{}) {
		t.Errorf("应为曲线动画，不是弹簧：%+v", playerButtonPressAnim.Spring)
	}
	if playerButtonPressAnim.Ease == nil {
		t.Fatal("缺少缓动曲线")
	}
	want := ui.CubicBezier(0.23, 1, 0.32, 1)
	for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
		if d := math.Abs(float64(playerButtonPressAnim.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("缓动在 %.2f 处 = %v, want %v", x, playerButtonPressAnim.Ease(x), want(x))
		}
	}
	// 与立方缓出 EaseOut（1-(1-t)^3）要能区分，证明用的是原版贝塞尔。
	if math.Abs(float64(playerButtonPressAnim.Ease(0.25)-ui.EaseOut(0.25))) < 1e-3 {
		t.Errorf("缓动与 EaseOut 相同，未用原版 cubic-bezier(0.23,1,0.32,1)")
	}
}

// TestPlayerPressMaterialTokens 钉住按下材质（原版 :active）：
// 底色 --liquid-bg-active（白 .16）、内压 inset 0 1px 3px rgba(15,23,42,.1)。
func TestPlayerPressMaterialTokens(t *testing.T) {
	if got := playerPressFill(); got != ui.Hex("#ffffff").Alpha(0.16) {
		t.Errorf("按下底色 = %v，应为白 .16（--liquid-bg-active）", got)
	}
	specs := playerPressShadowSpecs()
	if len(specs) != 1 {
		t.Fatalf("按下内压阴影应为 1 条，得到 %d", len(specs))
	}
	s := specs[0]
	if s.X != 0 || s.Y != 1 || s.Blur != 3 || s.Spread != 0 || s.C != ui.Hex("#0f172a").Alpha(0.10) {
		t.Errorf("按下内压 = %+v，应为 inset 0 1px 3px 墨 .1", s)
	}
	// 与 `.nav-icon-btn:active` 的 `inset 0 2px 4px 墨 .08` 不同族。
	nav := pressShadowSpecs()
	if len(nav) == 1 && s == nav[0] {
		t.Errorf("播放栏按下内压与 .nav-icon-btn 的相同了，应各自成族")
	}
}

// playerPressRender 渲染一条播放栏（白底），把指针移过或按在目标按钮上。
func playerPressRender(t *testing.T, label, action string) (*image.RGBA, ui.Rect) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Queue = []Track{{Bvid: "BV1", Cid: 1, Title: "t"}, {Bvid: "BV2", Cid: 2, Title: "t2"}}
	a.Index = 0
	a.Track = &a.Queue[0]
	tt := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#ffffff"))
		a.playerBar(c)
	}, 800, 200)
	tt.Frame()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("找不到按钮 %q", label)
	}
	switch action {
	case "press":
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
	case "hover":
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
	}
	tt.Frame()
	return tt.Image(), r
}

// inkTopIn 在按钮矩形内找最上面一行「图标核心」像素。阈值取 120：
// #334155（常态，亮度 63）与 #0284c7（悬停，亮度 101）都低于它，而按钮的
// 白填充 / slate 描边 / 蓝调悬停投影（亮度 ≥ 200）都被排除。这样图标即使
// 悬停变色也能量到，且不受悬停投影向上蔓延的干扰。
//
// 一行至少要有 2 个暗像素才算「图标开始」：图标最上沿的抗锯齿行只有 1 个
// 半透明像素，它的亮度会随图标颜色变（播放键 PlayOne 顶行常态 111 < 120，
// 悬停的亮蓝 #0284c7 下是 123 > 120）—— 按「有 1 个就算」会把这种抗锯齿
// 差异误读成「图标下移了 1px」。要求 2 个暗像素后，两种颜色都在同一行达标。
func inkTopIn(img *image.RGBA, r ui.Rect) int {
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		dark := 0
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := img.PixOffset(x, y)
			lum := (299*int(img.Pix[i]) + 587*int(img.Pix[i+1]) + 114*int(img.Pix[i+2])) / 1000
			if lum < 120 {
				dark++
			}
		}
		if dark >= 2 {
			return y
		}
	}
	return -1
}

// TestPlayerButtonsDontLiftOnHover 无头实测：播放栏按钮悬停**不上浮**
// （原版 `#player .player-button:hover` 没有 transform）。此前它们错用 pressPose
// 会上浮 1px，图标随之抬高一行（实测「下一个视频」22 → 21）。
func TestPlayerButtonsDontLiftOnHover(t *testing.T) {
	for _, label := range []string{"播放", "下一个视频", "音量", "播放速度 1 倍", "音量均衡: 关"} {
		t.Run(label, func(t *testing.T) {
			rest, rect := playerPressRender(t, label, "rest")
			hover, _ := playerPressRender(t, label, "hover")
			restTop, hoverTop := inkTopIn(rest, rect), inkTopIn(hover, rect)
			t.Logf("%s：静止图标 top=%d、悬停图标 top=%d", label, restTop, hoverTop)
			if restTop < 0 || hoverTop < 0 {
				t.Fatalf("量不到图标核心像素：rest=%d hover=%d", restTop, hoverTop)
			}
			if hoverTop != restTop {
				t.Errorf("悬停导致图标位移（top %d → %d），原版播放栏悬停不上浮", restTop, hoverTop)
			}
		})
	}
}

// TestPlayerPressSurfaceWired 无头实测：按下「下一集」时按钮矩形里大量像素变化
// （底色转 --liquid-bg-active + 内压阴影 + 缩放），反证 :active 材质确实接上了。
func TestPlayerPressSurfaceWired(t *testing.T) {
	hover, rect := playerPressRender(t, "下一个视频", "hover")
	press, _ := playerPressRender(t, "下一个视频", "press")
	diff, total := diffInRect(hover, press, rect)
	t.Logf("下一集按下：%d/%d 像素随之变化", diff, total)
	if diff < total/10 {
		t.Errorf("只有 %d/%d 像素受按下影响（<10%%）—— :active 材质没接上？", diff, total)
	}
}
