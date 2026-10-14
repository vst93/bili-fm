package view

import (
	"testing"
)

// 本轮（videoInfo 详情区 `#video-desc` 的最小高度）的测试。
//
// 原版 globals.css 给简介挂了两个盒子：
//
//	#video-desc   { margin: 14px 0 0; max-height: 60px; overflow: hidden;
//	                color: var(--studio-muted); font-size: 13px; line-height: 1.55 }
//	#video-desc p { min-height: 46px; max-height: 60px; display: -webkit-box;
//	                -webkit-line-clamp: 3; -webkit-box-orient: vertical }
//
// 外层 60px 是**上限**（3 行），内层 p 的 46px 是**下限**（约 2.28 行）。所以
// 简介不足两行时（详情还没拉到时固定文案「无描述」），这一块仍占 46px —— 下方
// 的 `.video-part-pill` / `#content-actions` / `#context-dock` 因此不会因为
// 简介只有一行而上移。原生版此前只做 MaxLines(3) 截断、没有下限，一行简介时
// 整块只有 ~20px，下面三段的相对位置随之偏离。

// descApp 造一个宽窗主界面：详情已就位但 `Desc` 为空 ⇒ infoDesc() 回落到
// 「无描述」（一行，正是需要被 46px 下限撑住的情形）。
func descApp() *App {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.CoverMode = "disc"
	a.Info = &Info{Bvid: "BV1", Aid: 1, Cid: 2, Title: "标题"}
	a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题", Part: "P1", Up: "UP"}
	a.Queue = []Track{*a.Track}
	a.Index = 0
	return a
}

// TestVideoDescMinHeight 真实跑一遍 App.Shell，量简介文案的包围盒高度：一行
// 「无描述」也必须被撑到 infoDescMinH（原版 `#video-desc p` 的 min-height）。
func TestVideoDescMinHeight(t *testing.T) {
	tt := shellTesterSize(t, descApp(), 1000, 700)
	r, ok := tt.Find("无描述")
	if !ok {
		t.Fatal("找不到简介文案「无描述」")
	}
	t.Logf("简介块 rect = %+v（高 %v）", r, r.H)
	if r.H != infoDescMinH {
		t.Errorf("简介块高 %v，want %v（原版 #video-desc p 的 min-height: 46px）", r.H, infoDescMinH)
	}
}

// TestVideoDescMinHeightShiftsPill 是上一条的反证：把下限归零，简介块随之变矮，
// 下方的选集胶囊位置跟着变 —— 说明这一处确实在撑布局，而不是被别的盒子吸收掉。
// 同一份 App 跑两次，只切 infoDescMinH。注意右栏现在是**固定盒高**（见
// TestVideoInfoFixedBoxHeight），盒顶不动，所以胶囊位移等于简介块高差的全量
// （26px），不再是旧实现里被外层居中折半后的 13px。
func TestVideoDescMinHeightShiftsPill(t *testing.T) {
	measure := func(minH float32) (descH, pillY float32) {
		old := infoDescMinH
		infoDescMinH = minH
		defer func() { infoDescMinH = old }()
		tt := shellTesterSize(t, descApp(), 1000, 700)
		d, ok := tt.Find("无描述")
		if !ok {
			t.Fatal("找不到简介文案「无描述」")
		}
		p, ok := tt.Find("当前选集")
		if !ok {
			t.Fatal("找不到选集胶囊文案「当前选集」")
		}
		return d.H, p.Y
	}
	withH, pillWith := measure(46)
	withoutH, pillWithout := measure(0)
	t.Logf("min-height 46：简介高 %v / 胶囊 Y %v；归零：简介高 %v / 胶囊 Y %v",
		withH, pillWith, withoutH, pillWithout)
	if withH <= withoutH {
		t.Errorf("下限没生效：46→%v，0→%v", withH, withoutH)
	}
	if pillWith <= pillWithout {
		t.Errorf("下限归零后胶囊没有上移：46→%v，0→%v", pillWith, pillWithout)
	}
}

// TestVideoDescMaxLines 是下限的另一面：简介超长时仍被 3 行（≈60px）截住 ——
// MinHeight 是地板不是天花板，不能把长简介也压成 46px，也不能让它无限撑开。
// 对应原版 `#video-desc { max-height: 60px }` + `p { -webkit-line-clamp: 3 }`。
func TestVideoDescMaxLines(t *testing.T) {
	a := descApp()
	// 足够长的一行中文，能铺满远多于 3 行。
	line := "这是一段用于测试三行截断的长简介内容，重复很多遍以确保一定会溢出。"
	for i := 0; i < 12; i++ {
		a.Info.Desc += line
	}
	tt := shellTesterSize(t, a, 1000, 700)
	r, ok := tt.Find(a.Info.Desc)
	if !ok {
		t.Fatal("找不到长简介文案")
	}
	t.Logf("长简介 rect 高 %v", r.H)
	if r.H != 60 {
		t.Errorf("长简介块高 %v，want 60（3 行 × 13px × 1.55 ≈ 60）", r.H)
	}
	if r.H <= infoDescMinH {
		t.Errorf("长简介被下限压住了：%v <= %v", r.H, infoDescMinH)
	}
}

// TestVideoInfoFixedBoxHeight 钉住宽窗右栏的**固定盒高**（原版
// `#video-info { height: 366px }` + `.video-meta-area { align-content: start }`）：
// 内容从盒顶排，整块再由外层 `.home-now-playing` 的 `align-items: center` 居中。
// 此前原生版右栏高度随内容走、又被外层居中，简介行数一变整块就漂（且漂幅只有
// 内容差的一半）。这里同一份 App 跑两次，只把简介从一行（46）换成三行（60）：
//
//   - 盒顶（用 UP 主行的 Y 近似）必须纹丝不动；
//   - 盒顶不动 ⇒ 下方胶囊的位移应等于简介块高差的全量（14），而不是一半（7）。
func TestVideoInfoFixedBoxHeight(t *testing.T) {
	if videoInfoHeight != 366 {
		t.Fatalf("videoInfoHeight = %v，应为原版 #video-info 的 366", videoInfoHeight)
	}

	// 同一份 App 里塞不同长度的简介，量盒顶附近的 UP 主行、简介块与胶囊。
	measure := func(long bool) (topY, descH, pillY float32) {
		a := descApp()
		if long {
			line := "这是一段用于测试三行截断的长简介内容，重复很多遍以确保一定会溢出。"
			for i := 0; i < 12; i++ {
				a.Info.Desc += line
			}
		}
		tt := shellTesterSize(t, a, 1000, 700)
		up, ok := tt.Find("UP 主")
		if !ok {
			t.Fatal("找不到 UP 主行")
		}
		label := a.Info.Desc
		if label == "" {
			label = "无描述"
		}
		d, ok := tt.Find(label)
		if !ok {
			t.Fatalf("找不到简介文案 %q", label)
		}
		p, ok := tt.Find("当前选集")
		if !ok {
			t.Fatal("找不到选集胶囊文案「当前选集」")
		}
		return up.Y, d.H, p.Y
	}

	shortTop, shortH, shortPill := measure(false)
	longTop, longH, longPill := measure(true)
	t.Logf("一行简介：盒顶 Y=%v 简介高 %v 胶囊 Y=%v；三行简介：盒顶 Y=%v 简介高 %v 胶囊 Y=%v",
		shortTop, shortH, shortPill, longTop, longH, longPill)

	if shortTop != longTop {
		t.Errorf("简介从 %v 变到 %v 时盒顶漂了：UP 主 Y %v → %v（固定盒高没生效）",
			shortH, longH, shortTop, longTop)
	}
	// 盒顶不动：胶囊位移 = 简介块高差的全量（不是旧实现的一半）。
	if got, want := longPill-shortPill, longH-shortH; got != want {
		t.Errorf("胶囊位移 %v，应为简介块高差的全量 %v（盒顶不动）", got, want)
	}
}
