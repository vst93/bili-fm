package video

// 平台相关的「缺什么 / 怎么修」，入口在 native.go 的 nativeFallbackPrompt。
//
// Linux 是唯一「用户自己装插件就能修好」的平台，所以这里给可直接复制的安装
// 命令，包名按实际解码需求给：
//
//   - playbin / appsink / audioconvert / videoconvert 在 gst-plugins-base；
//   - MP4 解复用（qtdemux）与渐进式 H.264/AAC 的常见解析器在 gst-plugins-good；
//   - H.264（avdec_h264）与 AAC（avdec_aac）解码器在 gst-libav。
//
// 少了其中任何一层，ui/video 的 OpenURL 都会返回 ErrUnsupported。

func nativeMissingText() string {
	return "缺少 GStreamer 1.x 的播放插件（playbin / appsink）或 H.264、AAC 解码器"
}

func nativeFixHint() string {
	return "修复（复制到终端执行，装完重启应用）：\n" +
		"  Debian / Ubuntu：sudo apt install gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-libav\n" +
		"  Fedora：sudo dnf install gstreamer1-plugins-base gstreamer1-plugins-good gstreamer1-libav\n" +
		"  Arch：sudo pacman -S gstreamer gst-plugins-base gst-plugins-good gst-libav"
}
