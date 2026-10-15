package view

import (
	"image"
	"strings"
	"testing"

	ui "github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/imagecache"
)

// 本轮把「无图可画时只有一层底色」的几处对齐原版：
//
//	UP 主头像   原版 videoInfo.tsx:253
//	            src={graftingImage(displayedOwner?.face || noface.jpg, 96)}
//	未登录头像  原版 searchForm.tsx 的 <Avatar showFallback={!userFace}>：人形剪影
//	评论头像    原版 danmakuList.tsx:302 无 face → /default-avatar.png（404）
//	            → RetryImg onError 退 /cover.png
//
// 这些测试都断言「该画的图/形状真的画了」，并把解码失败的构建事故也算作失败
//（placeholderAvailable 兜底），免得降级路径静默通过。

// TestOwnerFaceFallsBackToNoFaceAvatar 钉住 ownerFace() 的兜底：合作视频没选人、
// Info 的 OwnerFace 为空时都返回 B 站官方 noface 地址（原版
// `displayedOwner?.face || noface.jpg`），而不是空串（空串在渲染里只剩底色）。
func TestOwnerFaceFallsBackToNoFaceAvatar(t *testing.T) {
	a := NewApp(func() {})
	if got := a.ownerFace(); got != noFaceAvatar {
		t.Errorf("Info 为空时 ownerFace() = %q, want %q", got, noFaceAvatar)
	}

	a.Info = &Info{Bvid: "BV1", OwnerName: "UP"}
	if got := a.ownerFace(); got != noFaceAvatar {
		t.Errorf("OwnerFace 为空时 ownerFace() = %q, want %q", got, noFaceAvatar)
	}

	a.Info.OwnerFace = "https://i0.hdslb.com/bfs/face/real.jpg"
	if got := a.ownerFace(); got != a.Info.OwnerFace {
		t.Errorf("有 face 时应原样返回，得到 %q", got)
	}

	// 合作视频选中了参与者就整组看 TA：原版 displayedOwner 一旦有值，face 为空
	// 也退 noface，**不会**回落到主 UP 主的 face。
	a.Info.Staff = []Collaborator{{Mid: 7, Name: "合作者"}}
	a.CollabMid, a.CollabFor = 7, a.Info.Bvid
	if got := a.ownerFace(); got != noFaceAvatar {
		t.Errorf("选中无 face 的合作者时 ownerFace() = %q, want %q", got, noFaceAvatar)
	}
	a.Info.Staff[0].Face = "https://i0.hdslb.com/bfs/face/collab.jpg"
	if got := a.ownerFace(); got != a.Info.Staff[0].Face {
		t.Errorf("选中合作者时应用 TA 的 face，得到 %q", got)
	}
}

// TestOwnerFaceNoFaceURLIsHdslbRaw 钉住 noface 地址是「原样」的 hdslb 地址：
// 原版把它交给 graftingImage(img, 96)，下采样 @ 后缀由它加。原生版同理交给
// imagecache.ThumbURL —— 这里既不发请求也不手写后缀，只校验 URL 形状。
func TestOwnerFaceNoFaceURLIsHdslbRaw(t *testing.T) {
	if !strings.HasPrefix(noFaceAvatar, "https://i0.hdslb.com/bfs/") {
		t.Fatalf("默认头像应是 hdslb 原图地址，得到 %q", noFaceAvatar)
	}
	if strings.Contains(noFaceAvatar, "@") {
		t.Fatalf("默认头像地址不该预置下采样后缀（原版 graftingImage 才加），得到 %q", noFaceAvatar)
	}
}

// TestOwnerFaceRequestsNoFaceAtAvatarWidth 钉住渲染层确实按头像宽度去取默认头像
// （宽度 96 对齐原版 graftingImage 的显式 96；imagecache 会给 hdslb 地址自动
// 加 @96w.webp，与原版行为一致）。
func TestOwnerFaceRequestsNoFaceAtAvatarWidth(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	rec := newRecordingCache()
	a.Images = rec
	a.Info = &Info{Bvid: "BV1", Title: "T", OwnerName: "UP"} // OwnerFace 为空
	shellTester(t, a)

	if !rec.sawWidth(noFaceAvatar, imagecache.WidthAvatar) {
		t.Fatalf("无 face 时应以 %d 宽度请求 noface 默认头像，实际 %v",
			imagecache.WidthAvatar, rec.widths)
	}
}

// TestOwnerFacePlaceholderWhileLoading 钉住 UP 头像位图为 nil 时不再只铺底色：
// 占位图（原版 RetryImg onError 退 /cover.png 那一层）会被画出来。把占位图临时
// 换成纯红，量 #video-owner-face 矩形内的红像素。
func TestOwnerFacePlaceholderWhileLoading(t *testing.T) {
	old := placeholderCover
	defer func() { placeholderCover = old }()
	placeholderCover = solidRGBA(8, 8, 255, 0, 0, 255)
	if !placeholderAvailable() {
		t.Fatal("占位图未解码成功")
	}

	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Images = newRecordingCache() // 永不返回位图
	a.Info = &Info{Bvid: "BV1", Title: "T", OwnerName: "UP"}
	a.CoverMode = "disc" // 主区封面走碟片实底，像素测量不依赖封面位图
	tt := shellTester(t, a)

	r, ok := tt.Find("UP 主空间")
	if !ok {
		t.Fatal("找不到 UP 主头像（Label UP 主空间）")
	}
	img := tt.Image()
	reds := countRed(img, r)
	if reds < int(r.W*r.H)/2 {
		t.Fatalf("UP 头像未就绪时应铺占位图：%v 内红像素只有 %d（期望约 %d）",
			r, reds, int(r.W*r.H))
	}
}

// TestUserButtonSilhouetteWhenLoggedOut 钉住未登录时药丸右端画人形剪影
// （原版 HeroUI Avatar 的 showFallback 默认图形），而不是从前的 "?" 首字。
//
// 结构断言：渲染树里不该再出现 "?" 文本；像素断言：按钮中心偏上有一条实心
// 的「头」——未登录时画上去的数字 "?" 在 36px 圆里留不出这么饱满的一团。
func TestUserButtonSilhouetteWhenLoggedOut(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.LoggedIn = false
	a.Face = ""
	a.UName = ""
	tt := shellTester(t, a)

	if r, ok := tt.Find("?"); ok {
		t.Fatalf("未登录不该再画 \"?\" 首字（找到文本矩形 %v）", r)
	}

	r, ok := tt.Find("登录账号")
	if !ok {
		t.Fatal("找不到未登录药丸头像按钮")
	}
	tt.Frame()
	img := tt.Image()
	// 按钮中心横向 1/3~2/3、纵向 12%~58% 那一片里数「明显偏暗」的像素：剪影
	// 用的是主题 Faint 灰 #64748b，在 32% 白的底上会明显压暗（数字字形稀疏，
	// 同区域内远达不到这个量）。
	dark := 0
	x0, x1 := int(r.X+r.W/3), int(r.X+r.W*2/3)
	y0, y1 := int(r.Y+r.H*0.12), int(r.Y+r.H*0.58)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := img.PixOffset(x, y)
			if int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2]) < 3*170 {
				dark++
			}
		}
	}
	area := (x1 - x0) * (y1 - y0)
	if dark < area/4 {
		t.Fatalf("未登录药丸头像区域应有人形剪影（头部实心块）：暗像素 %d / %d", dark, area)
	}
}

// TestUserButtonInitialWhenLoggedIn 钉住已登录但头像未就绪时仍画用户名首字
//（剪影只属于未登录态）。
func TestUserButtonInitialWhenLoggedIn(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.LoggedIn = true
	a.Face = "face.png" // 录制的假缓存不返回位图 → 走未就绪分支
	a.UName = "阿绫"
	a.Images = newRecordingCache()
	tt := shellTester(t, a)

	if _, ok := tt.Find("阿"); !ok {
		t.Fatal("已登录且头像未就绪时应画用户名首字")
	}
}

// TestCommentAvatarPlaceholder 钉住评论 / 回复头像为空或位图为 nil 时铺占位图
//（原版 danmakuList getMemberInfo 无 face → /default-avatar.png → 404 →
// RetryImg 退 /cover.png），不再只铺 .bg-default-200 底色。
func TestCommentAvatarPlaceholder(t *testing.T) {
	old := placeholderCover
	defer func() { placeholderCover = old }()
	placeholderCover = solidRGBA(8, 8, 255, 0, 0, 255)
	if !placeholderAvailable() {
		t.Fatal("占位图未解码成功")
	}

	for _, tc := range []struct{ name, avatar string }{
		{"头像地址为空", ""},
		{"有地址未解码", "avatar.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.Images = newRecordingCache() // 永不返回位图
			a.Comments = []Comment{{User: "u", Avatar: tc.avatar, Content: "评论内容", Time: "刚刚"}}
			a.Drawer = DrawerDanmaku
			a.DanmakuTab = TabReply
			tt := shellTester(t, a)
			tt.Frame()

			// 头像盒是正文左侧 32×32 的圆；整幅图里除它没有别的红色，直接量全图。
			img := tt.Image()
			box := ui.Rect{W: float32(img.Bounds().Dx()), H: float32(img.Bounds().Dy())}
			reds := countRed(img, box)
			if reds < 400 {
				t.Fatalf("avatar=%q：评论头像应铺占位图，红像素只有 %d", tc.avatar, reds)
			}
		})
	}
}

// TestPlaylistRowPlaceholderWhenNoThumb 钉住播放列表行的缩略图位没用（地址为空
// 或还没抓到）时也铺占位图 —— 原版 RetryImg 是 `src={src || fallbackSrc}`，空
// 地址同样画 /cover.png。整幅图里只有行缩略图会被换成红，直接量全图。
func TestPlaylistRowPlaceholderWhenNoThumb(t *testing.T) {
	old := placeholderCover
	defer func() { placeholderCover = old }()
	placeholderCover = solidRGBA(8, 8, 255, 0, 0, 255)

	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Images = newRecordingCache() // 永不返回位图
	a.Playlist = []PlayItem{{ID: "a", Part: "P0", Title: "T0"}} // FirstFrame / Pic 都空
	a.Drawer = DrawerPlaylist
	tt := shellTester(t, a)
	tt.Frame()

	img := tt.Image()
	box := ui.Rect{W: float32(img.Bounds().Dx()), H: float32(img.Bounds().Dy())}
	if reds := countRed(img, box); reds < 400 {
		t.Fatalf("播放列表行无缩略图时应铺占位图，红像素只有 %d", reds)
	}
}

// solidRGBA 造一张纯色位图（测试里当「占位图」用，便于按颜色量像素）。
func solidRGBA(w, h int, r, g, b, a uint8) *ui.Bitmap {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(m.Pix); i += 4 {
		m.Pix[i], m.Pix[i+1], m.Pix[i+2], m.Pix[i+3] = r, g, b, a
	}
	return ui.NewBitmap(m)
}

// countRed 数矩形内「几乎纯红」的像素（占位图被换成纯红后的特征色）。
// 矩形先夹到图像范围内，越界部分不算。
func countRed(img *image.RGBA, r ui.Rect) int {
	b := img.Bounds()
	x0, y0 := max(int(r.X), b.Min.X), max(int(r.Y), b.Min.Y)
	x1, y1 := min(int(r.X+r.W), b.Max.X), min(int(r.Y+r.H), b.Max.Y)
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := img.PixOffset(x, y)
			if img.Pix[i] > 200 && img.Pix[i+1] < 60 && img.Pix[i+2] < 60 {
				n++
			}
		}
	}
	return n
}
