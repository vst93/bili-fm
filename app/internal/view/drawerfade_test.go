package view

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（抽屉列表悬停过渡）的测试。原版 globals.css 给抽屉里这几处状态切换
// 都挂了色值过渡：
//
//	评论行 .py-2.px-8.border-b      background-color/border-color 140ms ease
//	弹幕小胶囊 .danmaku-message-row  同上 140ms ease
//	弹幕时间标签 .danmaku-time-label background-color/color 140ms ease
//	弹幕分组行 .danmaku-time-group   border-color 150ms ease
//
// 此前原生版全是瞬时切换（悬停时整行 / 整颗胶囊「啪」地换色）。这轮按原版
// 补上（走 AnimateColor，与导航族 / 播放栏同一手法；阴影无逐通道动画通道，
// 仍当帧切换，不在本轮范围）。

// TestDrawerFadeAnimTokens 钉住两条过渡的参数：140ms（列表项）与 150ms
// （弹幕分组分隔线），都是 CSS ease 曲线 CubicBezier(0.25, 0.1, 0.25, 1)。
func TestDrawerFadeAnimTokens(t *testing.T) {
	cases := []struct {
		name string
		anim ui.Animation
		want time.Duration
	}{
		{"抽屉列表项 140ms", drawerFadeAnim, 140 * time.Millisecond},
		{"弹幕分组分隔线 150ms", danmakuBorderAnim, 150 * time.Millisecond},
	}
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	for _, tc := range cases {
		if tc.anim.Duration != tc.want {
			t.Errorf("%s 时长 = %v, want %v", tc.name, tc.anim.Duration, tc.want)
		}
		if tc.anim.Spring != (ui.SpringOptions{}) {
			t.Errorf("%s 应为曲线动画，不是弹簧：%+v", tc.name, tc.anim.Spring)
		}
		if tc.anim.Ease == nil {
			t.Fatalf("%s 缺少缓动曲线", tc.name)
		}
		for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
			if d := math.Abs(float64(tc.anim.Ease(x) - want(x))); d > 1e-6 {
				t.Errorf("%s 缓动在 %.2f 处 = %v, want CSS ease %v", tc.name, x, tc.anim.Ease(x), want(x))
			}
		}
	}
}

// TestDrawerFadeColorTransitions 实测 drawerFadeColor 真的以「过渡」而非
// 「瞬时」返回色值：目标变化的那一帧曲线 t=0 ⇒ 返回**上一帧的颜色**；
// 直接铺目标色会当帧跳到新值。
func TestDrawerFadeColorTransitions(t *testing.T) {
	colA, colB := ui.Hex("#ffffff").Alpha(0.32), ui.Hex("#0ea5e9").Alpha(0.22)
	cur := colA
	var samples []ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		samples = append(samples, drawerFadeColor(c, b, "probe", cur, drawerFadeAnim))
	}, 60, 60)
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if samples[0] != colA {
		t.Fatalf("首帧应直接取目标 %v，得到 %v", colA, samples[0])
	}

	samples = nil
	cur = colB
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("换目标后探针视图没有被构建")
	}
	if samples[0] == colB {
		t.Errorf("换目标的当帧就跳到新值 %v —— 没走 140ms 过渡", colB)
	}
	if samples[0] != colA {
		t.Errorf("过渡应从上一帧 %v 起步，得到 %v", colA, samples[0])
	}
}

// fadeProbe 换桩 drawerFadeColor，渲染一帧并把「键名 → 本帧目标色」记下来。
func fadeProbe(t *testing.T, seed func(a *App)) map[string]ui.Color {
	t.Helper()
	orig := drawerFadeColor
	t.Cleanup(func() { drawerFadeColor = orig })
	got := map[string]ui.Color{}
	drawerFadeColor = func(c *ui.Context, b ui.Element, key string, target ui.Color, a ui.Animation) ui.Color {
		got[key] = target
		return orig(c, b, key, target, a)
	}
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	seed(a)
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 720)
	tst.Frame()
	return got
}

// TestReplyRowTransitionWired 无头反证：评论行的底色 / 描边真的经过
// drawerFadeColor（键 cm-row-bg / cm-row-border），且基态值与原版一致
// （白 .32 底 + 白 .32 边）。删掉这两处调用（直接铺 paint）即变红。
func TestReplyRowTransitionWired(t *testing.T) {
	got := fadeProbe(t, func(a *App) {
		a.Drawer = DrawerDanmaku
		a.DanmakuTab = TabReply
		a.Comments = []Comment{{
			User: "甲", Content: "正文", Time: "2024-01-01 00:00", Level: 3,
			Replies: []Comment{{User: "乙", Content: "楼中楼"}},
		}}
	})
	if v, ok := got["cm-row-bg"]; !ok {
		t.Fatal("评论行底色没经过 drawerFadeColor（键 cm-row-bg 未记录）")
	} else if v != ui.Hex("#ffffff").Alpha(0.32) {
		t.Errorf("评论行基态底色 = %v, want 白 .32", v)
	}
	if v, ok := got["cm-row-border"]; !ok {
		t.Fatal("评论行描边没经过 drawerFadeColor（键 cm-row-border 未记录）")
	} else if v != ui.Hex("#ffffff").Alpha(0.32) {
		t.Errorf("评论行基态描边 = %v, want 白 .32", v)
	}
}

// TestDanmakuChipTransitionWired 无头反证：弹幕小胶囊的底色 / 描边经过
// drawerFadeColor（键 dmg-chip-bg / dmg-chip-border），基态值对齐原版
// .danmaku-message-row 的白 .48 底 + 白 .52 边。
func TestDanmakuChipTransitionWired(t *testing.T) {
	got := fadeProbe(t, func(a *App) {
		a.Drawer = DrawerDanmaku
		a.DanmakuTab = TabDanmaku
		a.Danmaku = []Danmaku{{Time: 0, Text: "哈哈"}, {Time: 30, Text: "高能"}}
	})
	if v, ok := got["dmg-chip-bg"]; !ok {
		t.Fatal("弹幕胶囊底色没经过 drawerFadeColor（键 dmg-chip-bg 未记录）")
	} else if v != ui.Hex("#ffffff").Alpha(0.48) {
		t.Errorf("弹幕胶囊基态底色 = %v, want 白 .48", v)
	}
	if v, ok := got["dmg-chip-border"]; !ok {
		t.Fatal("弹幕胶囊描边没经过 drawerFadeColor（键 dmg-chip-border 未记录）")
	} else if v != ui.Hex("#ffffff").Alpha(0.52) {
		t.Errorf("弹幕胶囊基态描边 = %v, want 白 .52", v)
	}
}

// TestDanmakuLabelAndSepTransitionWired 无头反证：弹幕时间标签的底色 / 墨色
// 与分组行的底部分隔线都经过 drawerFadeColor；非当前组标签底色是透明
// （原版没有背景），分隔线是灰蓝 .14。
func TestDanmakuLabelAndSepTransitionWired(t *testing.T) {
	got := fadeProbe(t, func(a *App) {
		a.Drawer = DrawerDanmaku
		a.DanmakuTab = TabDanmaku
		a.Danmaku = []Danmaku{{Time: 0, Text: "哈哈"}}
		a.Pos = -1 // 不标当前组
	})
	if v, ok := got["dmg-label-bg"]; !ok {
		t.Fatal("弹幕时间标签底色没经过 drawerFadeColor（键 dmg-label-bg 未记录）")
	} else if v != ui.Transparent {
		t.Errorf("非当前组标签底色 = %v, want 透明", v)
	}
	if v, ok := got["dmg-label-ink"]; !ok {
		t.Fatal("弹幕时间标签墨色没经过 drawerFadeColor（键 dmg-label-ink 未记录）")
	} else if v != ui.Hex("#334155") {
		t.Errorf("非当前组标签墨色 = %v, want #334155", v)
	}
	if v, ok := got["dmg-sep"]; !ok {
		t.Fatal("弹幕分组分隔线没经过 drawerFadeColor（键 dmg-sep 未记录）")
	} else if v != ui.Hex("#64748b").Alpha(0.14) {
		t.Errorf("弹幕分组分隔线基态 = %v, want 灰蓝 .14", v)
	}
}
