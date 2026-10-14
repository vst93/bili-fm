package view

import (
	"bytes"
	"image"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（抽屉表头小键材质）的无头测试。原版
// `[data-slot="wrapper"] header button[class*="variant-light"]` /
// `variant-flat` / `variant-solid` 三条逐字相同的规则：
//
//	常态  无边框、--glass-bg-active（白 .16）底 + inset 0 1px 1px --glass-highlight（白 .9）
//	悬停  --glass-bg-hover（白 .38）底 + inset 0 1px 1px 白 .9
//	      + inset 0 -1px 1px 墨 .04 + 0 4px 12px 墨 .06
//	按下  transform: scale(0.94)、底回到 --glass-bg-active
//
// 此前表头小键（刷新 / 定位 / 全部添加 / 关注 / 播放全部）借用播放栏的
// buttonSurface —— 那是 `#player .player-button` 的材质（白 .22 底 + 1px 白 .55
// 边 + 两层内影 + 常驻小投影），多出一圈白边和常驻投影、顶部内高光也暗一档。

// TestDrawerHeaderPaintTokens 逐值钉住两态。
func TestDrawerHeaderPaintTokens(t *testing.T) {
	base := drawerHeaderPaintFor(false)
	if base.BG != ui.Hex("#ffffff").Alpha(0.16) {
		t.Errorf("常态底 = %v，应为 --glass-bg-active 白 .16", base.BG)
	}
	if base.Inset != ui.Hex("#ffffff").Alpha(0.90) {
		t.Errorf("常态内高光 = %v，应为 --glass-highlight 白 .9", base.Inset)
	}
	if base.BotIn.A != 0 || base.Shadow.A != 0 {
		t.Errorf("常态不该有底部内暗线 / 外投影：%+v", base)
	}
	hover := drawerHeaderPaintFor(true)
	if hover.BG != ui.Hex("#ffffff").Alpha(0.38) {
		t.Errorf("悬停底 = %v，应为 --glass-bg-hover 白 .38", hover.BG)
	}
	if hover.Inset != ui.Hex("#ffffff").Alpha(0.90) {
		t.Errorf("悬停内高光 = %v，应为白 .9", hover.Inset)
	}
	if hover.BotIn != ui.Hex("#0f172a").Alpha(0.04) {
		t.Errorf("悬停底部内暗线 = %v，应为墨 .04", hover.BotIn)
	}
	if hover.Shadow != ui.Hex("#0f172a").Alpha(0.06) {
		t.Errorf("悬停外投影 = %v，应为墨 .06", hover.Shadow)
	}
}

// TestDrawerHeaderSurfaceWired 无头反证：把 drawerHeaderSurface 换成 no-op，
// 「全部添加」按钮矩形内应有一片像素随之变化（材质真的接在表头小键上）。
func TestDrawerHeaderSurfaceWired(t *testing.T) {
	orig := drawerHeaderSurface
	defer func() { drawerHeaderSurface = orig }()

	render := func(stub bool) (*image.RGBA, ui.Rect) {
		if stub {
			drawerHeaderSurface = func(b ui.Element, _ bool) ui.Element { return b }
		} else {
			drawerHeaderSurface = orig
		}
		_, tst := partsHeaderProbe(t)
		r, ok := tst.Find("全部添加")
		if !ok {
			t.Fatal("选集抽屉表头里找不到「全部添加」")
		}
		return tst.Image(), r
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
	t.Logf("关掉表头材质后「全部添加」内变化 %d 像素", changed)
	if changed < 100 {
		t.Errorf("表头材质没接上：只变了 %d 个像素", changed)
	}
}

// partsHeaderProbe 建一个打开了选集抽屉的 1000×700 探针（表头里有
// 「定位当前」+ 搜索框 + 「全部添加」）。
func partsHeaderProbe(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Info = &Info{Bvid: "BV1", Aid: 1, Title: "标题", Parts: []Part{
		{Cid: 11, Page: 1, Part: "第一集"},
		{Cid: 12, Page: 2, Part: "第二集"},
	}}
	a.Track = &Track{Bvid: "BV1", Cid: 11, Title: "标题"}
	a.Drawer = DrawerParts
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	return a, tst
}

// TestDrawerHeaderNoHoverLift 表头小键悬停**不上浮**：原版这条规则没有
// `:hover { transform }`，此前借用的 pressPose 会带 1px 上浮。
func TestDrawerHeaderNoHoverLift(t *testing.T) {
	_, tst := partsHeaderProbe(t)
	tst.Move(-1, -1)
	tst.Frame()
	rest, ok := tst.Find("全部添加")
	if !ok {
		t.Fatal("找不到「全部添加」")
	}
	tst.Move(rest.X+rest.W/2, rest.Y+rest.H/2)
	tst.Frame()
	hover, ok := tst.Find("全部添加")
	if !ok {
		t.Fatal("悬停后找不到「全部添加」")
	}
	if hover.Y != rest.Y {
		t.Errorf("悬停把表头小键上移了：Y %v → %v（原版没有 :hover 位移）", rest.Y, hover.Y)
	}
}

// TestDrawerHeaderPressScales 按下缩到 .94（原版 `:active { transform: scale(.94) }`），
// 且松手回到原尺寸。
func TestDrawerHeaderPressScales(t *testing.T) {
	if drawerHeaderPressAnim.Duration != 100*time.Millisecond {
		t.Errorf("按压动画时长 = %v，应为原版 transform 0.1s", drawerHeaderPressAnim.Duration)
	}
	_, tst := partsHeaderProbe(t)
	tst.Move(-1, -1)
	tst.Frame()
	rest, ok := tst.Find("全部添加")
	if !ok {
		t.Fatal("找不到「全部添加」")
	}
	cx, cy := rest.X+rest.W/2, rest.Y+rest.H/2
	tst.Press(cx, cy)
	pressed, ok := tst.Find("全部添加")
	if !ok {
		t.Fatal("按下后找不到「全部添加」")
	}
	if !(pressed.W < rest.W-0.5) {
		t.Errorf("按下应缩到 .94：宽 %v → %v", rest.W, pressed.W)
	}
	tst.Release(cx, cy)
	released, ok := tst.Find("全部添加")
	if !ok {
		t.Fatal("松手后找不到「全部添加」")
	}
	if d := released.W - rest.W; d > 0.5 || d < -0.5 {
		t.Errorf("松手应回到原尺寸：宽 %v → %v", rest.W, released.W)
	}
}

// TestDrawerHeaderSurfaceUsedForHeaderKeys 把 drawerHeaderSurface 换成记录
// 调用的桩，选集抽屉表头里应至少被调用两次（「定位当前」+「全部添加」）。
func TestDrawerHeaderSurfaceUsedForHeaderKeys(t *testing.T) {
	orig := drawerHeaderSurface
	defer func() { drawerHeaderSurface = orig }()
	calls := 0
	drawerHeaderSurface = func(b ui.Element, hovered bool) ui.Element {
		calls++
		return orig(b, hovered)
	}
	partsHeaderProbe(t)
	if calls < 2 {
		t.Errorf("选集抽屉表头只调用了 %d 次表头材质（应有定位 + 全部添加）", calls)
	}
}

// TestPartsAddAllTooltip 钉住「全部添加」键的工具提示：原版 pageList.tsx 给
// 这个键写了 title="将全部选集添加到播放列表"，悬停要能看到「加到哪儿」。
//
// 工具提示在无头环境里要等指针停留 0.6s 才出现，而 view 包的 Tester 拿不到
// 可推进的时钟（那是 ui 包内部的 clockTester），所以这里退一步：钉住常量，
// 并在源码里确认它真的接在那颗键上——写了常量没接等于没写。
func TestPartsAddAllTooltip(t *testing.T) {
	if partsAddAllTip != "将全部选集添加到播放列表" {
		t.Errorf("partsAddAllTip = %q", partsAddAllTip)
	}
	src, err := os.ReadFile("drawer.go")
	if err != nil {
		t.Fatalf("读 drawer.go 失败：%v", err)
	}
	if !bytes.Contains(src, []byte(".Tooltip(partsAddAllTip)")) {
		t.Error("「全部添加」键没接上 partsAddAllTip")
	}
}
