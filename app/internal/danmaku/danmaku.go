// Package danmaku 是弹幕的可复用数据层与覆盖层排版引擎。
//
// 为什么单独一层：弹幕数据原先有两条**各自请求**的链路（弹幕抽屉的
// startDanmakuFetch、热力条的 startDanmakuHeat），视频窗再做一份就是第三份
// 拉取 + 解析。这里把「按 cid 取 + 并发去重 + 有界缓存 + 转成排版用的条目」
// 收成一份 Store；抽屉、热力条与视频覆盖层都从它拿。
//
// 排版/时间轴（Overlay）也在这里而不是 view 里：轨道分配、滑窗、上限、
// seek 重建全是纯计算，放在没有 ui 依赖的包里才能无头测试（喂一组会互相
// 追上的弹幕，直接断言同轨不重叠、长弹幕占更久）。view 只负责把 Sprites()
// 画成文字元素。
//
// 本轮范围是「基本弹幕」：滚动 + 顶部/底部固定。发送、点赞、举报、屏蔽词、
// 关键词高亮、智能防挡脸都不做（见 app/PLAN.md）。
package danmaku

import (
	"errors"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/vst93/bili-fm/app/internal/bilibili"
)

// errNoFetcher 是 Store 没有数据来源时的错误（测试里的空实现）。
var errNoFetcher = errors.New("danmaku: 没有配置弹幕来源")

// Kind 是弹幕的显示方式。数值与 B 站 XML 的 p[1] 对齐，转换时直接比。
type Kind uint8

const (
	// Scroll 是从右向左滚动的弹幕（B 站 type 1/2/3）。
	Scroll Kind = 1
	// Bottom 是底部固定弹幕（type 4）。
	Bottom Kind = 4
	// Top 是顶部固定弹幕（type 5）。
	Top Kind = 5
)

// Item 是一条可排版的弹幕。Time 是视频时间轴上的秒数。
type Item struct {
	Time  float64
	Text  string
	Color int
	Kind  Kind
}

// Settings 是弹幕的基本设置（都落盘，见 app/prefs.go 的 danmaku* 键）。
//
// Max 同时是性能护栏与「同屏密度」：超过就丢弃新来的条目，不做全量重建。
// 它没有单独的「密度」设置 —— 两个值表达的是同一件事，留两个只会让用户
// 和代码都要维护一份重复的映射。
type Settings struct {
	On      bool
	Opacity float64 // 0.3 ~ 1
	Size    int     // 0 小 / 1 中 / 2 大
	Area    float64 // 1 铺满整屏 / 0.5 只占上方 1/2
	Max     int     // 同屏上限
}

// 字号三档（DIP）。B 站网页版默认 25px，中档取 24。
var fontSizeSteps = [3]float32{18, 24, 30}

// 同屏上限三档：30 保守、60 默认、100 激进（100 是实测仍能保持单帧
// < 1ms 的档位，见 PLAN.md 的实测数字）。
var maxOnScreenSteps = [3]int{30, 60, 100}

// DefaultSettings 是首次运行的弹幕设置（不透明度 100% 正好落在设置面板
// 的档位上，打开面板时有一颗选中）。
func DefaultSettings() Settings {
	return Settings{On: true, Opacity: 1, Size: 1, Area: 1, Max: 60}
}

// Normalize 把任意输入夹进合法区间。读旧键 / 手改 KV / 未来加档位都走它，
// 不认识的值得体地退回默认，而不是让排版拿到 0 字号或负上限。
func (s Settings) Normalize() Settings {
	if s.Opacity <= 0 {
		s.Opacity = DefaultSettings().Opacity
	}
	s.Opacity = clampFloat(s.Opacity, 0.3, 1)
	if s.Size < 0 || s.Size > 2 {
		s.Size = 1
	}
	if s.Area < 0.5 || s.Area > 1 {
		s.Area = 1
	}
	if s.Max <= 0 {
		s.Max = DefaultSettings().Max
	}
	s.Max = int(clampFloat(float64(s.Max), 10, 200))
	return s
}

// FontSize 是当前档位的字号（DIP）。
func (s Settings) FontSize() float32 {
	s = s.Normalize()
	return fontSizeSteps[s.Size]
}

// MaxOnScreen 返回第 i 档（0~2）的同屏上限，供设置菜单的档位按钮使用。
func MaxOnScreen(i int) int {
	if i < 0 || i >= len(maxOnScreenSteps) {
		i = 1
	}
	return maxOnScreenSteps[i]
}

// FontSizeStep 返回第 i 档（0~2）的字号。
func FontSizeStep(i int) float32 {
	if i < 0 || i >= len(fontSizeSteps) {
		i = 1
	}
	return fontSizeSteps[i]
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// maxTextRunes 是参与排版的单条文本上限。B 站允许的弹幕本来就不长，超长的
// 基本是刷屏；截断后一条弹幕的宽度有上界，滑窗回看的时长才有上界
// （见 Overlay.maxLife），seek 重建也就不会因为一条 500 字弹幕扫全表。
const maxTextRunes = 60

func clipText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxTextRunes {
		return s
	}
	return string(r[:maxTextRunes-1]) + "…"
}

// EstimateWidth 是文本宽度的无 ui 估计：CJK 按 1em、其余按 0.55em。
// 无头测试与没有测量回调时用它；真实视频窗用 ui.Context.MeasureText 覆盖
// （见 view.VideoChrome.danmaku），所以屏幕上的轨道间距是按真实字形算的。
func EstimateWidth(text string, size float32) float32 {
	var w float32
	for _, r := range text {
		if r < 0x2E80 {
			w += size * 0.55
		} else {
			w += size
		}
	}
	return w
}

// FromBilibili 把接口弹幕转成排版条目：丢掉空文本与高级/代码弹幕，按时间
// 排序。type 2/3（小/大滚动）与 6（逆向）都按滚动处理 —— 逆向弹幕本轮不做
// 独立方向，按普通滚动显示，至少不会消失。
func FromBilibili(items []bilibili.DanmakuItem) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		text := clipText(it.Content)
		if text == "" || it.Time < 0 {
			continue
		}
		kind := Scroll
		switch it.Type {
		case int(Bottom):
			kind = Bottom
		case int(Top):
			kind = Top
		case 7, 8, 9:
			// 高级 / 代码 / BAS 弹幕带坐标与样式，本轮不做定位，直接跳过
			// 比把它们挤进滚动轨道更不干扰画面。
			continue
		}
		out = append(out, Item{Time: it.Time, Text: text, Color: it.Color, Kind: kind})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	return out
}

// ---------------------------------------------------------------- Store

// Fetcher 是取弹幕所需的最小接口：*bilibili.BL 满足它，测试可注入计数桩。
type Fetcher interface {
	GetDanmakuList(cid int) (*bilibili.DanmakuList, error)
}

// storeMaxCids 是缓存保留的视频数：正在播的一条 + 抽屉里浏览的一条 +
// 刚切走的一条，4 个足够覆盖「来回切集」，又不会把整季的弹幕都留在内存。
const storeMaxCids = 4

type storeEntry struct {
	items []Item
	err   error
	done  chan struct{}
}

// Store 按 cid 取弹幕：并发去重（同一条视频只发一次请求）、有界缓存
// （最多 storeMaxCids 条）、结果只读共享。
//
// 返回的切片不要原地改：多个调用方（抽屉、热力、视频窗）拿的是同一份。
type Store struct {
	f Fetcher

	mu      sync.Mutex
	entries map[int64]*storeEntry
	order   []int64
}

// NewStore 建一个 Store；f 为 nil 时 Get 返回空（测试里的「没有网络」）。
func NewStore(f Fetcher) *Store {
	return &Store{f: f, entries: make(map[int64]*storeEntry)}
}

// Get 返回 cid 的弹幕（按时间排序）。同一条 cid 的并发调用共享一次请求。
func (s *Store) Get(cid int64) ([]Item, error) {
	if s == nil || cid <= 0 {
		return nil, nil
	}
	s.mu.Lock()
	if e, ok := s.entries[cid]; ok {
		s.mu.Unlock()
		<-e.done
		return e.items, e.err
	}
	e := &storeEntry{done: make(chan struct{})}
	s.entries[cid] = e
	s.order = append(s.order, cid)
	s.evictLocked(cid)
	s.mu.Unlock()

	var (
		items []Item
		err   error
	)
	if s.f == nil {
		err = errNoFetcher
	} else if d, ferr := s.f.GetDanmakuList(int(cid)); ferr != nil {
		err = ferr
	} else if d != nil {
		items = FromBilibili(d.Items)
	}
	e.items, e.err = items, err
	close(e.done)
	if err != nil {
		// 失败不进缓存：网络抖一下之后用户再点刷新应当能重试，而不是永远
		// 拿到同一条错误（成功的空结果仍然缓存，省掉重复请求）。
		s.mu.Lock()
		if cur := s.entries[cid]; cur == e {
			delete(s.entries, cid)
			for i, c := range s.order {
				if c == cid {
					s.order = append(s.order[:i], s.order[i+1:]...)
					break
				}
			}
		}
		s.mu.Unlock()
	}
	return items, err
}

// Cached 返回已完成的缓存项（不触发请求）。
func (s *Store) Cached(cid int64) ([]Item, bool) {
	if s == nil || cid <= 0 {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[cid]
	if !ok {
		return nil, false
	}
	select {
	case <-e.done:
		return e.items, true
	default:
		return nil, false
	}
}

// Len 报告缓存里的视频数（测试用）。
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// evictLocked 淘汰最老的已完成项；在途请求不动（否则等待方会永远等一个
// 已被移出 map 的 entry，虽然 done 仍会关闭，但缓存语义就乱了）。
func (s *Store) evictLocked(keep int64) {
	for len(s.order) > storeMaxCids {
		idx := -1
		for i, cid := range s.order {
			if cid == keep {
				continue
			}
			e := s.entries[cid]
			if e == nil {
				idx = i
				break
			}
			select {
			case <-e.done:
				idx = i
			default:
			}
			if idx >= 0 {
				break
			}
		}
		if idx < 0 {
			return
		}
		cid := s.order[idx]
		s.order = append(s.order[:idx], s.order[idx+1:]...)
		delete(s.entries, cid)
	}
}

// ---------------------------------------------------------------- Overlay

const (
	// fixedSeconds 是顶部 / 底部固定弹幕的停留时长。
	fixedSeconds = 4.0
	// scrollSeconds 是「零宽文本穿过整屏」的基准时长；实际速度 = 宽度/7.5，
	// 且不低于 100px/s。速度只由屏宽决定（不随文本长度变），所以同一轨道
	// 里的弹幕永远不会互相追上 —— 轨道空闲判据只需「前一条的尾巴离开右沿」。
	scrollSeconds = 7.5
	minScrollPx   = 100.0
	// resyncSlack 是本地时钟与播放位置的允许偏差。超过就按 seek 处理：
	// 时钟对齐播放位置并重建可视集。
	resyncSlack = 0.4
	// maxFrameStep 是单帧推进上限（秒）：窗口被挂起 / 断点调试后回来，
	// 不能把一大段时间一次算进弹幕时间轴。
	maxFrameStep = 0.25
	// lineFactor 是行高相对字号的倍数。
	lineFactor = 1.4
)

// Sprite 是一条已经排好轨道的弹幕。X 是这一帧的左边界（DIP）。
type Sprite struct {
	Item
	Lane  int
	W     float32
	X     float32
	Y     float32
	Enter float64
	Leave float64
}

// laneLayout 是「屏宽/屏高/设置」确定后的轨道几何。设置或窗口尺寸变化时
// 整体重排一次；每帧只比较它是否相等，不做任何按条目的全量计算。
type laneLayout struct {
	font        float32
	lineH       float32
	areaH       float32
	lanes       int
	topCount    int
	bottomCount int
	scrollFirst int
	scrollCount int
	speed       float64
	max         int
}

func layoutFor(w, h float32, s Settings, reserveFixed bool) laneLayout {
	font := s.FontSize()
	lineH := font * lineFactor
	areaH := h * float32(s.Area)
	lanes := int(areaH / lineH)
	if lanes < 1 {
		lanes = 1
	}
	// 上限 30 行：再多的轨道在 4K 屏上也没人读得过来，而且每帧的轨道
	// 扫描成本随行数增长（见 advance 的 enter）。
	if lanes > 30 {
		lanes = 30
	}
	top, bottom := 0, 0
	if reserveFixed {
		switch {
		case lanes >= 8:
			top, bottom = 2, 2
		case lanes >= 4:
			top, bottom = 1, 1
		}
	}
	scrollCount := lanes - top - bottom
	if scrollCount < 1 {
		scrollCount = 1
		top, bottom = 0, 0
		lanes = 1
	}
	speed := float64(w) / scrollSeconds
	if speed < minScrollPx {
		speed = minScrollPx
	}
	return laneLayout{
		font: font, lineH: lineH, areaH: areaH,
		lanes: lanes, topCount: top, bottomCount: bottom,
		scrollFirst: top, scrollCount: scrollCount,
		speed: speed, max: s.Max,
	}
}

// Overlay 是视频窗上的弹幕层：滑动窗口 + 轨道分配 + 时间轴推进。
//
// 零值可用；所有方法都只在该窗口的视图构建（主线程）里调用。
type Overlay struct {
	items   []Item
	fixed   bool // 这一集有没有顶部/底部固定弹幕（决定要不要预留固定带）
	measure func(text string, size float32) float32

	inited      bool
	clock       float64
	lastPlaying bool
	width       float32
	height      float32
	lay         laneLayout

	next     int
	active   []Sprite
	laneFree []float64

	rebuilds int
	dropped  int
}

// SetMeasure 换文本宽度测量函数（真实视频窗传 ui.Context.MeasureText）。
func (o *Overlay) SetMeasure(fn func(text string, size float32) float32) {
	o.measure = fn
}

// SetItems 换一集的弹幕。排序副本，调用方可以复用切片。
// 传 nil 即清空（换集 / 关窗）。
func (o *Overlay) SetItems(items []Item) {
	if len(items) == 0 {
		o.items = nil
	} else {
		o.items = append(o.items[:0], items...)
		sort.SliceStable(o.items, func(i, j int) bool { return o.items[i].Time < o.items[j].Time })
	}
	o.fixed = false
	for _, it := range o.items {
		if it.Kind == Top || it.Kind == Bottom {
			o.fixed = true
			break
		}
	}
	o.inited = false
	o.active = o.active[:0]
	o.next = 0
	o.clock = 0
}

// Items 报告当前这一集的条目数（测试 / 调试）。
func (o *Overlay) Items() int { return len(o.items) }

// Active 报告当前同屏条目数。
func (o *Overlay) Active() int { return len(o.active) }

// HasItems 报告这一集有没有弹幕（视图据此决定要不要逐帧重绘）。
func (o *Overlay) HasItems() bool { return len(o.items) > 0 }

// Rebuilds 是可视集重建次数（seek / 换设置 / 换窗口尺寸）。上限淘汰时它
// 不应增长 —— 测试用这个断言「超过上限是丢弃而不是全量重建」。
func (o *Overlay) Rebuilds() int { return o.rebuilds }

// Dropped 是被丢掉的新弹幕条数（轨道占满 / 同屏上限）。
func (o *Overlay) Dropped() int { return o.dropped }

// Sync 推进一帧。
//
//   - pos 是播放器位置（秒），时钟以它为准：偏差超过 resyncSlack 视为 seek，
//     对齐并重建可视集；
//   - playing 为假时时钟不推进（暂停即停）；
//   - rate 是倍速，本地时钟按 dt*rate 前进（倍速下弹幕同步加速）；
//   - 每帧只处理「过期 / 新入场」，不遍历全表。
func (o *Overlay) Sync(pos float64, playing bool, rate float64, dt float64, w, h float32, s Settings) {
	s = s.Normalize()
	if !s.On || w <= 0 || h <= 0 || len(o.items) == 0 {
		o.active = o.active[:0]
		o.next = 0
		o.inited = false
		o.lastPlaying = playing
		return
	}
	if rate <= 0 {
		rate = 1
	}
	if dt < 0 {
		dt = 0
	}
	lay := layoutFor(w, h, s, o.fixed)
	needRebuild := !o.inited || lay != o.lay || w != o.width || h != o.height
	if needRebuild {
		o.width, o.height, o.lay = w, h, lay
		o.clock = pos
		o.rebuild()
		o.inited = true
	} else {
		if playing {
			if o.lastPlaying {
				step := dt * rate
				if step > maxFrameStep {
					step = maxFrameStep
				}
				o.clock += step
			} else {
				// 从暂停恢复：暂停期间的时间不算进来，以播放器为准。
				o.clock = pos
			}
		}
		if drift := pos - o.clock; math.Abs(drift) > resyncSlack {
			o.clock = pos
			o.rebuild()
		} else {
			o.advance()
		}
	}
	o.lastPlaying = playing
}

// Clear 清空可视集但保留条目（开关关掉时用）。
func (o *Overlay) Clear() {
	o.active = o.active[:0]
	o.next = 0
	o.inited = false
}

// Sprites 返回这一帧要画的条目（X 已按当前时钟算好）。
func (o *Overlay) Sprites() []Sprite {
	for i := range o.active {
		o.active[i].X = o.xAt(o.active[i])
	}
	return o.active
}

// maxLife 是「一条弹幕从入场到离场」的最长时间上界：滑窗重建只回看这么久，
// 再早的条目不可能还在屏上。
func (o *Overlay) maxLife() float64 {
	maxW := float64(maxTextRunes) * float64(o.lay.font)
	scroll := (float64(o.width) + maxW) / o.lay.speed
	if scroll < fixedSeconds {
		scroll = fixedSeconds
	}
	return scroll
}

// rebuild 按当前时钟重建可视集：只从 [clock-maxLife, clock] 里取条目，
// 用同样的贪心轨道分配摆好。只在 seek / 换设置 / 换尺寸时发生，不是每帧。
func (o *Overlay) rebuild() {
	o.active = o.active[:0]
	o.laneFree = make([]float64, o.lay.lanes)
	for i := range o.laneFree {
		o.laneFree[i] = math.Inf(-1)
	}
	start := o.clock - o.maxLife()
	o.next = sort.Search(len(o.items), func(i int) bool { return o.items[i].Time >= start })
	for o.next < len(o.items) && o.items[o.next].Time <= o.clock {
		o.enter(o.items[o.next])
		o.next++
	}
	o.rebuilds++
}

// advance 是每帧的热路径：先回收离屏，再让到点的条目入场。
func (o *Overlay) advance() {
	keep := o.active[:0]
	for _, sp := range o.active {
		if sp.Leave > o.clock {
			keep = append(keep, sp)
		}
	}
	o.active = keep
	for o.next < len(o.items) && o.items[o.next].Time <= o.clock {
		o.enter(o.items[o.next])
		o.next++
	}
}

// enter 给一条弹幕找轨道并入场；没有空轨或到达同屏上限就丢弃（计数）。
func (o *Overlay) enter(it Item) {
	if len(o.active) >= o.lay.max {
		o.dropped++
		return
	}
	// Kind 的零值是「未指定」：按滚动处理，xAt 才不会把它当成固定弹幕居中。
	if it.Kind != Top && it.Kind != Bottom {
		it.Kind = Scroll
	}
	lane, ok := o.freeLane(it.Kind, it.Time)
	if !ok {
		o.dropped++
		return
	}
	width := float32(0)
	if o.measure != nil {
		width = o.measure(it.Text, o.lay.font)
	} else {
		width = EstimateWidth(it.Text, o.lay.font)
	}
	sp := Sprite{Item: it, Lane: lane, W: width, Enter: it.Time}
	switch it.Kind {
	case Top:
		sp.Leave = it.Time + fixedSeconds
		o.laneFree[lane] = sp.Leave
		sp.Y = float32(lane) * o.lay.lineH
	case Bottom:
		sp.Leave = it.Time + fixedSeconds
		o.laneFree[lane] = sp.Leave
		// 底部轨道段从下往上排：段内第一条落在最下面一行。
		idx := lane - (o.lay.lanes - o.lay.bottomCount)
		sp.Y = o.lay.areaH - float32(o.lay.bottomCount-idx)*o.lay.lineH
	default:
		sp.Leave = it.Time + (float64(o.width)+float64(width))/o.lay.speed
		// 同轨下一条要等这一条的尾巴离开右沿才能入场；速度相同，之后
		// 永远不会追上。
		o.laneFree[lane] = it.Time + float64(width)/o.lay.speed
		sp.Y = float32(lane-o.lay.scrollFirst)*o.lay.lineH + float32(o.lay.topCount)*o.lay.lineH
	}
	o.active = append(o.active, sp)
}

// freeLane 找一条在 at 时刻空闲的轨道。滚动 / 顶部 / 底部各用各的轨道段，
// 固定弹幕不会盖在滚动弹幕上。
func (o *Overlay) freeLane(kind Kind, at float64) (int, bool) {
	first, count := 0, 0
	switch kind {
	case Top:
		first, count = 0, o.lay.topCount
	case Bottom:
		first, count = o.lay.lanes-o.lay.bottomCount, o.lay.bottomCount
	default:
		first, count = o.lay.scrollFirst, o.lay.scrollCount
	}
	for i := first; i < first+count; i++ {
		if o.laneFree[i] <= at {
			return i, true
		}
	}
	return 0, false
}

func (o *Overlay) xAt(sp Sprite) float32 {
	if sp.Kind != Scroll {
		return (o.width - sp.W) / 2
	}
	return o.width - float32(o.lay.speed*(o.clock-sp.Enter))
}
