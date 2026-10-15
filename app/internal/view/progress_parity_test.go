package view

import (
	"os"
	"strings"
	"testing"
)

// 进度条区域对齐原版的三项差异修复：
// 1. disabled opacity 对齐（原版 0.42）
// 2. 滑块 scale 过渡（原版 100ms ease）
// 3. 滑块高光值对齐（亮色 0.98 / 深色 0.9）

func TestProgressDisabledOpacityAligned(t *testing.T) {
	got := disabledDim(progressDisabledOpacity)
	want := float32(0.84) // 0.42 / 0.5（框架 disabled 自动乘 0.5）
	if abs(float64(got-want)) > 0.001 {
		t.Errorf("disabledDim(%v) = %v, want %v", progressDisabledOpacity, got, want)
	}
}

func TestThumbScaleTransitionDuration(t *testing.T) {
	if thumbScaleAnim.Duration != 100*1e6 {
		t.Errorf("thumbScaleAnim.Duration = %v, want 100ms", thumbScaleAnim.Duration)
	}
}

func TestThumbGlintValuesAligned(t *testing.T) {
	light := rangeSkinForTheme(false, false)
	wantLight := uint8(250) // 0.98 * 255 ≈ 249.9
	if abs(float64(light.ThumbGlint.A)-float64(wantLight)) > 1 {
		t.Errorf("light ThumbGlint.A = %v, want ~%v (0.98)", light.ThumbGlint.A, wantLight)
	}
	dark := rangeSkinForTheme(true, false)
	wantDark := uint8(230) // 0.90 * 255 = 229.5
	if abs(float64(dark.ThumbGlint.A)-float64(wantDark)) > 1 {
		t.Errorf("dark ThumbGlint.A = %v, want ~%v (0.90)", dark.ThumbGlint.A, wantDark)
	}
}

func TestProgressUsesThumbScaleAnimationAndDisabledOpacity(t *testing.T) {
	body, err := os.ReadFile("playerbar.go")
	if err != nil {
		t.Fatalf("读取 playerbar.go 失败: %v", err)
	}
	src := string(body)

	progressStart := strings.Index(src, "func (a *App) progress(")
	if progressStart < 0 {
		t.Fatal("找不到 progress 函数")
	}
	progressEnd := strings.Index(src[progressStart:], "\nfunc ")
	progressBody := src[progressStart : progressStart+progressEnd]

	if !strings.Contains(progressBody, "AnimateTo") {
		t.Error("progress 函数中缺少 AnimateTo 调用（滑块 scale 过渡）")
	}
	if !strings.Contains(progressBody, "thumbScale") {
		t.Error("progress 函数中缺少 thumbScale 动画状态更新")
	}
	if !strings.Contains(progressBody, "progressDisabledOpacity") {
		t.Error("progress 函数中缺少禁用态 opacity 设置")
	}

	miniStart := strings.Index(src, "func (a *App) miniProgress(")
	if miniStart < 0 {
		t.Fatal("找不到 miniProgress 函数")
	}
	miniEnd := strings.Index(src[miniStart:], "\nfunc ")
	miniBody := src[miniStart : miniStart+miniEnd]

	if !strings.Contains(miniBody, "AnimateTo") {
		t.Error("miniProgress 函数中缺少 AnimateTo 调用（滑块 scale 过渡）")
	}
	if !strings.Contains(miniBody, "thumbScale") {
		t.Error("miniProgress 函数中缺少 thumbScale 动画状态更新")
	}
	if !strings.Contains(miniBody, "progressDisabledOpacity") {
		t.Error("miniProgress 函数中缺少禁用态 opacity 设置")
	}
}

func TestRangePaintHasThumbScaleField(t *testing.T) {
	body, err := os.ReadFile("rangeslider.go")
	if err != nil {
		t.Fatalf("读取 rangeslider.go 失败: %v", err)
	}
	src := string(body)
	if !strings.Contains(src, "ThumbScale") {
		t.Error("rangeslider.go 中 rangePaint 缺少 ThumbScale 字段")
	}
	if !strings.Contains(src, "rp.ThumbScale") {
		t.Error("rangeslider.go 中 paintRangeThumb 未使用 rp.ThumbScale")
	}
}

func TestTimelinePassesThumbScale(t *testing.T) {
	body, err := os.ReadFile("playerbar.go")
	if err != nil {
		t.Fatalf("读取 playerbar.go 失败: %v", err)
	}
	src := string(body)
	idx := strings.Index(src, "func paintTimeline(")
	if idx < 0 {
		t.Fatal("找不到 paintTimeline 函数")
	}
	end := strings.Index(src[idx:], "\nfunc ")
	if !strings.Contains(src[idx:idx+end], "ThumbScale") {
		t.Error("paintTimeline 中缺少 ThumbScale 传递给 paintRangeThumb")
	}
}
