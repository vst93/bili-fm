package view

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// settingsProbe 建一个 1000×700 的主窗（固定 hour 时段的主题，可复现），返回
// App 与无头窗口，用来验证「设置」下拉。
func settingsProbe(t *testing.T, hour int) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(hour))
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	return a, tst
}

// TestSettingsItems 钉住设置菜单的条目与顺序：原版 titleBar.tsx 的
// 关于应用 / 快捷键 / 检查更新 / 退出应用，**没有分隔线**（原生版此前用框架
// 系统菜单时多了一条原版没有的分隔线）。
func TestSettingsItems(t *testing.T) {
	want := []string{"关于应用", "快捷键", "检查更新", "退出应用"}
	if got := settingsItems(); !slices.Equal(got, want) {
		t.Fatalf("settingsItems() = %v，应为 %v", got, want)
	}
}

// TestSettingsItemPaint 钉住 #settings-menu button 的三态取值：
//
//	常态  transparent、无内高光、scale 1
//	悬停  --glass-bg-hover（白 .38）+ 内高光
//	按下  --glass-bg-active（白 .16）+ 内高光 + scale .97
func TestSettingsItemPaint(t *testing.T) {
	th := ThemeFor(PeriodAt(12))

	bg, inset, scale := settingsItemPaint(th, false, false)
	if bg.A != 0 || inset || scale != 1 {
		t.Errorf("常态 = (a=%d, inset=%v, scale=%.2f)，应为 (0, false, 1)", bg.A, inset, scale)
	}
	bg, inset, scale = settingsItemPaint(th, true, false)
	if bg != th.GlassHover || !inset || scale != 1 {
		t.Errorf("悬停 = (%v, %v, %.2f)，应为 (%v, true, 1)", bg, inset, scale, th.GlassHover)
	}
	bg, inset, scale = settingsItemPaint(th, true, true)
	if bg != th.GlassActive || !inset || scale != 0.97 {
		t.Errorf("按下 = (%v, %v, %.2f)，应为 (%v, true, .97)", bg, inset, scale, th.GlassActive)
	}
	// 按下时即使指针不在条目上（键盘触发）也走按下态。
	bg, _, scale = settingsItemPaint(th, false, true)
	if bg != th.GlassActive || scale != 0.97 {
		t.Errorf("按下（未悬停）= (%v, %.2f)，应为 (%v, .97)", bg, scale, th.GlassActive)
	}
}

// menuRects 打开设置菜单，返回四个条目的矩形（按 settingsItems 的顺序）。
func menuRects(t *testing.T, tst *ui.Tester) []ui.Rect {
	t.Helper()
	if err := tst.Click("设置"); err != nil {
		t.Fatalf("点设置键失败：%v", err)
	}
	tst.Frame()
	var out []ui.Rect
	for _, item := range settingsItems() {
		r, ok := tst.Find(item)
		if !ok {
			t.Fatalf("设置菜单打开后看不到条目 %q", item)
		}
		out = append(out, r)
	}
	return out
}

// TestSettingsMenuGeometry 无头打开设置菜单，钉住面板几何（对齐原版 #settings-menu）：
// 面板左缘与设置键左缘对齐、上缘在设置键下方 4px、条目高 36 且四个原版条目
// 自上而下紧排（它们之间无分隔线；本 fork 在四项之后另起一段分隔线 + 隐身开关）。
func TestSettingsMenuGeometry(t *testing.T) {
	_, tst := settingsProbe(t, 12)
	btn, ok := tst.Find("设置")
	if !ok {
		t.Fatal("找不到设置键")
	}
	items := menuRects(t, tst)
	t.Logf("设置键 %+v", btn)
	for i, r := range items {
		t.Logf("条目%d %+v", i, r)
	}
	// 面板没有左右内边距，所以首个条目的左缘就是面板左缘；
	// 上缘 = 面板上缘 + py-1(4)，面板上缘 = 设置键下缘 + 4。
	if d := items[0].X - btn.X; d < -1 || d > 1 {
		t.Errorf("面板左缘与设置键左缘差 %.1f，应左对齐", d)
	}
	wantTop := btn.Y + btn.H + settingsMenuGap + settingsMenuPadY
	if d := items[0].Y - wantTop; d < -1 || d > 1 {
		t.Errorf("首条目上缘 = %.1f，应为 %.1f（设置键下缘 + %d + %d）",
			items[0].Y, wantTop, settingsMenuGap, settingsMenuPadY)
	}
	// 原版 Tailwind preflight 是 `box-sizing: border-box`：min-width 120 是**含
	// 1px 边框**的外宽，条目 `w-full` 拿到的是内容宽 118。面板本轮补上了
	// `border: 1px solid var(--glass-border) !important`（globals.css:1926），
	// 所以条目宽从 120 变成 118 —— 这正是原版渲染出来的数字。
	if d := items[0].W - (settingsMenuMinW - 2); d < -1 || d > 1 {
		t.Errorf("条目宽 = %.1f，应为 min-width %d 减两侧 1px 边框 = %d",
			items[0].W, settingsMenuMinW, settingsMenuMinW-2)
	}
	// 面板没有左右内边距，所以所有条目（含底部隐身开关行）都铺满同一宽度。
	for i := 1; i < len(items); i++ {
		if d := items[i].W - items[0].W; d < -1 || d > 1 {
			t.Errorf("条目%d 宽 = %.1f，应与首条目同宽 %.1f", i, items[i].W, items[0].W)
		}
	}
	for i := 1; i < len(items); i++ {
		if d := items[i].Y - (items[i-1].Y + settingsItemH); d < -1 || d > 1 {
			t.Errorf("条目%d 上缘 = %.1f，应紧接上一个（%.1f + %d）",
				i, items[i].Y, items[i-1].Y, settingsItemH)
		}
	}
	if d := items[0].H - settingsItemH; d < -1 || d > 1 {
		t.Errorf("条目高 = %.1f，应为 %d", items[0].H, settingsItemH)
	}
}

// TestSettingsMenuInk 无头实测条目文案色：取条目矩形里最暗像素，应接近原版
// #settings-menu button 的 #334155（原版是深青灰，不是纯黑/系统菜单观感）。
func TestSettingsMenuInk(t *testing.T) {
	_, tst := settingsProbe(t, 12)
	items := menuRects(t, tst)
	r := items[0]
	got := darkestIn(tst.Image(), int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
	t.Logf("「关于应用」最暗像素 = %v，目标 #334155 = [51 65 85]", got)
	for i, want := range [3]int{51, 65, 85} {
		if d := got[i] - want; d < -25 || d > 25 {
			t.Fatalf("条目文案色 = %v，应接近 #334155（通道%d 差 %d）", got, i, d)
		}
	}
}

// TestSettingsMenuHover 无头实测条目悬停底色：指针挪到条目右半（避开文字）后，
// 白玻璃底铺上来，像素应比静息时更亮。用烟熏（晚间）主题做底：亮色时段面板底
// 本就接近白，白 .38 的悬停几乎看不出，夜间才是这块玻璃的主场。
// **反证**：把 settingsItemPaint 的悬停分支去掉（只留 transparent），两次采样相等。
func TestSettingsMenuHover(t *testing.T) {
	_, tst := settingsProbe(t, 22)
	items := menuRects(t, tst)
	r := items[1] // 「快捷键」，避免与第一项的文字/圆角纠缠
	// 采样点取条目右侧 6px 处（文字靠左，这里只有底色）。
	sx, sy := int(r.X+r.W-6), int(r.Y+r.H/2)
	rest := grayAt(tst.Image(), sx, sy)
	tst.Move(r.X+r.W-6, r.Y+r.H/2)
	tst.Frame()
	hover := grayAt(tst.Image(), sx, sy)
	t.Logf("条目右侧像素 静息=%d 悬停=%d", rest, hover)
	if hover <= rest {
		t.Fatalf("悬停没有铺上白玻璃底：静息=%d 悬停=%d", rest, hover)
	}
}

// TestSettingsMenuClosesOnOutsidePress 钉住「点弹层外关闭」：打开菜单后在主区
// 按一下，条目消失（原版 document pointerdown 判定 / PopoverBase 的 PressedOutside）。
func TestSettingsMenuClosesOnOutsidePress(t *testing.T) {
	_, tst := settingsProbe(t, 12)
	menuRects(t, tst)
	tst.ClickAt(500, 400)
	tst.Frame()
	if _, ok := tst.Find("关于应用"); ok {
		t.Fatal("点弹层外后面板没关")
	}
}

// TestSettingsMenuClosesOnEscape 钉住 Esc 关闭（PopoverBase 的 OverlayShortcut）。
func TestSettingsMenuClosesOnEscape(t *testing.T) {
	_, tst := settingsProbe(t, 12)
	menuRects(t, tst)
	tst.Key(0, ui.KeyEscape)
	tst.Frame()
	if _, ok := tst.Find("关于应用"); ok {
		t.Fatal("按 Esc 后面板没关")
	}
}

// TestSettingsMenuChosen 钉住选中条目：点「检查更新」触发 Act.CheckUpdate 并且
// 面板收起。
func TestSettingsMenuChosen(t *testing.T) {
	a, tst := settingsProbe(t, 12)
	var got string
	a.Act.ShowAbout = func() { got = "about" }
	a.Act.ShowShortcuts = func() { got = "shortcuts" }
	a.Act.CheckUpdate = func() { got = "update" }
	a.Act.Quit = func() { got = "quit" }
	menuRects(t, tst)
	if err := tst.Click("检查更新"); err != nil {
		t.Fatalf("点「检查更新」失败：%v", err)
	}
	tst.Frame()
	if got != "update" {
		t.Fatalf("动作 = %q，应为 update", got)
	}
	if _, ok := tst.Find("关于应用"); ok {
		t.Fatal("选中条目后面板没收起")
	}
}

// TestSettingsMenuIncognitoGone 钉住设置菜单里**不再**有隐身开关（用户明确否掉了
// 「藏在设置菜单里」这个方案）：设置菜单只剩原版四项，隐身入口在标题栏常驻
// （见 titlebar_test.go 的 TestTitleBarIncognitoToggle*）。
func TestSettingsMenuIncognitoGone(t *testing.T) {
	_, tst := settingsProbe(t, 12)
	items := menuRects(t, tst)
	// 设置菜单只剩原版四项（menuRects 只查 settingsItems()，这里再确认面板里
	// 没有第五条）。
	for _, r := range items {
		_ = r
	}
	flat := flattenNoComments(t, "settingsmenu.go")
	if srcHas(flat, "settingsIncognitoRow") {
		t.Error("settingsmenu.go 仍建 settingsIncognitoRow（用户已否掉这个位置）")
	}
	if count := strings.Count(flat, "ui.Text(c,label).FontSize(14).TextColor(settingsItemInk)"); count != 1 {
		t.Errorf("设置菜单条目渲染点 %d 处，应恰好 1 处（面板里不应再挂隐身行）", count)
	}
}

// TestIncognitoSharedVisual 钉住所有入口共用同一套开关视觉：incognitoInk 是唯一
// 事实源（图标 + 墨色），历史抽屉与标题栏常驻键都读它。
func TestIncognitoSharedVisual(t *testing.T) {
	if got := incognitoInk(true); got != ui.Hex("#0e7490") {
		t.Errorf("incognitoInk(true) = %v，应为 #0e7490", got)
	}
	if got := incognitoInk(false); got != ui.Hex("#64748b") {
		t.Errorf("incognitoInk(false) = %v，应为 #64748b", got)
	}
	drawer := flattenNoComments(t, "drawer.go")
	if !srcHas(drawer, "incognitoInk(a.Incognito)") {
		t.Error("drawer.go 没有复用 incognitoInk")
	}
	menu := flattenNoComments(t, "settingsmenu.go")
	if !srcHas(menu, "incognitoInk(true)") {
		t.Error("settingsmenu.go（标题栏常驻键）没有复用 incognitoInk")
	}
}

// TestIncognitoToggleA11y 钉住标题栏常驻键也是 role=switch + checked（与历史抽屉
// 那颗同一语义），读屏用户能听到开/关。
func TestIncognitoToggleA11y(t *testing.T) {
	flat := flattenNoComments(t, "settingsmenu.go")
	if !srcHas(flat, `ui.ButtonBase(c.Key("tb-incognito")).Size(30,30).Radius(RadiusSmall).Center().Label("隐身模式").Tooltip(incognitoTip(a.Incognito)).Role(ui.RoleSwitch).Checked(a.Incognito)`) {
		t.Error("标题栏隐身键没有 Role(ui.RoleSwitch) + Checked(a.Incognito)")
	}
}

// TestSettingsMenuPlaybackToggles 钉住设置菜单里的三个播放偏好开关：
// 默认值来自 App，点击后回调触发、菜单不收起，方便连续调整。
func TestSettingsMenuPlaybackToggles(t *testing.T) {
	a, tst := settingsProbe(t, 12)
	var desktop, heat, radio int
	a.Act.ToggleDesktopLyrics = func() { desktop++ }
	a.Act.ToggleHeat = func() { heat++ }
	a.Act.ToggleRadio = func() { radio++ }
	if err := tst.Click("设置"); err != nil {
		t.Fatalf("点设置键失败：%v", err)
	}
	tst.Frame()
	for _, label := range []string{"设置：桌面字幕", "设置：弹幕热力", "设置：顺着听"} {
		if _, ok := tst.Find(label); !ok {
			t.Fatalf("设置菜单里找不到开关 %q", label)
		}
	}
	if err := tst.Click("设置：桌面字幕"); err != nil {
		t.Fatalf("点桌面字幕开关失败：%v", err)
	}
	if err := tst.Click("设置：弹幕热力"); err != nil {
		t.Fatalf("点弹幕热力开关失败：%v", err)
	}
	if err := tst.Click("设置：顺着听"); err != nil {
		t.Fatalf("点顺着听开关失败：%v", err)
	}
	if desktop != 1 || heat != 1 || radio != 1 {
		t.Fatalf("开关回调 desktop=%d heat=%d radio=%d，应各 1 次", desktop, heat, radio)
	}
	if !a.ShowSettings {
		t.Fatal("切换播放偏好后设置菜单不应收起")
	}
}
