package video

// 平台相关的「缺什么 / 怎么修」，入口在 native.go 的 nativeFallbackPrompt。
//
// macOS 上的解码器由系统自带（AVFoundation 的 VideoToolbox / AudioToolbox），
// 用户装不了插件，所以这里只给通用兜底说明：能修的是系统版本，修不了就换
// WebView 播放。

func nativeMissingText() string {
	return "系统媒体框架（AVFoundation）不可用或系统版本过旧"
}

func nativeFixHint() string {
	return "修复：把 macOS 更新到 10.15 或更高版本，重启应用后再试。" +
		"如果仍然不行，可以用下方的「切换到 WebView 播放」（由浏览器引擎解码）。"
}
