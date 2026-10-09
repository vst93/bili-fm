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

// resolveResume 定这条视频的续播位置（秒）。云端最多等 800ms，超时或失败就
// 只用本地断点；晚到的云端响应不再管（否则会把已经在播的曲目跳走）。
func (c *controller) resolveResume(aid, cid int64) float64 {
	local := c.localResumePoint(mediaKey(aid, cid))
	if c.app.Incognito || aid == 0 || cid == 0 {
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
	key := mediaKey(t.Aid, t.Cid)
	now := time.Now()

	if pos > 0 && now.Sub(c.resumeWroteAt) >= localResumeWriteEvery {
		c.resumeWroteAt = now
		c.saveResumePoint(key, pos)
	}

	if c.app.Incognito || c.app.Dur <= 0 {
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
func (c *controller) reportProgress(aid, cid int64, seconds float64) {
	if c.app.Incognito || aid == 0 || cid == 0 {
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
	if c.app.Dur > 0 && pos >= c.app.Dur-resumeTailSeconds {
		return // 听完了，保留上一次的断点没有意义
	}
	c.resumeWroteAt = time.Now()
	c.saveResumePoint(mediaKey(t.Aid, t.Cid), pos)
	c.reportedAt = time.Now()
	c.reportProgress(t.Aid, t.Cid, pos)
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
