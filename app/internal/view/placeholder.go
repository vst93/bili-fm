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
// noFaceAvatar 是 UP 主没有头像时用的 B 站官方默认头像。
//
// 原版 videoInfo.tsx:253 的 RetryImg：
//
//	src={graftingImage(displayedOwner?.face || "https://i0.hdslb.com/bfs/face/member/noface.jpg", 96)}
//
// —— 不是画底色、也不是画首字，就是这张官方 noface 图（RetryImg 的 onError
// 再退 /cover.png 那一层，由 placeholderCover 兜）。地址原样、不带 @ 后缀，
// 交给 imagecache.ThumbURL 按 hdslb 规则加 @96w.webp，与 graftingImage 一致。
const noFaceAvatar = "https://i0.hdslb.com/bfs/face/member/noface.jpg"

// placeholderAvailable 报告占位图这次真的画得出来（解码成功）。只铺底色 /
// 不铺占位图的调用点要看它，免得让「占位图解码失败」这种构建事故静默通过
// 测试（issueImage 里真图 nil 会 NPE，属于同类事故）。
func placeholderAvailable() bool { return placeholderCover != nil }

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
