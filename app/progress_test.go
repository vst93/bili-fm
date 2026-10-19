package main

import (
	"os"
	"strings"
	"testing"

	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/view"
)

// --- 源码工具（契约断言用）---
//
// 契约断言必须先剥注释、再压空白：flattenSrc 会把换行也折掉，若注释里恰好出现
// 待匹配的代码片段就会误判。

// flattenCode 读源码并「剥注释 + 压空白」。
func flattenCode(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", path, err)
	}
	return strings.Join(strings.Fields(stripComments(string(src))), "")
}

// stripComments 删掉 Go 的行注释（// 到行尾）与块注释（/* */）。
func stripComments(src string) string {
	var out []byte
	for i := 0; i < len(src); {
		switch {
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i += 2
		default:
			out = append(out, src[i])
			i++
		}
	}
	return string(out)
}

func srcContains(haystack, needle string) bool {
	return needle != "" && strings.Contains(haystack, needle)
}

// TestCloudProgressOnIncognito 钉住隐身模式的总判据：开启后所有云端进度读写
// 都关掉（原版 cloudHistoryEnabled={!isIncognitoMode}）。
func TestCloudProgressOnIncognito(t *testing.T) {
	a := &view.App{}
	c := &controller{app: a}
	if !c.cloudProgressOn() {
		t.Error("默认应开启云端进度同步")
	}
	a.Incognito = true
	if c.cloudProgressOn() {
		t.Error("隐身模式下云端进度同步应关闭")
	}
}

// TestResolveResumeIgnoresCloudWhenIncognito 钉住「隐身模式不读取云端」：
// 隐身时 resolveResume 只回本地断点，一次云端请求都不发。
func TestResolveResumeIgnoresCloudWhenIncognito(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := &view.App{Incognito: true}
	c := &controller{app: a, kv: kv}

	// 本地断点：先写一笔再读回来。
	c.saveResumePoint(mediaKey(1, 2), 42)

	if got := c.resolveResume(1, 2); got != 42 {
		t.Fatalf("隐身模式 resolveResume = %v，应为本地断点 42（且不发云端请求）", got)
	}
	if got := c.resolveResume(3, 4); got != 0 {
		t.Fatalf("无本地断点时 resolveResume = %v，应为 0", got)
	}
}

// TestProgressIncognitoSourceChain 源码级断言：云端开关只有一个判据，且所有
// 碰云端的入口都过它；ReportPlayProgress 只在一处下发。
func TestProgressIncognitoSourceChain(t *testing.T) {
	flat := flattenCode(t, "progress.go")
	for _, want := range []string{
		"func(c*controller)cloudProgressOn()bool{return!c.app.Incognito}",
		// resolveResume：不读云端。
		"if!c.cloudProgressOn()||aid==0||cid==0{",
		// trackResume / reportProgress：不上报。
		"if!c.cloudProgressOn()||c.app.Dur<=0{",
		// finishProgress：本地断点照清 + finishedKey 记账，云端受守卫。
		"c.clearResume(t.Aid,t.Cid)c.finishedKey=mediaKey(t.Aid,t.Cid)if!c.cloudProgressOn(){return}",
		"c.reportProgress(t.Aid,t.Cid,-1)",
	} {
		if !srcContains(flat, want) {
			t.Errorf("progress.go 缺少隐身/完成守卫 %q", want)
		}
	}
	// ReportPlayProgress 只有一处调用（统一由 reportProgress 守卫）。
	if n := strings.Count(flat, "c.bl.ReportPlayProgress("); n != 1 {
		t.Errorf("ReportPlayProgress 调用点 %d 处，应恰好 1 处", n)
	}
	// GetPlayProgress 只有一处调用（resolveResume 里）。
	if n := strings.Count(flat, "c.bl.GetPlayProgress("); n != 1 {
		t.Errorf("GetPlayProgress 调用点 %d 处，应恰好 1 处", n)
	}
}

// TestResumeTargetFinishedVideoStartsFromZero 钉住「历史里已听完的视频从头播」：
// 断点太靠前（< 5s）或太靠后（离结尾 < 5s）都归 0（原版 player.tsx 的
// applyPendingCloudProgress）；只有中间那段才真续播。
func TestResumeTargetFinishedVideoStartsFromZero(t *testing.T) {
	const dur = 300
	cases := []struct {
		resume, dur, want float64
	}{
		{0, dur, 0},     // 无断点
		{-1, dur, 0},    // B 站「已看完」记成 -1
		{3, dur, 0},     // 太靠前
		{4.9, dur, 0},   // 边界（< 5）
		{5, dur, 5},     // 恰好 5 秒开始续
		{295, dur, 0},   // 离结尾恰好 5 秒（看完）
		{296, dur, 0},   // 已进尾部
		{150, dur, 150}, // 中间断点照续
		{295, 0, 295},   // 时长未知时不判尾部
	}
	for _, tc := range cases {
		if got := resumeTarget(tc.resume, tc.dur); got != tc.want {
			t.Errorf("resumeTarget(%v, %v) = %v，应为 %v", tc.resume, tc.dur, got, tc.want)
		}
	}
}

// TestFinishProgressClearsResume 钉住播完清断点的行为（云端上报由源码链钉住）。
func TestFinishProgressClearsResume(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := &view.App{Incognito: true} // 隐身：清断点但不发网络请求
	a.Queue = []view.Track{{Aid: 7, Cid: 8}}
	a.Index = 0
	c := &controller{app: a, kv: kv}
	c.saveResumePoint(mediaKey(7, 8), 100)
	c.finishProgress()
	if got := c.localResumePoint(mediaKey(7, 8)); got != 0 {
		t.Errorf("播完后本地断点 = %v，应被清掉", got)
	}
}

// TestOnEndedWiringIncludesFinishProgress 钉住主窗与视频弹窗两条播完路径都补了
// finishProgress（此前只有弹窗那条，主窗音频播完只续集 —— 用户报「每个视频都
// 停在最后一点」）。
func TestOnEndedWiringIncludesFinishProgress(t *testing.T) {
	flat := flattenCode(t, "main.go")
	if n := strings.Count(flat, "c.finishProgress()"); n != 2 {
		t.Errorf("main.go 调用 finishProgress %d 处，应恰好 2 处（主窗音频 + 视频弹窗）", n)
	}
	if !srcContains(flat, "c.finishProgress()c.handleEnded()") {
		t.Error("主窗音频 OnEnded 没有先 finishProgress 再 handleEnded")
	}
	if !srcContains(flat, "c.finishProgress()c.app.Win.Update(func(){c.handleEnded()})") {
		t.Error("视频弹窗 OnEnded 没有走 finishProgress")
	}
}

// TestRewindProgressResetsResume 钉住「把进度条拖回开头 → 断点归零」：
// 原版 `<audio>` 的 onSeeked 在 currentTime ≤ 0.5 时用 allowZero 发一笔 0，
// 把服务端断点真正重置；原生版此前 `pos <= 0` 直接 return，拖回开头只改了
// 当前位置，本地/云端断点仍停在旧值 —— 下次打开又从中间续播。
func TestRewindProgressResetsResume(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := &view.App{Incognito: true} // 隐身：不发网络，本地断点照清
	a.Queue = []view.Track{{Aid: 11, Cid: 12}}
	a.Index, a.Dur = 0, 300
	c := &controller{app: a, kv: kv}

	c.saveResumePoint(mediaKey(11, 12), 120)
	c.finishedKey = mediaKey(11, 12) // 已发过「已看完」的那条也要能重听
	c.rewindProgress()

	if got := c.localResumePoint(mediaKey(11, 12)); got != 0 {
		t.Errorf("拖回开头后本地断点 = %v，应为 0", got)
	}
	if c.finishedKey != "" {
		t.Errorf("拖回开头后 finishedKey = %q，应清空（否则后续进度全被拦）", c.finishedKey)
	}
	// 反证：中间位置的一次 seek 不该清断点。
	c.saveResumePoint(mediaKey(11, 12), 120)
	c.flushProgress(120)
	if got := c.localResumePoint(mediaKey(11, 12)); got != 120 {
		t.Errorf("中间位置 seek 后本地断点 = %v，应为 120（不能被归零逻辑误伤）", got)
	}
}

// TestRunSeekRewindThreshold 源码级断言：runSeek 在 0.5 秒阈值内调
// rewindProgress，且阈值与原版 onSeeked 的 `<= 0.5` 一致。
func TestRunSeekRewindThreshold(t *testing.T) {
	flat := flattenCode(t, "main.go")
	if !srcContains(flat, "ifseconds<=rewindThresholdSeconds{c.rewindProgress()}") {
		t.Error("runSeek 没有在拖回开头（≤ 0.5s）时调 rewindProgress")
	}
	if rewindThresholdSeconds != 0.5 {
		t.Errorf("rewindThresholdSeconds = %v，原版 onSeeked 是 <= 0.5", rewindThresholdSeconds)
	}
}

// TestFinishedKeyBlocksOverwrite 钉住「播完发出的 -1 不会被紧随其后的断点补报
// 盖回去」：handleEnded → step(1) 会先 flushProgress（末尾位置），若没有这道
// 闸门，服务端看到的最后一笔又变成「已看 99%」而不是「已看完」。
func TestFinishedKeyBlocksOverwrite(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := &view.App{Incognito: true} // 隐身：不发网络，但 finishedKey 的记账照走
	a.Queue = []view.Track{{Aid: 9, Cid: 10}}
	a.Index, a.Dur = 0, 100
	c := &controller{app: a, kv: kv}

	// 播完：清断点 + 置 finishedKey。
	c.saveResumePoint(mediaKey(9, 10), 99)
	c.finishProgress()
	if c.finishedKey != mediaKey(9, 10) {
		t.Fatalf("播完后 finishedKey = %q，应为 %q", c.finishedKey, mediaKey(9, 10))
	}

	// 事后从末尾补一次断点：不该再写回去（finishedKey 拦下）。
	c.flushProgress(99)
	if got := c.localResumePoint(mediaKey(9, 10)); got != 0 {
		t.Errorf("播完后 flushProgress 又写回了断点 %v，应为 0（被 finishedKey 拦下）", got)
	}
	c.trackResume(99)
	if got := c.localResumePoint(mediaKey(9, 10)); got != 0 {
		t.Errorf("播完后 trackResume 又写回了断点 %v，应为 0", got)
	}
}
