package bilibili

import (
	"net/url"
	"testing"
)

func TestWBIMixinKey(t *testing.T) {
	got := wbiMixinKey("abcdefghijklmnopqrstuvwxyz123456", "ABCDEFGHIJKLMNOPQRSTUVWXYZ654321")
	if got != "OPscVixApSk66dND2LfRBjKt43oHmGJn" {
		t.Fatalf("mixin key = %q", got)
	}
}

func TestSignWBIAt(t *testing.T) {
	values := url.Values{}
	values.Set("foo", "value!'()*")
	values.Set("bar", "space value")
	got := signWBIAt(values, "abcdefghijklmnopqrstuvwxyz123456", "ABCDEFGHIJKLMNOPQRSTUVWXYZ654321", 1700000000)
	if got.Get("foo") != "value" || got.Get("bar") != "space value" {
		t.Fatalf("参数过滤错误: %v", got)
	}
	if got.Get("wts") != "1700000000" {
		t.Fatalf("wts = %q", got.Get("wts"))
	}
	if got.Get("w_rid") != "e0bbf3c23838f46f9b7b2785d19dd01e" {
		t.Fatalf("w_rid = %q", got.Get("w_rid"))
	}
}
