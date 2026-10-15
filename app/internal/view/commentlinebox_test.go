package view

import "testing"

// 本轮（弹幕/评论抽屉的评论行文本行盒）的测试。
//
// 原版 `main:src/components/danmakuList.tsx` 的评论 tab 里各段文本用的是
// Tailwind / HeroUI 的字号类，行高是**写死的**：
//
//	昵称 <span className="text-sm font-medium …">          → 14px / 20px
//	「Lv.N · 时间」<span className="text-xs …">             → 12px / 16px
//	点赞数 <div className="mt-1 text-xs …">                → 12px / 16px
//	楼中楼昵称 <span className="text-xs …">                → 12px / 16px
//	「查看更多回复...」<div className="text-xs …">          → 12px / 16px
//	「加载中...」<div className="… py-2 text-xs">           → 12px / 16px
//
// 框架把每一行的行盒夹在 `max(指定行高, ascent+descent)` 之上
// （mygo internal/text/layout.go:1027），14px 的自然行盒是 22、12px 是 18，
// 不显式压回时用户名两行就高 40（原版 36）——头像只有 32，整行被撑高 4px，
// 每条评论都差；点赞 / 「查看更多」 / 「加载中」 同理各高 2px。修法是
// `Height(原版行盒) + Translate(负 half-leading)`（见 linebox.go）。
//
// 验收：昵称盒高 20、meta/点赞/「查看更多」/楼中楼昵称/「加载中」盒高 16；
// 昵称与 meta 之间零间距（两行紧贴，原版 flex-col 无 gap）。
// 反证：去掉任一 Height/Translate，对应盒高会回到自然行盒 22 / 18。

// commentRowApp 在 commentwrap_test.go 里定义（正文 + 楼中楼预览）。

func TestCommentTextLineBoxes(t *testing.T) {
	a := commentRowApp("正文内容",
		Comment{User: "乙", Content: "回复一"},
		Comment{User: "丙", Content: "回复二"},
		Comment{User: "丁", Content: "回复三"},
	)
	a.Comments[0].User = "甲"
	a.Comments[0].Likes = 3
	tt := shellTesterSize(t, a, 800, 1400)

	cases := []struct {
		text string
		want float32
		why  string
	}{
		{"甲", 20, "昵称 text-sm 14/20（自然行盒 22）"},
		{replyMetaText(6, "刚刚"), 16, "「Lv.N · 时间」text-xs 12/16（自然 18）"},
		{"👍 3", 16, "点赞数 text-xs 12/16"},
		{"查看更多回复...", 16, "「查看更多回复...」text-xs 12/16"},
		{"乙:", 16, "楼中楼昵称 text-xs 12/16（有 Shrink(0)，单行）"},
	}
	for _, tc := range cases {
		r, ok := tt.Find(tc.text)
		if !ok {
			t.Fatalf("渲染帧里找不到 %q", tc.text)
		}
		if r.H < tc.want-0.5 || r.H > tc.want+0.5 {
			t.Errorf("%s 盒高 = %v，应 ≈%v —— %s", tc.text, r.H, tc.want, tc.why)
		}
	}

	// 昵称与 meta 紧贴（原版两行在同一个 flex-col、无 gap）。
	name, _ := tt.Find("甲")
	meta, _ := tt.Find(replyMetaText(6, "刚刚"))
	if d := meta.Y - name.Y; d < 19.5 || d > 20.5 {
		t.Errorf("meta 相对昵称的纵向偏移 = %v，应 ≈20（昵称行盒；两行无 gap）", d)
	}
	if name.H+meta.H != 36 {
		t.Errorf("昵称 + meta 两行合计 = %v，应 36（原版 20+16）", name.H+meta.H)
	}
}

func TestCommentLoadingLineBox(t *testing.T) {
	a := commentRowApp("正文内容")
	a.RepliesLoading = true
	tt := shellTesterSize(t, a, 800, 1400)

	r, ok := tt.Find("加载中...")
	if !ok {
		t.Fatal("渲染帧里找不到「加载中...」")
	}
	if r.H < 15.5 || r.H > 16.5 {
		t.Errorf("「加载中...」盒高 = %v，应 ≈16（原版 text-xs 12/16；自然行盒 18）", r.H)
	}
}
