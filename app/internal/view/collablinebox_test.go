package view

import (
	"math"
	"testing"
)

// 本轮（合作 UP 主选择面板的条目行盒对齐）的测试。
//
// 原版条目是 HeroUI 的 DropdownItem，两个文本 slot 的行高由 HeroUI 的
// `text-small`（0.875rem / 1.25rem ⇒ 20px）与 `text-tiny`（0.75rem / 1rem
// ⇒ 16px）给定，而 globals.css 只覆写 font-size（name 12px / role 11px）——
// 行高是**写死的像素值**、不随字号缩。框架的自然行盒是 18 / 17，所以
// 带角色的条目此前是 7 + 18 + 17 + 7 = 49 高（原版 50），角色行也偏高
// 半个像素。用 pxLineBox 显式压回（见 linebox.go）。

// TestCollabItemLineBoxes 真实跑一遍合作 UP 主选择面板：
//
//   - 无角色的条目（主 UP / 乙）高 34 = 7 + 20 + 7，正好等于 min-height；
//   - 带角色的条目（甲）高 50 = 7 + 20 + 16 + 7（此前 48.5）；
//   - 角色行相对条目顶的偏移 = padding 7 + name 盒 20 + role 的半个负
//     half-leading（(16 − 17)/2 = −0.5）。
//
// 条目自身的 Finder 矩形会被 name 的 +1px 字形位移带着一起下移（布局没动），
// 所以这里的偏移量都是相对量。
func TestCollabItemLineBoxes(t *testing.T) {
	_, tt := collabProbe(t, 12)
	_, first := openCollab(t, tt)

	roleItem, ok := tt.Find("甲")
	if !ok {
		t.Fatal("面板里看不到参与者「甲」")
	}
	noRole, ok := tt.Find("乙")
	if !ok {
		t.Fatal("面板里看不到参与者「乙」")
	}

	// 带角色：7 + 20 + 16 + 7 = 50。
	if math.Abs(float64(roleItem.H-50)) > 0.01 {
		t.Errorf("带角色条目高 %v，want 50（7 + name 20 + role 16 + 7）", roleItem.H)
	}
	// 无角色：7 + 20 + 7 = 34，等于 .video-owner-picker-item 的 min-height。
	if math.Abs(float64(noRole.H-34)) > 0.01 {
		t.Errorf("无角色条目高 %v，want 34（min-height，7 + name 20 + 7）", noRole.H)
	}

	roleText, ok := tt.Find("联合投稿")
	if !ok {
		t.Fatal("找不到角色行「联合投稿」")
	}
	nameH, _ := pxLineBox(collabNameSize, collabNameBoxH)
	_, roleShift := pxLineBox(collabRoleSize, collabRoleBoxH)
	want := collabItemPadY + nameH + roleShift // 7 + 20 − 0.25
	got := roleText.Y - roleItem.Y
	t.Logf("角色行相对条目顶 %.2f，want %.2f（条目 %+v / 角色 %+v）", got, want, roleItem, roleText)
	if math.Abs(float64(got-want)) > 0.5 {
		t.Errorf("角色行偏移 %v，want %v（name 盒 20 + padding 7 + 半个负 half-leading）", got, want)
	}
	// 条目上缘相对触发键：面板贴着键下 8px、面板 padding 4（沿用既有的
	// TestCollabMenuGeometry，这里只确认本轮没把面板整体推走）。
	if d := first.Y - (roleItem.Y - 34); d < -1.5 || d > 1.5 {
		t.Errorf("首条目上缘 %v，应约 %v（甲的上缘 − 首条目 34 高）", first.Y, roleItem.Y-34)
	}
}
