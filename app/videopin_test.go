package main

// 视频窗「置顶」的落盘与接线测试（主窗一侧）。
//
// 语义（见 view/state.go 的 VideoPinned 与 prefs.go 的 prefVideoPinned）：
//   - 默认 false —— 与主窗迷你模式的固定一致（迷你模式默认不置顶）；
//   - 落盘用**新键** videoPinned（KV 只增不改）；
//   - 迷你模式那颗不落盘（退出迷你模式就复位），视频窗的置顶要跟用户走，
//     所以两边各记各的，不复用同一个值。

import (
	"os"
	"strings"
	"testing"

	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/view"
)

// TestVideoPinnedPrefsRoundTrip 钉住落盘并能读回：改一次 → 重新打开 KV →
// 新 controller loadPrefs 读回同一个值；键就是 videoPinned（"true"/"false"）。
func TestVideoPinnedPrefsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	kv, err := store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &controller{kv: kv, app: view.NewApp(func() {})}

	c.setVideoPinned(true)
	if got := c.kv.String(prefVideoPinned); got != "true" {
		t.Fatalf("落盘值 = %q，想要 \"true\"", got)
	}
	if !c.app.VideoPinned {
		t.Fatal("setVideoPinned 没有更新 app.VideoPinned")
	}

	c.kv, err = store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c.app = view.NewApp(func() {})
	c.loadPrefs()
	if !c.app.VideoPinned {
		t.Fatal("置顶状态没有 round-trip（读回为 false）")
	}

	// 再取消一次，同样要落盘并读回。
	c.setVideoPinned(false)
	if got := c.kv.String(prefVideoPinned); got != "false" {
		t.Fatalf("取消置顶后的落盘值 = %q，想要 \"false\"", got)
	}
	c.kv, err = store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c.app = view.NewApp(func() {})
	c.loadPrefs()
	if c.app.VideoPinned {
		t.Fatal("取消置顶没有 round-trip（读回为 true）")
	}
}

// TestVideoPinnedDefaultMatchesMiniPin 钉住默认值语义：老版本没有这个键时
// 视频窗**不**置顶 —— 与主窗迷你模式的固定（app.Pinned）同一个默认值；
// 并且空值 / 非字面量不会把置顶打开（只认 "true"）。
func TestVideoPinnedDefaultMatchesMiniPin(t *testing.T) {
	c := &controller{kv: mustKV(t), app: view.NewApp(func() {})}
	c.loadPrefs()
	if c.app.Pinned {
		t.Fatal("迷你模式的固定默认应为 false（本测试的前提）")
	}
	if got := c.app.VideoPinned; got != c.app.Pinned {
		t.Errorf("视频窗置顶默认 %v，迷你模式固定默认 %v：两者应一致", got, c.app.Pinned)
	}
	if c.app.VideoPinned {
		t.Error("没有 videoPinned 键时不该置顶")
	}

	// 键名是新的、与迷你窗位置等既有键不冲突；非 "true" 的字面量一律当关。
	if prefVideoPinned != "videoPinned" {
		t.Errorf("落盘键 = %q，想要 videoPinned", prefVideoPinned)
	}
	for _, v := range []string{"", "1", "yes", "TRUE", "false"} {
		c := &controller{kv: mustKV(t), app: view.NewApp(func() {})}
		if err := c.kv.SetString(prefVideoPinned, v); err != nil {
			t.Fatal(err)
		}
		c.loadPrefs()
		if c.app.VideoPinned {
			t.Errorf("%q 被读成置顶，只有 \"true\" 才算", v)
		}
	}
	// "true" 才算开。
	c = &controller{kv: mustKV(t), app: view.NewApp(func() {})}
	if err := c.kv.SetString(prefVideoPinned, "true"); err != nil {
		t.Fatal(err)
	}
	c.loadPrefs()
	if !c.app.VideoPinned {
		t.Error("\"true\" 没有读成置顶")
	}
}

func mustKV(t *testing.T) *store.KV {
	t.Helper()
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	return kv
}

// TestVideoPinnedSourceWiring 钉住主窗这一侧的两根线：openVideo 把落盘值交给
// 视频窗，视频窗的点击经 OnPinToggle 回到 setVideoPinned。两者各是一行赋值
// （没有纯逻辑可单测），按本仓库既有做法核源码。
func TestVideoPinnedSourceWiring(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(string(src)), "")
	for _, want := range []string{
		"VideoPinned:c.app.VideoPinned,",
		"c.vid.OnPinToggle(func(onbool){",
		"c.setVideoPinned(on)",
		`_=c.kv.SetString(prefVideoPinned,strconv.FormatBool(on))`,
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("main.go 缺少接线：%q", want)
		}
	}
}
