package main

import (
	"bilifm/service"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

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

type UpdateResult struct {
	HasUpdate      bool   `json:"hasUpdate"`
	LatestVersion  string `json:"latestVersion"`
	DownloadUrl    string `json:"downloadUrl"`
	IsLatest       bool   `json:"isLatest"`
	Error          string `json:"error"`
}

func (m *Menu) CheckForUpdates(isManualCheck bool, gitFrom string) UpdateResult {
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
		if gitFrom != "github" {
			return m.CheckForUpdates(isManualCheck, "github")
		}
		if isManualCheck {
			return UpdateResult{Error: "网络连接失败，请检查网络后重试"}
		}
		return UpdateResult{}
	}
	defer resp.Body.Close()

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil || release.TagName == "" {
		if gitFrom != "github" {
			return m.CheckForUpdates(isManualCheck, "github")
		}
		if isManualCheck {
			return UpdateResult{Error: "获取版本信息失败，请稍后重试"}
		}
		return UpdateResult{}
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(service.APP_VERSION, "v")

	if latestVersion > currentVersion {
		downloadUrl := release.HtmlUrl
		if gitFrom != "github" {
			downloadUrl = giteeHtmlUrl
		}
		return UpdateResult{
			HasUpdate:     true,
			LatestVersion: latestVersion,
			DownloadUrl:   downloadUrl,
		}
	}
	return UpdateResult{IsLatest: true}
}

func (m *Menu) GetPlatform() string {
	return runtime.Environment(context.Background()).Platform
}

func (m *Menu) CloseApp() {
	// 设置退出标志
	SetExiting()
	// 移除托盘图标 (Windows + Linux)
	removeTrayWindows()
	removeTrayLinux()
	// 强制退出进程
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
}
