package main

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/view"
)

// 换账号发生在选集抽屉里时，返回上一层不能被一个被清空的来源列表接住；
// 普通已初始化的来源仍必须原样恢复（滚动 / 页码 / 卡片都不动）。
func TestBackToDrawerAfterAccountInvalidation(t *testing.T) {
	t.Run("invalidated", func(t *testing.T) {
		a := view.NewApp(func() {})
		c := &controller{app: a}
		a.FolderID = 7
		l := a.ListFor("favorite")
		l.Context = a.ListContext("favorite")
		l.Initialized = true
		l.Cards = []view.Card{{Bvid: "BVold"}}
		l.Page = 5
		l.GridScroll.Y = 320
		a.Drawer = "favorite"
		a.BrowseTo(&view.Info{Bvid: "BVparts"})

		c.invalidateBrowsingForAccount()
		reloads := 0
		a.Act.Reload = func() {
			reloads++
			l.BeginFirstPage(a.ListContext(a.Drawer))
		}
		a.CloseDrawer()

		if a.Drawer != "favorite" {
			t.Fatalf("CloseDrawer 后 Drawer = %q，应回到 favorite", a.Drawer)
		}
		if reloads != 1 {
			t.Fatalf("作废来源应惰性重载一次，reloads=%d", reloads)
		}
		if !l.Loading || len(l.Cards) != 0 {
			t.Fatalf("重载状态不对：loading=%v cards=%d", l.Loading, len(l.Cards))
		}
	})

	t.Run("initialized", func(t *testing.T) {
		a := view.NewApp(func() {})
		a.FolderID = 7
		l := a.ListFor("favorite")
		l.Context = a.ListContext("favorite")
		l.Initialized = true
		l.Cards = []view.Card{{Bvid: "BV1"}, {Bvid: "BV2"}}
		l.Page = 5
		l.GridScroll.Y = 320
		a.Drawer = "favorite"
		a.BrowseTo(&view.Info{Bvid: "BVparts"})

		reloads := 0
		a.Act.Reload = func() { reloads++ }
		a.CloseDrawer()

		if a.Drawer != "favorite" {
			t.Fatalf("CloseDrawer 后 Drawer = %q，应回到 favorite", a.Drawer)
		}
		if reloads != 0 {
			t.Fatalf("已初始化来源被重拉：reloads=%d", reloads)
		}
		if len(l.Cards) != 2 || l.Page != 5 || l.GridScroll.Y != 320 || l.Loading {
			t.Fatalf("返回后会话状态丢失：cards=%d page=%d grid=%v loading=%v",
				len(l.Cards), l.Page, l.GridScroll.Y, l.Loading)
		}
	})
}
