package view

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func paginationComments(start, count int) []Comment {
	out := make([]Comment, count)
	for i := range out {
		n := start + i
		out[i] = Comment{User: "用户", Content: fmt.Sprintf("分页评论%03d", n), SendTime: int64(10000 - n)}
	}
	return out
}

func commentsPaginationTester(t *testing.T, a *App) *ui.Tester {
	t.Helper()
	a.Drawer, a.DanmakuTab = DrawerDanmaku, TabReply
	return settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
}

// API 按热度分页，下一页的发布时间可能更新；新页不能插入正在阅读的旧页。
func TestCommentPagesKeepServerOrder(t *testing.T) {
	a := NewApp(func() {})
	a.Comments = []Comment{
		{User: "甲", Content: "第一页热评", SendTime: 10},
		{User: "乙", Content: "第二页新评论", SendTime: 20},
	}
	tst := commentsPaginationTester(t, a)
	first, ok1 := tst.Find("第一页热评")
	second, ok2 := tst.Find("第二页新评论")
	if !ok1 || !ok2 || first.Y >= second.Y {
		t.Fatalf("续页不能重排已加载评论：第一页=%+v 第二页=%+v", first, second)
	}
}

// 加载提示的出现/消失不应改变滚动范围；包括惯性把视口推到提示行后末页为空。
func TestCommentLoadingFooterKeepsViewport(t *testing.T) {
	a := NewApp(func() {})
	a.Comments = paginationComments(0, 40)
	tst := commentsPaginationTester(t, a)
	a.RepliesLoading = true
	tst.Frame()
	tst.Scroll(490, 400, 0, 100000)
	tst.Frame()
	first, _ := a.repliesList.Visible()
	text := a.Comments[first+1].Content
	before, ok := tst.Find(text)
	if !ok {
		t.Fatalf("missing anchor %q", text)
	}
	for _, state := range []string{"idle", "error", "loading", "idle"} {
		a.RepliesLoading, a.RepliesLoadErr = state == "loading", state == "error"
		for frame := 0; frame < 3; frame++ {
			tst.Frame()
			after, ok := tst.Find(text)
			if !ok || math.Abs(float64(after.Y-before.Y)) > 0.5 {
				t.Fatalf("%s 第 %d 帧视口跳动：%q y=%v→%v, visible=%v", state, frame, text, before.Y, after.Y, ok)
			}
		}
	}
}

// 封顶后追加与裁头同时发生，总行数不变。必须等新布局完成再检查底部，
// 否则读取上一帧的 Visible 会在响应落地当帧立即再发下一页。
func TestCommentPageArrivalKeepsAnchorAndWaitsForScroll(t *testing.T) {
	for _, capped := range []bool{false, true} {
		t.Run(fmt.Sprintf("capped=%v", capped), func(t *testing.T) {
			a := NewApp(func() {})
			n := 40
			if capped {
				n = 240
			}
			a.Comments = paginationComments(0, n)
			// 评论与楼中楼会造成不同行高，不能只验证等高行的索引。
			for i := range a.Comments {
				for j := 0; j < i%3; j++ {
					a.Comments[i].Replies = append(a.Comments[i].Replies, Comment{User: "楼中楼", Content: "一条回复"})
				}
			}
			a.RepliesHasMore, a.ReplyPage = true, 1
			calls := 0
			a.Act.LoadComments = func(page int) { calls++; a.RepliesLoading = true }
			tst := commentsPaginationTester(t, a)
			tst.Scroll(490, 400, 0, 100000)
			tst.Frame()
			if calls != 1 {
				t.Fatalf("expected first request, calls=%d", calls)
			}
			// 请求在途仍可继续滚到底。
			tst.Scroll(490, 400, 0, 100000)
			tst.Frame()
			first, _ := a.repliesList.Visible()
			text := a.Comments[first+1].Content
			before, ok := tst.Find(text)
			if !ok {
				t.Fatalf("missing anchor %q", text)
			}
			next := paginationComments(n, 20)
			for i := range next {
				next[i].SendTime = int64(20000 + i) // 热度更低但发布时间更新。
			}
			if capped {
				a.Comments = a.Comments[20:]
			}
			a.Comments = append(append([]Comment(nil), a.Comments...), next...)
			a.RepliesLoading, a.ReplyPage = false, 2
			for frame := 0; frame < 4; frame++ {
				// Tester 的时钟冻结，显式推进到下一显示帧。
				tst.Advance(16 * time.Millisecond)
				after, ok := tst.Find(text)
				if !ok || math.Abs(float64(after.Y-before.Y)) > 0.5 {
					t.Fatalf("响应后第 %d 帧锚点跳动：%q y=%v→%v visible=%v", frame, text, before.Y, after.Y, ok)
				}
				if calls != 1 {
					t.Fatalf("未滚动就自动连翻，frame=%d calls=%d", frame, calls)
				}
			}
			tst.Scroll(490, 400, 0, 100000)
			tst.Frame()
			if calls != 2 {
				t.Fatalf("继续下滚应再翻一页，calls=%d", calls)
			}
		})
	}
}

func TestCommentFirstPageFailureCanRetry(t *testing.T) {
	a := NewApp(func() {})
	a.RepliesLoadErr = true
	calls := 0
	a.Act.LoadComments = func(page int) {
		if page != 1 {
			t.Fatalf("首屏重试应请求第一页：%d", page)
		}
		calls++
		a.RepliesLoadErr, a.RepliesLoading = false, true
	}
	tst := commentsPaginationTester(t, a)
	if tst.HasText("暂无评论") {
		t.Error("请求失败不能冒充暂无评论")
	}
	if err := tst.Click("加载失败，点击重试"); err != nil {
		t.Fatal(err)
	}
	tst.Frame()
	if calls != 1 || !tst.HasText("评论加载中…") {
		t.Fatalf("重试没有进入首屏加载态：calls=%d", calls)
	}
}
