package main

import (
	"log"

	"github.com/egoist/mygo"

	"github.com/vst93/bili-fm/app/internal/mediactl"
	"github.com/vst93/bili-fm/app/internal/view"
)

// setupMediaCenter 连接系统媒体中心（Linux MPRIS；macOS Now Playing；
// Windows SMTC，窗口创建后 AttachWindow 注册）。
//
// 系统发过来的播放控制都丢到主线程执行（它们会改界面/播放器状态）。
func (c *controller) setupMediaCenter() {
	c.media = mediactl.New(mediactl.Callbacks{
		Play: func() {
			c.mediaOnMain(func() {
				if !c.app.Playing {
					c.togglePlay()
				}
			})
		},
		Pause: func() {
			c.mediaOnMain(func() {
				if c.app.Playing {
					c.togglePlay()
				}
			})
		},
		PlayPause: func() { c.mediaOnMain(c.togglePlay) },
		Next:      func() { c.mediaOnMain(func() { c.step(1) }) },
		Previous:  func() { c.mediaOnMain(func() { c.step(-1) }) },
		Stop: func() {
			c.mediaOnMain(func() {
				if c.app.Playing {
					c.togglePlay()
				}
			})
		},
		Seek: func(us int64) { c.mediaOnMain(func() { c.seek(float64(us) / 1e6) }) },
		SetVolume: func(v float64) {
			c.mediaOnMain(func() { c.setVolume(clamp01(v)) })
		},
		SetRate: func(v float64) {
			c.mediaOnMain(func() { c.setSpeed(v) })
		},
		SetLoop: func(loop string) {
			c.mediaOnMain(func() { c.setMediaLoop(loop) })
		},
		SetShuffle: func(on bool) {
			c.mediaOnMain(func() { c.setMediaShuffle(on) })
		},
	})
}

// attachMediaCenter 在窗口创建并显示后把媒体中心接到这个 HWND 上。
// Linux/macOS 的实现不实现 WindowAttacher，调用是 no-op。
func (c *controller) attachMediaCenter() {
	if c.media == nil || c.app == nil || c.app.Win == nil {
		return
	}
	if a, ok := c.media.(mediactl.WindowAttacher); ok {
		if err := a.AttachWindow(c.app.Win.NativeHandle()); err != nil {
			log.Printf("系统媒体中心初始化失败: %v", err)
		}
	}
	c.syncMediaTrack()
}

// setMediaLoop 把系统媒体中心发来的 LoopStatus 映射到播放模式。
func (c *controller) setMediaLoop(loop string) {
	a := c.app
	switch loop {
	case "Track":
		a.PlayMode = view.PlayModeSingle
	case "Playlist":
		a.PlayMode = view.PlayModeSequence
	default:
		// None = 顺序播完不循环；没有对应的第三态，归为顺序。
		a.PlayMode = view.PlayModeSequence
		if a.PlayingPlaylist == "" {
			a.PlayMode = view.PlayModeSequence
		}
	}
	_ = c.kv.SetString("playlistPlayMode", a.PlayMode)
	a.Win.Update(func() {})
}

// setMediaShuffle 把系统媒体中心发来的 Shuffle 应用到播放模式。
func (c *controller) setMediaShuffle(on bool) {
	a := c.app
	if on {
		a.PlayMode = view.PlayModeShuffle
	} else {
		a.PlayMode = view.PlayModeSequence
	}
	_ = c.kv.SetString("playlistPlayMode", a.PlayMode)
	a.Win.Update(func() {})
}

// clamp01 把音量夹到 0..1。
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// mediaOnMain 在界面线程执行系统媒体中心的控制请求。
func (c *controller) mediaOnMain(fn func()) {
	if c.app.Win == nil {
		return
	}
	c.app.Win.Update(fn)
}

// syncMediaTrack 把当前曲目、播放状态、可导航性推给系统媒体中心。
// 只在主线程调用（它读界面状态）。
func (c *controller) syncMediaTrack() {
	// Windows 任务栏缩略图工具栏先同步（不依赖系统媒体中心是否可用）。
	c.syncTaskbar()
	if c.media == nil {
		return
	}
	a := c.app
	status := mediactl.Paused
	if a.Playing {
		status = mediactl.Playing
	}
	if a.Track == nil {
		status = mediactl.Stopped
	}
	title, artist, art := "", "", ""
	if a.Track != nil {
		title, artist, art = a.Track.Title, a.Track.Up, a.Track.Cover
	}
	if a.Info != nil {
		if a.Info.Title != "" {
			title = a.Info.Title
		}
		if a.Info.OwnerName != "" {
			artist = a.Info.OwnerName
		}
		if a.Info.Pic != "" {
			art = a.Info.Pic
		}
	}
	length := int64(0)
	if a.Dur > 0 {
		length = int64(a.Dur * 1e6)
	}
	c.media.SetTrack(mediactl.Track{Title: title, Artist: artist, ArtURL: art, LengthUs: length})
	c.media.SetStatus(status)
	c.media.SetVolume(a.Volume)
	c.media.SetRate(a.Speed)
	nav := a.CanNavigate()
	c.media.SetNavigable(nav, nav)
	loop, shuffle := "None", false
	switch a.PlayMode {
	case view.PlayModeSingle:
		loop = "Track"
	case view.PlayModeShuffle:
		shuffle = true
	default:
		loop = "Playlist"
	}
	c.media.SetLoopMode(loop, shuffle)
	c.syncProgressBar()
}

// syncMediaPosition 只更新位置（MPRIS 的 Position 不发信号，随便刷）。
func (c *controller) syncMediaPosition(seconds float64) {
	if seconds < 0 {
		return
	}
	// 桌面字幕跟主播放位置走；没有窗口时 syncLyricsAt 立即返回。
	c.syncLyricsAt(seconds)
	c.syncProgressBar()
	if c.media == nil {
		return
	}
	c.media.SetPosition(int64(seconds * 1e6))
}

// syncProgressBar 把播放进度同步到系统任务栏 / Dock / 启动器。
//
// mygo 0.6 修好了 Windows ITaskbarList3 的 IID，SetProgressBar 才真正可用；
// 这里只在有曲目时显示进度，缓冲中显示 indeterminate，暂停用 paused 状态。
// 500ms 一次的调用对平台层只是几次轻量属性写入，不会影响播放。
func (c *controller) syncProgressBar() {
	a := c.app
	if a == nil || a.Win == nil {
		return
	}
	if a.Track == nil || a.Dur <= 0 {
		a.Win.SetProgressBar(mygo.ProgressBar{})
		return
	}
	if a.Buffering {
		a.Win.SetProgressBar(mygo.ProgressBar{State: mygo.ProgressIndeterminate})
		return
	}
	state := mygo.ProgressNormal
	if !a.Playing {
		state = mygo.ProgressPaused
	}
	value := a.Pos / a.Dur
	if value < 0 {
		value = 0
	} else if value > 1 {
		value = 1
	}
	a.Win.SetProgressBar(mygo.ProgressBar{State: state, Value: value})
}
