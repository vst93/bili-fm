// Package mediactl 把正在播放的曲目同步到系统媒体中心。
//
// 各平台的实现：
//
//	Linux   MPRIS（org.mpris.MediaPlayer2），桌面环境的「正在播放」卡片、
//	        媒体键、锁屏控件都读它。
//	macOS   mygo 目前没有 Now Playing 的绑定，暂为 no-op。
//	Windows 同上（SMTC），暂为 no-op。
//
// 界面只通过这个接口调用，平台差异都收在实现文件里。
package mediactl

// Track 是要展示给系统媒体中心的一条曲目。
type Track struct {
	Title  string
	Artist string
	Album  string
	// ArtURL 是封面地址（http/https，系统自己去取）。
	ArtURL string
	// LengthUs 是时长（微秒，MPRIS 的单位）；未知为 0。
	LengthUs int64
}

// Status 是播放状态（取值与 MPRIS 的 PlaybackStatus 一致）。
type Status string

const (
	Playing Status = "Playing"
	Paused  Status = "Paused"
	Stopped Status = "Stopped"
)

// Callbacks 是系统媒体中心发过来的控制请求。
type Callbacks struct {
	Play      func()
	Pause     func()
	PlayPause func()
	Next      func()
	Previous  func()
	Stop      func()
	// Seek 是绝对定位（微秒），对应 MPRIS 的 SetPosition / Seek(相对)。
	Seek func(us int64)
}

// Controller 是系统媒体中心连接。方法都应当可以被任意 goroutine 调用。
type Controller interface {
	SetTrack(Track)
	SetStatus(Status)
	// SetPosition 上报当前播放位置（微秒）。
	SetPosition(us int64)
	// SetVolume 上报音量 0..1。
	SetVolume(v float64)
	// SetNavigable 上报「有下一首/上一首」，系统据此禁用按钮。
	SetNavigable(next, prev bool)
	Close()
}

// noop 是不支持或连不上系统媒体中心时的空实现。
type noop struct{}

func (noop) SetTrack(Track)               {}
func (noop) SetStatus(Status)             {}
func (noop) SetPosition(int64)            {}
func (noop) SetVolume(float64)            {}
func (noop) SetNavigable(next, prev bool) {}
func (noop) Close()                       {}
