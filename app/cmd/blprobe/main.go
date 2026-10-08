// blprobe 是开发用的探针：直接调用移植过来的 B 站客户端，确认接口还能通。
//
//	go run ./cmd/blprobe search 知识
//	go run ./cmd/blprobe info BV1xx411c7mD
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/store"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "用法: blprobe <search|info|playurl|popular> <参数>")
		os.Exit(2)
	}
	kv, err := store.OpenDefault()
	if err == nil {
		bilibili.UseStore(kv)
	}
	bl := bilibili.NewBL()

	switch os.Args[1] {
	case "search":
		t0 := time.Now()
		res := bl.SearchVideo(os.Args[2], "totalrank")
		fmt.Printf("SearchVideo(%q) → %d 条，耗时 %v\n", os.Args[2], len(res), time.Since(t0))
		for i, r := range res {
			if i >= 8 {
				break
			}
			fmt.Printf("  %2d. %-14s %-6s %-10s %s\n", i+1, r.Bvid, r.Length, r.Views, truncate(r.Title, 40))
			fmt.Printf("      封面 %s\n", truncate(r.PictureURL, 70))
		}
	case "info":
		info := bl.GetCList(os.Args[2])
		fmt.Printf("GetCList(%s) → %d 个分集\n", os.Args[2], len(info.Pages))
		for i, p := range info.Pages {
			if i >= 5 {
				break
			}
			fmt.Printf("  cid=%d page=%d %s\n", p.Cid, p.Page, truncate(p.Part, 40))
		}
		if len(info.Pages) > 0 {
			u := bl.GetUrlByCid(int(info.Aid), info.Pages[0].Cid)
			fmt.Printf("播放地址: %s\n", truncate(u.URL, 90))
		}
	case "play":
		secs, sp := 8.0, 1.0
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%f", &secs)
		}
		if len(os.Args) > 4 {
			fmt.Sscanf(os.Args[4], "%f", &sp)
		}
		playVideo(bl, os.Args[2], secs, sp, len(os.Args) > 5)
	case "rcmd":
		l, err := bl.GetBLRCMDList(1)
		fmt.Printf("GetBLRCMDList err=%v\n", err)
		if l != nil {
			fmt.Printf("  items=%d\n", len(l.Items))
			if len(l.Items) > 0 {
				b, _ := json.Marshal(l.Items[0])
				fmt.Printf("  first: %s\n", string(b[:min(len(b), 320)]))
			}
		}
	case "raw":
		rawSearch(bl, os.Args[2])
	case "popular":
		pl, err := bl.GetBLPopularList(1)
		fmt.Printf("GetBLPopularList → err=%v\n", err)
		if pl != nil {
			fmt.Printf("  items=%d hasMore=%v\n", len(pl.Items), pl.HasMore)
		}
	default:
		fmt.Fprintln(os.Stderr, "未知命令", os.Args[1])
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
