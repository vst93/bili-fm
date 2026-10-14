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
