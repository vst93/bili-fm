package bilibili

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPlayerExtrasURLPrefersBvid(t *testing.T) {
	// 多 P 视频必须带 aid + cid 定位分 P；bvid 只作为稿件身份的辅助校验。
	got := playerExtrasURL(123, 456, "BV1xx411c7mD")
	if !strings.Contains(got, "aid=123") {
		t.Fatalf("缺少 aid: %s", got)
	}
	if !strings.Contains(got, "bvid=BV1xx411c7mD") || !strings.Contains(got, "cid=456") {
		t.Fatalf("bvid/cid 参数缺失: %s", got)
	}
}

func TestPlayerExtrasURLFallsBackToAid(t *testing.T) {
	got := playerExtrasURL(123, 456, "")
	if !strings.Contains(got, "aid=123") || !strings.Contains(got, "cid=456") {
		t.Fatalf("无 bvid 时应退回 aid: %s", got)
	}
}

func TestPlayerExtrasURLOmitsZeroAid(t *testing.T) {
	got := playerExtrasURL(0, 456, "BV1xx411c7mD")
	if strings.Contains(got, "aid=0") {
		t.Fatalf("aid=0 不应写进查询: %s", got)
	}
	if !strings.Contains(got, "cid=456") || !strings.Contains(got, "bvid=BV1xx411c7mD") {
		t.Fatalf("缺 cid/bvid: %s", got)
	}
}

func TestParsePlayerExtrasAITrack(t *testing.T) {
	d := map[string]any{
		"subtitle": map[string]any{
			"subtitles": []any{
				map[string]any{
					"id_str":       "987654321",
					"lan":          "ai-zh",
					"lan_doc":      "中文（自动生成）",
					"subtitle_url": "//aisubtitle.hdslb.com/bfs/ai_subtitle/prod/abc.json",
					"ai_type":      float64(1),
				},
				map[string]any{
					"id":           float64(42),
					"lan":          "zh-CN",
					"lan_doc":      "中文（中国）",
					"subtitle_url": "/bfs/subtitle/def.json",
					"ai_type":      float64(0),
				},
			},
		},
	}
	out, err := parsePlayerExtras(d)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out.Subtitles) != 2 {
		t.Fatalf("tracks = %d, want 2: %#v", len(out.Subtitles), out.Subtitles)
	}
	ai := out.Subtitles[0]
	if ai.ID != "987654321" || ai.Lan != "ai-zh" || ai.AIType != 1 {
		t.Errorf("AI 轨解析错误: %#v", ai)
	}
	if !strings.HasPrefix(ai.URL, "https://aisubtitle.hdslb.com/") {
		t.Errorf("// 地址未补 https: %q", ai.URL)
	}
	human := out.Subtitles[1]
	if human.ID != "42" || !strings.HasPrefix(human.URL, "https://") {
		t.Errorf("相对地址未补 https: %#v", human)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// TestGetPlayerExtrasUnsignedWBICarriesAITrack 钉住字幕列表的回退顺序：
// 签名 WBI 成功但没轨时，不签名 WBI 返回的 AI 轨仍要被采用；同时验证
// 后一个源的失败不会把前面成功的结果盖成错误。
func TestGetPlayerExtrasUnsignedWBICarriesAITrack(t *testing.T) {
	oldClient := extraClient
	defer func() { extraClient = oldClient }()
	wbiCache.Lock()
	wbiCache.keys = wbiKeys{}
	wbiCache.Unlock()

	const nav = `{"code":0,"data":{"wbi_img":{"img_url":"https://i0.hdslb.com/bfs/wbi/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png","sub_url":"https://i0.hdslb.com/bfs/wbi/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png"}}}`
	const empty = `{"code":0,"data":{"aid":1,"cid":2,"bvid":"BV1","view_points":[],"subtitle":{"subtitles":[]}}}`
	const ai = `{"code":0,"data":{"aid":1,"cid":2,"bvid":"BV1","subtitle":{"subtitles":[{"id_str":"9","lan":"ai-zh","lan_doc":"中文（自动生成）","subtitle_url":"//aisubtitle.hdslb.com/x.json","ai_type":1}]}}}`
	extraClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Path, "/x/web-interface/nav"):
			return jsonResponse(nav), nil
		case strings.Contains(req.URL.Path, "/x/player/wbi/v2"):
			if req.URL.Query().Get("w_rid") != "" {
				return jsonResponse(empty), nil // 签名源成功但没有字幕
			}
			return jsonResponse(ai), nil // 不签名源有 AI 字幕
		case strings.Contains(req.URL.Path, "/x/player/v2"):
			return nil, errors.New("player/v2 should not be reached")
		default:
			return nil, fmt.Errorf("unexpected URL %s", req.URL)
		}
	})}

	bl := &BL{}
	out, err := bl.GetPlayerExtras(1, 2, "BV1")
	if err != nil {
		t.Fatalf("GetPlayerExtras: %v", err)
	}
	if len(out.Subtitles) != 1 {
		t.Fatalf("tracks = %d, want 1: %#v", len(out.Subtitles), out.Subtitles)
	}
	if got := out.Subtitles[0]; got.ID != "9" || got.Lan != "ai-zh" || got.AIType != 1 {
		t.Fatalf("AI 轨 = %#v", got)
	}
}

// TestGetPlayerExtrasStopsAtWBIWhenEmpty 钉住：WBI 成功但没有字幕轨时，
// 不再用 player/v2 的旧缓存补字幕（参考 Clipper 的「主来源成功但无字幕
// 直接判定无字幕」）。
func TestGetPlayerExtrasStopsAtWBIWhenEmpty(t *testing.T) {
	oldClient := extraClient
	defer func() { extraClient = oldClient }()
	wbiCache.Lock()
	wbiCache.keys = wbiKeys{}
	wbiCache.Unlock()

	const nav = `{"code":0,"data":{"wbi_img":{"img_url":"https://i0.hdslb.com/bfs/wbi/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png","sub_url":"https://i0.hdslb.com/bfs/wbi/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png"}}}`
	const empty = `{"code":0,"data":{"aid":1,"cid":2,"bvid":"BV1","view_points":[],"subtitle":{"subtitles":[]}}}`
	reachedPlayer := false
	extraClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Path, "/x/web-interface/nav"):
			return jsonResponse(nav), nil
		case strings.Contains(req.URL.Path, "/x/player/wbi/v2"):
			return jsonResponse(empty), nil
		case strings.Contains(req.URL.Path, "/x/player/v2"):
			reachedPlayer = true
			return jsonResponse(empty), nil
		default:
			return nil, fmt.Errorf("unexpected URL %s", req.URL)
		}
	})}

	bl := &BL{}
	out, err := bl.GetPlayerExtras(1, 2, "BV1")
	if err != nil {
		t.Fatalf("GetPlayerExtras: %v", err)
	}
	if len(out.Subtitles) != 0 {
		t.Fatalf("tracks = %d, want 0: %#v", len(out.Subtitles), out.Subtitles)
	}
	if reachedPlayer {
		t.Fatal("WBI 成功但没有字幕时不应回退 player/v2")
	}
}
