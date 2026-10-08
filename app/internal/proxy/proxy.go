// Package proxy 是内嵌的本地 HTTP 代理，监听 127.0.0.1:4654。
//
// 为什么需要它：视频弹窗是 webview，页面里的 <video> 没法带 Referer 和
// User-Agent 头，而 B 站 CDN 没有这两个头会返回 403。所以由 Go 侧代发请求。
//
// 端口沿用旧版（Wails/Tauri 都是 4654），这样页面里已经存在的 URL 不用变。
// 另外：媒体元素**不能**从自定义协议加载（见 mygo issue #161），所以视频
// 必须走这个本地 http 服务，不能换成 mygo 的自定义协议。
package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Port 是代理监听的端口，与旧版一致。
const Port = 4654

// 与旧版 main.go / proxy.rs 相同的 Chrome 131 UA。
const uaChrome131 = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

const (
	maxConcurrentFetches = 8
	failureCacheTTL      = 30 * time.Second
)

// Server 是代理服务器。
type Server struct {
	client *http.Client

	// 限制并发上游请求，避免一次滚动把连接数打满。
	limit chan struct{}

	mu       sync.Mutex
	failures map[string]time.Time

	srv *http.Server
}

// New 建一个代理。
func New() *Server {
	return &Server{
		client: &http.Client{
			Timeout: 0, // 音频是长连接，不能设总超时
			Transport: &http.Transport{
				// B 站图床有证书链不全的情况，旧版也是放行的。
				TLSClientConfig:       insecureTLS(),
				MaxIdleConns:          32,
				MaxIdleConnsPerHost:   8,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 20 * time.Second,
			},
		},
		limit:    make(chan struct{}, maxConcurrentFetches),
		failures: map[string]time.Time{},
	}
}

// Start 在后台启动代理。返回的错误只表示端口绑定失败。
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/image-proxy", s.handleImage)
	mux.HandleFunc("/audio-proxy", s.handleAudio)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", Port))
	if err != nil {
		return fmt.Errorf("proxy: 端口 %d 绑定失败: %w", Port, err)
	}
	s.srv = &http.Server{Handler: mux}
	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("proxy: 退出: %v", err)
		}
	}()
	return nil
}

// ImageURL 把图片地址包成代理 URL，空串原样返回。
func ImageURL(raw string) string {
	if raw == "" {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d/image-proxy?url=%s", Port, url.QueryEscape(raw))
}

// AudioURL 把音频地址包成代理 URL。
func AudioURL(raw string) string {
	if raw == "" {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d/audio-proxy?url=%s", Port, url.QueryEscape(raw))
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	target := normalize(r.URL.Query().Get("url"))
	if target == "" {
		http.Error(w, "缺少 url 参数", http.StatusBadRequest)
		return
	}
	if s.recentlyFailed(target) {
		http.Error(w, "图片暂时不可用", http.StatusBadGateway)
		return
	}
	select {
	case s.limit <- struct{}{}:
		defer func() { <-s.limit }()
	case <-r.Context().Done():
		return
	}

	// 与旧版一致：最多重试 3 次，间隔 200ms * attempt。
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(200*attempt) * time.Millisecond)
		}
		req, err := http.NewRequestWithContext(r.Context(), "GET", target, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.Header.Set("User-Agent", uaChrome131)
		req.Header.Set("Referer", "https://www.bilibili.com/")
		req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("上游返回 %d", resp.StatusCode)
			continue
		}
		ct := resp.Header.Get("Content-Type")
		if ct == "" {
			ct = "image/jpeg"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, err = io.Copy(w, resp.Body)
		resp.Body.Close()
		if err == nil {
			return
		}
		lastErr = err
	}
	s.markFailed(target)
	log.Printf("proxy: 图片失败 %s: %v", target, lastErr)
	http.Error(w, "图片获取失败", http.StatusBadGateway)
}

// handleAudio 转发音频请求，保留 Range 语义并回传必要的头，
// 这样 webview 的媒体栈能缓存分段、拖动进度走 Range 续传。
func (s *Server) handleAudio(w http.ResponseWriter, r *http.Request) {
	target := normalize(r.URL.Query().Get("url"))
	if target == "" {
		http.Error(w, "缺少 url 参数", http.StatusBadRequest)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", target, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Header.Set("User-Agent", uaChrome131)
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("Accept", "audio/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		http.Error(w, "音频获取失败", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	h := w.Header()
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "ETag", "Last-Modified", "Cache-Control"} {
		if v := resp.Header.Get(name); v != "" {
			h.Set(name, v)
		}
	}
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Accept-Ranges", "bytes")
	h.Set("Access-Control-Expose-Headers", "Accept-Ranges, Content-Length, Content-Range")
	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", "audio/mpeg")
	}
	// 上游没给缓存头时兜底：让媒体栈能缓存 Range 分段，拖动进度时走
	// 续传而不是重拉整个流（旧版同样处理）。
	if h.Get("Cache-Control") == "" {
		h.Set("Cache-Control", "public, max-age=86400")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// normalize 补全协议相对地址（"//host/path"）。
func normalize(raw string) string {
	if len(raw) > 2 && raw[0] == '/' && raw[1] == '/' {
		return "https:" + raw
	}
	return raw
}

func (s *Server) recentlyFailed(target string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	at, ok := s.failures[target]
	if !ok {
		return false
	}
	if time.Since(at) < failureCacheTTL {
		return true
	}
	delete(s.failures, target)
	return false
}

func (s *Server) markFailed(target string) {
	s.mu.Lock()
	s.failures[target] = time.Now()
	s.mu.Unlock()
}
