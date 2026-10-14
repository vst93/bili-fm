package view

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestDialogTypeStyle 钉住对话框类型的样子（原版 DialogProvider 的 iconForType /
// accentForType）：每种类型一个 28px 图标 + 图标色 + 面板强调色；loading 是转圈，
// 空值 / 未知值都退到 info。
func TestDialogTypeStyle(t *testing.T) {
	cases := []struct {
		typ     string
		icon    *ui.SVG
		ink     string
		tint    float32
		spinner bool
	}{
		{"success", iconCheckOne, "#22c55e", 0.10, false},
		{"error", iconCloseOne, "#ef4444", 0.10, false},
		{"warning", iconWarn, "#f59e0b", 0.10, false},
		{"question", iconRefresh, "#0ea5e9", 0.10, false},
		{"loading", nil, "#0ea5e9", 0.06, true},
		{"info", iconInfo, "#0ea5e9", 0.06, false},
		{"", iconInfo, "#0ea5e9", 0.06, false},
		{"nonsense", iconInfo, "#0ea5e9", 0.06, false},
	}
	for _, tc := range cases {
		got := dialogTypeStyleFor(tc.typ)
		if got.Icon != tc.icon || got.Spinner != tc.spinner {
			t.Errorf("dialogTypeStyleFor(%q) 图标 = %p / spinner %v，应为 %p / %v",
				tc.typ, got.Icon, got.Spinner, tc.icon, tc.spinner)
		}
		if got.Ink != ui.Hex(tc.ink) {
			t.Errorf("dialogTypeStyleFor(%q) 图标色 = %v，应为 %v", tc.typ, got.Ink, ui.Hex(tc.ink))
		}
		if want := ui.Hex(tc.ink).Alpha(tc.tint); got.Tint != want {
			t.Errorf("dialogTypeStyleFor(%q) 强调色 = %v，应为 %v", tc.typ, got.Tint, want)
		}
	}
}

// TestDialogSheenStops 钉住 .liquid-dialog::after 的四个停靠点
// （白 .35 → 透明 35% → 透明 65% → 白 .10）。
func TestDialogSheenStops(t *testing.T) {
	want := []struct {
		at    float32
		alpha float32
	}{{0, 0.35}, {0.35, 0}, {0.65, 0}, {1, 0.10}}
	if len(dialogSheenStops) != len(want) {
		t.Fatalf("停靠点 %d 个，应为 %d 个", len(dialogSheenStops), len(want))
	}
	for i, w := range want {
		s := dialogSheenStops[i]
		if s.At != w.at {
			t.Errorf("第 %d 个停靠点位置 = %v，应为 %v", i, s.At, w.at)
		}
		if s.C != ui.Hex("#ffffff").Alpha(w.alpha) {
			t.Errorf("第 %d 个停靠点颜色 = %v，应为白 .%v", i, s.C, w.alpha)
		}
	}
}

// TestDialogButtonStyles 钉住两种按钮的两态取值（原版 .liquid-dialog-btn /
// .liquid-dialog-btn-primary）。
func TestDialogButtonStyles(t *testing.T) {
	normal := dialogSecondaryStyleFor(false)
	if normal.Fill != ui.Hex("#ffffff").Alpha(0.45) ||
		normal.Border != ui.Hex("#ffffff").Alpha(0.50) ||
		normal.Ink != ui.Hex("#475569") || normal.Shadow.A != 0 {
		t.Errorf("普通键常态 = %+v，应为白 .45 / 边 .50 / #475569 / 无投影", normal)
	}
	hover := dialogSecondaryStyleFor(true)
	if hover.Fill != ui.Hex("#ffffff").Alpha(0.65) ||
		hover.Border != ui.Hex("#ffffff").Alpha(0.85) ||
		hover.Ink != ui.Hex("#1e293b") ||
		hover.Shadow != ui.Hex("#0f172a").Alpha(0.08) {
		t.Errorf("普通键悬停 = %+v，应为白 .65 / 边 .85 / #1e293b / slate .08", hover)
	}
	from, to := dialogPrimaryStops(false)
	if from != ui.Hex("#0ea5e9").Alpha(0.88) || to != ui.Hex("#38bdf8").Alpha(0.82) {
		t.Errorf("主键常态渐变 = %v → %v，应为 #0ea5e9 .88 → #38bdf8 .82", from, to)
	}
	from, to = dialogPrimaryStops(true)
	if from != ui.Hex("#0ea5e9").Alpha(0.95) || to != ui.Hex("#38bdf8").Alpha(0.90) {
		t.Errorf("主键悬停渐变 = %v → %v，应为 #0ea5e9 .95 → #38bdf8 .90", from, to)
	}
}

// modalProbe 建一个 1000×700 的主窗，可选装一个对话框。
//
// 入场动画（180ms 缩放 + 淡入）按 modalOpenedAt 算进度：把时间戳拨到过去，
// 首帧的 AnimateTo 通道是新建的、直接取目标值（scale 1 / opacity 1），
// 测试不用等真实动画。
func modalProbe(t *testing.T, m *Modal) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	if m != nil {
		a.Modal = m
		a.modalOpenedAt = time.Now().Add(-time.Second)
	}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	return tst
}

// countDominant 在 img 的一块区域里数「以某个通道为主色」的像素（该通道至少
// 比另外两个高 40、且本身 ≥120）。用来在无头渲染里找类型图标的颜色。
func countDominant(img *image.RGBA, x0, y0, x1, y1, dom int) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
				continue
			}
			i := img.PixOffset(x, y)
			ch := [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
			if ch[dom] < 120 {
				continue
			}
			if ch[dom]-ch[(dom+1)%3] < 40 || ch[dom]-ch[(dom+2)%3] < 40 {
				continue
			}
			n++
		}
	}
	return n
}

// TestModalTypeIconPixels 是无头渲染实测：对话框标题左侧真的画出了类型图标，
// 且颜色按类型取 —— error 是红叉、success 是绿勾（原版 iconForType）。区域按
// 几何取：图标 28px + 间距 16，落在标题左缘往左 44px..14px 那条带里。
func TestModalTypeIconPixels(t *testing.T) {
	cases := []struct {
		typ    string
		dom    int // 0 红 / 1 绿
		name   string
		other  int
		others string
	}{
		{"error", 0, "红", 1, "绿"},
		{"success", 1, "绿", 0, "红"},
	}
	for _, tc := range cases {
		m := &Modal{
			Kind: "message", Type: tc.typ, Title: "登录失败", Message: "取二维码失败",
			Buttons: []ModalButton{{Label: "确定", Primary: true}},
		}
		tst := modalProbe(t, m)
		rect, ok := tst.Find("登录失败")
		if !ok {
			t.Fatalf("%s：找不到对话框标题", tc.typ)
		}
		x0, x1 := int(rect.X)-44, int(rect.X)-14
		y0, y1 := int(rect.Y)-4, int(rect.Y)+30
		got := countDominant(tst.Image(), x0, y0, x1, y1, tc.dom)
		wrong := countDominant(tst.Image(), x0, y0, x1, y1, tc.other)
		t.Logf("%s：%s像素 %d，%s像素 %d（区域 %d,%d..%d,%d）",
			tc.typ, tc.name, got, tc.others, wrong, x0, y0, x1, y1)
		if got < 20 {
			t.Errorf("%s 对话框标题左侧没有%s色图标像素：%d", tc.typ, tc.name, got)
		}
		if wrong > 2 {
			t.Errorf("%s 对话框的图标带了%s色像素：%d（颜色取错了）", tc.typ, tc.others, wrong)
		}
	}
}

// TestModalMaxWidth 钉住 .liquid-dialog 的 `max-width: 460px`：长正文在面板内
// 换行，面板不会为了装下一整行而无限变宽（460 面板的内容宽 = 460 - 64 padding -
// 28 图标 - 16 间距 = 350）。
func TestModalMaxWidth(t *testing.T) {
	long := "用音频聆听 B 站内容，既是音乐播放器，也是知识学习工具，也是知识学习工具。"
	tst := modalProbe(t, &Modal{Kind: "message", Type: "info", Title: "关于 bili-FM", Message: long})
	r, ok := tst.Find(long)
	if !ok {
		t.Fatalf("找不到长正文：%v", tst.Texts())
	}
	t.Logf("长正文 rect = %+v", r)
	if r.W > 352 {
		t.Errorf("正文宽 %v，应被 460 面板（内容 350）限制", r.W)
	}
	if r.H < 36 {
		t.Errorf("正文没有换行：高 %v", r.H)
	}
	if r.X < 16 || r.X+r.W > 1000-16 {
		t.Errorf("正文越出窗口：%+v", r)
	}
}

// TestModalButtons 钉住底部按钮：调用点给什么就画什么（原版 DialogConfig.buttons），
// 不给时是默认主按钮「确定」。
func TestModalButtons(t *testing.T) {
	tst := modalProbe(t, &Modal{
		Kind: "message", Type: "info", Title: "快捷键", Message: "播放 / 暂停：空格键",
		Buttons: []ModalButton{{Label: "知道了", Primary: true}},
	})
	if !slices.Contains(tst.Texts(), "知道了") {
		t.Errorf("快捷键对话框没有「知道了」按钮：%v", tst.Texts())
	}
	if slices.Contains(tst.Texts(), "确定") {
		t.Errorf("快捷键对话框不该有默认按钮「确定」：%v", tst.Texts())
	}

	tst = modalProbe(t, &Modal{Kind: "message", Type: "info", Title: "关于 bili-FM"})
	if !slices.Contains(tst.Texts(), "确定") {
		t.Errorf("不给按钮时应画默认主按钮「确定」：%v", tst.Texts())
	}
}

// TestDialogLinkPaint 钉住 .dialog-link 的两态取值（原版 globals.css）：
// 文字常态 #0284c7、悬停转 #0ea5e9；1px 底边常态 rgba(2,132,199,.30)、
// 悬停 rgba(14,165,233,.60)。此前原生版用框架 ui.Link 的 accent 蓝 + 悬停
// text-decoration 下划线，两项都对不上。
func TestDialogLinkPaint(t *testing.T) {
	cases := []struct {
		name    string
		hovered bool
		ink     ui.Color
		edge    ui.Color
	}{
		{"常态", false, ui.Hex("#0284c7"), ui.Hex("#0284c7").Alpha(0.30)},
		{"悬停", true, ui.Hex("#0ea5e9"), ui.Hex("#0ea5e9").Alpha(0.60)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dialogLinkInk(c.hovered); !sameInk(got, c.ink) {
				t.Errorf("文字色 = %v，应为 %v", got, c.ink)
			}
			if got := dialogLinkEdge(c.hovered); !sameInk(got, c.edge) {
				t.Errorf("底边色 = %v，应为 %v", got, c.edge)
			}
		})
	}
}

// TestDialogLinkOpensURL 无头实测：关于对话框里的项目链接点一下就开系统浏览器，
// 打开的是配置里的 URL（自绘 Row 的 OnClick 接线，替代了原先 ui.Link 的内建行为）。
func TestDialogLinkOpensURL(t *testing.T) {
	const url = "https://github.com/vst93/bili-fm"
	label := "项目地址：" + url
	tst := modalProbe(t, &Modal{
		Kind: "message", Type: "info", Title: "关于 bili-FM",
		Message:   "用音频聆听 B 站内容。",
		LinkLabel: label, LinkURL: url,
	})
	if !slices.Contains(tst.Texts(), label) {
		t.Fatalf("对话框没有链接文案：%v", tst.Texts())
	}
	if err := tst.Click(label); err != nil {
		t.Fatalf("点链接失败：%v", err)
	}
	tst.Frame()
	if got := tst.OpenedURLs(); !slices.Contains(got, url) {
		t.Errorf("点链接没有打开 %q：%v", url, got)
	}
}

// TestDialogLinkHoverInk 无头实测：链接悬停后真的换成更亮的 #0ea5e9（原版
// .dialog-link:hover）。量的是链接矩形内的「蓝度」Σ(B−R)——悬停色比常态色蓝
// 通道高约 22，底边也从 .30 加深到 .60，整块的蓝度应明显上升。
func TestDialogLinkHoverInk(t *testing.T) {
	const url = "https://github.com/vst93/bili-fm"
	label := "项目地址：" + url
	blueFor := func(hover bool) (int, ui.Rect) {
		tst := modalProbe(t, &Modal{
			Kind: "message", Type: "info", Title: "关于 bili-FM",
			Message:   "用音频聆听 B 站内容。",
			LinkLabel: label, LinkURL: url,
		})
		r, ok := tst.Find(label)
		if !ok {
			t.Fatalf("找不到链接：%v", tst.Texts())
		}
		if hover {
			// 悬停到链接中心：先移动指针，再重画一帧让 Hovered() 生效。
			tst.Move(r.X+r.W/2, r.Y+r.H/2)
			tst.Frame()
		}
		img := tst.Image()
		x0, y0 := int(r.X)-3, int(r.Y)-3
		x1, y1 := int(r.X+r.W)+3, int(r.Y+r.H)+5
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		if x1 > img.Rect.Dx() {
			x1 = img.Rect.Dx()
		}
		if y1 > img.Rect.Dy() {
			y1 = img.Rect.Dy()
		}
		total := 0
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				i := img.PixOffset(x, y)
				total += int(img.Pix[i+2]) - int(img.Pix[i])
			}
		}
		return total, r
	}
	off, r := blueFor(false)
	on, _ := blueFor(true)
	t.Logf("链接矩形 %+v 蓝度：常态 %d → 悬停 %d（+%d）", r, off, on, on-off)
	if on-off < 2000 {
		t.Errorf("悬停后链接蓝度只涨了 %d（常态 %d、悬停 %d）：悬停态没接上？", on-off, off, on)
	}
}

// TestDialogLinkBottomEdge 无头实测：链接下方画了一条 1px 的「底边」（原版
// .dialog-link 的 border-bottom），且不是 text-decoration 下划线——它在链接
// 文字之下多留 1px、横贯整条链接宽度。扫文字底边往下几行，找那条「整行泛蓝」
// 的像素带；把 BorderWidth(0,0,1,0) 去掉（回到纯文字）后这条带子应消失。
func TestDialogLinkBottomEdge(t *testing.T) {
	const url = "https://github.com/vst93/bili-fm"
	label := "项目地址：" + url
	tst := modalProbe(t, &Modal{
		Kind: "message", Type: "info", Title: "关于 bili-FM",
		Message:   "用音频聆听 B 站内容。",
		LinkLabel: label, LinkURL: url,
	})
	r, ok := tst.Find(label)
	if !ok {
		t.Fatalf("找不到链接：%v", tst.Texts())
	}
	img := tst.Image()
	width := int(r.W)
	best, bestY := 0, -1
	for y := int(r.Y); y <= int(r.Y+r.H)+4; y++ {
		if y < 0 || y >= img.Rect.Dy() {
			continue
		}
		bluish := 0
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := img.PixOffset(x, y)
			if int(img.Pix[i+2])-int(img.Pix[i]) > 20 {
				bluish++
			}
		}
		if bluish > best {
			best, bestY = bluish, y
		}
	}
	t.Logf("链接矩形 %+v：最蓝的一行在 y=%d，覆盖 %d/%d 像素", r, bestY, best, width)
	// 文字本身有反锯齿，某些行也会泛蓝；底边要求几乎横贯整条链接宽度。
	if best < width*4/5 {
		t.Errorf("链接下方没有横贯的 1px 底边：最蓝一行只覆盖 %d/%d 像素（y=%d）",
			best, width, bestY)
	}
}
