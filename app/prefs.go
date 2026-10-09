package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/vst93/bili-fm/app/internal/view"
)

// 显示偏好的存储键（与原版 localStorage 的同名键对齐，方便用户迁移）。
const (
	prefSpeed   = "playbackRate"
	prefVolume  = "volume"
	prefEQ      = "loudnessEqEnabled"
	prefCover   = "coverMode"
	prefAmbient = "ambientBackgroundEnabled"
	prefPremium = "premiumTexture"
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
	c.app.Speed = v
	c.mp.SetSpeed(v)
	_ = c.kv.SetString(prefSpeed, strconv.FormatFloat(v, 'f', -1, 64))
}

// setVolume 设置音量并落盘。
func (c *controller) setVolume(v float64) {
	c.app.Volume = v
	c.mp.SetVolume(v)
	_ = c.kv.SetString(prefVolume, strconv.FormatFloat(v, 'f', -1, 64))
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
		Kind:    "message",
		Title:   "关于 bili-FM",
		Message: fmt.Sprintf("用音频聆听 B 站内容，既是音乐播放器，也是知识学习工具。\n\n版本 v%s\n项目地址：github.com/vst93/bili-fm", v),
	}
	c.app.Win.Update(func() {})
}

// showShortcuts 打开「快捷键」对话框。
func (c *controller) showShortcuts() {
	c.app.Modal = &view.Modal{
		Kind:  "message",
		Title: "快捷键",
		Message: "播放 / 暂停：空格键\n" +
			"上一首：←\n" +
			"下一首：→\n" +
			"音量：↑ / ↓\n" +
			"最小化：Ctrl/Cmd + W\n" +
			"退出：Ctrl/Cmd + Q",
	}
	c.app.Win.Update(func() {})
}

// closeModal 关闭当前对话框（登录框走 closeLogin）。
func (c *controller) closeModal() {
	c.app.Modal = nil
	c.app.Win.Update(func() {})
}

// checkUpdate 检查更新：mygo 的 updater 插件自己会开一个更新窗口，显示
// 新版本说明、下载进度和重启按钮（相当于原版「检查更新」对话框的完整流程）。
func (c *controller) checkUpdate() {
	if !updater.AutomaticChecks() {
		// 用户手动检查时先报告「不可更新」的情况（包管理器装的、开发构建）。
	}
	updater.CheckForUpdates()
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
			a.Win.Update(func() {
				a.Modal = &view.Modal{Kind: "message", Title: "登录失败", Message: "取二维码失败：" + err.Error()}
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
					a.Modal = nil
					a.Notify("登录成功")
				})
				return
			}
		}
		a.Win.Update(func() {
			if c.loginGen != gen {
				return
			}
			a.Modal = &view.Modal{Kind: "message", Title: "登录超时", Message: "二维码已过期，请重新登录。"}
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
