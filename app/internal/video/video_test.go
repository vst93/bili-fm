package video

import (
	"os"
	"strings"
	"testing"
)

// TestPageURL 钉住弹窗页面地址里的起播参数：位置 / 倍速 / 静音。这些值决定
// 弹窗打开时的播放状态，改错一个就会「打开是暂停 / 位置不对 / 没声音」。
func TestPageURL(t *testing.T) {
	got := pageURL(Options{
		Src:     "http://127.0.0.1:4654/a?u=x",
		Title:   "标题",
		Poster:  "http://127.0.0.1:4654/p.png",
		StartAt: 12.5,
		Speed:   1.25,
		Muted:   true,
	})
	for _, want := range []string{
		"/video.html?",
		"t=12.50",
		"speed=1.25",
		"muted=1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("pageURL 缺少 %q：%s", want, got)
		}
	}

	// 倍速下限 0.5（0 / 负值不能原样传进去，否则页面 playbackRate=0 会卡住）。
	low := pageURL(Options{Speed: 0})
	if !strings.Contains(low, "speed=0.50") {
		t.Errorf("倍速未夹到下限：%s", low)
	}
	// 不静音时不带 muted 参数（页面 muted=false）。
	unmuted := pageURL(Options{Speed: 1})
	if strings.Contains(unmuted, "muted=1") {
		t.Errorf("未静音却带了 muted=1：%s", unmuted)
	}
}

// TestVideoPageAutoplayContract 钉住弹窗页面的起播策略（用户反馈「弹窗打开是
// 暂停状态」的回归点）：
//
//  1. 被策略拦下要退回**静音**起播（muted autoplay 各引擎都放行）；
//  2. **不能**在起播后自动取消静音——WebKit/Chromium 会把无手势的取消静音
//     判定为违规并直接暂停媒体，这正是旧实现停住的原因；
//  3. 改为在弹窗里第一次真实手势时再取消静音。
func TestVideoPageAutoplayContract(t *testing.T) {
	data, err := os.ReadFile("../../frontend/video.html")
	if err != nil {
		t.Fatalf("读不到 video.html：%v", err)
	}
	html := string(data)

	if !strings.Contains(html, "v.muted = true;") {
		t.Error("缺少静音起播的兜底路径（v.muted = true 后 play）")
	}
	if strings.Contains(html, "setTimeout(() => { v.muted = false; }, 900)") {
		t.Error("仍有起播后定时取消静音：无手势取消静音会被引擎暂停媒体")
	}
	if !strings.Contains(html, "addEventListener('pointerdown', onGesture") {
		t.Error("缺少手势取消静音（pointerdown）")
	}
	if !strings.Contains(html, "v.muted = false;") {
		t.Error("手势路径里没有取消静音")
	}
}

// TestVideoPageCloseButtonMatchesOriginal 钉住弹窗关闭键的样式契约。原版浮层
// 的 .player-video-close（main:src/styles/globals.css）是 38×38 正圆、墨色
// 45% 底，悬停转 70% 并放大 1.06、按下缩 .92；图标是 icon-park 的 Close
// （playerVideo.tsx 的 `<Close theme="outline" size="20" fill="#f1f5f9"/>`）。
// 原生版此前是一枚 26×22 圆角矩形、还塞在悬停才出现的 #bar 里，鼠标一移开
// 就消失。
func TestVideoPageCloseButtonMatchesOriginal(t *testing.T) {
	data, err := os.ReadFile("../../frontend/video.html")
	if err != nil {
		t.Fatalf("读不到 video.html：%v", err)
	}
	html := string(data)

	for _, want := range []string{
		"width: 38px; height: 38px",             // 原版 38×38
		"border-radius: 50%",                    // 正圆
		"background: rgba(15,23,42,.45)",        // 常态墨色 45%
		"background: rgba(15,23,42,.7)",         // 悬停墨色 70%
		"transform: scale(1.06)",                // 悬停放大
		"#close:active { transform: scale(.92)", // 按下缩 .92
		"top: 44px; right: 10px",                // 原版 body.video-open 的 36+8
		`d="M8 8L40 40"`,                        // icon-park Close 两条 path
		`d="M8 40L40 8"`,
		`stroke="#f1f5f9"`,       // 图标色
		`width="20" height="20"`, // 原版 size=20
		`stroke-width="4"`,       // icon-park 48 画布原始描边
	} {
		if !strings.Contains(html, want) {
			t.Errorf("关闭键缺少 %q（原版 .player-video-close）", want)
		}
	}

	// 关闭键必须常驻：不能还留在 #bar 里（那条只在 #wrap:hover 时显形）。
	bar := strings.Index(html, `<div id="bar">`)
	if bar < 0 {
		t.Fatal("找不到 #bar")
	}
	end := strings.Index(html[bar:], "</div>")
	if end < 0 {
		t.Fatal("#bar 未闭合")
	}
	if strings.Contains(html[bar:bar+end], `id="close"`) {
		t.Error("关闭键仍在 #bar 里，会随标题条一起隐去")
	}
}

// TestVideoPageCloseTitle 钉住弹窗上关闭键的工具提示：原版视频浮层
// （playerVideo.tsx 的 .player-video-close）是 title="关闭视频"，原生版此前
// 写的是「关闭」——悬停时看不出关的是视频还是整个播放。
func TestVideoPageCloseTitle(t *testing.T) {
	data, err := os.ReadFile("../../frontend/video.html")
	if err != nil {
		t.Fatalf("读不到 video.html：%v", err)
	}
	html := string(data)
	if !strings.Contains(html, `id="close" title="关闭视频"`) {
		t.Error(`关闭键应带 title="关闭视频"（原版 playerVideo 的写法）`)
	}
	if strings.Contains(html, `id="close" title="关闭"`) {
		t.Error("关闭键仍是「关闭」（漏改？）")
	}
}
