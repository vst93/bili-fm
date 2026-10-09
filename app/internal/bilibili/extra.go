package bilibili

// 本文件是 Rust 版有、Wails 版 Go 代码没有的接口：稍后再看、收藏状态、
// SponsorBlock 跳过。写法与 api.go 保持一致（同一个 BL 类型、同一套 cookie）。

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ---------------------------------------------------------------- 通用请求

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

var extraClient = &http.Client{Timeout: 20 * time.Second}

// csrfFrom 从 cookie 里取 bili_jct，写操作都要它。
func csrfFrom(cookie string) (string, error) {
	const key = "bili_jct="
	i := strings.Index(cookie, key)
	if i < 0 {
		return "", errors.New("无法获取 csrf")
	}
	rest := cookie[i+len(key):]
	if j := strings.Index(rest, ";"); j >= 0 {
		rest = rest[:j]
	}
	if rest == "" {
		return "", errors.New("无法获取 csrf")
	}
	return rest, nil
}

func doJSON(method, rawURL, cookie string) (map[string]any, error) {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("Origin", "https://www.bilibili.com")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := extraClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if code, _ := v["code"].(float64); code != 0 {
		msg, _ := v["message"].(string)
		if msg == "" {
			msg = fmt.Sprintf("code=%d", int(code))
		}
		return nil, errors.New(msg)
	}
	return v, nil
}

func dataOf(v map[string]any) map[string]any {
	if d, ok := v["data"].(map[string]any); ok {
		return d
	}
	return map[string]any{}
}

func arrOf(m map[string]any, key string) []any {
	if a, ok := m[key].([]any); ok {
		return a
	}
	return nil
}

func strAt(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func intAt(m map[string]any, key string) int64 {
	switch n := m[key].(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	}
	return 0
}

// ---------------------------------------------------------------- 播放进度

// 与 src-tauri/src/bilibili.rs 的 get_play_progress 对齐。
const (
	// 回退翻历史的页数上限（player/v2 偶尔失败，或返回的是账号最近看的
	// **别的**分集）。
	progressHistoryPages = 5
)

// GetPlayProgress 读取这个视频在云端的上次观看位置（秒），用于续播。
//
// 先问 x/player/v2 的 last_play_time（毫秒）/ last_play_cid —— 只有 cid 对得上
// （或没记录过 cid）才认；再翻最多 5 页观看历史，找 aid+cid 都匹配的那条取
// progress。调用方自己有 800ms 起播预算，所以这里不该慢。
func (bl *BL) GetPlayProgress(aid, cid int64) (int, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return 0, errors.New("未登录")
	}
	if aid <= 0 || cid <= 0 {
		return 0, errors.New("未选择作品")
	}

	playerURL := fmt.Sprintf("https://api.bilibili.com/x/player/v2?aid=%d&cid=%d", aid, cid)
	if v, err := doJSON("GET", playerURL, cookie); err == nil {
		d := dataOf(v)
		lastCid := intAt(d, "last_play_cid")
		progress := intAt(d, "last_play_time") / 1000
		if progress > 0 && (lastCid == 0 || lastCid == cid) {
			return int(progress), nil
		}
	}

	var cursorMax, cursorViewAt int64
	for i := 0; i < progressHistoryPages; i++ {
		historyURL := fmt.Sprintf(
			"https://api.bilibili.com/x/web-interface/history/cursor"+
				"?type=archive&max=%d&view_at=%d&business=archive&ps=50",
			cursorMax, cursorViewAt)
		v, err := doJSON("GET", historyURL, cookie)
		if err != nil {
			return 0, err
		}
		d := dataOf(v)
		items := arrOf(d, "list")
		for _, it := range items {
			m, ok := it.(map[string]any)
			if !ok {
				continue
			}
			h, _ := m["history"].(map[string]any)
			if h == nil {
				continue
			}
			if intAt(h, "oid") != aid || intAt(h, "cid") != cid {
				continue
			}
			if p := intAt(m, "progress"); p > 0 {
				return int(p), nil
			}
			return 0, nil
		}

		nextMax, nextViewAt := int64(0), int64(0)
		if c, ok := d["cursor"].(map[string]any); ok {
			nextMax, nextViewAt = intAt(c, "max"), intAt(c, "view_at")
		}
		// 没更多了，或者游标没动（防死循环）。
		if len(items) == 0 || (nextMax == cursorMax && nextViewAt == cursorViewAt) || nextViewAt <= 0 {
			break
		}
		cursorMax, cursorViewAt = nextMax, nextViewAt
	}
	return 0, nil
}

// ---------------------------------------------------------------- 稍后再看

// WatchLaterItem 是稍后再看里的一条。
type WatchLaterItem struct {
	Aid      int64  `json:"aid"`
	Bvid     string `json:"bvid"`
	Title    string `json:"title"`
	Pic      string `json:"pic"`
	Duration int64  `json:"duration"`
	Owner    struct {
		Name string `json:"name"`
	} `json:"owner"`
	Stat struct {
		View int64 `json:"view"`
	} `json:"stat"`
}

// WatchLaterList 是稍后再看列表。
type WatchLaterList struct {
	List []WatchLaterItem `json:"list"`
}

// GetWatchLaterList 取稍后再看。未登录时返回空列表。
func (bl *BL) GetWatchLaterList() (*WatchLaterList, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return &WatchLaterList{}, nil
	}
	v, err := doJSON("GET", "https://api.bilibili.com/x/v2/history/toview?jsonp=jsonp", cookie)
	if err != nil {
		return nil, err
	}
	out := &WatchLaterList{}
	raw, _ := json.Marshal(arrOf(dataOf(v), "list"))
	_ = json.Unmarshal(raw, &out.List)
	return out, nil
}

// AddToWatchLater 加入稍后再看。
func (bl *BL) AddToWatchLater(aid int64) error {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return errors.New("未登录")
	}
	csrf, err := csrfFrom(cookie)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://api.bilibili.com/x/v2/history/toview/add?aid=%d&csrf=%s", aid, url.QueryEscape(csrf))
	_, err = doJSON("POST", u, cookie)
	return err
}

// RemoveFromWatchLater 从稍后再看移除。
func (bl *BL) RemoveFromWatchLater(aid int64) error {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return errors.New("未登录")
	}
	csrf, err := csrfFrom(cookie)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://api.bilibili.com/x/v2/history/toview/del?aid=%d&csrf=%s", aid, url.QueryEscape(csrf))
	_, err = doJSON("POST", u, cookie)
	return err
}

// ---------------------------------------------------------------- 收藏状态

// favFoldersFor 返回该视频在各收藏夹里的状态（fav_state=1 表示已收藏）。
func (bl *BL) favFoldersFor(aid int64, cookie string) ([]map[string]any, error) {
	mid, _ := NumberToString(GetItem("mid"))
	u := fmt.Sprintf("https://api.bilibili.com/x/v3/fav/folder/created/list-all?up_mid=%s&type=2&rid=%d", mid, aid)
	v, err := doJSON("GET", u, cookie)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, it := range arrOf(dataOf(v), "list") {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

func favFolderID(m map[string]any) int64 {
	if id := intAt(m, "id"); id > 0 {
		return id
	}
	return intAt(m, "fid")
}

// HasFavorite 报告该视频是否已收藏。
func (bl *BL) HasFavorite(aid int64) (bool, error) {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return false, errors.New("未登录")
	}
	if aid <= 0 {
		return false, errors.New("未选择作品")
	}
	folders, err := bl.favFoldersFor(aid, cookie)
	if err != nil {
		return false, err
	}
	for _, f := range folders {
		if intAt(f, "fav_state") == 1 {
			return true, nil
		}
	}
	return false, nil
}

// SetFavorite 收藏（加入默认收藏夹）或取消收藏（从所有已包含它的收藏夹移除）。
func (bl *BL) SetFavorite(aid int64, favorite bool) error {
	cookie := bl.GetSESSDATA()
	if cookie == "" {
		return errors.New("未登录")
	}
	if aid <= 0 {
		return errors.New("未选择作品")
	}
	folders, err := bl.favFoldersFor(aid, cookie)
	if err != nil {
		return err
	}
	var selected []string
	for _, f := range folders {
		if intAt(f, "fav_state") == 1 {
			if id := favFolderID(f); id > 0 {
				selected = append(selected, fmt.Sprint(id))
			}
		}
	}

	var addIDs, delIDs string
	if favorite {
		if len(selected) > 0 {
			return nil // 已经收藏
		}
		if len(folders) == 0 {
			return errors.New("没有可用的收藏夹")
		}
		id := favFolderID(folders[0])
		if id <= 0 {
			return errors.New("没有可用的收藏夹")
		}
		addIDs = fmt.Sprint(id)
	} else {
		if len(selected) == 0 {
			return nil // 本来就没收藏
		}
		delIDs = strings.Join(selected, ",")
	}

	csrf, err := csrfFrom(cookie)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://api.bilibili.com/x/v3/fav/resource/deal?rid=%d&type=2&add_media_ids=%s&del_media_ids=%s&csrf=%s",
		aid, url.QueryEscape(addIDs), url.QueryEscape(delIDs), url.QueryEscape(csrf))
	_, err = doJSON("POST", u, cookie)
	return err
}

// ---------------------------------------------------------------- SponsorBlock

// SponsorSegment 是一段要跳过的赞助/广告区间（秒）。
type SponsorSegment struct {
	Category string  `json:"category"`
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
}

// 独立且超时短的 client：SponsorBlock 是可选增强，绝不能让慢接口拖住播放。
var sponsorClient = &http.Client{Timeout: 5 * time.Second}

// GetSponsorSegments 取该视频的社区标记跳过分段。失败一律返回空切片：
// 这是可选功能，不该因为网络问题影响播放。
func (bl *BL) GetSponsorSegments(bvid string, cid int64) []SponsorSegment {
	out, _ := bl.FetchSponsorSegments(bvid, cid)
	return out
}

// FetchSponsorSegments 与 GetSponsorSegments 相同，但把错误回给调用方，
// 以便界面区分「查询失败」和「确实没有分段」（原版的绿/黄/红状态点）。
func (bl *BL) FetchSponsorSegments(bvid string, cid int64) ([]SponsorSegment, error) {
	if bvid == "" || cid <= 0 {
		return nil, nil
	}
	u := fmt.Sprintf("https://bsbsb.top/api/skipSegments?videoID=%s&cid=%d", url.QueryEscape(bvid), cid)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chromeUA)
	resp, err := sponsorClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sponsorblock 返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	// 服务端可能返回对象数组或分段数组，两种都试。
	var raw []struct {
		Category string          `json:"category"`
		Segment  []float64       `json:"segment"`
		Action   string          `json:"action"`
		Segments [][]interface{} `json:"segments"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	var out []SponsorSegment
	for _, r := range raw {
		if len(r.Segment) == 2 {
			out = append(out, SponsorSegment{Category: r.Category, Start: r.Segment[0], End: r.Segment[1]})
			continue
		}
		// 旧格式：segments 是 [[start, end], ...]
		for _, s := range r.Segments {
			if len(s) != 2 {
				continue
			}
			a, ok1 := s[0].(float64)
			b, ok2 := s[1].(float64)
			if ok1 && ok2 {
				out = append(out, SponsorSegment{Category: r.Category, Start: a, End: b})
			}
		}
	}
	return out, nil
}
