package main

import "github.com/vst93/bili-fm/app/internal/mediactl"

// setupMediaCenter 连接系统媒体中心（Linux MPRIS；其他平台 no-op）。
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
	})
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
}

// syncMediaPosition 只更新位置（MPRIS 的 Position 不发信号，随便刷）。
func (c *controller) syncMediaPosition(seconds float64) {
	if c.media == nil || seconds < 0 {
		return
	}
	c.media.SetPosition(int64(seconds * 1e6))
}
