// Package video 管理视频弹窗：一个临时的 webview 窗口。
//
// 为什么视频要单独开 webview：H.264 没有可用的纯 Go 解码器（唯一像样的
// eyevinn/hi264 还很新），自己实现等于重写一个解码器。而浏览器引擎本来就
// 带着 H.264 硬解、系统原生控件、画中画、全屏 —— 这些自己写反而做不好。
//
// 为什么是「临时」窗口：一个 webview 窗口在 Windows 上带着约 330MB 的
// WebView2 基础开销（浏览器/GPU/网络/存储等进程）。实测：原生 UI 主窗
// 只有 57MB，打开媒体窗口升到 506MB，**关掉后回落到 69MB**。所以看完就关，
// 内存能收回来。
//
// 音频不在这里：音频走 internal/media 的纯 Go 引擎，不需要 webview。
package video

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// Options 描述要打开的视频。
type Options struct {
	// Src 是播放地址，应当是本地代理 URL（webview 带不了 Referer）。
	Src string
	// Title 显示在弹窗标题栏。
	Title string
	// Poster 是封面（可选）。
	Poster string
	// StartAt 是续播位置（秒）。
	StartAt float64
	// Speed 是当前倍速，跟随主窗。
	Speed float64
	// Muted 表示主窗是静音状态。
	Muted bool
	// Parent 是主窗，用于让弹窗跟随。
	Parent *mygo.Window
}

// State 是弹窗回传的播放状态。
type State struct {
	Time     float64
	Duration float64
	Paused   bool
	Muted    bool
	Speed    float64
	Ready    int
	Err      string
}

// Manager 管理唯一一个视频弹窗。
type Manager struct {
	mu  sync.Mutex
	win *mygo.Window
	st  State
	// opening 表示一次 Open 正在进行（异步的窗口创建落定之前，
	// 再来的 Open 直接拒绝，避免快速连点开出多个弹窗）。
	opening bool

	onState func(State)
	onClose func(State)
	onEnded func()
}

// New 建一个管理器。
func New() *Manager { return &Manager{} }

// OnState 设置状态回调（约每秒一次）。
func (m *Manager) OnState(fn func(State)) { m.onState = fn }

// OnClose 设置关闭回调。
func (m *Manager) OnClose(fn func(State)) { m.onClose = fn }

// OnEnded 设置播放结束回调。
func (m *Manager) OnEnded(fn func()) { m.onEnded = fn }

// IsOpen 报告弹窗是否开着。
func (m *Manager) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.win != nil
}

// State 返回最近一次回传的状态。
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.st
}

// Open 打开（或复用）弹窗。已有弹窗时先关掉再开，保证换集时是新地址。
// 有一次 Open 还在创建中时，后续的 Open 拒绝：快速连点不会开出多个。
func (m *Manager) Open(o Options) error {
	m.mu.Lock()
	if m.opening {
		m.mu.Unlock()
		return nil
	}
	m.opening = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.opening = false
		m.mu.Unlock()
	}()

	m.Close()

	q := url.Values{}
	q.Set("src", o.Src)
	q.Set("title", o.Title)
	q.Set("poster", o.Poster)
	q.Set("t", fmt.Sprintf("%.2f", o.StartAt))
	q.Set("speed", fmt.Sprintf("%.2f", max(o.Speed, 0.5)))
	if o.Muted {
		q.Set("muted", "1")
	}
	page := "/video.html?" + q.Encode()

	w := mygo.NewWindow(mygo.WindowOptions{
		Title:     o.Title,
		URL:       page,
		Width:     960,
		Height:    600,
		MinWidth:  320,
		MinHeight: 200,
		Frameless: true,
	})
	if w == nil {
		return fmt.Errorf("video: 创建弹窗失败")
	}

	m.mu.Lock()
	m.win = w
	m.st = State{Speed: o.Speed}
	m.mu.Unlock()

	w.OnClosed(func() {
		m.mu.Lock()
		if m.win == w {
			m.win = nil
		}
		st := m.st
		cb := m.onClose
		m.mu.Unlock()
		if cb != nil {
			cb(st)
		}
	})
	return nil
}

// Close 关闭弹窗（若开着）。
func (m *Manager) Close() {
	m.mu.Lock()
	w := m.win
	m.win = nil
	m.mu.Unlock()
	if w != nil {
		w.Close()
	}
}

// SetSpeed 同步倍速到弹窗。
func (m *Manager) SetSpeed(v float64) {
	m.mu.Lock()
	w := m.win
	m.mu.Unlock()
	if w != nil {
		w.Page().Eval(fmt.Sprintf("(()=>{const v=document.getElementById('v');if(v)v.playbackRate=%f;})()", v))
	}
}

// Pause / Resume 供主窗控制弹窗（例如主窗按了空格）。
func (m *Manager) Pause()  { m.eval("document.getElementById('v')?.pause()") }
func (m *Manager) Resume() { m.eval("document.getElementById('v')?.play()") }

func (m *Manager) eval(js string) {
	m.mu.Lock()
	w := m.win
	m.mu.Unlock()
	if w != nil {
		w.Page().Eval(js)
	}
}

// ---------------------------------------------------------------- 页面 → Go

// Service 是暴露给弹窗页面的服务。
type Service struct{ m *Manager }

// NewService 建一个页面服务。
func NewService(m *Manager) *Service { return &Service{m: m} }

// Log 页面写日志。
func (s *Service) Log(msg string) { videoLog("弹窗 | " + msg) }

// Report 页面每秒回传状态。
func (s *Service) Report(t, dur float64, paused, muted bool, speed float64, ready int, err string) {
	s.m.mu.Lock()
	s.m.st = State{Time: t, Duration: dur, Paused: paused, Muted: muted, Speed: speed, Ready: ready, Err: err}
	cb := s.m.onState
	s.m.mu.Unlock()
	if cb != nil {
		cb(s.m.st)
	}
}

// Close 页面请求关闭弹窗。
func (s *Service) Close() {
	go func() {
		time.Sleep(20 * time.Millisecond) // 让这次调用先返回
		s.m.Close()
	}()
}

// Ended 页面报告播放结束。
func (s *Service) Ended() {
	s.m.mu.Lock()
	cb := s.m.onEnded
	s.m.mu.Unlock()
	if cb != nil {
		go cb()
	}
}

var videoLog = func(string) {}

// SetLog 注入日志函数（避免这个包依赖调用方的日志实现）。
func SetLog(fn func(string)) { videoLog = fn }

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
