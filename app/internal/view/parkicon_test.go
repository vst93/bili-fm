package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestIconParkPathsAreVerbatim 逐字钉住五枚 icon-park 图标（搜索 / 动态 /
// 热门与推荐 / 收藏 / 历史）的原始 path 数据。
//
// 这几条字符串是从 @icon-park/react 1.4.2 的 icons/*.js 逐字拷来的（48×48
// 画布）。原版 searchForm.tsx / videoInfo.tsx / pageList.tsx 用的就是这几枚
// （theme="outline"），用户实测反馈「搜索 icon 不如以前、「热门/推荐」的图标和
// 原版对不上」就是之前用 16 画布手抄近似的结果。这里把原始数据钉死，谁再手改
// 就会红。
func TestIconParkPathsAreVerbatim(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"Search", parkSearch, `<path d="M21 38C30.3888 38 38 30.3888 38 21C38 11.6112 30.3888 4 21 4C11.6112 4 4 11.6112 4 21C4 30.3888 11.6112 38 21 38Z"/><path d="M26.657 14.3431C25.2093 12.8954 23.2093 12 21.0001 12C18.791 12 16.791 12.8954 15.3433 14.3431"/><path d="M33.2216 33.2217L41.7069 41.707"/>`},
		{"ShareSys", parkShareSys, `<path d="M10.3654 41.6322C6.44902 37.9812 4 32.7766 4 27C4 15.9543 12.9543 7 24 7C35.0457 7 44 15.9543 44 27C44 32.7766 41.551 37.9812 37.6346 41.6322"/><path d="M15.1375 36.5109C12.5919 34.1378 11 30.7548 11 27C11 19.8203 16.8203 14 24 14C31.1797 14 37 19.8203 37 27C37 30.7548 35.4081 34.1378 32.8625 36.5109"/><path d="M19.9096 31.3897C18.7347 30.2944 18 28.733 18 27C18 23.6863 20.6863 21 24 21C27.3137 21 30 23.6863 30 27C30 28.733 29.2653 30.2944 28.0904 31.3897"/>`},
		{"ChartRing", parkChartRing, `<path d="M43.7756 20.9938C42.4735 12.3555 35.6463 5.5277 27.0084 4.22461M20.9757 4.22702C11.3651 5.68478 4 13.9822 4 23.9998C4 34.0212 11.3705 42.321 20.9863 43.7743C21.9692 43.9228 22.9756 43.9998 24 43.9998C25.0209 43.9998 26.024 43.9233 27.0038 43.7758C35.6458 42.4741 42.4762 35.6427 43.7764 27.0003"/><path d="M24 16C19.5817 16 16 19.5817 16 24C16 28.4183 19.5817 32 24 32C28.4183 32 32 28.4183 32 24C32 19.5817 28.4183 16 24 16Z"/>`},
		{"WeixinFavorites", parkWeixinFavorites, `<path d="M6 14L24 6L42 14M6 14L24 22M6 14V34L24 42M42 14L24 22M42 14V34L24 42M24 22V42"/>`},
		{"History", parkHistory, `<path d="M5.81836 6.72729V14H13.0911"/><path d="M4 24C4 35.0457 12.9543 44 24 44V44C35.0457 44 44 35.0457 44 24C44 12.9543 35.0457 4 24 4C16.598 4 10.1351 8.02111 6.67677 13.9981"/><path d="M24.005 12L24.0038 24.0088L32.4832 32.4882"/>`},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s 的 path 与原版 icon-park 不一致：\n got %q\nwant %q", c.name, c.got, c.want)
		}
	}
}

// renderIconAt 把一枚图标渲染到 (size+pad)² 的白底画布上，便于逐像素比对。
func renderIconAt(ic *ui.SVG, size int) *image.RGBA {
	const pad = 6
	return ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#ffffff"))
		ui.Column(c).Padding(pad).Children(func() {
			ui.Icon(c, ic).Size(float32(size), float32(size)).TextColor(ui.Hex("#0f172a"))
		})
	}, size+2*pad, size+2*pad, 1)
}

func sameImage(a, b *image.RGBA) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			return false
		}
	}
	return true
}

// TestSearchIconsShareIconParkSource 证明 iconMagnifier（搜索药丸的提交键）
// 与 iconSearch（工具条的「搜索结果」）确实是同一枚 icon-park Search：
// 两者与 iconPark(parkSearch) 渲染出来的位图逐像素相同。
func TestSearchIconsShareIconParkSource(t *testing.T) {
	want := renderIconAt(iconPark(parkSearch), 48)
	if got := renderIconAt(iconMagnifier, 48); !sameImage(got, want) {
		t.Error("iconMagnifier 不是 iconPark(parkSearch)（渲染位图不同）")
	}
	if got := renderIconAt(iconSearch, 48); !sameImage(got, want) {
		t.Error("iconSearch 不是 iconPark(parkSearch)（渲染位图不同）")
	}
}

// TestMagnifierIsNotOldHandDrawn 反证旧的 16 画布近似确实换了：
// 旧 iconMagnifier（圆 + 一段斜线、没有放大镜的高光弧线与 48 画布的比例）
// 渲染出来应与新图差出一大片像素。差异太小说明还是旧图（或换汤不换药）。
func TestMagnifierIsNotOldHandDrawn(t *testing.T) {
	old := icon(`<circle cx="7.2" cy="7.2" r="4.3"/><path d="M10.4 10.4L13.8 13.8"/>`)
	newImg := renderIconAt(iconMagnifier, 48)
	oldImg := renderIconAt(old, 48)
	diff, total := 0, len(newImg.Pix)/4
	for i := 0; i < len(newImg.Pix); i += 4 {
		if newImg.Pix[i] != oldImg.Pix[i] || newImg.Pix[i+1] != oldImg.Pix[i+1] || newImg.Pix[i+2] != oldImg.Pix[i+2] {
			diff++
		}
	}
	t.Logf("新旧放大镜：%d/%d 像素不同", diff, total)
	if diff < total/50 {
		t.Errorf("新放大镜与旧的几乎一样（仅 %d/%d 不同）—— 没换成 icon-park 原始 path？", diff, total)
	}
}

// TestSectionIconsMatchOriginal 四枚入口图标也逐枚等于 icon-park 原始 path
// （渲染位图对照），而不是 16 画布的近似。
func TestSectionIconsMatchOriginal(t *testing.T) {
	cases := []struct {
		name string
		got  *ui.SVG
		want string
	}{
		{"动态/ShareSys", iconFeed, parkShareSys},
		{"热门与推荐/ChartRing", iconPopular, parkChartRing},
		{"收藏/WeixinFavorites", iconFavorite, parkWeixinFavorites},
		{"历史/History", iconHistory, parkHistory},
	}
	for _, c := range cases {
		if !sameImage(renderIconAt(c.got, 48), renderIconAt(iconPark(c.want), 48)) {
			t.Errorf("%s 不是 iconPark 原始 path 渲染出来的", c.name)
		}
	}
}
