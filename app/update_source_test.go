package main

import (
	"io"
	"net/http"
	"net/url"
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

func TestUpdateSourceGiteeFirstForReleaseAPI(t *testing.T) {
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		switch {
		case req.URL.Host == "gitee.com" && strings.Contains(req.URL.Path, "/releases"):
			return textResponse(http.StatusOK, `[{"tag_name":"mygo-v3.2.0","prerelease":false},{"tag_name":"mygo-v3.3.0","prerelease":true}]`)
		case req.URL.Host == "api.github.com":
			t.Errorf("Gitee 成功时不该再请求 GitHub: %s", req.URL)
		}
		return nil
	}
	tr := &giteeFirstTransport{base: fake}
	req, _ := http.NewRequest(http.MethodGet, updateGitHubAPI+"/repos/"+updateGitHubRepo+"/releases?per_page=100&page=1", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	if !strings.Contains(got, `"tag_name":"mygo-v3.2.0"`) {
		t.Fatalf("合成响应 = %s，应为 Gitee 最新非预发布 tag", got)
	}
	if strings.Contains(got, "3.3.0") {
		t.Fatalf("预发布 tag 不该被选中: %s", got)
	}
}

func TestUpdateSourceGiteeAPIFailureFallsBackToGitHub(t *testing.T) {
	fake := &fakeTransport{}
	fake.fn = func(req *http.Request) *http.Response {
		switch req.URL.Host {
		case "gitee.com":
			return textResponse(http.StatusBadGateway, "down")
		case "api.github.com":
			return textResponse(http.StatusOK, `[{"tag_name":"mygo-v9.9.9","draft":false,"prerelease":false}]`)
		}
		return nil
	}
	tr := &giteeFirstTransport{base: fake}
	req, _ := http.NewRequest(http.MethodGet, updateGitHubAPI+"/repos/"+updateGitHubRepo+"/releases?per_page=100&page=1", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "mygo-v9.9.9") {
		t.Fatalf("Gitee 失败后应回落 GitHub，got %s", body)
	}
}

func TestUpdateSourceAssetUsesGiteeFirst(t *testing.T) {
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
	tr := &giteeFirstTransport{base: fake}
	req, _ := http.NewRequest(http.MethodGet, asset, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "gitee-archive" {
		t.Fatalf("body = %q", body)
	}
}

func TestUpdateSourceAsset404FallsBackToGitHub(t *testing.T) {
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
	tr := &giteeFirstTransport{base: fake}
	req, _ := http.NewRequest(http.MethodGet, asset, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "github-archive" {
		t.Fatalf("Gitee 404 后应回落 GitHub，got %q", body)
	}
	if len(fake.calls) != 2 {
		t.Fatalf("应各请求一次，calls=%v", fake.calls)
	}
}

func TestUpdateSourceManifestValidation(t *testing.T) {
	manifestURL := updateGitHubWeb + "/" + updateGitHubRepo + "/releases/download/mygo-v3.2.0/update-linux-amd64.json"
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
				switch req.URL.Host {
				case "gitee.com":
					return textResponse(http.StatusOK, c.giteeBody)
				case "github.com":
					return textResponse(http.StatusOK, updateManifestJSON("3.2.0"))
				}
				return nil
			}
			tr := &giteeFirstTransport{base: fake}
			req, _ := http.NewRequest(http.MethodGet, manifestURL, nil)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), `"version"`) {
				t.Fatalf("body 不是清单: %s", body)
			}
			usedGitHub := false
			for _, u := range fake.calls {
				if strings.HasPrefix(u, "https://github.com/") {
					usedGitHub = true
				}
			}
			if usedGitHub != c.wantGitHub {
				t.Fatalf("usedGitHub=%v want %v calls=%v", usedGitHub, c.wantGitHub, fake.calls)
			}
		})
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
