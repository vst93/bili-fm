//go:build darwin

package video

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

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
