package main

// 更新源兜底：Gitee 优先，GitHub 兜底。
//
// 背景（用户 2026-10-21）：
//   - Tauri 旧版更新器是「先查 Gitee 的 latest.json，不通再查 GitHub」，
//     下载也走同一个源；
//   - mygo 原生版的 `mygo.Updater` 把更新源编译成 GitHub 固定地址
//     （updates.github + tagPrefix），没有 Gitee 分支 —— 国内用户检查更新
//     经常直接超时。
//
// 这里不重写下载 / 验签 / 替换安装那套已经验证过的逻辑，而是在
// `http.DefaultTransport` 外面套一层「Gitee 优先」的传输层：
//
//   - GitHub release API 请求 → 先查 Gitee API 的最新 mygo-v* release，
//     拿到 tag 后合成 GitHub 形状的 release 列表返回；mygo 随后按自己的
//     TaggedFeed 规则拼出 `github.com/.../releases/download/<tag>/...`。
//   - 该 GitHub 下载 URL（清单 / 归档 / delta）→ 先请求对应的
//     `gitee.com/.../releases/download/<tag>/...`；失败（连不上 / 404 /
//     非 2xx / 清单不是 JSON）再回落原始 GitHub URL。
//
// 关键点：
//   - Gitee 的 release 资产是 sync_gitee workflow 从 GitHub **原样同步**的，
//     归档字节相同，所以清单里的 Ed25519 签名依旧有效；
//   - 清单里的 URL 仍指向 GitHub，传输层负责在请求时换成 Gitee，不需要
//     改动签名内容；
//   - 只拦截 github.com/vst93/bili-fm 的 release 请求，其它网络流量
//     （B 站 API、封面 CDN 等）原样走默认 transport。
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
	"strings"
	"sync"
	"time"
)

const (
	updateGitHubRepo = "vst93/bili-fm"
	updateGiteeRepo  = "vst93/bili-fm"
	updateTagPrefix  = "mygo-v"

	updateGitHubAPI = "https://api.github.com"
	updateGitHubWeb = "https://github.com"
	updateGiteeAPI  = "https://gitee.com/api/v5/repos"

	updateSourceTimeout = 8 * time.Second
	updateManifestMax   = 1 << 20
)

var installUpdateSourceOnce sync.Once

// installUpdateSource 给 http.DefaultTransport 套上 Gitee 优先的兜底。
// 必须在任何升级检查 / 下载之前调用（main 启动早期）。
func installUpdateSource() {
	installUpdateSourceOnce.Do(func() {
		base := http.DefaultTransport
		// Clone 一份，避免改到进程里共享的默认 *http.Transport 设置。
		if t, ok := base.(*http.Transport); ok {
			base = t.Clone()
		}
		http.DefaultTransport = &giteeFirstTransport{base: base}
	})
}

// giteeFirstTransport 只做「选源」，具体协议仍交给底层 transport。
type giteeFirstTransport struct {
	base http.RoundTripper
}

func (t *giteeFirstTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case isGitHubUpdateAPI(req.URL):
		if resp, err := t.giteeReleases(req); err == nil {
			return resp, nil
		}
	case isGitHubUpdateAsset(req.URL):
		if resp, err := t.giteeAsset(req); err == nil {
			return resp, nil
		}
	}
	return t.base.RoundTrip(req)
}

// isGitHubUpdateAPI 判断是否是 mygo 用来解析 latest tag 的 GitHub release 列表请求。
func isGitHubUpdateAPI(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.Host != "api.github.com" {
		return false
	}
	return strings.HasPrefix(u.Path, "/repos/"+updateGitHubRepo+"/releases")
}

// isGitHubUpdateAsset 判断是否是本仓库的 release 资产下载（清单 / 归档 / delta）。
func isGitHubUpdateAsset(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.Host != "github.com" {
		return false
	}
	return strings.HasPrefix(u.Path, "/"+updateGitHubRepo+"/releases/download/")
}

// giteeRelease 是 Gitee release API 里我们关心的字段。
type giteeRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
}

// giteeReleases 先问 Gitee 最新 release，再合成 GitHub API 的响应。
// Gitee 不可达 / 没有 mygo-v* release / 响应损坏时返回错误，让调用方回落 GitHub。
func (t *giteeFirstTransport) giteeReleases(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), updateSourceTimeout)
	defer cancel()

	api := updateGiteeAPI + "/" + updateGiteeRepo + "/releases?page=1&per_page=100&direction=desc"
	greq, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}
	greq.Header.Set("User-Agent", "bili-FM updater")
	resp, err := t.base.RoundTrip(greq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitee releases: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*updateManifestMax))
	if err != nil {
		return nil, err
	}
	var releases []giteeRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}
	for _, r := range releases {
		if r.Prerelease || !strings.HasPrefix(r.TagName, updateTagPrefix) {
			continue
		}
		// mygo 的 latestTagged 只想看到 GitHub 的 release 列表字段；
		// 返回一条就够（它看到 len<100 会停止翻页）。
		out, err := json.Marshal([]map[string]any{{
			"tag_name":   r.TagName,
			"draft":      false,
			"prerelease": false,
		}})
		if err != nil {
			return nil, err
		}
		return jsonResponse(req, out), nil
	}
	return nil, fmt.Errorf("gitee: no %s release", updateTagPrefix)
}

// giteeAsset 把 GitHub release 资产 URL 换到 Gitee 先试一次。
// 清单（.json）必须仍是合法 JSON 才算成功，防止 Gitee 返回登录页 / 错误页
// 时让 mygo 直接报「解析清单失败」而不再回退 GitHub。
func (t *giteeFirstTransport) giteeAsset(req *http.Request) (*http.Response, error) {
	gu, ok := giteeAssetURL(req.URL)
	if !ok {
		return nil, fmt.Errorf("not a release asset")
	}
	ctx := req.Context()
	cancel := func() {}
	isManifest := strings.HasSuffix(gu.Path, ".json")
	if isManifest {
		// 清单很小；给 Gitee 一个明确的超时，避免国内半通不通时卡住。
		ctx, cancel = context.WithTimeout(ctx, updateSourceTimeout)
	}
	defer cancel()

	greq := req.Clone(ctx)
	greq.URL = gu
	greq.Host = ""
	// 不把 GitHub 专属的 Accept / Authorization 带过去。
	greq.Header = req.Header.Clone()
	greq.Header.Del("Authorization")
	greq.Header.Del("Accept")

	resp, err := t.base.RoundTrip(greq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("gitee asset: %s", resp.Status)
	}
	if !isManifest {
		// Gitee 在资产缺失 / 需要登录时可能返回 200 + HTML 错误页；直接
		// 交给 mygo 会变成「签名不匹配」，不会回落 GitHub。这里提前识别。
		if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
			resp.Body.Close()
			return nil, fmt.Errorf("gitee asset is HTML")
		}
		return resp, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, updateManifestMax+1))
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var m struct {
		Version   string `json:"version"`
		URL       string `json:"url"`
		Signature string `json:"signature"`
	}
	if len(body) > updateManifestMax || json.Unmarshal(body, &m) != nil ||
		m.Version == "" || m.URL == "" || m.Signature == "" {
		return nil, fmt.Errorf("gitee manifest is not an update manifest")
	}
	return dataResponse(req, body, resp.StatusCode), nil
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

func jsonResponse(req *http.Request, body []byte) *http.Response {
	return dataResponse(req, body, http.StatusOK)
}

func dataResponse(req *http.Request, body []byte, status int) *http.Response {
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        h,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}
