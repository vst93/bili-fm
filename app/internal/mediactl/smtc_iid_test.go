package mediactl

import "testing"

// TestTypedEventHandlerIID 用 Windows 已知的 SMTC ButtonPressed 事件 IID 钉住
// 参数化接口的 GUID 生成规则。写错一个字节，Add_ButtonPressed 会静默失败，
// 媒体键/系统面板上的按钮就完全没反应。
func TestTypedEventHandlerIID(t *testing.T) {
	got := typedEventHandlerIID(
		"Windows.Media.SystemMediaTransportControls",
		"99fa3ff4-1742-42a6-902e-087d41f965ec",
		"Windows.Media.SystemMediaTransportControlsButtonPressedEventArgs",
		"b7f47116-a56f-4dc8-9e11-92031f4a87c2",
	)
	const want = "{0557e996-7b23-5bae-aa81-ea0d671143a4}"
	if got != want {
		t.Fatalf("TypedEventHandler IID = %s，want %s", got, want)
	}
}
