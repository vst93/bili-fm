package view

import (
	"math"
	"testing"
)

// TestScrollbarThumbAlpha 钉住滚动条拇指的不透明度取自原版**生效**的那条声明。
//
// 原版 globals.css 写了两条：
//
//	* { scrollbar-color: rgba(100,116,139,.35) transparent }      // Firefox 专用
//	*::-webkit-scrollbar-thumb { background: rgba(100,116,139,.3) } // WebView 实际生效
//
// Tauri 两端都是 WebView（Windows WebView2/Chromium、Linux WebKitGTK），渲染的是
// .3 那条。此前原生版抄了 .35，深 5% alpha。
func TestScrollbarThumbAlpha(t *testing.T) {
	th := Theme{}.uiTheme()
	if got := th.ScrollbarWidth; got != 6 {
		t.Fatalf("滚动条宽度 = %v，原版 ::-webkit-scrollbar { width: 6px }", got)
	}
	// ui.Color 的 A 是 8 位直通 alpha。
	got := float64(th.Scrollbar.A) / 255
	if math.Abs(got-0.30) > 0.01 {
		t.Fatalf("滚动条拇指 alpha = %.3f，原版 ::-webkit-scrollbar-thumb 是 .30", got)
	}
	// 反证：Firefox 的 scrollbar-color（.35）对不上。
	if math.Abs(got-0.35) <= 0.01 {
		t.Fatalf("滚动条拇指仍是 Firefox 那条的 .35，WebView 实际渲染是 .30")
	}
}
