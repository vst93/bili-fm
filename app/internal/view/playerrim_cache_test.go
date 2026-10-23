package view

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 聚光描边的路径缓存（playerbar.go 的 rimPathCache）。
//
// 为什么要钉住「零分配」：`Draw` / `DrawOver` 回调在每帧都跑，而玩家最常看到
// 的连续动画（唱片旋转、氛围光淡入淡出）走的是**只重绘不重建**的 repose 路径
// —— 界面元素不重建，但描边路径原来每帧都重新 append 一遍（一圈 = 20 条
// pathCmd，大表面还要拆 12 块）。无头 profile 里这部分曾占总分配量 ~35%。

func rimPartBounds(r ui.Rect, radius float32) []ui.Rect {
	parts := playerRimRingParts(r, radius, 1)
	out := make([]ui.Rect, len(parts))
	for i := range parts {
		out[i] = parts[i].Bounds()
	}
	return out
}

// TestPlayerRimPathCached 稳态重绘时路径必须命中缓存、零分配。
func TestPlayerRimPathCached(t *testing.T) {
	r := ui.Rect{X: 20, Y: 40, W: 320, H: 56}
	// 预热：第一次调用负责建路径。
	_ = rimRingPath(r, 12, 1)
	_ = rimRingParts(r, 12, 1)

	if got := testing.AllocsPerRun(20, func() {
		p := rimRingPath(r, 12, 1)
		_ = p.Bounds()
	}); got != 0 {
		t.Fatalf("整圈环路径命中缓存后仍分配：%.2f allocs/op", got)
	}
	if got := testing.AllocsPerRun(20, func() {
		parts := rimRingParts(r, 12, 1)
		for i := range parts {
			_ = parts[i].Bounds()
		}
	}); got != 0 {
		t.Fatalf("拆块环路径命中缓存后仍分配：%.2f allocs/op", got)
	}
}

// TestPlayerRimPathCacheKeyedByGeometry 键必须含 rect / radius / width：漏掉
// 任何一项，就会把上一个控件的描边画到新控件上（位置、尺寸、圆角都要区分）。
func TestPlayerRimPathCacheKeyedByGeometry(t *testing.T) {
	base := ui.Rect{X: 10, Y: 20, W: 100, H: 40}
	want := rimPartBounds(base, 8)
	if len(want) == 0 {
		t.Fatal("拆块路径为空，测试前提不成立")
	}
	for _, tc := range []struct {
		name   string
		r      ui.Rect
		radius float32
	}{
		{"位置", ui.Rect{X: 30, Y: 20, W: 100, H: 40}, 8},
		{"尺寸", ui.Rect{X: 10, Y: 20, W: 140, H: 40}, 8},
		{"圆角", base, 14},
	} {
		if got := rimPartBounds(tc.r, tc.radius); slices.Equal(got, want) {
			t.Errorf("%s 变了，但拆块路径的包围盒完全相同：%v", tc.name, got)
		}
	}
	// 同一几何必须给出同一份缓存（不是每次重建出「碰巧一样」的路径）。
	if got := rimPartBounds(base, 8); !slices.Equal(got, want) {
		t.Errorf("相同几何两次取到的路径不一致：%v != %v", got, want)
	}
}

// TestPlayerRimPathCacheBounded 缓存必须封顶：否则拖动窗口 / 缩放时会按每个
// 中途尺寸各留一条，内存无界增长。
func TestPlayerRimPathCacheBounded(t *testing.T) {
	rimRingPathCache.mu.Lock()
	rimRingPathCache.m = map[rimPathKey]ui.Path{}
	rimRingPathCache.mu.Unlock()
	rimRingPartsCache.mu.Lock()
	rimRingPartsCache.m = map[rimPathKey][]ui.Path{}
	rimRingPartsCache.mu.Unlock()
	t.Cleanup(func() {
		rimRingPathCache.mu.Lock()
		rimRingPathCache.m = map[rimPathKey]ui.Path{}
		rimRingPathCache.mu.Unlock()
		rimRingPartsCache.mu.Lock()
		rimRingPartsCache.m = map[rimPathKey][]ui.Path{}
		rimRingPartsCache.mu.Unlock()
	})

	for i := 0; i < rimPathCacheLimit*2; i++ {
		r := ui.Rect{X: float32(i), Y: 0, W: 40, H: 20}
		_ = rimRingPath(r, 6, 1)
		_ = rimRingParts(r, 6, 1)
	}
	if got := rimRingPathCache.len(); got > rimPathCacheLimit {
		t.Errorf("整圈环缓存无界增长：%d 条（上限 %d）", got, rimPathCacheLimit)
	}
	if got := rimRingPartsCache.len(); got > rimPathCacheLimit {
		t.Errorf("拆块环缓存无界增长：%d 条（上限 %d）", got, rimPathCacheLimit)
	}
}
