package view

import (
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/view/fonts"
)

// 三平台统一字体。原版是 WebView，字体由 CSS 决定；原生版此前没设
// `ui.Theme.Font`，于是每个平台各用各的系统字体：
//
//	macOS    SF Pro Text（拉丁）+ PingFang SC（中文回退）
//	Windows  Segoe UI（拉丁）+ Microsoft YaHei（中文回退）
//	Linux    GTK 的界面字体（Cantarell / Ubuntu / Noto Sans…）+ Noto Sans CJK
//
// 同一个界面因此在三平台的**字宽、字重、自然行盒**上都不一样。而本仓库大量
// 布局按真实字体度量来算（linebox.go 的 naturalLineBox、桌面字幕高度、
// 弹幕/楼中楼/气泡的宽度与行距），度量一漂，同一个常量在三平台就对不上。
//
// 统一方案，以 macOS 的观感为基准：
//
//   - 拉丁 / 数字 / 符号用 Inter（SIL OFL，可随包分发）：比例与 SF Pro 同族
//     （14px 自然行盒 17px，SF 约 16.3px），明显比 Segoe UI（19px）和
//     Noto Sans CJK（22px）更接近基准。五个字重覆盖本仓库实际用到的
//     400 / 500 / 600 / 650 / 700 / 800。
//   - 中文仍取各系统自己的界面字体，但回退顺序钉死为 macOS 的 PingFang SC
//     优先，其次 Windows 的 Microsoft YaHei UI、Linux 的 Noto Sans CJK SC。
//     表意字都是全角方块（advance 恒为 1em），换回退字体不会横向错位。
//
// 为什么不把中文也打包：Noto Sans SC 单字重完整集 ~10MB（GB2312 子集也要
// ~3.4MB），三个字重就把包体翻倍；Inter 的拉丁子集只有 ~245KB/字重。
// 真要逐像素统一中文，再把 Noto Sans SC 子集加进来即可，回退顺序不用动。
//
// 字体文件与注册在子包 fonts 里：注册必须早于本包的包级变量初始化（见
// fonts 包的说明）。这个常量因此可以放心给任何 var 初始化用。
const fontStack = fonts.Family + ", PingFang SC, Microsoft YaHei UI, Noto Sans CJK SC, Noto Sans SC, sans-serif"

// fontOf 返回统一字体栈下某个字号 / 字重的 ui.Font。凡是需要自己成型
// （ui.ShapeText）或量宽（textWidth）的地方都必须用它，不能写
// `ui.Font{Size: n}` —— 后者不带 Family，会悄悄用回系统字体。
func fontOf(size float32, weight int) ui.Font {
	return ui.Font{Family: fontStack, Size: size, Weight: weight}
}
