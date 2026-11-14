package view

import "sync"

// PlayerSettingKind 是共享设置里被改动的那一项。监听方按它决定要做什么：
// 主窗的监听把值落到音频引擎并落盘，视频窗的监听把值落到自己的播放器。
type PlayerSettingKind int

const (
	// SettingVolume 覆盖音量与静音（静音就是音量 0，见 PlayerSettings.SetVolume）。
	SettingVolume PlayerSettingKind = iota
	SettingSpeed
	SettingEQ
	SettingSponsor
)

// PlayerSettings 是主窗播放栏与视频窗控件条**共用的一份内存**。
//
// 分工（详见 PLAN.md 的「视频窗控件分工」）：
//
//   - 共享：音量 / 静音 / 倍速 / 音量均衡 / 广告跳过（含 SponsorBlock 状态）。
//     两个窗口拿到的是同一个指针，任何一侧改写都会立刻被另一侧看见；
//     监听器再把值下发到各自的引擎（主窗音频引擎 / 视频窗播放器）。
//   - 独立：播放/暂停、进度、时间、缓冲、上一集/下一集。它们不在这个结构里，
//     分别由 App（音频引擎）与 VideoChrome（视频播放器）自己的传输状态驱动。
//
// 所有字段都在界面主线程上读写（mygo 的所有窗口共用同一个主循环）；
// listeners 另加一把锁，只保护监听器列表本身，避免注册/注销与通知撞车。
type PlayerSettings struct {
	Volume     float64
	LastVolume float64
	Muted      bool
	Speed      float64
	EQ         bool
	Sponsor    bool

	// SponsorStatus / SponsorCount / SponsorReason 是 SponsorBlock 的查询状态，
	// 主窗写入、视频窗读取（状态点与 tooltip 两边同一份）。
	SponsorStatus string
	SponsorCount  int
	SponsorReason string

	mu        sync.Mutex
	listeners []playerSettingListener
	nextID    uint64
	persist   func(PlayerSettingKind)
	ready     bool
}

// playerSettingListener 带一个 id：注销时按 id 删除，函数值本身不可比较。
type playerSettingListener struct {
	id uint64
	fn func(PlayerSettingKind)
}

// Ensure 补齐零值。只做一次：之后音量 0（用户主动静音）不会被重新抬成 1。
func (s *PlayerSettings) Ensure() {
	if s == nil || s.ready {
		return
	}
	s.ready = true
	if s.Speed <= 0 {
		s.Speed = 1
	}
	if s.Volume == 0 && !s.Muted {
		s.Volume = 1
	}
	if s.LastVolume <= 0 {
		s.LastVolume = 1
	}
	if s.SponsorStatus == "" {
		if s.Sponsor {
			s.SponsorStatus = "loading"
		} else {
			s.SponsorStatus = "off"
		}
	}
}

// OnChange 注册一个设置变更监听，返回注销函数。两个窗口各注册一个：
// 主窗的负责音频引擎与落盘，视频窗的负责它自己的播放器。
func (s *PlayerSettings) OnChange(fn func(PlayerSettingKind)) func() {
	if s == nil || fn == nil {
		return func() {}
	}
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.listeners = append(s.listeners, playerSettingListener{id: id, fn: fn})
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		for i, l := range s.listeners {
			if l.id == id {
				s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
	}
}

// SetPersist 设置落盘钩子（只有主窗接 KV；视频窗共享同一个指针，所以视频窗
// 里拖完音量也能落盘）。kind 是 SettingVolume 时表示「音量拖动提交」。
func (s *PlayerSettings) SetPersist(fn func(PlayerSettingKind)) { s.persist = fn }

// PersistVolume 请求把当前音量写盘（拖动松手时调用一次，不是每帧）。
func (s *PlayerSettings) PersistVolume() {
	if s != nil && s.persist != nil {
		s.persist(SettingVolume)
	}
}

func (s *PlayerSettings) notify(k PlayerSettingKind) {
	if s == nil {
		return
	}
	s.mu.Lock()
	listeners := make([]func(PlayerSettingKind), len(s.listeners))
	for i, l := range s.listeners {
		listeners[i] = l.fn
	}
	s.mu.Unlock()
	for _, fn := range listeners {
		fn(k)
	}
}

// SetVolume 设置音量并维护「静音 = 音量 0」这条不变式（原版语义）。
func (s *PlayerSettings) SetVolume(v float64) {
	if s == nil {
		return
	}
	s.Ensure()
	v = clampF(v, 0, 1)
	if v == s.Volume && (v <= 0) == s.Muted {
		return
	}
	s.Volume = v
	if v > 0 {
		s.LastVolume = v
	}
	s.Muted = v <= 0
	s.notify(SettingVolume)
}

// SetMuted 直接设置静音（取消静音时恢复到 LastVolume，没有记录就回到 1）。
func (s *PlayerSettings) SetMuted(m bool) {
	if s == nil {
		return
	}
	s.Ensure()
	if m {
		s.SetVolume(0)
		return
	}
	v := s.LastVolume
	if v <= 0 {
		v = 1
	}
	s.SetVolume(v)
}

// ToggleMute 是静音键的语义：有声音就归零并记住，静音就恢复上次音量。
func (s *PlayerSettings) ToggleMute() {
	if s == nil {
		return
	}
	s.Ensure()
	if s.Volume > 0 {
		s.LastVolume = s.Volume
		s.SetVolume(0)
	} else {
		s.SetMuted(false)
	}
	s.PersistVolume()
}

// SetSpeed 设置倍速（非法值忽略，档位由界面给）。
func (s *PlayerSettings) SetSpeed(v float64) {
	if s == nil || v <= 0 {
		return
	}
	s.Ensure()
	if v == s.Speed {
		return
	}
	s.Speed = v
	s.notify(SettingSpeed)
}

// SetEQ 切换音量均衡。
func (s *PlayerSettings) SetEQ(on bool) {
	if s == nil {
		return
	}
	s.Ensure()
	if s.EQ == on {
		return
	}
	s.EQ = on
	s.notify(SettingEQ)
}

// SetSponsor 切换广告跳过。状态点由主窗随后写入（loading/ok/empty/error）。
func (s *PlayerSettings) SetSponsor(on bool) {
	if s == nil {
		return
	}
	s.Ensure()
	if s.Sponsor == on {
		return
	}
	s.Sponsor = on
	s.notify(SettingSponsor)
}
