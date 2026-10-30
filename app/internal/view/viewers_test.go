package view

import (
	"fmt"
	"math"
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
			for _, label := range []string{"152 人在看", "0 人在看"} {
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
	if got := base(152, 2).onlineBadgeText(); got != "152 人在看" {
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
	if got := base(15234, 2).onlineBadgeText(); got != "1.5万 人在看" {
		t.Fatalf("got %q", got)
	}
}

// TestOnlineBadgeShownForCurrentEpisode 钉住正向路径：数属于当前这一集时
// 画出来，且落在 UP 主行（与头像同一行、右对齐）。
func TestOnlineBadgeShownForCurrentEpisode(t *testing.T) {
	tt := shellTesterSize(t, viewersApp(152, 2), 800, 600)
	badge, ok := tt.Find("152 人在看")
	if !ok {
		t.Fatal("右栏没画出「152 人在看」")
	}
	face, ok := tt.Find("UP 主空间")
	if !ok {
		t.Fatal("找不到 UP 头像，无法比对行位置")
	}
	// 同一行：胶囊的垂直中心落在头像那一段里（不新增一行）。
	mid := badge.Y + badge.H/2
	if mid < face.Y-2 || mid > face.Y+face.H+2 {
		t.Fatalf("胶囊垂直中心 %v 不在头像行 [%v,%v] 内 —— 它把下方内容顶下去了",
			mid, face.Y, face.Y+face.H)
	}
	// 右对齐：胶囊右缘比头像右缘靠右得多（行尾），且在窗口内。
	if badge.X+badge.W <= face.X+face.W+40 {
		t.Fatalf("胶囊没被顶到行尾: badge=%v face=%v", badge, face)
	}
	if badge.X < 0 || badge.X+badge.W > 800 {
		t.Fatalf("胶囊超出窗口: %v", badge)
	}
}

// TestOnlineBadgeDoesNotShiftInfoLayout 是关键反证：胶囊必须**不改变**右栏
// 其它内容的落点。右栏是固定 366 高的盒，标题 / 简介 / 胶囊 / 操作行都按
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
			t.Fatalf("%s 的 Y 从 %v 变成 %v —— 在线胶囊不该把它顶下去", label, off, on)
		}
	}
}

// TestOnlineBadgeInkIsReadable 是无头像素实测：胶囊里的文字真画出来了，
// 而且比胶囊底明显更暗（墨色可读）；玫瑰红小圆点也落地了。
func TestOnlineBadgeInkIsReadable(t *testing.T) {
	tt := shellTesterSize(t, viewersApp(152, 2), 800, 600)
	img := tt.Image()
	r, ok := tt.Find("152 人在看")
	if !ok {
		t.Fatal("没画出在线胶囊")
	}
	darkest := 255.0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			rr, gg, bb, _ := rgbaAt(t, img, x, y)
			if lum := 0.299*float64(rr) + 0.587*float64(gg) + 0.114*float64(bb); lum < darkest {
				darkest = lum
			}
		}
	}
	if darkest > 150 {
		t.Fatalf("胶囊里没有足够暗的墨点（最暗 %v）—— 文字没画出来或太浅", darkest)
	}
	// 小圆点在文字左边：gap 6 + 半径 3。
	dotX := int(r.X) - 9
	dotY := int(r.Y + r.H/2)
	rr, gg, bb, _ := rgbaAt(t, img, dotX, dotY)
	if !(rr > 150 && rr-gg > 60 && bb > 30) {
		t.Fatalf("(%d,%d) 不是玫瑰红圆点：rgb(%d,%d,%d)", dotX, dotY, rr, gg, bb)
	}
}

// 长名字：用来把头像 + 名字 + 胶囊这一行撑到临界。
const longOwnerName = "一个非常非常非常长的UP主名字用来压测试右栏宽度限制测试"

// TestOnlineBadgeNeverOverlapsOwnerName 钉住两件事：
//
//  1. 胶囊与 UP 名字**永不重叠**（宽窗长名字、窄窗长名字、窄窗大数字）：
//     空间不够时是名字让位（名字那层带 Shrink(1)+MinWidth(0)），胶囊永远在行尾；
//  2. 没有在线数时名字仍是原版的 240 —— 让位只在胶囊真的要画时才发生。
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
				// 没胶囊时不该动原版尺寸。
				if name.W != 240 {
					t.Fatalf("没胶囊时名字宽应仍是原版 240，got %v", name.W)
				}
				if _, ok := tt.Find("0 人在看"); ok {
					t.Fatal("没数据不该有胶囊")
				}
				return
			}
			badge, ok := tt.Find(onlineCountText(tc.on) + " 人在看")
			if !ok {
				t.Fatal("没画出在线胶囊")
			}
			if badge.X+badge.W > float32(tc.w) {
				t.Fatalf("胶囊超出窗口：right=%v w=%d", badge.X+badge.W, tc.w)
			}
			if name.X+name.W > badge.X+0.5 {
				t.Fatalf("胶囊与名字重叠：name right=%v badge left=%v", name.X+name.W, badge.X)
			}
			if name.W > 240 {
				t.Fatalf("名字不该超过原版 240：got %v", name.W)
			}
		})
	}
}

// TestOnlineBadgeReadableInBothThemes 是无头像素实测的对比度门槛：
// 胶囊里的墨色与胶囊底在午间与夜间时段都要到 WCAG AA 的小字 4.5:1。
// （实测：胶囊 ≈6.5:1 / 5.3:1，与同胞的选集胶囊「P1」6.99 / 6.23 同一档。）
func TestOnlineBadgeReadableInBothThemes(t *testing.T) {
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
			r, ok := tt.Find("152 人在看")
			if !ok {
				t.Fatal("没画出在线胶囊")
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
			// 胶囊内的空白处（文字盒上方 8px）当底色。
			i := img.PixOffset(int(r.X+2), int(r.Y)-8)
			bg := relLum(int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2]))
			ratio := (bg + 0.05) / (darkest + 0.05)
			if ratio < 4.5 {
				t.Fatalf("%dh 时段墨/底对比只有 %.2f:1（小字至少 4.5:1）", hour, ratio)
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
	if got := onlineCountText(152) + " 人在看"; got != "152 人在看" {
		t.Fatalf("文案 = %q", got)
	}
}
