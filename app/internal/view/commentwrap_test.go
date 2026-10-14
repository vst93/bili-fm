package view

import (
	"strings"
	"testing"
)

// 本轮（评论正文 / 楼中楼正文的行数上限）的测试。
//
// 原版评论列表（`main:src/components/danmakuList.tsx`）里两条正文都是普通流式
// 文本，没有任何行数截断：
//
//	<p className="text-sm leading-relaxed text-default-600 dark:text-default-400">
//	  {reply.content?.message || ""}
//	</p>
//	…
//	<span className="ml-1 text-default-600 dark:text-default-400">
//	  {subReply.content?.message || ""}
//	</span>
//
// 全局 CSS 只给 `#video-title` / `#video-desc` / `.mini-title` 挂了
// `-webkit-line-clamp`，评论正文不在其中 —— 长评论整段铺开、行高由内容决定。
// 原生版此前给正文加了 `MaxLines(10)`、楼中楼正文加了 `MaxLines(4)`，超长评论
// 会被硬截到 10 行 / 4 行（实测 220px / 72px 封顶），与原版对不上。

// commentRowApp 造一个打开评论 tab 的窗口，塞一条正文为 body、楼中楼预览为
// replies 的评论（其余无关状态保持最小）。
func commentRowApp(body string, replies ...Comment) *App {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = DrawerDanmaku
	a.DanmakuTab = TabReply
	a.Comments = []Comment{{
		User: "甲", Time: "刚刚", Level: 6,
		Content: body, Replies: replies,
	}}
	return a
}

// textHeight 渲染一份 App 并返回给定文案的包围盒高度。窗口放高，保证长正文
// 整段落在视口内（否则量到的是视口裁剪后的高）。
func textHeight(t *testing.T, a *App, s string) float32 {
	t.Helper()
	tt := shellTesterSize(t, a, 800, 1400)
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("找不到文案（len=%d）", len(s))
	}
	return r.H
}

// bodyHeight 渲染一份评论并返回正文文案的包围盒高度。
func bodyHeight(t *testing.T, body string, replies ...Comment) float32 {
	t.Helper()
	return textHeight(t, commentRowApp(body, replies...), body)
}

// TestCommentBodyNotLineClamped 正文不设行数上限：正文翻倍，块高必须跟着长。
// 若正文被 `MaxLines(10)` 截住，两份长度不同的正文会停在同一个 10 行高度上
// （实测 220px），断言随即变红。
func TestCommentBodyNotLineClamped(t *testing.T) {
	unit := "这是一段很长的评论文本，用来测试换行行为。"
	short := strings.Repeat(unit, 20) // ≈10 行
	long := strings.Repeat(unit, 40)  // ≈20 行
	hs, hl := bodyHeight(t, short), bodyHeight(t, long)
	t.Logf("正文 20 段高 %v，40 段高 %v", hs, hl)
	if hl <= hs {
		t.Errorf("正文块高没随内容增长（20 段 %v / 40 段 %v）—— 仍被行数上限截断", hs, hl)
	}
}

// TestReplyBodyNotLineClamped 楼中楼正文同理：原版 `<span>` 不截断，超长回复整段
// 换行。被 `MaxLines(4)` 截住时两份长度不同的回复都会停在 4 行高度（实测 72px）。
func TestReplyBodyNotLineClamped(t *testing.T) {
	unit := "楼中楼很长的回复内容重复很多遍。"
	short := strings.Repeat(unit, 10) // ≈4 行
	long := strings.Repeat(unit, 30)  // ≈12 行
	hs := textHeight(t, commentRowApp("正文", Comment{User: "乙", Content: short}), short)
	hl := textHeight(t, commentRowApp("正文", Comment{User: "乙", Content: long}), long)
	t.Logf("楼中楼 10 段高 %v，30 段高 %v", hs, hl)
	if hl <= hs {
		t.Errorf("楼中楼块高没随内容增长（10 段 %v / 30 段 %v）—— 仍被行数上限截断", hs, hl)
	}
}
