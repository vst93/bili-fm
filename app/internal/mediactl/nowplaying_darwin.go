//go:build darwin

package mediactl

import (
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// macOS 的「系统媒体中心」实现：MPRemoteCommandCenter + MPNowPlayingInfoCenter。
//
// 为什么不是 MPRIS 那套：macOS 没有 MPRIS。媒体键（含蓝牙耳机上的播放/暂停）
// 由系统路由给**当前登记为 Now Playing 的应用**：应用要
//
//	① 在 MPRemoteCommandCenter 上为 play/pause/togglePlayPause/next/previous
//	   注册 handler，并 setEnabled:YES；
//	② 往 MPNowPlayingInfoCenter.defaultCenter 写 nowPlayingInfo（标题/时长等）
//	   与 playbackState，才能「占住」Now Playing 这个席位。
//
// 原版 Tauri 靠 WKWebView 自带的 Media Session 接管；原生版没有 WebView，
// 而 mygo 的全局快捷键走 Carbon RegisterEventHotKey，媒体键根本没有键码，
// 于是 mac 上媒体键完全失效（用户报「蓝牙耳机按暂停没反应」）。这里用 purego
// 直接调 Objective-C 运行时补上这块。
//
// 任何一步失败都静默降级成 noop：媒体键是锦上添花，不能影响播放。
type nowPlaying struct {
	mu     sync.Mutex
	cb     Callbacks
	center objc.ID // MPRemoteCommandCenter.sharedCommandCenter
	info   objc.ID // MPNowPlayingInfoCenter.defaultCenter

	track  Track
	status Status
	posUs  int64

	// blocks 持有注册给命令中心的 block，防止被 Go GC 回收。
	blocks []objc.Block
}

var (
	mediaPlayerOnce sync.Once
	mediaPlayerErr  error
)

// loadMediaPlayer 加载 MediaPlayer.framework（MPRemoteCommandCenter 等类都在
// 这里）。RTLD_GLOBAL 让 objc_getClass 能找到它注册的类。
func loadMediaPlayer() error {
	mediaPlayerOnce.Do(func() {
		_, mediaPlayerErr = purego.Dlopen(
			"/System/Library/Frameworks/MediaPlayer.framework/MediaPlayer",
			purego.RTLD_GLOBAL|purego.RTLD_NOW)
	})
	return mediaPlayerErr
}

// nsString 造一个 NSString。
func nsString(s string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).
		Send(objc.RegisterName("stringWithUTF8String:"), s+"\x00")
}

// nsNumber 造一个 NSNumber。
func nsNumber(v float64) objc.ID {
	return objc.ID(objc.GetClass("NSNumber")).Send(objc.RegisterName("numberWithDouble:"), v)
}

// New 连接 macOS 的 Now Playing / 远程控制中心。失败返回 noop。
func New(cb Callbacks) Controller {
	n := &nowPlaying{cb: cb}
	if err := n.setup(); err != nil {
		return noop{}
	}
	return n
}

// setup 找到两个中心并把各条命令的 handler 挂上。整个过程 panic 都会被兜成
// 错误（纯 Go 调 objc，任何一处签名不对都可能 panic，不能让它带崩应用）。
func (n *nowPlaying) setup() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("now playing: %v", r)
		}
	}()
	if err := loadMediaPlayer(); err != nil {
		return err
	}
	centerClass := objc.GetClass("MPRemoteCommandCenter")
	infoClass := objc.GetClass("MPNowPlayingInfoCenter")
	if centerClass == 0 || infoClass == 0 {
		return fmt.Errorf("now playing: MediaPlayer classes not found")
	}
	n.center = objc.ID(centerClass).Send(objc.RegisterName("sharedCommandCenter"))
	n.info = objc.ID(infoClass).Send(objc.RegisterName("defaultCenter"))
	if n.center == 0 || n.info == 0 {
		return fmt.Errorf("now playing: centers unavailable")
	}

	// 每条命令：取命令对象、置 enabled、addTargetWithHandler。
	// 蓝牙耳机/键盘上的「播放/暂停」在系统里会派发成 togglePlayPause 或
	// play/pause 三条里的一条，三条都接上才不会漏。
	n.wire("togglePlayPauseCommand", func() {
		if n.cb.PlayPause != nil {
			n.cb.PlayPause()
		}
	})
	n.wire("playCommand", func() {
		if n.cb.Play != nil {
			n.cb.Play()
		}
	})
	n.wire("pauseCommand", func() {
		if n.cb.Pause != nil {
			n.cb.Pause()
		}
	})
	n.wire("stopCommand", func() {
		if n.cb.Stop != nil {
			n.cb.Stop()
		}
	})
	n.wire("nextTrackCommand", func() {
		if n.cb.Next != nil {
			n.cb.Next()
		}
	})
	n.wire("previousTrackCommand", func() {
		if n.cb.Previous != nil {
			n.cb.Previous()
		}
	})
	// 进度条拖动（锁屏/控制中心的 seek）：事件带 positionTime（秒）。
	n.wireEvent("changePlaybackPositionCommand", func(event objc.ID) {
		if n.cb.Seek == nil {
			return
		}
		pos := objc.Send[float64](event, objc.RegisterName("positionTime"))
		n.cb.Seek(int64(pos * 1e6))
	})
	return nil
}

// wire 给一条无参命令挂 handler 并启用。
func (n *nowPlaying) wire(name string, fn func()) {
	n.wireEvent(name, func(objc.ID) { fn() })
}

// wireEvent 给一条命令挂带事件的 handler 并启用。
func (n *nowPlaying) wireEvent(name string, fn func(event objc.ID)) {
	cmd := n.center.Send(objc.RegisterName(name))
	if cmd == 0 {
		return
	}
	cmd.Send(objc.RegisterName("setEnabled:"), int(1))
	block := objc.NewBlock(func(_ objc.Block, event objc.ID) int64 {
		fn(event)
		return 0 // MPRemoteCommandHandlerStatusSuccess
	})
	n.blocks = append(n.blocks, block)
	cmd.Send(objc.RegisterName("addTargetWithHandler:"), block)
}

func (n *nowPlaying) SetTrack(t Track) {
	n.mu.Lock()
	n.track = t
	n.mu.Unlock()
	n.pushInfo()
}

func (n *nowPlaying) SetStatus(s Status) {
	n.mu.Lock()
	n.status = s
	n.mu.Unlock()
	n.pushStatus()
}

func (n *nowPlaying) SetPosition(us int64) {
	n.mu.Lock()
	n.posUs = us
	n.mu.Unlock()
	n.pushInfo()
}

func (n *nowPlaying) SetVolume(float64) {}
func (n *nowPlaying) SetRate(float64)   {}
func (n *nowPlaying) SetNavigable(next, prev bool) {
	// 命令中心不按「有没有下一首」禁用；step() 自己会忽略越界。
}
func (n *nowPlaying) SetLoopMode(string, bool) {}
func (n *nowPlaying) Close()                   {}

// pushInfo 把曲目信息写进 MPNowPlayingInfoCenter.nowPlayingInfo。
func (n *nowPlaying) pushInfo() {
	if n.info == 0 {
		return
	}
	defer func() { _ = recover() }()
	n.mu.Lock()
	t, posUs := n.track, n.posUs
	n.mu.Unlock()

	dict := objc.ID(objc.GetClass("NSMutableDictionary")).
		Send(objc.RegisterName("dictionary"))
	set := objc.RegisterName("setObject:forKey:")
	if t.Title != "" {
		dict.Send(set, nsString(t.Title), nsString("title"))
	}
	if t.Artist != "" {
		dict.Send(set, nsString(t.Artist), nsString("artist"))
	}
	if t.Album != "" {
		dict.Send(set, nsString(t.Album), nsString("albumTitle"))
	}
	if t.LengthUs > 0 {
		dict.Send(set, nsNumber(float64(t.LengthUs)/1e6), nsString("playbackDuration"))
	}
	dict.Send(set, nsNumber(float64(posUs)/1e6), nsString("elapsedPlaybackTime"))
	n.info.Send(objc.RegisterName("setNowPlayingInfo:"), dict)
}

// pushStatus 写 playbackState（系统据此决定媒体键派发到哪条命令、控制中心
// 显示播放还是暂停）。
func (n *nowPlaying) pushStatus() {
	if n.info == 0 {
		return
	}
	defer func() { _ = recover() }()
	n.mu.Lock()
	s := n.status
	n.mu.Unlock()
	n.info.Send(objc.RegisterName("setPlaybackState:"), nowPlayingState(s))
}
