package view

// issue #42：播放键上的「加载中 / 缓冲中」共用指示器。
//
// 这里只放针对性断言（不是整套 view 回归）：主窗 / 迷你窗 / 原生视频窗
// 共用 PlayerBusy，忙态只换播放键里的图标与文案，不改变几何、不禁用整条
// 控制条；缓冲中仍可暂停；加载中不碰旧引擎（控制器侧断言在 app 包）；
// 暂停 / 片尾 / 出错不转圈；窗口隐藏时不再请求逐帧重画。

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// issue42App 造一个够画播放栏的主窗状态。busy 直接写 App 的三个来源字段，
// 与生产接线（Buffering / Waiting / VideoBusy）保持一致。
func issue42App(busy PlayerBusy, playing bool) *App {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Queue = []Track{{Aid: 1, Bvid: "BV1issue42", Cid: 2, Title: "让喜欢的声音，陪你度过今天", Up: "音乐现场"}}
	a.Index = 0
	a.Track = &a.Queue[0]
	a.Playing = playing
	a.Dur = 300
	a.Pos = 42
	a.SeekValue = 42
	a.Buffered = 120
	switch busy {
	case PlayerLoading:
		a.Buffering = true
	case PlayerBuffering:
		a.Waiting = true
	}
	return a
}

// issue42Access 按名字与键尺寸找播放键：主窗/视频窗 42，迷你窗 28。Shell
// 里还有别的「暂停」字样（大封面上的按钮），不能拿第一个同名元素当播放键。
func issue42Access(t *testing.T, tt *ui.Tester, name string, size float32) ui.AccessInfo {
	t.Helper()
	for _, info := range tt.AccessAll() {
		if info.Name == name && info.Bounds.W == size && info.Bounds.H == size {
			return info
		}
	}
	t.Fatalf("找不到 %vpx 的播放键 %q（现有：%v）", size, name, tt.Texts())
	return ui.AccessInfo{}
}

// TestIssue42PlayButtonBusyMainAndMini 钉住三件事：
//  1. 主窗与迷你窗在加载 / 缓冲时都显示同一套文案与 accessibility busy；
//  2. 播放键的几何不变（同一个 42 / 28 的键位，没有多一行、没有位移）；
//  3. 缓冲中点击仍然下发 TogglePlay（暂停必须能用）。
func TestIssue42PlayButtonBusyMainAndMini(t *testing.T) {
	cases := []struct {
		name string
		busy PlayerBusy
		play bool
		tip  string
	}{
		{"加载中", PlayerLoading, false, "加载中…"},
		{"缓冲中", PlayerBuffering, true, "缓冲中 · 点击暂停"},
	}
	for _, mini := range []bool{false, true} {
		label := "主窗"
		w, h := 800, 600
		if mini {
			label = "迷你窗"
			w, h = miniWidth, miniHeight
		}
		t.Run(label, func(t *testing.T) {
			normal := issue42App(PlayerIdle, true)
			normal.Mini = mini
			normalTT := shellTesterSize(t, normal, w, h)
			playSize := float32(playerBarKeyMetrics.Play)
			if mini {
				playSize = 28
			}
			normalInfo := issue42Access(t, normalTT, "暂停", playSize)
			if normalInfo.Busy {
				t.Fatal("正常播放时播放键不该是 busy")
			}

			for _, tc := range cases {
				a := issue42App(tc.busy, tc.play)
				a.Mini = mini
				clicks := 0
				a.Act.TogglePlay = func() { clicks++ }
				tt := shellTesterSize(t, a, w, h)
				info := issue42Access(t, tt, tc.tip, playSize)
				if !info.Busy {
					t.Errorf("%s：%q 的 accessibility busy 没置位", label, tc.tip)
				}
				if info.Description != tc.tip {
					t.Errorf("%s：tooltip = %q，想 %q", label, info.Description, tc.tip)
				}
				if info.Bounds != normalInfo.Bounds {
					t.Errorf("%s：忙态改变了播放键几何：%v vs %v", label, info.Bounds, normalInfo.Bounds)
				}
				if err := tt.Click(tc.tip); err != nil {
					t.Fatalf("%s：点不到忙态播放键：%v", label, err)
				}
				tt.Frame()
				if clicks != 1 {
					t.Errorf("%s：忙态点击应仍下发 TogglePlay（缓冲中要能暂停），clicks=%d", label, clicks)
				}
			}

			// 忙态结束后文案立刻回到普通播放/暂停，不再 busy。
			a := issue42App(PlayerBuffering, true)
			a.Mini = mini
			tt := shellTesterSize(t, a, w, h)
			issue42Access(t, tt, "缓冲中 · 点击暂停", playSize)
			a.Waiting = false
			tt.Frame()
			after := issue42Access(t, tt, "暂停", playSize)
			if after.Busy {
				t.Errorf("%s：缓冲结束后播放键仍是 busy", label)
			}
		})
	}
}

// TestIssue42PlayerBusyVideoOwnsOutput 钉住输出归属优先于音频初始化：视频窗
// 占着声音时，后台残留的音频起播不能把播放键卡在「加载中」。
func TestIssue42PlayerBusyVideoOwnsOutput(t *testing.T) {
	a := &App{
		VideoOpen: true,
		VideoBusy: PlayerBuffering,
		Buffering: true,
		Waiting:   true,
		Playing:   true,
	}
	if got := a.playerBusy(); got != PlayerBuffering {
		t.Fatalf("视频缓冲中应显示缓冲，得到 %v（音频 Buffering 抢了优先级）", got)
	}
	a.VideoBusy = PlayerIdle
	if got := a.playerBusy(); got != PlayerIdle {
		t.Fatalf("视频就绪/暂停时不该被音频 Buffering 卡成加载中，得到 %v", got)
	}
	a.VideoOpen = false
	if got := a.playerBusy(); got != PlayerLoading {
		t.Fatalf("没有视频时音频初始化应显示加载中，得到 %v", got)
	}
}

// TestIssue42LoadingKeepsSettings 钉住忙态不碰设置：加载中倍速/音量键仍在、
// 共享设置值不变（不是把整条控制条禁用或换成另一套布局）。
func TestIssue42LoadingKeepsSettings(t *testing.T) {
	a := issue42App(PlayerLoading, false)
	a.Speed = 1.5
	a.Volume = .4
	tt := shellTesterSize(t, a, 800, 600)
	issue42Access(t, tt, "加载中…", playerBarKeyMetrics.Play)
	for _, name := range []string{"音量", "播放速度 1.5 倍"} {
		if _, ok := tt.Access(name); !ok {
			t.Errorf("加载中缺少设置键 %q（现有：%v）", name, tt.Texts())
		}
	}
	if a.Speed != 1.5 || a.Volume != .4 {
		t.Errorf("忙态改了共享设置：speed=%v volume=%v", a.Speed, a.Volume)
	}
}

// TestIssue42VideoBusyStates 钉住原生视频窗的忙态判据：
//   - 首帧未到（Ready=false 且仍在播）→ 加载中；
//   - 系统媒体框架等数据 → 缓冲中，点击可暂停；
//   - 暂停 / 片尾不忙（不能留下永远转的圆环）。
func TestIssue42VideoBusyStates(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 320, 200)

	pl.ready, pl.playing, pl.buffering = false, true, false
	tt.Frame()
	info := issue42Access(t, tt, "加载中…", playerBarKeyMetrics.Play)
	if !info.Busy {
		t.Fatal("首帧未到时应显示加载中")
	}

	pl.ready, pl.buffering = true, true
	tt.Frame()
	// 忙态有迟滞：缓冲要连续这么久才显示（HLS 的 buffering 位会抖）。
	tt.Advance(videoBusyConfirm + 20*time.Millisecond)
	tt.Frame()
	issue42Access(t, tt, "缓冲中 · 点击暂停", playerBarKeyMetrics.Play)
	if err := tt.Click("缓冲中 · 点击暂停"); err != nil {
		t.Fatalf("缓冲中播放键点不到：%v", err)
	}
	tt.Frame()
	if pl.playing || pl.pauses != 1 {
		t.Fatalf("缓冲中点击应暂停：playing=%v pauses=%d", pl.playing, pl.pauses)
	}
	if _, ok := tt.Access("播放"); !ok {
		t.Fatalf("暂停后应恢复普通「播放」文案：%v", tt.Texts())
	}

	// 暂停但引擎仍报 buffering：不转圈（暂停不是缓冲）。
	pl.buffering = true
	tt.Frame()
	tt.Advance(videoBusyRelease + 20*time.Millisecond)
	tt.Frame()
	if info, ok := tt.Access("缓冲中 · 点击暂停"); ok && info.Busy {
		t.Fatal("暂停时不该显示缓冲圆环")
	}

	// 片尾：即使缓冲位残留也不转圈。
	pl.ended, pl.playing, pl.buffering = true, false, true
	tt.Frame()
	if info, ok := tt.Access("播放"); ok && info.Busy {
		t.Fatal("片尾不该显示忙态")
	}
	if ch.playerBusy() != PlayerIdle {
		t.Fatalf("片尾 playerBusy=%v，想 Idle", ch.playerBusy())
	}
}

// TestIssue42VideoBusyHoldsChrome 钉住「缓冲 / 未就绪时控件条不自动隐藏」，
// 替代已删除的进度条呼吸描边：唯一的提示在播放键上，控件条必须看得见。
func TestIssue42VideoBusyHoldsChrome(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 640, 360)
	pl.ready, pl.playing, pl.buffering = false, true, false
	tt.Frame()
	tt.Advance(6 * time.Second)
	if ch.idle.alpha != 1 {
		t.Fatalf("加载中控件条淡出了（alpha=%v）", ch.idle.alpha)
	}
	pl.ready, pl.buffering = true, true
	tt.Frame()
	tt.Advance(videoBusyConfirm + 20*time.Millisecond)
	tt.Frame()
	tt.Advance(6 * time.Second)
	if ch.idle.alpha != 1 {
		t.Fatalf("缓冲中控件条淡出了（alpha=%v）", ch.idle.alpha)
	}
}

// TestIssue42HiddenVideoKeepsContentButOmitsChrome 钉住隐藏分支的边界：
// ui.Video（Content）必须继续被构建，引擎才能 advance / 更新 Ready/Buffering；
// 但忙态圆环与控件条不能画（也就不会由 spinner 的 Loop 驱动重画）。
//
// 这里不要求「隐藏时没有任何待处理帧」：真实 ui.Video 会按自己的节奏请求
// 解码帧，那是引擎的既有行为，不能由外壳替它停掉。
func TestIssue42HiddenVideoKeepsContentButOmitsChrome(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 320, 200)
	visits := 0
	ch.Content = func(c *ui.Context) ui.Element {
		visits++
		return ui.Box(c).Fill().Background(ui.Hex("#000000"))
	}
	pl.ready, pl.playing, pl.buffering = true, true, true
	tt.Frame()
	tt.Advance(videoBusyConfirm + 20*time.Millisecond)
	tt.Frame()
	if ch.playerBusy() != PlayerBuffering {
		t.Fatalf("前提：应处于缓冲，得到 %v", ch.playerBusy())
	}
	before := visits
	tt.SetWindowHidden(true)
	tt.Frame()
	if visits <= before {
		t.Fatal("窗口隐藏后 Content 没再被构建 —— ui.Video 引擎会冻结状态 / 错误 hold 静音视频")
	}
	for _, info := range tt.AccessAll() {
		if info.Busy {
			t.Fatalf("隐藏时仍画了忙态控件：%q", info.Name)
		}
	}
	if _, ok := tt.Access("缓冲中 · 点击暂停"); ok {
		t.Fatal("隐藏时仍画了播放键忙态文案")
	}
}

// TestIssue42BusyRingHonorsExistingSpinnerPolicy 钉住「减少动态效果」下沿用
// ringSpinner 的既有策略：Loop 是「还在忙」的指示，不随 ReduceMotion 停
// （见 reduce.go）。这里不断言它必须静止，只断言忙态仍然可见、仍会转。
func TestIssue42BusyRingHonorsExistingSpinnerPolicy(t *testing.T) {
	a := issue42App(PlayerBuffering, true)
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	info := issue42Access(t, tt, "缓冲中 · 点击暂停", playerBarKeyMetrics.Play)
	if !info.Busy {
		t.Fatal("ReduceMotion 下忙态文案丢了")
	}
	if !tt.FramePending() {
		t.Fatal("ReduceMotion 下忙态圆环停了 —— ringSpinner 的既有策略是 Loop 照常转")
	}
}

// TestIssue42BusyShots 导出人工对照图（默认跳过）：
//
//	BILI_ISSUE42_SHOTS=/tmp/bili-fm-issue42-shots go test ./internal/view -run TestIssue42BusyShots
func TestIssue42BusyShots(t *testing.T) {
	dir := os.Getenv("BILI_ISSUE42_SHOTS")
	if dir == "" {
		t.Skip("设置 BILI_ISSUE42_SHOTS=<目录> 导出 issue #42 忙态对照图")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, img *image.RGBA) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	themes := []struct {
		name string
		dark bool
	}{
		{"light", false},
		{"dark", true},
	}
	for _, th := range themes {
		for _, busy := range []PlayerBusy{PlayerIdle, PlayerLoading, PlayerBuffering} {
			tag := map[PlayerBusy]string{PlayerIdle: "normal", PlayerLoading: "loading", PlayerBuffering: "buffering"}[busy]
			// 忙态截图都按「用户想让它播」画：加载中是 Playing 意图已成立、
			// 首帧还没到；缓冲中是播放已建立、数据在等。
			playing := true
			for _, sc := range []struct {
				name string
				w, h int
				mini bool
			}{
				{"main", 800, 600, false},
				{"mini", miniWidth, miniHeight, true},
			} {
				a := issue42App(busy, playing)
				a.Mini = sc.mini
				if th.dark {
					a.Theme = ThemeFor(PeriodAt(12)).withAppearance("dark", .25)
				}
				tt := shellTesterSize(t, a, sc.w, sc.h)
				write(fmt.Sprintf("%s-%s-%s", th.name, sc.name, tag), tt.Image())
			}
			ch, pl, _, _ := videoChromeProbe(t, 320, 200)
			if th.dark {
				ch.Theme = ThemeFor(PeriodAt(12)).withAppearance("dark", .25)
			}
			pl.ready = busy != PlayerLoading
			pl.playing = playing
			pl.buffering = busy == PlayerBuffering
			vt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 320, 200)
			vt.Frame()
			write(fmt.Sprintf("%s-video-%s", th.name, tag), vt.Image())
		}
	}
}

// TestIssue42PlayerBusyVideoOpening 钉住「点了播放到视频窗开出来」这段的忙态。
//
// 这一段的特殊性：VideoOpen 还是假、没有任何播放器存在，取地址 + 建窗却要
// 几百毫秒到几秒。以前这段时间界面完全静止，用户看到的是「点了没反应」。
func TestIssue42PlayerBusyVideoOpening(t *testing.T) {
	a := &App{VideoOpening: true}
	if got := a.playerBusy(); got != PlayerLoading {
		t.Fatalf("开窗中应显示加载中，得到 %v", got)
	}
	// 与音频起播/欠载的优先级：开窗中优先，不能被旧一集的收尾状态改掉。
	a.Buffering = true
	a.Waiting = true
	a.Playing = true
	if got := a.playerBusy(); got != PlayerLoading {
		t.Fatalf("开窗中应保持加载中，得到 %v", got)
	}
	// 窗口开出来之后由视频自己的状态接管。
	a.VideoOpen = true
	a.VideoBusy = PlayerBuffering
	if got := a.playerBusy(); got != PlayerBuffering {
		t.Fatalf("窗口开出后应看 VideoBusy，得到 %v", got)
	}
	a.VideoOpen = false
	a.VideoOpening = false
	a.Buffering = false
	if got := a.playerBusy(); got != PlayerBuffering {
		t.Fatalf("回到音频欠载应显示缓冲中，得到 %v", got)
	}
}

// TestIssue42VideoOpeningRendersRing 钉住开窗中主窗与迷你窗都画那颗圆环
// （两处都读同一个 playerBusy，这里做行为断言防止某一路自己判状态）。
func TestIssue42VideoOpeningRendersRing(t *testing.T) {
	for _, mini := range []bool{false, true} {
		label := "主窗"
		w, h := 800, 600
		size := float32(42)
		if mini {
			label = "迷你窗"
			w, h = miniWidth, miniHeight
			size = 28
		}
		t.Run(label, func(t *testing.T) {
			a := issue42App(PlayerIdle, true)
			a.Mini = mini
			a.VideoOpening = true
			tt := shellTesterSize(t, a, w, h)
			if !issue42Access(t, tt, "加载中…", size).Busy {
				t.Errorf("%s：开窗中播放键没有置 busy", label)
			}
			a.VideoOpening = false
			tt.Frame()
			// Playing=true，忙态结束后文案回到「暂停」。
			if after := issue42Access(t, tt, "暂停", size); after.Busy {
				t.Errorf("%s：开窗结束后播放键仍是 busy", label)
			}
		})
	}
}

// TestIssue42VideoBusyHysteresisStopsFlicker 钉住忙态的迟滞：HLS / 网络播放的
// buffering 位会一两帧地翻，直接跟随它会让播放键图标在「暂停」与圆环之间闪、
// 控件条因 holdChrome 反复淡入淡出（用户报的「3 倍速时按钮闪动」）。
//
// 判据：抖动期间 playerBusy 始终是 Buffering、alpha 始终是 1；真正恢复后要
// 稳定 videoBusyRelease 才退出缓冲。
func TestIssue42VideoBusyHysteresisStopsFlicker(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 640, 360)
	pl.ready, pl.playing = true, true
	tt.Frame()
	pl.buffering = true
	tt.Frame()
	tt.Advance(videoBusyConfirm + 20*time.Millisecond)
	tt.Frame()
	if ch.playerBusy() != PlayerBuffering {
		t.Fatalf("前提：应进入缓冲，得到 %v", ch.playerBusy())
	}
	for i := 0; i < 20; i++ {
		pl.buffering = i%2 == 0
		tt.Advance(100 * time.Millisecond)
		tt.Frame()
		if ch.playerBusy() != PlayerBuffering {
			t.Fatalf("第 %d 次抖动时忙态掉回 %v（图标会闪）", i, ch.playerBusy())
		}
		if ch.idle.alpha != 1 {
			t.Fatalf("第 %d 次抖动时控件条 alpha=%v（会闪）", i, ch.idle.alpha)
		}
	}
	// 真正恢复：非缓冲稳定 videoBusyRelease 才放行。
	pl.buffering = false
	tt.Frame()
	tt.Advance(videoBusyRelease / 2)
	tt.Frame()
	if ch.playerBusy() != PlayerBuffering {
		t.Fatalf("恢复半程就退出缓冲：%v", ch.playerBusy())
	}
	tt.Advance(videoBusyRelease/2 + 20*time.Millisecond)
	tt.Frame()
	if ch.playerBusy() != PlayerIdle {
		t.Fatalf("恢复稳定后仍在缓冲：%v", ch.playerBusy())
	}
}
