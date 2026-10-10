# Plugins

A plugin adds a feature to web pages in two halves: Go services, and a
JavaScript package whose functions call them. The app uses the Go half with
`mygo.Use` and imports the JavaScript half from npm.

## Official plugins

- [Fetch](plugins/fetch.md): a `fetch` that makes HTTP requests from Go,
  with no CORS, any header, streamed bodies and cancellation.
- [WebSocket](plugins/websocket.md): a `WebSocket` whose connections Go
  makes, with headers on the handshake.
- [SQLite](plugins/sqlite.md): local databases, parameterized queries and
  atomic transactions, using SQLite's C engine compiled with Zig, without cgo.
- [Updater](plugins/updater.md): an update window in the manner of
  Sparkle, which checks for updates and offers to install them. It is all
  Go, and its window is a web page or, for apps of native UI, native UI.
- [Terminal](plugins/terminal.md): a terminal for native UI, a view that
  runs the shell or any program with Ghostty's terminal emulator. It is
  all Go, with a native library that the CLI puts into apps.
- [Glass](plugins/glass.md): Liquid Glass for native UI, as macOS draws
  it, on every platform, with macOS's scroll edges and backdrop blurs. It
  is all Go, with shaders of its own.

## Using plugins

`mygo.Use` takes the plugins of the app, before `App.Run`:

```go
import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/fetch"
	"github.com/egoist/mygo/plugins/websocket"
)

func main() {
	mygo.Use(fetch.Plugin, websocket.Plugin)
	// ...
}
```

and the frontend imports their packages:

```sh
bun add @mygo-plugins/fetch @mygo-plugins/websocket
```

```ts
import { fetch } from "@mygo-plugins/fetch";
import { WebSocket } from "@mygo-plugins/websocket";

const res = await fetch("https://api.example.com/me", {
  headers: { authorization: `Bearer ${token}` },
});
const ws = new WebSocket("wss://api.example.com/live", [], {
  headers: { authorization: `Bearer ${token}` },
});
```

The `Plugin` variable of an official plugin has the default options, and
its `New` function takes options, such as the `http.Client` of the fetch
and WebSocket plugins and an `Allow` function that decides which requests
pages may make. Each plugin's page lists them.

Using a plugin's JavaScript package without its Go half fails with an error
that says to call `mygo.Use`.

## Writing a plugin

The Go half is a `mygo.Plugin`: a name, a service whose methods pages call,
and an optional `Setup` that runs when the app uses it:

```go
package clock

// Plugin gives pages the time of the Go side.
var Plugin = mygo.Plugin{Name: "clock", Service: service{}}

type service struct{}

func (service) Now() time.Time { return time.Now() }
```

The service is bound like those of `mygo.Bind`, with the same rules for its
methods (contexts, channels, errors), but as `plugin:<name>` and outside the
generated TypeScript client. Its JavaScript package calls it with `call` of
`mygo-runtime`, which it lists as a peer dependency, so that it uses the
app's copy (and as a development dependency, for its own builds and tests):

```ts
import { call } from "mygo-runtime";

export const now = async () => new Date(await call<string>("plugin:clock.Now"));
```

Only the app's trusted pages may call a plugin, as with every bound method.
`mygo.Use` panics when a name is taken or the service's methods use types
that cannot cross to JavaScript, so mistakes show at startup.
