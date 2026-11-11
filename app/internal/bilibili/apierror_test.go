package bilibili

import (
	"errors"
	"strings"
	"testing"
)

// TestAPIErrorNotLoggedIn 钉住 -101（账号未登录 / 登录过期）与其它 code 的分流。
//
// 用户 2026-11-11 报「动态列表加载报错，登录了也还是报错」：根因是 SESSDATA
// 过期、接口回 -101，而这里此前一律返回 "API returned non-zero code" —— -101
// 这个关键信号被丢掉了，app 层没法据此收登录态、用户也只能看到「请稍后重试」。
func TestAPIErrorNotLoggedIn(t *testing.T) {
	err := apiError(-101, "账号未登录")
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("apiError(-101) = %v，应为 ErrNotLoggedIn", err)
	}
	if got := err.Error(); !strings.Contains(got, "登录") {
		t.Errorf("ErrNotLoggedIn 文案 = %q，应提到登录", got)
	}
	// 其它 code：带上 code 与上游文案（B 站的 message 基本是中文，failReason 原样透出）。
	other := apiError(-404, "啥都不存在")
	if errors.Is(other, ErrNotLoggedIn) {
		t.Error("非 -101 的 code 不该被当成登录过期")
	}
	for _, want := range []string{"-404", "啥都不存在"} {
		if !strings.Contains(other.Error(), want) {
			t.Errorf("apiError(-404) = %q，应含 %q", other, want)
		}
	}
	// 上游没给文案时也要能看懂是哪个 code。
	if got := apiError(-352, "").Error(); !strings.Contains(got, "-352") {
		t.Errorf("空文案的错误 = %q，应含 code", got)
	}
}

// TestSessionVerdict 钉住启动校验的判定：只有 nav 的 0 / -101 才算结论，
// 风控等其它 code 不能把用户登出（网络抖一下不该丢登录态）。
func TestSessionVerdict(t *testing.T) {
	cases := []struct {
		code         int
		valid, known bool
	}{
		{0, true, true},
		{-101, false, true},
		{-352, false, false}, // 风控校验失败：不是「没登录」，也不当有效
		{-400, false, false},
		{61000, false, false},
	}
	for _, c := range cases {
		valid, known := sessionVerdict(c.code)
		if valid != c.valid || known != c.known {
			t.Errorf("sessionVerdict(%d) = (%v,%v)，应为 (%v,%v)", c.code, valid, known, c.valid, c.known)
		}
	}
}
