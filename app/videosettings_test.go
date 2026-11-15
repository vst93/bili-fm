package main

// 视频窗设置面板在主窗这一侧的接线与落盘（用户 2026-11：「播放页面的设置不要
// 放在菜单栏下，统一在播放窗口下增加一个设置按钮」）。
//
// 面板本身（开合 / 每项交互 / 小窗口滚动）在 internal/view 的无头测试里；
// 这里钉主窗的两条线：
//
//   - 弹幕 5 项：面板改 → OnDanmakuStyle → setDanmakuStyle → KV（danmaku*
//     那一组既有键，只增不改）→ 重新读回还在；
//   - 原生视频播放：面板改 → OnNativeToggle → setNativeVideo → KV
//     nativeVideoPlayer + Manager 的开关立即生效（下一集起播按新值）；
//   - 这两个回调在 main.go 里接上（各是一行，按本仓库既有做法核源码）。

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/vst93/bili-fm/app/internal/danmaku"
	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/view"
)

// TestVideoSettingsDanmakuPersists 钉住面板里那 4 项（不透明度 / 字号 /
// 显示区域 / 同屏上限）的落盘与读回：setDanmakuStyle 是面板与顶栏共用的
// 唯一入口，落盘键是既有的 danmaku* 一组。
func TestVideoSettingsDanmakuPersists(t *testing.T) {
	dir := t.TempDir()
	kv, err := store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &controller{kv: kv, app: view.NewApp(func() {})}

	s := danmaku.DefaultSettings()
	s.Opacity = 0.5
	s.Size = 2
	s.Area = 0.5
	s.Max = danmaku.MaxOnScreen(0)
	c.setDanmakuStyle(s)

	if got := c.kv.String(prefDanmakuOpacity); got == "" || got != "0.5" {
		t.Errorf("落盘不透明度 = %q，想要 0.5", got)
	}
	if got := c.kv.String(prefDanmakuFontSize); got != "2" {
		t.Errorf("落盘字号 = %q，想要 2", got)
	}
	if got := c.kv.String(prefDanmakuArea); got != "half" {
		t.Errorf("落盘显示区域 = %q，想要 half", got)
	}
	if got := c.kv.String(prefDanmakuMaxOnScreen); got != strconv.Itoa(danmaku.MaxOnScreen(0)) {
		t.Errorf("落盘同屏上限 = %q，想要 %d", got, danmaku.MaxOnScreen(0))
	}

	// 重新打开 KV 读回：面板里调过的值重启后还在。
	c.kv, err = store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c.app = view.NewApp(func() {})
	c.loadDanmakuPrefs()
	got := c.app.DanmakuStyle
	if got.Opacity != 0.5 || got.Size != 2 || got.Area != 0.5 || got.Max != danmaku.MaxOnScreen(0) {
		t.Fatalf("弹幕设置没有 round-trip：%+v", got)
	}
}

// TestVideoSettingsNativeTogglePersists 钉住设置面板里的「原生视频播放」：
// 关掉立刻落盘（既有的 nativeVideoPlayer，只增不改），下一次起播按新值走
// WebView，重新打开应用仍然读回同一个值。
func TestVideoSettingsNativeTogglePersists(t *testing.T) {
	dir := t.TempDir()
	kv, err := store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &controller{kv: kv, app: view.NewApp(func() {})}

	c.setNativeVideo(false)
	if c.app.NativeVideo {
		t.Error("关掉后 app.NativeVideo 应为假")
	}
	if got := c.kv.String(prefNativeVideo); got != "false" {
		t.Errorf("落盘 = %q，想要 \"false\"", got)
	}

	c.kv, err = store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c.app = view.NewApp(func() {})
	c.loadPrefs()
	if c.app.NativeVideo {
		t.Fatal("关掉的开关没有 round-trip（读回为 true）")
	}

	// 再打开：同样落盘并读回。
	c.setNativeVideo(true)
	if got := c.kv.String(prefNativeVideo); got != "true" {
		t.Errorf("重新打开后落盘 = %q，想要 \"true\"", got)
	}
	c.kv, err = store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c.app = view.NewApp(func() {})
	c.loadPrefs()
	if !c.app.NativeVideo {
		t.Fatal("重新打开的开关没有 round-trip（读回为 false）")
	}
}

// TestVideoSettingsSourceWiring 钉住 main.go 的两根线：视频窗设置面板里的
// 弹幕设置与「原生视频播放」各自回到主窗的落盘入口。原生的入口**只**在
// 面板里（主窗菜单与 macOS 菜单里都删了），所以这两根线不能断。
func TestVideoSettingsSourceWiring(t *testing.T) {
	flat := func(path string) string {
		t.Helper()
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", path, err)
		}
		return strings.Join(strings.Fields(string(src)), "")
	}
	mainSrc := flat("main.go")
	for _, want := range []string{
		"c.vid.OnDanmakuStyle(func(sdanmaku.Settings){",
		"c.setDanmakuStyle(s)",
		"c.vid.OnNativeToggle(func(onbool){",
		"c.setNativeVideo(on)",
	} {
		if !strings.Contains(mainSrc, want) {
			t.Errorf("main.go 缺少接线：%q", want)
		}
	}

	// 主窗的设置下拉与 macOS 原生菜单都不再建这些入口（用户明确不要两处）。
	menuSrc := flat("internal/view/settingsmenu.go")
	for _, dead := range []string{`settingsToggle(c,"原生视频播放"`, "danmakuSettingsPanel", "SettingsDanmaku"} {
		if strings.Contains(menuSrc, dead) {
			t.Errorf("主窗设置菜单里仍有 %q（应只在视频窗设置面板里）", dead)
		}
	}
	appMenu := flat("menu.go")
	if strings.Contains(appMenu, "menuToggleDanmaku") {
		t.Error("macOS 原生菜单里仍有「弹幕」复选框（入口应只在视频窗）")
	}
}
