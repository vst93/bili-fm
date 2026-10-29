package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"
)

// fakeTransport answers requests from a function and records what was asked.
type fakeTransport struct {
	calls []string
	fn    func(*http.Request) *http.Response
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.calls = append(f.calls, req.URL.String())
	resp := f.fn(req)
	if resp == nil {
		resp = textResponse(http.StatusNotFound, "not found")
	}
	resp.Request = req
	return resp, nil
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func updateManifestJSON(version string) string {
	return `{"version":"` + version + `","url":"https://github.com/vst93/bili-fm/releases/download/mygo-v` + version + `/bili-fm.tar.gz","size":123,"signature":"c2ln"}` + "\n"
}

func giteeReleaseList(tags ...string) string {
	var parts []string
	for _, t := range tags {
		parts = append(parts, `{"tag_name":"`+t+`","prerelease":false}`)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// readManifest / readOpen 是测试里读清单 / 资产的简写。
func readManifest(t *testing.T, src giteeFirstSource, pre bool) string {
	t.Helper()
	body, err := src.Manifest(context.Background(), pre)
	return readBody(t, body, err)
}

func readOpen(t *testing.T, src giteeFirstSource, rawURL string) string {
	t.Helper()
	body, err := src.Open(context.Background(), rawURL)
	return readBody(t, body, err)
}

// readBody 读完并关闭。
func readBody(t *testing.T, body io.ReadCloser, err error) string {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func callsTo(fake *fakeTransport, host string) []string {
	var out []string
	for _, u := range fake.calls {
		parsed, err := url.Parse(u)
		if err == nil && parsed.Host == host {
			out = append(out, u)
		}
	}
	return out
}

// TestUpdateSourceManifestGiteeFirst 钉住「Gitee 成功时完全不碰 GitHub」：
// release 列表和清单都从 Gitee 拿，清单名按当前平台拼。
func TestUpdateSourceManifestGiteeFirst(t *testing.T) {
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		switch req.URL.Host {
		case "gitee.com":
			if strings.HasPrefix(req.URL.Path, "/api/v5/repos/") {
				return textResponse(http.StatusOK, giteeReleaseList("mygo-v3.2.0", "mygo-v3.1.2"))
			}
			want := "/" + updateGiteeRepo + "/releases/download/mygo-v3.2.0/" + updateManifestName()
			if req.URL.Path != want {
				t.Errorf("Gitee 清单路径 = %q，应为 %q", req.URL.Path, want)
			}
			return textResponse(http.StatusOK, updateManifestJSON("3.2.0"))
		}
		t.Errorf("Gitee 成功时不该请求 %s", req.URL)
		return nil
	}
	src := giteeFirstSource{rt: fake}
	got := readManifest(t, src, false)
	if !strings.Contains(got, `"version":"3.2.0"`) {
		t.Fatalf("清单 = %s", got)
	}
	if c := callsTo(fake, "api.github.com"); len(c) != 0 {
		t.Fatalf("不该请求 GitHub API: %v", c)
	}
	if c := callsTo(fake, "github.com"); len(c) != 0 {
		t.Fatalf("不该请求 GitHub 资产: %v", c)
	}
}

// TestUpdateSourceManifestSkipsPrerelease 钉住预发布过滤：默认只认正式版。
func TestUpdateSourceManifestSkipsPrerelease(t *testing.T) {
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		switch {
		case req.URL.Host == "gitee.com" && strings.HasPrefix(req.URL.Path, "/api/v5/repos/"):
			return textResponse(http.StatusOK,
				`[{"tag_name":"mygo-v3.3.0","prerelease":true},{"tag_name":"mygo-v3.2.0","prerelease":false}]`)
		case req.URL.Host == "gitee.com":
			if !strings.Contains(req.URL.Path, "/mygo-v3.2.0/") {
				t.Errorf("应选正式版 tag，path = %q", req.URL.Path)
			}
			return textResponse(http.StatusOK, updateManifestJSON("3.2.0"))
		}
		return nil
	}
	src := giteeFirstSource{rt: fake}
	got := readManifest(t, src, false)
	if !strings.Contains(got, `"version":"3.2.0"`) {
		t.Fatalf("清单 = %s", got)
	}
}

// TestUpdateSourceManifestAllowsPrerelease 钉住 SetPrereleases 打开后的行为：
// 预发布 tag 也接受。
func TestUpdateSourceManifestAllowsPrerelease(t *testing.T) {
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		switch {
		case req.URL.Host == "gitee.com" && strings.HasPrefix(req.URL.Path, "/api/v5/repos/"):
			return textResponse(http.StatusOK,
				`[{"tag_name":"mygo-v3.3.0-preview.1","prerelease":true},{"tag_name":"mygo-v3.2.0","prerelease":false}]`)
		case req.URL.Host == "gitee.com":
			if !strings.Contains(req.URL.Path, "/mygo-v3.3.0-preview.1/") {
				t.Errorf("应选最新预发布 tag，path = %q", req.URL.Path)
			}
			return textResponse(http.StatusOK, updateManifestJSON("3.3.0-preview.1"))
		}
		return nil
	}
	src := giteeFirstSource{rt: fake}
	got := readManifest(t, src, true)
	if !strings.Contains(got, `"version":"3.3.0-preview.1"`) {
		t.Fatalf("清单 = %s", got)
	}
}

// TestUpdateSourceReleaseAPIFailureFallsBackToGitHub 钉住 Gitee 不可达时
// 整条链路回落到 GitHub（release 列表 + 清单）。
func TestUpdateSourceReleaseAPIFailureFallsBackToGitHub(t *testing.T) {
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		switch req.URL.Host {
		case "gitee.com":
			return textResponse(http.StatusBadGateway, "down")
		case "api.github.com":
			return textResponse(http.StatusOK, `[{"tag_name":"mygo-v9.9.9","draft":false,"prerelease":false}]`)
		case "github.com":
			return textResponse(http.StatusOK, updateManifestJSON("9.9.9"))
		}
		return nil
	}
	src := giteeFirstSource{rt: fake}
	got := readManifest(t, src, false)
	if !strings.Contains(got, `"version":"9.9.9"`) {
		t.Fatalf("Gitee 失败后应回落 GitHub，清单 = %s", got)
	}
	if c := callsTo(fake, "github.com"); len(c) != 1 {
		t.Fatalf("应回源 GitHub 一次，calls=%v", fake.calls)
	}
}

// TestUpdateSourceBothSourcesFail 钉住两边都失败时错误里带两段原因：
// 排查「为什么国内用户更新不了」要能看出是哪一段断的。
func TestUpdateSourceBothSourcesFail(t *testing.T) {
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		return textResponse(http.StatusBadGateway, "down")
	}
	src := giteeFirstSource{rt: fake}
	body, err := src.Manifest(context.Background(), false)
	if err == nil {
		body.Close()
		t.Fatal("两边都 502 时应报错")
	}
	if !strings.Contains(err.Error(), "Gitee") {
		t.Fatalf("错误应带上 Gitee 的原因: %v", err)
	}
	if !strings.Contains(err.Error(), "Bad Gateway") {
		t.Fatalf("错误应带上 GitHub 的原始状态: %v", err)
	}
}

// TestUpdateSourceManifestValidation 钉住「Gitee 回 HTML / 错误 JSON 时回落
// GitHub」：否则 mygo 会报解析清单失败，用户永远更新不了。
func TestUpdateSourceManifestValidation(t *testing.T) {
	cases := []struct {
		name       string
		giteeBody  string
		wantGitHub bool
	}{
		{"valid", updateManifestJSON("3.2.0"), false},
		{"html", "<html>login</html>", true},
		{"json error", `{"error":"not found"}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakeTransport{}
			fake.fn = func(req *http.Request) *http.Response {
				switch {
				case req.URL.Host == "gitee.com" && strings.HasPrefix(req.URL.Path, "/api/v5/repos/"):
					return textResponse(http.StatusOK, giteeReleaseList("mygo-v3.2.0"))
				case req.URL.Host == "gitee.com":
					return textResponse(http.StatusOK, c.giteeBody)
				case req.URL.Host == "github.com":
					return textResponse(http.StatusOK, updateManifestJSON("3.2.0"))
				}
				return nil
			}
			src := giteeFirstSource{rt: fake}
			got := readManifest(t, src, false)
			if !strings.Contains(got, `"version"`) {
				t.Fatalf("body 不是清单: %s", got)
			}
			usedGitHub := len(callsTo(fake, "github.com")) > 0
			if usedGitHub != c.wantGitHub {
				t.Fatalf("usedGitHub=%v want %v calls=%v", usedGitHub, c.wantGitHub, fake.calls)
			}
		})
	}
}

// TestUpdateSourceOpenUsesGiteeFirst 钉住归档 / delta 的下载选源。
func TestUpdateSourceOpenUsesGiteeFirst(t *testing.T) {
	asset := updateGitHubWeb + "/" + updateGitHubRepo + "/releases/download/mygo-v3.2.0/bili-fm.tar.gz"
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		switch req.URL.Host {
		case "gitee.com":
			if got := req.URL.Path; got != "/"+updateGiteeRepo+"/releases/download/mygo-v3.2.0/bili-fm.tar.gz" {
				t.Errorf("Gitee 资产路径 = %q", got)
			}
			return textResponse(http.StatusOK, "gitee-archive")
		case "github.com":
			t.Errorf("Gitee 资产成功时不该回源 GitHub: %s", req.URL)
		}
		return nil
	}
	src := giteeFirstSource{rt: fake}
	if got := readOpen(t, src, asset); got != "gitee-archive" {
		t.Fatalf("body = %q", got)
	}
}

// TestUpdateSourceOpen404FallsBackToGitHub 钉住 Gitee 缺资产时回源 GitHub。
func TestUpdateSourceOpen404FallsBackToGitHub(t *testing.T) {
	asset := updateGitHubWeb + "/" + updateGitHubRepo + "/releases/download/mygo-v3.2.0/bili-fm.tar.gz"
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		switch req.URL.Host {
		case "gitee.com":
			return textResponse(http.StatusNotFound, "missing")
		case "github.com":
			return textResponse(http.StatusOK, "github-archive")
		}
		return nil
	}
	src := giteeFirstSource{rt: fake}
	if got := readOpen(t, src, asset); got != "github-archive" {
		t.Fatalf("Gitee 404 后应回落 GitHub，got %q", got)
	}
	if len(fake.calls) != 2 {
		t.Fatalf("应各请求一次，calls=%v", fake.calls)
	}
}

// TestUpdateSourceOpenHTMLFallsBackToGitHub 钉住 Gitee 的「200 + 登录页」
// 不会当成归档交给 mygo（那会变成「签名不匹配」）。
func TestUpdateSourceOpenHTMLFallsBackToGitHub(t *testing.T) {
	asset := updateGitHubWeb + "/" + updateGitHubRepo + "/releases/download/mygo-v3.2.0/bili-fm.tar.gz"
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		switch req.URL.Host {
		case "gitee.com":
			resp := textResponse(http.StatusOK, "<html>login</html>")
			resp.Header.Set("Content-Type", "text/html; charset=utf-8")
			return resp
		case "github.com":
			return textResponse(http.StatusOK, "github-archive")
		}
		return nil
	}
	src := giteeFirstSource{rt: fake}
	if got := readOpen(t, src, asset); got != "github-archive" {
		t.Fatalf("Gitee HTML 后应回落 GitHub，got %q", got)
	}
}

// TestUpdateSourceOpenLeavesOtherURLsAlone 钉住非本仓库的 URL 原样下载
// （清单里的 url 理论上可以指向任何地方，选源不该乱改）。
func TestUpdateSourceOpenLeavesOtherURLsAlone(t *testing.T) {
	other := "https://downloads.example.com/bili-fm/bili-fm.tar.gz"
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		if req.URL.String() != other {
			t.Errorf("请求被改写: %s", req.URL)
		}
		return textResponse(http.StatusOK, "archive")
	}
	src := giteeFirstSource{rt: fake}
	if got := readOpen(t, src, other); got != "archive" {
		t.Fatalf("body = %q", got)
	}
}

// TestUpdateManifestName 钉住清单文件名与框架的 update.ManifestName 同口径。
func TestUpdateManifestName(t *testing.T) {
	want := "update-" + runtime.GOOS + "-" + runtime.GOARCH + ".json"
	if got := updateManifestName(); got != want {
		t.Fatalf("updateManifestName() = %q，应为 %q", got, want)
	}
}

func TestGiteeAssetURLKeepsQueryAndOnlyMapsRepo(t *testing.T) {
	u, _ := url.Parse(updateGitHubWeb + "/" + updateGitHubRepo + "/releases/download/mygo-v3.2.0/a.tar.gz?x=1")
	got, ok := giteeAssetURL(u)
	if !ok || got.Host != "gitee.com" || got.Path != "/"+updateGiteeRepo+"/releases/download/mygo-v3.2.0/a.tar.gz" || got.RawQuery != "x=1" {
		t.Fatalf("mapped = %v ok=%v", got, ok)
	}
	other, _ := url.Parse("https://github.com/other/repo/releases/download/v1/a.tar.gz")
	if _, ok := giteeAssetURL(other); ok {
		t.Fatal("非本仓库的 GitHub 资产不该被改写")
	}
}
