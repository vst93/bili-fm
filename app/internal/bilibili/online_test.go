package bilibili

import (
	"encoding/json"
	"testing"
)

// 真实响应样本（2026-10-29 线上抓的）：count / total 都是字符串，
// show_switch 是两个布尔开关。
const onlineTotalSample = `{"code":0,"message":"OK","ttl":1,"data":{` +
	`"total":"1000+","count":"153",` +
	`"show_switch":{"total":true,"count":true},"abtest":{"group":"b"}}}`

func TestParseOnlineViewersRealSample(t *testing.T) {
	var v map[string]any
	if err := json.Unmarshal([]byte(onlineTotalSample), &v); err != nil {
		t.Fatal(err)
	}
	got := parseOnlineViewers(dataOf(v))
	if got.Count != 153 {
		t.Fatalf("Count = %d，应为 153", got.Count)
	}
	if got.Total != "1000+" {
		t.Fatalf("Total = %q，应为 1000+", got.Total)
	}
	if !got.ShowCount || !got.ShowTotal {
		t.Fatalf("展示开关应为真: %+v", got)
	}
}

func TestParseOnlineViewersToleratesShapes(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantCount int64
		wantTotal string
		wantShow  bool
	}{
		{"数字也接受", `{"count":42,"total":"1000+","show_switch":{"count":true,"total":true}}`, 42, "1000+", true},
		{"千分位", `{"count":"1,203","show_switch":{"count":true}}`, 1203, "", true},
		{"空串", `{"count":"","show_switch":{"count":true}}`, 0, "", true},
		{"非数字", `{"count":"unknown","show_switch":{"count":true}}`, 0, "", true},
		{"开关缺失", `{"count":"153"}`, 153, "", false},
		{"开关关掉", `{"count":"153","show_switch":{"count":false,"total":true}}`, 153, "", false},
		{"data 为空", `{}`, 0, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var d map[string]any
			if err := json.Unmarshal([]byte(c.body), &d); err != nil {
				t.Fatal(err)
			}
			got := parseOnlineViewers(d)
			if got.Count != c.wantCount {
				t.Fatalf("Count = %d, want %d", got.Count, c.wantCount)
			}
			if got.Total != c.wantTotal {
				t.Fatalf("Total = %q, want %q", got.Total, c.wantTotal)
			}
			if got.ShowCount != c.wantShow {
				t.Fatalf("ShowCount = %v, want %v", got.ShowCount, c.wantShow)
			}
		})
	}
}

// TestGetOnlineViewersNeedsIdentifiers 钉住「没 cid / 没稿件号就本地判死」：
// 不能拿空参数去打接口。
func TestGetOnlineViewersNeedsIdentifiers(t *testing.T) {
	bl := NewBL()
	if _, err := bl.GetOnlineViewers("", 123, 0); err == nil {
		t.Fatal("缺 cid 应报错")
	}
	if _, err := bl.GetOnlineViewers("", 0, 123); err == nil {
		t.Fatal("bvid 与 aid 都没有时应报错")
	}
}
