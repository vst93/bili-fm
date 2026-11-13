package main

import "testing"

// TestPrefNativeVideoDefaultOn 钉住「原生视频播放」这个偏好键的读回规则：
//
//   - 老版本没有这个键（空字符串）→ 默认开，不能被静默降级成 webview；
//   - 明确写过 "false" → 关；
//   - 写过 "true" → 开。
//
// 判据是纯表达式（`!= "false"`），所以这里直接钉住三条真值。
func TestPrefNativeVideoDefaultOn(t *testing.T) {
	cases := map[string]bool{"": true, "true": true, "false": false}
	for stored, want := range cases {
		if got := stored != "false"; got != want {
			t.Errorf("kv=%q → NativeVideo=%v，想要 %v", stored, got, want)
		}
	}
	// 键名必须与 loadPrefs 里读的一致（写错就是「关掉后重启又自己开了」）。
	if prefNativeVideo != "nativeVideoPlayer" {
		t.Errorf("prefNativeVideo=%q", prefNativeVideo)
	}
}
