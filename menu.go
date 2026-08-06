package main

import (
	"bilifm/service"
	"context"
	"encoding/json"
	"fmt"
	goruntime "runtime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	if compareVersions(latestVersion, currentVersion) > 0 {
		downloadUrl := release.HtmlUrl
		if gitFrom != "github" {
			downloadUrl = giteeHtmlUrl
		}
		// 平台匹配的安装包直链（用于应用内下载更新）
		if assetUrl := m.pickUpdateAsset(&release); assetUrl != "" {
			downloadUrl = assetUrl
		}
		return UpdateResult{
			HasUpdate:     true,
			LatestVersion: latestVersion,
			DownloadUrl:   downloadUrl,
		}
	}
	return UpdateResult{IsLatest: true}
}

// compareVersions 语义化比较版本号，返回 a>b=1, a<b=-1, a==b=0。
// 修复原字符串比较在 1.9.5 vs 1.10.0 时误判的问题。
func compareVersions(a, b string) int {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	// 去掉预发布后缀（-preview/-beta 等），仅比较数字段
	a = strings.SplitN(a, "-", 2)[0]
	b = strings.SplitN(b, "-", 2)[0]

	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	max := len(ap)
	if len(bp) > max {
		max = len(bp)
	}
	for i := 0; i < max; i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av, _ = strconv.Atoi(ap[i])
		}
		if i < len(bp) {
			bv, _ = strconv.Atoi(bp[i])
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

// pickUpdateAsset 按当前平台/架构从 release assets 中挑选安装包直链。
func (m *Menu) pickUpdateAsset(release *GithubRelease) string {
	env := runtime.Environment(context.Background())
	platform := env.Platform
	arch := env.Arch

	var fallback string
	for _, a := range release.Assets {
		url := a.BrowserDownloadUrl
		name := url[strings.LastIndex(url, "/")+1:]
		low := strings.ToLower(name)

		switch platform {
		case "windows":
			if !strings.HasSuffix(low, ".exe") || !strings.Contains(low, "setup") {
				continue
			}
			// amd64 资产不含 arm64；arm64 资产含 arm64
			if strings.Contains(low, "arm64") == (arch == "arm64") {
				return url
			}
			if fallback == "" {
				fallback = url
			}
		case "darwin":
			if !strings.HasSuffix(low, ".dmg") {
				continue
			}
			if (arch == "arm64" && strings.Contains(low, "silicon")) ||
				(arch == "amd64" && strings.Contains(low, "intel")) {
				return url
			}
			if fallback == "" {
				fallback = url
			}
		case "linux":
			if !strings.HasSuffix(low, ".appimage") {
				continue
			}
			if (arch == "arm64" && strings.Contains(low, "arm64")) ||
				(arch == "amd64" && strings.Contains(low, "x86_64")) {
				return url
			}
			if fallback == "" {
				fallback = url
			}
		}
	}
	return fallback
}

// IsMSStoreInstall 检测是否 Microsoft Store 安装版本。
// MS Store 应用的 exe 路径包含 `WindowsApps`，商店版无法应用内自更新，应跳转商店。
func (m *Menu) IsMSStoreInstall() bool {
	if goruntime.GOOS != "windows" {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(exe), "windowsapps")
}

// UpdateProgress 下载进度（通过事件 update:progress 推送给前端）
type UpdateProgress struct {
	Downloaded int64 `json:"downloaded"`
	Total      int64 `json:"total"`
}

// DownloadUpdate 下载更新安装包到临时目录，并在下载过程中推送进度事件。
// url 为 CheckForUpdates 返回的平台安装包直链。
func (m *Menu) DownloadUpdate(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("下载地址为空")
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "*/*")

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// 从 URL 提取文件名作为临时文件名
	name := "bili-fm-update"
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		if n := url[idx+1:]; n != "" {
			name = n
		}
	}
	tmpPath := filepath.Join(os.TempDir(), name)
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return "", werr
			}
			downloaded += int64(n)
			if AppContext != nil {
				runtime.EventsEmit(AppContext, "update:progress", UpdateProgress{Downloaded: downloaded, Total: total})
			}
		}
		if readErr != nil {
			break
		}
	}
	out.Close()
	return tmpPath, nil
}

// ApplyResult 安装结果
type ApplyResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ApplyUpdate 安装下载好的更新包并（尽量）重启。
// Windows: 静默运行 NSIS 安装器后退出；macOS/Linux: 打开安装包由用户完成安装。
func (m *Menu) ApplyUpdate(filePath string) ApplyResult {
	if filePath == "" {
		return ApplyResult{Success: false, Message: "更新文件不存在"}
	}
	if _, err := os.Stat(filePath); err != nil {
		return ApplyResult{Success: false, Message: "更新文件不存在"}
	}

	switch goruntime.GOOS {
	case "windows":
		// NSIS 静默安装，随后退出当前进程以便安装器覆盖文件
		if err := exec.Command(filePath, "/S").Start(); err != nil {
			return ApplyResult{Success: false, Message: "启动安装程序失败: " + err.Error()}
		}
		go func() {
			time.Sleep(800 * time.Millisecond)
			m.CloseApp()
		}()
		return ApplyResult{Success: true, Message: "更新程序已启动，安装完成后请重新打开应用"}
	case "darwin":
		if err := exec.Command("open", filePath).Start(); err != nil {
			return ApplyResult{Success: false, Message: "打开安装包失败: " + err.Error()}
		}
		return ApplyResult{Success: true, Message: "已打开安装包，请将 bili-FM 拖入 Applications 后重新打开应用"}
	case "linux":
		if err := exec.Command("xdg-open", filePath).Start(); err != nil {
			return ApplyResult{Success: false, Message: "打开安装包失败: " + err.Error()}
		}
		return ApplyResult{Success: true, Message: "已打开安装包，请完成安装后重新打开应用"}
	}
	return ApplyResult{Success: false, Message: "当前平台暂不支持自动更新"}
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
