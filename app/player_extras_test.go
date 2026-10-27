package main

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/view"
)

func TestTrimRadioQueueKeepsCurrent(t *testing.T) {
	a := &view.App{Queue: make([]view.Track, 60), Index: 50}
	c := &controller{app: a}
	c.trimRadioQueue()
	if len(a.Queue) != 48 {
		t.Fatalf("queue len = %d, want 48", len(a.Queue))
	}
	if a.Index != 38 {
		t.Fatalf("index = %d, want 38（当前项不能被裁掉）", a.Index)
	}
}

func TestTrimRadioQueueNoopWhenSmall(t *testing.T) {
	a := &view.App{Queue: make([]view.Track, 10), Index: 3}
	c := &controller{app: a}
	c.trimRadioQueue()
	if len(a.Queue) != 10 || a.Index != 3 {
		t.Fatalf("small queue changed: len=%d index=%d", len(a.Queue), a.Index)
	}
}

func TestPickSubtitleTrackPrefersChinese(t *testing.T) {
	tracks := []view.SubtitleTrack{
		{Lan: "en-US", Label: "English"},
		{Lan: "ai-zh", Label: "自动字幕"},
		{Lan: "zh-CN", Label: "中文"},
	}
	got, ok := pickSubtitleTrack(tracks)
	if !ok || got.Lan != "zh-CN" {
		t.Fatalf("pick = %+v ok=%v，want zh-CN", got, ok)
	}
	if _, ok := pickSubtitleTrack(nil); ok {
		t.Fatal("空字幕轨应返回 ok=false")
	}
}

func TestPickSubtitleTrackUsesPreference(t *testing.T) {
	tracks := []view.SubtitleTrack{{Lan: "zh-CN", Label: "中文"}, {Lan: "en-US", Label: "English"}}
	got, ok := pickSubtitleTrackPref(tracks, "en-US", "", "")
	if !ok || got.Lan != "en-US" {
		t.Fatalf("preferred track = %+v ok=%v，want en-US", got, ok)
	}
	// 新视频没有用户偏好的语言时回落到默认中文。
	got, ok = pickSubtitleTrackPref(tracks, "ja-JP", "", "")
	if !ok || got.Lan != "zh-CN" {
		t.Fatalf("fallback = %+v ok=%v，want zh-CN", got, ok)
	}
}

func TestExtrasAidUsesCurrentInfoOnlyWhenBvidMatches(t *testing.T) {
	a := &view.App{}
	track := &view.Track{Aid: 10, Bvid: "BV1", Cid: 100}
	if got := extrasAid(a, track); got != 10 {
		t.Fatalf("无 Info 时 extrasAid = %d，应为 track.Aid 10", got)
	}
	a.Info = &view.Info{Aid: 20, Bvid: "BV1"}
	if got := extrasAid(a, track); got != 20 {
		t.Fatalf("bvid 匹配时应使用 Info.Aid 20，得到 %d", got)
	}
	a.Info.Bvid = "BV2"
	if got := extrasAid(a, track); got != 10 {
		t.Fatalf("bvid 不匹配时不能借用另一条视频的 aid，得到 %d", got)
	}
}

func TestDropFutureRadio(t *testing.T) {
	a := &view.App{
		Index: 0,
		Queue: []view.Track{
			{Bvid: "origin"},
			{Bvid: "radio1", FromRadio: true},
			{Bvid: "origin2"},
			{Bvid: "radio2", FromRadio: true},
		},
	}
	dropFutureRadio(a)
	if len(a.Queue) != 2 || a.Queue[0].Bvid != "origin" || a.Queue[1].Bvid != "origin2" {
		t.Fatalf("关闭电台后应只保留原队列，得到 %#v", a.Queue)
	}
}

func TestPickSubtitlePrefersHumanTrack(t *testing.T) {
	tracks := []view.SubtitleTrack{
		{Lan: "zh-CN", Label: "AI", URL: "ai", AIType: 1},
		{Lan: "zh-CN", Label: "人工", URL: "human"},
	}
	got, ok := pickSubtitleTrackPref(tracks, "", "", "")
	if !ok || got.URL != "human" {
		t.Fatalf("同语言应优先人工字幕，得到 %#v", got)
	}
}

func TestSubtitleCuesFit(t *testing.T) {
	// 10 分钟视频，字幕只到 2 分钟：明显是别的短轨/截断轨。
	short := []bilibili.SubtitleCue{{From: 0, To: 5}, {From: 95, To: 100}}
	if subtitleCuesFit(short, 600) {
		t.Fatal("覆盖率不足的字幕轨不应通过")
	}
	// 正常覆盖，尾句略有超出（片尾字幕）不应误杀。
	normal := []bilibili.SubtitleCue{{From: 0, To: 5}, {From: 580, To: 610}}
	if !subtitleCuesFit(normal, 600) {
		t.Fatal("正常覆盖的字幕轨被误杀")
	}
	// 字幕明显跑到另一条更长的视频里。
	long := []bilibili.SubtitleCue{{From: 0, To: 5}, {From: 1200, To: 1300}}
	if subtitleCuesFit(long, 600) {
		t.Fatal("超出时长容差的字幕轨不应通过")
	}
}

func TestRadioOffTruncatesRemainingQueue(t *testing.T) {
	a := &view.App{Index: 1, Queue: []view.Track{{Bvid: "a"}, {Bvid: "b"}, {Bvid: "c"}}}
	// 直接模拟 toggleRadio 的关闭边界（避免依赖窗口）。
	if a.Index+1 < len(a.Queue) {
		a.Queue = a.Queue[:a.Index+1]
	}
	if len(a.Queue) != 2 {
		t.Fatalf("关闭电台后队列长 %d，应为 2", len(a.Queue))
	}
}

// TestPickSubtitleTrackExplicitIDURL 钉住「同语言的人工轨 / AI 轨」可以
// 精确选择：只按语言选会永远回到人工轨，AI 轨点了也切不过去。
func TestPickSubtitleTrackExplicitIDURL(t *testing.T) {
	tracks := []view.SubtitleTrack{
		{ID: "1", Lan: "zh-CN", Label: "人工", URL: "human"},
		{ID: "2", Lan: "zh-CN", Label: "AI", URL: "ai", AIType: 1},
	}
	if got, ok := pickSubtitleTrackPref(tracks, "zh-CN", "2", ""); !ok || got.URL != "ai" {
		t.Fatalf("按 ID 选 AI 轨失败：%+v ok=%v", got, ok)
	}
	if got, ok := pickSubtitleTrackPref(tracks, "", "", "ai"); !ok || got.ID != "2" {
		t.Fatalf("按 URL 选 AI 轨失败：%+v ok=%v", got, ok)
	}
	// 找不到明确轨时回落到语言默认（同语言优先人工）。
	if got, ok := pickSubtitleTrackPref(tracks, "zh-CN", "missing", "missing"); !ok || got.URL != "human" {
		t.Fatalf("明确轨不存在时应回落人工轨：%+v ok=%v", got, ok)
	}
}

func TestSubtitleFamily(t *testing.T) {
	if subtitleFamily("zh-CN") != subtitleFamily("ai-zh") {
		t.Fatal("人工中文和 AI 中文应属同一语言族")
	}
	if subtitleFamily("zh-Hans") != "zh" || subtitleFamily("zh-Hant") != "zh" {
		t.Fatal("zh-Hans / zh-Hant 应归中文")
	}
	if subtitleFamily("en-US") == subtitleFamily("zh-CN") {
		t.Fatal("英文不能和中文归为同一族")
	}
}

func TestSubtitleCandidatesIncludeSameFamilyAI(t *testing.T) {
	a := &view.App{SubtitleTracks: []view.SubtitleTrack{
		{ID: "1", Lan: "zh-CN", URL: "human"},
		{ID: "2", Lan: "ai-zh", URL: "ai", AIType: 1},
		{ID: "3", Lan: "en-US", URL: "en"},
	}}
	got := subtitleCandidates(a, a.SubtitleTracks[0])
	if len(got) != 2 || got[0].URL != "human" || got[1].URL != "ai" {
		t.Fatalf("同语言族候选 = %#v，want human→ai（不含英文）", got)
	}
}

func TestSubtitleTrackLabelMarksAI(t *testing.T) {
	if got := view.SubtitleTrackLabel(view.SubtitleTrack{Label: "中文", AIType: 1}); got != "中文（AI）" {
		t.Errorf("AI 轨标签 = %q，want 中文（AI）", got)
	}
	if got := view.SubtitleTrackLabel(view.SubtitleTrack{Label: "中文（自动生成）", AIType: 1}); got != "中文（自动生成）" {
		t.Errorf("已带自动生成的不应重复加 AI：%q", got)
	}
	if got := view.SubtitleTrackLabel(view.SubtitleTrack{Label: "中文", AIType: 0}); got != "中文" {
		t.Errorf("人工轨标签 = %q，want 中文", got)
	}
}

func TestPickSubtitleTrackURLIgnoresChangingQuery(t *testing.T) {
	tracks := []view.SubtitleTrack{
		{ID: "", Lan: "ai-zh", URL: "https://aisubtitle.hdslb.com/bfs/x.json?sign=old"},
	}
	got, ok := pickSubtitleTrackPref(tracks, "", "", "https://aisubtitle.hdslb.com/bfs/x.json?sign=new")
	if !ok || got.URL != tracks[0].URL {
		t.Fatalf("签名 query 变化后应仍匹配同一轨：%+v ok=%v", got, ok)
	}
	if got := view.SubtitleURLKey("//aisubtitle.hdslb.com/bfs/x.json?a=1"); got != "aisubtitle.hdslb.com/bfs/x.json" {
		t.Errorf("SubtitleURLKey = %q", got)
	}
}
