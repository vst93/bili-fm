package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestOverlayButtonStyle 钉住封面叠加小键的取值。原版两处来源不同：
//
//	选集「添加」  pageList.tsx 的行内类：min-w-6 w-6 h-6（24）、top-1 right-1（4）、
//	              rounded-full、bg-black/30（rgba(0,0,0,.30)）、border-0、
//	              图标 14px；已在列表 → 绿勾 #4ade80，否则白加号。
//	稍后再看「移除」 globals.css 的 .watchlater-remove-btn：26×26、top/right 6、
//	              圆角 7、1px 白 .48 边、底 rgba(15,23,42,.62)、图标 #f8fafc 12px；
//	              :hover 底 .76 + 图标 #fff + scale(1.04)；:disabled opacity .55。
//
// 原生版此前两处都用「#0f172a 半透明 + 白 .2 边」的近似值：添加键的底/边框/
// 图标尺寸都不对，移除键整体小一圈（22）、圆角偏方（4）、贴角偏内（4）。
func TestOverlayButtonStyle(t *testing.T) {
	// 选集「添加」键。
	if partAddSize != 24 {
		t.Errorf("添加键边长 = %d，应为 24（w-6 h-6）", partAddSize)
	}
	if partAddInset != 4 {
		t.Errorf("添加键贴角 = %d，应为 4（top-1 right-1）", partAddInset)
	}
	if partAddIconSize != 14 {
		t.Errorf("添加键图标 = %d，应为 14（<AddOne size={14} />）", partAddIconSize)
	}
	if got := partAddFill(false); got != ui.Hex("#000000").Alpha(0.30) {
		t.Errorf("添加键常态底 = %v，应为纯黑 .30（bg-black/30）", got)
	}
	if got := partAddFill(true); got != ui.Hex("#000000").Alpha(0.42) {
		t.Errorf("添加键悬停底 = %v，应为纯黑 .42", got)
	}
	if got := partAddInk(false); got != ui.Hex("#ffffff") {
		t.Errorf("添加键未在列表图标 = %v，应为白（AddOne fill=#fff）", got)
	}
	if got := partAddInk(true); got != ui.Hex("#4ade80") {
		t.Errorf("添加键已在列表图标 = %v，应为 #4ade80（Check fill=#4ade80）", got)
	}

	// 稍后再看「移除」键。
	if wlRemoveSize != 26 {
		t.Errorf("移除键边长 = %d，应为 26（.watchlater-remove-btn 26×26）", wlRemoveSize)
	}
	if wlRemoveInset != 6 {
		t.Errorf("移除键贴角 = %d，应为 6（.watchlater-remove-btn top/right 6px）", wlRemoveInset)
	}
	if wlRemoveRadius != 7 {
		t.Errorf("移除键圆角 = %d，应为 7（border-radius 7px）", wlRemoveRadius)
	}
	if wlRemoveIconSize != 12 {
		t.Errorf("移除键图标 = %d，应为 12（<Close size=\"12\" />）", wlRemoveIconSize)
	}
	if got := wlRemoveFill(false); got != ui.Hex("#0f172a").Alpha(0.62) {
		t.Errorf("移除键常态底 = %v，应为墨蓝 .62", got)
	}
	if got := wlRemoveFill(true); got != ui.Hex("#0f172a").Alpha(0.76) {
		t.Errorf("移除键悬停底 = %v，应为墨蓝 .76", got)
	}
	if got := wlRemoveBorder(); got != ui.Hex("#ffffff").Alpha(0.48) {
		t.Errorf("移除键描边 = %v，应为白 .48", got)
	}
	if got := wlRemoveInk(false); got != ui.Hex("#f8fafc") {
		t.Errorf("移除键常态图标 = %v，应为 #f8fafc", got)
	}
	if got := wlRemoveInk(true); got != ui.Hex("#ffffff") {
		t.Errorf("移除键悬停图标 = %v，应为白", got)
	}
	if got := wlRemoveScale(true); got != 1.04 {
		t.Errorf("移除键悬停缩放 = %v，应为 1.04", got)
	}
	if got := wlRemoveScale(false); got != 1 {
		t.Errorf("移除键常态缩放 = %v，应为 1", got)
	}
	if got := wlRemoveOpacity(true); got != 0.55 {
		t.Errorf("移除键请求中透明度 = %v，应为 .55", got)
	}
	if got := wlRemoveOpacity(false); got != 1 {
		t.Errorf("移除键常态透明度 = %v，应为 1", got)
	}
}

// TestPartAddButtonRendered 是整块无头实测：选集卡片的添加键渲染成 24×24 的
// 正圆，且填充是**中性黑**（原版 bg-black/30：三通道等量，B-R ≈ 0），而不是
// 之前的墨蓝 #0f172a（B-R 明显偏正）。反证：把 partAddFill 换回
// ui.Hex("#0f172a").Alpha(0.45) → 该像素 B-R 从 ~2 涨到 ~14，测试红。
func TestPartAddButtonRendered(t *testing.T) {
	a := NewApp(func() {})
	a.Info = newPartsInfo(1, "cover")
	a.Drawer = DrawerParts
	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))

	r, ok := tst.Find("添加到播放列表")
	if !ok {
		t.Fatal("找不到选集添加键")
	}
	if int(r.W+0.5) != partAddSize || int(r.H+0.5) != partAddSize {
		t.Fatalf("添加键实测 %v×%v，应为 %d×%d", r.W, r.H, partAddSize, partAddSize)
	}

	// 键内左侧（避开居中的图标）与键外的封面各取一点，同一行。
	bx, by := int(r.X+5), int(r.Y+r.H/2)
	button := rgbAt(tst.Image(), bx, by)
	cover := rgbAt(tst.Image(), int(r.X-8), by)
	t.Logf("添加键内=%v 旁边封面=%v", button, cover)

	lum := func(c [3]int) int { return c[0] + c[1] + c[2] }
	if lum(button) >= lum(cover) {
		t.Errorf("添加键应压暗封面：键内=%v 封面=%v", button, cover)
	}
	br := button[2] - button[0]
	if br > 6 {
		t.Errorf("添加键应是中性黑（B-R 小），得到 %v (B-R=%d)", button, br)
	}
}

// TestWatchLaterRemoveButtonRendered 是整块无头实测：稍后再看卡片的移除键
// 渲染成 26×26（不是之前的 22），底色是深墨蓝、整体比旧值更暗（.62 vs .45）。
// 反证：把 wlRemoveSize 改回 22 → TestOverlayButtonStyle 里硬写的 26 断言红
// （渲染断言跟着常量走）；把底改回 .45 → 本测的亮度阈值红（实测 sum 417 > 380）。
func TestWatchLaterRemoveButtonRendered(t *testing.T) {
	a := NewApp(func() {})
	l := a.ListFor("history")
	l.Cards = []Card{{Bvid: "BV1", Title: "稍后再看视频", Track: Track{Aid: 42, Bvid: "BV1"}}}
	a.Drawer = "history"
	a.HistTab = HistWatchLater

	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))

	r, ok := tst.Find("从稍后再看移除")
	if !ok {
		t.Fatal("找不到稍后再看移除键")
	}
	if int(r.W+0.5) != wlRemoveSize || int(r.H+0.5) != wlRemoveSize {
		t.Fatalf("移除键实测 %v×%v，应为 %d×%d", r.W, r.H, wlRemoveSize, wlRemoveSize)
	}

	button := rgbAt(tst.Image(), int(r.X+5), int(r.Y+r.H/2))
	t.Logf("移除键内=%v", button)
	lum := button[0] + button[1] + button[2]
	if lum > 380 {
		t.Errorf("移除键底应是深墨蓝 .62（比旧 .45 更暗），得到 %v (sum=%d)", button, lum)
	}
	if button[2]-button[0] < 10 {
		t.Errorf("移除键底应偏蓝（#0f172a），得到 %v", button)
	}
}
