# WebSocket

The WebSocket plugin gives pages a `WebSocket` whose connections Go makes,
with the API of the browser's. Unlike the webview's own, it:

- sends any header with the opening handshake, such as `Authorization`,
  `Cookie` or `Origin`, for servers that want a token in a header;
- is not subject to the page's Content Security Policy;
- keeps the messages of a fast server in Go while the page is behind,
  instead of in the page's memory.

## Set up

Use the plugin in Go:

```go
import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/websocket"
)

func main() {
	mygo.Use(websocket.Plugin)
	// ...
}
```

and add its package to the frontend:

```sh
bun add @mygo-plugins/websocket
```

A connection opened in an app that does not use the Go half fails, and the
console says to call `mygo.Use`.

## Connecting

Import `WebSocket` from the package and use it as the browser's:

```ts
import { WebSocket } from "@mygo-plugins/websocket";

const ws = new WebSocket("wss://example.com/live", ["v1"], {
  headers: { authorization: `Bearer ${token}` },
});
ws.binaryType = "arraybuffer";
ws.onopen = () => ws.send(JSON.stringify({ subscribe: "prices" }));
ws.onmessage = (e) => console.log(e.data);
ws.onclose = (e) => console.log(e.code, e.reason, e.wasClean);
```

The first two arguments are those of the browser's `WebSocket`: the URL,
`ws:` or `wss:` (`http:` and `https:` become them, and relative URLs
resolve against the page's), and the subprotocols, of which the server
picks one, `ws.protocol` once open. The third is the plugin's: `headers`,
the headers of the opening handshake, as `Headers`, an object or pairs.

The rest is the browser's API: `readyState` and its constants, `send` of
strings, `ArrayBuffer`s, typed arrays and `Blob`s, `close(code, reason)`,
`binaryType` (`"blob"` by default, or `"arraybuffer"`), and the `open`,
`message`, `error` and `close` events, as `on…` properties or with
`addEventListener`.

Libraries that take a `WebSocket` class, as GraphQL and realtime clients
often do, create connections with two arguments; give them a class that
adds the headers:

```ts
class AuthWebSocket extends WebSocket {
  constructor(url: string | URL, protocols?: string | string[]) {
    super(url, protocols, { headers: { authorization: `Bearer ${token}` } });
  }
}
```

## Sending and receiving

- `send` throws while the connection is still opening, and messages sent
  after it closed are discarded, as with the browser's. Messages go out in
  the order they were sent.
- `bufferedAmount` counts the bytes passed to `send` that Go has not
  written yet: a page that sends a lot can wait for it to drop.
- Messages from the server wait in Go while the page has more than a MiB of
  them yet to handle, so a server faster than the page cannot fill its
  memory. A message bigger than `MaxMessageSize`, 64 MiB by default, fails
  the connection.
- Go answers the server's pings.

## Closing

`close(code, reason)` takes a code of 1000 or between 3000 and 4999, and a
reason of at most 123 bytes, as the browser's does. Go then waits for the
server to finish the closing handshake: when it does, the `close` event
has the server's code and reason, and `wasClean` is true; when it has not
after `CloseTimeout` (5 seconds by default), Go drops the connection, and
the event has code 1006.

A connection closes on its own, telling the server it is going away (code
1001), when its page navigates away or its window closes.

When a connection fails, because the server cannot be reached, answers the
handshake without upgrading, redirects it or breaks the protocol, the page
gets an `error` event, then a `close` event with code 1006, and the console
says why, as with the browser's.

## Differences from the browser's WebSocket

- The constructor takes a third argument, the headers of the handshake.
- The page's Content Security Policy (`connect-src`) does not apply, and
  neither do the webview's cookies: those of the Go side's `http.Client`
  do (see [options](#options)).
- No extensions, such as permessage-deflate, are negotiated: `extensions`
  is always empty.

## Options

`websocket.Plugin` opens connections with `http.DefaultClient` to any URL.
`websocket.New` takes options instead:

```go
mygo.Use(websocket.New(websocket.Options{
	Client: &http.Client{Transport: transport},
	Allow: func(r *http.Request) bool {
		return r.URL.Hostname() == "live.example.com"
	},
	MaxMessageSize: 16 << 20,        // 16 MiB; default 64 MiB
	CloseTimeout:   2 * time.Second, // default 5 seconds
}))
```

- `Client` makes the opening handshakes, so its `Transport` (proxies, TLS
  settings) and `Jar` (cookies) apply. Its `Timeout` and `CheckRedirect`
  are ignored: connections last, and redirects fail them, as in browsers.
- `Allow` sees the handshake request of each connection, with its URL and
  headers, and decides whether pages may open it; a connection it refuses
  fails.
- `MaxMessageSize` is the most bytes a message from the server may have; a
  bigger one fails the connection.
- `CloseTimeout` is how long a connection that the page closes waits for
  the server to finish the closing handshake.

Only the app's own pages may call the plugin, as with every bound method
(see [who may call](../bindings.md#who-may-call)).
