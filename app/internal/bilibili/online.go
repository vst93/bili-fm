package bilibili

// 当前观看人数（x/player/online/total）。
//
// 这就是 B 站网页播放器里那个「N 人在看」气泡的数据源：
//
//	GET https://api.bilibili.com/x/player/online/total?bvid=BV..&cid=..
//	{"code":0,"data":{"total":"1000+","count":"153",
//	  "show_switch":{"total":true,"count":true},"abtest":{"group":"b"}}}
//
//   - count：此刻正在看**这一集**的人数，实时变化（实测同一视频 3s 内
//     12 → 152 → 13）；
//   - total：累计观看数的**已格式化**文本（"1000+" 这种量级打码，不是精确数）；
//   - show_switch：B 站侧对这两个数各自的展示开关（部分稿件/地区会关掉）。
//
// 用 bvid 或 aid 都行，未登录也能拿，响应几十字节，适合周期性轮询。
//
// 为什么不用 x/player/v2 的 `online_count`：那是播放器配置快照里的字段，
// 实测未登录时恒为 1（一部百万播放的视频也是 1），拿它当「当前观看人数」
// 会一直显示「1 人在看」。

import (
	"fmt"
	"strconv"
)

// OnlineViewers 是一部视频**当前**的观看人数。
type OnlineViewers struct {
	// Count 是此刻正在看这一集的人数。接口没给 / 给了非数字时为 0，
	// 调用方按「不显示」处理。
	Count int64
	// Total 是 B 站格式化过的累计观看数文本（如 "1000+"）；可能为空。
	Total string
	// ShowCount / ShowTotal 是 B 站侧的展示开关。关掉时接口仍可能返回数字，
	// 但官方界面不展示 —— 本项目尊重这个开关，不显示被关掉的那一项。
	ShowCount bool
	ShowTotal bool
}

// GetOnlineViewers 读取当前观看人数。bvid 优先，没有 bvid 时用 aid；
// cid 必填（这是**分集**级的人数，不是整支稿件的）。
func (bl *BL) GetOnlineViewers(bvid string, aid, cid int64) (*OnlineViewers, error) {
	if cid <= 0 {
		return nil, fmt.Errorf("未选择作品")
	}
	if bvid == "" && aid <= 0 {
		return nil, fmt.Errorf("未选择作品")
	}
	u := "https://api.bilibili.com/x/player/online/total?cid=" + strconv.FormatInt(cid, 10)
	if bvid != "" {
		u += "&bvid=" + bvid
	} else {
		u += "&aid=" + strconv.FormatInt(aid, 10)
	}
	v, err := doJSON("GET", u, bl.GetSESSDATA())
	if err != nil {
		return nil, err
	}
	return parseOnlineViewers(dataOf(v)), nil
}

// parseOnlineViewers 解析已经取到的 data 段。count / total 都是**字符串**
// （count 也可能哪天改成数字），所以数值解析要容错：拿不到就当 0。
func parseOnlineViewers(d map[string]any) *OnlineViewers {
	out := &OnlineViewers{}
	switch n := d["count"].(type) {
	case string:
		// 允许千分位与空白（B 站的计数字符串偶尔带格式）。
		s := make([]rune, 0, len(n))
		for _, r := range n {
			if r >= '0' && r <= '9' {
				s = append(s, r)
			}
		}
		if len(s) > 0 {
			if v, err := strconv.ParseInt(string(s), 10, 64); err == nil {
				out.Count = v
			}
		}
	case float64:
		out.Count = int64(n)
	case int64:
		out.Count = n
	}
	out.Total = strAt(d, "total")
	if sw, ok := d["show_switch"].(map[string]any); ok {
		// 字段缺失时按「不展示」处理，避免把接口的默认值当承诺。
		out.ShowCount, _ = sw["count"].(bool)
		out.ShowTotal, _ = sw["total"].(bool)
	}
	return out
}
