package main

// 自动检查更新的门禁 / 只提示一次 / 不打断（见 update_auto.go）的无头断言。
//
// 全部用注入的固定时钟、内存 KV 替身与假的检查函数：不联网、不建窗口、
// 不启动应用。检查本身在 goroutine 里跑，需要等结果的用例统一用 waitUpdate
// 轮询 KV（store.KV 的 String/SetString 本身就带锁，替身也照此实现）。
//
// 每个「应该发生」都配一条反证（不该发生的用例断言计数 / 键为空）。

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/view"
)

// updateKV 是 prefKV 的内存替身：记录每个键被写了几次，便于断言「Store
// 版一个键都不写」。
type updateKV struct {
	mu     sync.Mutex
	vals   map[string]string
	writes []string
}

func newUpdateKV() *updateKV { return &updateKV{vals: map[string]string{}} }

func (k *updateKV) String(key string) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.vals[key]
}

func (k *updateKV) SetString(key, value string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.vals[key] = value
	k.writes = append(k.writes, key)
	return nil
}

func (k *updateKV) writeCount() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.writes)
}

// updateClock 是测试用的可变时钟（跨天用例直接改它）。
type updateClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *updateClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *updateClock) addDay() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.AddDate(0, 0, 1)
}

// newUpdateController 起一个只有更新流程所需字段的 controller。
func newUpdateController(t *testing.T, kv *updateKV, clock *updateClock,
	check func(context.Context) (*mygo.Update, error)) *controller {
	t.Helper()
	c := &controller{
		app:            view.NewApp(func() {}),
		kv:             kv,
		now:            clock.now,
		checkUpdateFn:  check,
		updaterEnabled: func() bool { return true },
		storeInstall:   func() bool { return false },
	}
	return c
}

// waitUpdate 等一个条件成立（最多 2s）。
func waitUpdate(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("等待 %s 超时", what)
}

func updateTestClock() *updateClock {
	return &updateClock{t: time.Date(2026, 3, 7, 9, 30, 0, 0, time.Local)}
}

// ---------------------------------------------------------------- 每日一次

// TestAutoUpdateDailyGate 钉住：当天第一次自动检查会跑，同一天第二次启动
// 不再跑，跨天再跑一次；每次都把当天日期写进 updateCheckedOn。
func TestAutoUpdateDailyGate(t *testing.T) {
	kv := newUpdateKV()
	clock := updateTestClock()
	var calls atomic.Int32
	check := func(context.Context) (*mygo.Update, error) {
		calls.Add(1)
		return nil, nil // 已是最新也算「查过」
	}

	c := newUpdateController(t, kv, clock, check)
	c.runUpdateCheck(false)
	waitUpdate(t, "首次自动检查写入当天日期", func() bool {
		return kv.String(prefUpdateCheckedOn) == "2026-03-07"
	})
	if got := calls.Load(); got != 1 {
		t.Fatalf("首次自动检查发起 %d 次，想要 1", got)
	}

	// 同一天第二次启动：门禁在 runUpdateCheck 入口同步拦掉，不发起网络。
	c2 := newUpdateController(t, kv, clock, check)
	c2.runUpdateCheck(false)
	if got := calls.Load(); got != 1 {
		t.Fatalf("同一天第二次启动又检查了（累计 %d 次）", got)
	}
	if got := kv.String(prefUpdateCheckedOn); got != "2026-03-07" {
		t.Errorf("同一天跳过时改坏了 updateCheckedOn：%q", got)
	}

	// 跨天：日期变了，再查一次，键跟着前进。
	clock.addDay()
	c3 := newUpdateController(t, kv, clock, check)
	c3.runUpdateCheck(false)
	waitUpdate(t, "跨天后再次检查并写新日期", func() bool {
		return kv.String(prefUpdateCheckedOn) == "2026-03-08" && calls.Load() == 2
	})
}

// TestManualUpdateBypassesDailyGate 钉住：当天已自动查过，手动检查仍然跑，
// 成功后刷新 updateCheckedOn。
func TestManualUpdateBypassesDailyGate(t *testing.T) {
	kv := newUpdateKV()
	kv.vals[prefUpdateCheckedOn] = "2026-03-01" // 当天已查过（旧日期）
	clock := updateTestClock()
	var calls atomic.Int32
	check := func(context.Context) (*mygo.Update, error) {
		calls.Add(1)
		return nil, nil
	}
	c := newUpdateController(t, kv, clock, check)
	c.runUpdateCheck(true)

	waitUpdate(t, "手动检查完成后刷新日期", func() bool {
		return calls.Load() == 1 && kv.String(prefUpdateCheckedOn) == "2026-03-07"
	})
}

// ---------------------------------------------------------------- 只提示一次

// TestAutoPromptOncePerVersion 钉住：自动检查命中同一版本不弹；不同版本弹
// 并记录；手动检查命中同一版本仍然弹（不受这个键限制）。
func TestAutoPromptOncePerVersion(t *testing.T) {
	kv := newUpdateKV()
	kv.vals[prefUpdatePromptedVersion] = "1.2.3"
	clock := updateTestClock()
	c := newUpdateController(t, kv, clock, nil)

	// 同一版本：不弹，但检查结果照常算「今天查过」。
	c.updateGen = 1
	c.finishUpdateCheck(false, 1, "2026-03-07", &mygo.Update{Version: "1.2.3"}, nil)
	if c.app.Modal != nil {
		t.Fatalf("同一版本自动检查不该弹框，实际 %+v", c.app.Modal)
	}
	if got := kv.String(prefUpdatePromptedVersion); got != "1.2.3" {
		t.Errorf("同一版本不该改写 updatePromptedVersion：%q", got)
	}
	if got := kv.String(prefUpdateCheckedOn); got != "2026-03-07" {
		t.Errorf("自动检查没写 updateCheckedOn：%q", got)
	}

	// 新版本：弹，并在弹的这一刻记录版本（「稍后再说」也算已提示）。
	c.updateGen = 2
	c.finishUpdateCheck(false, 2, "2026-03-07", &mygo.Update{Version: "1.3.0"}, nil)
	if c.app.Modal == nil || c.app.Modal.Title != "发现新版本" {
		t.Fatalf("新版本没有弹更新框：%+v", c.app.Modal)
	}
	if got := kv.String(prefUpdatePromptedVersion); got != "1.3.0" {
		t.Errorf("弹框后 updatePromptedVersion = %q，想要 1.3.0", got)
	}

	// 手动检查发现同一版本（键里已是 1.3.0）：必须照常弹。
	c.app.Modal = nil
	c.updateGen = 3
	c.finishUpdateCheck(true, 3, "2026-03-07", &mygo.Update{Version: "1.3.0"}, nil)
	if c.app.Modal == nil || c.app.Modal.Title != "发现新版本" {
		t.Fatalf("手动检查同一版本被 updatePromptedVersion 挡住了：%+v", c.app.Modal)
	}
	if got := kv.String(prefUpdatePromptedVersion); got != "1.3.0" {
		t.Errorf("手动检查改写了自动提示键：%q", got)
	}
}

// TestAutoUpdateFailureIsSilent 钉住：自动检查失败 / 离线完全静默，不弹框、
// 不写 updatePromptedVersion，但要写 updateCheckedOn（今天真的查过了）。
func TestAutoUpdateFailureIsSilent(t *testing.T) {
	kv := newUpdateKV()
	clock := updateTestClock()
	c := newUpdateController(t, kv, clock, nil)
	c.updateGen = 1
	c.finishUpdateCheck(false, 1, "2026-03-07", nil, errors.New("network unreachable"))

	if c.app.Modal != nil {
		t.Fatalf("自动检查失败弹了框：%+v", c.app.Modal)
	}
	if got := kv.String(prefUpdateCheckedOn); got != "2026-03-07" {
		t.Errorf("失败时没写 updateCheckedOn：%q", got)
	}
	if got := kv.String(prefUpdatePromptedVersion); got != "" {
		t.Errorf("失败时写了 updatePromptedVersion：%q", got)
	}
	if c.updatePending.Load() != nil {
		t.Error("失败时挂起了 pending")
	}
}

// ---------------------------------------------------------------- 不打断

// TestAutoUpdateDefersWhileModalOrVideo 钉住：有模态框 / 视频窗开着时不弹，
// 挂起；关闭后弹一次、且只弹一次；一直不等到就丢弃（不写提示键）。
func TestAutoUpdateDefersWhileModalOrVideo(t *testing.T) {
	up := &mygo.Update{Version: "2.0.0"}

	// (a) 已有别的模态框。
	kv := newUpdateKV()
	clock := updateTestClock()
	c := newUpdateController(t, kv, clock, nil)
	other := &view.Modal{Kind: "message", Title: "别的对话框"}
	c.app.Modal = other
	c.updateGen = 1
	c.finishUpdateCheck(false, 1, "2026-03-07", up, nil)
	if c.app.Modal != other {
		t.Fatalf("有别的模态框时被覆盖了：%+v", c.app.Modal)
	}
	if c.updatePending.Load() == nil {
		t.Fatal("有别的模态框时没有挂起，直接丢弃了结果")
	}
	if got := kv.String(prefUpdatePromptedVersion); got != "" {
		t.Errorf("挂起期间就写了提示键：%q", got)
	}
	// 模态框关闭：弹一次。
	c.app.Modal = nil
	c.showPendingUpdateIfIdle()
	if c.app.Modal == nil || c.app.Modal.Title != "发现新版本" {
		t.Fatalf("模态框关闭后没有弹出挂起的更新：%+v", c.app.Modal)
	}
	if got := kv.String(prefUpdatePromptedVersion); got != "2.0.0" {
		t.Errorf("挂起结果弹出后没写提示键：%q", got)
	}
	if c.updatePending.Load() != nil {
		t.Error("弹出后没有清掉 pending")
	}
	// 再调一次：只弹一次。
	c.app.Modal = nil
	c.showPendingUpdateIfIdle()
	if c.app.Modal != nil {
		t.Errorf("挂起结果弹了不止一次：%+v", c.app.Modal)
	}

	// (b) 视频窗正开着（含全屏）：同样挂起，关掉视频窗后弹。
	kv2 := newUpdateKV()
	c2 := newUpdateController(t, kv2, updateTestClock(), nil)
	c2.app.VideoOpen = true
	c2.updateGen = 1
	c2.finishUpdateCheck(false, 1, "2026-03-07", up, nil)
	if c2.app.Modal != nil {
		t.Fatalf("视频窗开着时插入了更新框：%+v", c2.app.Modal)
	}
	if c2.updatePending.Load() == nil {
		t.Fatal("视频窗开着时没有挂起")
	}
	if got := kv2.String(prefUpdatePromptedVersion); got != "" {
		t.Errorf("视频窗挂起期间就写了提示键：%q", got)
	}
	c2.app.VideoOpen = false
	c2.showPendingUpdateIfIdle()
	if c2.app.Modal == nil || c2.app.Modal.Title != "发现新版本" {
		t.Fatalf("回到主窗后没有弹出挂起的更新：%+v", c2.app.Modal)
	}
	if got := kv2.String(prefUpdatePromptedVersion); got != "2.0.0" {
		t.Errorf("回到主窗弹出后没写提示键：%q", got)
	}
}

// TestAutoUpdateStoreAndDisabledSkipEverything 钉住：Store 版与不可自更新
// 的构建整条自动链路跳过 —— 不联网、不写两个门禁键。
func TestAutoUpdateStoreAndDisabledSkipEverything(t *testing.T) {
	// Store 版：自动检查完全不发起。
	kv := newUpdateKV()
	var calls atomic.Int32
	check := func(context.Context) (*mygo.Update, error) {
		calls.Add(1)
		return &mygo.Update{Version: "9.9.9"}, nil
	}
	c := newUpdateController(t, kv, updateTestClock(), check)
	c.storeInstall = func() bool { return true }
	c.runUpdateCheck(false)
	if calls.Load() != 0 {
		t.Errorf("Store 版自动检查联网了（%d 次）", calls.Load())
	}
	if got := kv.writeCount(); got != 0 {
		t.Errorf("Store 版写了 %d 个 KV 键（自动链路应一个都不写）", got)
	}

	// Store 版手动检查：给商店提示，也不联网、不写 KV。
	c.runUpdateCheck(true)
	if calls.Load() != 0 {
		t.Errorf("Store 版手动检查联网了（%d 次）", calls.Load())
	}
	if c.app.Modal == nil || !strings.Contains(c.app.Modal.Message, "Microsoft Store") {
		t.Errorf("Store 版手动检查没有给商店提示：%+v", c.app.Modal)
	}
	if got := kv.writeCount(); got != 0 {
		t.Errorf("Store 版手动检查写了 %d 个 KV 键", got)
	}

	// 不可自更新的构建：自动检查直接跳过（连 ErrUpdatesDisabled 都不去碰）。
	kv2 := newUpdateKV()
	var calls2 atomic.Int32
	check2 := func(context.Context) (*mygo.Update, error) {
		calls2.Add(1)
		return nil, mygo.ErrUpdatesDisabled
	}
	c2 := newUpdateController(t, kv2, updateTestClock(), check2)
	c2.updaterEnabled = func() bool { return false }
	c2.runUpdateCheck(false)
	if calls2.Load() != 0 {
		t.Errorf("不可自更新的构建仍发起了检查（%d 次）", calls2.Load())
	}
	if got := kv2.writeCount(); got != 0 {
		t.Errorf("不可自更新的构建写了 %d 个 KV 键", got)
	}
}

// TestAutoUpdateDoesNotBlockUI 钉住：检查在 goroutine 里跑，界面线程不等
// 网络 —— runUpdateCheck 立刻返回、检查进行中界面仍能处理一次点击，加载框
// 也已经画出来了。
func TestAutoUpdateDoesNotBlockUI(t *testing.T) {
	kv := newUpdateKV()
	started := make(chan struct{})
	release := make(chan struct{})
	check := func(context.Context) (*mygo.Update, error) {
		close(started)
		<-release
		return nil, nil
	}
	c := newUpdateController(t, kv, updateTestClock(), check)

	returned := make(chan struct{})
	go func() {
		c.runUpdateCheck(true)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("runUpdateCheck 在等网络，界面线程被阻塞了")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("检查 goroutine 没有启动")
	}
	if c.app.Modal == nil || c.app.Modal.Message != "正在检查更新…" {
		t.Fatalf("检查进行中没有加载框：%+v", c.app.Modal)
	}

	// 检查还卡在网络上，界面照样处理点击。
	clicked := false
	tt := ui.NewTester(func(ctx *ui.Context) {
		ui.Column(ctx).Children(func() {
			ui.Text(ctx, "界面仍在处理事件")
			b := ui.ButtonBase(ctx.Key("probe")).Label("界面按钮").Size(120, 30)
			if b.Clicked() {
				clicked = true
			}
		})
	}, 240, 120)
	tt.Frame()
	if err := tt.Click("界面按钮"); err != nil {
		t.Fatalf("检查进行中点不到界面按钮：%v", err)
	}
	tt.Frame()
	if !clicked {
		t.Fatal("检查进行中界面没有处理点击（被网络阻塞了）")
	}
	if c.app.Modal == nil || c.app.Modal.Message != "正在检查更新…" {
		t.Errorf("检查进行中加载框被界面事件冲掉了：%+v", c.app.Modal)
	}

	close(release)
	waitUpdate(t, "检查结束写入当天日期", func() bool {
		return kv.String(prefUpdateCheckedOn) == "2026-03-07"
	})
}

// TestStartupAutoUpdateTiming 钉住启动处仍是「10 秒后自动检查」并接上了
// 挂起观察循环；不是 0 秒（别在首帧抢用户的操作），也不是没有观察循环。
func TestStartupAutoUpdateTiming(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}
	flat := strings.Join(strings.Fields(string(src)), "")
	for _, want := range []string{
		`time.Sleep(10*time.Second)`,
		`mygo.Updater.Enabled()&&!isStoreInstall()`,
		`app.Win.Update(func(){c.runUpdateCheck(false)})`,
		`goc.watchPendingUpdate()`,
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("main.go 启动更新链路缺少 %q", want)
		}
	}
}

// TestAutoCheckYieldsToManualInFlight 钉住自动检查不抢手动检查的代号：
// 手动检查还在等网络时，到点的自动检查直接让路（否则手动加载框会因为结果
// 被代号丢弃而永远转下去）。
func TestAutoCheckYieldsToManualInFlight(t *testing.T) {
	kv := newUpdateKV()
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	check := func(context.Context) (*mygo.Update, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return nil, nil
	}
	c := newUpdateController(t, kv, updateTestClock(), check)
	c.runUpdateCheck(true)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("手动检查没有启动")
	}
	// 手动检查在途：自动检查不发起第二次，也不改面板。
	c.runUpdateCheck(false)
	if got := calls.Load(); got != 1 {
		t.Fatalf("自动检查抢跑了（累计 %d 次检查）", got)
	}
	if c.app.Modal == nil || c.app.Modal.Message != "正在检查更新…" {
		t.Fatalf("自动检查动了手动的加载框：%+v", c.app.Modal)
	}
	close(release)
	waitUpdate(t, "手动检查结束", func() bool {
		return kv.String(prefUpdateCheckedOn) == "2026-03-07"
	})
	if c.updateManual {
		t.Error("手动检查结束后 updateManual 没有复位，自动检查会被永久挡住")
	}
}
