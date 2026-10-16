package main

import (
	"encoding/json"
	"log"
	"strconv"
	"time"
)

// 播放进度同步。对齐 src/components/player.tsx 的行为：
//
//   - 本地断点（不依赖账号）：每 5 秒落一次盘，暂停 / 切歌时补写，
//     7 天后过期，最多留 50 条；
//   - 云端进度：起播时读一次（最多等 800ms），播放中每 30 秒上报一次，
//     暂停 / 切歌 / 跳转时立刻补报；
//   - 续播点取 max(云端, 本地)：本地断点可能比云端新（刚听完的那一段
//     还没上报上去）；
//   - 隐身模式：读和写都不碰云端，只用本地断点。
const (
	cloudProgressBudget   = 800 * time.Millisecond
	cloudReportInterval   = 30 * time.Second
	localResumeWriteEvery = 5 * time.Second
	localResumeMaxAge     = 7 * 24 * time.Hour
	localResumeMaxEntries = 50
	resumeKVKey           = "resume_points"

	// 断点太靠前就等于从头，不值得续。
	resumeMinSeconds = 5
	// 断点离结束太近（看成听完了）也不续。
	resumeTailSeconds = 5
)

// resumePoint 是本地保存的一个播放断点。
type resumePoint struct {
	Seconds float64 `json:"p"`
	At      int64   `json:"ts"` // unix 秒
}

// mediaKey 标识一条视频的一集。
func mediaKey(aid, cid int64) string {
	return strconv.FormatInt(aid, 10) + ":" + strconv.FormatInt(cid, 10)
}

// resumePoints 读出本地断点表。
func (c *controller) resumePoints() map[string]resumePoint {
	out := map[string]resumePoint{}
	if raw := c.kv.String(resumeKVKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

// localResumePoint 取本地断点（秒）；没有、过期或无效都返回 0。
func (c *controller) localResumePoint(key string) float64 {
	p, ok := c.resumePoints()[key]
	if !ok || p.Seconds <= 0 {
		return 0
	}
	if time.Since(time.Unix(p.At, 0)) > localResumeMaxAge {
		return 0
	}
	return p.Seconds
}

// saveResumePoint 写本地断点：顺手扔掉过期的，并裁到上限条数。
func (c *controller) saveResumePoint(key string, seconds float64) {
	if seconds <= 0 {
		return
	}
	points := c.resumePoints()
	now := time.Now()
	points[key] = resumePoint{Seconds: seconds, At: now.Unix()}

	// 过期的先删。
	for k, p := range points {
		if now.Sub(time.Unix(p.At, 0)) > localResumeMaxAge {
			delete(points, k)
		}
	}
	// 还是太多就按时间留最新的。
	if len(points) > localResumeMaxEntries {
		type kv struct {
			k string
			t int64
		}
		all := make([]kv, 0, len(points))
		for k, p := range points {
			all = append(all, kv{k, p.At})
		}
		for i := 1; i < len(all); i++ {
			for j := i; j > 0 && all[j].t > all[j-1].t; j-- {
				all[j], all[j-1] = all[j-1], all[j]
			}
		}
		for _, e := range all[localResumeMaxEntries:] {
			delete(points, e.k)
		}
	}
	if b, err := json.Marshal(points); err == nil {
		_ = c.kv.SetString(resumeKVKey, string(b))
	}
}

// resumeTarget 把「续播点」修正成实际要跳到的秒数（原版 player.tsx 的
// applyPendingCloudProgress）：
//
//	断点比开头还靠（< resumeMinSeconds）     → 0，从头
//	断点离结尾不到 resumeTailSeconds        → 0，从头
//	其余                                  → 断点
//
// 这条对**历史记录里已听完的视频**是关键：B 站把「已看完」记成 progress = -1
// （云端读到就是 0，自然从头）；但云端也可能把断点停在最后几秒，或者本地
// 断点就是最后几秒 —— 原版一律回到 0 从**头**播，而不是跳到末尾一闪而过。
// 此前原生版直接把断点传给 Seek（只有 `>= 5` 一道门槛），从历史里点一条
// 看完的视频会直接停在最后一点。抽成纯函数便于单测。
func resumeTarget(resume, dur float64) float64 {
	if resume < resumeMinSeconds {
		return 0
	}
	if dur > 0 && resume >= dur-resumeTailSeconds {
		return 0
	}
	return resume
}

// cloudProgressOn 报告云端播放记录/进度同步是否开启。隐身模式（原版
// localStorage 的 incognitoMode）下关掉：**不读取也不上报**（本地断点照写，
// 与原版一致）。所有碰云端进度的路径都必须先过这一条 —— resolveResume 读、
// trackResume/flushProgress/finishProgress 写。
func (c *controller) cloudProgressOn() bool { return !c.app.Incognito }

// resolveResume 定这条视频的续播位置（秒）。云端最多等 800ms，超时或失败就
// 只用本地断点；晚到的云端响应不再管（否则会把已经在播的曲目跳走）。
//
// 隐身模式下**根本不发云端请求**（此前守卫写成 `c.app.Incognito || …`，语义
// 相同但散落在多处；抽成 cloudProgressOn 后只有一处判据）。
func (c *controller) resolveResume(aid, cid int64) float64 {
	local := c.localResumePoint(mediaKey(aid, cid))
	if !c.cloudProgressOn() || aid == 0 || cid == 0 {
		return local
	}
	ch := make(chan int, 1)
	go func() {
		p, err := c.bl.GetPlayProgress(aid, cid)
		if err != nil {
			p = 0
		}
		ch <- p
	}()
	select {
	case p := <-ch:
		if float64(p) > local {
			return float64(p)
		}
		return local
	case <-time.After(cloudProgressBudget):
		return local
	}
}

// trackResume 播放中的进度记账：本地断点按 5 秒落盘，云端按 30 秒上报。
// 由 Player 的进度回调（每 500ms 一次）驱动。
func (c *controller) trackResume(pos float64) {
	t := c.app.Current()
	if t == nil || t.Cid == 0 || pos <= 0 {
		return
	}
	// 快到了就别续了，当成听完。
	if c.app.Dur > 0 && pos >= c.app.Dur-resumeTailSeconds {
		pos = 0
	}
	// 刚播完的这条：不再写进度（已发 -1，别用末尾位置盖回去）。
	key := mediaKey(t.Aid, t.Cid)
	if c.finishedKey != "" && key == c.finishedKey {
		return
	}
	now := time.Now()

	if pos > 0 && now.Sub(c.resumeWroteAt) >= localResumeWriteEvery {
		c.resumeWroteAt = now
		c.saveResumePoint(key, pos)
	}

	if !c.cloudProgressOn() || c.app.Dur <= 0 {
		return
	}
	// 换了一集就立刻报一次。
	if key != c.reportedKey {
		c.reportedKey, c.reportedAt = key, time.Time{}
	}
	if now.Sub(c.reportedAt) >= cloudReportInterval {
		c.reportedAt = now
		c.reportProgress(t.Aid, t.Cid, pos)
	}
}

// reportProgress 上报一次云端进度，异步、失败只记日志。
//
// 这是**唯一**调 BL.ReportPlayProgress 的地方（契约测试钉住）：隐身模式下
// 直接返回，一次请求都不发。seconds 为 -1 表示「已看完」（B 站约定）。
func (c *controller) reportProgress(aid, cid int64, seconds float64) {
	if !c.cloudProgressOn() || aid == 0 || cid == 0 {
		return
	}
	go func() {
		if _, err := c.bl.ReportPlayProgress(int(aid), int(cid), int(seconds)); err != nil {
			log.Printf("上报播放进度失败: %v", err)
		}
	}()
}

// flushProgress 在暂停 / 跳转 / 切歌 / 退出时立刻补写本地断点并补报云端。
func (c *controller) flushProgress(pos float64) {
	t := c.app.Current()
	if t == nil || t.Cid == 0 || pos <= 0 {
		return
	}
	// 刚播完（已发 -1）：不要再用末尾位置盖回去。
	if c.finishedKey != "" && mediaKey(t.Aid, t.Cid) == c.finishedKey {
		return
	}
	if c.app.Dur > 0 && pos >= c.app.Dur-resumeTailSeconds {
		return // 听完了，保留上一次的断点没有意义
	}
	c.resumeWroteAt = time.Now()
	c.saveResumePoint(mediaKey(t.Aid, t.Cid), pos)
	c.reportedAt = time.Now()
	c.reportProgress(t.Aid, t.Cid, pos)
}

// finishProgress 在一条视频**自然播完**时补一次「已看完」上报。
//
// 原版 <audio> 的 onEnded 会调 `reportCloudProgress(-1, true)`
//（main:src/components/player.tsx:1171）：B 站用 progress = -1 表示已看完，
// 只有发了这一笔，服务端的观看记录才会把这集标成「看完」而不是「停在最后
// 几秒」。原生版此前只在**视频弹窗**那条路径上报 -1，主窗音频引擎播完只走
// handleEnded 续播下一集 —— 于是每条视频都停在最后一点，历史记录里全是
// 「未看完」，下次加载又从末端续播。这里补上这一笔并清掉本地断点。
//
// 隐身模式下不发云端请求（cloudProgressOn），本地断点照清。
func (c *controller) finishProgress() {
	t := c.app.Current()
	if t == nil || t.Cid == 0 {
		return
	}
	c.clearResume(t.Aid, t.Cid)
	// 记住这条已完：后续 flushProgress/trackResume 不再盖回去（同一次播放期间），
	// 隐身与否都要拦（本地断点也不该在播完后立刻写回末尾）。
	c.finishedKey = mediaKey(t.Aid, t.Cid)
	if !c.cloudProgressOn() {
		return
	}
	// 换集后第一笔上报不该被 30 秒节流窗口挡住（-1 是「完成」这个独立事件）。
	c.reportedKey, c.reportedAt = "", time.Time{}
	c.reportProgress(t.Aid, t.Cid, -1)
}

// clearResume 把这一集的断点清掉（听完 / 手动从头播）。
func (c *controller) clearResume(aid, cid int64) {
	points := c.resumePoints()
	key := mediaKey(aid, cid)
	if _, ok := points[key]; !ok {
		return
	}
	delete(points, key)
	if b, err := json.Marshal(points); err == nil {
		_ = c.kv.SetString(resumeKVKey, string(b))
	}
}
