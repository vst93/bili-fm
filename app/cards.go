package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
		// 观看历史的条目把 bvid / aid 嵌在 history 里，不在顶层。
		var hist map[string]any
		if bvid == "" {
			if hist = subMap(m, "history"); hist != nil {
				bvid = pickStr(hist, "bvid")
			}
		}
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

		// meta 行：旧版的字段优先级是 作者 > 播放量 > 发布时间 > 附加（进度等）。
		pubdate, extra := "", ""
		if t := pickInt(m, "view_at", "pubdate", "ctime"); t > 0 {
			pubdate = relTime(t)
		}
		if p := pickInt(m, "progress"); p != 0 {
			extra = progressLabel(p, dur)
		}
		// 播放量缺了用弹幕数顶上（原版 viewsMetaField 的回退）。
		if views == "" {
			if d := pickInt(m, "danmaku"); d > 0 {
				views = fmtViews(d)
			}
			if st := subMap(m, "stat"); st != nil && views == "" {
				if d := pickInt(st, "danmaku", "reply"); d > 0 {
					views = fmtViews(d)
				}
			}
			if ci := subMap(m, "cnt_info"); ci != nil && views == "" {
				if d := pickInt(ci, "danmaku"); d > 0 {
					views = fmtViews(d)
				}
			}
		}

		aid := pickInt(m, "aid", "id")
		if aid == 0 && hist != nil {
			aid = pickInt(hist, "oid")
		}
		out = append(out, view.Card{
			Bvid:     bvid,
			Cover:    cover,
			Title:    title,
			Duration: fmtDur(dur),
			Up:       up,
			Views:    views,
			Pubdate:  pubdate,
			Extra:    extra,
			Track: view.Track{
				Aid:      aid,
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

// toUpCards 把 UP 空间（动态 feed）的条目转成卡片。
//
// 这个接口返回的是**动态卡片**而不是视频对象：视频在
// modules.module_dynamic.major.archive，作者与发布时间在 modules.module_author，
// 播放量已经是格式化好的字符串（stat.play = "69.2万"），时长在 archive.duration_text。
func toUpCards(items []any) []view.Card {
	out := make([]view.Card, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		modules := subMap(m, "modules")
		major := subMap(subMap(modules, "module_dynamic"), "major")
		arch := subMap(major, "archive")
		if arch == nil {
			continue // 图文 / 转发 / 直播等非视频动态
		}
		bvid := pickStr(arch, "bvid")
		if bvid == "" {
			continue
		}
		author := subMap(modules, "module_author")
		up := pickStr(author, "name")
		cover := pickStr(arch, "cover")
		title := pickStr(arch, "title")

		views := ""
		if st := subMap(arch, "stat"); st != nil {
			views = pickStr(st, "play")
		}

		out = append(out, view.Card{
			Bvid:     bvid,
			Cover:    cover,
			Title:    title,
			Duration: pickStr(arch, "duration_text"),
			Up:       up,
			Views:    views,
			Pubdate:  pickStr(author, "pub_time"),
			Track: view.Track{
				Aid:   pickInt(arch, "aid"),
				Bvid:  bvid,
				Title: title,
				Up:    up,
				Cover: cover,
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

// progressLabel 把稍后再看的观看进度变成「已看 30% / 已看完」。
// B 站用 progress = -1 表示已看完（直接按比例算会得到负数）。
func progressLabel(progress, duration int64) string {
	if progress < 0 {
		return "已看完"
	}
	if duration <= 0 {
		return ""
	}
	ratio := progress * 100 / duration
	if ratio > 100 {
		ratio = 100
	}
	if ratio <= 0 {
		return ""
	}
	return fmt.Sprintf("已看 %d%%", ratio)
}

// relTime 把 unix 秒变成相对时间（旧版的 formatRelativeTime）。
func relTime(unix int64) string {
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d小时前", int(d.Hours()))
	case d < 48*time.Hour:
		return "昨天"
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d天前", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d个月前", int(d.Hours()/(24*30)))
	default:
		return time.Unix(unix, 0).Format("2006-01-02")
	}
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

// metaDateText 把搜索结果里的日期（"2024-05-01 12:00" 等）转成相对时间，
// 与原版 formatMetaDate 一致；解析不出来就原样返回，空串给空。
func metaDateText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// 只取前 19 位（yyyy-MM-dd[ HH:mm[:ss]]）。
	if len(raw) > 19 {
		raw = raw[:19]
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02",
	} {
		ts, err := time.ParseInLocation(layout, raw, time.Local)
		if err != nil {
			continue
		}
		// 1970 附近的都是没拿到 pubdate 的占位（接口给 0），不显示。
		if ts.Year() <= 1971 {
			return ""
		}
		return relTime(ts.Unix())
	}
	return raw
}
