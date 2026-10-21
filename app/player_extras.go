package main

// 播放器附加信息：章节、字幕、弹幕热力、相关推荐电台。
//
// 这些数据都只在当前播放视频的生命周期里有效；换集时统一清空，异步响应
// 用 reqID + 当前 cid 双重校验，旧视频的数据绝不能落到新视频的界面上。

import (
	"log"
	"math"
	"strings"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/view"
)

// clearPlaybackExtras 在换集时清空所有播放器附加信息。调用方在主线程。
func (c *controller) clearPlaybackExtras() {
	a := c.app
	// 换代：所有在途的章节 / 字幕 / 热力响应立刻作废。仅清数据不够 ——
	// 旧请求可能刚好在新的 startCurrent 之后返回，单看 cid 时容易在
	// 快速切集时落到同一帧。
	c.extrasReqID++
	c.subtitleReqID++
	c.heatReqID++
	a.Chapters = nil
	a.SubtitleTracks = nil
	a.Subtitles = nil
	a.SubtitleLang = ""
	a.ExtrasCid = 0
	a.ExtrasLoading = false
	a.SubtitlesLoading = false
	a.SubtitlesCid = 0
	a.SubtitleError = ""
	a.DanmakuHeat = nil
	a.HeatCid = 0
	a.ResetSubtitleScroll()
	c.extrasInflightCid = 0
	c.subtitleInflightCid = 0
	c.ensureLyricsForTrack()
}

// currentExtrasTrack 返回章节 / 字幕所属的曲目：正在播放的那条。
// 与弹幕 / 评论不同，章节和字幕是播放器时间轴的一部分，浏览另一条视频时
// 不应该把正在听的章节换掉。
func currentExtrasTrack(a *view.App) *view.Track {
	if a.Track != nil && a.Track.Cid != 0 {
		return a.Track
	}
	return a.Current()
}

// loadPlaybackExtras 拉章节和字幕轨。可以先由起播触发，也可以在用户打开
// 章节 / 字幕页时补拉。
func (c *controller) loadPlaybackExtras(aid, cid int64, bvid string) {
	a := c.app
	if aid <= 0 || cid <= 0 || a == nil {
		return
	}
	c.extrasReqID++
	reqID := c.extrasReqID
	c.extrasInflightCid = cid
	// 新的 extras 请求开始时先清掉旧字幕正文，避免用户在响应回来前继续看到
	// 上一条视频的文稿；旧响应由 reqID / 当前 cid 双重挡住。
	a.Chapters = nil
	a.SubtitleTracks = nil
	a.Subtitles = nil
	a.SubtitlesCid = 0
	a.SubtitlesLoading = false
	a.SubtitleError = ""
	a.ResetSubtitleScroll()
	a.ExtrasLoading = true
	a.Win.Update(func() {})
	go func() {
		ex, err := c.bl.GetPlayerExtras(aid, cid, bvid)
		a.Win.Update(func() {
			if c.extrasReqID != reqID {
				return
			}
			if c.extrasInflightCid == cid {
				c.extrasInflightCid = 0
			}
			a.ExtrasLoading = false
			if err != nil || ex == nil {
				if err != nil {
					log.Printf("章节/字幕加载失败: %v", err)
				}
				return
			}
			cur := currentExtrasTrack(a)
			if cur == nil || cur.Cid != cid {
				return
			}
			a.ExtrasCid = cid
			a.Chapters = make([]view.Chapter, 0, len(ex.Chapters))
			for _, ch := range ex.Chapters {
				a.Chapters = append(a.Chapters, view.Chapter{From: ch.From, To: ch.To, Title: ch.Title})
			}
			c.normalizeChapterEnds()
			a.SubtitleTracks = make([]view.SubtitleTrack, 0, len(ex.Subtitles))
			for _, st := range ex.Subtitles {
				a.SubtitleTracks = append(a.SubtitleTracks, view.SubtitleTrack{
					Lan: st.Lan, Label: st.Label, URL: st.URL,
				})
			}
			if a.LyricsOn && len(a.SubtitleTracks) == 0 {
				c.closeDesktopLyrics()
				a.Notify("当前视频没有字幕，无法显示桌面字幕")
			}
			// 如果用户正停在字幕页/桌面字幕，数据一到就继续下载正文。
			if a.DanmakuTab == view.TabSubtitle || a.LyricsOn {
				c.ensureSubtitles()
			}
		})
	}()
}

// ensureExtras 在打开章节 / 字幕页时补拉当前曲目的附加信息。
func (c *controller) ensureExtras() {
	a := c.app
	t := currentExtrasTrack(a)
	if t == nil || t.Cid == 0 || t.Aid == 0 {
		return
	}
	if a.ExtrasCid == t.Cid {
		return
	}
	if c.extrasInflightCid == t.Cid {
		return
	}
	c.loadPlaybackExtras(t.Aid, t.Cid, t.Bvid)
}

// normalizeChapterEnds 用视频时长补最后一章的 To（view_points 的末章常常
// 只给 from，没有 to）。
func (c *controller) normalizeChapterEnds() {
	a := c.app
	if len(a.Chapters) == 0 || a.Dur <= 0 {
		return
	}
	for i := range a.Chapters {
		ch := &a.Chapters[i]
		if ch.To <= ch.From {
			if i+1 < len(a.Chapters) {
				ch.To = a.Chapters[i+1].From
			} else {
				ch.To = a.Dur
			}
		}
		if ch.To > a.Dur {
			ch.To = a.Dur
		}
	}
}

// pickSubtitleTrack 选默认字幕：优先中文，其次第一条。用户目前只需要一个
// 文稿视图，不为了「多语言选择」增加一整套下拉 UI。
func pickSubtitleTrack(tracks []view.SubtitleTrack) (view.SubtitleTrack, bool) {
	return pickSubtitleTrackPref(tracks, "")
}

func pickSubtitleTrackPref(tracks []view.SubtitleTrack, pref string) (view.SubtitleTrack, bool) {
	if len(tracks) == 0 {
		return view.SubtitleTrack{}, false
	}
	if pref != "" {
		for _, t := range tracks {
			if strings.EqualFold(t.Lan, pref) {
				return t, true
			}
		}
	}
	for _, want := range []string{"zh-cn", "zh-hans", "zh", "ai-zh"} {
		for _, t := range tracks {
			if strings.EqualFold(t.Lan, want) {
				return t, true
			}
		}
	}
	for _, t := range tracks {
		if strings.Contains(strings.ToLower(t.Lan), "zh") {
			return t, true
		}
	}
	return tracks[0], true
}

// ensureSubtitles 为当前曲目下载字幕正文；已加载 / 未登录 / 无字幕轨都
// 静默结束，不打扰播放。
func (c *controller) ensureSubtitles() {
	a := c.app
	t := currentExtrasTrack(a)
	if t == nil || t.Cid == 0 {
		return
	}
	if a.ExtrasCid != t.Cid || a.ExtrasLoading {
		c.ensureExtras()
		return
	}
	track, ok := pickSubtitleTrackPref(a.SubtitleTracks, c.subtitleLangPref)
	if !ok {
		c.ensureExtras()
		return
	}
	if a.SubtitlesCid == t.Cid && a.SubtitleLang == track.Lan &&
		(len(a.Subtitles) > 0 || a.SubtitleError != "") {
		return
	}
	if c.subtitleInflightCid == t.Cid {
		return
	}
	a.SubtitleLang = track.Lan
	c.startSubtitleLoad(t, track)
}

// startSubtitleLoad 下载一条字幕正文。调用方已经校验过当前曲目 / cid；
// 这里再用 subtitleReqID + 当前 cid 挡住过期响应。
func (c *controller) startSubtitleLoad(t *view.Track, track view.SubtitleTrack) {
	a := c.app
	c.subtitleReqID++
	reqID := c.subtitleReqID
	c.subtitleInflightCid = t.Cid
	a.Subtitles = nil
	a.SubtitlesCid = 0
	a.SubtitlesLoading = true
	a.SubtitleError = ""
	a.ResetSubtitleScroll()
	a.Win.Update(func() {})
	go func() {
		cues, err := c.bl.GetSubtitleCues(track.URL)
		a.Win.Update(func() {
			if c.subtitleReqID != reqID {
				return
			}
			if c.subtitleInflightCid == t.Cid {
				c.subtitleInflightCid = 0
			}
			a.SubtitlesLoading = false
			cur := currentExtrasTrack(a)
			if cur == nil || cur.Cid != t.Cid {
				return
			}
			if err != nil {
				a.SubtitleError = "字幕加载失败"
				log.Printf("字幕加载失败: %v", err)
				return
			}
			a.SubtitleLang = track.Lan
			a.SubtitlesCid = t.Cid
			a.Subtitles = make([]view.SubtitleCue, 0, len(cues))
			for _, cue := range cues {
				a.Subtitles = append(a.Subtitles, view.SubtitleCue{
					From: cue.From, To: cue.To, Content: cue.Content,
				})
			}
			if len(a.Subtitles) == 0 {
				a.SubtitleError = "字幕内容为空"
				if a.LyricsOn {
					c.closeDesktopLyrics()
					a.Notify("当前视频没有可用字幕")
				}
			}
			a.ResetSubtitleScroll()
			c.syncLyricsAt(a.Pos)
			if a.LyricsOn {
				c.openDesktopLyricsIfReady()
			}
		})
	}()
}

// selectSubtitle 切换字幕版本 / 语言。只接受当前视频的轨道；选择结果作为
// 跨视频的语言偏好保存，下次有同语言轨时优先用。
func (c *controller) selectSubtitle(index int) {
	a := c.app
	t := currentExtrasTrack(a)
	if t == nil || t.Cid == 0 || index < 0 || index >= len(a.SubtitleTracks) {
		return
	}
	if a.ExtrasCid != t.Cid {
		return
	}
	track := a.SubtitleTracks[index]
	c.subtitleLangPref = track.Lan
	_ = c.kv.SetString(prefSubtitleLang, track.Lan)
	a.SubtitleLang = track.Lan
	c.startSubtitleLoad(t, track)
}

// reloadPlaybackExtras 是章节 / 字幕页刷新键：强迫重拉当前曲目的附加信息。
func (c *controller) reloadPlaybackExtras() {
	a := c.app
	t := currentExtrasTrack(a)
	if t == nil || t.Cid == 0 {
		return
	}
	a.ExtrasCid = 0
	a.SubtitlesCid = 0
	a.Subtitles = nil
	a.SubtitleTracks = nil
	a.Chapters = nil
	a.SubtitleError = ""
	c.extrasInflightCid = 0
	c.subtitleInflightCid = 0
	c.loadPlaybackExtras(t.Aid, t.Cid, t.Bvid)
	if a.DanmakuTab == view.TabSubtitle || a.LyricsOn {
		c.ensureSubtitles()
	}
}

// startDanmakuHeat 为当前曲目单独拉一次弹幕并聚合成热力桶。它和弹幕抽屉
// 的请求互相独立：抽屉跟的是「浏览中」的视频，避免「播放 A、浏览 B」时
// 把 B 的热力画到 A 的进度条上。
func (c *controller) startDanmakuHeat(cid int64) {
	if cid == 0 {
		return
	}
	a := c.app
	c.heatReqID++
	reqID := c.heatReqID
	go func() {
		d, err := c.bl.GetDanmakuList(int(cid))
		if err != nil || d == nil {
			return
		}
		pts := buildDanmakuHeat(d.Items)
		a.Win.Update(func() {
			if c.heatReqID != reqID {
				return
			}
			cur := a.Current()
			if cur == nil || cur.Cid != cid {
				return
			}
			a.DanmakuHeat = pts
			a.HeatCid = cid
		})
	}()
}

// buildDanmakuHeat 把弹幕按 10 秒聚合成 0..1 的热力点。只保存聚合结果，
// 不把完整弹幕文本长期留在内存里。
func buildDanmakuHeat(items []bilibili.DanmakuItem) []view.HeatPoint {
	if len(items) == 0 {
		return nil
	}
	maxT := 0.0
	for _, it := range items {
		if it.Time > maxT {
			maxT = it.Time
		}
	}
	if maxT <= 0 {
		return nil
	}
	// 5 秒一桶；超长视频按比例放大桶宽，保证热力条最多 480 个点，
	// 既足够画出峰形，又让每帧绘制成本不随视频长度增长。
	bucket := 5.0
	if maxT/bucket > 480 {
		bucket = math.Ceil(maxT/480/5) * 5
	}
	counts := map[int]int{}
	for _, it := range items {
		if it.Time < 0 {
			continue
		}
		b := int(it.Time / bucket)
		counts[b]++
	}
	if len(counts) == 0 {
		return nil
	}
	maxN := 0
	for _, n := range counts {
		if n > maxN {
			maxN = n
		}
	}
	if maxN == 0 {
		return nil
	}
	n := int(maxT/bucket) + 1
	out := make([]view.HeatPoint, 0, n)
	for i := 0; i < n; i++ {
		if c := counts[i]; c > 0 {
			// sqrt 压缩高峰：一个超级爆点不会把其余时段的热力压成一条线。
			v := math.Sqrt(float64(c) / float64(maxN))
			out = append(out, view.HeatPoint{At: float64(i) * bucket, Value: float32(v)})
		}
	}
	return out
}

// nextHighlight 跳到当前时间之后第一个足够明显的弹幕热区。
func (c *controller) nextHighlight() {
	a := c.app
	pos := a.Pos
	for _, p := range a.DanmakuHeat {
		if p.At > pos+1 && p.Value >= 0.25 {
			c.seek(p.At)
			return
		}
	}
	a.Notify("后面没有更多高能片段")
}

// ---------------------------------------------------------------- 顺着听

// toggleRadio 开启/关闭以当前曲目为种子的相关推荐电台。开启时退出「播放
// 列表来源」模式，但保留当前队列和索引，之后由队列续播；关闭时只停止扩展，
// 不打断当前播放。
func (c *controller) toggleRadio() {
	a := c.app
	if a.RadioOn {
		a.RadioOn = false
		a.RadioLoading = false
		c.radioReqID++
		c.radioSeen = nil
		c.radioWantNext = false
		a.Win.Update(func() {})
		return
	}
	t := a.Current()
	if t == nil || t.Bvid == "" {
		a.NotifyType("warning", "请先播放一个视频")
		return
	}
	// 电台需要在队列里续播；从播放列表进入时保留当前队列，但离开列表来源，
	// 否则 step 会一直回到播放列表分支。
	a.PlayingPlaylist = ""
	a.RadioOn = true
	a.RadioLoading = false
	c.radioSeen = map[string]bool{}
	for _, q := range a.Queue {
		if q.Bvid != "" {
			c.radioSeen[q.Bvid] = true
		}
	}
	a.Win.Update(func() {})
	c.maybeExtendRadio()
}

// maybeExtendRadio 在当前队列尾部补充相关推荐。已有 5 条以上余量时不请求。
func (c *controller) maybeExtendRadio() {
	a := c.app
	if !a.RadioOn || a.RadioLoading {
		return
	}
	t := a.Current()
	if t == nil || t.Bvid == "" {
		return
	}
	if len(a.Queue)-a.Index > 5 {
		return
	}
	c.radioReqID++
	reqID := c.radioReqID
	a.RadioLoading = true
	a.Win.Update(func() {})
	seed := t.Bvid
	go func() {
		rel, err := c.bl.GetRelatedVideos(seed)
		a.Win.Update(func() {
			if c.radioReqID != reqID {
				return
			}
			a.RadioLoading = false
			if err != nil {
				log.Printf("顺着听加载失败: %v", err)
				if c.radioWantNext {
					c.radioWantNext = false
					a.NotifyType("error", "顺着听加载失败")
				}
				return
			}
			cur := a.Current()
			if !a.RadioOn || cur == nil || cur.Bvid != seed {
				return
			}
			added := 0
			for _, r := range rel {
				if r.Bvid == "" || c.radioSeen[r.Bvid] {
					continue
				}
				c.radioSeen[r.Bvid] = true
				a.Queue = append(a.Queue, view.Track{
					Aid: r.Aid, Bvid: r.Bvid, Cid: r.Cid, Title: r.Title,
					Up: r.UP, Cover: r.Pic, Duration: r.Duration,
				})
				added++
				if added >= 12 {
					break
				}
			}
			c.trimRadioQueue()
			if added == 0 {
				a.RadioOn = false
				c.radioWantNext = false
				a.Notify("没有更多相关视频，顺着听已停止")
				return
			}
			if c.radioWantNext {
				c.radioWantNext = false
				c.step(1)
			}
		})
	}()
}

// trimRadioQueue 限制电台队列驻留量，避免长时间电台无限积累。保留当前项
// 及之后的内容，丢掉前面已经听过的部分（current index 会同步左移）。
func (c *controller) trimRadioQueue() {
	const cap = 48
	a := c.app
	if len(a.Queue) <= cap {
		return
	}
	drop := len(a.Queue) - cap
	if drop > a.Index {
		drop = a.Index
	}
	if drop <= 0 {
		return
	}
	a.Queue = append([]view.Track(nil), a.Queue[drop:]...)
	a.Index -= drop
}

// radioNext 处理电台在队列尾部继续播放；没有余量时请求一批，响应回来再续。
func (c *controller) radioNext() {
	a := c.app
	if !a.RadioOn {
		return
	}
	if a.Index+1 < len(a.Queue) {
		c.step(1)
		return
	}
	c.radioWantNext = true
	c.maybeExtendRadio()
}
