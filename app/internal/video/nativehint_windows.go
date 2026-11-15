package video

// 平台相关的「缺什么 / 怎么修」，入口在 native.go 的 nativeFallbackPrompt。
//
// Windows 的解码靠系统自带的 Media Foundation：N 版 / 精简版 Windows 需要
// 额外装「媒体功能包」，很老的系统可能根本没有 H.264、AAC 解码器。用户能
// 做的是装功能包 / 升系统，修不了就换 WebView 播放。

func nativeMissingText() string {
	return "系统媒体组件（Media Foundation）不可用，或缺少 H.264 / AAC 解码器"
}

func nativeFixHint() string {
	return "修复：N 版 Windows 请安装系统「媒体功能包」，或更新到 Windows 10 / 11 后" +
		"重启应用再试。如果仍然不行，可以用下方的「切换到 WebView 播放」（由浏览器引擎解码）。"
}
