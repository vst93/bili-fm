package view

import (
	"bytes"
	"image"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// playerMuteRect 返回音量弹层里静音键的矩形。标签随静音态变化（未静音「静音」/
// 已静音「取消静音」），两种都认。
func playerMuteRect(t *testing.T, tst *ui.Tester) ui.Rect {
	t.Helper()
	if r, ok := tst.Find("静音"); ok {
		return r
	}
	if r, ok := tst.Find("取消静音"); ok {
		return r
	}
	t.Fatal("音量弹层里找不到静音键（弹层没打开？）")
	return ui.Rect{}
}

// muteFillSamples 取静音键内的填充取样点（避开居中的 16px 图标与 1px 描边，
// 也避开悬停时浮在键上方的 tooltip——它会把顶部几行盖住）。
var muteFillSamples = [][2]int{{3, 3}, {3, 12}, {3, 22}, {22, 3}, {12, 22}}

func sampleFill(img *image.RGBA, r ui.Rect) [][3]int {
	out := make([][3]int, 0, len(muteFillSamples))
	for _, p := range muteFillSamples {
		out = append(out, rgbAt(img, int(r.X)+p[0], int(r.Y)+p[1]))
	}
	return out
}

// TestPlayerMuteSurfaceTokens 钉住静音键（原版 `#player .player-volume-mute-button`）
// 的三组令牌：
//
//	border-color: rgba(148,163,184,.2)  /* 常态 */  / rgba(2,132,199,.28) /* data-active */
//	box-shadow: inset 0 1px 1px rgba(255,255,255,.44), 0 2px 7px rgba(15,23,42,.045)
//	color: #526174 /* 常态 */ / var(--studio-blue) #0284c7 /* data-active */
//
// 以及一个关键否定：`[data-active]` **不换底色**（始终是 --liquid-bg 白 .22），
// 之前原生版给静音态铺了 Rose 底/墨，整键泛红。
func TestPlayerMuteSurfaceTokens(t *testing.T) {
	lt := ThemeFor(PeriodAt(12))
	if got := playerMuteBorder(lt, false); got != ui.Hex("#94a3b8").Alpha(0.20) {
		t.Errorf("常态描边 = %v，应为 slate .2", got)
	}
	if got := playerMuteBorder(lt, true); got != ui.Hex("#0284c7").Alpha(0.28) {
		t.Errorf("已静音描边 = %v，应为 studio-blue .28", got)
	}
	if got := playerMuteInk(lt, false); got != ui.Hex("#526174") {
		t.Errorf("常态墨色 = %v，应为 #526174", got)
	}
	if got := playerMuteInk(lt, true); got != ui.Hex("#0284c7") {
		t.Errorf("已静音墨色 = %v，应为 studio-blue #0284c7", got)
	}
	// 明确不是 Rose（上一版整键泛红的根因）。
	if playerMuteInk(lt, true) == ui.Hex("#e11d48") {
		t.Error("已静音墨色仍是 Rose，应转 studio-blue")
	}
	// 深色（烟熏玻璃）时段：静音键也是 .player-button，基态换成白 .15 边 /
	// Theme.PlayerButton 墨；`[data-active]` (1,2,0) 输给深色基态 (1,2,1)，
	// 所以深色下已静音**不转蓝**。
	dt := ThemeFor(PeriodAt(0))
	if got := playerMuteBorder(dt, false); got != ui.Hex("#ffffff").Alpha(0.15) {
		t.Errorf("深色常态描边 = %v，应为白 .15", got)
	}
	if got := playerMuteBorder(dt, true); got != ui.Hex("#ffffff").Alpha(0.15) {
		t.Errorf("深色已静音描边 = %v，应为白 .15（不转蓝）", got)
	}
	if got := playerMuteInk(dt, false); got != dt.PlayerButton {
		t.Errorf("深色常态墨色 = %v，应为 Theme.PlayerButton", got)
	}
	if got := playerMuteInk(dt, true); got != dt.PlayerButton {
		t.Errorf("深色已静音墨色 = %v，应为 Theme.PlayerButton（不转蓝）", got)
	}
}

// TestPlayerMuteHoverKeepsSurface 无头实测：悬停静音键**不改面**（原版这颗键
// 不在工具键悬停名单 next/volume/speed/eq 里，`:hover` 对它无规则）。上一版
// 误铺了 GlassHover，一悬停键面就变亮。判据：静止与悬停两帧在键内填充取样点
// 上的颜色完全一致（取样点避开悬停 tooltip 覆盖的顶部几行）。
func TestPlayerMuteHoverKeepsSurface(t *testing.T) {
	_, tst := playerBarProbe(t, func(a *App) { a.ShowVolume = true })
	tst.Move(-1, -1)
	tst.Frame()
	rect := playerMuteRect(t, tst)
	tst.Frame()
	rest := sampleFill(tst.Image(), rect)

	tst.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	tst.Frame()
	tst.Frame()
	hoverRect := playerMuteRect(t, tst)
	hover := sampleFill(tst.Image(), hoverRect)

	for i, p := range muteFillSamples {
		t.Logf("取样点 %v：静止 %v、悬停 %v", p, rest[i], hover[i])
		if rest[i] != hover[i] {
			t.Errorf("悬停改变了静音键填充点 %v：%v → %v（原版 :hover 不改键面）",
				p, rest[i], hover[i])
		}
	}
}

// TestPlayerMuteSurfaceWired 无头反证：视图真的把 playerMuteSurface 接在静音键上，
// 且把当前静音态当 `muted` 传进去（对应原版的 `data-active`）。把 playerMuteSurface
// 换成记录参数的桩，已静音时最后一次调用应带 muted=true。上一版根本没走这个函数
// （自己在那段代码里手写 `if Muted { Rose } else if Hovered { GlassHover }`）。
func TestPlayerMuteSurfaceWired(t *testing.T) {
	orig := playerMuteSurface
	defer func() { playerMuteSurface = orig }()
	var got []bool
	playerMuteSurface = func(t Theme, b ui.Element, muted bool) ui.Element {
		got = append(got, muted)
		return orig(t, b, muted)
	}
	_, tst := playerBarProbe(t, func(a *App) {
		a.ShowVolume = true
		a.Muted = true
	})
	_ = tst
	if len(got) == 0 {
		t.Fatal("静音键没走 playerMuteSurface（材质没接上？）")
	}
	if !got[len(got)-1] {
		t.Errorf("已静音时 playerMuteSurface 收到 muted=false")
	}
}

// TestPlayerMuteCheckedWired 钉住音量弹层静音键把按压态交给无障碍层：原版这颗键是
// `aria-pressed={volume === 0}`，mygo 的对应 API 是 Checked(on)（toggle button
// 角色下屏幕阅读器读作「已按下」）。它和视觉/标签读同一个 `muted` 判据——标签、
// 材质、墨色都对了，但按压态此前没接线，读屏用户听到的「取消静音」按钮是「未按下」。
//
// 无头 Tester 不暴露无障碍树（mygo 的 ui 包只把 Access/Checked 写进内部 tree，
// 没有导出查询），所以沿用 cardtooltip_test.go 的手法：在源码里确认 `.Checked(muted)`
// 真接在静音键那一串上（写了常量没接等于没写），并断言主窗音量键**没有**被误接
// （它是弹出层开关，原版是 `aria-expanded` 语义而非 `aria-pressed`）。
func TestPlayerMuteCheckedWired(t *testing.T) {
	src, err := os.ReadFile("playerbar.go")
	if err != nil {
		t.Fatalf("读 playerbar.go 失败：%v", err)
	}

	// 静音键：Label/Tooltip 后面紧跟 Checked(muted)。把源码里的空白全部去掉后逐字
	// 匹配整条链，避免只搜到别处的 Checked(...)。RoleToggleButton 是原版隐式
	// toggle button 角色（aria-pressed 的 button）的对应角色，各平台靠它把 Checked
	// 读作「已按下」，也一并钉住。
	flat := strings.Join(strings.Fields(string(src)), "")
	chain := strings.Join(strings.Fields(`Center().Role(ui.RoleToggleButton).Label(pick(muted, "取消静音", "静音")).Tooltip(pick(muted, "取消静音", "静音")).Checked(muted)`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("音量弹层静音键没把 Role(ui.RoleToggleButton) / Checked(muted) 接在标签链上（原版 aria-pressed={volume===0} 的 button）")
	}

	// 反证：主窗音量键是弹出层开关，不应有 Checked（原版只有 aria-label/title 加
	// `aria-expanded`，是 Expanded 语义而不是 Checked/aria-pressed）。
	volChain := strings.Join(strings.Fields(`Label("音量").Tooltip("音量")`), "")
	if i := strings.Index(flat, volChain); i >= 0 {
		// 取这段到函数末尾（播放栏音量键体积很小）的一小段，确认其中没有 Checked。
		seg := flat[i:min(len(flat), i+400)]
		if strings.Contains(seg, ".Checked(") || strings.Contains(seg, "Checked(") {
			t.Error("主窗音量键被误接 Checked：原版它是 aria-expanded 的弹层开关，不是 aria-pressed")
		}
	}

	// 反证：静音键的 Checked(muted) 只该接一次（均衡 / 恰饭键各有自己的 Checked，
	// 但那两颗读的是 a.EQ / a.Sponsor，不是 muted）。
	if n := bytes.Count(src, []byte("Checked(muted)")); n != 1 {
		t.Errorf("playerbar.go 里 Checked(muted) 出现 %d 次，应只在静音键接一次", n)
	}
}

// TestPlayerMuteActiveTurnsBlue 无头实测：已静音时墨色转 studio-blue、描边转蓝。
// 取「未静音 vs 已静音」两帧的图标核心像素比较：未静音偏中性灰（#526174），
// 已静音应偏蓝（B>R）。上一版已静音用的是 Rose（R>B）。
func TestPlayerMuteActiveTurnsBlue(t *testing.T) {
	_, off := playerBarProbe(t, func(a *App) { a.ShowVolume = true })
	offRect := playerMuteRect(t, off)
	off.Move(-1, -1)
	off.Frame()

	_, on := playerBarProbe(t, func(a *App) {
		a.ShowVolume = true
		a.Muted = true
	})
	onRect := playerMuteRect(t, on)
	on.Move(-1, -1)
	on.Frame()

	core := func(img *image.RGBA, r ui.Rect) [3]int {
		return darkestIn(img, int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
	}
	offCore := core(off.Image(), offRect)
	onCore := core(on.Image(), onRect)
	t.Logf("静音键图标核心：未静音 %v、已静音 %v", offCore, onCore)
	// 已静音转 studio-blue：蓝通道明显压过红通道（#0284c7 的 B-R=197）。
	if onCore[2]-onCore[0] < 100 {
		t.Errorf("已静音图标核心应明显偏蓝（B-R≥100），实测 %v", onCore)
	}
	// 未静音是中性灰蓝 #526174（B-R 只有 34，且远不如 studio-blue 饱和）。
	if offCore[2]-offCore[0] >= 100 {
		t.Errorf("未静音图标核心不应是饱和蓝 #526174，实测 %v", offCore)
	}
	// 两态都不是 Rose（上一版已静音用 Rose，R 会压过 B）。
	if offCore[0] > offCore[2] || onCore[0] > onCore[2] {
		t.Errorf("图标核心泛红（R>B），不应是 Rose：未静音 %v、已静音 %v", offCore, onCore)
	}
}
