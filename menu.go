package main

import (
	"bilifm/service"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Menu struct {
}

var AppContext context.Context

func NewMenu() *Menu {
	return &Menu{}
}

func (m *Menu) SetAppContext(ctx context.Context) {
	AppContext = ctx
}

func (m *Menu) ShowAbout() {
	runtime.MessageDialog(AppContext, runtime.MessageDialogOptions{
		Title:   "关于 bili-FM v" + service.APP_VERSION,
		Message: "通过音频来听B站节目，你可以把它作为一个音乐播放器，也可以用来作为知识学习的工具。\n\n版本：" + service.APP_VERSION + " (Build " + strconv.Itoa(service.APP_VERSION_NO) + ")\n\n项目开源：https://github.com/vst93/bili-fm",
		Type:    "info",
		Buttons: []string{"好的"},
	})
}

func (m *Menu) ShowVersion() {
	runtime.MessageDialog(AppContext, runtime.MessageDialogOptions{

		Title:   "版本",
		Message: service.APP_VERSION,
		Type:    "info",
		Buttons: []string{"好的"},
	})
}

func (m *Menu) ShowKeyboardShortcuts() {
	runtime.MessageDialog(AppContext, runtime.MessageDialogOptions{
		Title:   "快捷键",
		Message: "播放/暂停：空格键\n上一首：<-\n下一首：->\n最小化：ctrl/cmd + w\n退出：ctrl/cmd + q",
		Type:    "info",
		Buttons: []string{"好的"},
	})
}

func (m *Menu) CheckForUpdates(isManualCheck bool, gitFrom string) {
	githubReleaseUrl := "https://api.github.com/repos/vst93/bili-fm/releases/latest"
	giteeReleaseUrl := "https://gitee.com/api/v5/repos/vst93/bili-fm/releases/latest"
	giteeHtmlUrl := "https://gitee.com/vst93/bili-fm/releases/latest"
	releaseUrl := ""
	if gitFrom == "github" {
		releaseUrl = githubReleaseUrl
	} else {
		releaseUrl = giteeReleaseUrl
	}
	resp, err := http.Get(releaseUrl)
	if err != nil {
		// 网络连接失败，尝试切换到github
		if gitFrom != "github" {
			m.CheckForUpdates(isManualCheck, "github")
			return
		}
		if isManualCheck {
			runtime.MessageDialog(AppContext, runtime.MessageDialogOptions{
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
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil || release.TagName == "" {
		// 解析版本信息失败，尝试切换到github
		if gitFrom != "github" {
			m.CheckForUpdates(isManualCheck, "github")
			return
		}
		if isManualCheck {
			runtime.MessageDialog(AppContext, runtime.MessageDialogOptions{
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
	currentVersion := strings.TrimPrefix(service.APP_VERSION, "v")

	if latestVersion > currentVersion {
		if gitFrom != "github" {
			release.HtmlUrl = giteeHtmlUrl
		}
		choice, err := runtime.MessageDialog(AppContext, runtime.MessageDialogOptions{
			Title:         "发现新版本",
			Message:       "发现新版本 " + latestVersion + "，下载地址 " + release.HtmlUrl + "\n是否前往下载？",
			Type:          runtime.QuestionDialog,
			Buttons:       []string{"是", "否"},
			DefaultButton: "是",
			CancelButton:  "否",
		})

		if err == nil && (choice == "是" || choice == "Ok" || choice == "Yes") {
			runtime.BrowserOpenURL(AppContext, release.HtmlUrl)
		}
	} else if isManualCheck {
		runtime.MessageDialog(AppContext, runtime.MessageDialogOptions{
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

func (m *Menu) CloseApp() {
	runtime.Quit(context.Background())
}
