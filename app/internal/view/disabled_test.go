package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestDisabledOpacityTargets 钉住各族「禁用时该落到画布上的绝对值」：
// `.nav-icon-btn:disabled` = .34、`#player .player-button:disabled` = .42、
// `#player input[type="range"]:disabled` = .42、`.watchlater-remove-btn:disabled`
// = .55、`.mini-window-controls button:disabled` = .34。
//
// mygo 0.8.0 起用 `DimDisabled(false)` 关掉框架对禁用节点的 0.5 减半，所以这些
// 常量**就是**写在元素上的值（见 disabled.go 的总说明）。
func TestDisabledOpacityTargets(t *testing.T) {
	if navIconDisabledOpacity != 0.34 {
		t.Errorf("nav 禁用绝对值 = %v，应为 .34", navIconDisabledOpacity)
	}
	if playerButtonDisabledOpacity != 0.42 {
		t.Errorf("播放按钮禁用绝对值 = %v，应为 .42", playerButtonDisabledOpacity)
	}
	if progressDisabledOpacity != 0.42 {
		t.Errorf("进度条禁用绝对值 = %v，应为 .42", progressDisabledOpacity)
	}
	if miniControlDisabledOpacity != 0.34 {
		t.Errorf("迷你窗控制键禁用绝对值 = %v，应为 .34", miniControlDisabledOpacity)
	}
	if got := wlRemoveOpacity(true); got != 0.55 {
		t.Errorf("稍后再看移除键请求中透明度 = %v，应为 .55", got)
	}
	if got := wlRemoveOpacity(false); got != 1 {
		t.Errorf("稍后再看移除键常态透明度 = %v，应为 1", got)
	}
}

// TestNavIconOpacityTarget 钉住 `.nav-icon-btn` 的常/禁用两档目标值。
func TestNavIconOpacityTarget(t *testing.T) {
	if got := navIconOpacityTarget(true, 0.82); got != navIconDisabledOpacity {
		t.Errorf("nav 禁用目标 = %v，应为 %v（原版绝对值，不再补偿）", got, navIconDisabledOpacity)
	}
	if got := navIconOpacityTarget(false, 0.82); got != 0.82 {
		t.Errorf("nav 常态目标 = %v，应为 .82", got)
	}
}

// TestSetDisabledOpacityWritesAbsoluteValue 是**绘制实测**：`setDisabledOpacity`
// 落到画布上的不透明度必须等于原版的绝对值。
//
// 白底上一块纯黑方块，不透明度 α 时像素亮度 = 255×(1−α)。反证：若忘了
// `DimDisabled(false)`，框架会把整棵子树再乘 .5，.34 会变成 .17 —— 这正是
// 0.7.x 里那段「写 target/0.5」补偿存在的理由。
func TestSetDisabledOpacityWritesAbsoluteValue(t *testing.T) {
	for _, tc := range []struct {
		name   string
		off    bool
		target float32
		want   float32 // 期望落到画布上的整体不透明度
	}{
		{"nav 禁用", true, navIconDisabledOpacity, 0.34},
		{"播放按钮禁用", true, playerButtonDisabledOpacity, 0.42},
		{"稍后再看移除键请求中", true, wlRemoveOpacity(true), 0.55},
		{"未禁用", false, navIconDisabledOpacity, 1},
	} {
		off, target, want := tc.off, tc.target, tc.want
		img := renderedDisabledProbe(t, off, target)
		// 方块居中铺满 64×64 的白色画布。
		got := 1 - float32(lum(pixelAt(t, img, 32, 32)))/255
		if abs(float64(got-want)) > 0.02 {
			t.Errorf("%s：落到画布上的不透明度 = %.3f，应为 %.2f（忘了 DimDisabled(false) 会再乘 .5）",
				tc.name, got, want)
		}
	}
}

// renderedDisabledProbe 在白底上渲染一块 40×40 纯黑方块，套上 setDisabledOpacity。
func renderedDisabledProbe(t *testing.T, off bool, target float32) *image.RGBA {
	t.Helper()
	tst := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Background(ui.Hex("#ffffff"))
		b := ui.ButtonBase(c.Key("probe")).Size(40, 40).Background(ui.Hex("#000000"))
		setDisabledOpacity(b, off, target)
	}, 64, 64)
	tst.Frame()
	return tst.Image()
}

// TestSearchSubmitKeepsFullOpacity 钉住搜索提交键这条**与不透明度无关**的例外：
// 原版 `.search-submit-btn:disabled` 不改不透明度（仍是常态度 .82），而且照样
// 吃 `:hover`。mygo 的 Hovered() 对禁用元素恒假，所以这颗键仍不能走框架的
// Disabled —— 只能在回调里挡（见 home.go searchField 的注释）。
func TestSearchSubmitKeepsFullOpacity(t *testing.T) {
	if got := topToolStyleFor(topToolSubmit, false, false).Opacity; got <= 0.5 {
		t.Errorf("搜索提交键常态度 = %v，应 > .5（原版 disabled 不改不透明度，故不能用框架 Disabled 的置灰）", got)
	}
}
