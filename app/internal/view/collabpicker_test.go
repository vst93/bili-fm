package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// collabProbe 建一个 1000×700 的主窗（固定时段主题，可复现），主区是一个
// 三位参与者的合作视频，返回 App 与无头窗口，用来验证合作 UP 主选择面板。
//
// 参与者特意分成「无角色」（主UP / 乙，条目恰好 34 高）与「带角色」（甲，
// 两行文本会更高），并且主 UP 的名字与面板默认显示的 UP 主一致 —— 这样
// `Find("主UP")` 命中的是主区文案、面板首条目要靠几何算。
func collabProbe(t *testing.T, hour int) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(hour))
	a.Info = &Info{
		Bvid: "BV1collab", OwnerMid: 100, OwnerName: "主UP", OwnerFace: "face-owner",
		Staff: []Collaborator{
			{Mid: 100, Name: "主UP"},
			{Mid: 200, Name: "甲", Title: "联合投稿"},
			{Mid: 300, Name: "乙"},
		},
	}
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tt.Frame()
	return a, tt
}

// openCollab 点开合作 UP 主选择键并返回触发键与首条目的矩形（首条目由面板
// 几何推出：左缘 = 触发键左缘 + padding 4、上缘 = 触发键下缘 + gap 8 + padding 4、
// 宽 = min(240,100vw-24) - 8）。
func openCollab(t *testing.T, tt *ui.Tester) (btn, first ui.Rect) {
	t.Helper()
	if err := tt.Click("选择合作 UP 主"); err != nil {
		t.Fatalf("点不到合作 UP 主选择键：%v", err)
	}
	tt.Frame()
	var ok bool
	btn, ok = tt.Find("选择合作 UP 主")
	if !ok {
		t.Fatal("找不到选择键")
	}
	first = ui.Rect{
		X: btn.X + collabMenuPad,
		Y: btn.Y + btn.H + collabMenuGap + collabMenuPad,
		W: collabMenuWidth - 2*collabMenuPad,
		H: collabItemMinH,
	}
	return btn, first
}

// TestCollabMenuWidthFor 钉住面板宽度 min(240px, 100vw - 24px)：宽窗恒 240，
// 窗口收窄时跟着收（不顶出屏幕）。
func TestCollabMenuWidthFor(t *testing.T) {
	if got := collabMenuWidthFor(1000); got != collabMenuWidth {
		t.Errorf("1000px 宽 = %v，应为 %d", got, collabMenuWidth)
	}
	if got := collabMenuWidthFor(200); got != 176 {
		t.Errorf("200px 宽 = %v，应为 176（100vw - 24）", got)
	}
	if got := collabMenuWidthFor(240 + 24); got != collabMenuWidth {
		t.Errorf("恰好放得下时应是 %d，得到 %v", collabMenuWidth, got)
	}
}

// TestCollabItemStyleFor 钉住 `.video-owner-picker-item` 的三态：
//
//	常态  transparent + #334155 + 无内影 + scale 1
//	悬停  --glass-bg-hover + inset 0 1px 1px --glass-highlight + 蓝字
//	选中  蓝字（背景仍由悬停决定）
//	按下  --glass-bg-active + inset 0 2px 4px rgba(15,23,42,.08) + scale .98
func TestCollabItemStyleFor(t *testing.T) {
	th := ThemeFor(PeriodAt(12))
	navy := ui.Hex(collabItemInk)

	normal := collabItemStyleFor(th, false, false, false)
	if normal.Bg.A != 0 || normal.Ink != navy || normal.InsetInk.A != 0 || normal.Scale != 1 {
		t.Errorf("常态 = %+v，应为透明底/#334155/无内影/scale 1", normal)
	}

	hover := collabItemStyleFor(th, false, true, false)
	if hover.Bg != th.GlassHover || hover.InsetInk != th.Highlight ||
		hover.InsetDy != 1 || hover.InsetBlur != 1 {
		t.Errorf("悬停 = %+v，应为 --glass-bg-hover + 1/1 内高光", hover)
	}
	if hover.Ink != th.Blue {
		t.Errorf("悬停字色 = %v，应为蓝 %v", hover.Ink, th.Blue)
	}

	// 选中但没悬停：底色仍是透明，只有字色转蓝。
	checked := collabItemStyleFor(th, true, false, false)
	if checked.Bg.A != 0 || checked.Ink != th.Blue {
		t.Errorf("选中 = %+v，应为透明底 + 蓝字", checked)
	}

	pressed := collabItemStyleFor(th, false, true, true)
	wantInk := ui.Hex(collabPressedInk).Alpha(collabPressedAlpha)
	if pressed.Bg != th.GlassActive || pressed.InsetInk != wantInk ||
		pressed.InsetDy != 2 || pressed.InsetBlur != 4 || pressed.Scale != 0.98 {
		t.Errorf("按下 = %+v，应为 --glass-bg-active + 2/4 深内影 + scale .98", pressed)
	}
	// 按下且选中：字色仍蓝（按下只覆盖底色/内影/缩放）。
	if s := collabItemStyleFor(th, true, false, true); s.Ink != th.Blue {
		t.Errorf("按下+选中字色 = %v，应为蓝", s.Ink)
	}
}

// TestCollabTriggerStyleFor 钉住选择键的规则（原版
// `.nav-icon-btn.video-owner-picker-button` 与 [aria-expanded=true]）：
// 常态透明 + #475569；展开铺玻璃底 + 内高光且图标转蓝；按下走 active 底。
func TestCollabTriggerStyleFor(t *testing.T) {
	th := ThemeFor(PeriodAt(12))
	neutral := ui.Hex(collabTriggerInk)

	rest := collabTriggerStyleFor(th, false, false, false)
	if rest.Bg.A != 0 || rest.Ink != neutral || rest.InsetInk.A != 0 {
		t.Errorf("常态 = %+v，应为透明底 + #475569 + 无内高光", rest)
	}

	hover := collabTriggerStyleFor(th, false, true, false)
	if hover.Bg != th.GlassHover || hover.Ink != neutral || hover.InsetInk != th.Highlight {
		t.Errorf("悬停 = %+v，应为玻璃底 + 中性图标 + 内高光", hover)
	}

	open := collabTriggerStyleFor(th, true, false, false)
	if open.Bg != th.GlassHover || open.Ink != th.Blue || open.InsetInk != th.Highlight {
		t.Errorf("展开 = %+v，应为玻璃底 + 蓝图标 + 内高光", open)
	}

	pressed := collabTriggerStyleFor(th, false, true, true)
	if pressed.Bg != th.GlassActive {
		t.Errorf("按下底 = %v，应为 --glass-bg-active %v", pressed.Bg, th.GlassActive)
	}
}

// TestCollabMenuGeometry 无头打开面板，钉住几何（对齐原版
// `.video-owner-picker-menu`）：面板左缘与选择键左缘对齐、上缘在键下缘 8px 处、
// 条目内缩 4、宽 240-8、无角色的条目恰好 34 高、条目自上而下紧排。
func TestCollabMenuGeometry(t *testing.T) {
	_, tt := collabProbe(t, 12)
	btn, first := openCollab(t, tt)
	t.Logf("触发键 %+v 首条目 %+v", btn, first)

	// 首条目（主UP，无角色）与面板几何。
	got, ok := tt.Find("乙")
	if !ok {
		t.Fatal("面板里看不到参与者「乙」")
	}
	// 条目的左缘 = 面板左缘 + padding；面板左缘贴触发键左缘。
	if d := got.X - (btn.X + collabMenuPad); d < -1.5 || d > 1.5 {
		t.Errorf("条目左缘 = %.1f，应为触发键左缘 %.1f + %d", got.X, btn.X, collabMenuPad)
	}
	// 面板本轮补上 1px 边框（原版 `.video-owner-picker-menu` 的白边），
	// box-sizing: border-box 下条目宽相应少 2：240 - 2*4 - 2 = 230。
	if d := got.W - (collabMenuWidth - 2*collabMenuPad - 2); d < -1.5 || d > 1.5 {
		t.Errorf("条目宽 = %.1f，应为 %d", got.W, collabMenuWidth-2*collabMenuPad-2)
	}
	if d := got.H - collabItemMinH; d < -1.5 || d > 1.5 {
		t.Errorf("无角色条目高 = %.1f，应为 %d", got.H, collabItemMinH)
	}

	// 首条目（主UP）上缘：面板贴着触发键下方 8px，再加 4 内边距。
	wantTop := btn.Y + btn.H + collabMenuGap + collabMenuPad
	if d := first.Y - wantTop; d < -1.5 || d > 1.5 {
		t.Errorf("首条目上缘 = %.1f，应为 %.1f（键下缘 + %d + %d）",
			first.Y, wantTop, collabMenuGap, collabMenuPad)
	}
	// 甲（带角色）紧排在主UP 之下：它的上缘 = 首条目下缘（主UP 34 高）。
	role, ok := tt.Find("甲")
	if !ok {
		t.Fatal("面板里看不到参与者「甲」")
	}
	if d := role.Y - (first.Y + collabItemMinH); d < -1.5 || d > 1.5 {
		t.Errorf("甲上缘 = %.1f，应紧接主UP（%.1f + %d）", role.Y, first.Y, collabItemMinH)
	}
	// 带角色的条目更高（两行文本），确认 name/role 是两行而不是挤一行。
	if role.H <= collabItemMinH {
		t.Errorf("带角色条目高 = %.1f，应高于 %d（两行）", role.H, collabItemMinH)
	}
}

// TestCollabMenuInk 无头实测条目文案色：取「乙」条目矩形里最暗像素，
// 应接近原版 `.video-owner-picker-item` 的 #334155（不是系统菜单观感）。
func TestCollabMenuInk(t *testing.T) {
	_, tt := collabProbe(t, 12)
	openCollab(t, tt)
	r, ok := tt.Find("乙")
	if !ok {
		t.Fatal("面板里看不到「乙」")
	}
	got := darkestIn(tt.Image(), int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
	t.Logf("「乙」最暗像素 = %v，目标 #334155 = [51 65 85]", got)
	for i, want := range [3]int{51, 65, 85} {
		if d := got[i] - want; d < -30 || d > 30 {
			t.Fatalf("条目文案色 = %v，应接近 #334155（通道%d 差 %d）", got, i, d)
		}
	}
}

// maxBlueMinusRed 给出矩形内「最蓝」的那个像素的 B - R（蓝字远大于灰字）。
func maxBlueMinusRed(img *image.RGBA, x0, y0, x1, y1 int) int {
	best := -1 << 20
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := img.PixOffset(x, y)
			if d := int(img.Pix[i+2]) - int(img.Pix[i]); d > best {
				best = d
			}
		}
	}
	return best
}

// TestCollabMenuCheckedIsBlue 钉住选中项是蓝字（原版 [aria-checked=true]）：
// 主UP 是选中项时首条目的最蓝像素远蓝于未选中的「乙」。
// **反证**：把 collabItemStyleFor 里的 `checked` 分支去掉 → 首条目也变灰，
// 两者 B-R 接近，测试红。
func TestCollabMenuCheckedIsBlue(t *testing.T) {
	a, tt := collabProbe(t, 12)
	a.CollabMid, a.CollabFor = 100, "BV1collab" // 主UP 即当前显示的 UP 主
	tt.Frame()
	_, first := openCollab(t, tt)
	blue := maxBlueMinusRed(tt.Image(),
		int(first.X), int(first.Y), int(first.X+first.W), int(first.Y+first.H))

	r, ok := tt.Find("乙")
	if !ok {
		t.Fatal("面板里看不到「乙」")
	}
	gray := maxBlueMinusRed(tt.Image(), int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
	t.Logf("选中项 B-R=%d，未选中项 B-R=%d", blue, gray)
	if blue < 100 {
		t.Fatalf("选中项没有蓝字：B-R=%d", blue)
	}
	if gray > 80 {
		t.Fatalf("未选中项不应是蓝字：B-R=%d", gray)
	}
	if blue <= gray {
		t.Fatalf("选中项应比未选中更蓝：选中=%d 未选中=%d", blue, gray)
	}
}

// TestCollabMenuCheckedDefaultsToOwner 钉住「没选过合作者时默认给主 UP 打勾」：
// 原版 selectedKeys = displayedOwner.mid，而 displayedOwner 在 selectedOwner
// 为空时退回 creators[0]（主 UP）—— 所以下拉一打开，主 UP 那一条就是选中项。
// mygo 此前只在 CollabMid 命中时才算选中，默认一个勾都没有。
// **反证**：把 collabChecker 的 Staff[0] 回退去掉 → 首条目不再有蓝字，测试红。
func TestCollabMenuCheckedDefaultsToOwner(t *testing.T) {
	a, tt := collabProbe(t, 12)
	// 没选过：CollabMid/CollabFor 都是零值。
	checked := a.collabChecker()
	if !checked(a.Info.Staff[0]) {
		t.Fatal("默认应给主 UP（首条目）打勾")
	}
	if checked(a.Info.Staff[1]) || checked(a.Info.Staff[2]) {
		t.Fatal("默认只该给主 UP 打勾，其它参与者不该是选中项")
	}
	// 无头实测：首条目应显蓝（选中），「乙」仍灰。
	_, first := openCollab(t, tt)
	blue := maxBlueMinusRed(tt.Image(),
		int(first.X), int(first.Y), int(first.X+first.W), int(first.Y+first.H))
	r, ok := tt.Find("乙")
	if !ok {
		t.Fatal("面板里看不到「乙」")
	}
	gray := maxBlueMinusRed(tt.Image(), int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
	t.Logf("默认主 UP B-R=%d，未选中「乙」B-R=%d", blue, gray)
	if blue < 100 {
		t.Fatalf("默认主 UP 应是蓝字（选中）：B-R=%d", blue)
	}
	if blue <= gray {
		t.Fatalf("主 UP 应比未选中项更蓝：选中=%d 未选中=%d", blue, gray)
	}
}

// TestCollabCheckerFallsBackAfterVideoChange 钉住「换视频后选中项失效 → 勾回主 UP」：
// CollabFor 不再等于当前 bvid 时，原版 displayedOwner 退回 creators[0]，
// 选中标记也应跟着回主 UP。
func TestCollabCheckerFallsBackAfterVideoChange(t *testing.T) {
	a, _ := collabProbe(t, 12)
	// 选中合作者「甲」，但属于另一条视频 → 失效。
	a.CollabMid, a.CollabFor = 200, "BV1other"
	checked := a.collabChecker()
	if !checked(a.Info.Staff[0]) {
		t.Fatal("换视频后应把勾退回主 UP")
	}
	if checked(a.Info.Staff[1]) {
		t.Fatal("换视频后旧选中项不该还是选中态")
	}
	// 选中项仍在当前视频里 → 就是它。
	a.CollabFor = "BV1collab"
	checked = a.collabChecker()
	if !checked(a.Info.Staff[1]) || checked(a.Info.Staff[0]) {
		t.Fatal("选中项属于当前视频时应勾在它身上")
	}
}

// TestCollabMenuHover 无头实测条目悬停底色：指针挪到「乙」右半（避开文字）后，
// 白玻璃底铺上来，像素应比静息更亮。用烟熏（晚间）主题做底。
// **反证**：把悬停分支去掉（只留 transparent），两次采样相等。
func TestCollabMenuHover(t *testing.T) {
	_, tt := collabProbe(t, 22)
	openCollab(t, tt)
	r, ok := tt.Find("乙")
	if !ok {
		t.Fatal("面板里看不到「乙」")
	}
	sx, sy := int(r.X+r.W-6), int(r.Y+r.H/2)
	rest := grayAt(tt.Image(), sx, sy)
	tt.Move(r.X+r.W-6, r.Y+r.H/2)
	tt.Frame()
	hover := grayAt(tt.Image(), sx, sy)
	t.Logf("条目右侧像素 静息=%d 悬停=%d", rest, hover)
	if hover <= rest {
		t.Fatalf("悬停没有铺上白玻璃底：静息=%d 悬停=%d", rest, hover)
	}
}

// TestCollabTriggerTurnsBlueWhenOpen 无头实测选择键展开态：面板打开后图标转蓝
// （原版 `.video-owner-picker-button[aria-expanded=true]` 的 `color: --studio-blue`）。
// 原生版此前用的是框架系统菜单，拿不到开合状态，图标一直是灰的。
// **反证**：把 collabTriggerStyleFor 的 `if expanded { s.Ink = t.Blue }` 去掉 →
// 展开态与静息态一样灰，测试红。
func TestCollabTriggerTurnsBlueWhenOpen(t *testing.T) {
	_, tt := collabProbe(t, 12)
	btn, ok := tt.Find("选择合作 UP 主")
	if !ok {
		t.Fatal("找不到选择键")
	}
	closed := maxBlueMinusRed(tt.Image(), int(btn.X), int(btn.Y), int(btn.X+btn.W), int(btn.Y+btn.H))
	if err := tt.Click("选择合作 UP 主"); err != nil {
		t.Fatalf("点不到选择键：%v", err)
	}
	tt.Frame()
	open := maxBlueMinusRed(tt.Image(), int(btn.X), int(btn.Y), int(btn.X+btn.W), int(btn.Y+btn.H))
	t.Logf("选择键图标 B-R 静息=%d 展开=%d", closed, open)
	if closed > 80 {
		t.Fatalf("静息时图标不该是蓝的：B-R=%d", closed)
	}
	if open < 100 {
		t.Fatalf("展开后图标应转蓝：B-R=%d", open)
	}
}

// TestCollabMenuClosesOnOutsidePress 钉住「点面板外关闭」（PopoverBase 的
// PressedOutside，对应原版 document pointerdown 判定）。
func TestCollabMenuClosesOnOutsidePress(t *testing.T) {
	a, tt := collabProbe(t, 12)
	openCollab(t, tt)
	tt.ClickAt(20, 400)
	tt.Frame()
	if a.CollabOpen {
		t.Fatal("点面板外后面板没关")
	}
	if _, ok := tt.Find("联合投稿"); ok {
		t.Fatal("点面板外后条目还在屏上")
	}
}

// TestCollabMenuClosesOnEscape 钉住 Esc 关闭（PopoverBase 的 OverlayShortcut）。
func TestCollabMenuClosesOnEscape(t *testing.T) {
	a, tt := collabProbe(t, 12)
	openCollab(t, tt)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.CollabOpen {
		t.Fatal("按 Esc 后面板没关")
	}
}

// TestCollabMenuClosesOnVideoChange 钉住「换视频后展开态自动失效」：原版
// videoInfo 的 useEffect 在 bvid 变化时 setIsOwnerMenuOpen(false)。mygo 没有
// bvid 变化钩子，改为把展开态记在打开它的那条 bvid 上（CollabOpenFor），
// 画之前归一化。**反证**：去掉 collabPicker 里的归一化 → 换 bvid 后面板仍开着、
// a.CollabOpen 仍为真，测试红。
func TestCollabMenuClosesOnVideoChange(t *testing.T) {
	a, tt := collabProbe(t, 12)
	openCollab(t, tt)
	if !a.collabMenuOpen() {
		t.Fatal("打开后应属于当前视频")
	}

	// 换成另一条合作视频（staff 仍 > 1，选择键照画）。
	a.Info.Bvid = "BV2collab"
	tt.Frame()
	if a.CollabOpen {
		t.Fatal("换视频后 a.CollabOpen 应收起")
	}
	if a.collabMenuOpen() {
		t.Fatal("换视频后展开态不该再有效")
	}
	if _, ok := tt.Find("联合投稿"); ok {
		t.Fatal("换视频后面板还画在屏上")
	}

	// 在新视频上重新打开：展开态记在新 bvid 上。
	if err := tt.Click("选择合作 UP 主"); err != nil {
		t.Fatalf("点不到选择键：%v", err)
	}
	tt.Frame()
	if !a.collabMenuOpen() || a.CollabOpenFor != "BV2collab" {
		t.Fatalf("新视频上重开后展开态不对：open=%v for=%q", a.collabMenuOpen(), a.CollabOpenFor)
	}
}
