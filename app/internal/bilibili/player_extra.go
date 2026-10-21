package bilibili

// 播放器附加信息：章节、字幕、相关视频。
//
// 这三块都来自 x/player/v2 或相邻的视频接口，属于「播放时顺带拿到」的文本
// 数据，不引入新的解码/存储依赖。字幕只在用户打开字幕页或桌面歌词时再下载
// BCC JSON，避免每个视频都多一次请求。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// Chapter 是 B 站视频的一个章节（view_points 的一项），时间单位为秒。
type Chapter struct {
	From  float64
	To    float64
	Title string
}

// SubtitleTrack 是 player/v2 返回的一条字幕轨。
type SubtitleTrack struct {
	Lan    string
	Label  string
	URL    string
	AIType int
}

// SubtitleCue 是字幕里的一条时间轴文本（BCC 的 body 项）。
type SubtitleCue struct {
	From    float64
	To      float64
	Content string
}

// PlayerExtras 是播放一部视频时顺手拿到的章节与字幕轨列表。
type PlayerExtras struct {
	Chapters  []Chapter
	Subtitles []SubtitleTrack
}

// RelatedVideo 是相关推荐里的一条（「顺着听」的种子）。
type RelatedVideo struct {
	Aid      int64
	Bvid     string
	Cid      int64
	Title    string
	Pic      string
	UP       string
	Duration int64
	Views    int64
}

func mapAt(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

func floatAt(m map[string]any, key string) float64 {
	switch n := m[key].(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

// GetPlayerExtras 读取 view_points（章节）与 subtitle.subtitles（字幕轨）。
// 未登录也能拿到章节；字幕轨在未登录时可能为空，调用方按空处理即可。
func (bl *BL) GetPlayerExtras(aid, cid int64, bvid ...string) (*PlayerExtras, error) {
	cookie := bl.GetSESSDATA()
	if aid <= 0 || cid <= 0 {
		return nil, fmt.Errorf("未选择作品")
	}
	// bvid 优先：搜索 / 动态 / 合集卡片上的 aid 偶尔会和 bvid 不是同一条
	// 稿件，同时传两个参数时上游可能按 aid 取，结果拿到别的稿件字幕。
	// 有 bvid 时就只用 bvid + cid 定位。
	bv := ""
	if len(bvid) > 0 {
		bv = bvid[0]
	}
	playerURL := playerExtrasURL(aid, cid, bv)
	v, err := doJSON("GET", playerURL, cookie)
	if err != nil {
		return nil, err
	}
	d := dataOf(v)
	out := &PlayerExtras{}
	for _, it := range arrOf(d, "view_points") {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		from := floatAt(m, "from")
		to := floatAt(m, "to")
		title := strAt(m, "content")
		if title == "" || from < 0 {
			continue
		}
		if to <= from {
			to = from // 末章可能没有 to，调用方会用视频时长补
		}
		out.Chapters = append(out.Chapters, Chapter{From: from, To: to, Title: title})
	}
	sort.SliceStable(out.Chapters, func(i, j int) bool { return out.Chapters[i].From < out.Chapters[j].From })

	sub := mapAt(d, "subtitle")
	for _, it := range arrOf(sub, "subtitles") {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		u := strAt(m, "subtitle_url")
		if strings.HasPrefix(u, "//") {
			u = "https:" + u
		}
		if u == "" {
			continue
		}
		label := strAt(m, "lan_doc")
		if label == "" {
			label = strAt(m, "lan")
		}
		out.Subtitles = append(out.Subtitles, SubtitleTrack{
			Lan:    strAt(m, "lan"),
			Label:  label,
			URL:    u,
			AIType: int(intAt(m, "ai_type")),
		})
	}
	return out, nil
}

// playerExtrasURL 生成 player/v2 地址：有 bvid 时只带 bvid + cid，否则退回
// aid + cid。拆成纯函数便于钉住「有 bvid 时不能再把可能过期的 aid 带上」。
func playerExtrasURL(aid, cid int64, bvid string) string {
	if bvid != "" {
		return fmt.Sprintf("https://api.bilibili.com/x/player/v2?cid=%d&bvid=%s",
			cid, url.QueryEscape(bvid))
	}
	return fmt.Sprintf("https://api.bilibili.com/x/player/v2?aid=%d&cid=%d", aid, cid)
}

// GetSubtitleCues 下载并解析 BCC 字幕 JSON。
func (bl *BL) GetSubtitleCues(rawURL string) ([]SubtitleCue, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("没有字幕地址")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("字幕地址协议不支持")
	}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Referer", "https://www.bilibili.com/")
	if cookie := bl.GetSESSDATA(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := extraClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("字幕返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var v struct {
		Body []struct {
			From    float64 `json:"from"`
			To      float64 `json:"to"`
			Content string  `json:"content"`
		} `json:"body"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	out := make([]SubtitleCue, 0, len(v.Body))
	for _, b := range v.Body {
		if b.Content == "" || b.To < b.From {
			continue
		}
		out = append(out, SubtitleCue{From: b.From, To: b.To, Content: b.Content})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].From < out[j].From })
	return out, nil
}

// SubtitleCuesFit 用视频时长做一次兜底校验：字幕最后一条远超视频时长，
// 基本可以断定是上游把另一条稿件的字幕轨挂过来了。宁可少显示，也不把
// 明显对不上的文稿贴到当前视频上。dur<=0（时长还没拿到）时不判断。
func SubtitleCuesFit(cues []SubtitleCue, dur float64) bool {
	if dur <= 0 || len(cues) == 0 {
		return true
	}
	first, last := cues[0], cues[len(cues)-1]
	if first.From < 0 || last.To <= 0 {
		return false
	}
	if first.From > dur+5 {
		return false
	}
	tolerance := dur * 0.1
	if tolerance < 30 {
		tolerance = 30
	}
	return last.To <= dur+tolerance
}

// GetRelatedVideos 取一条视频的相关推荐，用作「顺着听」的候选队列。
// 接口失败返回错误，由调用方决定是否只停掉电台；不阻塞当前播放。
func (bl *BL) GetRelatedVideos(bvid string) ([]RelatedVideo, error) {
	if bvid == "" {
		return nil, fmt.Errorf("没有视频")
	}
	u := "https://api.bilibili.com/x/web-interface/archive/related?bvid=" + url.QueryEscape(bvid)
	v, err := doJSON("GET", u, bl.GetSESSDATA())
	if err != nil {
		return nil, err
	}
	raw, _ := v["data"].([]any)
	out := make([]RelatedVideo, 0, len(raw))
	for _, it := range raw {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		rv := RelatedVideo{
			Aid:      intAt(m, "aid"),
			Bvid:     strAt(m, "bvid"),
			Cid:      intAt(m, "cid"),
			Title:    strAt(m, "title"),
			Pic:      strAt(m, "pic"),
			Duration: intAt(m, "duration"),
		}
		if o := mapAt(m, "owner"); o != nil {
			rv.UP = strAt(o, "name")
		}
		if st := mapAt(m, "stat"); st != nil {
			rv.Views = intAt(st, "view")
		}
		if rv.Bvid != "" {
			out = append(out, rv)
		}
	}
	return out, nil
}
