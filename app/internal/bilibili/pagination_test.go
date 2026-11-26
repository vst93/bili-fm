package bilibili

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/vst93/bili-fm/app/internal/store"
)

func paginationClient(t *testing.T, respond func(*http.Request) *http.Response) *BL {
	t.Helper()
	s, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	oldStore, oldTransport, oldTicket := kv, http.DefaultTransport, Ticket
	UseStore(s)
	Ticket = "test-ticket"
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return respond(req), nil
	})
	t.Cleanup(func() { kv, http.DefaultTransport, Ticket = oldStore, oldTransport, oldTicket })
	bl := NewBL()
	bl.SetSESSDATA("SESSDATA=test")
	return bl
}

func TestSearchPaginationShortPages(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		more       bool
	}{
		{"short_with_total", `"numPages":3,"result":[{"bvid":"BV1"}]`, true},
		{"last_with_total", `"numPages":2,"result":[{"bvid":"BV1"}]`, false},
		{"short_without_total", `"result":[{"bvid":"BV1"}]`, true},
		{"empty_without_total", `"result":[]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bl := paginationClient(t, func(req *http.Request) *http.Response {
				if req.URL.Query().Get("page") != "2" {
					t.Fatalf("wrong page: %s", req.URL)
				}
				return jsonResponse(`{"code":0,"data":{` + tc.data + `}}`)
			})
			_, more, err := bl.SearchVideoPage("test", "", 2)
			if err != nil || more != tc.more {
				t.Fatalf("more=%v err=%v, want more=%v", more, err, tc.more)
			}
		})
	}
}

func TestRecommendationPaginationAdvancesRequest(t *testing.T) {
	bl := paginationClient(t, func(req *http.Request) *http.Response {
		q := req.URL.Query()
		if q.Get("fresh_idx") != "11" || q.Get("fresh_idx_1h") != "11" {
			t.Errorf("recommendation page not forwarded: %s", req.URL)
		}
		return jsonResponse(`{"code":0,"data":{"item":[{"bvid":"BV11"}]}}`)
	})
	page, err := bl.GetBLRCMDList(11)
	if err != nil || page == nil || !page.HasMore {
		t.Fatalf("page=%+v err=%v: recommendations must not stop at page 10", page, err)
	}
}

func TestSeriesDirectoryLoadsAllPages(t *testing.T) {
	calls := 0
	bl := paginationClient(t, func(req *http.Request) *http.Response {
		calls++
		page, _ := strconv.Atoi(req.URL.Query().Get("page_num"))
		if page != calls || page > 3 {
			t.Fatalf("unexpected page %d (call %d)", page, calls)
		}
		// 服务端页大小小于请求值，且中间页没有合集、只有旧版系列。
		seasons := []any{}
		series := []any{}
		if page == 2 {
			series = append(series, map[string]any{"meta": map[string]any{"series_id": 200}})
		} else {
			seasons = append(seasons, map[string]any{"meta": map[string]any{"season_id": page, "name": fmt.Sprint(page)}})
		}
		body, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]any{
			"page":        map[string]any{"page_num": page, "page_size": 1, "total": 3},
			"items_lists": map[string]any{"seasons_list": seasons, "series_list": series},
		}})
		return jsonResponse(string(body))
	})
	items, err := bl.GetSeriesList(123)
	if err != nil || calls != 3 || len(items) != 2 {
		t.Fatalf("items=%d calls=%d err=%v, want 2 seasons across 3 pages", len(items), calls, err)
	}
}

func TestSeriesDirectoryStopsOnRepeatedPage(t *testing.T) {
	calls := 0
	bl := paginationClient(t, func(req *http.Request) *http.Response {
		calls++
		if calls > 2 {
			t.Fatal("repeated page caused unbounded requests")
		}
		return jsonResponse(`{"code":0,"data":{"page":{"page_size":1,"total":99},"items_lists":{"seasons_list":[{"meta":{"season_id":1}}]}}}`)
	})
	items, err := bl.GetSeriesList(123)
	if err != nil || len(items) != 1 || calls != 2 {
		t.Fatalf("items=%d calls=%d err=%v", len(items), calls, err)
	}
}

func TestOtherListPaginationMetadata(t *testing.T) {
	t.Run("popular_short_page", func(t *testing.T) {
		bl := paginationClient(t, func(req *http.Request) *http.Response {
			if req.URL.Query().Get("pn") != "2" {
				t.Fatalf("wrong page: %s", req.URL)
			}
			return jsonResponse(`{"code":0,"data":{"list":[{"bvid":"BV1"}],"no_more":false}}`)
		})
		page, err := bl.GetBLPopularList(2)
		if err != nil || !page.HasMore || len(page.Items) != 1 {
			t.Fatalf("page=%+v err=%v", page, err)
		}
	})
	for _, source := range []string{"feed", "up"} {
		t.Run(source+"_short_page", func(t *testing.T) {
			bl := paginationClient(t, func(req *http.Request) *http.Response {
				if req.URL.Query().Get("offset") != "old" {
					t.Fatalf("wrong cursor: %s", req.URL)
				}
				return jsonResponse(`{"code":0,"data":{"items":[{}],"has_more":true,"offset":"next"}}`)
			})
			var page *FeedList
			var err error
			if source == "feed" {
				page, err = bl.GetBLFeedList("old")
			} else {
				page, err = bl.GetUpVideoList(123, "old")
			}
			if err != nil || !page.HasMore || len(page.Items) != 1 || page.Offset != "next" {
				t.Fatalf("page=%+v err=%v", page, err)
			}
		})
	}
	t.Run("history_short_page", func(t *testing.T) {
		bl := paginationClient(t, func(req *http.Request) *http.Response {
			q := req.URL.Query()
			if q.Get("max") != "100" || q.Get("view_at") != "200" {
				t.Fatalf("wrong cursor: %s", req.URL)
			}
			return jsonResponse(`{"code":0,"data":{"list":[{"bvid":"BV1"}],"cursor":{"max":99,"view_at":199,"business":"archive"}}}`)
		})
		page, err := bl.GetBLHistoryList(100, 200, "archive", 30)
		if err != nil || len(page.List) != 1 || page.Cursor.Max != 99 || page.Cursor.ViewAt != 199 {
			t.Fatalf("page=%+v err=%v", page, err)
		}
	})
	t.Run("series_videos_short_page", func(t *testing.T) {
		bl := paginationClient(t, func(req *http.Request) *http.Response {
			if req.URL.Query().Get("page_num") != "2" || req.URL.Query().Get("season_id") != "456" {
				t.Fatalf("wrong page: %s", req.URL)
			}
			return jsonResponse(`{"code":0,"data":{"archives":[{"bvid":"BV1"}]}}`)
		})
		items, err := bl.GetSeriesVideos(123, 456, 2)
		if err != nil || len(items) != 1 {
			t.Fatalf("items=%+v err=%v", items, err)
		}
	})
}

func TestSeriesDirectoryPaginationFallbackAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		second    string
		wantCount int
		wantErr   bool
	}{
		{"empty_page", `{"code":0,"data":{"items_lists":{"seasons_list":[]}}}`, 1, false},
		{"overlapping_last_page", `{"code":0,"data":{"page":{"page_size":2,"total":3},"items_lists":{"seasons_list":[{"meta":{"season_id":1}},{"meta":{"season_id":2}}]}}}`, 2, false},
		{"error_not_partial_success", `{"code":-500,"message":"failed"}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			bl := paginationClient(t, func(req *http.Request) *http.Response {
				calls++
				if calls > 2 {
					t.Fatal("unexpected request after final page")
				}
				if calls == 1 {
					// 旧响应无分页元数据，短页也应继续。
					return jsonResponse(`{"code":0,"data":{"items_lists":{"seasons_list":[{"meta":{"season_id":1}}]}}}`)
				}
				return jsonResponse(tc.second)
			})
			items, err := bl.GetSeriesList(123)
			if (err != nil) != tc.wantErr || len(items) != tc.wantCount || calls != 2 {
				t.Fatalf("items=%d err=%v calls=%d", len(items), err, calls)
			}
		})
	}
}
