package view

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/imagecache"
)

// 这些测试钉住「每个调用点按展示尺寸传对下采样宽度」——对齐 Tauri 原版
// src/utils/string.tsx 的 graftingImage(img, width) 显式宽度：
//
//	列表卡片 240（默认）、主播放器封面 480、背景氛围光 320、
//	播放列表缩略图 192、各类头像 96。
//
// 从前本构建一律 240：播放器封面（展示 263 CSS px、2x 屏 526 设备像素）
// 被压到 240 发糊，而头像（32–40 px）/播放列表缩略图（64×36）反而
// 按 240 下载、解码出 6~8 倍于所需的位图。
func TestCoverWidthsPerCallSite(t *testing.T) {
	t.Run("列表卡片 240", func(t *testing.T) {
		a := NewApp(func() {})
		rec := newRecordingCache()
		a.Images = rec
		a.ListFor("feed").Cards = []Card{{Bvid: "BV1", Title: "标题", Cover: "card.png"}}
		a.Drawer = "feed"
		ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600).Frame()
		if !rec.sawWidth("card.png", imagecache.WidthCard) {
			t.Fatalf("卡片封面应以 %d 宽度请求，实际 %v", imagecache.WidthCard, rec.widths)
		}
	})

	t.Run("选集卡片 240", func(t *testing.T) {
		a := NewApp(func() {})
		rec := newRecordingCache()
		a.Images = rec
		a.Info = newPartsInfo(2, "cover")
		a.Drawer = DrawerParts
		ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600).Frame()
		if !rec.sawWidth("cover/part0.png", imagecache.WidthCard) {
			t.Fatalf("选集封面应以 %d 宽度请求，实际 %v", imagecache.WidthCard, rec.widths)
		}
	})

	t.Run("播放列表缩略图 192", func(t *testing.T) {
		a := NewApp(func() {})
		rec := newRecordingCache()
		a.Images = rec
		a.Playlist = []PlayItem{{ID: "1", FirstFrame: "pl.png", Part: "P1", Title: "T"}}
		a.Drawer = DrawerPlaylist
		ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600).Frame()
		if !rec.sawWidth("pl.png", imagecache.WidthPlaylist) {
			t.Fatalf("播放列表缩略图应以 %d 宽度请求，实际 %v", imagecache.WidthPlaylist, rec.widths)
		}
	})

	t.Run("用户头像 96", func(t *testing.T) {
		a := NewApp(func() {})
		rec := newRecordingCache()
		a.Images = rec
		a.Face = "face.png"
		ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600).Frame()
		if !rec.sawWidth("face.png", imagecache.WidthAvatar) {
			t.Fatalf("用户头像应以 %d 宽度请求，实际 %v", imagecache.WidthAvatar, rec.widths)
		}
	})

	t.Run("UP 主头像 96", func(t *testing.T) {
		a := NewApp(func() {})
		rec := newRecordingCache()
		a.Images = rec
		a.Info = &Info{Bvid: "BV1", OwnerFace: "owner.png"}
		ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600).Frame()
		if !rec.sawWidth("owner.png", imagecache.WidthAvatar) {
			t.Fatalf("UP 主头像应以 %d 宽度请求，实际 %v", imagecache.WidthAvatar, rec.widths)
		}
	})

	t.Run("评论头像 96", func(t *testing.T) {
		a := NewApp(func() {})
		rec := newRecordingCache()
		a.Images = rec
		a.Comments = []Comment{{User: "u", Avatar: "avatar.png", Content: "c", Time: "刚刚"}}
		a.Drawer = DrawerDanmaku
		a.DanmakuTab = TabReply
		ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600).Frame()
		if !rec.sawWidth("avatar.png", imagecache.WidthAvatar) {
			t.Fatalf("评论头像应以 %d 宽度请求，实际 %v", imagecache.WidthAvatar, rec.widths)
		}
	})

	t.Run("播放器封面 480 + 氛围光 320", func(t *testing.T) {
		a := NewApp(func() {})
		rec := newRecordingCache()
		a.Images = rec
		a.Track = &Track{Bvid: "BV1", FirstFrame: "player.png"}
		ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600).Frame()
		if !rec.sawWidth("player.png", imagecache.WidthPlayer) {
			t.Fatalf("播放器封面应以 %d 宽度请求，实际 %v", imagecache.WidthPlayer, rec.widths)
		}
		if !rec.sawWidth("player.png", imagecache.WidthAmbient) {
			t.Fatalf("氛围光应以 %d 宽度请求，实际 %v", imagecache.WidthAmbient, rec.widths)
		}
	})
}
