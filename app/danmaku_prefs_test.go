package main

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/danmaku"
	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/video"
	"github.com/vst93/bili-fm/app/internal/view"
)

// TestDanmakuPrefsRoundTrip 钉住弹幕设置的五项都落盘并能读回：
// 总开关 / 不透明度 / 字号档 / 显示区域 / 同屏上限。
func TestDanmakuPrefsRoundTrip(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	want := danmaku.Settings{On: false, Opacity: 0.5, Size: 2, Area: 0.5, Max: 30}

	c := &controller{app: view.NewApp(func() {}), kv: kv, vid: video.New()}
	c.setDanmakuStyle(want)

	// 键名与值都必须是稳定、可读的（KV 键只增不改）。
	for key, value := range map[string]string{
		prefDanmakuOn:          "false",
		prefDanmakuOpacity:     "0.5",
		prefDanmakuFontSize:    "2",
		prefDanmakuArea:        "half",
		prefDanmakuMaxOnScreen: "30",
	} {
		if got := kv.String(key); got != value {
			t.Errorf("kv[%s] = %q，想要 %q", key, got, value)
		}
	}
	// 立即生效：设置在下发的同一刻就到了视频管理器。
	if got := c.vid.DanmakuSettings(); got != want {
		t.Errorf("视频管理器设置 = %+v，想要 %+v", got, want)
	}

	// 重启读回：新的 App + controller 从同一份 KV 恢复。
	a2 := view.NewApp(func() {})
	c2 := &controller{app: a2, kv: kv, vid: video.New()}
	c2.loadDanmakuPrefs()
	if got := a2.DanmakuStyle; got != want {
		t.Errorf("读回 = %+v，想要 %+v", got, want)
	}
}

// TestDanmakuPrefsDefaults 钉住老版本（没有这些键）时的默认值：开 / 90% /
// 中 / 整屏 / 60。不能让缺键变成「关」或 0 字号。
func TestDanmakuPrefsDefaults(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	a := view.NewApp(func() {})
	c := &controller{app: a, kv: kv, vid: video.New()}
	c.loadDanmakuPrefs()
	want := danmaku.DefaultSettings()
	if a.DanmakuStyle != want {
		t.Errorf("默认设置 = %+v，想要 %+v", a.DanmakuStyle, want)
	}
	if got := c.vid.DanmakuSettings(); got != want {
		t.Errorf("视频管理器默认设置 = %+v，想要 %+v", got, want)
	}
}

// TestDanmakuPrefsClamp 钉住脏值不会把排版搞坏：手改 KV 写 0 字号 / 负上限 /
// 5 倍不透明度，读回时夹到合法区间。
func TestDanmakuPrefsClamp(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	_ = kv.SetString(prefDanmakuOpacity, "5")
	_ = kv.SetString(prefDanmakuFontSize, "9")
	_ = kv.SetString(prefDanmakuMaxOnScreen, "-3")
	a := view.NewApp(func() {})
	c := &controller{app: a, kv: kv, vid: video.New()}
	c.loadDanmakuPrefs()
	if a.DanmakuStyle.Opacity != danmaku.DefaultSettings().Opacity {
		t.Errorf("不透明度 = %v，应退回默认 %v", a.DanmakuStyle.Opacity, danmaku.DefaultSettings().Opacity)
	}
	if a.DanmakuStyle.Size != 1 {
		t.Errorf("字号档 = %d，应退回中档 1", a.DanmakuStyle.Size)
	}
	if a.DanmakuStyle.Max != danmaku.DefaultSettings().Max {
		t.Errorf("同屏上限 = %d，应退回默认 %d", a.DanmakuStyle.Max, danmaku.DefaultSettings().Max)
	}
}
