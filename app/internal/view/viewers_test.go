package view

import (
	"fmt"
	"image"
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// viewersApp 造一个「正在播一条视频」的右栏：在线人数是唯一变量。
func viewersApp(on int64, cid int64) *App {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Info = &Info{Bvid: "BV1", Aid: 1, Cid: 2, Title: "短标题", Desc: "无描述", OwnerName: "某UP"}
	a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "短标题", Part: "P1"}
	a.Queue = []Track{*a.Track}
	a.Index = 0
	a.OnlineCount, a.OnlineCid = on, cid
	return a
}

// onlineLabel 是右栏行尾那行在线人数的完整文案（测试里统一从这里取，
// 改文案时只改一处）。
func onlineLabel(n int64) string { return onlineCountText(n) + " 人在看" }

// TestOnlineBadgeAbsentWithoutData 钉住「不知道就不画」：没有数、或数不属于
// 当前这一集时，右栏不该出现「0 人在看」这种误导文案。
func TestOnlineBadgeAbsentWithoutData(t *testing.T) {
	for _, tc := range []struct {
		name    string
		on, cid int64
	}{
		{"接口没给", 0, 0},
		{"数没归属", 152, 0},
		{"归属是别的集", 152, 999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := viewersApp(tc.on, tc.cid).onlineBadgeText(); got != "" {
				t.Fatalf("不该有文案，got %q", got)
			}
			tt := shellTesterSize(t, viewersApp(tc.on, tc.cid), 800, 600)
			for _, label := range []string{onlineLabel(152), onlineLabel(0)} {
				if _, ok := tt.Find(label); ok {
					t.Fatalf("%s 不该被画出来", label)
				}
			}
		})
	}
}

// TestOnlineBadgeTextOwnership 钉住文案的三道闸：没数 / 数没归属 / 归属不是
// 当前这一集，都不显示。
func TestOnlineBadgeTextOwnership(t *testing.T) {
	base := func(on, cid int64) *App {
		return viewersApp(on, cid) // Track.Cid = 2，Info.Cid = 2
	}
	if got := base(152, 2).onlineBadgeText(); got != onlineLabel(152) {
		t.Fatalf("归属对得上应有文案，got %q", got)
	}
	if got := base(0, 2).onlineBadgeText(); got != "" {
		t.Fatalf("没数不该有文案，got %q", got)
	}
	if got := base(152, 0).onlineBadgeText(); got != "" {
		t.Fatalf("数没归属不该有文案，got %q", got)
	}
	if got := base(152, 999).onlineBadgeText(); got != "" {
		t.Fatalf("归属是别的集不该有文案，got %q", got)
	}
	// 一万以上按「x.x万」。
	if got := base(15234, 2).onlineBadgeText(); got != onlineLabel(15234) {
		t.Fatalf("got %q", got)
	}
}

// TestOnlineBadgeWording 钉住用词口径：与 B 站网页播放器那个气泡一致，
// 就是「N 人在看」。用户 2026-10-29 明确要求回到这个说法（中途试过
// 「同时在线 N 人」这种更笼统的表达，最后决定还是跟 B 站一致，不需要额外解释）。
func TestOnlineBadgeWording(t *testing.T) {
	text := viewersApp(152, 2).onlineBadgeText()
	if text != "152 人在看" {
		t.Fatalf("文案 = %q，应为「152 人在看」（与 B 站口径一致）", text)
	}
	if !strings.HasSuffix(text, "人在看") {
		t.Fatalf("文案 %q 应以「人在看」结尾", text)
	}
}

// TestOnlineBadgeSitsInPartPill 钉住主位置：挂在**选集胶囊**里、分集标题之后
// —— 那颗胶囊写的就是「这一集在播什么」（来源 + 分集标题），在线人数是同一件
// 事的另一面，读作「● 当前选集 P1 · 152 人在看」。
//
// 用户四轮反馈的落点：胶囊 + 圆点太扎眼 → UP 主行行尾太飘 → 操作行行尾
// 「一样很丑，甚至比之前还丑」（与那一排图标键远隔 100px，读不成一组）。
func TestOnlineBadgeSitsInPartPill(t *testing.T) {
	for _, w := range []int{800, 600, 420, 400} {
		t.Run(fmt.Sprintf("%dw", w), func(t *testing.T) {
			a := viewersApp(152, 2)
			a.Info.Parts = []Part{{Cid: 2, Page: 1, Part: "P1"}}
			tt := shellTesterSize(t, a, w, 600)
			part, ok := tt.Find("P1")
			if !ok {
				t.Fatal("找不到分集标题")
			}
			badge, ok := tt.Find(onlineLabel(152))
			if !ok {
				t.Fatalf("没画出「%s」", onlineLabel(152))
			}
			// 同一行（同一颗胶囊里，y 差不超过 1px）。
			if d := badge.Y - part.Y; d < -1 || d > 1 {
				t.Fatalf("在线人数与分集标题 y 差 %v，应在同一行", d)
			}
			// 紧接在分集标题之后（含分隔点，间距不超过 30px）。
			if gap := badge.X - (part.X + part.W); gap <= 0 || gap > 30 {
				t.Fatalf("在线人数离分集标题 %v，应紧邻（0 < gap ≤ 30）", gap)
			}
			if badge.X+badge.W > float32(w) {
				t.Fatalf("超出窗口：right=%v w=%d", badge.X+badge.W, w)
			}
		})
	}
}

// TestOnlineBadgeFallsBackToActionsRow 钉住退路：单 P 视频没有分集标题，选集
// 胶囊整块省略（见 partPillShown），此时在线人数退到操作行行尾 —— 这个数不能
// 因为胶囊不在就消失。
func TestOnlineBadgeFallsBackToActionsRow(t *testing.T) {
	a := viewersApp(152, 2)
	a.Track.Part = "" // 单 P：胶囊不画
	tt := shellTesterSize(t, a, 800, 600)
	if _, ok := tt.Find("当前选集"); ok {
		t.Fatal("单 P 视频不该有选集胶囊")
	}
	like, ok := tt.Find("点赞")
	if !ok {
		t.Fatal("找不到点赞键")
	}
	badge, ok := tt.Find(onlineLabel(152))
	if !ok {
		t.Fatalf("单 P 视频的在线人数消失了（应退到操作行）")
	}
	if badge.Y != like.Y || badge.H != like.H {
		t.Fatalf("退路形态应与操作行按钮同行同高：badge=%v like=%v", badge, like)
	}
	if badge.X+badge.W > 800 {
		t.Fatalf("超出窗口：right=%v", badge.X+badge.W)
	}
}

// TestOnlineBadgeDoesNotShiftInfoLayout 是关键反证：这行字必须**不改变**右栏
// 其它内容的落点。右栏是固定 366 高的盒，标题 / 简介 / 选集胶囊 / 操作行都按
// 原版逐值排过 —— 一旦在线人数占掉一行高度，下方就整体上移/下移。
func TestOnlineBadgeDoesNotShiftInfoLayout(t *testing.T) {
	yOf := func(t *testing.T, on int64, label string) float32 {
		t.Helper()
		tt := shellTesterSize(t, viewersApp(on, 2), 800, 600)
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("找不到 %q", label)
		}
		return r.Y
	}
	for _, label := range []string{"短标题", "搜索结果"} {
		off := yOf(t, 0, label)
		on := yOf(t, 152, label)
		if off != on {
			t.Fatalf("%s 的 Y 从 %v 变成 %v —— 在线人数这行不该把它顶下去", label, off, on)
		}
	}
}

// TestOnlineBadgeHasNoChrome 是无头像素实测的「轻量化」证据：
//
//	① 文字盒上下各取一行像素，与右栏背景同一色（没有胶囊底、没有描边）；
//	② 文字左侧没有小圆点（先前那个玫瑰红 #e11d48 的 6px 圆点已删除）。
func TestOnlineBadgeHasNoChrome(t *testing.T) {
	tt := shellTesterSize(t, viewersApp(152, 2), 800, 600)
	r, ok := tt.Find(onlineLabel(152))
	if !ok {
		t.Fatal("没画出在线人数")
	}
	img := tt.Image()

	// ① 文字盒上方 6px 处（原胶囊顶内边距的位置）应与右栏背景一致。
	sameColor := func(x, y int, want [3]int) bool {
		rr, gg, bb, _ := rgbaAt(t, img, x, y)
		d := absInt(rr-want[0]) + absInt(gg-want[1]) + absInt(bb-want[2])
		return d <= 6
	}
	refX, refY := int(r.X+2), int(r.Y)-6
	rr, gg, bb, _ := rgbaAt(t, img, refX, refY)
	bg := [3]int{rr, gg, bb}
	// 文字盒右侧再往外 6px 也应是同一片背景（原胶囊的右内边距 + 描边）。
	if !sameColor(int(r.X+r.W)+6, int(r.Y+r.H/2), bg) {
		r2, g2, b2, _ := rgbaAt(t, img, int(r.X+r.W)+6, int(r.Y+r.H/2))
		t.Fatalf("文字盒右侧出现装饰：rgb(%d,%d,%d) vs 背景 rgb(%d,%d,%d)", r2, g2, b2, rr, gg, bb)
	}

	// ② 文字左侧 9px（原圆点的位置）不该是玫瑰红。
	dotX, dotY := int(r.X)-9, int(r.Y+r.H/2)
	dr, dg, db, _ := rgbaAt(t, img, dotX, dotY)
	if dr > 150 && dr-dg > 60 && db > 30 {
		t.Fatalf("(%d,%d) 仍是玫瑰红圆点 rgb(%d,%d,%d) —— 用户要求删掉", dotX, dotY, dr, dg, db)
	}
}

// TestOnlineBadgeIsDeemphasized 钉住「弱化到位」：它的墨色必须比正文
// （简介 / 选集标题用的 --studio-muted #475569）明显更浅 —— 用户要求
// 「弱化颜色」，不能只是删掉装饰。
//
// 但它也不能浅到读不出来：挂在操作行后，它是「作品数据」的一员，旁边是
// 点赞 / 投币 / 收藏的计数，太浅会显得像坏了。所以用 --studio-faint
// （#64748b，与空态提示同一档），比正文轻、比 kicker 的小标签深。
func TestOnlineBadgeIsDeemphasized(t *testing.T) {
	relLum := func(r, g, b int) float64 {
		chanL := func(v int) float64 {
			c := float64(v) / 255
			if c <= 0.03928 {
				return c / 12.92
			}
			return math.Pow((c+0.055)/1.055, 2.4)
		}
		return 0.2126*chanL(r) + 0.7152*chanL(g) + 0.0722*chanL(b)
	}
	// 某一盒里最暗的那颗像素（墨色最深的一处）。
	darkestIn := func(img *image.RGBA, r ui.Rect) (int, int, int) {
		best := 1.0
		br, bg, bb := 0, 0, 0
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				i := img.PixOffset(x, y)
				if l := relLum(int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])); l < best {
					best, br, bg, bb = l, int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
				}
			}
		}
		return br, bg, bb
	}
	for _, hour := range []int{12, 22} {
		t.Run(fmt.Sprintf("%dh", hour), func(t *testing.T) {
			a := viewersApp(152, 2)
			a.Theme = ThemeFor(PeriodAt(hour))
			tt := shellTesterSize(t, a, 800, 600)
			badge, ok := tt.Find(onlineLabel(152))
			if !ok {
				t.Fatal("没画出在线人数")
			}
			kick, ok := tt.Find("UP 主")
			if !ok {
				t.Fatal("找不到 kicker")
			}
			img := tt.Image()

			_ = kick
			br, bg, bb := darkestIn(img, badge)

			// ① 比正文轻：正文用的是 --studio-muted #475569。
			badgeL := relLum(br, bg, bb)
			mutedL := relLum(0x47, 0x55, 0x69)
			if badgeL <= mutedL {
				t.Fatalf("在线人数墨色亮度 %.3f 不高于正文 %.3f —— 没弱化", badgeL, mutedL)
			}
			// ② 但也不能浅到读不出来：至少比空态提示那档（--studio-faint）不深太多，
			//    即实测墨色应当落在 faint 附近（±0.03 亮度）。
			faintL := relLum(0x64, 0x74, 0x8b)
			if badgeL < faintL-0.03 || badgeL > faintL+0.03 {
				t.Fatalf("在线人数墨色亮度 %.3f 偏离 --studio-faint（%.3f）太多", badgeL, faintL)
			}
		})
	}
}

// TestOnlineCountText 钉住文案：不足一万给确切数字（实时量，缩写了就看不出
// 人在动），一万以上才按「x.x万」。
func TestOnlineCountText(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{1, "1"},
		{152, "152"},
		{9999, "9999"},
		{10000, "1.0万"},
		{15234, "1.5万"},
		{1200000, "120.0万"},
	}
	for _, c := range cases {
		if got := onlineCountText(c.n); got != c.want {
			t.Errorf("onlineCountText(%d) = %q, want %q", c.n, got, c.want)
		}
	}
	if got := onlineLabel(152); got != "152 人在看" {
		t.Fatalf("文案 = %q", got)
	}
}
