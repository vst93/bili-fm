package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Menu struct {
}

func NewMenu() *Menu {
	return &Menu{}
}

func (m *Menu) ShowAbout(ctx context.Context) {
	runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Title:   "关于",
		Message: "通过音频来听B站节目，你可以把它作为一个音乐播放器，也可以用来作为知识学习的工具。\n\n项目开源地址：https://github.com/vst93/bili-fm",
		Type:    "info",
		Buttons: []string{"好的"},
	})

}

func (m *Menu) ShowVersion(ctx context.Context) {
	runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Title:   "版本",
		Message: APP_VERSION,
		Type:    "info",
		Buttons: []string{"好的"},
	})
}

func (m *Menu) CheckForUpdates(ctx context.Context, isManualCheck bool) {
	resp, err := http.Get("https://api.github.com/repos/vst93/bili-fm/releases/latest")
	if err != nil {
		if isManualCheck {
			runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
				Title:   "检查更新失败",
				Message: "网络连接失败，请稍后重试",
				Type:    runtime.ErrorDialog,
				Buttons: []string{"确定"},
			})
		}
		return
	}
	defer resp.Body.Close()

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		if isManualCheck {
			runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
				Title:   "检查更新失败",
				Message: "解析版本信息失败，请稍后重试",
				Type:    runtime.ErrorDialog,
				Buttons: []string{"确定"},
			})
		}
		return
	}

	// 移除版本号前的 'v' 如果存在
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(APP_VERSION, "v")

	if latestVersion > currentVersion {
		choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Title:         "发现新版本",
			Message:       "发现新版本 " + latestVersion + "\n是否前往下载？",
			Type:          runtime.QuestionDialog,
			Buttons:       []string{"是", "否"},
			DefaultButton: "是",
			CancelButton:  "否",
		})

		if err == nil && choice == "是" {
			runtime.BrowserOpenURL(ctx, release.HtmlUrl)
		}
	} else if isManualCheck {
		runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Title:   "检查更新",
			Message: "当前已是最新版本",
			Type:    runtime.InfoDialog,
			Buttons: []string{"确定"},
		})
	}
}

func (m *Menu) GetPlatform() string {
	return runtime.Environment(context.Background()).Platform
}
