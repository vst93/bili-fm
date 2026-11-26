package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/store"
)

// 收藏分页以接口 has_more 为准，不能按请求的 21 条推断是否到底。
func TestFavoritePaginationUsesServerHasMore(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	bilibili.UseStore(kv)
	t.Cleanup(func() { bilibili.UseStore(nil) })
	bl := bilibili.NewBL()
	bl.SetSESSDATA("SESSDATA=test")
	c := &controller{bl: bl}
	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	for _, tc := range []struct {
		name  string
		count int
		more  bool
	}{
		{"twenty_with_more", 20, true},
		{"short_page_with_more", 3, true},
		{"full_page_without_more", 20, false},
		{"twenty_one_without_more", 21, false},
		{"last_short_page", 3, false},
		{"empty_last_page", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := make([]any, tc.count)
			for i := range items {
				items[i] = map[string]any{"bvid": fmt.Sprintf("BV%04d", i)}
			}
			body, err := json.Marshal(map[string]any{
				"code": 0,
				"data": map[string]any{"medias": items, "has_more": tc.more},
			})
			if err != nil {
				t.Fatal(err)
			}
			http.DefaultTransport = &fakeTransport{fn: func(req *http.Request) *http.Response {
				if req.URL.Path != "/x/v3/fav/resource/list" || req.URL.Query().Get("media_id") != "123" || req.URL.Query().Get("pn") != "2" || req.URL.Query().Get("ps") != "20" {
					t.Fatalf("unexpected request: %s", req.URL)
				}
				return textResponse(http.StatusOK, string(body))
			}}
			cards, more, err := c.fetchSection("favorite", 2, 123, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if len(cards) != tc.count || more != tc.more {
				t.Fatalf("cards=%d more=%v, want cards=%d more=%v", len(cards), more, tc.count, tc.more)
			}
		})
	}
}
