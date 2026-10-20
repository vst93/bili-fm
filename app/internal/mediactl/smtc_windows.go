//go:build windows

package mediactl

// Windows 的系统媒体中心：System Media Transport Controls（SMTC）。
//
// 这套调用没有 cgo，也没有引入 WinRT 包装库：直接通过 combase.dll 的
// RoGetActivationFactory / RoActivateInstance 拿 COM 接口，再按 vtable 调用。
// 只在窗口创建后 AttachWindow(HWND) 时注册；失败静默降级为 no-op，绝不影响
// 音频播放。
//
// 已接：
//   - 标题 / 艺术家 / 专辑 / 封面 URL
//   - 播放状态、上一首 / 下一首可用性
//   - 播放 / 暂停 / 停止 / 上一首 / 下一首按键（含蓝牙耳机媒体键）
//   - 时间轴（时长 / 位置）与系统侧拖动进度
//
// 音量与倍速暂不写回系统：SMTC 没有音量属性，倍速的 WinRT 方法走浮点 ABI，
// 用 SyscallN 传 double 不可靠；这两项仍由应用内控件负责。

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	smtcClassName     = "Windows.Media.SystemMediaTransportControls"
	smtcTimelineClass = "Windows.Media.SystemMediaTransportControlsTimelineProperties"
	smtcArgsClass     = "Windows.Media.SystemMediaTransportControlsButtonPressedEventArgs"
	smtcSeekArgsClass = "Windows.Media.PlaybackPositionChangeRequestedEventArgs"
	smtcControlsIID   = "99fa3ff4-1742-42a6-902e-087d41f965ec"
	smtcControls2IID  = "ea98d2f6-7f3c-4af2-a586-72889808efb1"
	smtcArgsIID       = "b7f47116-a56f-4dc8-9e11-92031f4a87c2"
	smtcSeekArgsIID   = "b4493f88-eb28-4961-9c14-335e44f3e125"

	typeMusic     = 1
	statusStopped = 2
	statusPlaying = 3
	statusPaused  = 4

	buttonPlay     = 0
	buttonPause    = 1
	buttonStop     = 2
	buttonNext     = 6
	buttonPrevious = 7

	// ITaskbar/SMTC vtable 索引；IInspectable 的 6 个方法之后开始。
	interopGetForWindow = 6

	controlsGetStatus      = 6
	controlsPutStatus      = 7
	controlsGetUpdater     = 8
	controlsPutEnabled     = 11
	controlsPutPlay        = 13
	controlsPutStop        = 15
	controlsPutPause       = 17
	controlsPutPrevious    = 25
	controlsPutNext        = 27
	controlsAddButtonPress = 32
	controlsRemoveButton   = 33

	updaterPutType      = 7
	updaterPutThumbnail = 11
	updaterGetMusic     = 12
	updaterClearAll     = 16
	updaterUpdate       = 17

	musicPutTitle  = 7
	musicPutArtist = 11
	music2PutAlbum = 7

	uriCreate          = 6
	streamsCreateByURI = 7

	controls2UpdateTimeline       = 12
	controls2AddPositionChange    = 13
	controls2RemovePositionChange = 14
	controls2PutAutoRepeat        = 7
	controls2PutShuffle           = 9

	timelinePutStart    = 7
	timelinePutEnd      = 9
	timelinePutMinSeek  = 11
	timelinePutMaxSeek  = 13
	timelinePutPosition = 15
)

var (
	iidSmtcInterop = mustGUID("{DDB0472D-C911-4A1F-86D9-DC3D71A95F5A}")
	iidSmtc        = mustGUID("{" + smtcControlsIID + "}")
	iidSmtc2       = mustGUID("{" + smtcControls2IID + "}")
	iidMusic2      = mustGUID("{00368462-97D3-44B9-B00F-008AFCEFAF18}")
	iidURI         = mustGUID("{44A9796F-723E-4FDF-A218-033E75B0C084}")
	iidStreams     = mustGUID("{857309DC-3FBF-4E7D-986F-EF3B1A07A964}")
	iidTimeline    = mustGUID("{5125316A-C3A2-475B-8507-93534DC88F15}")
	iidButtonArgs  = mustGUID("{" + smtcArgsIID + "}")
	iidSeekArgs    = mustGUID("{" + smtcSeekArgsIID + "}")
	iidUnknown     = mustGUID("{00000000-0000-0000-C000-000000000046}")
	iidAgile       = mustGUID("{94EA2B94-E9CC-49E0-C0FF-EE64CA8F5B90}")
	iidButtonEvent = mustGUID(typedEventHandlerIID(smtcClassName, smtcControlsIID, smtcArgsClass, smtcArgsIID))
	iidSeekEvent   = mustGUID(typedEventHandlerIID(smtcClassName, smtcControlsIID, smtcSeekArgsClass, smtcSeekArgsIID))
)

var (
	combaseDLL         = windows.NewLazySystemDLL("combase.dll")
	procRoInitialize   = combaseDLL.NewProc("RoInitialize")
	procRoUninitialize = combaseDLL.NewProc("RoUninitialize")
	procRoGetFactory   = combaseDLL.NewProc("RoGetActivationFactory")
	procRoActivate     = combaseDLL.NewProc("RoActivateInstance")
	procCreateString   = combaseDLL.NewProc("WindowsCreateString")
	procDeleteString   = combaseDLL.NewProc("WindowsDeleteString")
)

func mustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString(s)
	if err != nil {
		panic(err)
	}
	return g
}

type hresult uint32

func (h hresult) Error() string { return fmt.Sprintf("Windows error 0x%08X", uint32(h)) }

func vcall(obj unsafe.Pointer, index int, args ...uintptr) error {
	if obj == nil {
		return errors.New("smtc: nil COM object")
	}
	table := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(table, uintptr(index)*unsafe.Sizeof(uintptr(0))))
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(obj))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	if int32(uint32(r)) < 0 {
		return hresult(uint32(r))
	}
	return nil
}

func release(p *unsafe.Pointer) {
	if p != nil && *p != nil {
		_ = vcall(*p, 2)
		*p = nil
	}
}

func hstring(s string) (uintptr, error) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h uintptr
	hr, _, _ := procCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h)))
	if int32(uint32(hr)) < 0 {
		return 0, hresult(uint32(hr))
	}
	return h, nil
}

func deleteHString(h uintptr) { procDeleteString.Call(h) }

func factory(class string, iid *windows.GUID) (unsafe.Pointer, error) {
	h, err := hstring(class)
	if err != nil {
		return nil, err
	}
	defer deleteHString(h)
	var out unsafe.Pointer
	hr, _, _ := procRoGetFactory.Call(h, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	if int32(uint32(hr)) < 0 {
		return nil, hresult(uint32(hr))
	}
	return out, nil
}

func activateInstance(class string) (unsafe.Pointer, error) {
	h, err := hstring(class)
	if err != nil {
		return nil, err
	}
	defer deleteHString(h)
	var out unsafe.Pointer
	hr, _, _ := procRoActivate.Call(h, uintptr(unsafe.Pointer(&out)))
	if int32(uint32(hr)) < 0 {
		return nil, hresult(uint32(hr))
	}
	return out, nil
}

func putString(obj unsafe.Pointer, index int, s string) {
	h, err := hstring(s)
	if err != nil {
		return
	}
	defer deleteHString(h)
	_ = vcall(obj, index, h)
}

// smtcHandler 是 WinRT 传进来的 TypedEventHandler。它只需要实现 IUnknown
// 与事件接口的 QI；AddRef/Release 保持对象存活到 smtc 关闭。
type smtcHandler struct {
	// lpVtbl 必须是第一个字段：COM 接口指针指向这里，*(*unsafe.Pointer)(ptr)
	// 读到的才是 vtable。漏了这一层会把 QueryInterface 的地址当成 vtable。
	lpVtbl *[4]uintptr
	vtbl   [4]uintptr
	s      *smtc
	iid    windows.GUID
	kind   int // 0 = button, 1 = seek
}

func callbacksHandler(s *smtc, kind int, iid windows.GUID) *smtcHandler {
	h := &smtcHandler{s: s, iid: iid, kind: kind}
	h.vtbl = [4]uintptr{
		windows.NewCallback(func(this unsafe.Pointer, iid *windows.GUID, out *unsafe.Pointer) uintptr {
			hh := (*smtcHandler)(this)
			if iid != nil && (*iid == iidUnknown || *iid == iidAgile || *iid == hh.iid) {
				*out = this
				return 0
			}
			*out = nil
			return 0x80004002
		}),
		windows.NewCallback(func(this unsafe.Pointer) uintptr { return 2 }),
		windows.NewCallback(func(this unsafe.Pointer) uintptr { return 1 }),
		windows.NewCallback(func(this unsafe.Pointer, sender, args unsafe.Pointer) uintptr {
			hh := (*smtcHandler)(this)
			if hh.s == nil {
				return 0
			}
			if hh.kind == 0 {
				button := smtcButtonFromArgs(args)
				hh.s.onButton(button)
			} else {
				pos := smtcSeekFromArgs(args)
				hh.s.onSeek(pos)
			}
			return 0
		}),
	}
	h.lpVtbl = &h.vtbl
	return h
}

func smtcButtonFromArgs(args unsafe.Pointer) int32 {
	if args == nil {
		return -1
	}
	var itf unsafe.Pointer
	if err := vcall(args, 0, uintptr(unsafe.Pointer(&iidButtonArgs)), uintptr(unsafe.Pointer(&itf))); err != nil || itf == nil {
		return -1
	}
	defer release(&itf)
	var button int32
	_ = vcall(itf, 6, uintptr(unsafe.Pointer(&button)))
	return button
}

func smtcSeekFromArgs(args unsafe.Pointer) int64 {
	if args == nil {
		return 0
	}
	var itf unsafe.Pointer
	if err := vcall(args, 0, uintptr(unsafe.Pointer(&iidSeekArgs)), uintptr(unsafe.Pointer(&itf))); err != nil || itf == nil {
		return 0
	}
	defer release(&itf)
	var pos int64
	_ = vcall(itf, 6, uintptr(unsafe.Pointer(&pos)))
	return pos
}

type smtc struct {
	mu sync.Mutex
	cb Callbacks

	hwnd     uintptr
	attached bool

	controls, updater, music, music2 unsafe.Pointer
	uris, streams                    unsafe.Pointer
	controls2, timeline, timelineObj unsafe.Pointer
	buttonHandler, seekHandler       *smtcHandler
	buttonToken, seekToken           int64

	track   Track
	status  Status
	posUs   int64
	next    bool
	prev    bool
	loop    string
	shuffle bool
	art     string
}

// New 在 Windows 上返回一个等待 AttachWindow 的控制器。
func New(cb Callbacks) Controller {
	return &smtc{cb: cb, status: Stopped, loop: "None"}
}

// AttachWindow 在窗口创建后注册 SMTC；必须在窗口所属线程调用。
func (s *smtc) AttachWindow(hwnd uintptr) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.attached {
		return nil
	}
	if hwnd == 0 {
		return errors.New("smtc: no window")
	}
	if err := s.attachLocked(hwnd); err != nil {
		s.releaseLocked()
		return err
	}
	s.hwnd = hwnd
	s.attached = true
	s.applyMetaLocked()
	s.applyStatusLocked()
	s.applyNavLocked()
	s.applyTimelineLocked()
	s.applyLoopLocked()
	return nil
}

func (s *smtc) attachLocked(hwnd uintptr) error {
	procRoInitialize.Call(0) // STA/RPC_E_CHANGED_MODE 都直接继续；窗口线程已由 mygo 初始化
	interop, err := factory(smtcClassName, &iidSmtcInterop)
	if err != nil {
		return fmt.Errorf("SMTC interop: %w", err)
	}
	defer release(&interop)
	if err := vcall(interop, interopGetForWindow, hwnd,
		uintptr(unsafe.Pointer(&iidSmtc)), uintptr(unsafe.Pointer(&s.controls))); err != nil {
		return fmt.Errorf("SMTC GetForWindow: %w", err)
	}
	if s.controls == nil {
		return errors.New("SMTC controls unavailable")
	}
	if err := vcall(s.controls, controlsGetUpdater, uintptr(unsafe.Pointer(&s.updater))); err != nil || s.updater == nil {
		return fmt.Errorf("SMTC display updater: %w", err)
	}
	_ = vcall(s.updater, updaterPutType, typeMusic)
	_ = vcall(s.updater, updaterGetMusic, uintptr(unsafe.Pointer(&s.music)))
	_ = vcall(s.music, 0, uintptr(unsafe.Pointer(&iidMusic2)), uintptr(unsafe.Pointer(&s.music2)))

	// 播放控制按钮。
	for _, idx := range []int{controlsPutEnabled, controlsPutPlay, controlsPutPause, controlsPutStop, controlsPutPrevious, controlsPutNext} {
		_ = vcall(s.controls, idx, 1)
	}
	s.buttonHandler = callbacksHandler(s, 0, iidButtonEvent)
	if err := vcall(s.controls, controlsAddButtonPress,
		uintptr(unsafe.Pointer(s.buttonHandler)), uintptr(unsafe.Pointer(&s.buttonToken))); err != nil {
		return fmt.Errorf("SMTC button event: %w", err)
	}

	// SMTC 2 提供时间轴与 seek 事件；拿不到就只保留基本控制。
	var c2 unsafe.Pointer
	if err := vcall(s.controls, 0, uintptr(unsafe.Pointer(&iidSmtc2)), uintptr(unsafe.Pointer(&c2))); err == nil && c2 != nil {
		s.controls2 = c2
		if obj, err := activateInstance(smtcTimelineClass); err == nil && obj != nil {
			var timeline unsafe.Pointer
			if err := vcall(obj, 0, uintptr(unsafe.Pointer(&iidTimeline)), uintptr(unsafe.Pointer(&timeline))); err == nil {
				s.timelineObj = obj
				s.timeline = timeline
			} else {
				release(&obj)
			}
		}
		if s.timeline != nil {
			s.seekHandler = callbacksHandler(s, 1, iidSeekEvent)
			_ = vcall(s.controls2, controls2AddPositionChange,
				uintptr(unsafe.Pointer(s.seekHandler)), uintptr(unsafe.Pointer(&s.seekToken)))
		}
	}

	// 封面工厂；失败不影响标题/控制。
	if f, err := factory("Windows.Foundation.Uri", &iidURI); err == nil {
		s.uris = f
	}
	if f, err := factory("Windows.Storage.Streams.RandomAccessStreamReference", &iidStreams); err == nil {
		s.streams = f
	}
	return nil
}

func (s *smtc) releaseLocked() {
	if s.controls != nil && s.buttonToken != 0 {
		_ = vcall(s.controls, controlsRemoveButton, uintptr(s.buttonToken))
		s.buttonToken = 0
	}
	if s.controls2 != nil && s.seekToken != 0 {
		_ = vcall(s.controls2, controls2RemovePositionChange, uintptr(s.seekToken))
		s.seekToken = 0
	}
	release(&s.timeline)
	release(&s.timelineObj)
	release(&s.controls2)
	release(&s.music2)
	release(&s.music)
	release(&s.updater)
	release(&s.uris)
	release(&s.streams)
	release(&s.controls)
	s.attached = false
	s.buttonHandler = nil
	s.seekHandler = nil
}

func (s *smtc) SetTrack(t Track) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.track = t
	if s.attached {
		s.applyMetaLocked()
		s.applyTimelineLocked()
	}
}

func (s *smtc) SetStatus(st Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = st
	if s.attached {
		s.applyStatusLocked()
	}
}

func (s *smtc) SetPosition(us int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.posUs = us
	if s.attached {
		s.applyTimelineLocked()
	}
}

func (s *smtc) SetVolume(float64) {}
func (s *smtc) SetRate(float64)   {}

func (s *smtc) SetNavigable(next, prev bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next, s.prev = next, prev
	if s.attached {
		s.applyNavLocked()
	}
}

func (s *smtc) SetLoopMode(loop string, shuffle bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loop, s.shuffle = loop, shuffle
	if s.attached && s.controls2 != nil {
		mode := uintptr(0)
		switch loop {
		case "Track":
			mode = 1
		case "Playlist":
			mode = 2
		}
		_ = vcall(s.controls2, controls2PutAutoRepeat, mode)
		if shuffle {
			_ = vcall(s.controls2, controls2PutShuffle, 1)
		} else {
			_ = vcall(s.controls2, controls2PutShuffle, 0)
		}
	}
}

func (s *smtc) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseLocked()
}

func (s *smtc) applyMetaLocked() {
	if !s.attached || s.updater == nil || s.music == nil {
		return
	}
	if s.track.Title == "" {
		_ = vcall(s.updater, updaterClearAll)
		_ = vcall(s.updater, updaterPutType, typeMusic)
		_ = vcall(s.updater, updaterUpdate)
		s.art = ""
		return
	}
	putString(s.music, musicPutTitle, s.track.Title)
	putString(s.music, musicPutArtist, s.track.Artist)
	if s.music2 != nil {
		putString(s.music2, music2PutAlbum, s.track.Album)
	}
	if s.track.ArtURL != s.art {
		s.art = s.track.ArtURL
		s.putThumbnailLocked(s.art)
	}
	_ = vcall(s.updater, updaterUpdate)
}

func (s *smtc) putThumbnailLocked(address string) {
	if s.updater == nil {
		return
	}
	if address == "" || s.uris == nil || s.streams == nil {
		_ = vcall(s.updater, updaterPutThumbnail, 0)
		return
	}
	if strings.HasPrefix(address, "//") {
		address = "https:" + address
	}
	if u, err := url.Parse(address); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		_ = vcall(s.updater, updaterPutThumbnail, 0)
		return
	}
	h, err := hstring(address)
	if err != nil {
		return
	}
	defer deleteHString(h)
	var uri, stream unsafe.Pointer
	if err := vcall(s.uris, uriCreate, h, uintptr(unsafe.Pointer(&uri))); err != nil || uri == nil {
		return
	}
	defer release(&uri)
	if err := vcall(s.streams, streamsCreateByURI, uintptr(unsafe.Pointer(uri)), uintptr(unsafe.Pointer(&stream))); err != nil || stream == nil {
		return
	}
	defer release(&stream)
	_ = vcall(s.updater, updaterPutThumbnail, uintptr(unsafe.Pointer(stream)))
}

func (s *smtc) applyStatusLocked() {
	if !s.attached || s.controls == nil {
		return
	}
	status := uintptr(statusStopped)
	switch s.status {
	case Playing:
		status = statusPlaying
	case Paused:
		status = statusPaused
	}
	_ = vcall(s.controls, controlsPutStatus, status)
}

func (s *smtc) applyNavLocked() {
	if !s.attached || s.controls == nil {
		return
	}
	put := func(idx int, on bool) {
		if on {
			_ = vcall(s.controls, idx, 1)
		} else {
			_ = vcall(s.controls, idx, 0)
		}
	}
	put(controlsPutPrevious, s.prev)
	put(controlsPutNext, s.next)
}

func (s *smtc) applyLoopLocked() {
	if !s.attached || s.controls2 == nil {
		return
	}
	mode := uintptr(0)
	switch s.loop {
	case "Track":
		mode = 1
	case "Playlist":
		mode = 2
	}
	_ = vcall(s.controls2, controls2PutAutoRepeat, mode)
	if s.shuffle {
		_ = vcall(s.controls2, controls2PutShuffle, 1)
	} else {
		_ = vcall(s.controls2, controls2PutShuffle, 0)
	}
}

func (s *smtc) applyTimelineLocked() {
	if !s.attached || s.controls2 == nil || s.timeline == nil || s.track.LengthUs <= 0 {
		return
	}
	pos := s.posUs
	if pos < 0 {
		pos = 0
	}
	if pos > s.track.LengthUs {
		pos = s.track.LengthUs
	}
	end := uintptr(uint64(s.track.LengthUs) * 10) // TimeSpan: 100ns
	at := uintptr(uint64(pos) * 10)
	_ = vcall(s.timeline, timelinePutStart, 0)
	_ = vcall(s.timeline, timelinePutMinSeek, 0)
	_ = vcall(s.timeline, timelinePutMaxSeek, end)
	_ = vcall(s.timeline, timelinePutEnd, end)
	_ = vcall(s.timeline, timelinePutPosition, at)
	_ = vcall(s.controls2, controls2UpdateTimeline, uintptr(unsafe.Pointer(s.timeline)))
}

func (s *smtc) onButton(button int32) {
	switch button {
	case buttonPlay:
		if s.cb.Play != nil {
			s.cb.Play()
		}
	case buttonPause:
		if s.cb.Pause != nil {
			s.cb.Pause()
		}
	case buttonStop:
		if s.cb.Stop != nil {
			s.cb.Stop()
		}
	case buttonNext:
		if s.cb.Next != nil {
			s.cb.Next()
		}
	case buttonPrevious:
		if s.cb.Previous != nil {
			s.cb.Previous()
		}
	}
}

func (s *smtc) onSeek(us int64) {
	if us < 0 {
		return
	}
	if s.cb.Seek != nil {
		// SMTC 的 RequestedPlaybackPosition 是 100ns 单位。
		s.cb.Seek(us / 10)
	}
}
