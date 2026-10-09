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

// Logo 是标题栏用的品牌位图，解码失败时为 nil（那就只显示文字）。
var Logo = func() *ui.Bitmap {
	bmp, err := ui.DecodeBitmap(logoPNG)
	if err != nil {
		return nil
	}
	return bmp
}()
