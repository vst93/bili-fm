package view

import (
	"testing"
)

// TestVideoActionDisabledWithoutVideo 钉住内容操作行 / 工具条里几个「依赖
// 当前视频」的按钮在**没有视频**时是禁用态（原版 videoInfo.tsx 的
// `disabled={!bvid}` / `disabled={!cid}`）：
//
//	浏览器打开  !bvid
//	视频播放    !bvid
//	弹幕/评论    !cid
//	选集        !bvid
//
// 之前这几个键任何时候都能点：没有视频时点「弹幕/评论」会打开一个空抽屉、
// 点「视频播放」会打开空的视频弹窗 —— 看着可用却什么都不做。
func TestVideoActionDisabledWithoutVideo(t *testing.T) {
	a := NewApp(func() {})
	opens, videos, danmakus, parts := 0, 0, 0, 0
	a.Act.OpenBrowser = func() { opens++ }
	a.Act.OpenVideo = func() { videos++ }
	a.Act.ToggleDanmaku = func() { danmakus++ }
	a.Act.OpenParts = func(Track) { parts++ }
	// 有曲目但还没拉到详情：原版 bvid 为空 ⇒ 选集也禁用（openParts 否则会
	// 打开抽屉，所以要给一个非 nil 的 Track 才能看出「禁用挡住了」）。
	a.Track = &Track{Bvid: "", Title: "未知"}
	a.Queue = []Track{*a.Track}
	a.Index = 0

	tt := shellTester(t, a)
	for _, label := range []string{"浏览器打开", "视频播放", "弹幕、评论、章节与字幕", "选集"} {
		_ = tt.Click(label) // 禁用键点不到，忽略返回值
	}
	tt.Frame()
	if opens != 0 || videos != 0 || danmakus != 0 || parts != 0 {
		t.Fatalf("没有视频时按钮仍触发：open=%d video=%d danmaku=%d parts=%d",
			opens, videos, danmakus, parts)
	}
	if a.Drawer != "" {
		t.Fatalf("没有视频时打开了抽屉：%q", a.Drawer)
	}
}

// TestVideoActionEnabledWithVideo 是上一条的另一半：有了详情（bvid/cid 都在）
// 后每个键都恢复可点。每个键用一份独立的 App —— 弹幕/选集会把抽屉打开盖住
// 后面的按钮，同一次会话里连着点测不准。
func TestVideoActionEnabledWithVideo(t *testing.T) {
	info := func() *Info {
		return &Info{Bvid: "BV1test", Aid: 1, Cid: 2, Title: "标题", OwnerMid: 5, OwnerName: "UP"}
	}
	cases := []struct {
		label string
		hit   func(a *App, n *int)
	}{
		{"浏览器打开", func(a *App, n *int) { a.Act.OpenBrowser = func() { *n++ } }},
		{"视频播放", func(a *App, n *int) { a.Act.OpenVideo = func() { *n++ } }},
		{"弹幕、评论、章节与字幕", func(a *App, n *int) { a.Act.ToggleDanmaku = func() { *n++ } }},
		{"选集", func(a *App, n *int) { a.Act.OpenParts = func(Track) { *n++ } }},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			a := NewApp(func() {})
			a.Info = info()
			a.Track = &Track{Bvid: "BV1test", Title: "标题"}
			a.Queue = []Track{*a.Track}
			a.Index = 0
			n := 0
			tc.hit(a, &n)
			tt := shellTester(t, a)
			if err := tt.Click(tc.label); err != nil {
				t.Fatalf("有视频时点不到 %q：%v", tc.label, err)
			}
			tt.Frame()
			if n != 1 {
				t.Fatalf("有视频时 %q 没触发：n=%d", tc.label, n)
			}
		})
	}
}

// TestCollabOwnerFallback 钉住合作视频「选中参与者后显示 TA」的换算：
// 选中项属于当前视频才生效，换视频（CollabFor 不匹配）自动退回主 UP ——
// 对应原版 videoInfo 的 displayedOwner 与 bvid 变化时的 setSelectedOwner(null)。
func TestCollabOwnerFallback(t *testing.T) {
	a := NewApp(func() {})
	a.Info = &Info{
		Bvid: "BV1collab", OwnerMid: 100, OwnerName: "主UP", OwnerFace: "face-owner",
		Staff: []Collaborator{
			{Mid: 100, Name: "主UP", Face: "face-owner"},
			{Mid: 200, Name: "合作者", Title: "联合投稿", Face: "face-2"},
		},
	}
	if a.ownerName() != "主UP" || a.ownerFace() != "face-owner" {
		t.Fatalf("默认应显示主 UP，得到 %q / %q", a.ownerName(), a.ownerFace())
	}

	a.CollabMid, a.CollabFor = 200, "BV1collab"
	if a.ownerName() != "合作者" || a.ownerFace() != "face-2" {
		t.Fatalf("选中后应显示合作者，得到 %q / %q", a.ownerName(), a.ownerFace())
	}

	// 换视频：选择失效，退回主 UP。
	a.CollabFor = "BVother"
	if a.ownerName() != "主UP" || a.ownerFace() != "face-owner" {
		t.Fatalf("换视频后应退回主 UP，得到 %q / %q", a.ownerName(), a.ownerFace())
	}
}

// TestCollabPickerMenu 无头点开合作 UP 主选择键，钉住：
//  1. 只有 staff > 1（合作视频）才出现该键；
//  2. 面板里列出全部参与者（含主 UP），每人一行名字 +（有的话）角色；
//  3. 选一个就把面板显示的 UP 主换成 TA，并以 TA 的 mid/名字打开空间
//     （原版 onAction 的 setSelectedOwner + onOwnerClick），面板随即收起。
//
// 视觉细节（玻璃面板 / 三态 / 蓝字选中）在 collabpicker_test.go 里无头实测。
func TestCollabPickerMenu(t *testing.T) {
	// 单 UP 视频：不画选择键。
	single := NewApp(func() {})
	single.Info = &Info{Bvid: "BV1solo", OwnerMid: 100, OwnerName: "独UP",
		Staff: []Collaborator{{Mid: 100, Name: "独UP"}}}
	ttSingle := shellTester(t, single)
	if _, ok := ttSingle.Find("选择合作 UP 主"); ok {
		t.Fatal("单 UP 视频不该出现合作 UP 主选择键")
	}

	a := NewApp(func() {})
	a.Info = &Info{
		Bvid: "BV1collab", OwnerMid: 100, OwnerName: "主UP", OwnerFace: "face-owner",
		Staff: []Collaborator{
			{Mid: 100, Name: "主UP", Face: "face-owner"},
			{Mid: 200, Name: "合作者", Title: "联合投稿", Face: "face-2"},
		},
	}
	var gotMid int64
	var gotName string
	a.Act.OpenUp = func(mid int64, name string) { gotMid, gotName = mid, name }

	tt := shellTester(t, a)
	if err := tt.Click("选择合作 UP 主"); err != nil {
		t.Fatalf("点不到合作 UP 主选择键：%v", err)
	}
	tt.Frame()
	if !a.CollabOpen {
		t.Fatal("点选择键后面板没打开")
	}
	// 角色文案只出现在面板里，用它判定面板是否在屏上。
	if _, ok := tt.Find("联合投稿"); !ok {
		t.Fatal("面板里看不到参与者「合作者」的角色")
	}
	if err := tt.Click("合作者"); err != nil {
		t.Fatalf("选不中合作者：%v", err)
	}
	tt.Frame()
	if gotMid != 200 || gotName != "合作者" {
		t.Fatalf("选合作者应打开 TA 的空间，得到 mid=%d name=%q", gotMid, gotName)
	}
	if a.ownerName() != "合作者" || a.ownerFace() != "face-2" {
		t.Fatalf("选中后面板应显示合作者，得到 %q / %q", a.ownerName(), a.ownerFace())
	}
	if a.CollabOpen {
		t.Fatal("选中条目后面板没收起")
	}
	if _, ok := tt.Find("联合投稿"); ok {
		t.Fatal("面板收起后角色文案还在屏上")
	}

	// 再打开：选中的那一条就是当前显示的合作者（原版 selectedKeys）。
	if err := tt.Click("选择合作 UP 主"); err != nil {
		t.Fatalf("再次点不到选择键：%v", err)
	}
	tt.Frame()
	checked := a.collabChecker()
	if !checked(a.Info.Staff[1]) {
		t.Fatal("重开面板后合作者应是选中项")
	}
	if checked(a.Info.Staff[0]) {
		t.Fatal("主 UP 不该是选中项")
	}
}
