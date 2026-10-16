package view

import (
	"image"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（操作行计数的行盒 / 字重）的测试。
//
// 原版 `.nav-icon-btn > .nav-stat-value`（globals.css:759 起）里的计数是
// `font-size: 9px; line-height: 12px; font-weight: 600`。前三项原生版此前
// 都没接：框架把每行夹在 `max(指定行高, ascent+descent)` 之上，9px 文本的
// 自然行盒（本机 Pango 实测 ascent 11 + descent 3 = 14）压不下去 —— 计数盒
// 因此比原版高 2px、数字落点低 1px；字重也走 numericText 的默认 400，
// 比原版轻一档。计数不占布局（见 stataction_test.go），所以这些只体现在
// 计数自身的盒子与墨迹上。

// statCountInkY 返回计数文字在整窗渲染图里的墨迹**纵向重心**（按钮正下方采样带
// 里暗于底色的像素的平均 y）。用重心而不是包围盒：字形带 −1px 的 transform 位移
// （见 statCountShift），而 9px 字号的墨迹行高只有 7 行 —— 位移 1px 会让重心整体
// 平移，但上下边缘的暗像素强弱差得不多，包围盒的边界会黏在采样窗上。
func statCountInkY(t *testing.T, img *image.RGBA, btnR ui.Rect) float64 {
	t.Helper()
	// 采样窗：按钮正下方 1px 起、12px 高的带（原版展示区），x 只取按钮中间
	// 一半 —— 计数字形居中（left 50% + translateX(-50%)），避开按钮圆角与
	// 下缘投影那两处本来就暗的像素。
	x0, x1 := int(btnR.X+btnR.W/4), int(btnR.X+3*btnR.W/4)
	y0, y1 := int(btnR.Y+btnR.H+1), int(btnR.Y+btnR.H+14)
	sum, n := 0.0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			o := img.PixOffset(x, y)
			if (int(img.Pix[o])+int(img.Pix[o+1])+int(img.Pix[o+2]))/3 < 200 {
				sum += float64(y)
				n++
			}
		}
	}
	if n == 0 {
		t.Fatal("计数墨迹一个暗像素都没有，采样窗选错了")
	}
	return sum / float64(n)
}

// TestStatCountBoxHeight 钉住计数盒高 = 原版 `line-height: 12px`（框架自然行盒
// 是 14，压不到）。
func TestStatCountBoxHeight(t *testing.T) {
	tt := shellTesterSize(t, statCountApp(), 1000, 700)
	for _, count := range []string{"1234", "5678", "9012"} {
		r, ok := tt.Find(count)
		if !ok {
			t.Fatalf("找不到计数 %q", count)
		}
		t.Logf("计数 %q rect = %+v", count, r)
		if r.H != statCountBoxH {
			t.Errorf("计数 %q 盒高 %v，want %v（原版 .nav-stat-value 的 line-height: 12px）",
				count, r.H, statCountBoxH)
		}
	}
	if statCountBoxH != 12 {
		t.Errorf("statCountBoxH = %v，应为原版 line-height 的 12", statCountBoxH)
	}
}

// TestStatCountShiftMatchesLineBox 钉住字形位移：等于 pxLineBox 算出的负
// half-leading ——（原版行盒 − 框架自然行盒）/ 2 = (12 − 14)/2 = −1。
func TestStatCountShiftMatchesLineBox(t *testing.T) {
	h, sh := pxLineBox(9, 12)
	if h != statCountBoxH || sh != statCountShift {
		t.Fatalf("statCountBoxH/Shift = (%v, %v)，应为 pxLineBox(9,12) 的 (%v, %v)",
			statCountBoxH, statCountShift, h, sh)
	}
	if want := (float32(12) - naturalLineBox(9)) / 2; sh != want {
		t.Errorf("位移 %v，want %v（(12 − 自然行盒)/2）", sh, want)
	}
}

// TestStatCountInkFollowsShift 是无头像素实测：字形墨迹的纵向重心跟着
// statCountShift 走 —— 位移归零后重心应整体下移 1px（|shift|），盒高不变。
func TestStatCountInkFollowsShift(t *testing.T) {
	measure := func() float64 {
		tt := shellTesterSize(t, statCountApp(), 1000, 700)
		btn, ok := tt.Find("点赞")
		if !ok {
			t.Fatal("找不到点赞按钮")
		}
		return statCountInkY(t, tt.Image(), btn)
	}
	// statCountShift 为负 ⇒ 字形上移。
	if statCountShift >= 0 {
		t.Fatalf("statCountShift = %v，应为负（字形上移）", statCountShift)
	}
	shifted := measure()
	old := statCountShift
	statCountShift = 0
	defer func() { statCountShift = old }()
	unshifted := measure()
	dy := unshifted - shifted
	t.Logf("墨迹重心：有位移 y=%.2f，位移归零 y=%.2f，差 %.2f", shifted, unshifted, dy)
	if dy < 0.9 || dy > 1.1 {
		t.Errorf("位移归零后墨迹重心下移 %.2f，want 1（shift = %v）", dy, statCountShift)
	}
}

// TestStatCountLineBoxWired 是反证：去掉显式盒高，计数退回框架自然行盒 14。
func TestStatCountLineBoxWired(t *testing.T) {
	measure := func() float32 {
		tt := shellTesterSize(t, statCountApp(), 1000, 700)
		r, ok := tt.Find("1234")
		if !ok {
			t.Fatal("找不到计数 1234")
		}
		return r.H
	}
	withBox := measure()
	old := statCountBoxH
	statCountBoxH = naturalLineBox(9) // 框架自然行盒（旧行为）
	defer func() { statCountBoxH = old }()
	natural := measure()
	t.Logf("计数盒高：显式 12 → %v，自然行盒 → %v", withBox, natural)
	if withBox != 12 {
		t.Errorf("显式盒高未生效：%v，want 12", withBox)
	}
	if natural == 12 {
		t.Errorf("换回自然行盒后盒高仍是 12，说明盒高不由 statCountBoxH 驱动")
	}
}

// TestStatCountWeight 钉住原版同一条规则里的 `font-weight: 600`。
func TestStatCountWeight(t *testing.T) {
	if statCountWeight != 600 {
		t.Fatalf("statCountWeight = %d，应为原版 .nav-stat-value 的 600", statCountWeight)
	}
}

// TestStatCountWeightWired 是字重的**接线反证**：600 与 400 在按钮下方的采样带里
// 渲染出不同的像素。不比较「深色像素更多」——界面字体在小字号下 600 的字面反而
// 比 400 窄（实测同一文案的 label 宽 19 vs 20），暗像素数不单调。
func TestStatCountWeightWired(t *testing.T) {
	pixels := func() []uint8 {
		tt := shellTesterSize(t, statCountApp(), 1000, 700)
		// 采样窗锚在**按钮**上（不按计数矩形）：字重换档后计数的宽 / 位置会变，
		// 拿计数矩形做窗会让两次采样覆盖不同区域。
		btn, ok := tt.Find("点赞")
		if !ok {
			t.Fatal("找不到点赞按钮")
		}
		img := tt.Image()
		var out []uint8
		for y := int(btn.Y + btn.H - 2); y < int(btn.Y+btn.H+16); y++ {
			for x := int(btn.X); x < int(btn.X+btn.W); x++ {
				o := img.PixOffset(x, y)
				out = append(out, img.Pix[o], img.Pix[o+1], img.Pix[o+2])
			}
		}
		return out
	}
	bold := pixels()
	old := statCountWeight
	statCountWeight = 400
	defer func() { statCountWeight = old }()
	regular := pixels()
	if len(bold) != len(regular) {
		t.Fatalf("两次采样长度不同：%d vs %d", len(bold), len(regular))
	}
	diff := 0
	for i := range bold {
		if bold[i] != regular[i] {
			diff++
		}
	}
	t.Logf("字重 600 vs 400 的计数区域像素差：%d 个通道", diff)
	if diff == 0 {
		t.Error("字重没有接线：600 与 400 渲染出的计数区域逐像素相同")
	}
}

// TestStatCountWiredInSource 钉住接线链（无头 ui.Tester 不导出字重/行高，只能在源码
// 上逐字匹配，同 a11ystate_test.go 的手法）：计数必须同时吃到 statCountBoxH /
// statCountShift / statCountWeight。
func TestStatCountWiredInSource(t *testing.T) {
	flat := flatten(t, "home.go")
	chain := strings.Join(strings.Fields(
		`numericText(c, spec.Count).FontSize(9).FontWeight(statCountWeight).TextColor(valueInk).
			Opacity(navIconStatValueOpacity(spec.Off)).SingleLine().
			Height(statCountBoxH).Translate(0, statCountShift).
			Attach(ui.AnchorBottom, ui.AnchorTop).Top(1)`), "")
	if !strings.Contains(flat, chain) {
		t.Error("操作行计数没接上 statCountWeight / statCountBoxH / statCountShift")
	}
	// 常量的换算必须来自 pxLineBox（原版是写死的像素行高，不是字号倍数）。
	if !strings.Contains(flat, `varstatCountBoxH,statCountShift=pxLineBox(9,12)`) {
		t.Error("statCountBoxH/Shift 应 = pxLineBox(9, 12)（原版 line-height 是写死的 12px）")
	}
}
