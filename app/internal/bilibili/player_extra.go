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
	"strconv"
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
	ID     string
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
//
// 字幕列表参考 Bilibili-Obsidian-Clipper：
//   - player/wbi/v2 是主来源，不签名也能用；先签名版，再裸版。
//   - 两个 WBI 请求都失败才回退 player/v2。
//   - WBI 成功但没有字幕轨 = 这条视频没有字幕，不再拿 player/v2 的旧缓存
//     去补 —— 那是「有时没字幕、有时字幕对不上」的来源。
//
// 每个响应都校验 aid/cid/bvid 身份，绝不接受另一条稿件的数据。
func (bl *BL) GetPlayerExtras(aid, cid int64, bvid ...string) (*PlayerExtras, error) {
	cookie := bl.GetSESSDATA()
	if cid <= 0 {
		return nil, fmt.Errorf("未选择作品")
	}
	bv := ""
	if len(bvid) > 0 {
		bv = bvid[0]
	}
	fetch := func(playerURL string) (*PlayerExtras, error) {
		v, err := doJSON("GET", playerURL, cookie)
		if err != nil {
			return nil, err
		}
		d := dataOf(v)
		if err := playerExtrasIdentityError(d, aid, cid, bv); err != nil {
			return nil, err
		}
		return parsePlayerExtras(d)
	}

	values := url.Values{"cid": {strconv.FormatInt(cid, 10)}}
	if aid > 0 {
		values.Set("aid", strconv.FormatInt(aid, 10))
	}
	if bv != "" {
		values.Set("bvid", bv)
	}
	wbiURLs := make([]string, 0, 2)
	if signed, err := bl.signedWBIURL("https://api.bilibili.com/x/player/wbi/v2", values); err == nil {
		wbiURLs = append(wbiURLs, signed)
	}
	// 不签名版放在签名版之后：签名 key 拿不到（未登录 / nav 风控）时它
	// 仍可能返回公开数据；签名版成功但没有轨时也再看一眼裸版，AI 字幕
	// 有时只在其中一个响应里出现。
	wbiURLs = append(wbiURLs, "https://api.bilibili.com/x/player/wbi/v2?"+values.Encode())

	var lastErr error
	var first *PlayerExtras
	for _, u := range wbiURLs {
		out, err := fetch(u)
		if err != nil {
			lastErr = err
			continue
		}
		if first == nil {
			first = out
		}
		if len(out.Subtitles) > 0 {
			return out, nil
		}
	}
	// 任一 WBI 成功（哪怕没有字幕）就以它为准，不再回退 player/v2。
	if first != nil {
		return first, nil
	}

	out, err := fetch(playerExtrasURL(aid, cid, bv))
	if err != nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, err
	}
	return out, nil
}

// parsePlayerExtras 从已经通过身份校验的 player 响应里解析章节和字幕轨。
func parsePlayerExtras(d map[string]any) (*PlayerExtras, error) {
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
			to = from
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
		u := normalizeSubtitleURL(strAt(m, "subtitle_url"))
		if u == "" {
			continue
		}
		label := strAt(m, "lan_doc")
		if label == "" {
			label = strAt(m, "lan")
		}
		// id_str 比 id 更稳（大整数在 JSON 里可能丢精度）；AI 轨常见
		// 只有 id_str。两者都没有时留空，选择回退到 URL。
		id := strAt(m, "id_str")
		if id == "" {
			switch n := m["id"].(type) {
			case string:
				id = n
			case float64:
				id = fmt.Sprintf("%.0f", n)
			}
		}
		out.Subtitles = append(out.Subtitles, SubtitleTrack{
			ID: id, Lan: strAt(m, "lan"), Label: label, URL: u,
			AIType: int(intAt(m, "ai_type")),
		})
	}
	return out, nil
}

// playerExtrasIdentityError 校验 player 响应身份，避免 CDN 缓存串到别的稿件。
func playerExtrasIdentityError(d map[string]any, aid, cid int64, bvid string) error {
	if got := intAt(d, "aid"); got > 0 && aid > 0 && got != aid {
		return fmt.Errorf("player 返回了另一条稿件（aid %d，请求 %d）", got, aid)
	}
	if got := intAt(d, "cid"); got > 0 && cid > 0 && got != cid {
		return fmt.Errorf("player 返回了另一个分 P（cid %d，请求 %d）", got, cid)
	}
	if got := strAt(d, "bvid"); got != "" && bvid != "" && !strings.EqualFold(got, bvid) {
		return fmt.Errorf("player 返回了另一条稿件（bvid %s，请求 %s）", got, bvid)
	}
	return nil
}

// playerExtrasURL 返回 player/v2 回退地址；WBI 主地址由 signedWBIURL
// 在请求时动态签名。
func playerExtrasURL(aid, cid int64, bvid string) string {
	query := fmt.Sprintf("cid=%d", cid)
	if aid > 0 {
		query = fmt.Sprintf("aid=%d&%s", aid, query)
	}
	if bvid != "" {
		query += "&bvid=" + url.QueryEscape(bvid)
	}
	return "https://api.bilibili.com/x/player/v2?" + query
}

// normalizeSubtitleURL 把 B 站字幕地址补成完整 URL：协议相对（//host/…）
// 补 https:，站点相对（/bfs/…）补 https://，已经是 http(s) 的原样返回。
func normalizeSubtitleURL(raw string) string {
	u := strings.TrimSpace(raw)
	switch {
	case u == "":
		return ""
	case strings.HasPrefix(u, "//"):
		return "https:" + u
	case strings.HasPrefix(u, "http://"), strings.HasPrefix(u, "https://"):
		return u
	case strings.HasPrefix(u, "/"):
		return "https://" + strings.TrimLeft(u, "/")
	default:
		return "https://" + u
	}
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
