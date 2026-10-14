package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（烟熏玻璃时段播放栏调参）的无头接线测试。纯函数已把每颗键的深浅取值
// 钉死（playerbar_align_test.go 的 TestPlayerToolPaintDark / TestPlayerPlayPaintDark、
// mutebutton_test.go 与 minibg_test.go 的深色分支），这里再补一条端到端判据：
// 深色令牌**真的**被视图送进播放栏绘制，而不是只停留在被测试直接调用的纯函数里。
//
// 手法：换桩 playerFadeColor —— 视图用它把每颗键的目标色交给动画层
// （playerToolSurface / playerPlaySurface / 各键的图标着色都经过它），
// 记录「键名 → 目标色」，同一套键在亮/暗两次渲染下应给出深浅两套值。

// TestPlayerBarDarkThemeWired 无头反证：把主题换成午夜（烟熏玻璃）后，同一套
// 播放栏键的目标色整体转成深色值；亮色基线与深色值同时钉住，任一侧走错都变红。
func TestPlayerBarDarkThemeWired(t *testing.T) {
	orig := playerFadeColor
	defer func() { playerFadeColor = orig }()

	render := func(theme Theme) map[string]ui.Color {
		got := map[string]ui.Color{}
		playerFadeColor = func(c *ui.Context, b ui.Element, key string, target ui.Color) ui.Color {
			got[key] = target
			return orig(c, b, key, target)
		}
		_, tst := playerBarProbe(t, func(a *App) { a.Theme = theme })
		_ = tst
		return got
	}

	lt := ThemeFor(PeriodAt(12))
	dt := ThemeFor(PeriodAt(0))
	if dt.Dark != true {
		t.Fatal("midnight 应为烟熏玻璃时段")
	}

	light := render(lt)
	dark := render(dt)

	// 亮色基线：先确认换桩真的抓到了这些键（否则下面的深色断言会静默空过）。
	if got := light["player-tool-bg"]; got != ui.Hex("#ffffff").Alpha(0.22) {
		t.Fatalf("亮色工具键底色 = %v，换桩没抓到键（应为白 .22）", got)
	}
	if got := light["player-tool-border"]; got != ui.Hex("#94a3b8").Alpha(0.20) {
		t.Fatalf("亮色工具键描边 = %v，换桩没抓到键（应为 slate .2）", got)
	}
	if got := light["player-play-ink"]; got != ui.Hex("#475569") {
		t.Errorf("亮色暂停图标 = %v，应为 #475569", got)
	}

	// 深色：工具键底/边换成白 .085 / 白 .15。
	if got := dark["player-tool-bg"]; got != ui.Hex("#ffffff").Alpha(0.085) {
		t.Errorf("深色工具键底色 = %v，应为白 .085", got)
	}
	if got := dark["player-tool-border"]; got != ui.Hex("#ffffff").Alpha(0.15) {
		t.Errorf("深色工具键描边 = %v，应为白 .15", got)
	}
	// 深色：五颗工具键的墨色都换成 Theme.PlayerButton（rgba(51,65,85,.84)）。
	for _, key := range []string{
		"player-next-ink", "player-volume-ink", "player-speed-ink",
		"player-eq-ink", "player-sponsor-ink",
	} {
		if got := dark[key]; got != dt.PlayerButton {
			t.Errorf("深色 %s = %v，应为 Theme.PlayerButton %v", key, got, dt.PlayerButton)
		}
	}
	// 深色播放键基态固定 #0369a1（与播放中同色，播放与否同一个面）。
	if got := dark["player-play-ink"]; got != ui.Hex("#0369a1") {
		t.Errorf("深色播放键图标 = %v，应为 #0369a1", got)
	}
}
