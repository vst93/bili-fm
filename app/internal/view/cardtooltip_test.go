package view

import (
	"bytes"
	"os"
	"testing"
)

// TestCardTooltips 钉住列表卡片的 tooltip 行为对齐原版：
//   - 卡片标题（原版 ListCard 的 `<b title={title}>`）
//   - meta 行作者名（原版 CardMeta 轮 22 的 title）
//   - meta 行发布时间（原版 CardMeta 轮 28 的 title）
//
// 工具提示在无头环境里要等指针停留才出现，view 包的 Tester 拿不到可推进的
// 时钟，所以这里退一步：在源码里确认 .Tooltip() 真的接在标题和 meta 行的
// 作者/发布时间上——写了常量没接等于没写。
func TestCardTooltips(t *testing.T) {
	src, err := os.ReadFile("drawer.go")
	if err != nil {
		t.Fatalf("读 drawer.go 失败：%v", err)
	}

	// 卡片标题：原版 <b title={title}>，截断时悬浮显示完整标题。
	// renderCard 里两处 ui.Text(c, spec.Title) 都要接 Tooltip。
	titleTooltip := bytes.Count(src, []byte(".Tooltip(spec.Title)"))
	if titleTooltip != 2 {
		t.Errorf("卡片标题应有 2 处 .Tooltip(spec.Title)（有/无 Badge 两个分支），实际 %d", titleTooltip)
	}

	// meta 行作者 + 发布时间：原版 CardMeta 给 author（轮 22）和 pubdate
	// （轮 28）字段加了 title tooltip。cardMeta 里两处都要接 Tooltip。
	tooltipCount := bytes.Count(src, []byte(".Tooltip(f.Text)"))
	if tooltipCount < 2 {
		t.Errorf("meta 行作者/发布时间应有至少 2 处 .Tooltip(f.Text)，实际 %d", tooltipCount)
	}
}
