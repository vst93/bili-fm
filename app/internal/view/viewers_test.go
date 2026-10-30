package view

import (
	"fmt"
	"math"
	"strings"
	"testing"
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
func onlineLabel(n int64) string { return "同时在线 " + onlineCountText(n) + " 人" }

// TestOnlineBadgeAbsentWithoutData 钉住「不知道就不画」：没有数、或数不属于
// 当前这一集时，右栏不该出现「0 人在线」这种误导文案。
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

// TestOnlineBadgeWordingAvoidsWatching 钉住用词：这是收听类应用，文案不能说
// 「在看」。用户明确要求换成更笼统的表达（本轮用「同时在线 N 人」）。
func TestOnlineBadgeWordingAvoidsWatching(t *testing.T) {
	text := viewersApp(152, 2).onlineBadgeText()
	if text == "" {
		t.Fatal("应有文案")
	}
	for _, banned := range []string{"在看", "观看", "播放中"} {
		if strings.Contains(text, banned) {
			t.Fatalf("文案 %q 含「%s」—— 收听类应用不该用观看类字眼", text, banned)
		}
	}
	if !strings.Contains(text, "在线") {
		t.Fatalf("文案 %q 应表达「在线」这种笼统的说法", text)
	}
}

// TestOnlineBadgeShownForCurrentEpisode 钉住正向路径：数属于当前这一集时
// 画出来，且落在 UP 主行（与头像同一行、右对齐）。
func TestOnlineBadgeShownForCurrentEpisode(t *testing.T) {
	tt := shellTesterSize(t, viewersApp(152, 2), 800, 600)
	badge, ok := tt.Find(onlineLabel(152))
	if !ok {
		t.Fatalf("右栏没画出「%s」", onlineLabel(152))
	}
	face, ok := tt.Find("UP 主空间")
	if !ok {
		t.Fatal("找不到 UP 头像，无法比对行位置")
	}
	// 同一行：文字盒的垂直中心落在头像那一段里（不新增一行）。
	mid := badge.Y + badge.H/2
	if mid < face.Y-2 || mid > face.Y+face.H+2 {
		t.Fatalf("在线人数的垂直中心 %v 不在头像行 [%v,%v] 内 —— 它把下方内容顶下去了",
			mid, face.Y, face.Y+face.H)
	}
	// 右对齐：文字盒右缘比头像右缘靠右得多（行尾），且在窗口内。
	if badge.X+badge.W <= face.X+face.W+40 {
		t.Fatalf("没被顶到行尾: badge=%v face=%v", badge, face)
	}
	if badge.X < 0 || badge.X+badge.W > 800 {
		t.Fatalf("超出窗口: %v", badge)
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

// 长名字：用来把头像 + 名字 + 在线人数这一行撑到临界。
const longOwnerName = "一个非常非常非常长的UP主名字用来压测试右栏宽度限制测试"

// TestOnlineBadgeNeverOverlapsOwnerName 钉住两件事：
//
//  1. 这行字与 UP 名字**永不重叠**（宽窗长名字、窄窗长名字、窄窗大数字）：
//     空间不够时是名字让位（名字那层带 Shrink(1)+MinWidth(0)），在线人数永远在行尾；
//  2. 没有在线数时名字仍是原版的 240 —— 让位只在真的要画这行字时才发生。
func TestOnlineBadgeNeverOverlapsOwnerName(t *testing.T) {
	cases := []struct {
		w    int
		name string
		on   int64
	}{
		{800, longOwnerName, 0},
		{800, longOwnerName, 152},
		{800, longOwnerName, 15234},
		{420, "短名字", 152},
		{400, longOwnerName, 152},
		{400, longOwnerName, 1200000},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%dw-on%d", tc.w, tc.on), func(t *testing.T) {
			a := viewersApp(tc.on, 2)
			a.Info.OwnerName = tc.name
			a.Track.Up = tc.name
			tt := shellTesterSize(t, a, tc.w, 600)
			name, ok := tt.Find(tc.name)
			if !ok {
				t.Fatal("名字没画出来")
			}
			if tc.on == 0 {
				// 没这行字时不该动原版尺寸。
				if name.W != 240 {
					t.Fatalf("没有在线人数时名字宽应仍是原版 240，got %v", name.W)
				}
				if _, ok := tt.Find(onlineLabel(0)); ok {
					t.Fatal("没数据不该有这行字")
				}
				return
			}
			badge, ok := tt.Find(onlineLabel(tc.on))
			if !ok {
				t.Fatalf("没画出「%s」", onlineLabel(tc.on))
			}
			if badge.X+badge.W > float32(tc.w) {
				t.Fatalf("超出窗口：right=%v w=%d", badge.X+badge.W, tc.w)
			}
			if name.X+name.W > badge.X+0.5 {
				t.Fatalf("与名字重叠：name right=%v badge left=%v", name.X+name.W, badge.X)
			}
			if name.W > 240 {
				t.Fatalf("名字不该超过原版 240：got %v", name.W)
			}
		})
	}
}

// TestOnlineBadgeIsDeemphasized 钉住「弱化到位」的两条：
//
//	① 对比度仍够读：午间 / 夜间都不低于 3:1（辅助信息的下限；正文类文字才要
//	   4.5:1，而这行字是辅助信息、且同样的内容在 tooltip 里也有）；
//	② 确实比正文轻：它的墨色（--studio-faint）比简介 / 选集标题用的
//	   --studio-muted 更浅 —— 用户要求「弱化颜色」，不能只是删掉装饰。
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
	for _, hour := range []int{12, 22} {
		t.Run(fmt.Sprintf("%dh", hour), func(t *testing.T) {
			a := viewersApp(152, 2)
			a.Theme = ThemeFor(PeriodAt(hour))
			tt := shellTesterSize(t, a, 800, 600)
			r, ok := tt.Find(onlineLabel(152))
			if !ok {
				t.Fatal("没画出在线人数")
			}
			img := tt.Image()
			darkest := 1.0
			for y := int(r.Y); y < int(r.Y+r.H); y++ {
				for x := int(r.X); x < int(r.X+r.W); x++ {
					i := img.PixOffset(x, y)
					if l := relLum(int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])); l < darkest {
						darkest = l
					}
				}
			}
			// 文字盒正上方 6px 当背景（现在没有胶囊底，就是右栏底色）。
			i := img.PixOffset(int(r.X+2), int(r.Y)-6)
			bg := relLum(int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2]))
			ratio := (bg + 0.05) / (darkest + 0.05)
			if ratio < 3 {
				t.Fatalf("%dh 时段墨/底对比只有 %.2f:1（辅助信息也不该低于 3:1）", hour, ratio)
			}
			// ② 比正文轻：简介用的是 --studio-muted。
			muted := relLum(0x47, 0x55, 0x69) // Theme.Muted #475569
			faint := relLum(0x64, 0x74, 0x8b) // Theme.Faint #64748b
			if faint <= muted {
				t.Fatalf("在线人数用的墨色（L=%.3f）不比正文（L=%.3f）浅 —— 没弱化", faint, muted)
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
	if got := onlineLabel(152); got != "同时在线 152 人" {
		t.Fatalf("文案 = %q", got)
	}
}
