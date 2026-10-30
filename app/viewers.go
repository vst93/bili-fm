package main

// 当前观看人数（右栏 UP 主行行尾的「N 人在看」）。
//
// 数据来自 x/player/online/total —— 就是 B 站网页播放器里那个「N 人在看」气泡
// 的接口（见 internal/bilibili/online.go）。它与右栏已有的 View（累计播放量）是
// 两个不同的数：View 起播时随详情一次性拿到，online/total 的 count 是**此刻**
// 正在看这一集的人数，会实时涨落，所以要周期刷新。
//
// 为什么用轮询：B 站没有把这个数走长连接（弹幕有 WebSocket，但那是消息流，
// 不等于在线人数）。30s 是权衡值 —— 网页端自己也是几十秒一档，再快是白打接口，
// 再慢这个数字看起来就像死的。
//
// 生命周期跟「正在播放的某一集」绑：
//   - 起播 / 换集 → 立刻拉一次（不等下一个 tick）；clearPlaybackExtras 里先把
//     上一集的数清掉，所以界面上不会出现「新视频挂旧人数」；
//   - 每 30s → 只对当前这一集再拉一次；没有曲目（队列空 / 已停）就不打接口，
//     界面上的数也跟着消失。
//
// 并发与陈旧：读写界面状态一律在主线程跑（起播那一枪在 startCurrent 的
// Win.Update 闭包里，轮询经 Win.Update 转进来），请求发在后台 goroutine；响应
// 回来用 reqID + cid 双重校验 —— 与 player_extras.go 同一套路。怎么解读一次
// 响应（失败要不要留旧数、展示开关关掉怎么办）全部集中在 onlineOutcomeFor
// 这个纯函数里，便于无头测试。

import (
	"log"
	"time"

	"github.com/vst93/bili-fm/app/internal/bilibili"
)

const (
	// onlineRefreshEvery 是轮询间隔。
	onlineRefreshEvery = 30 * time.Second
	// staleAfterFailures 是「连续拉不到多少轮后就不再显示旧数」。3 轮 ≈ 90s：
	// 比一个周期长，又不至于让一个早就没人看的数一直挂着。
	staleAfterFailures = 3
)

// onlineOutcome 是一次轮询响应该落成什么样的界面状态。
type onlineOutcome struct {
	// Value 是要写进界面的数；0 = 不显示。
	Value int64
	// KeepOld 为真表示这次失败但旧数还留着（界面上不动）。
	KeepOld bool
	// Fails 是新的连续失败轮数。
	Fails int
}

// onlineOutcomeFor 把「接口结果 + 上一轮的失败计数」折成界面决策。
//
//   - 出错：没超过 staleAfterFailures 就留着上一轮的数（这个数字本来就是
//     几十秒一档的实时量，一闪一闪地消失比短暂陈旧更扰人）；超了就认输。
//   - B 站把 show_switch.count 关掉，或当下没人（count ≤ 0）：不显示。
//   - 正常：写新值，失败计数归零。
func onlineOutcomeFor(v *bilibili.OnlineViewers, err error, fails int) onlineOutcome {
	if err != nil {
		fails++
		if fails < staleAfterFailures {
			return onlineOutcome{KeepOld: true, Fails: fails}
		}
		return onlineOutcome{Fails: fails}
	}
	if v == nil || !v.ShowCount || v.Count <= 0 {
		return onlineOutcome{}
	}
	return onlineOutcome{Value: v.Count}
}

// startOnlineViewersPoll 起 30s 轮询，由 WhenReady 在主窗建好后调一次。
func (c *controller) startOnlineViewersPoll() {
	a := c.app
	if a == nil || a.Win == nil || c.onlinePollStop != nil {
		return
	}
	stop := make(chan struct{})
	c.onlinePollStop = stop
	go func() {
		t := time.NewTicker(onlineRefreshEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
			}
			// refreshOnlineViewers 读写界面状态，放回主线程跑（与用户点菜单
			// 走的是同一条路径）。
			a.Win.Update(func() { c.refreshOnlineViewers() })
		}
	}()
}

// stopOnlineViewersPoll 停掉轮询（退出时调用，免得关窗路上还在打接口）。
func (c *controller) stopOnlineViewersPoll() {
	if c.onlinePollStop != nil {
		close(c.onlinePollStop)
		c.onlinePollStop = nil
	}
}

// refreshOnlineViewers 拉一次当前曲目的在线人数。**必须在主线程调用**，
// 且在一个更新回合里（写状态靠该回合结束后框架自己重绘，这里不再嵌套
// Win.Update）。
func (c *controller) refreshOnlineViewers() {
	a := c.app
	if a == nil || a.Win == nil {
		return
	}
	t := currentExtrasTrack(a)
	if t == nil || t.Cid == 0 {
		// 没有正在播的曲目：数跟着曲目一起消失。
		c.setOnlineViewers(0, 0)
		return
	}
	// 主窗不可见（收到托盘 / 迷你模式 / 最小化）时不打接口：这一栏只在主窗
	// 右栏画，后台挂着音乐时没人看这个数。窗口回到可见后，下一个 tick
	// （≤30s）就补上；界面上先留着同一集的旧值，不抹成空。
	if !a.Win.IsVisible() {
		return
	}
	cid, bvid, aid := t.Cid, t.Bvid, extrasAid(a, t)
	c.onlineReqID++
	reqID := c.onlineReqID
	go func() {
		v, err := c.bl.GetOnlineViewers(bvid, aid, cid)
		a.Win.Update(func() {
			if c.onlineReqID != reqID {
				return
			}
			// 换集了（或已经停了）：这一轮的数作废。
			cur := currentExtrasTrack(a)
			if cur == nil || cur.Cid != cid {
				return
			}
			out := onlineOutcomeFor(v, err, c.onlineFails)
			c.onlineFails = out.Fails
			switch {
			case out.KeepOld:
				// 留着上一轮的数，界面不动。
			case err != nil:
				log.Printf("当前观看人数拉取失败（连续第 %d 轮）: %v", c.onlineFails, err)
				c.setOnlineViewers(0, 0)
			default:
				// 没数（开关关掉 / 当下没人）时把 cid 一起清成 0：界面那边
				// 两个条件都挡，不留一个「有归属但没值」的中间态。
				owner := int64(0)
				if out.Value > 0 {
					owner = cid
				}
				c.setOnlineViewers(out.Value, owner)
			}
		})
	}()
}

// setOnlineViewers 写界面上的在线人数（主线程；调用方已经在一个更新回合里，
// 所以不自己触发重绘）。值没变时直接返回 —— 这个数 30s 才动一次，
// 不该顺手把整棵界面重建一遍。
func (c *controller) setOnlineViewers(n, cid int64) {
	a := c.app
	if a == nil || (a.OnlineCount == n && a.OnlineCid == cid) {
		return
	}
	a.OnlineCount, a.OnlineCid = n, cid
}

// clearOnlineViewers 在换集时清掉上一集的数（由 clearPlaybackExtras 在主线程调）。
// 同时在途的那一枪响应回来会因 reqID 不符被丢，不需要再等它。
func (c *controller) clearOnlineViewers() {
	c.onlineReqID++
	c.onlineFails = 0
	if c.app != nil {
		c.app.OnlineCount, c.app.OnlineCid = 0, 0
	}
}
