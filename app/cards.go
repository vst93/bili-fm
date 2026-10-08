package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vst93/bili-fm/app/internal/view"
)

// 各列表接口返回的条目字段名不完全一致（推荐/热门用 owner.name，收藏用
// upper.name，历史用 author_name），所以这里统一做一层宽松取值。

func pickStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case string:
				if x != "" {
					return x
				}
			case float64:
				return fmt.Sprintf("%.0f", x)
			}
		}
	}
	return ""
}

func pickInt(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		switch x := m[k].(type) {
		case float64:
			return int64(x)
		case int64:
			return x
		case string:
			var n int64
			if _, err := fmt.Sscanf(x, "%d", &n); err == nil {
				return n
			}
		}
	}
	return 0
}

func subMap(m map[string]any, key string) map[string]any {
	if s, ok := m[key].(map[string]any); ok {
		return s
	}
	return nil
}

// toCards 把任意列表接口的条目转成界面卡片。识别不了的条目跳过。
func toCards(items []any) []view.Card {
	out := make([]view.Card, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		bvid := pickStr(m, "bvid")
		if bvid == "" {
			continue // 不是视频（活动卡片、直播等）
		}
		cover := pickStr(m, "pic", "cover", "first_frame")
		title := pickStr(m, "title", "name")
		up := pickStr(m, "author", "author_name", "uname")
		if up == "" {
			if o := subMap(m, "owner"); o != nil {
				up = pickStr(o, "name")
			}
			if o := subMap(m, "upper"); o != nil {
				up = pickStr(o, "name")
			}
		}
		views := ""
		if s := subMap(m, "stat"); s != nil {
			if n := pickInt(s, "view"); n > 0 {
				views = fmtViews(n)
			}
		}
		if s := subMap(m, "cnt_info"); s != nil && views == "" {
			if n := pickInt(s, "play"); n > 0 {
				views = fmtViews(n)
			}
		}
		if views == "" {
			if n := pickInt(m, "play", "view"); n > 0 {
				views = fmtViews(n)
			}
		}
		dur := pickInt(m, "duration", "length")
		out = append(out, view.Card{
			Bvid:     bvid,
			Cover:    cover,
			Title:    title,
			Duration: fmtDur(dur),
			Meta:     []string{up, views},
			Track: view.Track{
				Aid:      pickInt(m, "aid", "id"),
				Bvid:     bvid,
				Cid:      pickInt(m, "cid"),
				Title:    title,
				Up:       up,
				Cover:    cover,
				Duration: dur,
			},
		})
	}
	return out
}

// toCardsFromRaw 用于返回 []interface{} 的接口（推荐 / 收藏 / 合集 / UP 主）。
func toCardsFromRaw(raw []any) []view.Card { return toCards(raw) }

// jsonToAny 把结构体切片转成 []any，便于复用上面的宽松取值。
func jsonToAny(v any) []any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out []any
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

// fmtDur 把秒转成 mm:ss / h:mm:ss。
func fmtDur(sec int64) string {
	if sec <= 0 {
		return ""
	}
	h, m, s := sec/3600, (sec%3600)/60, sec%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// fmtViews 把播放量转成「1.2万」这样的短文本。
func fmtViews(n int64) string {
	switch {
	case n >= 100000000:
		return fmt.Sprintf("%.1f亿", float64(n)/1e8)
	case n >= 10000:
		return fmt.Sprintf("%.1f万", float64(n)/1e4)
	default:
		return fmt.Sprint(n)
	}
}

// stripHTML 去掉搜索结果标题里的 <em> 高亮（api.go 已处理，这里兜底）。
func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<em class=\"keyword\">", "")
	return strings.ReplaceAll(s, "</em>", "")
}
