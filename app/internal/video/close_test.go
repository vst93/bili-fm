package video

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestResumeOnClose 钉住「关掉视频窗之后要不要把音频接回来」的判据。
//
// 原版基准（tauri 分支 src/components/playerVideo.tsx 的 closePlayerVideo）：
// 关窗**不**恢复音频播放，由用户自己点播放键。本仓库的音视接力是「播放中
// 关窗才接回」（webview 路径的既有行为），暂停时绝不自动起播 —— 用户报的
// 「视频暂停时关窗，声音出来了」正是判成了「在播」。
func TestResumeOnClose(t *testing.T) {
	cases := []struct {
		name   string
		st     State
		relay  bool
		reason string
	}{
		{
			name:  "播放中关窗",
			st:    State{Time: 42, Duration: 300, Paused: false, Ready: 4},
			relay: true,
		},
		{
			name:   "暂停时关窗",
			st:     State{Time: 42, Duration: 300, Paused: true, Ready: 4},
			relay:  false,
			reason: "暂停是用户的明确意图，关窗不是开始播放的手势",
		},
		{
			name:   "暂停且页面状态还没上报",
			st:     State{}, // 零值：Paused=false 但 Ready=0
			relay:  false,
			reason: "没就绪的视频不能接（会是静音 / 错误）",
		},
		{
			name:   "起播失败",
			st:     State{Time: 0, Paused: false, Ready: 2, Err: "decode"},
			relay:  false,
			reason: "出错的视频接回来只会是一段错误",
		},
		{
			name:   "自然播完（暂停态）",
			st:     State{Time: 299.9, Duration: 300, Paused: true, Ready: 4},
			relay:  false,
			reason: "片尾不应被当成续播点",
		},
		{
			name:   "自然播完（界面缓存的 Paused 还是旧的 false）",
			st:     State{Time: 300, Duration: 300, Paused: false, Ready: 4},
			relay:  false,
			reason: "ended 之后页面不再回传，靠位置落在片尾兜住",
		},
		{
			name:  "直播流（时长未知）在播",
			st:    State{Time: 12, Duration: 0, Paused: false, Ready: 4},
			relay: true,
		},
	}
	for _, c := range cases {
		if got := RelayOnClose(c.st); got != c.relay {
			t.Errorf("%s: RelayOnClose=%v，想要 %v（%s）", c.name, got, c.relay, c.reason)
		}
	}
}

// TestVideoPageReportsOnPlayPause 钉住页面在 play / pause 时**立刻**回报状态。
//
// 主窗关窗时按最后一次 Report 判定音视接力（video.RelayOnClose）。只有 1s
// 心跳的话，「暂停后马上关窗」会让 Go 侧拿到旧的「在播」→ 音频自动续播
// （用户报的正是这个：视频暂停着，关窗后声音出来了）。
func TestVideoPageReportsOnPlayPause(t *testing.T) {
	data, err := os.ReadFile("../../frontend/video.html")
	if err != nil {
		t.Fatalf("读不到 video.html：%v", err)
	}
	html := string(data)
	for _, want := range []string{
		"const reportState = () => {",
		"setInterval(reportState, 1000);",
		"v.addEventListener('play', reportState);",
		"v.addEventListener('pause', reportState);",
		"const closeVideo = () => { reportState(); call('Video.Close'); };",
		"v.addEventListener('ended', () => { reportState(); call('Video.Ended'); });",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("弹窗页缺少即时状态回传：%s", want)
		}
	}
	// 页面里的 Esc 关窗也必须先补报一次（与关闭键同一条语义）。
	esc := strings.Index(html, "if (e.code === 'Escape') {")
	if esc < 0 {
		t.Fatal("找不到 Esc 分支")
	}
	block := html[esc:]
	if end := strings.Index(block, "} else if"); end > 0 {
		block = block[:end]
	}
	if !strings.Contains(block, "reportState();") || !strings.Contains(block, "call('Video.Close');") {
		t.Error("Esc 关窗没有先补报状态再关闭")
	}
}

// TestNativeCloseStateUsesCurrentPlayer 钉住关窗收尾用的是**当前**播放器。
//
// 原地换集（switchNative）会把 n.pl 换成新播放器、旧的那个当场关掉；如果
// OnClosed 还用建窗时捕获的那个播放器，真正在放的新播放器就永远关不掉 ——
// 窗口没了声音还在（用户报的「关了视频窗，声音出来了、主窗却显示暂停」）。
func TestNativeCloseStateUsesCurrentPlayer(t *testing.T) {
	old := &stubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 10 * time.Second}
	cur := &stubPlayer{playing: false, ready: true, dur: 300 * time.Second, pos: 42 * time.Second}
	old.closed = true // 已经换集：旧播放器在 switchNative 里关过了

	n := &nativeWindow{pl: cur, title: "第二集", speed: 1, volume: 1}
	st := n.closeState()

	if !cur.closed {
		t.Error("关窗没有关掉当前播放器：窗口没了，视频声音还会继续放")
	}
	if !st.Paused {
		t.Error("当前播放器是暂停态，回传的 State.Paused 应为真（否则主窗会续播）")
	}
	if st.Time != 42 || st.Duration != 300 {
		t.Errorf("回传状态 %.1f/%.1f，想要 42/300（应来自当前播放器，不是旧的那一个）",
			st.Time, st.Duration)
	}
	if RelayOnClose(st) {
		t.Error("暂停态的收尾状态不该触发音视接力")
	}
}

// TestNativeCloseStateNilPlayer 没有播放器时也不能 panic（窗口已消失 / 建窗
// 失败等路径仍会走 OnClosed）。
func TestNativeCloseStateNilPlayer(t *testing.T) {
	n := &nativeWindow{}
	if st := n.closeState(); st.Time != 0 || st.Paused {
		t.Errorf("空播放器应回零值状态，得到 %+v", st)
	}
}

// TestNativeOnClosedUsesCloseState 钉住 OnClosed 走 closeState 而不是建窗时
// 捕获的 pl：只测行为很容易在重构里把这一行换回去（换回去后上面的
// TestNativeCloseStateUsesCurrentPlayer 仍然绿，因为它直接调 closeState）。
func TestNativeOnClosedUsesCloseState(t *testing.T) {
	src, err := os.ReadFile("native.go")
	if err != nil {
		t.Fatalf("读不到 native.go：%v", err)
	}
	code := string(src)
	if !strings.Contains(code, "st := n.closeState()") {
		t.Error("OnClosed 没有走 closeState（会关错播放器 / 状态与引擎分叉）")
	}
	body := code[strings.Index(code, "n.win.OnClosed(func() {"):]
	if end := strings.Index(body, "\t})\n\treturn n"); end > 0 {
		body = body[:end]
	}
	if strings.Contains(body, "pl.Close()") {
		t.Error("OnClosed 还在关建窗时捕获的 pl：原地换集后的当前播放器永远关不掉")
	}
}
