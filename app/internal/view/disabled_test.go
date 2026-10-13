package view

import "testing"

// TestDisabledDim 钉住「框架禁用节点再乘 .5」的抵消换算（见 disabled.go 的总说明）：
// 目标 ≤ .5 时写 target/0.5 抵消；目标 > .5 时封顶 1（补不回来，调用点必须改用
// 「自己挡住交互」的办法）。
func TestDisabledDim(t *testing.T) {
	for _, tc := range []struct {
		target, want float32
	}{
		{navIconDisabledOpacity, 0.68},      // .34 / .5
		{playerButtonDisabledOpacity, 0.84}, // .42 / .5
		{0.34, 0.68},
		{0.42, 0.84},
		{0.5, 1},  // 边界：正好 .5 也算「补不回来」
		{0.55, 1}, // wl-remove 的 .55
		{0.82, 1}, // search-submit 的 .82
	} {
		if got := disabledDim(tc.target); got != tc.want {
			t.Errorf("disabledDim(%v) = %v，应为 %v", tc.target, got, tc.want)
		}
	}
}

// TestDisabledOpacityTargets 钉住两族「禁用时该落到画布上的绝对值」：
// `.nav-icon-btn:disabled` = .34、`#player .player-button:disabled` = .42，
// 且都经 disabledDim 换算成要写在元素上的值（.68 / .84）。
func TestDisabledOpacityTargets(t *testing.T) {
	if got := navIconOpacityTarget(true, 0.82); got != disabledDim(0.34) {
		t.Errorf("nav 禁用目标 = %v，应为 disabledDim(.34)", got)
	}
	// 非禁用时直接返回常态度。
	if got := navIconOpacityTarget(false, 0.82); got != 0.82 {
		t.Errorf("nav 常态目标 = %v，应为 .82", got)
	}
	if navIconDisabledOpacity != 0.34 {
		t.Errorf("nav 禁用绝对值 = %v，应为 .34", navIconDisabledOpacity)
	}
	if playerButtonDisabledOpacity != 0.42 {
		t.Errorf("播放按钮禁用绝对值 = %v，应为 .42", playerButtonDisabledOpacity)
	}
}

// TestExceptionsAboveHalf 钉住两处「目标 > .5、不能借框架 Disabled」的例外：
// 搜索提交键搜索中仍是常态度 .82（不改不透明度）；稍后再看移除键请求中是 .55。
func TestExceptionsAboveHalf(t *testing.T) {
	if got := wlRemoveOpacity(true); got != 0.55 {
		t.Errorf("稍后再看移除键请求中透明度 = %v，应为 .55", got)
	}
	if got := wlRemoveOpacity(false); got != 1 {
		t.Errorf("稍后再看移除键常态透明度 = %v，应为 1", got)
	}
	// 这两处的原版绝对值都 > .5，disabledDim 只能补到 1（= 实际仍会到 .5），
	// 所以调用点不能走框架 Disabled —— 用 disabledDim 的结果恰好暴露这一点。
	if disabledDim(wlRemoveOpacity(true)) != 1 {
		t.Errorf("disabledDim(.55) 应封顶 1（说明补不回来，需改自挡交互）")
	}
	if disabledDim(0.82) != 1 {
		t.Errorf("disabledDim(.82) 应封顶 1（说明补不回来，需改自挡交互）")
	}
}
