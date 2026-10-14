package view

import (
	"testing"
)

// 本轮（videoInfo 右栏盒高 / 标题行高按原版钉住）的测试。
//
// 原版 globals.css 对右栏这两处显式给了盒高与内边距，mygo 不显式给就会退回
// 框架默认，把整列撑高：
//
//	#video-info       { height: 366px; padding: 2px 0 2px 3px }
//	#video-title h2   { min-height: 52px; max-height: 58px; line-height: 1.18 }
//
// 简介块（#video-desc）的 min / max 高已由 videodesc_test.go 单独钉住。

// TestVideoTitleBoxHeight 钉住标题的两行盒高：长标题被 -webkit-line-clamp: 2
// 截住后盒子应停在 max-height 58，短标题则由 min-height 撑到 52。
// 此前只给了 MinHeight(52)、没给 MaxHeight，固定 366 盒高里的剩余空间被标题
// 吸收，长标题时盒子会涨到 66（实测），整列随之被顶。
func TestVideoTitleBoxHeight(t *testing.T) {
	long := ""
	for i := 0; i < 6; i++ {
		long += "这是一个很长的视频标题用于测试两行截断"
	}
	a := descApp()
	a.Info.Title = long
	a.Track.Title = long
	tt := shellTesterSize(t, a, 1000, 700)
	r, ok := tt.Find(long)
	if !ok {
		t.Fatal("找不到长标题")
	}
	t.Logf("长标题 rect = %+v", r)
	if r.H != 58 {
		t.Errorf("长标题盒高 %v，want 58（#video-title h2 max-height）", r.H)
	}

	b := descApp()
	b.Info.Title = "短标题"
	b.Track.Title = "短标题"
	t2 := shellTesterSize(t, b, 1000, 700)
	r2, ok := t2.Find("短标题")
	if !ok {
		t.Fatal("找不到短标题")
	}
	t.Logf("短标题 rect = %+v", r2)
	if r2.H != 52 {
		t.Errorf("短标题盒高 %v，want 52（#video-title h2 min-height）", r2.H)
	}
}

// TestVideoTitleMaxHeightWired 是上一条的反证：把标题的 max-height 归零后，
// 固定 366 盒高里的剩余空间会被标题吸收，长标题的盒子随之变高。
func TestVideoTitleMaxHeightWired(t *testing.T) {
	long := ""
	for i := 0; i < 6; i++ {
		long += "这是一个很长的视频标题用于测试两行截断"
	}
	measure := func() float32 {
		a := descApp()
		a.Info.Title = long
		a.Track.Title = long
		tt := shellTesterSize(t, a, 1000, 700)
		r, ok := tt.Find(long)
		if !ok {
			t.Fatal("找不到长标题")
		}
		return r.H
	}
	// videoTitleMaxH 是标题盒高的上限（见 home.go）；归零即撤掉 MaxHeight。
	if videoTitleMaxH != 58 {
		t.Fatalf("videoTitleMaxH = %v，应为原版 #video-title h2 的 max-height 58", videoTitleMaxH)
	}
	withCap := measure()
	old := videoTitleMaxH
	videoTitleMaxH = 10000 // 足够大 ⇒ 等于不设上限
	defer func() { videoTitleMaxH = old }()
	noCap := measure()
	t.Logf("标题盒高：有 max-height %v，撤掉后 %v", withCap, noCap)
	if noCap <= withCap {
		t.Errorf("撤掉 max-height 后标题没有变高（%v → %v），上限没接上", withCap, noCap)
	}
}

// TestVideoInfoPadShiftsContent 钉住右栏内边距：原版 `#video-info` 的
// `padding: 2px 0 2px 3px`。把 padding 归零，内容应整体上移 2px、左移 3px。
func TestVideoInfoPadShiftsContent(t *testing.T) {
	want := [4]float32{2, 0, 2, 3}
	if videoInfoPad != want {
		t.Fatalf("videoInfoPad = %v，应为原版 #video-info 的 padding 2px 0 2px 3px", videoInfoPad)
	}
	measure := func() (x, y float32) {
		tt := shellTesterSize(t, descApp(), 1000, 700)
		r, ok := tt.Find("UP 主")
		if !ok {
			t.Fatal("找不到「UP 主」小标")
		}
		return r.X, r.Y
	}
	withPadX, withPadY := measure()
	old := videoInfoPad
	videoInfoPad = [4]float32{}
	defer func() { videoInfoPad = old }()
	noPadX, noPadY := measure()
	t.Logf("「UP 主」位置：有 padding (%v,%v)，归零 (%v,%v)", withPadX, withPadY, noPadX, noPadY)
	if d := withPadX - noPadX; d != 3 {
		t.Errorf("内边距左边距差 %v，want 3", d)
	}
	if d := withPadY - noPadY; d != 2 {
		t.Errorf("内边距上边距差 %v，want 2", d)
	}
}
