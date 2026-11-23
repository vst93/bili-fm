package video

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

func TestFastPlaybackSources(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			for _, rate := range []float64{1, 2, 3} {
				// >2 倍速在 macOS（AVFoundation 不给画面）与 Windows（MF 的
				// 流式源拒绝 >2 倍速）都改用合流 MP4；Linux 继续用分片源。
				want := rate == 3 && (platform == "darwin" || platform == "windows")
				if got := PreferProgressive(platform, rate); got != want {
					t.Fatalf("开窗倍速 %v：合流=%v，预期 %v", rate, got, want)
				}
			}
			for _, playing := range []bool{false, true} {
				old := &stubPlayer{ready: true, playing: playing, pos: 37 * time.Second}
				next := &stubPlayer{playing: true}
				var opened []string
				m := New()
				m.openNative = func(src string) (playerAPI, error) {
					opened = append(opened, src)
					return next, nil
				}
				o := Options{Src: "http://local/master.m3u8", SrcFallback: "http://local/combined.mp4"}
				n := stubNativeWindow(o, old)
				m.wireNativeSource(n, o, platform)
				tester := ui.NewTester(n.view, 320, 200)
				tester.Frame()
				n.settings.SetVolume(0.4)
				n.settings.SetSpeed(2)
				tester.Frame()
				if n.pl != old || old.rate != 2 || len(opened) != 0 {
					t.Fatal("2 倍速应在当前播放器生效")
				}
				n.settings.SetSpeed(3)
				tester.Frame()
				if platform == "linux" {
					if n.pl != old || old.rate != 3 || len(opened) != 0 {
						t.Fatal("Linux 的 3 倍速应在当前分片播放器上生效")
					}
					continue
				}
				if n.pl != next || !old.closed || next.playing != playing || next.rate != 3 || next.vol != 0.4 || !slices.Equal(next.seeks, []time.Duration{old.pos}) {
					t.Fatalf("换源丢失播放状态：%+v", next)
				}
				for _, rate := range []float64{2, 3} {
					n.settings.SetSpeed(rate)
					tester.Frame()
				}
				if !slices.Equal(opened, []string{o.SrcFallback}) {
					t.Fatalf("本集应只换一次合流源：%v", opened)
				}
			}
		})
	}
}

func TestProgressiveThreeTimesPlayback(t *testing.T) {
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
	player.SetRate(3)
	tester := ui.NewTester(func(ctx *ui.Context) { ui.Video(ctx, player).Size(96, 64) }, 96, 64)
	deadline := time.Now().Add(15 * time.Second)
	for !player.Ready() || player.Position() < 300*time.Millisecond {
		tester.Frame()
		if player.Err() != nil || time.Now().After(deadline) {
			t.Fatalf("等待 3 倍速起播：position=%v err=%v", player.Position(), player.Err())
		}
		time.Sleep(10 * time.Millisecond)
	}
	start, pos := time.Now(), player.Position()
	levels := map[uint8]bool{}
	for time.Since(start) < 900*time.Millisecond {
		tester.Frame()
		levels[tester.Image().RGBAAt(48, 32).R] = true
		time.Sleep(10 * time.Millisecond)
	}
	rate := float64(player.Position()-pos) / float64(time.Since(start))
	t.Logf("实测 %.2fx，采到 %d 个不同画面", rate, len(levels))
	if player.Err() != nil || rate < 2.5 || rate > 3.5 || len(levels) < 10 {
		t.Fatalf("3 倍速画面或时钟异常：err=%v", player.Err())
	}
}
