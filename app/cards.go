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

// pickCount 与 pickInt 类似，但把「字段缺失」与「真实 0」区分开：命中时连值
// 一起返回布尔真，一个都没命中才返回假。
//
// 原版 viewsMetaField（修复轮 22 的问题 1）靠这个区分：字段存在就照实渲染
// （真实 0 也显示 0），只有字段缺失才回退弹幕数。用 `n > 0` 判断会把真实 0
// 也当成缺失。空串 / 无法解析成数字的串都按缺失处理（原版 toFiniteOrNull 对
// "" / NaN 返回 null）。
func pickCount(m map[string]any, keys ...string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	for _, k := range keys {
		switch x := m[k].(type) {
		case float64:
			return int64(x), true
		case int64:
			return x, true
		case string:
			if strings.TrimSpace(x) == "" {
				continue
			}
			var n int64
			if _, err := fmt.Sscanf(x, "%d", &n); err == nil {
				return n, true
			}
		}
	}
	return 0, false
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
		// 播放量：字段**存在**就照实渲染（真实 0 也显示 0），只有字段缺失才
		// 回退弹幕数——原版 viewsMetaField（修复轮 22 的问题 1）。此前用 `n > 0`
		// 判断，把「真实 0」也当成缺失，于是有 0 播放的视频会顶着弹幕数挂评论
		// 图标（正是原版专门修掉的混同）。
		views := ""
		if n, ok := pickCount(subMap(m, "stat"), "view"); ok {
			views = fmtViews(n)
		}
		if views == "" {
			if n, ok := pickCount(subMap(m, "cnt_info"), "play"); ok {
				views = fmtViews(n)
			}
		}
		if views == "" {
			if n, ok := pickCount(m, "play", "view"); ok {
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
		// 播放量缺失时才用弹幕数顶上（原版 viewsMetaField 的回退）；这时 meta
		// 行的前导图标要换成评论图标，否则「弹幕数挂播放图标」语义错位。
		// 回退取值的来源顺序与上面播放量的取值一致（stat → cnt_info → 顶层），
		// 原版每个列表的 viewsMetaField 也是从**同一个来源对象**同时取 play
		// 与 danmaku。
		viewsFromDanmaku := false
		if views == "" {
			for _, src := range []map[string]any{subMap(m, "stat"), subMap(m, "cnt_info"), m} {
				if d, ok := pickCount(src, "danmaku", "reply"); ok {
					views, viewsFromDanmaku = fmtViews(d), true
					break
				}
			}
		}

		aid := pickInt(m, "aid", "id")
		if aid == 0 && hist != nil {
			aid = pickInt(hist, "oid")
		}
		out = append(out, view.Card{
			Bvid:         bvid,
			Cover:        cover,
			Title:        title,
			Duration:     fmtDur(dur),
			Up:           up,
			Views:        views,
			ViewsDanmaku: viewsFromDanmaku,
			Pubdate:      pubdate,
			Extra:        extra,
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
//
// UP 空间列表**不渲染**标题前胶囊（原版 upVideoList 的 ListCard 不传 titlePrefix），
// 需要胶囊的推荐用 toFeedCards。
func toUpCards(items []any) []view.Card { return upCards(items, false) }

// toFeedCards 与 toUpCards 同源，但会额外解析动态卡片的「充电专属」标题胶囊
// （原版只有 feedList 的 ListCard 传 titlePrefix）。
func toFeedCards(items []any) []view.Card { return upCards(items, true) }

// specialBadges 是列表卡片会透传为标题前胶囊的「特殊徽标」白名单。
//
// 原版 feedList 的判据是 `archive.badge.text === '充电专属'`（唯一一种）；
// 这里再加上用户点名的「合作/联合投稿」类特殊徽标。普通投稿的来源分类
// （「投稿视频」「动态视频」等）不在白名单里，因此不会显示。
//
// 白名单而不是黑名单：B 站以后新增任何来源分类文案都会被挡掉，只有明确
// 认为是「特殊」的才会显示。
var specialBadges = map[string]bool{
	"充电专属": true,
	"合作视频": true,
	"联合投稿": true,
}

// specialBadge 只在 text 是白名单里的特殊徽标时返回它，否则返回空串。
func specialBadge(text string) string {
	if specialBadges[text] {
		return text
	}
	return ""
}

func upCards(items []any, withBadge bool) []view.Card {
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
		// 标题前的小胶囊（原版 feedList 的 titlePrefix）。
		//
		// B 站的 archive.badge.text 对**普通投稿**会给出「投稿视频」「动态视频」
		// 这类**来源分类**标签，而原版 feedList 只在 `badge.text === '充电专属'`
		// 时才渲染胶囊（src/components/feedList.tsx:98），从不显示来源分类。
		// 用户也明确要求「普通视频不要打投稿/动态标签，只有特殊的才打」。
		// 所以不直接透传 badge.text，而是走 specialBadge 白名单。
		badge := ""
		if withBadge {
			badge = specialBadge(pickStr(subMap(arch, "badge"), "text"))
		}

		// 播放量：archive.stat.play 已是格式化好的字符串（如 "69.2万"），字段
		// 存在就照实取（真实 0 会得到 "0"，也是非空串）。缺失时才回退弹幕数
		// （原版 feedList / upVideoList 的 viewsMetaField(stat.play, stat.danmaku)），
		// 并标记 ViewsDanmaku 让 meta 行换评论图标。pickStr 对空串视为未命中，
		// 所以显式空串也走回退。
		views, viewsFromDanmaku := "", false
		if st := subMap(arch, "stat"); st != nil {
			views = pickStr(st, "play")
			if views == "" {
				if d := pickStr(st, "danmaku", "reply"); d != "" {
					views, viewsFromDanmaku = d, true
				}
			}
		}

		out = append(out, view.Card{
			Bvid:         bvid,
			Cover:        cover,
			Title:        title,
			Badge:        badge,
			Duration:     pickStr(arch, "duration_text"),
			Up:           up,
			Views:        views,
			ViewsDanmaku: viewsFromDanmaku,
			Pubdate:      metaDateText(pickStr(author, "pub_time")),
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

// relTime 把 unix 秒变成相对时间，对齐原版 dateFormLadder 的「最具体形态」：
//
//	≤0 天 → 今天；1 天 → 昨天；≤7 天 → N天前；
//	当年 → MM-DD（比相对形态更具体）；跨年 → yyyy-MM-DD（原版轮 30 的「具体
//	日期优先」，不再一律压到 N个月前/N年前）。
//
// 放不下时的逐档降级（yyyy-MM-DD → yyyy-MM → N个月前 → N年前）由卡片 meta
// 层按实测列宽完成（见 view.pubdateCandidates）。
func relTime(unix int64) string {
	if unix <= 0 {
		return ""
	}
	target := time.Unix(unix, 0)
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	startOfTarget := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, now.Location()).Unix()
	dayDiff := (startOfToday - startOfTarget) / 86400
	switch {
	case dayDiff <= 0:
		return "今天"
	case dayDiff == 1:
		return "昨天"
	case dayDiff <= 7:
		return fmt.Sprintf("%d天前", dayDiff)
	}

	if target.Year() == now.Year() {
		return target.Format("01-02")
	}
	// 跨年：尽量给出完整年月日，列放不下时由 meta 层降级。
	return target.Format("2006-01-02")
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

// fmtViews 把播放量转成短文本（原版 formatViewCount）：
// ≥100万 → 整数万（539万），≥1万 → 一位小数（4.1万）。
func fmtViews(n int64) string {
	switch {
	case n >= 100000000:
		return fmt.Sprintf("%.1f亿", float64(n)/1e8)
	case n >= 1000000:
		return fmt.Sprintf("%d万", n/10000)
	case n >= 10000:
		return fmt.Sprintf("%.1f万", float64(n)/1e4)
	default:
		return fmt.Sprint(n)
	}
}

// fmtCompactCount 把三键数值（点赞/投币/收藏）转成短文本（原版
// formatCompactCount）：≥1亿 → x.x亿，≥100万 → 整数 w，≥1万 → x.x w。
func fmtCompactCount(n int64) string {
	switch {
	case n >= 100000000:
		return fmt.Sprintf("%.1f亿", float64(n)/1e8)
	case n >= 1000000:
		return fmt.Sprintf("%dw", n/10000)
	case n >= 10000:
		return fmt.Sprintf("%.1fw", float64(n)/1e4)
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
