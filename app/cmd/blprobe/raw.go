package main

// raw 命令：把搜索接口的原始 JSON 打出来，用于核对字段名（接口会变）。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/vst93/bili-fm/app/internal/bilibili"
)

func rawSearch(bl *bilibili.BL, keyword string) {
	ticket, err := bl.GetBiliTicket("")
	fmt.Println("bili_ticket:", err, len(ticket))
	u := fmt.Sprintf("https://api.bilibili.com/x/web-interface/wbi/search/type?search_type=video&page=1&page_size=3&order=totalrank&keyword=%s",
		url.QueryEscape(keyword))
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("accept", "*/*")
	req.Header.Set("origin", "https://search.bilibili.com")
	req.Header.Set("referer", "https://search.bilibili.com/video")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.62")
	req.Header.Set("Cookie", "bili_ticket="+ticket)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("请求失败:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var v map[string]any
	json.Unmarshal(body, &v)
	fmt.Println("code:", v["code"], "message:", v["message"])
	data, _ := v["data"].(map[string]any)
	if data == nil {
		fmt.Println(string(body[:min(len(body), 400)]))
		return
	}
	items, _ := data["result"].([]any)
	fmt.Println("result 条数:", len(items))
	if len(items) > 0 {
		first, _ := items[0].(map[string]any)
		keys := make([]string, 0, len(first))
		for k := range first {
			keys = append(keys, k)
		}
		sortStrings(keys)
		fmt.Println("字段:", keys)
		for _, k := range []string{"bvid", "aid", "title", "pic", "author", "play", "duration", "length", "pubdate", "video_review"} {
			fmt.Printf("  %-13s = %v\n", k, first[k])
		}
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
