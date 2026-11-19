package view

import (
	"testing"
	"time"
)

// TestVideoChromeHoverKeepsFramesComing 钉住「悬停期间必须逐帧重建」。
//
// 悬停 / 按压的材质色是构建期算的（Hovered + AnimateColor 的 140ms 过渡），
// 而视频窗的帧大多由视频驱动、只重绘不重建：不逐帧请求重建，过渡就只在
// 偶发的重建帧里跳一格，画面在新旧两色之间闪（用户报的「悬停闪烁」）。
func TestVideoChromeHoverKeepsFramesComing(t *testing.T) {
	_, _, _, tt := videoChromeProbe(t, 640, 360)
	r, ok := tt.Find("视频设置")
	if !ok {
		t.Fatalf("找不到设置键")
	}
	// 指针停在按钮上：必须持续请求下一帧。
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if !tt.FramePending() {
		t.Fatal("指针停在按钮上，却没有请求下一帧（悬停过渡不会推进）")
	}
	// 指针离开控件条：过渡尾巴（140ms）内仍要请求。
	tt.Move(320, 200)
	tt.Frame()
	missing := 0
	for i := 0; i < 10; i++ { // 10 × 16ms = 160ms
		tt.Advance(16 * time.Millisecond)
		if !tt.FramePending() {
			missing++
		}
		tt.Frame()
	}
	if missing > 0 {
		t.Errorf("指针离开后 160ms 内有 %d 帧没请求重建（过渡会停在半路）", missing)
	}
	// 尾巴走完后不再无谓地逐帧重画。
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if tt.FramePending() {
		t.Error("悬停尾巴已过，仍在逐帧请求重建")
	}
}
