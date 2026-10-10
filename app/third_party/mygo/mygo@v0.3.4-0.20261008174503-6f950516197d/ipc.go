package mygo

import (
	"context"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/egoist/mygo/internal/tsgen"
)

// jsonOptions are used for every value crossing the IPC boundary.
var jsonOptions = json.JoinOptions(
	jsonv1.FormatDurationAsNano(true),
	jsontext.EscapeForJS(true),
)

type service struct {
	name     string
	typ      reflect.Type
	methods  []*method
	internal bool
}

type method struct {
	service  string
	name     string
	fn       reflect.Value
	ctx      bool
	params   []reflect.Type
	variadic bool
	// chans marks the parameters that are channels (*Channel[T]), if any
	// is (streams).
	chans   []bool
	streams bool
	result  reflect.Type
	hasErr  bool
	pc      uintptr
}

type eventInfo struct {
	name string
	typ  reflect.Type
	pc   uintptr
	file string
	line int
}

var ipc struct {
	sync.RWMutex
	services []*service
	methods  map[string]*method
	events   []*eventInfo
}

var (
	contextType = reflect.TypeFor[context.Context]()
	errorType   = reflect.TypeFor[error]()
)

// Bind exposes the exported methods of each service to the frontend. The
// generated TypeScript client (see `mygo generate`) turns
//
//	type Greeter struct{}
//
//	// Greet returns a greeting.
//	func (Greeter) Greet(name string) (string, error)
//
// into a typed `Greeter.greet(name: string): Promise<string>` function.
//
// Arguments and results are encoded as JSON. A method may take a
// context.Context as its first parameter: it is canceled when the calling
// page navigates away or its window closes, and CallerWindow returns the
// calling window from it. A method may return nothing, a value, an error, or
// a value and an error; a non-nil error rejects the promise on the
// JavaScript side. Calls run on their own goroutine.
//
// The service is named after its type; Bind panics if the name is taken or a
// method uses a type that cannot be encoded as JSON (channels, functions).
func Bind(services ...any) {
	for _, s := range services {
		if err := bind("", s, false); err != nil {
			panic(err)
		}
	}
}

// BindAs is Bind with an explicit service name, for services whose type
// names collide.
func BindAs(name string, service any) {
	if err := bind(name, service, false); err != nil {
		panic(err)
	}
}

func bind(name string, svc any, internal bool) error {
	v := reflect.ValueOf(svc)
	if !v.IsValid() {
		return errors.New("mygo: cannot bind nil")
	}
	t := v.Type()
	base := t
	if base.Kind() == reflect.Pointer {
		base = base.Elem()
	}
	if name == "" {
		name = base.Name()
	}
	if name == "" {
		return fmt.Errorf("mygo: cannot bind %s: type has no name, use BindAs", t)
	}
	if !internal && !tsgen.IsIdentifier(name) {
		return fmt.Errorf("mygo: service name %q is not a valid identifier", name)
	}
	s := &service{name: name, typ: t, internal: internal}
	for i := range t.NumMethod() {
		m, err := newMethod(name, t.Method(i), v)
		if err != nil {
			return err
		}
		s.methods = append(s.methods, m)
	}
	if len(s.methods) == 0 {
		return fmt.Errorf("mygo: %s has no exported methods to bind", t)
	}

	ipc.Lock()
	defer ipc.Unlock()
	for _, x := range ipc.services {
		if x.name == name {
			return fmt.Errorf("mygo: a service named %s is already bound", name)
		}
	}
	if ipc.methods == nil {
		ipc.methods = map[string]*method{}
	}
	ipc.services = append(ipc.services, s)
	for _, m := range s.methods {
		ipc.methods[name+"."+m.name] = m
	}
	return nil
}

func newMethod(service string, m reflect.Method, recv reflect.Value) (*method, error) {
	ft := m.Type
	mi := &method{service: service, name: m.Name, fn: recv.Method(m.Index), variadic: ft.IsVariadic()}
	where := service + "." + m.Name
	start := 1 // skip the receiver
	if ft.NumIn() > 1 && ft.In(1) == contextType {
		mi.ctx = true
		start = 2
	}
	for i := start; i < ft.NumIn(); i++ {
		p := ft.In(i)
		isChan := p.Implements(channelParamType)
		t := p
		if isChan {
			t = reflect.Zero(p).Interface().(channelParam).valueType()
		}
		if err := validateType(t); err != nil {
			return nil, fmt.Errorf("mygo: %s: parameter %d: %w", where, i-start+1, err)
		}
		mi.params = append(mi.params, p)
		mi.chans = append(mi.chans, isChan)
		mi.streams = mi.streams || isChan
	}
	switch ft.NumOut() {
	case 0:
	case 1:
		if ft.Out(0) == errorType {
			mi.hasErr = true
		} else {
			mi.result = ft.Out(0)
		}
	case 2:
		if ft.Out(1) != errorType {
			return nil, fmt.Errorf("mygo: %s: the second result must be an error", where)
		}
		mi.result, mi.hasErr = ft.Out(0), true
	default:
		return nil, fmt.Errorf("mygo: %s: methods may return at most a value and an error", where)
	}
	if mi.result != nil {
		if err := validateType(mi.result); err != nil {
			return nil, fmt.Errorf("mygo: %s: result: %w", where, err)
		}
	}
	mi.pc = methodPC(recv.Type(), m.Name)
	return mi, nil
}

// validateType reports an error if values of type t cannot cross the IPC
// boundary.
func validateType(t reflect.Type) error {
	if hasChannel(t, map[reflect.Type]bool{}) {
		return fmt.Errorf("type %s holds a Channel, which can only be a parameter", t)
	}
	return tsgen.Validate(t)
}

// methodPC returns the entry of the method's own code rather than the
// wrapper Go generates for value receivers called through a pointer.
func methodPC(t reflect.Type, name string) uintptr {
	if t.Kind() == reflect.Pointer {
		if m, ok := t.Elem().MethodByName(name); ok {
			return m.Func.Pointer()
		}
	}
	if m, ok := t.MethodByName(name); ok {
		return m.Func.Pointer()
	}
	return 0
}

func lookupMethod(name string) *method {
	ipc.RLock()
	defer ipc.RUnlock()
	return ipc.methods[name]
}

// call calls the method for the page of w whose context is page and whose
// token is token.
func (m *method) call(w *Window, page context.Context, token string, args []rawValue) (result any, err error) {
	ctx, cancel := page, context.CancelFunc(nil)
	var chans []*channel
	if m.streams {
		// The page closing a channel cancels the call. The channels close
		// when it returns, before its result is sent.
		ctx, cancel = context.WithCancel(page)
		defer func() {
			for _, c := range chans {
				c.close(closedByGo)
			}
			cancel()
		}()
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
			log.Printf("mygo: panic in %s.%s: %v\n%s", m.service, m.name, r, debug.Stack())
		}
	}()
	in := make([]reflect.Value, 0, len(m.params)+1)
	if m.ctx {
		in = append(in, reflect.ValueOf(ctx))
	}
	last := len(m.params) - 1
	for i, t := range m.params {
		if m.chans[i] {
			// The page passes the id of its channel.
			var id int64
			if i < len(args) {
				_ = json.Unmarshal(args[i], &id)
			}
			if id <= 0 {
				return nil, fmt.Errorf("invalid argument %d: not a Channel", i+1)
			}
			c := w.newChannel(page, ctx, cancel, id, token)
			chans = append(chans, c)
			v := reflect.New(t.Elem())
			v.Interface().(channelParam).init(c)
			in = append(in, v)
			continue
		}
		if m.variadic && i == last {
			for j := i; j < len(args); j++ {
				v, err := decodeArg(args[j], t.Elem(), j)
				if err != nil {
					return nil, err
				}
				in = append(in, v)
			}
			break
		}
		if i >= len(args) {
			in = append(in, reflect.Zero(t))
			continue
		}
		v, err := decodeArg(args[i], t, i)
		if err != nil {
			return nil, err
		}
		in = append(in, v)
	}
	out := m.fn.Call(in)
	if m.hasErr {
		if e := out[len(out)-1]; !e.IsNil() {
			return nil, e.Interface().(error)
		}
	}
	if m.result != nil {
		return out[0].Interface(), nil
	}
	return nil, nil
}

func decodeArg(raw rawValue, t reflect.Type, i int) (reflect.Value, error) {
	v := reflect.New(t)
	if err := json.Unmarshal(raw, v.Interface(), jsonOptions); err != nil {
		return reflect.Value{}, fmt.Errorf("invalid argument %d: %w", i+1, err)
	}
	return v.Elem(), nil
}

// rawValue is a JSON value that shares the bytes it was decoded from,
// which must outlive it unchanged: arguments, which can be large, go from
// the message to their parameters without being copied.
type rawValue []byte

// UnmarshalJSONFrom keeps the value in place. json.Unmarshal decodes from
// the slice it is given, which it never modifies.
func (v *rawValue) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	*v = rawValue(raw)
	return err
}

// stringBytes returns the bytes of s without copying them. They must not
// be modified.
func stringBytes(s string) []byte { return unsafe.Slice(unsafe.StringData(s), len(s)) }

// handleCall decodes and runs a call from the page on the current
// goroutine, then queues the reply. Calls from untrusted pages are
// rejected.
func handleCall(w *Window, ctx context.Context, raw string, trusted bool) {
	var m struct {
		ID int64      `json:"id"`
		K  string     `json:"k"`
		M  string     `json:"m"`
		A  []rawValue `json:"a"` // shares raw
	}
	if err := json.Unmarshal(stringBytes(raw), &m); err != nil {
		log.Printf("mygo: malformed call from window %d: %v", w.id, err)
		return
	}
	var result any
	var err error
	if !trusted {
		err = fmt.Errorf("this page is not allowed to call Go methods (see PageOptions.TrustedOrigins)")
	} else if mi := lookupMethod(m.M); mi == nil {
		err = notBound(m.M)
	} else {
		result, err = mi.call(w, ctx, m.K, m.A)
	}
	w.enqueue(encodeReply(m.ID, m.K, result, err), false)
}

// message is a message for the page: a JSON object, in parts that are
// joined when it is sent, so that an encoded value, which can be large, is
// not copied into it.
type message struct {
	head, value, tail []byte
}

func (m message) len() int { return len(m.head) + len(m.value) + len(m.tail) }

func (m message) appendTo(b []byte) []byte {
	return append(append(append(b, m.head...), m.value...), m.tail...)
}

var closeBrace = []byte("}")

func encodeReply(id int64, token string, result any, err error) message {
	b := make([]byte, 0, 96)
	b = append(b, `{"t":"reply","id":`...)
	b = strconv.AppendInt(b, id, 10)
	b = append(b, `,"k":`...)
	b, _ = jsontext.AppendQuote(b, token)
	if err == nil {
		v, merr := json.Marshal(result, jsonOptions)
		if merr == nil {
			return message{append(b, `,"ok":true,"v":`...), v, closeBrace}
		}
		err = fmt.Errorf("cannot encode result: %w", merr)
	}
	b = append(b, `,"ok":false,"e":`...)
	b, _ = jsontext.AppendQuote(b, err.Error())
	return message{head: append(b, '}')}
}

type callerKey struct{}

// CallerWindow returns the window whose page called a bound method, given
// the context.Context passed to that method. It returns nil for other
// contexts.
func CallerWindow(ctx context.Context) *Window {
	w, _ := ctx.Value(callerKey{}).(*Window)
	return w
}

// Event is a typed event the Go side sends to pages. Declare events at
// package level so `mygo generate` includes them in the TypeScript client:
//
//	var Progress = mygo.NewEvent[ProgressInfo]("progress")
//
//	Progress.Emit(win, ProgressInfo{Done: 1, Total: 3})
//
// and subscribe in the frontend with `events.progress.on(p => ...)`.
type Event[T any] struct {
	name string
}

// NewEvent declares an event with payload type T. Names must be unique;
// those starting with "mygo:" are reserved.
func NewEvent[T any](name string) *Event[T] {
	if strings.HasPrefix(name, "mygo:") {
		panic(fmt.Sprintf("mygo: event %q: names starting with \"mygo:\" are reserved", name))
	}
	t := reflect.TypeFor[T]()
	if err := validateType(t); err != nil {
		panic(fmt.Sprintf("mygo: event %q: %v", name, err))
	}
	pc, file, line, _ := runtime.Caller(1)
	ipc.Lock()
	defer ipc.Unlock()
	for _, e := range ipc.events {
		if e.name == name {
			panic(fmt.Sprintf("mygo: event %q is already declared", name))
		}
	}
	ipc.events = append(ipc.events, &eventInfo{name: name, typ: t, pc: pc, file: file, line: line})
	return &Event[T]{name: name}
}

// Name returns the event name.
func (e *Event[T]) Name() string { return e.name }

// Emit sends the event to the page shown in w. Events sent before the
// page's DOM is ready are delivered once it is.
func (e *Event[T]) Emit(w *Window, payload T) error {
	msg, err := encodeEvent(e.name, payload)
	if err != nil {
		return err
	}
	if w.IsDestroyed() {
		return errDestroyed
	}
	w.enqueue(msg, true)
	return nil
}

// Broadcast sends the event to every window.
func (e *Event[T]) Broadcast(payload T) error {
	msg, err := encodeEvent(e.name, payload)
	if err != nil {
		return err
	}
	for _, w := range Windows() {
		w.enqueue(msg, true)
	}
	return nil
}

func encodeEvent(name string, payload any) (message, error) {
	p, err := json.Marshal(payload, jsonOptions)
	if err != nil {
		return message{}, fmt.Errorf("mygo: cannot encode event %q: %w", name, err)
	}
	b := make([]byte, 0, len(name)+32)
	b = append(b, `{"t":"event","n":`...)
	b, _ = jsontext.AppendQuote(b, name)
	b = append(b, `,"p":`...)
	return message{b, p, closeBrace}, nil
}

// windowControls backs window.mygo.window in the page.
type windowControls struct{}

func (windowControls) Minimize(ctx context.Context)         { CallerWindow(ctx).Minimize() }
func (windowControls) Maximize(ctx context.Context)         { CallerWindow(ctx).Maximize() }
func (windowControls) Unmaximize(ctx context.Context)       { CallerWindow(ctx).Unmaximize() }
func (windowControls) ToggleMaximize(ctx context.Context)   { CallerWindow(ctx).ToggleMaximize() }
func (windowControls) IsMaximized(ctx context.Context) bool { return CallerWindow(ctx).IsMaximized() }
func (windowControls) ToggleFullScreen(ctx context.Context) { CallerWindow(ctx).ToggleFullScreen() }
func (windowControls) Close(ctx context.Context)            { CallerWindow(ctx).Close() }
func (windowControls) SetTitle(ctx context.Context, title string) {
	CallerWindow(ctx).SetTitle(title)
}

func init() {
	if err := bind("mygo:window", windowControls{}, true); err != nil {
		panic(err)
	}
}
