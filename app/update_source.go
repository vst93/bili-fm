package main

// 更新源兜底：Gitee 优先，GitHub 兜底。
//
// 背景（用户 2026-10-21）：
//   - Tauri 旧版更新器是「先查 Gitee 的 latest.json，不通再查 GitHub」，
//     下载也走同一个源；
//   - mygo 原生版的 `mygo.Updater` 把更新源编译成 GitHub 固定地址
//     （mygo.json 的 updates.github + tagPrefix），没有 Gitee 分支 ——
//     国内用户检查更新经常直接超时。
//
// 为什么从「改 http.DefaultTransport」换成 mygo.UpdateSource（0.7.3 适配）：
//   - 之前是在 `http.DefaultTransport` 外面套一层传输层，把 GitHub 的
//     release API / 资产请求改写到 Gitee；
//   - mygo 0.7.2 起（#799）updater 不再走 `http.DefaultClient`，而是自带
//     `netproxy.Transport()`（跟随系统代理），**根本看不到我们的传输层**；
//     更糟的是 `netproxy.Transport()` 把 `http.DefaultTransport` 断言成
//     `*http.Transport`，我们换进去的自定义类型会让它直接 panic ——
//     应用一检查更新就崩。
//   - 所以改用框架 0.7.2 新给的 `Updater.SetSource`：选源逻辑变成框架的
//     正式扩展点，不再劫持全局 transport。
//
// 现在的实现（`giteeFirstSource`，实现 mygo.UpdateSource）：
//   - `Manifest`：先问 Gitee 的 release 列表拿最新 mygo-v* tag，再取
//     `update-<target>.json`；Gitee 不通 / 没有 release / 资产不是清单时
//     逐级回落 GitHub。
//   - `Open`：清单里点名的 GitHub 资产 URL 先换成 Gitee 同名路径下载
//     （归档 / delta），失败再回原始 GitHub URL。
//
// 关键点（和旧实现一致）：
//   - Gitee 的 release 资产是 sync_gitee workflow 从 GitHub **原样同步**的，
//     归档字节相同，所以清单里的 Ed25519 签名依旧有效；
//   - 清单里的 URL 仍指向 GitHub，选源只发生在下载时，不需要改动签名内容；
//   - 只改写本仓库（vst93/bili-fm）的 GitHub 资产，其它网络流量
//     （B 站 API、封面 CDN 等）完全不受影响 —— 现在连 http.DefaultTransport
//     都不动了。
//
// 这条链路的测试见 update_source_test.go：Gitee 成功时完全不碰 GitHub，
// Gitee 失败 / 资产缺失 / 清单损坏时逐请求回落。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
)

const (
	updateGitHubRepo = "vst93/bili-fm"
	updateGiteeRepo  = "vst93/bili-fm"
	updateTagPrefix  = "mygo-v"

	updateGitHubAPI = "https://api.github.com"
	updateGitHubWeb = "https://github.com"
	updateGiteeAPI  = "https://gitee.com/api/v5/repos"

	// updateSourceTimeout 是 Gitee 的「探路」超时（release 列表 / 清单）。
	// 国内半通不通时不能无限等：超时就回落 GitHub。归档下载不套这个超时
	// （见 Open），大文件要走多久由调用方的 ctx 决定。
	updateSourceTimeout = 8 * time.Second
	updateManifestMax   = 1 << 20
	// updateReleasesPerPage / updateMaxReleasePages 与 mygo 自己的
	// latestTagged 一致：GitHub 的 release 列表按 100 一页翻，最多 10 页。
	updateReleasesPerPage = 100
	updateMaxReleasePages = 10
	updateReleasesBodyMax = 4 * updateManifestMax
	updateUserAgent       = "bili-FM updater"
)

var installUpdateSourceOnce sync.Once

// installUpdateSource 让 mygo 的更新检查 / 下载走「Gitee 优先」的源。
// 必须在任何升级检查之前调用（main 启动早期）。
func installUpdateSource() {
	installUpdateSourceOnce.Do(func() {
		mygo.Updater.SetSource(giteeFirstSource{})
	})
}

// giteeFirstSource 是 mygo.UpdateSource 的 Gitee 优先实现。
//
// rt 只在测试里注入；生产环境用 netproxy.Transport()，与 mygo 自己的更新
// 请求一致 —— 跟随系统代理设置（PAC / WPAD 等），国内用户挂代理时也能用。
type giteeFirstSource struct {
	rt http.RoundTripper
}

var _ mygo.UpdateSource = giteeFirstSource{}

// Manifest 返回当前平台的更新清单（mygo 约定的 update-<target>.json）。
// prereleases 为真时（应用调用过 SetPrereleases）也接受预发布 tag。
func (s giteeFirstSource) Manifest(ctx context.Context, prereleases bool) (io.ReadCloser, error) {
	tag, err := s.latestTag(ctx, prereleases)
	if err != nil {
		return nil, err
	}
	asset := updateGitHubWeb + "/" + updateGitHubRepo + "/releases/download/" +
		url.PathEscape(tag) + "/" + url.PathEscape(updateManifestName())
	return s.openManifest(ctx, asset)
}

// Open 下载清单点名的文件（归档或 delta）。清单里的 URL 是 GitHub 的，
// 这里先换到 Gitee 同名路径试一次。
func (s giteeFirstSource) Open(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if gu, ok := giteeAssetURL(u); ok {
		if body, err := s.fetchGiteeAsset(ctx, gu); err == nil {
			return body, nil
		}
	}
	return s.fetch(ctx, rawURL)
}

// updateManifestName 是 mygo build 为当前平台写出的清单文件名。
// 发版 workflow 按平台分别构建（darwin/amd64、darwin/arm64、linux/amd64…），
// 所以 GOOS-GOARCH 就是 target，与框架内部的 update.ManifestName 一致。
func updateManifestName() string {
	return "update-" + runtime.GOOS + "-" + runtime.GOARCH + ".json"
}

// latestTag 先问 Gitee，拿不到再问 GitHub。两边都失败时把 Gitee 的原因
// 一起带上：排查「为什么国内用户更新不了」时要能看出是哪一段断的。
func (s giteeFirstSource) latestTag(ctx context.Context, prereleases bool) (string, error) {
	giteeErr := error(nil)
	if tag, err := s.giteeTag(ctx, prereleases); err == nil {
		return tag, nil
	} else {
		giteeErr = err
	}
	tag, err := s.githubTag(ctx, prereleases)
	if err != nil {
		return "", fmt.Errorf("%w（Gitee 兜底也失败：%v）", err, giteeErr)
	}
	return tag, nil
}

// giteeTag 从 Gitee 的 release 列表里取最新的 mygo-v* tag。
// Gitee 的列表按创建时间倒序，与 mygo 在 GitHub 上「取第一个匹配项」的
// 语义一致（预发布只在不限预发布时才接受）。
func (s giteeFirstSource) giteeTag(ctx context.Context, prereleases bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, updateSourceTimeout)
	defer cancel()

	api := updateGiteeAPI + "/" + updateGiteeRepo + "/releases?page=1&per_page=100&direction=desc"
	body, err := s.fetch(ctx, api)
	if err != nil {
		return "", err
	}
	defer body.Close()

	var releases []struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(io.LimitReader(body, updateReleasesBodyMax)).Decode(&releases); err != nil {
		return "", fmt.Errorf("gitee releases: %w", err)
	}
	for _, r := range releases {
		if tag, ok := pickUpdateTag(r.TagName, r.Prerelease, prereleases); ok {
			return tag, nil
		}
	}
	return "", fmt.Errorf("gitee: no %s release", updateTagPrefix)
}

// githubTag 是 Gitee 不可用时的回落：直接读 GitHub 的 release 列表。
func (s giteeFirstSource) githubTag(ctx context.Context, prereleases bool) (string, error) {
	for page := 1; page <= updateMaxReleasePages; page++ {
		api := fmt.Sprintf("%s/repos/%s/releases?per_page=%d&page=%d",
			updateGitHubAPI, updateGitHubRepo, updateReleasesPerPage, page)
		body, err := s.fetch(ctx, api)
		if err != nil {
			return "", err
		}
		var releases []struct {
			TagName    string `json:"tag_name"`
			Draft      bool   `json:"draft"`
			Prerelease bool   `json:"prerelease"`
		}
		err = json.NewDecoder(io.LimitReader(body, updateReleasesBodyMax)).Decode(&releases)
		body.Close()
		if err != nil {
			return "", fmt.Errorf("github releases: %w", err)
		}
		for _, r := range releases {
			if r.Draft {
				continue
			}
			if tag, ok := pickUpdateTag(r.TagName, r.Prerelease, prereleases); ok {
				return tag, nil
			}
		}
		if len(releases) < updateReleasesPerPage {
			break
		}
	}
	return "", fmt.Errorf("github: no %s release", updateTagPrefix)
}

// pickUpdateTag 判断一条 release 是否可用：tag 必须是 mygo-v*，
// 不限预发布时还要跳过 prerelease。
func pickUpdateTag(tag string, prerelease, allowPrerelease bool) (string, bool) {
	if !strings.HasPrefix(tag, updateTagPrefix) {
		return "", false
	}
	if prerelease && !allowPrerelease {
		return "", false
	}
	return tag, true
}

// openManifest 下载清单：Gitee 先试（带超时 + 内容校验），失败回落 GitHub。
func (s giteeFirstSource) openManifest(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	var giteeErr error
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if gu, ok := giteeAssetURL(u); ok {
		if body, err := s.fetchManifest(ctx, gu.String()); err == nil {
			return body, nil
		} else {
			giteeErr = err
		}
	}
	// GitHub 是最后一条路：不再套 8s 探路超时，也不改内容（框架自己解析，
	// 出错信息由框架给），与 0.7.1 之前的行为一致。
	body, err := s.fetch(ctx, rawURL)
	if err != nil && giteeErr != nil {
		return nil, fmt.Errorf("%w（Gitee 兜底也失败：%v）", err, giteeErr)
	}
	return body, err
}

// fetchManifest 下载清单并校验它确实是 mygo 的更新清单：Gitee 在资产缺失 /
// 需要登录时会回 200 + 登录页 HTML，直接交给 mygo 会变成「解析清单失败」
// 而不再回落 GitHub。
func (s giteeFirstSource) fetchManifest(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(ctx, updateSourceTimeout)
	defer cancel()

	body, err := s.fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	b, err := io.ReadAll(io.LimitReader(body, updateManifestMax+1))
	if err != nil {
		return nil, err
	}
	var m struct {
		Version   string `json:"version"`
		URL       string `json:"url"`
		Signature string `json:"signature"`
	}
	if len(b) > updateManifestMax || json.Unmarshal(b, &m) != nil ||
		m.Version == "" || m.URL == "" || m.Signature == "" {
		return nil, fmt.Errorf("%s is not an update manifest", rawURL)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

// fetchGiteeAsset 下载归档 / delta。不套探路超时（大文件由调用方 ctx 控制），
// 但要挡住 Gitee 的「200 + HTML 错误页」——否则 mygo 会把它当成归档去验签，
// 报「签名不匹配」而不再回落 GitHub。
func (s giteeFirstSource) fetchGiteeAsset(ctx context.Context, u *url.URL) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	// 不把 GitHub 专属的 Accept / Authorization 带过去。
	req.Header.Set("User-Agent", updateUserAgent)
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("gitee asset: %s", resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
		resp.Body.Close()
		return nil, fmt.Errorf("gitee asset is HTML")
	}
	return resp.Body, nil
}

// fetch GETs rawURL 并返回 2xx 的响应体。
func (s giteeFirstSource) fetch(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", updateUserAgent)
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	return resp.Body, nil
}

// client 返回选源用的 HTTP 客户端。transport 复用（netproxy 的 transport 是
// 全局共享的），所以连接也复用，不会每次请求重建连接池。
func (s giteeFirstSource) client() *http.Client {
	rt := s.rt
	if rt == nil {
		rt = netproxy.Transport()
	}
	return &http.Client{Transport: rt}
}

// giteeAssetURL 把 GitHub release 资产 URL 映射到 Gitee；仓库 / tag / 文件名
// 原样保留（sync_gitee 上传的就是同一批文件）。
func giteeAssetURL(u *url.URL) (*url.URL, bool) {
	prefix := "/" + updateGitHubRepo + "/releases/download/"
	if u == nil || u.Scheme != "https" || u.Host != "github.com" || !strings.HasPrefix(u.Path, prefix) {
		return nil, false
	}
	out := *u
	out.Scheme = "https"
	out.Host = "gitee.com"
	out.Path = "/" + updateGiteeRepo + "/releases/download/" + strings.TrimPrefix(u.Path, prefix)
	return &out, true
}
