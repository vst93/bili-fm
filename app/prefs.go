package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/vst93/bili-fm/app/internal/view"
)

// 显示偏好的存储键（与原版 localStorage 的同名键对齐，方便用户迁移）。
const (
	prefSpeed               = "playbackRate"
	prefVolume              = "volume"
	prefEQ                  = "loudnessEqEnabled"
	prefCover               = "coverMode"
	prefAmbient             = "ambientBackgroundEnabled"
	prefPremium             = "premiumTexture"
	prefRecTab              = "recommendTab"
	prefHistTab             = "historyTab"
	prefMiniPos             = "miniWindowPosition"
	prefLyricsColor         = "lyricsColor"
	prefLyricsSize          = "lyricsFontSize"
	prefLyricsMargin        = "lyricsMargin"
	prefLyricsBgOpacity     = "lyricsBgOpacity"
	prefLyricsTextOpacity   = "lyricsTextOpacity"
	prefLyricsEnabled       = "desktopSubtitles"
	prefLyricsOpacityLegacy = "lyricsOpacity"
	prefSubtitleLang        = "subtitleLanguage"
)

// loadPrefs 启动时读回显示偏好（原版把这些存在 localStorage，这里存本地 KV）。
func (c *controller) loadPrefs() {
	a := c.app
	if v := parseFloat(c.kv.String(prefSpeed)); v > 0 {
		a.Speed = v
	}
	if v := parseFloat(c.kv.String(prefVolume)); v >= 0 && c.kv.String(prefVolume) != "" {
		a.Volume = v
	}
	// 原版把「静音」也当成音量 0 持久化：重启后仍是静音（滑杆在 0、图标是
	// 静音图标）。所以读到 0 时把 Muted 也标上。
	a.Muted = a.Volume <= 0
	a.EQ = c.kv.String(prefEQ) == "true"
	if m := c.kv.String(prefCover); m == "square" {
		a.CoverMode = "square"
	}
	// 没有记录时用默认 true（原版默认开）。
	if c.kv.String(prefAmbient) == "false" {
		a.Ambient = false
	}
	if c.kv.String(prefPremium) == "false" {
		a.Premium = false
	}
	// 抽屉表头的 tab 记忆（原版 recommendList / historyList 各自持久化）。
	if v := c.kv.String(prefRecTab); v == view.RecRecommend {
		a.RecTab = v
	}
	if v := c.kv.String(prefHistTab); v == view.HistWatchLater {
		a.HistTab = v
	}
	// 桌面字幕：颜色必须是 #rrggbb；数值做范围夹取，防止旧配置/手改把
	// 字号或透明度弄成不可读的值。
	if v := c.kv.String(prefLyricsColor); validHexColor(v) {
		a.LyricsColor = v
	}
	if v := parseFloat(c.kv.String(prefLyricsSize)); v >= 14 && v <= 30 {
		a.LyricsFontSize = v
	}
	if v := parseFloat(c.kv.String(prefLyricsMargin)); v >= 4 && v <= 40 {
		a.LyricsMargin = v
	}
	// 旧版只有一个透明度：迁移为背景透明度，文字保持 1。
	if v := parseFloat(c.kv.String(prefLyricsOpacityLegacy)); v >= 0 && v <= 1 {
		a.LyricsBgOpacity = v
	}
	if v := parseFloat(c.kv.String(prefLyricsBgOpacity)); v >= 0 && v <= 1 {
		a.LyricsBgOpacity = v
	}
	if v := parseFloat(c.kv.String(prefLyricsTextOpacity)); v >= 0 && v <= 1 {
		a.LyricsTextOpacity = v
	}
	if c.kv.String(prefLyricsEnabled) == "true" {
		a.LyricsEnabled = true
	}
	c.subtitleLangPref = c.kv.String(prefSubtitleLang)
}

// validHexColor 接受 #rrggbb（桌面字幕配置只开放这一种格式）。
func validHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// setSpeed 设置倍速并落盘。
func (c *controller) setSpeed(v float64) {
	dbgSlow("setSpeed", func() { c.mp.SetSpeed(v) })
	c.app.Speed = v
	// 视频弹窗开着时同步倍速。
	if c.app.VideoOpen {
		dbgSlow("vid.SetSpeed", func() { c.vid.SetSpeed(v) })
	}
	if c.media != nil {
		dbgSlow("media.SetRate", func() { c.media.SetRate(v) })
	}
	dbgSlow("kv.SetString", func() {
		_ = c.kv.SetString(prefSpeed, strconv.FormatFloat(v, 'f', -1, 64))
	})
}

// dbgSlow 打印耗时超过 30ms 的步骤（排查界面卡顿用）。
func dbgSlow(name string, fn func()) {
	t0 := time.Now()
	fn()
	if d := time.Since(t0); d > 30*time.Millisecond {
		log.Printf("[slow] %s 用了 %v", name, d)
	}
}

// setVolume 设置音量并落盘（按原版语义：音量与静音是同一个事实）。
func (c *controller) setVolume(v float64) {
	a := c.app
	a.Volume = v
	// 原版 handleVolumeChange：`audio.volume = value; audio.muted = value === 0;`
	// 且 `if (value > 0) lastVolumeRef.current = value;` —— 音量 > 0 时记住它
	// （解除静音要用），音量 0 即静音。
	if v > 0 {
		a.LastVolume = v
	}
	a.Muted = v <= 0
	c.applyVolume()
	if c.media != nil {
		c.media.SetVolume(v)
	}
}

// persistVolume 只把音量写盘。单独拆出来是因为拖滑杆时每帧都会调
// setVolume，而落盘只在松手时做一次（原版同样的分工）。
func (c *controller) persistVolume(v float64) {
	_ = c.kv.SetString(prefVolume, strconv.FormatFloat(v, 'f', -1, 64))
}

// toggleMute 切换静音（原版音量弹层的静音键；主窗/mini 的音量键也走这里）。
//
// 原版的静音键不是「翻一个布尔」，而是把音量写成 0 / 写回上次音量：
//
//	if (volume === 0) { handleVolumeChange(lastVolumeRef.current || 1); persistVolume(restored); }
//	else             { handleVolumeChange(0); persistVolume(0); }
//
// 于是点静音时音量滑杆会一起归零、图标转静音，解除时回到之前的音量；静音态
// 也随音量一起落盘（重启后仍是静音）。此前原生版用独立的 Muted 布尔：静音时
// 音量/滑杆停在原值，且不落盘 —— 两处都与原版对不上。
func (c *controller) toggleMute() {
	a := c.app
	v, last := muteToggleState(a.Volume, a.LastVolume)
	a.LastVolume = last
	c.setVolume(v)
	c.persistVolume(v)
	a.Win.Update(func() {})
}

// muteToggleState 计算点一次静音键之后的（音量, 记住的上次音量）。纯函数，
// 便于单测逐值钉住原版的两条分支：
//
//	volume > 0  → (0, volume)          归零并记住当前音量
//	volume == 0 → (last || 1, 该值)     恢复到记住的音量，没记录就回到 1
func muteToggleState(volume, lastVolume float64) (float64, float64) {
	if volume > 0 {
		return 0, volume
	}
	if lastVolume <= 0 {
		return 1, 1
	}
	return lastVolume, lastVolume
}

// applyVolume 把当前音量/静音真正下发到引擎。
func (c *controller) applyVolume() {
	if c.mp == nil {
		return
	}
	v := c.app.Volume
	if c.app.Muted {
		v = 0
	}
	c.mp.SetVolume(v)
}

// setCoverMode 切换碟片/封面模式并落盘。
func (c *controller) setCoverMode(mode string) {
	c.app.CoverMode = mode
	_ = c.kv.SetString(prefCover, mode)
	c.app.Win.Update(func() {})
}

// toggleAmbient 切换封面背景（氛围光）并落盘。
func (c *controller) toggleAmbient() {
	c.app.Ambient = !c.app.Ambient
	_ = c.kv.SetString(prefAmbient, strconv.FormatBool(c.app.Ambient))
	c.app.Win.Update(func() {})
}

// togglePremium 切换高级质感并落盘。
func (c *controller) togglePremium() {
	c.app.Premium = !c.app.Premium
	_ = c.kv.SetString(prefPremium, strconv.FormatBool(c.app.Premium))
	c.app.Win.Update(func() {})
}

// showAbout 打开「关于应用」对话框（原版菜单里的关于）。
func (c *controller) showAbout() {
	v := mygo.App.Version()
	if v == "" {
		v = c.bl.GetAppVersion().Version
	}
	c.app.Modal = &view.Modal{
		Kind:      "message",
		Type:      "info",
		Title:     "关于 bili-FM",
		Message:   fmt.Sprintf("用音频聆听 B 站内容，既是音乐播放器，也是知识学习工具。\n\n版本 v%s", v),
		LinkLabel: "项目地址：github.com/vst93/bili-fm",
		LinkURL:   "https://github.com/vst93/bili-fm",
		Buttons:   []view.ModalButton{{Label: "好的", Primary: true}},
	}
	c.app.Win.Update(func() {})
}

// showShortcuts 打开「快捷键」对话框。
func (c *controller) showShortcuts() {
	c.app.Modal = &view.Modal{
		Kind:  "message",
		Type:  "info",
		Title: "快捷键",
		Message: "播放 / 暂停：空格键\n" +
			"上一首：←\n" +
			"下一首：→\n" +
			"音量：↑ / ↓\n" +
			"最小化：Ctrl/Cmd + W\n" +
			"退出：Ctrl/Cmd + Q",
		// 原版这一条主按钮文案是「知道了」。
		Buttons: []view.ModalButton{{Label: "知道了", Primary: true}},
	}
	c.app.Win.Update(func() {})
}

// checkUpdate 检查更新：与原版的「设置 → 检查更新」一致，弹的是应用内的液态
// 玻璃对话框（同「关于应用」「快捷键」），而不是 updater 插件的独立原生窗口。
//
// 流程照抄原版 titleBar.tsx 的 handleCheckUpdate / handleDownloadUpdate：
//
//	检查更新中 →（已是最新 | 检查失败 | 发现新版本）
//	发现新版本 →「立即更新」→ 下载中 → 更新完成（重启）
//
// 每一步都在同一套对话框里原地切换内容。
func (c *controller) checkUpdate() {
	c.runUpdateCheck(true)
}

// runUpdateCheck 是检查更新流程的共同入口。user 为真时（用户主动点）先弹
// 「正在检查更新…」的加载框，并在结果是最新版 / 失败时也给反馈；user 为假时
// （后台自动检查）静默跑，只在真的发现新版本时才弹框。
func (c *controller) runUpdateCheck(user bool) {
	a := c.app
	if a == nil || a.Win == nil {
		return
	}
	// Store 版只能由 Microsoft Store 更新；不要再碰 GitHub 更新包。
	if isStoreInstall() {
		if user {
			a.Modal = updateStoreModal()
			a.Win.Update(func() {})
		}
		return
	}
	c.updateGen++
	gen := c.updateGen
	c.cancelUpdateCtx()
	ctx, cancel := context.WithCancel(context.Background())
	c.updateCancel = cancel

	if user {
		a.Modal = updateCheckingModal(func() { c.cancelUpdate(gen) })
		a.Win.Update(func() {})
	}

	go func() {
		up, err := mygo.Updater.Check(ctx)
		a.Win.Update(func() {
			if c.updateGen != gen {
				return
			}
			c.updateCancel = nil
			switch {
			case err != nil && errors.Is(err, mygo.ErrUpdatesDisabled):
				if user {
					a.Modal = updateDisabledModal()
				}
			case err != nil:
				if user {
					a.Modal = updateFailedModal(updateFailText(err, true), "检查更新失败")
				}
			case up == nil:
				if user {
					a.Modal = updateUpToDateModal()
				}
			default:
				a.Modal = updateAvailableModal(up.Version,
					func() { c.installUpdate(up) },
					func() { c.closeModal() })
			}
		})
	}()
}

// installUpdate 下载并安装找到的新版本，全程在同一个对话框里显示进度。
func (c *controller) installUpdate(up *mygo.Update) {
	a := c.app
	if a == nil || a.Win == nil {
		return
	}
	c.updateGen++
	gen := c.updateGen
	c.cancelUpdateCtx()
	ctx, cancel := context.WithCancel(context.Background())
	c.updateCancel = cancel

	a.Modal = updateDownloadingModal(up.Version, 0, 0, func() { c.cancelUpdate(gen) })
	a.Win.Update(func() {})

	go func() {
		err := up.Install(ctx, func(downloaded, total int64) {
			a.Win.Update(func() {
				if c.updateGen != gen {
					return
				}
				a.Modal = updateDownloadingModal(up.Version, downloaded, total,
					func() { c.cancelUpdate(gen) })
			})
		})
		a.Win.Update(func() {
			if c.updateGen != gen {
				return
			}
			c.updateCancel = nil
			if err != nil {
				a.Modal = updateFailedModal(updateFailText(err, false), "更新失败")
				return
			}
			a.Modal = updateReadyModal(func() { mygo.App.Relaunch() })
		})
	}()
}

// cancelUpdate 取消当前更新流程并关掉面板（用户点「取消」/「稍后再说」或
// 关掉对话框）。换代让在途的检查 / 下载回调失效。
func (c *controller) cancelUpdate(gen int) {
	if c.updateGen != gen {
		return
	}
	c.updateGen++
	c.cancelUpdateCtx()
	c.closeModal()
}

func (c *controller) cancelUpdateCtx() {
	if c.updateCancel != nil {
		c.updateCancel()
		c.updateCancel = nil
	}
}

// closeModal 关闭当前对话框（登录框走 closeLogin）。同时让在途的更新检查 /
// 下载失效（Esc / 点背景关闭也要真的取消，而不只是隐藏面板）。
func (c *controller) closeModal() {
	c.updateGen++
	c.cancelUpdateCtx()
	c.app.Modal = nil
	c.app.Win.Update(func() {})
}

// ---------------------------------------------------------------- 更新对话框
//
// 这几个构造函数把更新流程的每个阶段包成应用内的 Modal（Kind == "message"，
// 复用 dialogTypeStyleFor 的图标 / 强调色 / Spinner）。拆成纯函数便于单测
// 逐字段断言（见 main_test.go 的 TestUpdateModals）。

func updateCheckingModal(onCancel func()) *view.Modal {
	return &view.Modal{
		Kind:    "message",
		Type:    "loading",
		Title:   "检查更新",
		Message: "正在检查更新…",
		Buttons: []view.ModalButton{{Label: "取消", OnClick: onCancel}},
	}
}

func updateUpToDateModal() *view.Modal {
	return &view.Modal{
		Kind:    "message",
		Type:    "success",
		Title:   "检查更新",
		Message: "当前已是最新版本",
		Buttons: []view.ModalButton{{Label: "好的", Primary: true}},
	}
}

// updateStoreModal 是 Microsoft Store 安装版本的「检查更新」结果：商店负责
// 更新，应用内不提供 GitHub 下载入口（Store 政策也不允许）。
func updateStoreModal() *view.Modal {
	return &view.Modal{
		Kind:    "message",
		Type:    "info",
		Title:   "检查更新",
		Message: "Microsoft Store 版本会随商店自动更新。\n如需立即检查，请打开 Microsoft Store → 库 → 获取更新。",
		Buttons: []view.ModalButton{{Label: "好的", Primary: true}},
	}
}

func updateDisabledModal() *view.Modal {
	return &view.Modal{
		Kind:      "message",
		Type:      "info",
		Title:     "检查更新",
		Message:   "当前构建不支持应用内更新（开发构建，或由包管理器安装）。\n可前往下载页面获取最新版本。",
		LinkLabel: "下载页面：github.com/vst93/bili-fm/releases",
		LinkURL:   "https://github.com/vst93/bili-fm/releases/latest",
		Buttons:   []view.ModalButton{{Label: "好的", Primary: true}},
	}
}

// updateFailText 把更新流程的底层错误翻成用户能看懂的中文。
//
// 背景（用户 2026-10-18 晚间 ③）：检查更新失败时，对话框直接把 mygo 的英文
// 报错贴出来（清单缺失 / 没有发行版本 / 网络错误的原文都是英文），用户看到
// 一串英文一头雾水。这里按种类分区给中文提示，原文只进日志：
//
//   - 网络/服务异常（超时、DNS、连接、TLS、EOF、5xx）→ 提示检查网络；
//   - 确实没有可用的发行版本（清单缺失 / 404 / 没有 release）→ 提示稍后再试；
//   - 其它 → 检查失败 / 更新失败，请稍后重试。
//
// checking 区分「检查」与「下载安装」两个阶段，只说用户当时在等的那件事。
func updateFailText(err error, checking bool) string {
	if err == nil {
		return ""
	}
	raw := err.Error()
	stage := "下载安装"
	if checking {
		stage = "检查"
	}
	log.Printf("更新：%s失败（原始错误：%s）", stage, raw)
	lower := strings.ToLower(raw)
	for _, s := range []string{"timeout", "deadline", "no such host", "connection", "tls", "eof", "network", "proxy", "unreachable", "502", "503", "504"} {
		if strings.Contains(lower, s) {
			return "网络连接失败，请检查网络后重试"
		}
	}
	for _, s := range []string{"manifest", "release", "not found", "no such", "404", "incomplete", "empty"} {
		if strings.Contains(lower, s) {
			return "暂时没有可用的发行版本，请稍后再试"
		}
	}
	if checking {
		return "检查更新失败，请稍后重试"
	}
	return "更新下载或安装失败，请稍后重试"
}

func updateFailedModal(msg, title string) *view.Modal {
	return &view.Modal{
		Kind:    "message",
		Type:    "error",
		Title:   title,
		Message: msg,
		Buttons: []view.ModalButton{{Label: "确定", Primary: true}},
	}
}

func updateAvailableModal(version string, onInstall, onLater func()) *view.Modal {
	return &view.Modal{
		Kind:    "message",
		Type:    "question",
		Title:   "发现新版本",
		Message: fmt.Sprintf("新版本 v%s 已发布\n是否立即下载并安装更新？", version),
		Buttons: []view.ModalButton{
			{Label: "立即更新", Primary: true, OnClick: onInstall},
			{Label: "稍后再说", OnClick: onLater},
		},
	}
}

// updateDownloadingModal 描述下载中的进度框。total 为 0（尚未收到总长）时
// 只报已下载字节数，与原版一致。
func updateDownloadingModal(version string, downloaded, total int64, onCancel func()) *view.Modal {
	var msg string
	if total > 0 {
		pct := downloaded * 100 / total
		msg = fmt.Sprintf("正在下载 v%s… %d%%\n%s / %s",
			version, pct, formatBytes(downloaded), formatBytes(total))
	} else {
		msg = fmt.Sprintf("正在下载 v%s… %s", version, formatBytes(downloaded))
	}
	return &view.Modal{
		Kind:    "message",
		Type:    "loading",
		Title:   "正在下载更新",
		Message: msg,
		Buttons: []view.ModalButton{{Label: "取消", OnClick: onCancel}},
	}
}

func updateReadyModal(onRelaunch func()) *view.Modal {
	return &view.Modal{
		Kind:    "message",
		Type:    "success",
		Title:   "更新完成",
		Message: "新版本已安装，重启应用即可使用。",
		Buttons: []view.ModalButton{{Label: "立即重启", Primary: true, OnClick: onRelaunch}},
	}
}

// formatBytes 把字节数格式化成人类可读的 KB / MB / GB（原版 titleBar.tsx 的
// formatBytes）。
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%.0f B", float64(n))
	}
	val := float64(n)
	for _, u := range []string{"KB", "MB", "GB"} {
		val /= unit
		if val < unit {
			return fmt.Sprintf("%.1f %s", val, u)
		}
	}
	return fmt.Sprintf("%.1f TB", val/unit)
}

// ---------------------------------------------------------------- 扫码登录

// login 打开登录面板：取二维码、生成位图、后台轮询扫码状态。
//
// 轮询在用户关闭面板或扫码成功后停止（loginGen 换代）。
func (c *controller) login() {
	a := c.app
	c.loginGen++
	gen := c.loginGen
	a.Modal = &view.Modal{Kind: "login", Title: "使用 B站 App 扫码登录"}
	a.Win.Update(func() {})

	go func() {
		url, err := c.bl.GetLoginQRCode()
		if err != nil {
			log.Printf("取二维码失败（原始错误：%v）", err)
			a.Win.Update(func() {
				// 原始错误只进日志：对话框里给中文原因（同 toast 的策略，见 main.go
				// 的 failReason）。
				a.Modal = &view.Modal{Kind: "message", Type: "error", Title: "登录失败", Message: "取二维码失败：" + failReason(err)}
			})
			return
		}
		bmp := qrBitmap(url)
		a.Win.Update(func() {
			if c.loginGen == gen && a.Modal != nil && a.Modal.Kind == "login" {
				a.Modal.QR = bmp
			}
		})

		// 轮询扫码状态：每 2 秒一次，最多 90 次（3 分钟）。
		for i := 0; i < 90; i++ {
			time.Sleep(2 * time.Second)
			if c.loginGen != gen {
				return // 面板被关掉 / 重新发起登录
			}
			if c.bl.GetLoginQRCodeStatus() {
				info := c.bl.GetBLUserInfo()
				a.Win.Update(func() {
					if c.loginGen != gen {
						return
					}
					a.LoggedIn = true
					if info != nil {
						a.UName = info.Uname
						a.Face = info.Face
						_ = c.kv.SetString("uname", info.Uname)
						_ = c.kv.SetString("face", info.Face)
						_ = c.kv.SetString("mid", strconv.Itoa(info.Mid))
					}
					c.bl.SetLoginStatus(true)
					a.Modal = nil
					a.Notify("登录成功")
				})
				// 登录后重拉当前抽屉（收藏/历史/动态这些需要登录的内容）。
				go func() {
					time.Sleep(300 * time.Millisecond)
					if c.loginGen != gen {
						return
					}
					if c.app.Drawer == "favorite" {
						c.loadFolders()
						return
					}
					key := c.app.CurrentSection().Key
					if c.app.Drawer != "" {
						key = c.app.Drawer
					}
					if key != "" && key != view.DrawerPlaylist {
						a.Win.Update(func() { c.loadSection(key, 1) })
					}
				}()
				return
			}
		}
		a.Win.Update(func() {
			if c.loginGen != gen {
				return
			}
			a.Modal = &view.Modal{Kind: "message", Type: "warning", Title: "登录超时", Message: "二维码已过期，请重新登录。"}
		})
	}()
}

// closeLogin 关闭登录面板并停止轮询。
func (c *controller) closeLogin() {
	c.loginGen++
	c.app.Modal = nil
	c.app.Win.Update(func() {})
}

// qrBitmap 把一段文本编成二维码位图。
func qrBitmap(text string) *ui.Bitmap {
	q, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		log.Printf("生成二维码失败: %v", err)
		return nil
	}
	png, err := q.PNG(320)
	if err != nil {
		log.Printf("编码二维码 PNG 失败: %v", err)
		return nil
	}
	bmp, err := ui.DecodeBitmap(png)
	if err != nil {
		log.Printf("解码二维码位图失败: %v", err)
		return nil
	}
	return bmp
}
