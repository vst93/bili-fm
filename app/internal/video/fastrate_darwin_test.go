//go:build darwin

package video

import (
	"errors"
	"image"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"

	"github.com/vst93/bili-fm/app/internal/view"
)

// TestThreeTimesAudioHandoff uses the video window's real shared settings
// and content path, starting at the position inherited from audio playback.
func TestThreeTimesAudioHandoff(t *testing.T) {
	server := httptest.NewServer(http.FileServer(http.Dir("testdata")))
	defer server.Close()
	player, err := video.OpenURL(server.URL + "/fastplay.mp4")
	if errors.Is(err, video.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	settings := &view.PlayerSettings{Speed: 3, Muted: true, Volume: 1, LastVolume: 1}
	n := stubNativeWindow(Options{}, uiPlayer{p: player})
	n.settings, n.startAt = settings, 1
	defer n.chromeDetach()
	player.SetMuted(true).SetRate(settings.Speed)
	player.Seek(time.Second)
	tester := ui.NewTester(n.view, 320, 200)
	deadline := time.Now().Add(time.Minute)
	for !player.Ready() || player.Position() < 1300*time.Millisecond {
		tester.Frame()
		if player.Err() != nil || time.Now().After(deadline) {
			t.Fatalf("3 倍速音频切视频未自动起播：position=%v err=%v", player.Position(), player.Err())
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, phase := range []string{"自动起播", "暂停再播放"} {
		if phase == "暂停再播放" {
			n.pause()
			n.play()
		}
		// 先等到这一轮真的出现新画面再计时：Ready 只表示「曾经有过第一帧」，
		// 3 倍速换到压缩时间轴后它仍为真，而 Frame 在新 item 出第一帧前会继续
		// 返回旧缓冲（changed=false）。不等这一下，600ms 可能全花在 retime/seek
		// 的落地里，看起来就是「倍速对、只有 1 个画面」。
		waitForVideoPicture(t, tester, player, phase)
		start, pos := time.Now(), player.Position()
		pictures := map[[3]uint32]bool{}
		var last [3]uint32
		for time.Since(start) < 600*time.Millisecond {
			tester.Frame()
			last = videoPicture(tester.Image())
			pictures[last] = true
			time.Sleep(10 * time.Millisecond)
		}
		rate := float64(player.Position()-pos) / float64(time.Since(start))
		t.Logf("%s：%.2fx，%d 个画面", phase, rate, len(pictures))
		if player.Err() != nil || rate < 2.5 || rate > 3.5 || len(pictures) < 5 || n.settings.Speed != 3 {
			t.Fatalf("%s 丢失倍速或画面：rate=%v pictures=%d speed=%v pos=%v ready=%v playing=%v buffering=%v last=%v err=%v",
				phase, rate, len(pictures), n.settings.Speed, player.Position(),
				player.Ready(), player.Playing(), player.Buffering(), last, player.Err())
		}
	}
}

// 320×200 的窗口里，顶栏下沿在 48（12 inset + 36 高），窄窗两行控件条
// 上沿在 110（200 − 12 − 78），所以 y=80 一定落在画面里；96×64 的帧按
// Contain 放进 300×200（x=10..310），三个采样点都避开了两侧留黑与两条栏。
var videoProbeXs = [...]int{80, 160, 240}

const videoProbeY = 80

// videoPicture 是画面区一帧的三个 RGB 采样。静止画面（哪怕渐变跨过采样点）
// 永远只有同一个值；只有解码出的新帧才会改变它，不会被栏的静态颜色冒充成
// 「新画面」。
func videoPicture(img *image.RGBA) [3]uint32 {
	var pic [3]uint32
	for i, x := range videoProbeXs {
		r, g, b, _ := img.RGBAAt(x, videoProbeY).RGBA()
		pic[i] = uint32(r>>8)<<16 | uint32(g>>8)<<8 | uint32(b>>8)
	}
	return pic
}

// waitForVideoPicture 等到画面区连续出现三帧不同画面再开始计时（只等一帧
// 可能等到的还是 retime 前旧 item 的最后一帧），失败时带上播放器状态与最后
// 一次采样，便于区分「没有新帧」和「采样点被界面挡住」。
func waitForVideoPicture(t *testing.T, tester *ui.Tester, player *ui.VideoPlayer, phase string) {
	t.Helper()
	before := videoPicture(tester.Image())
	seen := map[[3]uint32]bool{before: true}
	start := time.Now()
	deadline := start.Add(time.Minute)
	for {
		tester.Frame()
		if err := player.Err(); err != nil {
			t.Fatalf("%s 等新画面时出错：pos=%v ready=%v playing=%v err=%v",
				phase, player.Position(), player.Ready(), player.Playing(), err)
		}
		pic := videoPicture(tester.Image())
		if !seen[pic] {
			seen[pic] = true
			if len(seen) >= 3 {
				t.Logf("%s：新画面等待 %.0fms（%d 个不同画面）",
					phase, float64(time.Since(start).Microseconds())/1000, len(seen))
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s 没有新画面（等待 %v）：pos=%v ready=%v playing=%v buffering=%v 采样点=%v 最后画面=%v 不同画面=%d err=%v",
				phase, time.Since(start), player.Position(), player.Ready(), player.Playing(),
				player.Buffering(), videoProbeXs, pic, len(seen), player.Err())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestProgressiveRateChangeKeepsFrames 钉住「播放中把 2 倍速提到 3 倍速」：
// AVFoundation 在 >2 倍速时根本不给画面（时钟与声音照走），引擎改成压缩
// 时间轴后按 1.0 播，画面必须继续来，而且位置不能因为换 item 而跳。
func TestProgressiveRateChangeKeepsFrames(t *testing.T) {
	server := httptest.NewServer(http.FileServer(http.Dir("testdata")))
	defer server.Close()
	player, err := video.OpenURL(server.URL + "/fastplay.mp4")
	if errors.Is(err, video.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	player.SetMuted(true)
	player.SetRate(2)
	tester := ui.NewTester(func(ctx *ui.Context) { ui.Video(ctx, player).Size(96, 64) }, 96, 64)
	deadline := time.Now().Add(15 * time.Second)
	for !player.Ready() || player.Position() < 300*time.Millisecond {
		tester.Frame()
		if player.Err() != nil || time.Now().After(deadline) {
			t.Fatalf("等待 2 倍速起播：position=%v err=%v", player.Position(), player.Err())
		}
		time.Sleep(10 * time.Millisecond)
	}

	at2 := countPictures(tester, player, 600*time.Millisecond)
	before := player.Position()
	player.SetRate(3)
	tester.Frame()
	if pos := player.Position(); pos < before-500*time.Millisecond || pos > before+500*time.Millisecond {
		t.Fatalf("提速后位置跳变：%v → %v", before, pos)
	}
	// 换 item 后新时间轴要就绪、seek 要落地，这是一次性的停顿（几十到几百毫秒），
	// 不能算进稳态速率：先等它过去，再量 3 倍速本身。
	for i := 0; i < 50; i++ {
		tester.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	rate, frames := measure(tester, player, 900*time.Millisecond)
	t.Logf("2 倍速 %d 个画面；3 倍速 %.2fx、%d 个画面", at2, rate, frames)
	if player.Err() != nil || rate < 2.5 || rate > 3.5 || frames < 10 || at2 < 5 {
		t.Fatalf("播放中提速的时钟或画面异常：err=%v", player.Err())
	}
}

// countPictures is how many different pictures the window showed over d.
func countPictures(tester *ui.Tester, player *ui.VideoPlayer, d time.Duration) int {
	_, frames := measure(tester, player, d)
	return frames
}

// measure is the rate of the clock and the number of different pictures shown
// over d.
func measure(tester *ui.Tester, player *ui.VideoPlayer, d time.Duration) (float64, int) {
	start, pos := time.Now(), player.Position()
	levels := map[uint8]bool{}
	for time.Since(start) < d {
		tester.Frame()
		levels[tester.Image().RGBAAt(48, 32).R] = true
		time.Sleep(10 * time.Millisecond)
	}
	return float64(player.Position()-pos) / float64(time.Since(start)), len(levels)
}

// TestSpeedChangeDoesNotBlockUI 钉住「点倍速档位不能卡住绘制线程」。
//
// 压缩时间轴要建 composition、插轨、seek —— 这些都是 AVFoundation 的同步
// 调用。它们一旦跑在 SetRate 的调用线程（App 的 UI 线程）上，用户点一下
// 档位就会卡几十到几百毫秒：点击丢、悬停闪。SetRate 必须只记下目标就返回。
func TestSpeedChangeDoesNotBlockUI(t *testing.T) {
	server := httptest.NewServer(http.FileServer(http.Dir("testdata")))
	defer server.Close()
	player, err := video.OpenURL(server.URL + "/fastplay.mp4")
	if errors.Is(err, video.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	player.SetMuted(true)
	tester := ui.NewTester(func(ctx *ui.Context) { ui.Video(ctx, player).Size(96, 64) }, 96, 64)
	deadline := time.Now().Add(15 * time.Second)
	for !player.Ready() || player.Position() < 300*time.Millisecond {
		tester.Frame()
		if player.Err() != nil || time.Now().After(deadline) {
			t.Fatalf("等待起播：position=%v err=%v", player.Position(), player.Err())
		}
		time.Sleep(10 * time.Millisecond)
	}
	// 2 倍速 ↔ 3 倍速跨过压缩阈值，两个方向都必须是「立刻返回」。
	for _, rate := range []float64{3, 2, 3} {
		start := time.Now()
		player.SetRate(rate)
		if d := time.Since(start); d > 50*time.Millisecond {
			t.Fatalf("SetRate(%v) 阻塞了绘制线程 %v（>50ms）", rate, d)
		}
		for i := 0; i < 20; i++ {
			tester.Frame()
			time.Sleep(5 * time.Millisecond)
		}
	}
}
