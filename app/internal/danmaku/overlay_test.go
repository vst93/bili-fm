package danmaku

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/vst93/bili-fm/app/internal/bilibili"
)

func testSettings() Settings {
	s := DefaultSettings()
	s.Max = 100
	return s
}

// syncRun 把 Overlay 从 0 秒推进到 seconds，每帧 1/60 秒，播放器位置与
// 本地时钟同速（rate=1）。fn 每帧拿到当时的 sprites。
func syncRun(o *Overlay, seconds float64, s Settings, fn func(t float64, sprites []Sprite)) {
	const dt = 1.0 / 60
	for t := 0.0; t <= seconds+1e-9; t += dt {
		o.Sync(t, true, 1, dt, 960, 540, s)
		if fn != nil {
			fn(t, o.Sprites())
		}
	}
}

func overlap(a, b Sprite) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W
}

// TestScrollLanesNeverOverlap 是本轮最核心的轨道约束：喂一组「如果速度随
// 文本长度变化就一定会互相追上」的弹幕（长的一条先入场、短的紧随其后），
// 逐帧断言同一轨道内任意两条的水平区间永不相交。
func TestScrollLanesNeverOverlap(t *testing.T) {
	// 密集到必然复用轨道：0~2s 每 0.05s 一条，长度从长到短交替。
	items := make([]Item, 0, 120)
	texts := []string{
		"这一条非常非常非常长，如果按长度分配速度就一定会被后面的短弹幕追上",
		"短", "也短", "还是短", "中等长度的弹幕测试", "短2",
	}
	for i := 0; i < 120; i++ {
		items = append(items, Item{Time: float64(i) * 0.05, Text: texts[i%len(texts)]})
	}
	o := &Overlay{}
	o.SetItems(items)
	s := testSettings()
	frames, checked, maxActive := 0, 0, 0
	syncRun(o, 25, s, func(now float64, sprites []Sprite) {
		frames++
		if len(sprites) > maxActive {
			maxActive = len(sprites)
		}
		byLane := map[int][]Sprite{}
		for _, sp := range sprites {
			if sp.Kind != Scroll {
				continue
			}
			byLane[sp.Lane] = append(byLane[sp.Lane], sp)
		}
		for lane, list := range byLane {
			for i := 0; i < len(list); i++ {
				for j := i + 1; j < len(list); j++ {
					checked++
					if overlap(list[i], list[j]) {
						t.Fatalf("t=%.2fs 轨道 %d 两条重叠：%q(x=%.1f,w=%.1f) / %q(x=%.1f,w=%.1f)",
							now, lane, list[i].Text, list[i].X, list[i].W, list[j].Text, list[j].X, list[j].W)
					}
				}
			}
		}
	})
	if frames == 0 || checked == 0 {
		t.Fatalf("没有检查到同轨配对：frames=%d checked=%d", frames, checked)
	}
	if maxActive == 0 {
		t.Error("逐帧检查期间一条弹幕都没上屏")
	}
}

// TestLongItemBlocksLaneLonger 钉住「长弹幕占轨更久」：长的一条入场后，
// 紧接着的短弹幕不能挤进同一轨；等它的尾巴离开右沿后，该轨才可复用。
func TestLongItemBlocksLaneLonger(t *testing.T) {
	long := Item{Time: 0, Text: "很长很长很长很长很长很长很长很长很长很长很长很长"}
	o := &Overlay{}
	o.SetItems([]Item{long, {Time: 0.2, Text: "紧跟"}, {Time: 0.3, Text: "再跟"}})
	s := testSettings()
	o.Sync(0, true, 1, 0, 960, 540, s)
	sprites := o.Sprites()
	if len(sprites) != 1 {
		t.Fatalf("t=0 应只有长弹幕，得到 %d", len(sprites))
	}
	longLane := sprites[0].Lane
	longW := sprites[0].W
	if longW <= EstimateWidth("紧跟", s.FontSize())*3 {
		t.Fatalf("长弹幕宽度 %.1f 没有明显大于短弹幕", longW)
	}

	o.Sync(0.2, true, 1, 0.2, 960, 540, s)
	sprites = o.Sprites()
	if len(sprites) != 2 {
		t.Fatalf("t=0.2 应有 2 条，得到 %d", len(sprites))
	}
	for _, sp := range sprites[1:] {
		if sp.Lane == longLane {
			t.Fatalf("长弹幕还在入场，短弹幕不该复用轨道 %d（X=%.1f W=%.1f）", longLane, sp.X, sp.W)
		}
	}

	// 长弹幕尾巴离开右沿的时刻 = 入场 + 宽度/速度；之后同轨可以复用。
	free := long.Time + float64(longW)/o.lay.speed
	o2 := &Overlay{}
	o2.SetItems([]Item{long, {Time: free + 0.05, Text: "复用"}})
	o2.Sync(0, true, 1, 0, 960, 540, s)
	o2.Sync(free+0.05, true, 1, free+0.05, 960, 540, s)
	for _, sp := range o2.Sprites() {
		if sp.Text == "复用" && sp.Lane != longLane {
			t.Errorf("长弹幕尾巴离开后应复用轨道 %d，得到 %d", longLane, sp.Lane)
		}
	}
}

// TestFixedTopBottom 钉住顶部 / 底部固定弹幕：水平居中、分别落在上/下带，
// 且滚动弹幕不会占用固定带。
func TestFixedTopBottom(t *testing.T) {
	o := &Overlay{}
	o.SetItems([]Item{
		{Time: 0, Text: "顶部", Kind: Top},
		{Time: 0, Text: "底部", Kind: Bottom},
		{Time: 0, Text: "滚动", Kind: Scroll},
	})
	s := testSettings()
	o.Sync(0, true, 1, 0, 960, 540, s)
	got := map[Kind]Sprite{}
	for _, sp := range o.Sprites() {
		got[sp.Kind] = sp
	}
	top, ok := got[Top]
	if !ok {
		t.Fatal("顶部固定弹幕没有出现")
	}
	bottom, ok := got[Bottom]
	if !ok {
		t.Fatal("底部固定弹幕没有出现")
	}
	scroll, ok := got[Scroll]
	if !ok {
		t.Fatal("滚动弹幕没有出现")
	}
	if top.Y >= o.lay.areaH/2 || bottom.Y <= o.lay.areaH/2 {
		t.Errorf("固定弹幕位置不对：top.y=%.1f bottom.y=%.1f areaH=%.1f", top.Y, bottom.Y, o.lay.areaH)
	}
	if math.Abs(float64(top.X-(960-top.W)/2)) > 0.01 {
		t.Errorf("顶部弹幕没有水平居中：x=%.1f w=%.1f", top.X, top.W)
	}
	if scroll.Lane < o.lay.topCount || scroll.Lane >= o.lay.lanes-o.lay.bottomCount {
		t.Errorf("滚动弹幕占用了固定带：lane=%d top=%d bottom=%d lanes=%d",
			scroll.Lane, o.lay.topCount, o.lay.bottomCount, o.lay.lanes)
	}
}

// TestPauseFreezesClock 钉住「暂停即停」：播放器位置不动时，推进再多帧
// 弹幕也不动。
func TestPauseFreezesClock(t *testing.T) {
	o := &Overlay{}
	o.SetItems([]Item{{Time: 0, Text: "暂停测试"}})
	s := testSettings()
	o.Sync(1, true, 1, 0, 960, 540, s)
	x0 := o.Sprites()[0].X
	for i := 0; i < 120; i++ {
		o.Sync(1, false, 1, 1.0/60, 960, 540, s)
	}
	x1 := o.Sprites()[0].X
	if x0 != x1 {
		t.Errorf("暂停时弹幕移动了：x %.1f → %.1f", x0, x1)
	}
}

// TestSeekRebuildsVisibleSet 钉住 seek：跳到 100s 后只显示 100s 附近的条目，
// 10s 附近的旧条目全部回收。
func TestSeekRebuildsVisibleSet(t *testing.T) {
	items := []Item{
		{Time: 9, Text: "旧1"}, {Time: 10, Text: "旧2"},
		{Time: 100, Text: "新1"}, {Time: 100.2, Text: "新2"},
	}
	o := &Overlay{}
	o.SetItems(items)
	s := testSettings()
	o.Sync(10, true, 1, 0, 960, 540, s)
	if got := len(o.Sprites()); got == 0 {
		t.Fatal("10s 处应有旧弹幕")
	}
	before := o.Rebuilds()
	o.Sync(100, true, 1, 1.0/60, 960, 540, s)
	if o.Rebuilds() != before+1 {
		t.Errorf("seek 应触发一次重建：before=%d after=%d", before, o.Rebuilds())
	}
	for _, sp := range o.Sprites() {
		if sp.Time < 99 {
			t.Errorf("seek 后仍显示旧弹幕 %q（time=%.1f）", sp.Text, sp.Time)
		}
	}
	found := false
	for _, sp := range o.Sprites() {
		if sp.Text == "新1" {
			found = true
		}
	}
	if !found {
		t.Errorf("seek 后没有显示 100s 的弹幕：%+v", o.Sprites())
	}
}

// TestRateScalesClock 钉住倍速：同样的真实时间，2x 下弹幕推进距离约为 1x 的
// 两倍（时钟按 dt*rate 走，播放器位置也按 2x 前进，不触发 resync）。
func TestRateScalesClock(t *testing.T) {
	s := testSettings()
	measure := func(rate float64) float64 {
		o := &Overlay{}
		o.SetItems([]Item{{Time: 0, Text: "倍速测试弹幕"}})
		o.Sync(0, true, rate, 0, 960, 540, s)
		x0 := o.Sprites()[0].X
		const dt = 1.0 / 60
		pos := 0.0
		for i := 0; i < 60; i++ {
			pos += dt * rate
			o.Sync(pos, true, rate, dt, 960, 540, s)
		}
		return float64(x0 - o.Sprites()[0].X)
	}
	d1, d2 := measure(1), measure(2)
	if d1 <= 0 {
		t.Fatalf("1x 下弹幕没有移动：%.2f", d1)
	}
	if ratio := d2 / d1; ratio < 1.8 || ratio > 2.2 {
		t.Errorf("2x/1x 位移比 = %.2f，期望约 2", ratio)
	}
}

// TestMaxOnScreenDropsWithoutRebuild 钉住性能护栏的淘汰语义：同屏上限满了
// 以后新弹幕被丢弃，可视集不重建（不是每来一条就清空重排）。
func TestMaxOnScreenDropsWithoutRebuild(t *testing.T) {
	s := testSettings()
	s.Max = 30
	items := make([]Item, 0, 400)
	for i := 0; i < 400; i++ {
		items = append(items, Item{Time: float64(i) * 0.02, Text: fmt.Sprintf("密集弹幕 %d", i)})
	}
	o := &Overlay{}
	o.SetItems(items)
	rebuilds := 0
	syncRun(o, 8, s, func(_ float64, sprites []Sprite) {
		if len(sprites) > s.Max {
			t.Fatalf("同屏 %d 条超过上限 %d", len(sprites), s.Max)
		}
	})
	rebuilds = o.Rebuilds()
	if rebuilds != 1 {
		t.Errorf("上限淘汰不该重建可视集，rebuilds=%d（应为首次布局的 1）", rebuilds)
	}
	if o.Active() != s.Max {
		t.Errorf("同屏条数 = %d，上限 %d 应被占满", o.Active(), s.Max)
	}
	if o.Dropped() == 0 {
		t.Error("超过上限后应有丢弃计数")
	}
}

// TestSetItemsClears 钉住换集：换一集（含 nil）后旧可视集立刻消失。
func TestSetItemsClears(t *testing.T) {
	o := &Overlay{}
	o.SetItems([]Item{{Time: 0, Text: "上一集"}})
	s := testSettings()
	o.Sync(0, true, 1, 0, 960, 540, s)
	if o.Active() == 0 {
		t.Fatal("换集前应有弹幕")
	}
	o.SetItems(nil)
	o.Sync(0, true, 1, 0, 960, 540, s)
	if o.Active() != 0 {
		t.Errorf("换集后仍有 %d 条弹幕", o.Active())
	}
}

// TestWindowCostVsFullScan 是性能护栏的量化证据：2 万条弹幕、600 帧，
// 滑窗 Sync 的每帧耗时对比「每帧扫全表」的朴素做法。两个数字都打印出来
// （PLAN.md 引用的是同一组量级）。
func TestWindowCostVsFullScan(t *testing.T) {
	const n = 20000
	items := make([]Item, n)
	for i := range items {
		items[i] = Item{Time: float64(i) * 0.05, Text: fmt.Sprintf("弹幕 %d", i)}
	}
	s := testSettings()
	o := &Overlay{}
	o.SetItems(items)
	o.Sync(0, true, 1, 0, 960, 540, s)

	const frames = 600
	const dt = 1.0 / 60
	start := time.Now()
	pos := 0.0
	for i := 0; i < frames; i++ {
		pos += dt
		o.Sync(pos, true, 1, dt, 960, 540, s)
	}
	windowedTotal := time.Since(start)
	windowed := windowedTotal.Seconds() * 1e6 / frames

	// 朴素对照：每帧遍历全表，挑出 [pos-10, pos] 的条目（只做判定，不排版）。
	start = time.Now()
	pos = 0.0
	visible := 0
	for i := 0; i < frames; i++ {
		pos += dt
		lo := pos - 10
		visible = 0
		for _, it := range items {
			if it.Time >= lo && it.Time <= pos {
				visible++
			}
		}
	}
	full := time.Since(start).Seconds() * 1e6 / frames
	t.Logf("2 万条弹幕、600 帧：滑窗 Sync %.2fµs/帧（同屏 %d 条），全表扫描 %.1fµs/帧（可见 %d 条），加速 %.0fx",
		windowed, o.Active(), full, visible, full/windowed)
	if windowed > 200 {
		t.Errorf("滑窗单帧 %.2fµs，超过 200µs 护栏", windowed)
	}
	if full/windowed < 3 {
		t.Errorf("滑窗只比全表快 %.1fx，护栏没有意义", full/windowed)
	}
}

// TestStoreDedupesAndCaches 钉住数据层：同一条 cid 的并发请求只发一次，
// 第二次直接命中缓存。
func TestStoreDedupesAndCaches(t *testing.T) {
	f := &fakeFetcher{delay: 30 * time.Millisecond}
	s := NewStore(f)
	done := make(chan struct{}, 8)
	for i := 0; i < 8; i++ {
		go func() {
			if _, err := s.Get(42); err != nil {
				t.Errorf("Get: %v", err)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if f.calls != 1 {
		t.Errorf("并发 8 次只该请求 1 次，实际 %d", f.calls)
	}
	if _, err := s.Get(42); err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 {
		t.Errorf("缓存命中后不该再请求，calls=%d", f.calls)
	}
	if _, ok := s.Cached(42); !ok {
		t.Error("Cached 应命中")
	}
}

// TestStoreBounded 钉住缓存有界：超过 4 条视频后淘汰最老的已完成项。
func TestStoreBounded(t *testing.T) {
	f := &fakeFetcher{}
	s := NewStore(f)
	for cid := int64(1); cid <= 8; cid++ {
		if _, err := s.Get(cid); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.Len(); got > storeMaxCids {
		t.Errorf("缓存 %d 条，上限 %d", got, storeMaxCids)
	}
}

type fakeFetcher struct {
	delay time.Duration
	calls int
}

func (f *fakeFetcher) GetDanmakuList(cid int) (*bilibili.DanmakuList, error) {
	f.calls++
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return &bilibili.DanmakuList{Items: []bilibili.DanmakuItem{
		{Content: "第一条", Time: 1.5, Type: 1, Color: 0xffffff},
		{Content: "第二条", Time: 2.5, Type: 5, Color: 0xff0000},
	}}, nil
}

// TestFromBilibili 钉住转换：排序、丢空文本、丢高级/代码弹幕、按 60 字符截断。
func TestFromBilibili(t *testing.T) {
	long := strings.Repeat("长", 80)
	items := FromBilibili([]bilibili.DanmakuItem{
		{Content: "后面的", Time: 9, Type: 1},
		{Content: "   ", Time: 2, Type: 1},
		{Content: "顶部", Time: 3, Type: 5},
		{Content: "底部", Time: 4, Type: 4},
		{Content: "高级", Time: 5, Type: 7},
		{Content: "代码", Time: 6, Type: 8},
		{Content: long, Time: 1, Type: 1},
	})
	if len(items) != 4 {
		t.Fatalf("条目数 = %d，想要 4：%+v", len(items), items)
	}
	for i := 1; i < len(items); i++ {
		if items[i].Time < items[i-1].Time {
			t.Errorf("没有按时间排序：%+v", items)
		}
	}
	if items[0].Kind != Scroll || items[1].Kind != Top || items[2].Kind != Bottom {
		t.Errorf("kind 映射不对：%+v", items)
	}
	if n := len([]rune(items[0].Text)); n != maxTextRunes {
		t.Errorf("超长文本截断到 %d 字符，想要 %d", n, maxTextRunes)
	}
}
