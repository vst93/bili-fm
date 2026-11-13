package main

import (
	"testing"

	"github.com/egoist/mygo"

	"github.com/vst93/bili-fm/app/internal/view"
)

// fakeRegionWindow 记录下发给「窗口」的输入区域与整窗穿透状态。真窗口在无头
// 测试里建不出来（mygo 的 NewWindow 要事件循环），所以穿透内核只依赖
// lyricsRegionTarget 这一个接口。
type fakeRegionWindow struct {
	bounds  mygo.Rectangle
	regions [][]mygo.Rectangle // 每次 SetInputRegion 的入参（nil = 收回整窗）
	ignores []bool             // 每次 SetIgnoreMouseEvents 的入参
}

func (f *fakeRegionWindow) Bounds() mygo.Rectangle { return f.bounds }
func (f *fakeRegionWindow) SetInputRegion(rects []mygo.Rectangle) {
	f.regions = append(f.regions, rects)
}
func (f *fakeRegionWindow) SetIgnoreMouseEvents(v bool) { f.ignores = append(f.ignores, v) }

func (f *fakeRegionWindow) lastRegion() ([]mygo.Rectangle, bool) {
	if len(f.regions) == 0 {
		return nil, false
	}
	return f.regions[len(f.regions)-1], true
}

func (f *fakeRegionWindow) lastIgnore() (bool, bool) {
	if len(f.ignores) == 0 {
		return false, false
	}
	return f.ignores[len(f.ignores)-1], true
}

// TestLyricsInputRegionRects 钉住「穿透时仍然收点击的矩形」：与绘制出来的设置
// 键同源（view.LyricsGearInset / LyricsGearSize + slop），并随窗口缩放走、夹在
// 窗口内、小窗口不产生负尺寸。
func TestLyricsInputRegionRects(t *testing.T) {
	// 560×100（显示页默认宽度）：x = 560-4-26-6 = 524，右边与上边的 slop
	// 落在窗口外，夹掉后 36×36。
	got := lyricsInputRegionRects(560, 100)
	if len(got) != 1 {
		t.Fatalf("560×100 的输入区域有 %d 块，应为 1 块", len(got))
	}
	if want := (mygo.Rectangle{X: 524, Y: 0, Width: 36, Height: 36}); got[0] != want {
		t.Errorf("560×100 的输入区域 = %+v，应为 %+v", got[0], want)
	}

	// 窗口缩放：区域跟着右上角走（宽窗右移，高窗上边不再被夹）。
	got = lyricsInputRegionRects(900, 200)
	if want := (mygo.Rectangle{X: 864, Y: 0, Width: 36, Height: 36}); len(got) != 1 || got[0] != want {
		t.Errorf("900×200 的输入区域 = %+v，应为 [%+v]", got, want)
	}
	// 足够高的窗口：上边留出 inset-slop = -2 → 仍夹到 0；加大 inset 对比看
	// 「夹进窗口」只在越界时发生。
	got = lyricsInputRegionRects(560, 100)
	if got[0].Y != 0 {
		t.Errorf("上边 slop 越界时应夹到 0，得到 y=%d", got[0].Y)
	}

	// 最小尺寸：比设置键还小的窗口也要给出正宽高，且不超出窗口。
	got = lyricsInputRegionRects(20, 10)
	if len(got) != 1 {
		t.Fatalf("小窗口的输入区域有 %d 块，应为 1 块", len(got))
	}
	if r := got[0]; r.Width <= 0 || r.Height <= 0 || r.X < 0 || r.Y < 0 ||
		r.X+r.Width > 20 || r.Y+r.Height > 10 {
		t.Errorf("20×10 的输入区域 = %+v，应为窗口内的正矩形", r)
	}

	// 还没布局（宽高为 0）：没有可交互矩形，调用方据此回退整窗穿透。
	if got := lyricsInputRegionRects(0, 0); got != nil {
		t.Errorf("零尺寸的输入区域 = %+v，应为 nil", got)
	}
}

// TestLyricsClickThroughRouteFor 钉住回退选择逻辑：要穿透 + 输入区域可用才走
// 区域，其余全部退回整窗穿透（用户不能因为换实现就失去「穿透 + 设置键可点」）。
func TestLyricsClickThroughRouteFor(t *testing.T) {
	cases := []struct {
		name       string
		click      bool
		regionOK   bool
		wantRegion lyricsClickThroughRoute
	}{
		{"穿透 + 区域可用", true, true, lyricsRouteInputRegion},
		{"穿透 + 区域不可用（回退）", true, false, lyricsRouteWholeWindow},
		{"不穿透（区域可用）", false, true, lyricsRouteWholeWindow},
		{"不穿透（区域不可用）", false, false, lyricsRouteWholeWindow},
	}
	for _, c := range cases {
		if got := lyricsClickThroughRouteFor(c.click, c.regionOK); got != c.wantRegion {
			t.Errorf("%s：选路 = %v，应为 %v", c.name, got, c.wantRegion)
		}
	}
}

// TestLyricsApplyClickThroughRegion 钉住首选路径：输入区域可用时下发设置键那
// 一块，并且**显式关掉整窗穿透** —— mygo 里 SetIgnoreMouseEvents 的优先级高于
// SetInputRegion，留着 true 新 API 就白设了。
func TestLyricsApplyClickThroughRegion(t *testing.T) {
	w := &fakeRegionWindow{bounds: mygo.Rectangle{X: 100, Y: 200, Width: 560, Height: 100}}
	region, whole := lyricsApplyClickThrough(w, true, true, false)
	if !region || whole {
		t.Fatalf("穿透 + 区域可用：region=%v whole=%v，应为 true/false", region, whole)
	}
	rects, ok := w.lastRegion()
	if !ok || len(rects) != 1 {
		t.Fatalf("没有下发输入区域：%+v", w.regions)
	}
	if want := (mygo.Rectangle{X: 524, Y: 0, Width: 36, Height: 36}); rects[0] != want {
		t.Errorf("下发的输入区域 = %+v，应为 %+v", rects[0], want)
	}
	if v, ok := w.lastIgnore(); !ok || v {
		t.Errorf("整窗穿透应为 false（否则盖掉输入区域），得到 %v（设置过：%v）", v, ok)
	}

	// 齿轮热区为真也不影响首选路径：区域自己就保证了键可点。
	w = &fakeRegionWindow{bounds: mygo.Rectangle{Width: 560, Height: 100}}
	if region, whole := lyricsApplyClickThrough(w, true, true, true); !region || whole {
		t.Errorf("齿轮热区不应改变首选路径：region=%v whole=%v", region, whole)
	}
}

// TestLyricsApplyClickThroughFallback 钉住回退路径：区域不可用时退回整窗穿透 +
// 「指针在设置键上就临时不穿透」，并且把输入区域收回（否则区域外还是点不动）。
func TestLyricsApplyClickThroughFallback(t *testing.T) {
	// 指针不在齿轮上：整窗穿透。
	w := &fakeRegionWindow{bounds: mygo.Rectangle{Width: 560, Height: 100}}
	region, whole := lyricsApplyClickThrough(w, true, false, false)
	if region || !whole {
		t.Fatalf("回退路径 + 指针不在齿轮上：region=%v whole=%v，应为 false/true", region, whole)
	}
	if v, ok := w.lastIgnore(); !ok || !v {
		t.Errorf("整窗穿透应为 true，得到 %v（设置过：%v）", v, ok)
	}
	if rects, ok := w.lastRegion(); !ok || rects != nil {
		t.Errorf("回退路径要把输入区域收回（nil），得到 %+v（设置过：%v）", rects, ok)
	}

	// 指针压在设置键上：临时取消穿透，键才点得到。
	w = &fakeRegionWindow{bounds: mygo.Rectangle{Width: 560, Height: 100}}
	region, whole = lyricsApplyClickThrough(w, true, false, true)
	if region || whole {
		t.Fatalf("回退路径 + 指针在齿轮上：region=%v whole=%v，应为 false/false", region, whole)
	}

	// 不穿透：区域收回、整窗可交互。
	w = &fakeRegionWindow{bounds: mygo.Rectangle{Width: 560, Height: 100}}
	region, whole = lyricsApplyClickThrough(w, false, true, false)
	if region || whole {
		t.Fatalf("不穿透：region=%v whole=%v，应为 false/false", region, whole)
	}
	if rects, ok := w.lastRegion(); !ok || rects != nil {
		t.Errorf("不穿透时输入区域应收回（nil），得到 %+v", rects)
	}
}

// TestLyricsApplyClickThroughFallsBackWhenNoRects 钉住「区域可用但窗口还没布局」：
// 拿不到矩形时不能下发空区域（那等于整窗穿透，齿轮就点不到了），必须回退。
func TestLyricsApplyClickThroughFallsBackWhenNoRects(t *testing.T) {
	w := &fakeRegionWindow{} // 宽高为 0
	region, whole := lyricsApplyClickThrough(w, true, true, false)
	if region || !whole {
		t.Fatalf("窗口未布局：region=%v whole=%v，应为 false/true", region, whole)
	}
	if v, ok := w.lastIgnore(); !ok || !v {
		t.Errorf("回退路径应设整窗穿透 true，得到 %v（设置过：%v）", v, ok)
	}
}

// TestLyricsRegionChanged 钉住「穿透状态变化时会重新下发区域」的接线判据：
// 开关翻转一定要重下发，尺寸变化（设置页展开 / 宽度比例 / 显示器切换）也要，
// 否则可点区域会停在旧位置。
func TestLyricsRegionChanged(t *testing.T) {
	cases := []struct {
		name               string
		prevOn             bool
		prevW, prevH, w, h float32
		on                 bool
		want               bool
	}{
		{"首次打开穿透", false, 0, 0, 560, 100, true, true},
		{"关掉穿透", true, 560, 100, 560, 100, false, true},
		{"穿透中尺寸没变", true, 560, 100, 560, 100, true, false},
		{"穿透中窗口变宽（设置页展开）", true, 560, 100, 900, 100, true, true},
		{"穿透中窗口变高（设置页展开）", true, 560, 100, 560, 420, true, true},
		{"不穿透时尺寸变化不重下发", false, 560, 100, 900, 100, false, false},
	}
	for _, c := range cases {
		got := lyricsRegionChanged(c.prevOn, c.prevW, c.prevH, c.on, c.w, c.h)
		if got != c.want {
			t.Errorf("%s：lyricsRegionChanged = %v，应为 %v", c.name, got, c.want)
		}
	}
}

// TestLyricsClickThroughToggleRedownloadsRegion 钉住整条接线：穿透关 → 开 时，
// 窗口一定收到一次「设置键那一块」的输入区域，再切回关时区域被收回。
//
// 这就是「穿透状态变化会重新下发区域」的行为断言（applyLyricsClickThrough 每次
// 同步都会走 lyricsApplyClickThrough，不存在只在初始化时下发一次的漏洞）。
func TestLyricsClickThroughToggleRedownloadsRegion(t *testing.T) {
	w := &fakeRegionWindow{bounds: mygo.Rectangle{X: 0, Y: 0, Width: 560, Height: 100}}

	// 初始：不穿透。
	if region, _ := lyricsApplyClickThrough(w, false, true, false); region {
		t.Fatal("不穿透时不该走输入区域")
	}
	if rects, _ := w.lastRegion(); rects != nil {
		t.Fatalf("不穿透时应收回输入区域，得到 %+v", rects)
	}

	// 打开穿透：立刻下发设置键那一块。
	region, whole := lyricsApplyClickThrough(w, true, true, false)
	if !region || whole {
		t.Fatalf("打开穿透：region=%v whole=%v，应为 true/false", region, whole)
	}
	rects, _ := w.lastRegion()
	if len(rects) != 1 || rects[0] != (mygo.Rectangle{X: 524, Y: 0, Width: 36, Height: 36}) {
		t.Fatalf("打开穿透后下发的区域 = %+v，应为设置键那一块", rects)
	}

	// 关掉穿透：区域收回。
	if region, _ := lyricsApplyClickThrough(w, false, true, false); region {
		t.Fatal("关掉穿透后不该再走输入区域")
	}
	if rects, _ := w.lastRegion(); rects != nil {
		t.Fatalf("关掉穿透后应收回输入区域，得到 %+v", rects)
	}
}

// TestLyricsInputRegionMatchesGearHot 钉住「输入区域与设置键热区同源」：输入
// 区域的矩形必须与 lyricsGearHot 判定的那一块重合（绘制 / 点击 / 穿透三处一致）。
func TestLyricsInputRegionMatchesGearHot(t *testing.T) {
	w, h := float32(560), float32(100)
	rects := lyricsInputRegionRects(w, h)
	if len(rects) != 1 {
		t.Fatalf("输入区域有 %d 块，应为 1 块", len(rects))
	}
	r := rects[0]
	bounds := mygo.Rectangle{Width: int(w), Height: int(h)}
	// 矩形正中：既在输入区域里，也在设置键热区里。
	inside := mygo.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
	if !lyricsGearHot(inside, bounds) {
		t.Errorf("输入区域中心 %v 不在设置键热区里", inside)
	}
	// 设置键本身（view 常量算出的中心）也必须落在输入区域里。
	gear := mygo.Point{
		X: int(w - view.LyricsGearInset - view.LyricsGearSize/2),
		Y: int(view.LyricsGearInset + view.LyricsGearSize/2),
	}
	if gear.X < r.X || gear.X >= r.X+r.Width || gear.Y < r.Y || gear.Y >= r.Y+r.Height {
		t.Errorf("设置键中心 %v 不在输入区域 %+v 里（两处几何不同源）", gear, r)
	}
	// 字幕文字区：既不在输入区域里，也不该被判成「在齿轮上」。
	text := mygo.Point{X: 100, Y: 60}
	if lyricsGearHot(text, bounds) {
		t.Error("字幕文字区不该判成在齿轮上")
	}
}
