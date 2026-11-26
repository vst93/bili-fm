package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/view"
)

func commentPage(text string) *bilibili.ReplyList {
	return &bilibili.ReplyList{
		Items:   []bilibili.ReplyItem{{Content: bilibili.ReplyContent{Message: text}}},
		HasMore: true, TotalCount: 40,
	}
}

func TestCommentsFirstPageFailureAndRetry(t *testing.T) {
	a := view.NewApp(func() {})
	a.Info = &view.Info{Aid: 123}
	c := &controller{app: a}
	id, ok := c.beginCommentsLoad(123, 1)
	if !ok || !a.RepliesLoading {
		t.Fatal("首屏没有开始请求")
	}
	if _, ok := c.beginCommentsLoad(123, 1); ok {
		t.Fatal("同一视频首屏并发重复请求")
	}
	c.finishCommentsLoad(id, 123, 1, nil, errors.New("评论接口返回 412，请稍后重试"))
	if a.RepliesLoading || !a.RepliesLoadErr || !strings.Contains(a.RepliesError, "412") {
		t.Fatalf("首屏失败被吞掉：loading=%v error=%q", a.RepliesLoading, a.RepliesError)
	}
	id, ok = c.beginCommentsLoad(123, 1)
	if !ok || !a.RepliesLoading || a.RepliesLoadErr || a.RepliesError != "" {
		t.Fatal("失败后无法重新请求首屏")
	}
	c.finishCommentsLoad(id, 123, 1, commentPage("首屏评论"), nil)
	if a.RepliesLoading || a.RepliesLoadErr || len(a.Comments) != 1 || a.Comments[0].Content != "首屏评论" || a.ReplyOid != 123 || a.ReplyPage != 1 {
		t.Fatalf("重试成功未落地：loading=%v error=%v comments=%v", a.RepliesLoading, a.RepliesLoadErr, a.Comments)
	}
	// 续页失败保留已加载评论与页码，重试成功才递增。
	id, ok = c.beginCommentsLoad(123, 2)
	if !ok {
		t.Fatal("无法加载续页")
	}
	c.finishCommentsLoad(id, 123, 2, nil, errors.New("timeout"))
	if a.ReplyPage != 1 || len(a.Comments) != 1 || !a.RepliesLoadErr {
		t.Fatal("续页失败丢失原内容或重试状态")
	}
	id, _ = c.beginCommentsLoad(123, 2)
	c.finishCommentsLoad(id, 123, 2, commentPage("下一页"), nil)
	if a.ReplyPage != 2 || len(a.Comments) != 2 || a.RepliesLoadErr {
		t.Fatal("续页重试没有恢复")
	}
}

func TestCommentsOldRequestDoesNotBlockNewVideo(t *testing.T) {
	a := view.NewApp(func() {})
	a.Info = &view.Info{Aid: 1}
	c := &controller{app: a}
	old, _ := c.beginCommentsLoad(1, 1)
	a.Info = &view.Info{Aid: 2}
	current, ok := c.beginCommentsLoad(2, 1)
	if !ok {
		t.Fatal("旧视频在途请求挡住新视频首屏")
	}
	c.finishCommentsLoad(old, 1, 1, commentPage("旧视频"), nil)
	if !a.RepliesLoading || len(a.Comments) != 0 {
		t.Fatal("旧请求清掉了新请求的加载态或写入旧内容")
	}
	c.finishCommentsLoad(current, 2, 1, commentPage("新视频"), nil)
	if a.RepliesLoading || len(a.Comments) != 1 || a.Comments[0].Content != "新视频" {
		t.Fatal("新视频评论未落地")
	}
	// A→B→A：目标 aid 相同也不能接受第一轮慢响应。
	a.Info = &view.Info{Aid: 1}
	current, _ = c.beginCommentsLoad(1, 1)
	c.finishCommentsLoad(old, 1, 1, nil, errors.New("late failure"))
	if !a.RepliesLoading || a.RepliesLoadErr {
		t.Fatal("旧的同目标响应污染新请求")
	}
	c.finishCommentsLoad(current, 1, 1, commentPage("新一轮"), nil)
	if a.RepliesLoading || a.Comments[0].Content != "新一轮" {
		t.Fatal("新一轮请求未正常落地")
	}
}

func TestCommentsEmptySuccessAndMissingResponse(t *testing.T) {
	a := view.NewApp(func() {})
	a.Info = &view.Info{Aid: 1}
	c := &controller{app: a}
	id, _ := c.beginCommentsLoad(1, 1)
	c.finishCommentsLoad(id, 1, 1, &bilibili.ReplyList{}, nil)
	if a.RepliesLoading || a.RepliesLoadErr || a.ReplyOid != 1 || len(a.Comments) != 0 {
		t.Fatal("真正的空结果应正常结束")
	}
	id, _ = c.beginCommentsLoad(1, 1)
	c.finishCommentsLoad(id, 1, 1, nil, nil)
	if a.RepliesLoading || !a.RepliesLoadErr {
		t.Fatal("缺失响应应进入可重试错误态")
	}
}
