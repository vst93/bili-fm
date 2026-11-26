package bilibili

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestReplyUsesAPIClientAndReportsFailures(t *testing.T) {
	for _, tc := range []struct {
		name            string
		status          int
		body, wantError string
	}{
		{"success", 200, `{"code":0,"data":{"replies":[{"rpid":1,"content":{"message":"评论"}}],"page":{"num":1,"size":20,"count":40}}}`, ""},
		{"blocked", 412, `<html>internal error page</html>`, "412"},
		{"bad_json", 200, `<html>internal error page</html>`, "评论响应格式异常"},
		{"api_error", 200, `{"code":-352,"message":"请求被拦截"}`, "-352"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldAPI, oldImage := extraClient, fetchImageClient
			t.Cleanup(func() { extraClient, fetchImageClient = oldAPI, oldImage })
			// 评论不应走绕过默认代理、强制 IPv4 的图片专用传输通道。
			fetchImageClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("unexpected image transport")
			})}
			calls := 0
			extraClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				q := req.URL.Query()
				if req.URL.Path != "/x/v2/reply" || q.Get("oid") != "123" || q.Get("pn") != "1" || q.Get("sort") != "2" {
					t.Errorf("unexpected request: %s", req.URL)
				}
				resp := jsonResponse(tc.body)
				resp.StatusCode = tc.status
				return resp, nil
			})}
			page, err := NewBL().GetReplyList(123, 1)
			if calls != 1 {
				t.Fatalf("API client calls=%d err=%v", calls, err)
			}
			if tc.wantError == "" {
				if err != nil || len(page.Items) != 1 || !page.HasMore {
					t.Fatalf("page=%+v err=%v", page, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) || strings.Contains(err.Error(), "<html>") {
				t.Fatalf("error=%v, want %q without raw HTML", err, tc.wantError)
			}
		})
	}
}
