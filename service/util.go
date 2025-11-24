package service

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"runtime"
	"strconv"
	"strings"
)

// NumberToString 任意类型数字转换为字符串(太长的数字会有精度问题)
func NumberToString(n interface{}) (string, bool) {
	r := ""
	ok := true
	switch n := n.(type) {
	case string:
		r = n
	case int:
		r = strconv.Itoa(n)
	case int64:
		r = strconv.FormatInt(n, 10)
	case float64:
		r = strconv.FormatFloat(n, 'f', -1, 64)
	default:
		ok = false
	}
	return r, ok
}

func GetUserAgent() string {
	// 根据实际平台类型生成模拟的浏览器 User-Agent
	switch runtime.GOOS {
	case "darwin":
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36"
	case "windows":
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36"
	default: // Linux 和其他类Unix系统
		return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36"
	}
}

func PostJson(url string, msg []byte, headers map[string]string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, strings.NewReader(string(msg)))
	if err != nil {
		return "", err
	}
	for key, header := range headers {
		req.Header.Set(key, header)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// SendAppStats 发送统计信息到 umami.dev
func SendAppStats() {
	website := "32c24ade-d689-4252-a37a-52c61aa04e5a"
	title := "bili-fm"
	jsonMap := map[string]interface{}{
		"type": "event",
		"payload": map[string]interface{}{
			"website":  website,
			"screen":   "",
			"language": "",
			"title":    title,
			"hostname": "meimingzi.top",
			"url":      "https://meimingzi.top/" + title,
			"referrer": "",
		},
	}
	jsonStr, _ := json.Marshal(jsonMap)
	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   GetUserAgent(),
	}
	PostJson("https://api-gateway.umami.dev/api/send", jsonStr, headers)
}
