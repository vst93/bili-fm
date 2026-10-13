package view

import "github.com/egoist/mygo/ui"

// 列表空态。原版没有统一的空态组件，三处各写各的类名/CSS，差别在**竖直位置
// 与颜色**上（对照 main:src 与 globals.css）：
//
//	.history-empty-tip        收藏 / 稍后再看 / 合集
//	    min-height 160 + display:grid + place-content:center + padding 24 12
//	    + 13px + #94a3b8
//	热门/推荐                   flex items-center justify-center h-40 text-gray-400
//	    h-40 = 160 + 居中 + 14px，颜色被抽屉覆写成 #64748b（比其它空态深一档）
//	弹幕/评论                   text-center text-default-400 py-8
//	    上下各 32px、14px、#94a3b8（default-400 在抽屉里同样覆写成 #94a3b8）
//
// 原生版此前是一份 `Padding(32, 0)` 的行给所有列表用：三个「160 高的块里居中」
// 的空态（收藏 / 稍后再看 / 合集 / 热门 / 推荐）全都贴到了正文顶上，且热门/推荐
// 的颜色也用了 #94a3b8 而不是覆写后的 #64748b。
type emptyVariant int

const (
	// emptyTipVariant 是 .history-empty-tip（160 高的块，文案落在正中）。
	emptyTipVariant emptyVariant = iota
	// emptyCenterVariant 是热门 / 推荐抽屉的 h-40 + text-gray-400。
	emptyCenterVariant
	// emptyNoteVariant 是弹幕 / 评论的 py-8（贴正文顶）。
	emptyNoteVariant
)

// emptyStyle 是一种空态文字块的几何与配色。
type emptyStyle struct {
	MinHeight float32 // 最小高度（原版 min-height:160 / h-40）
	PadY      float32 // 上下内边距（原版 padding:24px 12px / py-8）
	PadX      float32
	FontSize  float32
	Ink       ui.Color
}

// emptyStyleFor 给出某种空态写法的取值。
func emptyStyleFor(v emptyVariant) emptyStyle {
	switch v {
	case emptyTipVariant:
		return emptyStyle{
			MinHeight: 160, PadY: 24, PadX: 12,
			FontSize: 13, Ink: ui.Hex("#94a3b8"),
		}
	case emptyCenterVariant:
		// h-40 没有内边距；text-gray-400 在抽屉里被 `[data-slot="wrapper"]
		// .text-gray-400` 覆写成 #64748b。
		return emptyStyle{MinHeight: 160, FontSize: 14, Ink: ui.Hex("#64748b")}
	default:
		return emptyStyle{PadY: 32, FontSize: 14, Ink: ui.Hex("#94a3b8")}
	}
}

// emptyVariantFor 按抽屉选原版对应的空态写法：只有热门与推荐是 h-40 +
// text-gray-400，弹幕/评论是 py-8，其余（收藏 / 稍后再看 / 合集 / 搜索 /
// 动态 / UP 空间 / 选集）都是 .history-empty-tip。
func emptyVariantFor(drawer string) emptyVariant {
	switch drawer {
	case "popular":
		return emptyCenterVariant
	case DrawerDanmaku:
		return emptyNoteVariant
	}
	return emptyTipVariant
}

// emptyBlock 画一块空态：按 variant 定最小高度 / 内边距 / 字号 / 颜色，文案
// 在块内水平竖直都居中。
func (a *App) emptyBlock(c *ui.Context, text string, v emptyVariant) {
	st := emptyStyleFor(v)
	box := ui.Row(c).FillWidth().MinHeight(st.MinHeight).Center().
		Padding(st.PadY, st.PadX)
	box.Children(func() {
		ui.Text(c, text).FontSize(st.FontSize).TextColor(st.Ink)
	})
}

// emptyState 是列表抽屉的空态：按当前抽屉选原版对应的那种写法。
func (a *App) emptyState(c *ui.Context, text string) {
	a.emptyBlock(c, text, emptyVariantFor(a.Drawer))
}

// emptyNote 是弹幕 / 评论的空态 / 加载中提示（原版 `text-center
// text-default-400 py-8`）：贴着正文顶、上下各 32px 留白。
func (a *App) emptyNote(c *ui.Context, text string) {
	a.emptyBlock(c, text, emptyNoteVariant)
}
