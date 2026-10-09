//go:build linux

package mediactl

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// MPRIS 的固定名字与对象路径（规范规定，不能改）。
const (
	mprisBusName = "org.mpris.MediaPlayer2.bili_fm"
	mprisPath    = dbus.ObjectPath("/org/mpris/MediaPlayer2")
	mprisRoot    = "org.mpris.MediaPlayer2"
	mprisPlayer  = "org.mpris.MediaPlayer2.Player"
)

// mpris 是 Linux 上的实现（对界面暴露的 Controller 接口）。
type mpris struct {
	conn  *dbus.Conn
	props *prop.Properties
	cb    Callbacks

	mu       sync.Mutex
	trackID  dbus.ObjectPath
	lengthUs int64
	posUs    int64
}

// New 连接会话总线并注册 MPRIS 服务。任何一步失败都返回 no-op，绝不影响播放。
func New(cb Callbacks) Controller {
	conn, err := dbus.SessionBus()
	if err != nil {
		return noop{}
	}
	reply, err := conn.RequestName(mprisBusName, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		// 已经有一个实例在跑（或会话总线不可用）：静默降级。
		return noop{}
	}
	m := &mpris{conn: conn, cb: cb, trackID: dbus.ObjectPath("/org/bilifm/track/none")}

	spec := prop.Map{
		mprisRoot: {
			"Identity":            {Value: "bili-FM", Writable: false, Emit: prop.EmitTrue},
			"DesktopEntry":        {Value: "bili-fm", Writable: false, Emit: prop.EmitTrue},
			"CanQuit":             {Value: false, Writable: false, Emit: prop.EmitFalse},
			"CanRaise":            {Value: false, Writable: false, Emit: prop.EmitFalse},
			"HasTrackList":        {Value: false, Writable: false, Emit: prop.EmitFalse},
			"SupportedUriSchemes": {Value: []string{}, Writable: false, Emit: prop.EmitFalse},
			"SupportedMimeTypes":  {Value: []string{}, Writable: false, Emit: prop.EmitFalse},
		},
		mprisPlayer: {
			"PlaybackStatus": {Value: string(Stopped), Writable: false, Emit: prop.EmitTrue},
			"LoopStatus":     {Value: "None", Writable: false, Emit: prop.EmitFalse},
			"Rate": {Value: 1.0, Writable: true, Emit: prop.EmitFalse, Callback: func(ch *prop.Change) *dbus.Error {
				if v, ok := ch.Value.(float64); ok && m.cb.SetRate != nil {
					m.cb.SetRate(v)
				}
				return nil
			}},
			"Shuffle":  {Value: false, Writable: false, Emit: prop.EmitFalse},
			"Metadata": {Value: map[string]dbus.Variant{}, Writable: false, Emit: prop.EmitTrue},
			"Volume": {Value: 1.0, Writable: true, Emit: prop.EmitFalse, Callback: func(ch *prop.Change) *dbus.Error {
				if v, ok := ch.Value.(float64); ok && m.cb.SetVolume != nil {
					m.cb.SetVolume(v)
				}
				return nil
			}},
			// Position 按规范不发 PropertiesChanged，客户端自己轮询。
			"Position":      {Value: int64(0), Writable: false, Emit: prop.EmitFalse},
			"MinimumRate":   {Value: 0.5, Writable: false, Emit: prop.EmitFalse},
			"MaximumRate":   {Value: 3.0, Writable: false, Emit: prop.EmitFalse},
			"CanGoNext":     {Value: false, Writable: false, Emit: prop.EmitTrue},
			"CanGoPrevious": {Value: false, Writable: false, Emit: prop.EmitTrue},
			"CanPlay":       {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanPause":      {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanSeek":       {Value: true, Writable: false, Emit: prop.EmitTrue},
			"CanControl":    {Value: true, Writable: false, Emit: prop.EmitFalse},
		},
	}
	p, err := prop.Export(conn, mprisPath, spec)
	if err != nil {
		_, _ = conn.ReleaseName(mprisBusName)
		return noop{}
	}
	m.props = p

	player := &playerIface{m: m}
	root := &rootIface{m: m}
	if err := conn.Export(player, mprisPath, mprisPlayer); err != nil {
		_, _ = conn.ReleaseName(mprisBusName)
		return noop{}
	}
	if err := conn.Export(root, mprisPath, mprisRoot); err != nil {
		_, _ = conn.ReleaseName(mprisBusName)
		return noop{}
	}
	node := introspectNode(player, root)
	_ = conn.Export(introspect.NewIntrospectable(node), mprisPath, "org.freedesktop.DBus.Introspectable")
	return m
}

// ---------------------------------------------------------------- 属性更新

func (m *mpris) SetTrack(t Track) {
	m.mu.Lock()
	m.lengthUs = t.LengthUs
	m.trackID = dbus.ObjectPath("/org/bilifm/track/" + trackSlug(t.Title))
	m.mu.Unlock()

	md := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(m.trackID),
		"xesam:title":   dbus.MakeVariant(t.Title),
	}
	if t.Artist != "" {
		md["xesam:artist"] = dbus.MakeVariant([]string{t.Artist})
	}
	if t.Album != "" {
		md["xesam:album"] = dbus.MakeVariant(t.Album)
	}
	if t.LengthUs > 0 {
		md["mpris:length"] = dbus.MakeVariant(t.LengthUs)
	}
	if t.ArtURL != "" {
		md["mpris:artUrl"] = dbus.MakeVariant(t.ArtURL)
	}
	m.props.SetMust(mprisPlayer, "Metadata", md)
}

func (m *mpris) SetStatus(s Status) {
	m.props.SetMust(mprisPlayer, "PlaybackStatus", string(s))
}

func (m *mpris) SetPosition(us int64) {
	if us < 0 {
		us = 0
	}
	m.mu.Lock()
	m.posUs = us
	m.mu.Unlock()
	m.props.SetMust(mprisPlayer, "Position", us)
}

func (m *mpris) SetVolume(v float64) {
	m.props.SetMust(mprisPlayer, "Volume", v)
}

func (m *mpris) SetRate(v float64) {
	m.props.SetMust(mprisPlayer, "Rate", v)
}

func (m *mpris) SetNavigable(next, prev bool) {
	m.props.SetMust(mprisPlayer, "CanGoNext", next)
	m.props.SetMust(mprisPlayer, "CanGoPrevious", prev)
}

func (m *mpris) Close() {
	if m.conn != nil {
		_, _ = m.conn.ReleaseName(mprisBusName)
	}
}

func (m *mpris) seekTo(us int64) {
	if us < 0 {
		us = 0
	}
	if m.cb.Seek != nil {
		m.cb.Seek(us)
	}
	_ = m.conn.Emit(mprisPath, mprisPlayer+".Seeked", us)
}

func (m *mpris) relativeSeek(offset int64) {
	m.mu.Lock()
	pos := m.posUs + offset
	m.mu.Unlock()
	m.seekTo(pos)
}

// ---------------------------------------------------------------- D-Bus 接口

// playerIface 承接 org.mpris.MediaPlayer2.Player 的方法，避开与 Controller
// 接口上的 SetPosition 重名。
type playerIface struct{ m *mpris }

func (p *playerIface) Play() *dbus.Error {
	p.m.call(p.m.cb.Play)
	return nil
}

func (p *playerIface) Pause() *dbus.Error {
	p.m.call(p.m.cb.Pause)
	return nil
}

func (p *playerIface) PlayPause() *dbus.Error {
	p.m.call(p.m.cb.PlayPause)
	return nil
}

func (p *playerIface) Stop() *dbus.Error {
	p.m.call(p.m.cb.Stop)
	return nil
}

func (p *playerIface) Next() *dbus.Error {
	p.m.call(p.m.cb.Next)
	return nil
}

func (p *playerIface) Previous() *dbus.Error {
	p.m.call(p.m.cb.Previous)
	return nil
}

// Seek 是相对跳转（微秒）。
func (p *playerIface) Seek(offset int64) *dbus.Error {
	p.m.relativeSeek(offset)
	return nil
}

// SetPosition 是绝对跳转（微秒）。
func (p *playerIface) SetPosition(trackID dbus.ObjectPath, position int64) *dbus.Error {
	p.m.seekTo(position)
	return nil
}

func (p *playerIface) OpenUri(uri string) *dbus.Error { return nil }

func (m *mpris) call(fn func()) {
	if fn != nil {
		fn()
	}
}

// rootIface 承接 org.mpris.MediaPlayer2 的方法。
type rootIface struct{ m *mpris }

func (r *rootIface) Raise() *dbus.Error { return nil }
func (r *rootIface) Quit() *dbus.Error  { return nil }

// ---------------------------------------------------------------- introspection

func introspectNode(player *playerIface, root *rootIface) *introspect.Node {
	return &introspect.Node{
		Name: string(mprisPath),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:    mprisRoot,
				Methods: introspect.Methods(root),
				Properties: []introspect.Property{
					{Name: "Identity", Type: "s", Access: "read"},
					{Name: "DesktopEntry", Type: "s", Access: "read"},
					{Name: "CanQuit", Type: "b", Access: "read"},
					{Name: "CanRaise", Type: "b", Access: "read"},
					{Name: "HasTrackList", Type: "b", Access: "read"},
					{Name: "SupportedUriSchemes", Type: "as", Access: "read"},
					{Name: "SupportedMimeTypes", Type: "as", Access: "read"},
				},
			},
			{
				Name:    mprisPlayer,
				Methods: introspect.Methods(player),
				Properties: []introspect.Property{
					{Name: "PlaybackStatus", Type: "s", Access: "read"},
					{Name: "LoopStatus", Type: "s", Access: "read"},
					{Name: "Rate", Type: "d", Access: "readwrite"},
					{Name: "Shuffle", Type: "b", Access: "read"},
					{Name: "Metadata", Type: "a{sv}", Access: "read"},
					{Name: "Volume", Type: "d", Access: "readwrite"},
					{Name: "Position", Type: "x", Access: "read"},
					{Name: "MinimumRate", Type: "d", Access: "read"},
					{Name: "MaximumRate", Type: "d", Access: "read"},
					{Name: "CanGoNext", Type: "b", Access: "read"},
					{Name: "CanGoPrevious", Type: "b", Access: "read"},
					{Name: "CanPlay", Type: "b", Access: "read"},
					{Name: "CanPause", Type: "b", Access: "read"},
					{Name: "CanSeek", Type: "b", Access: "read"},
					{Name: "CanControl", Type: "b", Access: "read"},
				},
				Signals: []introspect.Signal{
					{Name: "Seeked", Args: []introspect.Arg{{Name: "Position", Type: "x"}}},
				},
			},
		},
	}
}

// trackSlug 把标题变成 MPRIS trackid 里合法的片段。
func trackSlug(title string) string {
	if title == "" {
		return "none"
	}
	var b strings.Builder
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 48 {
			break
		}
	}
	if b.Len() == 0 {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return b.String()
}
