package websocket

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// serverConn is the server's side of a hijacked connection.
type serverConn struct {
	net.Conn
	br *bufio.Reader
}

func (c serverConn) Read(p []byte) (int, error) { return c.br.Read(p) }

// testServer accepts connections and runs handle on the server's side.
func testServer(t *testing.T, handle func(r *http.Request, c *conn)) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reject" || r.Header.Get("Sec-WebSocket-Version") != "13" || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "not a WebSocket handshake", http.StatusBadRequest)
			return
		}
		nc, brw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		proto := ""
		if p := r.Header.Get("Sec-WebSocket-Protocol"); p != "" {
			proto = "Sec-WebSocket-Protocol: " + strings.Split(p, ",")[0] + "\r\n"
		}
		brw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + acceptKey(r.Header.Get("Sec-WebSocket-Key")) + "\r\n" + proto + "\r\n")
		brw.Flush()
		c := newConn(serverConn{nc, brw.Reader}, brw.Reader, true, 1<<20)
		defer c.terminate()
		handle(r, c)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// echo sends messages back; "close" makes it close the connection, "ping"
// pings, "split" sends a fragmented message.
func echo(r *http.Request, c *conn) {
	for {
		op, data, err := c.readMessage()
		if err != nil {
			return
		}
		switch {
		case op == opClose:
			code, _, _ := parseClose(data)
			c.writeClose(code, "")
			return
		case string(data) == "close":
			c.writeClose(4000, "bye")
		case string(data) == "ping":
			c.writeFrame(opPing, []byte("p"))
			c.writeFrame(opText, []byte("pinged"))
		case string(data) == "split":
			c.rwc.Write([]byte{opText, 2, 'a', 'b'})
			c.rwc.Write([]byte{0x80 | opContinuation, 1, 'c'})
		case string(data) == "origin":
			c.writeFrame(opText, []byte(r.Header.Get("Origin")))
		default:
			c.writeFrame(op, data)
		}
	}
}

// open connects the page of window 1 and returns its events.
func open(t *testing.T, s *service, opts connectOptions) (<-chan event, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan event, 100)
	go func() {
		s.connect(ctx, 1, opts, func(e event) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			events <- e
			return nil
		})
		close(events)
	}()
	t.Cleanup(cancel)
	return events, cancel
}

func next(t *testing.T, events <-chan event) event {
	t.Helper()
	select {
	case e := <-events:
		return e
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
		return event{}
	}
}

func TestWebSocket(t *testing.T) {
	srv := testServer(t, echo)
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	s := New(Options{}).Service.(*service)

	events, _ := open(t, s, connectOptions{ID: "a", URL: url, Protocols: []string{"chat", "v2"}, Headers: [][2]string{{"origin", "https://example.com"}}})
	if e := next(t, events); e.Type != "open" || e.Protocol != "chat" {
		t.Fatalf("open: %+v", e)
	}
	// Sent in the order of their sequence numbers, whatever the order of
	// the calls.
	errs := make(chan error, 3)
	go func() { errs <- s.send(1, "a", 3, message{Type: "binary", Data: []byte{1, 2}}) }()
	go func() { errs <- s.send(1, "a", 2, message{Type: "text", Text: "two"}) }()
	time.Sleep(50 * time.Millisecond)
	go func() { errs <- s.send(1, "a", 1, message{Type: "text", Text: "one"}) }()
	for range 3 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{"text:one", "text:two", "binary:\x01\x02"} {
		e := next(t, events)
		if got := e.Type + ":" + e.Text + string(e.Data); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	for i, m := range []string{"ping", "split", "origin"} {
		if err := s.send(1, "a", int64(4+i), message{Type: "text", Text: m}); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{"pinged", "abc", "https://example.com"} {
		if e := next(t, events); e.Text != want {
			t.Errorf("got %+v, want %q", e, want)
		}
	}

	// The page closes the connection.
	if err := s.send(1, "a", 7, message{Type: "close", Code: 1000, Reason: "done"}); err != nil {
		t.Fatal(err)
	}
	if e := next(t, events); e.Type != "close" || e.Code != 1000 || !e.Clean {
		t.Errorf("close: %+v", e)
	}
	if err := s.send(1, "a", 8, message{Type: "text", Text: "late"}); err == nil {
		t.Error("sent on a closed connection")
	}

	// The server closes it.
	events, _ = open(t, s, connectOptions{ID: "b", URL: url})
	next(t, events)
	s.send(1, "b", 1, message{Type: "text", Text: "close"})
	if e := next(t, events); e.Type != "close" || e.Code != 4000 || e.Reason != "bye" || !e.Clean {
		t.Errorf("closed by the server: %+v", e)
	}

	// Failed handshakes.
	events, _ = open(t, s, connectOptions{ID: "c", URL: srv.URL + "/reject", Headers: [][2]string{{"sec-websocket-version", "8"}}})
	if e := next(t, events); e.Type != "close" || e.Code != 1006 || !strings.Contains(e.Error, "400") {
		t.Errorf("rejected handshake: %+v", e)
	}
	deny := New(Options{Allow: func(*http.Request) bool { return false }}).Service.(*service)
	events, _ = open(t, deny, connectOptions{ID: "d", URL: url})
	if e := next(t, events); !strings.Contains(e.Error, "does not allow") {
		t.Errorf("Allow: %+v", e)
	}
}

func TestWebSocketPageGoesAway(t *testing.T) {
	closed := make(chan int, 1)
	srv := testServer(t, func(r *http.Request, c *conn) {
		_, data, _ := c.readMessage()
		code, _, _ := parseClose(data)
		closed <- code
	})
	s := New(Options{}).Service.(*service)
	events, cancel := open(t, s, connectOptions{ID: "a", URL: "ws" + strings.TrimPrefix(srv.URL, "http")})
	next(t, events)
	cancel()
	select {
	case code := <-closed:
		if code != closeGoingAway {
			t.Errorf("closed with %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the connection was not closed")
	}
	for range events {
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.conns) != 0 {
		t.Errorf("connections left: %v", s.conns)
	}
}

func TestWebSocketCloseTimeout(t *testing.T) {
	srv := testServer(t, func(r *http.Request, c *conn) {
		time.Sleep(2 * time.Second) // never answers the close
	})
	s := New(Options{CloseTimeout: 100 * time.Millisecond}).Service.(*service)
	events, _ := open(t, s, connectOptions{ID: "a", URL: "ws" + strings.TrimPrefix(srv.URL, "http")})
	next(t, events)
	s.send(1, "a", 1, message{Type: "close", Code: 1000})
	if e := next(t, events); e.Code != 1006 || e.Clean || e.Error != "" {
		t.Errorf("close without an answer: %+v", e)
	}
}
