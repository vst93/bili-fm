package main

// 自动检查更新的三条补齐（用户 2026-11 的要求）：
//
//  1. **每日一次**：原版是每天第一次启动后台查一次；此前每次启动都查。
//     新 KV 键 updateCheckedOn（YYYY-MM-DD，本地时区）记「上次真正跑过检查
//     的日期」。自动检查前先比，当天已查过就跳过；检查真的跑了（无论成功、
//     失败、已是最新）都写入当天日期。手动检查不受门禁限制，成功后同样刷新。
//  2. **同一版本只自动提示一次**：新 KV 键 updatePromptedVersion 记「上一次
//     自动弹窗提示过的版本」。自动检查发现同版本不再弹（可以静默记录）；发现
//     不同版本才弹，并在弹的那一刻写入 —— 用户点「稍后再说」/关闭也算提示过。
//     手动检查发现新版本必须照常弹，不受这个键限制。
//  3. **不打断用户**：自动检查命中新版本时，如果当前已有别的模态框打开，
//     或视频窗正开着（尤其全屏播放中），不立刻弹：把结果挂在内存里
//     （updatePending），等回到主窗且没有模态框时再弹一次；一直等不到就
//     随进程丢弃，不写 updatePromptedVersion，下次启动自然再试。
//
// 检查本身始终在 goroutine 里跑（见 prefs.go 的 runUpdateCheck），界面线程
// 只做结果落地。Store 版与不可自更新的构建由 runUpdateCheck 入口直接跳过，
// 连上面两个 KV 键都不写。

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/egoist/mygo"
)

// 自动检查更新的两个门禁键（只增不改，语义见文件头）。
const (
	prefUpdateCheckedOn       = "updateCheckedOn"
	prefUpdatePromptedVersion = "updatePromptedVersion"
)

// updateNow 返回当前时间：生产是 time.Now，测试注入固定时钟。
func (c *controller) updateNow() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// updateDay 是 updateCheckedOn 的取值：本地时区的 YYYY-MM-DD。
// 用本地时区而不是 UTC —— 用户说的「当天」是本地日历日。
func (c *controller) updateDay() string {
	return c.updateNow().Format("2006-01-02")
}

// updateCheck 返回检查更新的函数：生产是 mygo.Updater.Check，测试注入假结果。
func (c *controller) updateCheck() func(context.Context) (*mygo.Update, error) {
	if c.checkUpdateFn != nil {
		return c.checkUpdateFn
	}
	return mygo.Updater.Check
}

// updaterOn 报告当前构建能否自更新（开发构建 / 包管理器安装返回 false）。
func (c *controller) updaterOn() bool {
	if c.updaterEnabled != nil {
		return c.updaterEnabled()
	}
	return mygo.Updater.Enabled()
}

// storeBuild 报告当前进程是否 Microsoft Store 版。
func (c *controller) storeBuild() bool {
	if c.storeInstall != nil {
		return c.storeInstall()
	}
	return isStoreInstall()
}

// updatePref 读一个更新门禁键（KV 未注入时按空处理）。
func (c *controller) updatePref(key string) string {
	if c.kv == nil {
		return ""
	}
	return c.kv.String(key)
}

// setUpdatePref 写一个更新门禁键。写失败只记日志：门禁是节流而不是安全边界。
func (c *controller) setUpdatePref(key, value string) {
	if c.kv == nil {
		return
	}
	if err := c.kv.SetString(key, value); err != nil {
		log.Printf("写更新偏好 %s 失败: %v", key, err)
	}
}

// onUI 把 fn 送回界面线程。生产走 app.Win.Update；无窗口的单测同步执行。
func (c *controller) onUI(fn func()) {
	if c.app != nil && c.app.Win != nil {
		c.app.Win.Update(fn)
		return
	}
	fn()
}

// repaint 请求界面重画一帧（与 app.Win.Update(func(){}) 同义，集中一处便于
// 无窗口单测）。
func (c *controller) repaint() { c.onUI(func() {}) }

// finishUpdateCheck 是检查结果落到界面与 KV 的唯一出口，必须在界面线程调用。
// user 为真表示用户主动检查；gen 是发起检查时的代号，落后的结果直接丢弃。
func (c *controller) finishUpdateCheck(user bool, gen int, today string, up *mygo.Update, err error) {
	a := c.app
	if a == nil || c.updateGen != gen {
		return
	}
	c.updateCancel = nil
	c.updateManual = false
	// 每日门禁的写入时机：自动检查只要真的跑了（成功 / 失败 / 已最新）都算
	// 今天查过；手动检查只在成功后刷新（失败不占掉明天的自动检查）。
	if !user || err == nil {
		c.setUpdatePref(prefUpdateCheckedOn, today)
	}

	switch {
	case err != nil && errors.Is(err, mygo.ErrUpdatesDisabled):
		if user {
			a.Modal = updateDisabledModal()
		}
	case err != nil:
		// 自动检查失败 / 离线：完全静默，不弹任何框；updatePromptedVersion
		// 不写（updateCheckedOn 上面已经写了）。
		if user {
			a.Modal = updateFailedModal(updateFailText(err, true), "检查更新失败")
		}
	case up == nil:
		// 已是最新：自动静默，手动给反馈。
		if user {
			a.Modal = updateUpToDateModal()
		}
	default:
		if !user && up.Version == c.updatePref(prefUpdatePromptedVersion) {
			// 这个版本自动提示过了：不再弹（静默记录），也不覆盖 pending。
			log.Printf("自动检查更新：v%s 已提示过，静默跳过", up.Version)
			return
		}
		if !user && c.updateBusy() {
			// 不打断：挂起，等回到主窗且没有模态框（见 watchPendingUpdate）。
			c.updatePending.Store(up)
			return
		}
		c.showUpdateAvailable(up, user)
	}
}

// updateBusy 报告现在是否不适合插入自动更新框：已有模态框，或视频窗开着
// （尤其全屏播放中）。只在界面线程调用。
func (c *controller) updateBusy() bool {
	return c.app == nil || c.app.Modal != nil || c.app.VideoOpen
}

// showUpdateAvailable 弹出「发现新版本」对话框。自动提示在弹的这一刻就写
// updatePromptedVersion：用户点「稍后再说」/关闭也算已提示（当天不再弹）。
// 手动检查走同一条弹框函数，但不写这个键 —— 手动永远照常弹。
func (c *controller) showUpdateAvailable(up *mygo.Update, user bool) {
	a := c.app
	if a == nil || up == nil {
		return
	}
	version := up.Version
	a.Modal = updateAvailableModal(version,
		func() { c.installUpdate(up) },
		func() { c.closeModal() })
	if !user {
		c.setUpdatePref(prefUpdatePromptedVersion, version)
	}
	// 调用方都在界面线程（检查结果经 onUI 回到主线程 / 观察循环经 onUI
	// 投递），不必再嵌套一次 Win.Update。
}

// showPendingUpdateIfIdle 把挂起的自动更新提示在「回到主窗且没有模态框」时
// 弹出一次并清掉 pending；条件不满足就继续等。由 watchPendingUpdate 调用。
func (c *controller) showPendingUpdateIfIdle() {
	up := c.updatePending.Load()
	if up == nil || c.updateBusy() {
		return
	}
	c.updatePending.Store(nil)
	c.showUpdateAvailable(up, false)
}

// watchPendingUpdate 是挂起提示的观察循环：每秒看一眼有没有 pending（只在
// 有 pending 时才往界面线程投一次回调，平时零开销）。一直等不到「回到主窗
// 且没有模态框」就一直是内存里的 pending，退出即丢弃。
func (c *controller) watchPendingUpdate() {
	for range time.Tick(time.Second) {
		if c.updatePending.Load() == nil {
			continue
		}
		c.onUI(func() { c.showPendingUpdateIfIdle() })
	}
}
