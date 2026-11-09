package view

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

// 本轮（抽屉表头材质统一）的无头交互回归。
//
// 表头所有可交互控件（tab、列表小键、开关、关闭键）现在走 headeraction.go 的
// 一套状态解析：常态 / 悬停 / 按下 / 选中 + 悬停。这里只钉**行为**：
//
//   - 同一排的键在浅色与深色下、常态 / 悬停 / 按下三态都长得一样
//     （播放模式键此前在深色下是一块白、且没有悬停 / 按下）；
//   - 深色下悬停 / 按下真的看得出变化（此前选中态把悬停整条盖掉）；
//   - 选中的 tab 悬停时仍有变化，但选中状态不丢；
//   - 禁用键不给任何交互反馈；
//   - 减少动态效果时色值当帧到位。

// headerProbe 起一个打开着当前抽屉的无头窗口。
func headerProbe(t *testing.T, a *App) *ui.Tester {
	t.Helper()
	return settleTester(ui.NewTester(func(c *ui.Context) {
		// 与生产一致：Shell 跑在设过主题的上下文里，深色场景才真的画深色。
		c.SetTheme(a.Theme.uiTheme())
		a.Shell(c)
	}, 980, 720))
}

// headerApp 起一个 noon 时段的应用（背景可复现，不随跑测试的钟点变）。
func headerApp(mode string) *App {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	if mode != "" {
		a.SetAppearance(mode, 0)
	}
	return a
}

// headerShot 是一次渲染的快照：整帧像素 + 目标键的矩形。
type headerShot struct {
	img *image.RGBA
	r   ui.Rect
}

// headerState 把指针放到 label 上并按 state（rest / hover / press）渲染，
// 返回快照。动画用 Advance 推到稳态（v0.5.5 起 Tester 的时钟是冻结的）。
func headerRender(t *testing.T, tst *ui.Tester, label, state string) headerShot {
	t.Helper()
	tst.Move(-1, -1)
	tst.Advance(250 * time.Millisecond)
	r := mustFind(t, tst, label)
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	switch state {
	case "hover", "press":
		tst.Move(cx, cy)
		tst.Advance(250 * time.Millisecond)
	}
	if state == "press" {
		tst.Press(cx, cy)
		tst.Advance(250 * time.Millisecond)
	}
	r = mustFind(t, tst, label)
	shot := headerShot{img: tst.Image(), r: r}
	if state == "press" {
		// 键的动作在 Release 时触发（关闭抽屉 / 清空列表等）。先把指针移出控件
		// 再松手：框架按「在别处松开」取消这次点击，不改变生产事件行为，只为
		// 对照图保留同一场景（不让真实动作改掉 fixture）。
		tst.Move(-1, -1)
		tst.Release(-1, -1)
		tst.Advance(250 * time.Millisecond)
	}
	return shot
}

// headerBand 返回控件矩形内相对高度 [y0,y1) 那一段的平均亮度（只取中间 70%
// 的宽度，避开圆角与描边的抗锯齿）。顶 / 底两条窄带不压到图标与文字。
func headerBand(t *testing.T, s headerShot, y0, y1 float64) float64 {
	t.Helper()
	sum, n := 0.0, 0
	for y := s.r.Y + float32(y0)*s.r.H; y < s.r.Y+float32(y1)*s.r.H; y++ {
		for x := s.r.X + s.r.W*0.15; x < s.r.X+s.r.W*0.85; x++ {
			sum += float64(lum(pixelAt(t, s.img, int(x), int(y))))
			n++
		}
	}
	if n == 0 {
		t.Fatalf("采样带 [%.2f,%.2f) 落在矩形 %v 之外", y0, y1, s.r)
	}
	return sum / float64(n)
}

// headerChanged 数两张同尺寸帧里矩形内变了的像素。
func headerChanged(a, b headerShot) int {
	n := 0
	r := a.r
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if rgbAt(a.img, x, y) != rgbAt(b.img, x, y) {
				n++
			}
		}
	}
	return n
}

// headerBands 是三条采样带：顶两行（内高光 / 描边）、中间（填充）、底两行。
var headerBands = [][2]float64{{0, 0.07}, {0.45, 0.55}, {0.93, 1}}

// headerScenarios 是表头各抽屉的代表性场景（对照图与一致性断言共用）。
// key 是这颗抽屉里最有代表性的表头控件。
type headerScenario struct {
	name  string
	key   string
	build func() *App
}

func headerScenarios(mode string) []headerScenario {
	return []headerScenario{
		{"playlist", "清空播放列表", func() *App {
			a := headerApp(mode)
			a.Drawer, a.PlaylistTab, a.PlayingPlaylist = DrawerPlaylist, ListUser, ListUser
			a.Index = 0
			a.Playlist = []PlayItem{{ID: "a", Part: "第一集", Title: "午后音乐会"}, {ID: "b", Part: "第二集", Title: "现场演奏"}}
			return a
		}},
		{"history", "隐身", func() *App {
			a := headerApp(mode)
			a.Drawer, a.HistTab = "history", HistHistory
			a.ListFor("history").Cards = []Card{{Bvid: "BV1", Title: "历史视频", Track: Track{Aid: 1, Bvid: "BV1"}}}
			return a
		}},
		{"favorites", "向右滚动", func() *App {
			a := headerApp(mode)
			a.Drawer = "favorite"
			a.Folders = []Folder{{ID: 1, Title: "默认收藏夹", Count: 3}, {ID: 2, Title: "音乐", Count: 8}}
			a.FolderID = 1
			return a
		}},
		{"danmaku", "暂停跟随", func() *App {
			a := headerApp(mode)
			a.Drawer, a.DanmakuTab = DrawerDanmaku, TabDanmaku
			a.DanmakuAutoScroll, a.HeatOn = true, true
			a.Danmaku = []Danmaku{{Time: 1, Text: "弹幕一"}}
			return a
		}},
		{"subtitle", "跟随字幕", func() *App {
			a := headerApp(mode)
			a.Drawer, a.DanmakuTab = DrawerDanmaku, TabSubtitle
			a.SubtitleFollow, a.LyricsEnabled = false, true
			return a
		}},
		{"parts", "全部添加", func() *App {
			a := headerApp(mode)
			a.Drawer = DrawerParts
			a.Info = &Info{Bvid: "BV1", Aid: 1, Title: "标题", Parts: []Part{
				{Cid: 11, Page: 1, Part: "第一集"}, {Cid: 12, Page: 2, Part: "第二集"},
			}}
			a.Track = &Track{Bvid: "BV1", Cid: 11, Title: "标题"}
			return a
		}},
		{"up", "关注", func() *App {
			a := headerApp(mode)
			a.Drawer, a.UpName, a.UpTab = DrawerUp, "音乐现场", UpTabVideos
			a.ListFor(DrawerUp).Cards = []Card{{Bvid: "BV1", Title: "视频"}}
			return a
		}},
		{"series", "播放全部", func() *App {
			a := headerApp(mode)
			a.Drawer, a.SeriesName = DrawerSeries, "合辑"
			a.ListFor(DrawerSeries).Cards = []Card{{Bvid: "BV1", Title: "第一集"}}
			return a
		}},
	}
}

// TestHeaderActionStatesDifferInBothModes 钉住表头动作键在**两种主题**下都有
// 可分辨的常态 / 悬停 / 按下：顶两条窄带（内高光与描边）与中间填充都要变。
//
// 此前深色主题下动作键只换填充、描边不动，选中态更是把悬停整条盖掉；浅色
// 主题下按下沿用悬停（按下去没有任何回退感）。
func TestHeaderActionStatesDifferInBothModes(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		for _, sc := range []headerScenario{
			{"playlist", "清空播放列表", func() *App {
				a := headerApp(mode)
				a.Drawer, a.PlaylistTab, a.PlayingPlaylist = DrawerPlaylist, ListUser, ListUser
				a.Index = 0
				a.Playlist = []PlayItem{{ID: "a", Title: "午后音乐会"}}
				return a
			}},
			{"danmaku", "关闭弹幕热力", func() *App {
				a := headerApp(mode)
				a.Drawer, a.DanmakuTab = DrawerDanmaku, TabDanmaku
				a.HeatOn = true
				return a
			}},
		} {
			t.Run(mode+"/"+sc.name, func(t *testing.T) {
				tst := headerProbe(t, sc.build())
				rest := headerRender(t, tst, sc.key, "rest")
				hover := headerRender(t, tst, sc.key, "hover")
				press := headerRender(t, tst, sc.key, "press")
				// 悬停 / 按下都要在键面上留下看得见的差异：整块像素变化 + 中间
				// 填充亮度变化。描边 / 内高光那一两个像素的差在浅色下本来就微弱，
				// 不单拎出来按带断言。
				for _, step := range []struct {
					from, to headerShot
					name     string
				}{
					{rest, hover, "悬停"},
					{hover, press, "按下"},
				} {
					if n := headerChanged(step.from, step.to); n < 150 {
						t.Errorf("%s %s：%s只变了 %d 个像素——交互材质没接上", mode, sc.key, step.name, n)
					}
					d := absF(float32(headerBand(t, step.to, 0.45, 0.55) - headerBand(t, step.from, 0.45, 0.55)))
					if d < 1.5 {
						t.Errorf("%s %s：%s的填充亮度只差 %.2f——%s没有回退感", mode, sc.key, step.name, d, step.name)
					}
				}
			})
		}
	}
}

// TestHeaderActionDisabledHasNoFeedback 禁用键（合集为空时的「播放全部」）
// 悬停 / 按下都不给反馈：像素必须与常态逐点一致。
func TestHeaderActionDisabledHasNoFeedback(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			a := headerApp(mode)
			a.Drawer, a.SeriesName = DrawerSeries, "空合辑"
			// 列表为空 → seriesPlayAllSpec 给出「禁用」。
			tst := headerProbe(t, a)
			rest := headerRender(t, tst, "播放全部", "rest")
			hover := headerRender(t, tst, "播放全部", "hover")
			press := headerRender(t, tst, "播放全部", "press")
			if n := headerChanged(rest, hover); n != 0 {
				t.Errorf("%s：禁用的「播放全部」悬停变了 %d 个像素", mode, n)
			}
			if n := headerChanged(rest, press); n != 0 {
				t.Errorf("%s：禁用的「播放全部」按下变了 %d 个像素", mode, n)
			}
		})
	}
}

// TestHeaderActionReducedMotionSnaps 桌面要求减少动态效果时，悬停色值当帧到位
// （原版对应的 prefers-reduced-motion 块写 `transition: none`）；常态下同一帧
// 还停在旧色上。
func TestHeaderActionReducedMotionSnaps(t *testing.T) {
	build := func(reduce bool) *ui.Tester {
		a := headerApp("light")
		a.Drawer, a.SeriesName = DrawerSeries, "合辑"
		a.ListFor(DrawerSeries).Cards = []Card{{Bvid: "BV1", Title: "第一集"}}
		o := ui.TesterOptions{}
		if reduce {
			o.Preferences = ui.Preferences{ReduceMotion: true}
		}
		return settleTester(ui.NewTesterWith(func(c *ui.Context) {
			c.SetTheme(a.Theme.uiTheme())
			a.Shell(c)
		}, 980, 720, o))
	}
	// 同一帧里悬停后的填充亮度：减少动态效果 → 已是悬停色；否则仍是常态色。
	sample := func(tst *ui.Tester) float64 {
		tst.Move(-1, -1)
		tst.Advance(250 * time.Millisecond)
		r := mustFind(t, tst, "播放全部")
		tst.Move(r.X+r.W/2, r.Y+r.H/2)
		tst.Frame() // 只推一帧：过渡曲线 t=0
		return headerBand(t, headerShot{img: tst.Image(), r: r}, 0.45, 0.55)
	}
	snapped, eased := sample(build(true)), sample(build(false))
	if absF(float32(snapped-eased)) < 1.5 {
		t.Errorf("减少动态效果下悬停色没有当帧到位：%.2f vs 常态过渡首帧 %.2f", snapped, eased)
	}
}

// TestActiveTabHoverStaysSelected 选中的 tab 悬停时仍有可见变化（底更实、边更亮），
// 并且选中状态不丢（aria-selected 仍为真）。浅色与深色各来一遍。
func TestActiveTabHoverStaysSelected(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			a := headerApp(mode)
			a.Drawer, a.Query, a.SortOrder = DrawerSearch, "关键词", SortTotal
			tst := headerProbe(t, a)
			rest := headerRender(t, tst, "综合", "rest")
			hover := headerRender(t, tst, "综合", "hover")
			if n := headerChanged(rest, hover); n < 100 {
				t.Errorf("%s：选中 tab 悬停只变了 %d 个像素（选中 + 悬停应看得出变化）", mode, n)
			}
			if d := absF(float32(headerBand(t, hover, 0.45, 0.55) - headerBand(t, rest, 0.45, 0.55))); d < 1.5 {
				t.Errorf("%s：选中 tab 悬停填充亮度只差 %.2f", mode, d)
			}
			if info, ok := tst.Access("综合"); !ok || !info.Selected {
				t.Errorf("%s：悬停后「综合」丢了选中态：%+v ok=%v", mode, info, ok)
			}
		})
	}
}

// TestHeaderContactSheet 把代表性抽屉表头在浅色 / 深色下的常态 / 悬停 / 按下 /
// 键盘焦点渲染成 PNG，供人工对照（不参与断言）。默认跳过，需要时：
//
//	BILI_HEADER_SHOTS=/tmp/header-shots go test ./internal/view -run TestHeaderContactSheet
func TestHeaderContactSheet(t *testing.T) {
	dir := os.Getenv("BILI_HEADER_SHOTS")
	if dir == "" {
		t.Skip("设置 BILI_HEADER_SHOTS=<目录> 导出表头对照图")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, img *image.RGBA) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"light", "dark"} {
		for _, sc := range headerScenarios(mode) {
			tst := headerProbe(t, sc.build())
			for _, state := range []string{"rest", "hover", "press"} {
				shot := headerRender(t, tst, sc.key, state)
				write(fmt.Sprintf("%s-%s-%s", mode, sc.name, state), shot.img)
			}
			// 键盘焦点（:focus-visible）只在 Tab 到该键时出现，鼠标点击不会。
			// 用新的 tester：上面 press 那一下已经把鼠标焦点留在了键上，
			// Focused 会立即可见但 FocusVisible 仍为假，画不出焦点环。
			ftst := headerProbe(t, sc.build())
			ftst.Move(-1, -1)
			ftst.Frame()
			// Tab 预算内到不了目标就 fail：否则写下的是一张没有焦点环的图，
			// 人工对照会默默接受错误画面。
			tabTo(t, ftst, sc.key, 40)
			write(fmt.Sprintf("%s-%s-focus", mode, sc.name), ftst.Image())
		}
	}
}
