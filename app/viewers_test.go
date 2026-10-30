package main

import (
	"errors"
	"testing"
	"time"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/view"
)

func okViewers(n int64) *bilibili.OnlineViewers {
	return &bilibili.OnlineViewers{Count: n, ShowCount: true}
}

// TestOnlineOutcomeSuccess 钉住正常一轮：写新值、失败计数归零。
func TestOnlineOutcomeSuccess(t *testing.T) {
	out := onlineOutcomeFor(okViewers(152), nil, 2)
	if out.Value != 152 || out.KeepOld || out.Fails != 0 {
		t.Fatalf("outcome = %+v", out)
	}
}

// TestOnlineOutcomeTransientFailureKeepsOld 钉住「单轮失败留着旧数」：
// 一闪一闪地消失比短暂陈旧更扰人。
func TestOnlineOutcomeTransientFailureKeepsOld(t *testing.T) {
	err := errors.New("i/o timeout")
	for fails := 0; fails < staleAfterFailures-1; fails++ {
		out := onlineOutcomeFor(nil, err, fails)
		if !out.KeepOld {
			t.Fatalf("第 %d 轮失败应留旧数，outcome = %+v", fails+1, out)
		}
		if out.Value != 0 {
			t.Fatalf("留旧数时不能同时给新值: %+v", out)
		}
		if out.Fails != fails+1 {
			t.Fatalf("Fails = %d, want %d", out.Fails, fails+1)
		}
	}
}

// TestOnlineOutcomeStaleAfterRepeatedFailures 钉住「连着 staleAfterFailures
// 轮都失败就退回不知道」：不能让一个早就没人看的数一直挂着。
func TestOnlineOutcomeStaleAfterRepeatedFailures(t *testing.T) {
	out := onlineOutcomeFor(nil, errors.New("no such host"), staleAfterFailures-1)
	if out.KeepOld || out.Value != 0 || out.Fails < staleAfterFailures {
		t.Fatalf("连着失败后应清掉，outcome = %+v", out)
	}
}

// TestOnlineOutcomeHonorsShowSwitch 钉住 B 站关掉展示开关时不显示：
// 接口这时仍可能返回数字，但官方界面自己不展示。
func TestOnlineOutcomeHonorsShowSwitch(t *testing.T) {
	v := &bilibili.OnlineViewers{Count: 152, ShowCount: false}
	if out := onlineOutcomeFor(v, nil, 0); out.Value != 0 || out.KeepOld {
		t.Fatalf("开关关掉时不该显示: %+v", out)
	}
}

// TestOnlineOutcomeZeroMeansHidden 钉住「0 = 不知道 / 没人」→ 不显示。
func TestOnlineOutcomeZeroMeansHidden(t *testing.T) {
	if out := onlineOutcomeFor(okViewers(0), nil, 0); out.Value != 0 || out.KeepOld {
		t.Fatalf("count=0 应隐藏: %+v", out)
	}
}

// TestClearOnlineViewers 钉住换集时：数与归属 cid 一起清、请求代号换代
// （在途响应回来会因 reqID 不符被丢）、失败计数复位。
func TestClearOnlineViewers(t *testing.T) {
	a := &view.App{OnlineCount: 152, OnlineCid: 9}
	c := &controller{app: a, onlineReqID: 4, onlineFails: 2}
	c.clearOnlineViewers()
	if a.OnlineCount != 0 || a.OnlineCid != 0 {
		t.Fatalf("清完还剩 OnlineCount=%d OnlineCid=%d", a.OnlineCount, a.OnlineCid)
	}
	if c.onlineReqID != 5 {
		t.Fatalf("请求代号应换代: %d", c.onlineReqID)
	}
	if c.onlineFails != 0 {
		t.Fatalf("失败计数应复位: %d", c.onlineFails)
	}
}

// TestSetOnlineViewersSkipsNoop 钉住「值没变不重建」：这个数 30s 才动一次。
func TestSetOnlineViewersSkipsNoop(t *testing.T) {
	a := &view.App{OnlineCount: 152, OnlineCid: 9}
	c := &controller{app: a}
	c.setOnlineViewers(152, 9)
	if a.OnlineCount != 152 || a.OnlineCid != 9 {
		t.Fatalf("不该被改动: %+v", a)
	}
	c.setOnlineViewers(0, 0)
	if a.OnlineCount != 0 || a.OnlineCid != 0 {
		t.Fatalf("清成 0 应生效: %+v", a)
	}
}

// TestRefreshOnlineViewersNeedsWindow 钉住两个守卫：没有窗口（开发期 /
// 已销毁）不起请求，也没窗口时不 panic。
func TestRefreshOnlineViewersNeedsWindow(t *testing.T) {
	a := &view.App{}
	c := &controller{app: a}
	c.refreshOnlineViewers() // a.Win == nil → 直接返回
	if c.onlineReqID != 0 {
		t.Fatalf("没有窗口时不该发请求: reqID=%d", c.onlineReqID)
	}
	c.startOnlineViewersPoll()
	if c.onlinePollStop != nil {
		t.Fatal("没有窗口时不该起轮询")
	}
	c.stopOnlineViewersPoll() // 没起过也不该 panic
}

// TestOnlinePollInterval 钉住刷新周期与接口节奏匹配（网页端也是几十秒一档）。
func TestOnlinePollInterval(t *testing.T) {
	if onlineRefreshEvery <= 0 || onlineRefreshEvery > time.Minute {
		t.Fatalf("轮询间隔 = %v，应在 0 ~ 60s 之间", onlineRefreshEvery)
	}
	if staleAfterFailures < 2 {
		t.Fatal("至少要容忍一轮瞬时失败")
	}
}
