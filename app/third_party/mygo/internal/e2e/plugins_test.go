package e2e

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/fetch"
	"github.com/egoist/mygo/plugins/sqlite"
	"github.com/egoist/mygo/plugins/websocket"
)

// pluginsPage loads the plugins' JavaScript packages, as built, with an
// import map in place of a bundler.
const pluginsPage = `<!doctype html><html><head>
<script type="importmap">{"imports": {
  "mygo-runtime": "/js/runtime.js",
  "@mygo-plugins/fetch": "/js/fetch.js",
  "@mygo-plugins/sqlite": "/js/sqlite.js",
  "@mygo-plugins/websocket": "/js/websocket.js"
}}</script>
<script type="module">
import { fetch } from "@mygo-plugins/fetch";
import { open } from "@mygo-plugins/sqlite";
import { WebSocket } from "@mygo-plugins/websocket";
window.plugins = { fetch, WebSocket, open };
</script></head><body>plugins</body></html>`

// A request of the page that the page aborted.
var aborted = make(chan struct{}, 1)

// Requests to /hold that were not canceled within a second.
var held = make(chan struct{}, 1)

// usePlugins adds the plugins and serves their test page.
func usePlugins(mux *http.ServeMux) {
	mygo.Use(fetch.Plugin, websocket.Plugin, sqlite.Plugin)
	mux.HandleFunc("/plugins.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, pluginsPage)
	})
	for name, path := range map[string]string{
		"runtime":   "../../packages/runtime/dist/index.js",
		"fetch":     "../../plugins/fetch/dist/index.js",
		"sqlite":    "../../plugins/sqlite/dist/index.js",
		"websocket": "../../plugins/websocket/dist/index.js",
	} {
		mux.HandleFunc("/js/"+name+".js", func(w http.ResponseWriter, r *http.Request) {
			b, err := os.ReadFile(path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/javascript")
			w.Write(b)
		})
	}
}

// pluginServer is a server the page reaches through the plugins: it sends
// no CORS headers.
func pluginServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/origin", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "origin "+r.Header.Get("Origin"))
	})
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "first")
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
			aborted <- struct{}{}
		case <-time.After(10 * time.Second):
		}
	})
	mux.HandleFunc("/hold", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
			held <- struct{}{}
		}
	})
	mux.HandleFunc("/ws", wsEcho)
	return httptest.NewServer(mux)
}

// wsEcho answers text messages with the X-Token of the handshake and the
// message, and the closing handshake with the same code.
func wsEcho(w http.ResponseWriter, r *http.Request) {
	h := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	conn, brw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	fmt.Fprintf(brw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		base64.StdEncoding.EncodeToString(h[:]))
	brw.Flush()
	for {
		op, p, err := readClientFrame(brw.Reader)
		if err != nil {
			return
		}
		if op == 0x8 {
			conn.Write(append([]byte{0x88, byte(min(len(p), 2))}, p[:min(len(p), 2)]...))
			return
		}
		reply := r.Header.Get("X-Token") + ":" + string(p)
		conn.Write(append([]byte{0x81, byte(len(reply))}, reply...))
	}
}

// readClientFrame reads a short masked frame.
func readClientFrame(br *bufio.Reader) (byte, []byte, error) {
	var h [2]byte
	if _, err := io.ReadFull(br, h[:]); err != nil {
		return 0, nil, err
	}
	n := int(h[1] & 0x7f)
	if n == 126 {
		var l [2]byte
		io.ReadFull(br, l[:])
		n = int(binary.BigEndian.Uint16(l[:]))
	}
	var mask [4]byte
	io.ReadFull(br, mask[:])
	p := make([]byte, n)
	if _, err := io.ReadFull(br, p); err != nil {
		return 0, nil, err
	}
	for i := range p {
		p[i] ^= mask[i%4]
	}
	return h[0] & 0x0f, p, nil
}

func TestPlugins(t *testing.T) {
	for _, dist := range []string{"../../packages/runtime/dist", "../../plugins/fetch/dist", "../../plugins/websocket/dist", "../../plugins/sqlite/dist"} {
		if _, err := os.Stat(dist); err != nil {
			t.Skip("the JavaScript packages are not built; run bun run build")
		}
	}
	srv := pluginServer()
	defer srv.Close()
	w := newWindow(t, mygo.WindowOptions{Hidden: true})
	if err := w.Page().LoadURL("app://localhost/plugins.html"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "window.plugins")

	// No CORS, and headers a page may not set.
	got, err := mygo.EvalAs[string](w.Page(), fmt.Sprintf(`(async () => {
		const res = await plugins.fetch(%q, { headers: { origin: "https://example.test" } });
		return res.status + ":" + await res.text();
	})()`, srv.URL+"/origin"))
	if want := "200:origin https://example.test"; err != nil || got != want {
		t.Errorf("fetch: %q, %v; want %q", got, err, want)
	}

	// The body streams, and aborting stops the request in Go.
	got, err = mygo.EvalAs[string](w.Page(), fmt.Sprintf(`(async () => {
		const ctrl = new AbortController();
		const res = await plugins.fetch(%q, { signal: ctrl.signal });
		const reader = res.body.getReader();
		const first = new TextDecoder().decode((await reader.read()).value);
		ctrl.abort();
		return first + ":" + await reader.read().then(() => "read", (e) => e.name);
	})()`, srv.URL+"/stream"))
	if want := "first:AbortError"; err != nil || got != want {
		t.Errorf("aborted fetch: %q, %v; want %q", got, err, want)
	}
	select {
	case <-aborted:
	case <-time.After(5 * time.Second):
		t.Error("the server's request was not canceled")
	}

	// Aborted right away, before Go even started the request.
	got, err = mygo.EvalAs[string](w.Page(), fmt.Sprintf(`(async () => {
		const ctrl = new AbortController();
		const res = plugins.fetch(%q, { signal: ctrl.signal });
		ctrl.abort();
		return await res.then(() => "resolved", (e) => e.name);
	})()`, srv.URL+"/hold"))
	if err != nil || got != "AbortError" {
		t.Errorf("fetch aborted at once: %q, %v", got, err)
	}
	select {
	case <-held:
		t.Error("the request aborted at once ran in Go")
	case <-time.After(1500 * time.Millisecond):
	}

	got, err = mygo.EvalAs[string](w.Page(), fmt.Sprintf(`new Promise((resolve, reject) => {
		const ws = new plugins.WebSocket(%q, [], { headers: { "x-token": "secret" } });
		let data;
		ws.onopen = () => ws.send("hi");
		ws.onmessage = (e) => { data = e.data; ws.close(1000, "done"); };
		ws.onerror = () => reject(new Error("WebSocket error"));
		ws.onclose = (e) => resolve(data + ":" + e.code + ":" + e.wasClean);
	})`, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws"))
	if want := "secret:hi:1000:true"; err != nil || got != want {
		t.Errorf("WebSocket: %q, %v; want %q", got, err, want)
	}
}

// TestSQLitePlugin exercises the built client through the real IPC transport:
// parameters, int64/BLOB values, rollback and page-owned connection cleanup.
func TestSQLitePlugin(t *testing.T) {
	for _, dist := range []string{"../../packages/runtime/dist", "../../plugins/fetch/dist", "../../plugins/websocket/dist", "../../plugins/sqlite/dist"} {
		if _, err := os.Stat(dist); err != nil {
			t.Skip("run bun run build first")
		}
	}
	w := newWindow(t, mygo.WindowOptions{Hidden: true})
	if err := w.Page().LoadURL("app://localhost/plugins.html"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "window.plugins")
	got, err := mygo.EvalAs[string](w.Page(), `(async () => {
        const db = await plugins.open(":memory:");
        await db.execute("CREATE TABLE items(id INTEGER PRIMARY KEY, title TEXT, bytes BLOB)");
        const big = 9223372036854775807n;
        const inserted = await db.execute("INSERT INTO items VALUES (?, ?, ?)", [big, "中文🙂", new Uint8Array([0, 255])]);
        let rolledBack = false;
        try {
            await db.transaction([
                { sql: "INSERT INTO items VALUES (1, 'rollback', NULL)" },
                { sql: "INSERT INTO items VALUES (?, 'duplicate', NULL)", params: [big] },
            ]);
        } catch { rolledBack = true; }
        const result = await db.query("SELECT id, title, bytes FROM items");
        await db.close();
        return [String(inserted.lastInsertId), rolledBack, result.rows.length,
            String(result.rows[0][0]), result.rows[0][1], [...result.rows[0][2]].join(",")].join(":");
    })()`)
	if want := "9223372036854775807:true:1:9223372036854775807:中文🙂:0,255"; err != nil || got != want {
		t.Fatalf("SQLite: %q, %v; want %q", got, err, want)
	}
	// Retain just the old handle across a navigation: the new page's context
	// must not be able to use the connection of the page it replaced.
	id, err := mygo.EvalAs[string](w.Page(), `(async () => {
        window.sqliteDB = await plugins.open(":memory:");
        return window.sqliteDB.id;
    })()`)
	if err != nil {
		t.Fatal(err)
	}
	other := newWindow(t, mygo.WindowOptions{Hidden: true})
	if err := other.Page().LoadURL("app://localhost/plugins.html"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, other, "window.plugins")
	got, err = mygo.EvalAs[string](other.Page(), fmt.Sprintf(`mygo.call("plugin:sqlite.Query", %q, "SELECT 1", []).then(() => "allowed", e => e.message)`, id))
	if err != nil || !strings.Contains(got, "another window") {
		t.Fatalf("SQLite ownership: %q %v", got, err)
	}
	if err := w.Page().LoadURL("app://localhost/plugins.html"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "window.plugins && !window.sqliteDB")
	got, err = mygo.EvalAs[string](w.Page(), fmt.Sprintf(`mygo.call("plugin:sqlite.Query", %q, "SELECT 1", []).then(() => "allowed", e => e.message)`, id))
	if err != nil || !strings.Contains(got, "closed") {
		t.Fatalf("SQLite after navigation: %q %v", got, err)
	}
}
