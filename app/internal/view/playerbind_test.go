package view

// 本轮（视频窗控件条复用主窗播放栏）的绑定目标测试。
//
// 同一组 playerTransportItems / playerTimelineItems / playerSettingsItems
// 用两个 PlayerBarBind 分别驱动主窗音频引擎与视频窗播放器：
//   - 传输（播放/暂停、进度、时间）各绑各的，互不影响；
//   - 设置（音量/静音/倍速/均衡/广告跳过）读写同一个 PlayerSettings，
//     改一边另一边立刻看见，并由各自的监听落到自己的引擎上。

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// playerBindApp 起一个 1000×700 的主窗，曲目在播，供播放栏点击。
func playerBindApp(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Queue = []Track{{Bvid: "BV1", Title: "标题", Up: "UP"}}
	a.Index = 0
	a.Dur, a.Pos, a.SeekValue = 200, 100, 100
	a.Playing = true
	a.Volume = 0.5
	a.LastVolume = 0.5
	a.PlayerSettings.Ensure()
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Advance(400 * time.Millisecond)
	tst.Frame()
	return a, tst
}

// playerBindVideo 起一个共享同一份设置的视频窗（900 宽是完整一档）。
func playerBindVideo(t *testing.T, set *PlayerSettings) (*VideoChrome, *videoStubPlayer, *ui.Tester) {
	t.Helper()
	pl := &videoStubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second, rate: 1, vol: 1}
	ch := &VideoChrome{
		Theme: ThemeFor(PeriodAt(12)),
		Title: "测试稿件",
		Pl:    pl,
		Win:   newVideoStubWindow(),
		Set:   set,
	}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 900, 500)
	tt.Frame()
	return ch, pl, tt
}

// TestPlayerBindTargetsPlayIndependently 钉住「同一份代码、两个绑定目标」：
// 播放键在哪个窗口按，就只动那个窗口的引擎。
func TestPlayerBindTargetsPlayIndependently(t *testing.T) {
	a, mainTT := playerBindApp(t)
	mainToggles := 0
	a.Act.TogglePlay = func() { mainToggles++; a.Playing = !a.Playing }

	_, pl, vidTT := playerBindVideo(t, &a.PlayerSettings)

	// 主窗的播放键只走音频引擎（Act.TogglePlay），视频播放器一动不动。
	if err := mainTT.Click("暂停"); err != nil {
		t.Fatalf("点主窗暂停失败：%v", err)
	}
	mainTT.Frame()
	if mainToggles != 1 {
		t.Fatalf("主窗播放键没有走 Act.TogglePlay（%d 次）", mainToggles)
	}
	if pl.pauses != 0 || pl.plays != 0 {
		t.Errorf("主窗播放键动了视频播放器：pauses=%d plays=%d", pl.pauses, pl.plays)
	}

	// 视频窗的播放键只走视频播放器，主窗的 Act.TogglePlay 不再被调。
	// （主窗刚被自己那颗键切到暂停，这里把它摆回播放中再点视频窗。）
	a.Playing = true
	mainTT.Frame()
	if err := vidTT.Click("暂停"); err != nil {
		t.Fatalf("点视频窗暂停失败：%v", err)
	}
	vidTT.Frame()
	if pl.pauses != 1 {
		t.Errorf("视频窗播放键没有暂停视频（pauses=%d）", pl.pauses)
	}
	if mainToggles != 1 {
		t.Errorf("视频窗播放键误触了主窗引擎（Act.TogglePlay %d 次）", mainToggles)
	}
	if !a.Playing {
		t.Error("视频窗暂停把主窗 Playing 也改了")
	}
	if !mainTT.HasText("暂停") {
		t.Errorf("视频窗暂停后主窗按钮变了：%v", mainTT.Texts())
	}
}

// TestPlayerBindPlayStateNotMirrored 反证两边播放状态不互相镜像：
// 主窗暂停不影响视频，视频恢复也不影响主窗。
func TestPlayerBindPlayStateNotMirrored(t *testing.T) {
	a, mainTT := playerBindApp(t)
	a.Act.TogglePlay = func() { a.Playing = !a.Playing }
	_, pl, vidTT := playerBindVideo(t, &a.PlayerSettings)

	// 先恢复视频（一开始是播放中，点一下暂停、再点一下恢复）。
	if err := vidTT.Click("暂停"); err != nil {
		t.Fatalf("视频暂停失败：%v", err)
	}
	if err := vidTT.Click("播放"); err != nil {
		t.Fatalf("视频恢复失败：%v", err)
	}
	if !pl.playing {
		t.Fatal("视频没有恢复播放")
	}

	// 主窗暂停：视频仍在放，视频的暂停次数不变。
	beforePauses := pl.pauses
	if err := mainTT.Click("暂停"); err != nil {
		t.Fatalf("主窗暂停失败：%v", err)
	}
	mainTT.Frame()
	if a.Playing {
		t.Error("主窗没有暂停")
	}
	if !pl.playing || pl.pauses != beforePauses {
		t.Errorf("主窗暂停影响了视频：playing=%v pauses=%d（之前 %d）", pl.playing, pl.pauses, beforePauses)
	}
}

// TestPlayerSettingsSharedTwoWay 钉住共享项的双向同步：视频窗改音量/倍速/
// 均衡/广告跳过 → 主窗状态跟着变；主窗改 → 视频窗的播放器跟着变。
func TestPlayerSettingsSharedTwoWay(t *testing.T) {
	a, mainTT := playerBindApp(t)
	ch, pl, vidTT := playerBindVideo(t, &a.PlayerSettings)
	if ch.settings() != &a.PlayerSettings {
		t.Fatal("视频窗拿到的不是主窗那份 PlayerSettings")
	}

	// —— 视频窗 → 主窗：音量（静音）。
	if err := vidTT.Click("音量"); err != nil {
		t.Fatalf("视频音量键失败：%v", err)
	}
	vidTT.Frame()
	if err := vidTT.Click("静音"); err != nil {
		t.Fatalf("视频静音键失败：%v", err)
	}
	vidTT.Frame()
	if a.Volume != 0 || !a.Muted {
		t.Errorf("视频窗静音后主窗 = Volume %v / Muted %v", a.Volume, a.Muted)
	}
	if !pl.muted || pl.vol != 0 {
		t.Errorf("视频窗静音没有落到自己的播放器：muted=%v vol=%v", pl.muted, pl.vol)
	}

	// —— 主窗 → 视频窗：音量滑杆（弹层里那条与视频窗同一份实现）。
	mainTT.Frame()
	if err := mainTT.Click("音量"); err != nil {
		t.Fatalf("主窗音量键失败：%v", err)
	}
	mainTT.Frame()
	r, ok := mainTT.Find("音量滑块")
	if !ok {
		t.Fatalf("主窗音量弹层里找不到滑杆：%v", mainTT.Texts())
	}
	mainTT.ClickAt(r.X+r.W*0.4, r.Y+r.H/2)
	mainTT.Frame()
	if a.Volume <= 0 || a.Muted {
		t.Fatalf("主窗拖动音量滑杆没生效：Volume %v / Muted %v", a.Volume, a.Muted)
	}
	if want := a.Volume; pl.vol != want || pl.muted {
		t.Errorf("主窗音量 %v 没同步到视频：vol=%v muted=%v", want, pl.vol, pl.muted)
	}

	// —— 视频窗 → 主窗：倍速（弹层）。
	if err := vidTT.Click("1.0x"); err != nil {
		t.Fatalf("视频倍速键失败：%v", err)
	}
	vidTT.Frame()
	if err := vidTT.Click("2.0x"); err != nil {
		t.Fatalf("视频选 2.0x 失败：%v", err)
	}
	vidTT.Frame()
	if a.Speed != 2.0 {
		t.Errorf("视频窗选 2.0x 后主窗 Speed = %v", a.Speed)
	}
	if pl.rate != 2.0 {
		t.Errorf("视频窗选 2.0x 后自己的播放器 rate = %v", pl.rate)
	}

	// —— 主窗 → 视频窗：倍速（同一份弹层）。
	mainTT.Frame()
	if err := mainTT.Click("2.0x"); err != nil {
		t.Fatalf("主窗倍速键失败：%v", err)
	}
	mainTT.Frame()
	if err := mainTT.Click("1.25x"); err != nil {
		t.Fatalf("主窗选 1.25x 失败：%v", err)
	}
	mainTT.Frame()
	if a.Speed != 1.25 || !mainTT.HasText("1.25x") {
		t.Errorf("主窗倍速没落到 1.25x：%v（文案 %v）", a.Speed, mainTT.Texts())
	}
	if pl.rate != 1.25 {
		t.Errorf("主窗倍速 1.25x 没同步到视频：rate=%v", pl.rate)
	}

	// —— 均衡：视频窗点一下，主窗状态跟着变；主窗再点回来。
	if err := vidTT.Click("音量均衡: 关"); err != nil {
		t.Fatalf("视频均衡键失败：%v", err)
	}
	vidTT.Frame()
	if !a.EQ {
		t.Error("视频窗打开音量均衡后主窗 EQ 仍为关")
	}
	mainTT.Frame()
	if !mainTT.HasText("音量均衡: 开") {
		t.Errorf("主窗没有反映音量均衡已开：%v", mainTT.Texts())
	}
	if err := mainTT.Click("音量均衡: 开"); err != nil {
		t.Fatalf("主窗均衡键失败：%v", err)
	}
	mainTT.Frame()
	if a.EQ {
		t.Error("主窗关闭音量均衡后 EQ 仍为开")
	}

	// —— 广告跳过：两边同一份开关与状态。
	if err := vidTT.Click("自动跳过恰饭片段（SponsorBlock）"); err != nil {
		t.Fatalf("视频恰饭键失败：%v", err)
	}
	vidTT.Frame()
	if !a.Sponsor {
		t.Error("视频窗打开广告跳过后主窗 Sponsor 仍为关")
	}
	a.SponsorStatus = "ok"
	a.SponsorCount = 2
	vidTT.Frame()
	if got := ch.settings().SponsorStatus; got != "ok" {
		t.Errorf("主窗写的 SponsorStatus 没被视频窗读到：%q", got)
	}
	if err := mainTT.Click("自动跳过恰饭片段（SponsorBlock）"); err != nil {
		t.Fatalf("主窗恰饭键失败：%v", err)
	}
	mainTT.Frame()
	if a.Sponsor {
		t.Error("主窗关闭广告跳过后 Sponsor 仍为开")
	}
}
