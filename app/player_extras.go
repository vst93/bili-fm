package main

// 播放器附加信息：章节、字幕、弹幕热力、相关推荐电台。
//
// 这些数据都只在当前播放视频的生命周期里有效；换集时统一清空，异步响应
// 用 reqID + 当前 cid 双重校验，旧视频的数据绝不能落到新视频的界面上。

import (
	"log"
	"math"
	"strconv"
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
	c.resetLyricsOffsetOnTrackChange()
	a.Chapters = nil
	a.SubtitleTracks = nil
	a.Subtitles = nil
	a.SubtitleLang = ""
	a.ExtrasCid = 0
	a.ExtrasBvid = ""
	a.ExtrasLoading = false
	a.SubtitlesLoading = false
	a.SubtitlesCid = 0
	a.SubtitlesBvid = ""
	a.SubtitlesURL = ""
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

// extrasAid 返回可信的 aid：卡片/历史条目里的 aid 可能缺失或过期，而
// 当前视频的 Info 是按 bvid 重新拉到的。只在 bvid 对得上时采用 Info.Aid，
// 避免把「浏览中」的另一条视频 aid 套到正在播放的曲目上。
func extrasAid(a *view.App, t *view.Track) int64 {
	if t == nil {
		return 0
	}
	if a.Info != nil && a.Info.Aid > 0 && a.Info.Bvid != "" && a.Info.Bvid == t.Bvid {
		return a.Info.Aid
	}
	return t.Aid
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
	a.SubtitlesBvid = ""
	a.SubtitlesURL = ""
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
			if cur == nil || cur.Cid != cid || (bvid != "" && cur.Bvid != "" && cur.Bvid != bvid) {
				return
			}
			a.ExtrasCid = cid
			a.ExtrasBvid = bvid
			a.Chapters = make([]view.Chapter, 0, len(ex.Chapters))
			for _, ch := range ex.Chapters {
				a.Chapters = append(a.Chapters, view.Chapter{From: ch.From, To: ch.To, Title: ch.Title})
			}
			c.normalizeChapterEnds()
			a.SubtitleTracks = make([]view.SubtitleTrack, 0, len(ex.Subtitles))
			for _, st := range ex.Subtitles {
				a.SubtitleTracks = append(a.SubtitleTracks, view.SubtitleTrack{
					Lan: st.Lan, Label: st.Label, URL: st.URL, AIType: st.AIType,
				})
			}
			if len(a.SubtitleTracks) == 0 {
				// 没字幕只隐藏窗口，保留用户开关；下一个有字幕的视频会自动显示。
				if a.LyricsEnabled {
					c.hideDesktopLyrics()
				}
			} else if a.DanmakuTab == view.TabSubtitle || a.LyricsEnabled {
				c.ensureSubtitles()
			}
		})
	}()
}

// ensureExtras 在打开章节 / 字幕页时补拉当前曲目的附加信息。
func (c *controller) ensureExtras() {
	a := c.app
	t := currentExtrasTrack(a)
	if t == nil || t.Cid == 0 {
		return
	}
	aid := extrasAid(a, t)
	if aid == 0 {
		return
	}
	if a.ExtrasCid == t.Cid && (t.Bvid == "" || a.ExtrasBvid == t.Bvid) {
		return
	}
	if c.extrasInflightCid == t.Cid {
		return
	}
	c.loadPlaybackExtras(aid, t.Cid, t.Bvid)
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
	// 同一语言优先人工轨（AIType==0），再退回 AI 轨；AI 转写经常有错字/
	// 断句问题，不能因为 lan 相同就把人工字幕挤掉。
	pickLang := func(want string) (view.SubtitleTrack, bool) {
		var ai view.SubtitleTrack
		foundAI := false
		for _, t := range tracks {
			if !strings.EqualFold(t.Lan, want) {
				continue
			}
			if t.AIType == 0 {
				return t, true
			}
			if !foundAI {
				ai, foundAI = t, true
			}
		}
		return ai, foundAI
	}
	if pref != "" {
		if t, ok := pickLang(pref); ok {
			return t, true
		}
	}
	for _, want := range []string{"zh-cn", "zh-hans", "zh", "ai-zh"} {
		if t, ok := pickLang(want); ok {
			return t, true
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
	if a.ExtrasCid != t.Cid || (t.Bvid != "" && a.ExtrasBvid != t.Bvid) || a.ExtrasLoading {
		c.ensureExtras()
		return
	}
	track, ok := pickSubtitleTrackPref(a.SubtitleTracks, c.subtitleLangPref)
	if !ok {
		c.ensureExtras()
		return
	}
	if a.SubtitlesCid == t.Cid && a.SubtitlesBvid == t.Bvid &&
		a.SubtitlesURL == track.URL && a.SubtitleLang == track.Lan &&
		(len(a.Subtitles) > 0 || a.SubtitleError != "") {
		return
	}
	if c.subtitleInflightCid == t.Cid {
		return
	}
	a.SubtitleLang = track.Lan
	c.startSubtitleLoad(t, track)
}

// retrySubtitles 在用户重新打开字幕页时重试一次失败的字幕正文。网络抖动
// 或上游临时返回空轨时，失败被 markFailed 记下来是为了避免抽屉每帧重试；
// 但用户主动打开页面应当得到一次新的机会。
func (c *controller) retrySubtitles() {
	a := c.app
	if a == nil || a.SubtitlesLoading || len(a.Subtitles) > 0 || a.SubtitleError == "" {
		c.ensureSubtitles()
		return
	}
	a.SubtitleError = ""
	a.SubtitlesURL = ""
	a.SubtitlesCid = 0
	a.SubtitlesBvid = ""
	c.ensureSubtitles()
}

// startSubtitleLoad 下载一条字幕正文。调用方已经校验过当前曲目 / cid；
// 这里再用 subtitleReqID + 当前 cid 挡住过期响应。
func (c *controller) startSubtitleLoad(t *view.Track, track view.SubtitleTrack) {
	a := c.app
	// 请求参数在发请求这一刻快照下来；回调不再读可变的 t（它可能在用户
	// 切到另一集时被原地改写），只用快照和当前曲目做身份校验。
	bvid, cid := t.Bvid, t.Cid
	c.subtitleReqID++
	reqID := c.subtitleReqID
	c.subtitleInflightCid = cid
	a.Subtitles = nil
	a.SubtitlesCid = 0
	a.SubtitlesBvid = ""
	a.SubtitlesURL = ""
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
			if c.subtitleInflightCid == cid {
				c.subtitleInflightCid = 0
			}
			a.SubtitlesLoading = false
			cur := currentExtrasTrack(a)
			if cur == nil || cur.Cid != cid || (bvid != "" && cur.Bvid != "" && cur.Bvid != bvid) {
				return
			}
			// 失败也要记下「这条轨道已经试过」：SubtitlesURL 是 ensureSubtitles
			// 的缓存键，否则抽屉每帧都会重新发起同一个失败请求。
			markFailed := func(msg string) {
				a.SubtitleLang = track.Lan
				a.SubtitlesCid = cid
				a.SubtitlesBvid = bvid
				a.SubtitlesURL = track.URL
				a.SubtitleError = msg
				if a.LyricsEnabled {
					c.hideDesktopLyrics()
				}
			}
			if err != nil {
				markFailed("字幕加载失败")
				log.Printf("字幕加载失败: %v", err)
				return
			}
			a.SubtitleLang = track.Lan
			a.SubtitlesCid = cid
			a.SubtitlesBvid = bvid
			a.SubtitlesURL = track.URL
			a.Subtitles = make([]view.SubtitleCue, 0, len(cues))
			for _, cue := range cues {
				a.Subtitles = append(a.Subtitles, view.SubtitleCue{
					From: cue.From, To: cue.To, Content: cue.Content,
				})
			}
			if len(a.Subtitles) == 0 {
				markFailed("字幕内容为空")
			}
			a.ResetSubtitleScroll()
			c.syncLyricsAt(a.Pos)
			if a.LyricsEnabled {
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
	if a.ExtrasCid != t.Cid || (t.Bvid != "" && a.ExtrasBvid != t.Bvid) {
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
	a.ExtrasBvid = ""
	a.SubtitlesCid = 0
	a.SubtitlesBvid = ""
	a.SubtitlesURL = ""
	a.Subtitles = nil
	a.SubtitleTracks = nil
	a.Chapters = nil
	a.SubtitleError = ""
	c.extrasInflightCid = 0
	c.subtitleInflightCid = 0
	c.loadPlaybackExtras(extrasAid(a, t), t.Cid, t.Bvid)
	if a.DanmakuTab == view.TabSubtitle || a.LyricsEnabled {
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
		c.syncMenuToggles()
		c.radioReqID++
		c.radioSeen = nil
		c.radioWantNext = false
		dropFutureRadio(a)
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
	c.syncMenuToggles()
	c.radioSeen = map[string]bool{}
	for _, q := range a.Queue {
		if q.Bvid != "" {
			c.radioSeen[q.Bvid] = true
		}
	}
	a.Win.Update(func() {})
	c.maybeExtendRadio()
}

// persistFollowPrefs 保存弹幕 / 字幕的跟随开关。用户手动滚动导致自动关闭
// 时也走这里，所以下次打开抽屉/重启不会被重置回跟随。
func (c *controller) persistFollowPrefs() {
	if c.kv == nil || c.app == nil {
		return
	}
	_ = c.kv.SetString(prefSubtitleFollow, strconv.FormatBool(c.app.SubtitleFollow))
	_ = c.kv.SetString(prefDanmakuFollow, strconv.FormatBool(c.app.DanmakuAutoScroll))
}

// toggleHeat 切换弹幕热力开关并落盘。菜单和弹幕页表头共用。
func (c *controller) toggleHeat() {
	c.setHeatOn(!c.app.HeatOn)
}

// setHeatOn 设置热力开关并落盘，供菜单复选框直接设置目标值。
func (c *controller) setHeatOn(on bool) {
	a := c.app
	a.HeatOn = on
	_ = c.kv.SetString(prefHeatOn, strconv.FormatBool(on))
	if c.menuHeat != nil {
		c.menuHeat.SetChecked(on)
	}
	a.Win.Update(func() {})
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
					Up: r.UP, Cover: r.Pic, Duration: r.Duration, FromRadio: true,
				})
				added++
				if added >= 12 {
					break
				}
			}
			c.trimRadioQueue()
			if added == 0 {
				a.RadioOn = false
				c.syncMenuToggles()
				c.radioWantNext = false
				dropFutureRadio(a)
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

// dropFutureRadio 删除当前项之后由「顺着听」追加的推荐；用户关闭电台后，
// 当前项继续播完就停，不再自动/手动切到下一首推荐。原有的选集/播放列表
// 条目（FromRadio=false）保留。
func dropFutureRadio(a *view.App) {
	if a == nil || a.Index+1 >= len(a.Queue) {
		return
	}
	out := a.Queue[:a.Index+1]
	for _, t := range a.Queue[a.Index+1:] {
		if !t.FromRadio {
			out = append(out, t)
		}
	}
	a.Queue = out
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
