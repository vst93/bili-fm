package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 复现：视频窗倍速弹层里的档位点不动 / 悬停闪动。
func TestVideoChromeSpeedPopoverClick(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 640, 360)
	// 先把指针放到控件条上，别让控件条自己收起来。
	tt.Move(320, 360-videoInset-videoChromeBarH/2)
	tt.Frame()

	// 点倍速键打开弹层。
	key, ok := tt.Find("播放速度 1 倍")
	if !ok {
		t.Fatalf("找不到倍速键")
	}
	tt.ClickAt(key.X+key.W/2, key.Y+key.H/2)
	tt.Frame()
	if !ch.showSpeed {
		t.Fatalf("点倍速键没有打开弹层")
	}

	// 悬停到 1.5x 档位上：矩形与悬停底色都必须稳定（不稳就是「闪动」）。
	opt, ok := tt.Find("1.5x")
	if !ok {
		t.Fatalf("找不到 1.5x 档位")
	}
	cx, cy := opt.X+opt.W/2, opt.Y+opt.H/2
	tt.Move(cx, cy)
	type frame struct {
		r    ui.Rect
		ink  uint8
		hit  bool
		open bool
	}
	var got []frame
	for i := 0; i < 6; i++ {
		tt.Frame()
		r, _ := tt.Find("1.5x")
		got = append(got, frame{r: r, ink: tt.Image().RGBAAt(int(cx), int(cy)).R, open: ch.showSpeed})
	}
	for i := 1; i < len(got); i++ {
		if got[i].r != got[0].r {
			t.Errorf("第 %d 帧档位矩形变了：%+v → %+v", i, got[0].r, got[i].r)
		}
		if got[i].ink != got[0].ink {
			t.Errorf("第 %d 帧悬停底色变了：%d → %d", i, got[0].ink, got[i].ink)
		}
		if got[i].open != got[0].open {
			t.Errorf("第 %d 帧弹层开合变了：%v → %v", i, got[0].open, got[i].open)
		}
	}

	// 点它：倍速必须变，弹层必须关。
	tt.ClickAt(cx, cy)
	tt.Frame()
	if pl.rate != 1.5 {
		t.Errorf("点 1.5x 档位没生效：player.rate=%v（弹层开着=%v）", pl.rate, ch.showSpeed)
	}
	if ch.showSpeed {
		t.Errorf("选中档位后弹层没关")
	}
}

// 复现：弹层开着时控件条是否被 hold（不该淡出）。
func TestVideoChromeSpeedPopoverHoldsChrome(t *testing.T) {
	ch, _, _, tt := videoChromeProbe(t, 640, 360)
	ch.showSpeed = true
	tt.Move(320, 360-videoInset-videoChromeBarH/2)
	tt.Frame()
	tt.Advance(5 * time.Second)
	if ch.idle.alpha != 1 {
		t.Errorf("倍速弹层开着，控件条却淡出了（alpha=%v）", ch.idle.alpha)
	}
}