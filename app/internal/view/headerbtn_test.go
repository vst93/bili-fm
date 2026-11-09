package view

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（抽屉表头材质统一）之后，表头小键与 tab / 开关 / 关闭键共用
// headeraction.go 的一套状态解析（见 headeraction_test.go 的交互回归）。
// 这里只留选集抽屉表头小键自身的两条手感与文案断言。

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
	return a, settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700))
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
	if headerPressAnim.Duration != 100*time.Millisecond {
		t.Errorf("按压动画时长 = %v，应为原版 transform 0.1s", headerPressAnim.Duration)
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
	tst.Advance(150 * time.Millisecond) // 推过 100ms 按压动画（v0.5.5 冻结时钟）
	pressed, ok := tst.Find("全部添加")
	if !ok {
		t.Fatal("按下后找不到「全部添加」")
	}
	if !(pressed.W < rest.W-0.5) {
		t.Errorf("按下应缩到 %.2f：宽 %v → %v", headerPressScale, rest.W, pressed.W)
	}
	tst.Release(cx, cy)
	tst.Advance(150 * time.Millisecond)
	released, ok := tst.Find("全部添加")
	if !ok {
		t.Fatal("松手后找不到「全部添加」")
	}
	if d := released.W - rest.W; d > 0.5 || d < -0.5 {
		t.Errorf("松手应回到原尺寸：宽 %v → %v", rest.W, released.W)
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

// TestPartsAddAllInk 钉住「全部添加」键的图标 / 文字色（原版 pageList.tsx
// 的 AddOne fill="#666" + span 继承；此前误用抽屉表头通用的 #475569）。
func TestPartsAddAllInk(t *testing.T) {
	if partsAddAllInk != ui.Hex("#666666") {
		t.Errorf("「全部添加」图标色 = %v，应为 #666666（原版 AddOne fill=#666）", partsAddAllInk)
	}
}
