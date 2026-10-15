package view

import (
	"os"
	"strings"
	"testing"
)

// 与 a11ystate_test.go 相同的 flatten 手法：读源码去掉空白后逐字匹配。
func flattenCover(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("home.go")
	if err != nil {
		t.Fatalf("读 home.go 失败：%v", err)
	}
	return strings.Join(strings.Fields(string(src)), "")
}

// TestCoverToggleBackgroundTransition 钉住封面小开关的背景色过渡接线。
// 原版 `.cover-mode-toggle { transition: opacity 0.2s ease, background 0.2s ease }`
// —— 背景色切换有 0.2s 过渡。mygo 用 AnimateColor 给 Background 加同样的过渡。
func TestCoverToggleBackgroundTransition(t *testing.T) {
	flat := flattenCover(t)
	// coverToggle 函数中 Background 的参数是 b.AnimateColor("bg", ...)。
	chain := strings.Join(strings.Fields(
		`b.Background(b.AnimateColor("bg",bg,ui.Animation{Duration:200*time.Millisecond,Ease:ui.CubicBezier(0.25,0.5,0.25,1)}))`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("coverToggle 背景色没有用 AnimateColor 加 0.2s 过渡：找不到 %q", chain)
	}
}

// TestCoverDiscShadowTransition 钉住唱片（disc）的阴影过渡接线。
// 原版 `#video-cover { transition: box-shadow 0.3s ease }` —— 三态投影切换有 0.3s 过渡。
// mygo 用 AnimateTo 过渡 y/blur、用 AnimateColor 过渡颜色。
func TestCoverDiscShadowTransition(t *testing.T) {
	flat := flattenCover(t)
	// disc 的阴影参数来自 coverShadowAnim(disc, cs)。
	chain := strings.Join(strings.Fields(
		`dy,blur,outer,inset:=coverShadowAnim(disc,cs)`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("唱片阴影没有用 coverShadowAnim 加 0.3s 过渡：找不到 %q", chain)
	}
	// coverShadowAnim 辅助函数本身必须存在，且用 AnimateTo + AnimateColor。
	animChain := strings.Join(strings.Fields(
		`func coverShadowAnim(el ui.Element, cs coverSurface) (dy, blur float32, outer, inset ui.Color)`), "")
	if !strings.Contains(flat, animChain) {
		t.Errorf("coverShadowAnim 辅助函数不存在或签名不对：找不到 %q", animChain)
	}
	// AnimateTo 用于 y/blur。
	yChain := strings.Join(strings.Fields(
		`el.AnimateTo("shadow-y",cs.Dy,anim)`), "")
	if !strings.Contains(flat, yChain) {
		t.Errorf("coverShadowAnim 没有用 AnimateTo 过渡 shadow-y：找不到 %q", yChain)
	}
	blurChain := strings.Join(strings.Fields(
		`el.AnimateTo("shadow-blur",cs.Blur,anim)`), "")
	if !strings.Contains(flat, blurChain) {
		t.Errorf("coverShadowAnim 没有用 AnimateTo 过渡 shadow-blur：找不到 %q", blurChain)
	}
	// AnimateColor 用于颜色。
	outerChain := strings.Join(strings.Fields(
		`el.AnimateColor("shadow-color",cs.Outer,anim)`), "")
	if !strings.Contains(flat, outerChain) {
		t.Errorf("coverShadowAnim 没有用 AnimateColor 过渡 shadow-color：找不到 %q", outerChain)
	}
	insetChain := strings.Join(strings.Fields(
		`el.AnimateColor("inset-shadow",cs.Inset,anim)`), "")
	if !strings.Contains(flat, insetChain) {
		t.Errorf("coverShadowAnim 没有用 AnimateColor 过渡 inset-shadow：找不到 %q", insetChain)
	}
}

// TestCoverSquareShadowTransition 钉住方形封面（sq）的阴影过渡接线。
// 原版 `#video-cover.cover-square { transition: box-shadow 0.3s ease }`。
func TestCoverSquareShadowTransition(t *testing.T) {
	flat := flattenCover(t)
	// sq 的阴影参数来自 coverShadowAnim(sq, cs)。
	chain := strings.Join(strings.Fields(
		`dy,blur,outer,inset:=coverShadowAnim(sq,cs)`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("方形封面阴影没有用 coverShadowAnim 加 0.3s 过渡：找不到 %q", chain)
	}
}
