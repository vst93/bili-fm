package view

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// 标题栏里的品牌标志。与前端用的是同一张图
// （public/logo-transparent.png，原版 .app-title-bar img 是 24×24）。
//
//go:embed assets/logo.png
var logoPNG []byte

// Logo 是标题栏用的品牌位图（透明底），解码失败时为 nil（那就只显示文字）。
var Logo = func() *ui.Bitmap {
	bmp, err := ui.DecodeBitmap(logoPNG)
	if err != nil {
		return nil
	}
	return bmp
}()

// logoCoverPNG 是封面缺失时的兜底图（原版 cover || "/logo.png"）。它与标题栏
// 那张**是两张图**：标题栏用透明底（public/logo-transparent.png），兜底封面用
// **白底**（public/logo.png）—— 封面盘面的底色是深色（#0f172a），透明底会透出
// 黑底，用户看到的就是「默认封面黑底」。这里嵌入白底图单独给封面用。
//
//go:embed assets/logo-cover.png
var logoCoverPNG []byte

// LogoCover 是封面缺失时用的白底品牌位图，解码失败时为 nil（兜底退回 Logo）。
var LogoCover = func() *ui.Bitmap {
	bmp, err := ui.DecodeBitmap(logoCoverPNG)
	if err != nil {
		return nil
	}
	return bmp
}()

// coverLogo 返回封面缺失时该铺的位图：白底 LogoCover 优先，解码失败时退回
// 透明的 Logo（至少还有台标）。标题栏仍只用 Logo。
func coverLogo() *ui.Bitmap {
	if LogoCover != nil {
		return LogoCover
	}
	return Logo
}
