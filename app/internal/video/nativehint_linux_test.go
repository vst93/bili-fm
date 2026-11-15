package video

import "testing"

// TestNativeFixHintLinuxCommands 钉住 Linux 的「怎么修」：用户唯一能自己动手
// 的平台，必须给出可直接复制的命令，且包名覆盖实际解码需求
// （playbin/appsink 在 base、MP4 解复用与常见解析器在 good、H.264/AAC 解码器在
// libav）—— 只装 base 仍然播不了 H.264。
func TestNativeFixHintLinuxCommands(t *testing.T) {
	reason := nativeMissingText()
	for _, want := range []string{"GStreamer", "H.264", "AAC"} {
		if !contains(reason, want) {
			t.Errorf("「缺什么」没写清 %q：%q", want, reason)
		}
	}
	hint := nativeFixHint()
	for _, want := range []string{
		"sudo apt install gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-libav",
		"sudo dnf install gstreamer1-plugins-base gstreamer1-plugins-good gstreamer1-libav",
		"sudo pacman -S gstreamer gst-plugins-base gst-plugins-good gst-libav",
	} {
		if !contains(hint, want) {
			t.Errorf("安装命令缺少 %q：%q", want, hint)
		}
	}
	if !contains(hint, "重启应用") {
		t.Errorf("安装后要说明重启应用：%q", hint)
	}
}
