package view

import (
	"image"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（抽屉表头 tab 材质）的测试。原版
// `[data-slot="wrapper"] header [data-slot="tab"]`：
//
//	常态  1px --glass-border（白 .55）边、--liquid-bg（白 .22）底、box-shadow none
//	悬停  1px --glass-border-bright（白 .72）边、--liquid-bg-hover（白 .30）底、
//	      box-shadow --liquid-shadow（0 8 24 墨 .07 + 0 1 4 墨 .03）
//	选中  1px --glass-border-bright 边、白 .52 底、
//	      box-shadow --liquid-shadow-lifted（0 16 38 墨 .11 + 0 2 8 墨 .04）
//	::before  --glass-sheen-static 洗光
//	::after   1px --liquid-rim 聚光描边（常态 opacity .65、选中 1）
//	          + --liquid-chromatic / inset 0 1px 1px .5px 白 .9 / inset 0 -1px 1px 墨 .06
//	:active   transform: none（tab 按下不缩放）
//
// 此前 tab 借的是播放栏的 buttonSurface —— 多出一层原版没有的常驻投影、少了
// 两层液态玻璃、顶部内高光暗一档，且带着 pressPose 的悬停上浮 / 按下缩放。

// freezeDrawerTabFades 把本轮补上的色值 / 阴影 / 不透明度过渡换成瞬时写入，供
// 像素对照测试消除动画相位干扰（同 freezePartSearchFades 的手法）。过渡本身的
// 正确性由 TestDrawerTabFadeWired / TestDrawerTabShadowInflightWired 单独钉住。
func freezeDrawerTabFades(t *testing.T) {
	t.Helper()
	fade, rim, shadow := drawerTabFade, drawerTabRimOpacity, drawerTabShadow
	drawerTabFade = func(b ui.Element, key string, target ui.Color) ui.Color { return target }
	drawerTabRimOpacity = func(b ui.Element, target float32) float32 { return target }
	drawerTabShadow = func(b ui.Element, key string, s shadowSpec) shadowSpec { return s }
	t.Cleanup(func() { drawerTabFade, drawerTabRimOpacity, drawerTabShadow = fade, rim, shadow })
}

// drawerTabProbe 起一个打开了搜索抽屉的无头窗口：表头是「搜索」标题 +
// 综合 / 最多播放 / 最新发布 三个 tab，默认选中「综合」。
func drawerTabProbe(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = DrawerSearch
	a.Query = "关键词"
	return a, settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 680))
}

// TestDrawerTabPaintTokens 逐值钉住三态。
func TestDrawerTabPaintTokens(t *testing.T) {
	white := ui.Hex("#ffffff")
	base := drawerTabPaintFor(false, false)
	if base.BG != white.Alpha(0.22) {
		t.Errorf("常态底 = %v，应为 --liquid-bg 白 .22", base.BG)
	}
	if base.Border != white.Alpha(0.55) {
		t.Errorf("常态边 = %v，应为 --glass-border 白 .55", base.Border)
	}
	if base.Rim != 0.65 {
		t.Errorf("常态 ::after opacity = %v，应为 .65", base.Rim)
	}
	if base.LiftLG != (shadowSpec{}) || base.LiftSM != (shadowSpec{}) {
		t.Errorf("常态 box-shadow 应为 none（两层零值），得到 %+v / %+v", base.LiftLG, base.LiftSM)
	}

	hover := drawerTabPaintFor(false, true)
	if hover.BG != white.Alpha(0.30) {
		t.Errorf("悬停底 = %v，应为 --liquid-bg-hover 白 .30", hover.BG)
	}
	if hover.Border != white.Alpha(0.72) {
		t.Errorf("悬停边 = %v，应为 --glass-border-bright 白 .72", hover.Border)
	}
	if hover.Rim != 0.65 {
		t.Errorf("悬停未选中不覆写 ::after opacity，应为 .65，得到 %v", hover.Rim)
	}
	if hover.LiftLG != (shadowSpec{0, 8, 24, 0, shadowInk.Alpha(0.07)}) {
		t.Errorf("悬停抬起第一层 = %+v，应为 --liquid-shadow 0 8 24 墨 .07", hover.LiftLG)
	}
	if hover.LiftSM != (shadowSpec{0, 1, 4, 0, shadowInk.Alpha(0.03)}) {
		t.Errorf("悬停抬起第二层 = %+v，应为 --liquid-shadow 0 1 4 墨 .03", hover.LiftSM)
	}

	act := drawerTabPaintFor(true, false)
	if act.BG != white.Alpha(0.52) {
		t.Errorf("选中底 = %v，应为白 .52", act.BG)
	}
	if act.Border != white.Alpha(0.72) {
		t.Errorf("选中边 = %v，应为 --glass-border-bright 白 .72", act.Border)
	}
	if act.Rim != 1 {
		t.Errorf("选中 ::after opacity = %v，应为 1", act.Rim)
	}
	if act.LiftLG != (shadowSpec{0, 16, 38, 0, shadowInk.Alpha(0.11)}) {
		t.Errorf("选中抬起第一层 = %+v，应为 --liquid-shadow-lifted 0 16 38 墨 .11", act.LiftLG)
	}
	if act.LiftSM != (shadowSpec{0, 2, 8, 0, shadowInk.Alpha(0.04)}) {
		t.Errorf("选中抬起第二层 = %+v，应为 --liquid-shadow-lifted 0 2 8 墨 .04", act.LiftSM)
	}

	// 选中态特异度更高：同时悬停仍按选中取值。
	if both := drawerTabPaintFor(true, true); both != act {
		t.Errorf("选中 + 悬停 = %+v，应同选中态 %+v", both, act)
	}
}

// TestDrawerTabInkTokens 钉住文字色三态。
func TestDrawerTabInkTokens(t *testing.T) {
	if got := drawerTabInk(false, false); got != ui.Hex("#475569") {
		t.Errorf("常态墨色 = %v，应为 #475569", got)
	}
	if got := drawerTabInk(false, true); got != ui.Hex("#334155") {
		t.Errorf("悬停墨色 = %v，应为 #334155", got)
	}
	if got := drawerTabInk(true, false); got != ui.Hex("#0369a1") {
		t.Errorf("选中墨色 = %v，应为 #0369a1", got)
	}
	if got := drawerTabInk(true, true); got != ui.Hex("#0369a1") {
		t.Errorf("选中 + 悬停墨色 = %v，应为 #0369a1", got)
	}
}

// TestDrawerTabAnimTokens 钉住过渡参数：原版 tab 的 border-color / box-shadow /
// color / ::before background-color / ::after opacity 都是 0.2s ease。
func TestDrawerTabAnimTokens(t *testing.T) {
	if drawerTabAnim.Duration != 200*time.Millisecond {
		t.Errorf("时长 = %v, want 200ms（原版 0.2s）", drawerTabAnim.Duration)
	}
	if drawerTabAnim.Spring != (ui.SpringOptions{}) {
		t.Errorf("应为曲线动画，不是弹簧：%+v", drawerTabAnim.Spring)
	}
	if drawerTabAnim.Ease == nil {
		t.Fatal("缺少缓动曲线")
	}
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
		if d := math.Abs(float64(drawerTabAnim.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("缓动在 %.2f 处 = %v, want CSS ease %v", x, drawerTabAnim.Ease(x), want(x))
		}
	}
}

// TestDrawerTabFadeWired 实测 tab 的底 / 边色值真的走 0.2s 过渡（换目标那一帧
// 曲线 t=0 ⇒ 返回上一帧的值）。
func TestDrawerTabFadeWired(t *testing.T) {
	colA, colB := ui.Hex("#ffffff").Alpha(0.22), ui.Hex("#ffffff").Alpha(0.30)
	cur := colA
	var cols []ui.Color
	var rims []float32
	rimTarget := float32(0.65)
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		cols = append(cols, drawerTabFade(b, "probe", cur))
		rims = append(rims, drawerTabRimOpacity(b, rimTarget))
	}, 60, 60)
	tt.Frame()
	if len(cols) == 0 || len(rims) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if cols[0] != colA {
		t.Fatalf("首帧应直接取目标 %v，得到 %v", colA, cols[0])
	}
	if rims[0] != 0.65 {
		t.Fatalf("首帧不透明度应直接取目标 .65，得到 %v", rims[0])
	}

	cols, rims = nil, nil
	cur, rimTarget = colB, 1
	tt.Frame()
	if cols[0] == colB {
		t.Errorf("换目标的当帧就跳到新色 %v —— 没走 200ms 过渡", colB)
	}
	if cols[0] != colA {
		t.Errorf("色值过渡应从上一帧 %v 起步，得到 %v", colA, cols[0])
	}
	if rims[0] == 1 {
		t.Error("::after opacity 换目标的当帧就跳到 1 —— 没走 200ms 过渡")
	}
	if rims[0] != 0.65 {
		t.Errorf("::after opacity 应从上一帧 .65 起步，得到 %v", rims[0])
	}
}

// TestDrawerTabShadowGrowsFromNone 实测常态（box-shadow: none）→ 抬起时几何真的
// 从零长出来（同 partSearch 的同类测试）。
func TestDrawerTabShadowGrowsFromNone(t *testing.T) {
	cur := shadowSpec{}
	var got []shadowSpec
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		got = append(got, drawerTabShadow(b, "probe", cur))
	}, 60, 60)
	tt.Frame() // 首帧：零
	cur = shadowSpec{0, 8, 24, 0, shadowInk.Alpha(0.07)}
	got = nil
	tt.Frame()
	if len(got) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if got[0] != (shadowSpec{}) {
		t.Errorf("常态 → 抬起应仍为零几何起步，得到 %+v", got[0])
	}
}

// TestDrawerTabSurfaceWired 无头反证：把 drawerTabSurface 换成 no-op，选中 tab
// 的矩形内应有一片像素随之变化（材质真的接在 tab 上）。
func TestDrawerTabSurfaceWired(t *testing.T) {
	freezeDrawerTabFades(t)
	orig := drawerTabSurface
	defer func() { drawerTabSurface = orig }()

	render := func(stub bool) (*image.RGBA, ui.Rect) {
		if stub {
			drawerTabSurface = func(b ui.Element, active, hovered bool) ui.Element { return b }
		} else {
			drawerTabSurface = orig
		}
		_, tt := drawerTabProbe(t)
		r, ok := tt.Find("综合")
		if !ok {
			t.Fatal("搜索抽屉表头里找不到「综合」tab")
		}
		return tt.Image(), r
	}

	on, rect := render(false)
	off, _ := render(true)

	changed := 0
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			if rgbAt(on, x, y) != rgbAt(off, x, y) {
				changed++
			}
		}
	}
	t.Logf("关掉 tab 材质后「综合」内变化 %d 像素", changed)
	if changed < 100 {
		t.Errorf("tab 材质没接上：只变了 %d 个像素", changed)
	}
}

// TestDrawerTabGlassWired 反证 tab 真的画了 ::before 洗光 + ::after 聚光描边：
// 把 drawerTabGlass 换成 no-op 后 tab 矩形内像素跟着变。
func TestDrawerTabGlassWired(t *testing.T) {
	freezeDrawerTabFades(t)
	orig := drawerTabGlass
	defer func() { drawerTabGlass = orig }()

	render := func(stub bool) (*image.RGBA, ui.Rect) {
		if stub {
			drawerTabGlass = func(b ui.Element, rim float32) ui.Element { return b }
		} else {
			drawerTabGlass = orig
		}
		_, tt := drawerTabProbe(t)
		r, ok := tt.Find("综合")
		if !ok {
			t.Fatal("搜索抽屉表头里找不到「综合」tab")
		}
		return tt.Image(), r
	}

	on, rect := render(false)
	off, _ := render(true)

	changed := 0
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			if rgbAt(on, x, y) != rgbAt(off, x, y) {
				changed++
			}
		}
	}
	t.Logf("关掉 tab 玻璃层后「综合」内变化 %d 像素", changed)
	if changed < 100 {
		t.Errorf("tab 的 ::before/::after 没画上：只变了 %d 个像素", changed)
	}
}

// TestDrawerTabNoPressScale tab 按下**不缩放**：原版
// `header [data-slot="tab"]:active { transform: none !important }`。此前借的
// pressPose 会缩到 .94 并带悬停上浮。
func TestDrawerTabNoPressScale(t *testing.T) {
	freezeDrawerTabFades(t)
	_, tt := drawerTabProbe(t)
	tt.Move(-1, -1)
	tt.Frame()
	rest, ok := tt.Find("综合")
	if !ok {
		t.Fatal("找不到「综合」tab")
	}
	cx, cy := rest.X+rest.W/2, rest.Y+rest.H/2
	tt.Press(cx, cy)
	pressed, ok := tt.Find("综合")
	if !ok {
		t.Fatal("按下后找不到「综合」tab")
	}
	if d := pressed.W - rest.W; d > 0.5 || d < -0.5 {
		t.Errorf("按下应保持原尺寸（原版 :active transform: none）：宽 %v → %v", rest.W, pressed.W)
	}
	tt.Release(cx, cy)
}

// TestDrawerTabNoHoverLift tab 悬停**不上浮**：原版这条规则没有 `:hover` 位移。
func TestDrawerTabNoHoverLift(t *testing.T) {
	freezeDrawerTabFades(t)
	_, tt := drawerTabProbe(t)
	tt.Move(-1, -1)
	tt.Frame()
	rest, ok := tt.Find("综合")
	if !ok {
		t.Fatal("找不到「综合」tab")
	}
	tt.Move(rest.X+rest.W/2, rest.Y+rest.H/2)
	tt.Frame()
	hover, ok := tt.Find("综合")
	if !ok {
		t.Fatal("悬停后找不到「综合」tab")
	}
	if hover.Y != rest.Y {
		t.Errorf("悬停把 tab 上移了：Y %v → %v（原版没有 :hover 位移）", rest.Y, hover.Y)
	}
}

// TestDrawerTabSurfaceUsedForAllTabs 把 drawerTabSurface 换成记录调用的桩，
// 搜索抽屉表头的三个 tab 应各调用一次。
func TestDrawerTabSurfaceUsedForAllTabs(t *testing.T) {
	orig := drawerTabSurface
	defer func() { drawerTabSurface = orig }()
	calls := 0
	drawerTabSurface = func(b ui.Element, active, hovered bool) ui.Element {
		calls++
		return orig(b, active, hovered)
	}
	drawerTabProbe(t)
	if calls < 3 {
		t.Errorf("搜索抽屉表头只调用了 %d 次 tab 材质（应有综合 / 最多播放 / 最新发布三个）", calls)
	}
}

// TestFolderTabUsesDrawerTabSurface 收藏夹 tab 也走同一套材质（原版它们就是
// 同一个 `[data-slot="tab"]` 规则，只是放在 .collect-tabs-scroll 里）。
func TestFolderTabUsesDrawerTabSurface(t *testing.T) {
	orig := drawerTabSurface
	defer func() { drawerTabSurface = orig }()
	calls := 0
	drawerTabSurface = func(b ui.Element, active, hovered bool) ui.Element {
		calls++
		return orig(b, active, hovered)
	}
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = "favorite"
	a.Folders = []Folder{{ID: 1, Title: "默认收藏夹", Count: 3}}
	settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 680))
	if calls < 1 {
		t.Errorf("收藏夹 tab 没走 drawerTabSurface（调用 %d 次）", calls)
	}
}

// TestDrawerTabHoverFillWired 无头实测：悬停把 tab 底从 --liquid-bg（白 .22）
// 换成 --liquid-bg-hover（白 .30）、边提到 --glass-border-bright 并抬起一档，
// tab 区域内的像素应随之变化。此前借 buttonSurface 时悬停只换底色，且没有抬起。
//
// 必须悬停**未选中**的 tab（「最多播放」）：选中态材质由 active 分支固定，
// 悬停不再改底/边/投影（原版选中规则在 CSS 里更靠后，压过 :hover）。
func TestDrawerTabHoverFillWired(t *testing.T) {
	freezeDrawerTabFades(t)

	render := func(hover bool) (*image.RGBA, ui.Rect) {
		_, tt := drawerTabProbe(t)
		r, ok := tt.Find("最多播放")
		if !ok {
			t.Fatal("找不到「最多播放」tab")
		}
		if hover {
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Frame()
			r, ok = tt.Find("最多播放")
			if !ok {
				t.Fatal("悬停后找不到「最多播放」tab")
			}
		}
		return tt.Image(), r
	}

	rest, rect := render(false)
	hover, _ := render(true)
	changed := 0
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			if rgbAt(rest, x, y) != rgbAt(hover, x, y) {
				changed++
			}
		}
	}
	t.Logf("悬停让「最多播放」内 %d 像素变化", changed)
	if changed < 100 {
		t.Errorf("悬停只让 %d 个像素变化：悬停材质（白 .30 底 + 亮边 + 抬起）没接上？", changed)
	}
}

// TestDanmakuTabSwitchResetsAutoScroll 钉住切 tab 重置自动跟随（原版
// danmakuList.tsx 的 handleViewModeChange：setIsAutoScroll(true) +
// lastScrollIndexRef=-1）。用户手动暂停跟随后切到评论再切回弹幕，跟随
// 应恢复开启。
func TestDanmakuTabSwitchResetsAutoScroll(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = DrawerDanmaku
	a.DanmakuAutoScroll = false // 用户手动暂停跟随
	tt := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 680))

	// 弹幕 tab 标签带条数（无弹幕时是「弹幕 (0)」），Click 精确匹配。
	if err := tt.Click("评论"); err != nil {
		t.Fatalf("点「评论」tab：%v", err)
	}
	tt.Frame()
	if err := tt.Click("弹幕 (0)"); err != nil {
		t.Fatalf("点回「弹幕」tab：%v", err)
	}
	tt.Frame()

	if !a.DanmakuAutoScroll {
		t.Error("切回弹幕 tab 后自动跟随应重置为开（原版 handleViewModeChange 里 setIsAutoScroll(true)）")
	}
}

// TestDrawerTabGroupRoleAndSelection 钉住抽屉 tab 排的无障碍结构与键盘行为，
// 对齐原版 HeroUI <Tabs aria-label=…>（每个抽屉表头都是它）：
//
//   - 整排是 role=tablist（mygo RoleTabList）+ FocusGroup(Horizontal)；
//   - 每颗 tab 是 role=tab（RoleTab），选中态带 checked（aria-selected）；
//   - 左右方向键在 tab 间移动焦点，且**移动即选中**（HeroUI Tabs 默认
//     keyboardActivation="automatic"）。
//
// 此前整排只是个裸 Row、每颗 tab 是普通 button：读屏看不到「这是一组 tab」、
// 也不知道谁被选中，键盘也只能逐个 Tab。
func TestDrawerTabGroupRoleAndSelection(t *testing.T) {
	// 源码链（无头 Tester 不导出无障碍树）：
	flat := flattenNoComments(t, "drawer.go")
	if !strings.Contains(flat, `Role(ui.RoleTabList).FocusGroup(ui.Horizontal).Label(tabListLabel(group))`) {
		t.Error("tab 排没有接 role=tablist + FocusGroup(Horizontal) + aria-label")
	}
	if !strings.Contains(flat, `Role(ui.RoleTab).Checked(active)`) {
		t.Error("tab 没有接 role=tab + 选中态（aria-selected）")
	}
	if !strings.Contains(flat, `tabArrowSelect(els,func(iint){onSelect(items[i].Key)})`) {
		t.Error("tab 排没有接左右方向键的「移动即选中」")
	}
	// aria-label 逐组对齐原版：六个抽屉 tab 组各有文案。
	for group, label := range map[string]string{
		"sort": "搜索结果排序", "rec": "热门/推荐切换", "hist": "历史/稍后再看切换",
		"up": "视频/合集切换", "dm": "弹幕/评论切换", "playlist": "播放列表来源切换",
	} {
		if got := tabListLabel(group); got != label {
			t.Errorf("tabListLabel(%q) = %q，应为 %q（原版 <Tabs aria-label>）", group, got, label)
		}
	}

	// 无头实测：搜索抽屉三个 tab，点「综合」聚焦，Right 两次依次选中
	// 「最多播放」「最新发布」。
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = DrawerSearch
	a.Query = "关键词"
	a.SortOrder = SortTotal
	tt := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 680))
	if err := tt.Click("综合"); err != nil {
		t.Fatalf("点「综合」失败：%v", err)
	}
	tt.Frame()
	if !tt.Focused("综合") {
		t.Fatal("点「综合」后焦点不在它上面")
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if !tt.Focused("最多播放") {
		t.Error("按 → 后焦点没有移到「最多播放」")
	}
	if a.SortOrder != SortClick {
		t.Errorf("按 → 后排序 = %q，应为 %q（移动即选中）", a.SortOrder, SortClick)
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if !tt.Focused("最新发布") {
		t.Error("再按 → 后焦点没有移到「最新发布」")
	}
	if a.SortOrder != SortUpdate {
		t.Errorf("再按 → 后排序 = %q，应为 %q", a.SortOrder, SortUpdate)
	}
	// ← 回退：焦点与选中一起回到「最多播放」。
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	if !tt.Focused("最多播放") || a.SortOrder != SortClick {
		t.Errorf("按 ← 后没有回到「最多播放」（focused=%v sort=%q）",
			tt.Focused("最多播放"), a.SortOrder)
	}
	// 反证：把 tabArrowSelect 里的 choose(to) 去掉 → 上面三条
	// 「移动即选中」断言变红（已实测）。
}
