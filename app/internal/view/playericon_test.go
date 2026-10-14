package view

import (
	"bytes"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放栏 / 迷你窗 / 列表卡片 meta 图标对齐 icon-park 原始 path）的测试。
//
// 原版 player.tsx / videoInfo.tsx / cardMeta.tsx 用的都是 @icon-park/react 1.4.2
// 的图标（48×48 画布、描边 4）。原生版此前只有搜索 / 热门 / 推荐等几枚换成了
// 原始 path，播放控制这一组仍是 16 画布手抄：下一集画成「实心三角 + 竖线」（▶|），
// 而原版 GoEnd 是「人字形 + 竖线」（>|）；暂停键是两块粗圆角矩形（比原版两根
// 细圆头竖线粗矮）；音量 / 均衡 / 恰饭也都是观感近似。这里逐字拷原始 path，
// 并把「不是旧近似」逐枚反证。

// TestPlayerControlIconParkPathsAreVerbatim 逐字钉住本轮 icon-park 的 path 数据
// （@icon-park/react 1.4.2 的 icons/*.js，48×48 画布）。谁再手改就会红。
func TestPlayerControlIconParkPathsAreVerbatim(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"PlayOne", parkPlayOne, `<path fill="currentColor" d="M15 24V11.8756L25.5 17.9378L36 24L25.5 30.0622L15 36.1244V24Z"/>`},
		{"Pause", parkPause, `<path d="M16 12V36"/><path d="M32 12V36"/>`},
		{"GoEnd", parkGoEnd, `<path d="M14 12L26 24L14 36"/><path d="M34 12V36"/>`},
		{"GoStart", parkGoStart, `<path d="M34 36L22 24L34 12"/><path d="M14 12V36"/>`},
		{"VolumeNotice", parkVolumeNotice,
			`<path d="M24 6V42C17 42 11.7985 32.8391 11.7985 32.8391H6C4.89543 32.8391 4 31.9437 4 30.8391V17.0108C4 15.9062 4.89543 15.0108 6 15.0108H11.7985C11.7985 15.0108 17 6 24 6Z"/>` +
				`<path d="M32 15L32 15C32.6232 15.5565 33.1881 16.1797 33.6841 16.8588C35.1387 18.8504 36 21.3223 36 24C36 26.6545 35.1535 29.1067 33.7218 31.0893C33.2168 31.7885 32.6391 32.4293 32 33"/>` +
				`<path d="M34.2359 41.1857C40.0836 37.6953 44 31.305 44 24C44 16.8085 40.2043 10.5035 34.507 6.97906"/>`},
		{"VolumeMute", parkVolumeMute,
			`<path d="M40.7348 20.2858L32.2495 28.7711"/>` +
				`<path d="M32.2496 20.2858L40.7349 28.7711"/>` +
				`<path d="M24 6V42C17 42 11.7985 32.8391 11.7985 32.8391H6C4.89543 32.8391 4 31.9437 4 30.8391V17.0108C4 15.9062 4.89543 15.0108 6 15.0108H11.7985C11.7985 15.0108 17 6 24 6Z"/>`},
		{"Equalizer", parkEqualizer,
			`<path d="M11 16V42"/>` +
				`<path d="M24 29V42"/>` +
				`<path d="M24 19V6"/>` +
				`<path d="M37 6V32"/>` +
				`<path d="M11 16C13.7614 16 16 13.7614 16 11C16 8.23858 13.7614 6 11 6C8.23858 6 6 8.23858 6 11C6 13.7614 8.23858 16 11 16Z"/>` +
				`<path d="M24 29C26.7614 29 29 26.7614 29 24C29 21.2386 26.7614 19 24 19C21.2386 19 19 21.2386 19 24C19 26.7614 21.2386 29 24 29Z"/>` +
				`<path d="M37 42C39.7614 42 42 39.7614 42 37C42 34.2386 39.7614 32 37 32C34.2386 32 32 34.2386 32 37C32 39.7614 34.2386 42 37 42Z"/>`},
		{"Ad", parkAd,
			`<circle cx="24" cy="24" r="20"/>` +
				`<path d="M38 38L35 35"/>` +
				`<path d="M10 10L13 13"/>` +
				`<path d="M14 31L14.8571 28M22 31L21.1429 28M21.1429 28L18 17L14.8571 28M21.1429 28H14.8571"/>` +
				`<path d="M35 24C35 29 31.4183 31 27 31V17C31.4183 17 35 19 35 24Z"/>`},
		{"PreviewOpen", parkPreviewOpen,
			`<path d="M24 36C35.0457 36 44 24 44 24C44 24 35.0457 12 24 12C12.9543 12 4 24 4 24C4 24 12.9543 36 24 36Z"/>` +
				`<path d="M24 29C26.7614 29 29 26.7614 29 24C29 21.2386 26.7614 19 24 19C21.2386 19 19 21.2386 19 24C19 26.7614 21.2386 29 24 29Z"/>`},
		{"Comment", parkComment,
			`<path d="M44 6H4V36H13V41L23 36H44V6Z"/>` +
				`<path d="M14 19.5V22.5"/>` +
				`<path d="M24 19.5V22.5"/>` +
				`<path d="M34 19.5V22.5"/>`},
		{"ThumbsUp", parkThumbsUp,
			`<path d="M27.6002 18.5998V11.3998C27.6002 8.41743 25.1826 5.99977 22.2002 5.99977L15.0002 22.1998V41.9998H35.9162C37.7113 42.0201 39.2471 40.7147 39.5162 38.9398L42.0002 22.7398C42.1587 21.6955 41.8506 20.6343 41.1576 19.8373C40.4645 19.0403 39.4564 18.5878 38.4002 18.5998H27.6002Z"/>` +
				`<path d="M15 22.0001H10.194C8.08532 21.9628 6.2827 23.7095 6 25.7994V38.3994C6.2827 40.4894 8.08532 42.0367 10.194 41.9994H15V22.0001Z"/>`},
		{"HandleB", parkHandleB,
			`<circle cx="24" cy="24" r="20"/>` +
				`<path d="M24.7918 23C27.6682 23 30 20.7614 30 18C30 15.2386 27.6682 13 24.7918 13H18V23H24.7918Z"/>` +
				`<path d="M26.831 35C29.6857 35 32 32.3137 32 29C32 25.6863 29.6857 23 26.831 23H18V35H26.831Z"/>`},
		{"Star", parkStar, `<path d="M23.9986 5L17.8856 17.4776L4 19.4911L14.0589 29.3251L11.6544 43L23.9986 36.4192L36.3454 43L33.9586 29.3251L44 19.4911L30.1913 17.4776L23.9986 5Z"/>`},
		{"StarFilled", parkStarFilled, `<path fill="currentColor" d="M23.9986 5L17.8856 17.4776L4 19.4911L14.0589 29.3251L11.6544 43L23.9986 36.4192L36.3454 43L33.9586 29.3251L44 19.4911L30.1913 17.4776L23.9986 5Z"/>`},
		{"Browser", parkBrowser,
			`<path d="M42 18V40C42 41.1046 41.1046 42 40 42H8C6.89543 42 6 41.1046 6 40V18"/>` +
				`<path d="M6 8C6 6.89543 6.89543 6 8 6H40C41.1046 6 42 6.89543 42 8V18H6V8Z"/>` +
				`<path fill="currentColor" d="M12 14C13.1046 14 14 13.1046 14 12C14 10.8954 13.1046 10 12 10C10.8954 10 10 10.8954 10 12C10 13.1046 10.8954 14 12 14Z"/>` +
				`<path fill="currentColor" d="M18 14C19.1046 14 20 13.1046 20 12C20 10.8954 19.1046 10 18 10C16.8954 10 16 10.8954 16 12C16 13.1046 16.8954 14 18 14Z"/>` +
				`<path fill="currentColor" d="M24 14C25.1046 14 26 13.1046 26 12C26 10.8954 25.1046 10 24 10C22.8954 10 22 10.8954 22 12C22 13.1046 22.8954 14 24 14Z"/>`},
		{"VideoTwo", parkVideoTwo,
			`<path d="M39 6H9C7.34315 6 6 7.34315 6 9V39C6 40.6569 7.34315 42 9 42H39C40.6569 42 42 40.6569 42 39V9C42 7.34315 40.6569 6 39 6Z"/>` +
				`<path d="M20.5 28V21.9378L25.75 24.9689L31 28L25.75 31.0311L20.5 34.0622V28Z"/>` +
				`<path d="M6 15H42"/>` +
				`<path d="M33 6L27 15"/>` +
				`<path d="M21 6L15 15"/>`},
		{"DoubleUp", parkDoubleUp, `<path d="M12 24L24 12L36 24"/><path d="M12 36L24 24L36 36"/>`},
		{"Layers", parkLayers,
			`<path d="M4 11.9143L24 19L44 11.9143L24 5L4 11.9143Z"/>` +
				`<path d="M4 20L24 27L44 20"/>` +
				`<path d="M4 28L24 35L44 28"/>` +
				`<path d="M4 36L24 43L44 36"/>`},
		{"MusicList", parkMusicList,
			`<path d="M24 19H40"/>` +
				`<path d="M24 10H40"/>` +
				`<path d="M8 38H40"/>` +
				`<path d="M8 28H40"/>` +
				`<path d="M8 10L16 15L8 20V10Z"/>`},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s 的 path 与原版 icon-park 不一致：\n got %q\nwant %q", c.name, c.got, c.want)
		}
	}
}

// TestPlayerControlIconsMatchOriginal 每枚图标变量都等于 iconPark(原始 path)
// 渲染出来的位图 —— 防止以后又被换成 16 画布的手抄近似。
func TestPlayerControlIconsMatchOriginal(t *testing.T) {
	cases := []struct {
		name string
		got  *ui.SVG
		want string
	}{
		{"播放/PlayOne", iconPlayOne, parkPlayOne},
		{"暂停/Pause", iconPause, parkPause},
		{"下一集/GoEnd", iconNext, parkGoEnd},
		{"上一集/GoStart", iconPrev, parkGoStart},
		{"音量/VolumeNotice", iconVolume, parkVolumeNotice},
		{"静音/VolumeMute", iconMute, parkVolumeMute},
		{"均衡/Equalizer", iconEQ, parkEqualizer},
		{"恰饭/Ad", iconSponsor, parkAd},
		{"播放量/PreviewOpen", iconViews, parkPreviewOpen},
		{"弹幕/Comment", iconDanmaku, parkComment},
		{"点赞/ThumbsUp", iconLike, parkThumbsUp},
		{"投币/HandleB", iconCoin, parkHandleB},
		{"收藏(常态)/Star", iconStar, parkStar},
		{"收藏(已收藏)/StarFilled", iconStarFilled, parkStarFilled},
		{"浏览器/Browser", iconBrowser, parkBrowser},
		{"视频/VideoTwo", iconVideo, parkVideoTwo},
		{"选集/DoubleUp", iconList, parkDoubleUp},
		{"合集/Layers", iconSeries, parkLayers},
		{"播放列表/MusicList", iconMusicList, parkMusicList},
	}
	for _, c := range cases {
		if !sameImage(renderIconAt(c.got, 48), renderIconAt(iconPark(c.want), 48)) {
			t.Errorf("%s 不是 iconPark 原始 path 渲染出来的", c.name)
		}
	}
}

// TestStarFilledIsFilled 反证收藏键的 filled 变体真的是实心星（只是同一条 path
// 加上 fill=currentColor）：与 outline 星在像素上拉开差距。
func TestStarFilledIsFilled(t *testing.T) {
	diff, total := iconPixelDiff(iconStar, iconStarFilled, 40)
	if diff < total/20 {
		t.Errorf("空心/实心星差别太小（%d/%d）", diff, total)
	}
}

// iconPixelDiff 返回两枚图标在同一尺寸下渲染时 RGB 不同的像素数与总像素数。
func iconPixelDiff(a, b *ui.SVG, size int) (int, int) {
	ai, bi := renderIconAt(a, size), renderIconAt(b, size)
	if ai.Bounds() != bi.Bounds() {
		return -1, 0
	}
	diff, total := 0, len(ai.Pix)/4
	for i := 0; i < len(ai.Pix); i += 4 {
		if ai.Pix[i] != bi.Pix[i] || ai.Pix[i+1] != bi.Pix[i+1] || ai.Pix[i+2] != bi.Pix[i+2] {
			diff++
		}
	}
	return diff, total
}

// TestPlayerControlIconsNotOldHandDrawn 反证旧的 16 画布近似确实换掉了：把旧图
// 与现图在同一尺寸下渲染，逐枚要求差出一片像素（差异太小说明还是旧图）。
func TestPlayerControlIconsNotOldHandDrawn(t *testing.T) {
	cases := []struct {
		name string
		old  *ui.SVG
		now  *ui.SVG
		size int
	}{
		{"播放",
			icon(`<path d="M5.5 3.2v9.6a.8.8 0 0 0 1.22.68l7.6-4.8a.8.8 0 0 0 0-1.36l-7.6-4.8A.8.8 0 0 0 5.5 3.2z"/>`),
			iconPlayOne, 24},
		{"暂停",
			icon(`<rect x="4.4" y="3" width="2.8" height="10" rx="0.9"/><rect x="8.8" y="3" width="2.8" height="10" rx="0.9"/>`),
			iconPause, 24},
		{"下一集",
			icon(`<path d="M3.6 3.4v9.2a.7.7 0 0 0 1.08.6l6.2-4.6a.7.7 0 0 0 0-1.2L4.68 2.8a.7.7 0 0 0-1.08.6z"/><path d="M12.2 3v10" stroke-width="1.8"/>`),
			iconNext, 18},
		{"音量",
			icon(`<path d="M3.2 6h2.1l3.3-2.7a.6.6 0 0 1 .98.46v8.48a.6.6 0 0 1-.98.46L5.3 10H3.2a.7.7 0 0 1-.7-.7V6.7a.7.7 0 0 1 .7-.7z"/><path d="M11.4 6.2a2.6 2.6 0 0 1 0 3.6"/><path d="M12.9 4.6a4.6 4.6 0 0 1 0 6.8"/>`),
			iconVolume, 20},
		{"静音",
			icon(`<path d="M3.2 6h2.1l3.3-2.7a.6.6 0 0 1 .98.46v8.48a.6.6 0 0 1-.98.46L5.3 10H3.2a.7.7 0 0 1-.7-.7V6.7a.7.7 0 0 1 .7-.7z"/><path d="M10.8 6.4l3.4 3.2M14.2 6.4l-3.4 3.2" stroke-width="1.6"/>`),
			iconMute, 20},
		{"均衡",
			icon(`<path d="M3 11V5M6.5 13V3M10 10V6M13.5 12V4"/>`),
			iconEQ, 17},
		{"恰饭",
			icon(`<rect x="2" y="4.5" width="12" height="7" rx="1.2"/><path d="M6.4 6.4l3.6 1.6-3.6 1.6z"/>`),
			iconSponsor, 17},
		{"弹幕/评论",
			icon(`<path d="M2.8 4.6a1.8 1.8 0 0 1 1.8-1.8h6.8a1.8 1.8 0 0 1 1.8 1.8v4.2a1.8 1.8 0 0 1-1.8 1.8H6.4l-2.6 2.2v-2.2h-.2a1.8 1.8 0 0 1-1.8-1.8z"/><path d="M5.4 6.7h5.2M5.4 8.6h3.2" stroke-width="1.1"/>`),
			iconDanmaku, 20},
		{"卡片播放量",
			icon(`<path d="M5.5 3.2v9.6a.8.8 0 0 0 1.22.68l7.6-4.8a.8.8 0 0 0 0-1.36l-7.6-4.8A.8.8 0 0 0 5.5 3.2z"/>`), iconViews, 12},
		{"点赞",
			icon(`<path d="M6.4 13.6V7l2.2-4a1.1 1.1 0 0 1 1.9.9l-.4 2.7h2.8a1.2 1.2 0 0 1 1.18 1.42l-.86 4.4a1.6 1.6 0 0 1-1.57 1.3H6.4z"/><path d="M6.4 7H3.9a.9.9 0 0 0-.9.9v5a.9.9 0 0 0 .9.9h2.5"/>`),
			iconLike, 20},
		{"投币",
			icon(`<circle cx="8" cy="8" r="5.6"/><path d="M8 5.2v5.6M6.4 6.6c0-.8.7-1.4 1.6-1.4s1.6.6 1.6 1.4-.7 1.2-1.6 1.4c-.9.2-1.6.6-1.6 1.4s.7 1.4 1.6 1.4 1.6-.6 1.6-1.4"/>`),
			iconCoin, 20},
		{"收藏",
			icon(`<path d="M8 1.9l1.9 4.2 4.6.5-3.4 3.1.9 4.5L8 11.9l-4 2.2.9-4.5L1.5 6.5l4.6-.5z"/>`),
			iconStar, 20},
		{"浏览器",
			icon(`<rect x="1.8" y="3" width="12.4" height="10" rx="1.6"/><path d="M1.8 6.2h12.4"/><circle cx="4.3" cy="4.6" r="0.5"/><circle cx="6.3" cy="4.6" r="0.5"/>`),
			iconBrowser, 18},
		{"视频",
			icon(`<rect x="1.8" y="3" width="12.4" height="9" rx="1.6"/><path d="M7 6.6l3 1.4-3 1.4z"/>`),
			iconVideo, 18},
		{"选集",
			icon(`<path d="M3.4 8.6L8 4.2l4.6 4.4M3.4 12.2L8 7.8l4.6 4.4" stroke-width="1.7"/>`),
			iconList, 20},
		{"合集",
			icon(`<path d="M8 1.9l6.2 3.2L8 8.3 1.8 5.1z"/><path d="M2.6 8.2l5.4 2.8 5.4-2.8"/><path d="M2.6 11.2l5.4 2.8 5.4-2.8"/>`),
			iconSeries, 20},
		{"播放列表",
			icon(`<path d="M2.5 4h7M2.5 7.5h7M2.5 11h4"/><path d="M12 4.6v6.2"/><circle cx="10.6" cy="11.6" r="1.6"/>`),
			iconMusicList, 20},
	}
	for _, c := range cases {
		diff, total := iconPixelDiff(c.old, c.now, c.size)
		if total == 0 {
			t.Fatalf("%s：渲染失败", c.name)
		}
		t.Logf("%s：旧近似 vs 现图 %d/%d 像素不同", c.name, diff, total)
		if diff < total/100 {
			t.Errorf("%s 与旧的 16 画布近似几乎一样（仅 %d/%d 不同）—— 没换成 icon-park？", c.name, diff, total)
		}
	}
}

// TestPlayerControlIconsNotBlank 反证这些图标在**实际使用的尺寸**下真的画得出
// 东西（path 写错、解析失败会得到一张全白图）。
func TestPlayerControlIconsNotBlank(t *testing.T) {
	cases := []struct {
		name string
		ic   *ui.SVG
		size int
	}{
		{"播放", iconPlayOne, 24},
		{"暂停", iconPause, 24},
		{"下一集", iconNext, 18},
		{"音量", iconVolume, 20},
		{"静音", iconMute, 20},
		{"均衡", iconEQ, 17},
		{"恰饭", iconSponsor, 17},
		{"播放量", iconViews, 12},
		{"弹幕", iconDanmaku, 12},
		{"点赞", iconLike, 20},
		{"投币", iconCoin, 20},
		{"收藏", iconStar, 20},
		{"已收藏", iconStarFilled, 20},
		{"浏览器", iconBrowser, 18},
		{"视频", iconVideo, 18},
		{"选集", iconList, 20},
		{"合集", iconSeries, 20},
		{"播放列表", iconMusicList, 20},
	}
	for _, c := range cases {
		img := renderIconAt(c.ic, c.size)
		ink := 0
		for i := 0; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0xff || img.Pix[i+1] != 0xff || img.Pix[i+2] != 0xff {
				ink++
			}
		}
		if ink == 0 {
			t.Errorf("%s 在 %dpx 下渲染为空白（path 写错？）", c.name, c.size)
		}
	}
}

// TestPlayerControlIconWiring 接线反证：播放键（主窗 / 迷你窗）用的是 PlayOne
// （iconPlayOne），列表卡片 meta 的播放量前导图标用的是 PreviewOpen（iconViews）
// 而不是旧的三角 iconPlay。写了新图标没接上等于没改。
func TestPlayerControlIconWiring(t *testing.T) {
	cases := []struct {
		file string
		want string
		why  string
	}{
		{"playerbar.go", "pick(a.Playing, iconPause, iconPlayOne)", "主窗播放/暂停键"},
		{"mini.go", "pick(a.Playing, iconPause, iconPlayOne)", "迷你窗播放/暂停键"},
		{"drawer.go", "icon := iconViews", "列表卡片 meta 的播放量图标"},
		{"home.go", "pick(a.Faved, iconStarFilled, iconStar)", "收藏键的实心星"},
		{"home.go", "\"浏览器打开\", iconBrowser, 18", "浏览器图标的 18px"},
		{"home.go", "\"视频播放\", iconVideo, 18", "视频图标的 18px"},
		{"home.go", "\"弹幕/评论\", iconDanmaku, 18", "弹幕图标的 18px"},
	}
	for _, c := range cases {
		src, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", c.file, err)
		}
		if !bytes.Contains(src, []byte(c.want)) {
			t.Errorf("%s 没接上 %q（%s）", c.file, c.want, c.why)
		}
	}
	// 卡片 meta 不能再拿手抄三角当播放量图标（合集「播放全部」与播放列表当前行
	// 的 Play 已改成逐色 ui.Image，drawer.go 里不应再有 `icon := iconPlay`）。
	src, err := os.ReadFile("drawer.go")
	if err != nil {
		t.Fatalf("读 drawer.go 失败：%v", err)
	}
	if bytes.Contains(src, []byte("icon := iconPlay\n")) {
		t.Error("drawer.go 里仍有 `icon := iconPlay` —— 卡片 meta 的播放量图标应改成 iconViews")
	}
	if bytes.Contains(src, []byte("ui.Icon(c, iconPlay)")) {
		t.Error("drawer.go 里仍有 ui.Icon(c, iconPlay) —— filled 的 Play 应走 ui.Image(c, iconPlayFilled)")
	}
}
