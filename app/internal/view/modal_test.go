package view

import (
	"image"
	"math"
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

// TestDialogMessageLineHeight 钉住 .dialog-message 的行高（原版 globals.css:5679
// 的 `font-size:13.5px; line-height:1.7`，即每行 13.5×1.7=22.95px）。原生版
// 此前只写了 FontSize(13.5)，框架取字体自然行盒（本机实测约 20px），每行少
// 约 3px，多行正文与 `\n\n` 段间距都会明显比原版挤。
func TestDialogMessageLineHeight(t *testing.T) {
	tst := modalProbe(t, &Modal{
		Kind: "message", Type: "info", Title: "关于 bili-FM",
		Message: "第一行正文\n\n第二行正文",
	})
	one, ok := tst.Find("第一行正文")
	if !ok {
		t.Fatalf("找不到第一行正文：%v", tst.Texts())
	}
	two, ok := tst.Find("第二行正文")
	if !ok {
		t.Fatalf("找不到第二行正文：%v", tst.Texts())
	}
	t.Logf("第一行 %+v，第二行 %+v", one, two)
	// 单行行盒 ≈22.95（不给 LineHeight 时实测约 20）。
	if one.H < 22 || one.H > 23.5 {
		t.Errorf("正文行高 = %v，应 ≈22.95（原版 line-height:1.7）", one.H)
	}
	// 「第一行 + 空行 + 第二行」两行间距 = 2×22.95 = 45.9（原版）。
	gap := two.Y - one.Y
	if gap < 45 || gap > 47 {
		t.Errorf("两行（含中间空行）的间距 = %v，应 ≈45.9（2×22.95）", gap)
	}
}

// TestDialogSpinnerArc 钉住加载转圈顶弧的区间：原版 `.dialog-loading-spinner`
// 是「3px 浅蓝整圈 + 深蓝顶弧」，顶弧占圆环上 1/4（border-top-color）。圆环
// 路径从右侧（0）起、顺时针一圈为 1，顶弧基准区间是 [0.625, 0.875)；旋转 ph
// （0→1 一圈）把这个区间整体沿圆周平移，越过起点（1）时拆分。
func TestDialogSpinnerArc(t *testing.T) {
	cases := []struct {
		ph         float64
		start, end float32
		wrap       bool
	}{
		{0, 0.625, 0.875, false},
		{0.1, 0.725, 0.975, false},
		{0.25, 0.875, 0.125, true}, // 转 1/4 圈时顶弧跨过起点
		{0.3, 0.925, 0.175, true},
		{0.5, 0.125, 0.375, false},
		{0.75, 0.375, 0.625, false},
		{0.9, 0.525, 0.775, false},
	}
	for _, c := range cases {
		start, end, wrap := dialogSpinnerArc(c.ph)
		if absF(start-c.start) > 1e-5 || absF(end-c.end) > 1e-5 || wrap != c.wrap {
			t.Errorf("dialogSpinnerArc(%v) = (%v, %v, %v)，应为 (%v, %v, %v)",
				c.ph, start, end, wrap, c.start, c.end, c.wrap)
		}
	}
	// 区间宽度恒为 1/4（顶弧就是四分之一圈）。
	for _, ph := range []float64{0, 0.13, 0.4, 0.62, 0.99} {
		start, end, wrap := dialogSpinnerArc(ph)
		width := end - start
		if wrap {
			width += 1
		}
		if absF(width-0.25) > 1e-5 {
			t.Errorf("dialogSpinnerArc(%v) 的弧宽 = %v，应为 0.25", ph, width)
		}
	}
}

// absF 是 float32 绝对值。
func absF(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestDialogSpinnerRing 是无头渲染实测：loading 对话框左侧画的是**圆环**
// （空心），而不是 mygo ui.Spinner 的 12 根辐条（中心密、边缘疏）。扫标题左侧
// 那 28px 带里所有泛蓝像素（B−R > 15：浅蓝环 rgba(14,165,233,.15) 铺在面板
// 底上 B−R ≈ 40），取它们的包围盒——圆环的包围盒近似正方形且直径 ≈ 28−3；
// 再查包围盒正中心一小块：圆环中心是空的（辐条的中心是实的）。
func TestDialogSpinnerRing(t *testing.T) {
	tst := modalProbe(t, &Modal{
		Kind: "message", Type: "loading", Title: "检查更新中",
		Message: "正在检查更新…",
	})
	r, ok := tst.Find("检查更新中")
	if !ok {
		t.Fatalf("找不到对话框标题：%v", tst.Texts())
	}
	img := tst.Image()
	// 图标/转圈在标题左侧 28px + 间距 16 的带里（同 TestModalTypeIconPixels 的几何）。
	x0, x1 := int(r.X)-48, int(r.X)-16
	y0, y1 := int(r.Y)-8, int(r.Y)+36
	bluish := func(x, y int) bool {
		if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
			return false
		}
		i := img.PixOffset(x, y)
		// 浅蓝环 rgba(14,165,233,.15) 铺在面板底上 B−R ≈ 40；面板的 loading
		// 强调色 tint（#0ea5e9 .06）只有 ≈21，阈值取 30 把两者分开。
		return int(img.Pix[i+2])-int(img.Pix[i]) > 30
	}
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if bluish(x, y) {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if maxX < 0 {
		t.Fatalf("标题左侧没有泛蓝像素：转圈没画出来（区域 %d,%d..%d,%d）", x0, y0, x1, y1)
	}
	w, h := maxX-minX+1, maxY-minY+1
	t.Logf("转圈泛蓝像素包围盒 %d,%d..%d,%d（%d×%d）", minX, minY, maxX, maxY, w, h)
	// 圆环外径 ≈ 28（含描边），包围盒宽高都应在 24..30。
	if w < 24 || w > 30 || h < 24 || h > 30 {
		t.Errorf("转圈包围盒 %d×%d，应 ≈ 28×28 的圆环", w, h)
	}
	// 圆环中心是空的：正中心 5×5 里几乎不该有泛蓝像素。
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	hollow := 0
	for y := cy - 2; y <= cy+2; y++ {
		for x := cx - 2; x <= cx+2; x++ {
			if bluish(x, y) {
				hollow++
			}
		}
	}
	if hollow > 3 {
		t.Errorf("转圈中心有 %d/25 个泛蓝像素，圆环应是空心的", hollow)
	}
	// 关键判据：圆环在中心线半径（(28−3)/2 = 12.5）上是**连续**的。扫一圈 72
	// 个方向（每 5°）、每个方向在半径 12.5±1.5 里找泛蓝像素，圆环应几乎全覆盖；
	// mygo 的 ui.Spinner 是 12 根 2.5px 宽的辐条（在半径 12.5 处只盖住 ≈1/3
	// 的方向），覆盖率会低到 ≈0.35。这条把两者分开。
	const rad = 12.5
	hit := 0
	for k := 0; k < 72; k++ {
		ang := float64(k) * 5 * math.Pi / 180
		ok := false
		for d := -1.5; d <= 1.5; d += 0.5 {
			rr := rad + d
			px := cx + int(math.Round(rr*math.Cos(ang)))
			py := cy + int(math.Round(rr*math.Sin(ang)))
			if bluish(px, py) {
				ok = true
				break
			}
		}
		if ok {
			hit++
		}
	}
	coverage := float64(hit) / 72
	t.Logf("半径 %v 上 72 个方向的泛蓝覆盖率 = %.2f（%d/72）", rad, coverage, hit)
	if coverage < 0.9 {
		t.Errorf("圆环在半径 %v 上不连续：覆盖率 %.2f（辐条 spinner ≈ 0.35，圆环应 ≥ 0.9）", rad, coverage)
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

// TestModalButtonOnClick 钉住对话框按钮的动作语义：带 OnClick 的按钮点击走
// 它自己（不自动关面板，用于「立即更新」「取消」这类会切换面板内容的动作）；
// 不带 OnClick 的按钮沿用惯例——关掉当前对话框。「检查更新」从原生更新窗
// 改成应用内对话框后依赖这条。
func TestModalButtonOnClick(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	clicked, closed := false, false
	a.Act.CloseModal = func() { closed = true }
	a.Modal = &Modal{
		Kind: "message", Type: "question", Title: "发现新版本",
		Message: "新版本 v1.2.3 已发布",
		Buttons: []ModalButton{
			{Label: "立即更新", Primary: true, OnClick: func() { clicked = true }},
			{Label: "稍后再说"},
		},
	}
	a.modalOpenedAt = time.Now().Add(-time.Second)
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()

	if err := tst.Click("立即更新"); err != nil {
		t.Fatalf("点不到「立即更新」：%v", err)
	}
	tst.Frame()
	if !clicked {
		t.Fatal("带 OnClick 的按钮没有触发回调")
	}
	if closed {
		t.Fatal("带 OnClick 的按钮不该自动关面板")
	}

	closed = false
	if err := tst.Click("稍后再说"); err != nil {
		t.Fatalf("点不到「稍后再说」：%v", err)
	}
	tst.Frame()
	if !closed {
		t.Fatal("不带 OnClick 的按钮应沿用「关面板」的惯例")
	}
}
