package view

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// 抽屉列表卡片 / 播放列表行封面未就绪时的占位图。
//
// 原版 ListCard 的 RetryImg 有 fallbackSrc="/cover.png"（public/cover.png），
// 拿不到真图时画它。原生版此前只铺一层很淡的 Theme.CoverPlaceholder() 底色
// （8% 墨，在白色抽屉面板上几乎看不见），用户报「图片要等下载完才出现、
// 之前是一片空」。这里把同一张占位图烘进位图：封面盒先画占位图，真图
// 解码完成后再由 ui.Image 盖上去（渐进式加载）。
//
//go:embed assets/cover.png
var coverPlaceholderPNG []byte

// placeholderCover 是占位图位图，解码失败时为 nil（那就退回底色）。
var placeholderCover = func() *ui.Bitmap {
	bmp, err := ui.DecodeBitmap(coverPlaceholderPNG)
	if err != nil {
		return nil
	}
	return bmp
}()
