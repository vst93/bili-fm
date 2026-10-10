package fetch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func run(t *testing.T, s *service, req request) ([]part, error) {
	t.Helper()
	var parts []part
	err := s.fetch(context.Background(), req, func(p part) error {
		p.Chunk = append([]byte(nil), p.Chunk...)
		parts = append(parts, p)
		return nil
	})
	return parts, err
}

func header(p part, name string) string {
	for _, h := range p.Headers {
		if h[0] == name {
			return h[1]
		}
	}
	return ""
}

func body(parts []part) string {
	var b strings.Builder
	for _, p := range parts[1:] {
		b.Write(p.Chunk)
	}
	return b.String()
}

func TestFetch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.Header().Add("X-Multi", "a")
		w.Header().Add("X-Multi", "b")
		w.Header().Set("X-Origin", r.Header.Get("Origin"))
		w.Header().Set("X-Host", r.Host)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, r.Method+" "+string(b))
	})
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strings.Repeat("x", 3*chunkSize/2))
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/echo", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	s := New(Options{}).Service.(*service)

	parts, err := run(t, s, request{
		URL:     srv.URL + "/echo",
		Method:  "POST",
		Headers: [][2]string{{"origin", "https://example.com"}, {"host", "api.test"}, {"content-length", "999"}},
		Body:    []byte("hi"),
	})
	if err != nil {
		t.Fatal(err)
	}
	head := parts[0]
	if head.Status != 201 || head.StatusText != "Created" || head.Redirected {
		t.Errorf("head = %+v", head)
	}
	if header(head, "x-origin") != "https://example.com" || header(head, "x-host") != "api.test" {
		t.Errorf("request headers were not sent: %v", head.Headers)
	}
	n := 0
	for _, h := range head.Headers {
		if h[0] == "x-multi" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("repeated header sent %d times: %v", n, head.Headers)
	}
	if got := body(parts); got != "POST hi" {
		t.Errorf("body = %q", got)
	}

	parts, err = run(t, s, request{URL: srv.URL + "/big", Method: "GET"})
	if err != nil || len(parts) < 3 || len(body(parts)) != 3*chunkSize/2 {
		t.Errorf("big body: %d parts, %v", len(parts), err)
	}
	for _, p := range parts[1:] {
		if len(p.Chunk) > chunkSize {
			t.Errorf("chunk of %d bytes", len(p.Chunk))
		}
	}

	parts, err = run(t, s, request{URL: srv.URL + "/redirect", Method: "GET"})
	if err != nil || parts[0].Status != 201 || !parts[0].Redirected || parts[0].URL != srv.URL+"/echo" {
		t.Errorf("followed redirect: %+v, %v", parts, err)
	}
	parts, err = run(t, s, request{URL: srv.URL + "/redirect", Method: "GET", Redirect: "manual"})
	if err != nil || parts[0].Status != 302 || header(parts[0], "location") != "/echo" {
		t.Errorf("manual redirect: %+v, %v", parts, err)
	}
	if _, err = run(t, s, request{URL: srv.URL + "/redirect", Method: "GET", Redirect: "error"}); err == nil {
		t.Error("redirect mode error followed a redirect")
	}
	if _, err = run(t, s, request{URL: "file:///etc/passwd", Method: "GET"}); err == nil {
		t.Error("fetched a file URL")
	}

	deny := New(Options{Allow: func(r *http.Request) bool { return r.URL.Path != "/echo" }}).Service.(*service)
	if _, err = run(t, deny, request{URL: srv.URL + "/echo", Method: "GET"}); err == nil || !strings.Contains(err.Error(), "does not allow") {
		t.Errorf("Allow: %v", err)
	}
}

func TestFetchStops(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "first")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	s := New(Options{}).Service.(*service)

	// A streamed body arrives as it is written, and the request stops
	// when the page stops taking it.
	var got []string
	stop := io.ErrClosedPipe
	err := s.fetch(context.Background(), request{URL: srv.URL, Method: "GET"}, func(p part) error {
		if p.Chunk != nil {
			got = append(got, string(p.Chunk))
			return stop
		}
		return nil
	})
	if err != stop || len(got) != 1 || got[0] != "first" {
		t.Errorf("got %q, %v", got, err)
	}
}
