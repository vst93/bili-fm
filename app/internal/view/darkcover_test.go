package view

import (
	"image"
	"image/color"

	"github.com/egoist/mygo/ui"
)

// darkCoverBitmap 是一张纯深色（#0f172a）的假封面位图。测量封面几何的测试
// 用它可以不依赖「无封面时的兜底 logo」——兜底图已按原版换成白底（见
// coverlogo_test.go），白底没法做深色像素测量。纯色深底让 #0f172a 的测量阈值
// （measureCover / measureNavy）稳定生效，同时保持封面形状（圆形 / 圆角方块）。
func darkCoverBitmap() *ui.Bitmap {
	const n = 300
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			img.Set(x, y, color.RGBA{15, 23, 42, 255})
		}
	}
	return ui.NewBitmap(img)
}

// giveDarkCover 给 app 装一张确定的深色封面（走 coverURL → Images.Bitmap），
// 让封面几何测量与兜底 logo 解耦。同时关掉「封面背景」（氛围光）——它会把同一
// 张深色封面模糊铺满主区，测封面几何时会混进大片深色像素。
func giveDarkCover(a *App) *App {
	if a.Track == nil {
		a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	}
	a.Track.Cover = darkCoverURL
	a.Ambient = false
	a.Images = &darkCoverStore{bmp: darkCoverBitmap()}
	return a
}

const darkCoverURL = "test-cover"

// darkCoverStore 是只对封面 URL 返回深色位图的假 Images；其他地址（头像等）
// 返回 nil，免得它们的测量混进来。实现 ImageStore。
type darkCoverStore struct{ bmp *ui.Bitmap }

func (s *darkCoverStore) Bitmap(url string, _ int) *ui.Bitmap {
	if url == darkCoverURL {
		return s.bmp
	}
	return nil
}
func (s *darkCoverStore) Get(url string, w int) *ui.Bitmap { return s.Bitmap(url, w) }
func (s *darkCoverStore) Touch(string, int)                {}
func (s *darkCoverStore) Retain(string, int)               {}
func (s *darkCoverStore) Cancel(string, int)               {}
func (s *darkCoverStore) Ambient(string, int, bool) *ui.Bitmap {
	return nil
}
func (s *darkCoverStore) Pending() int { return 0 }
