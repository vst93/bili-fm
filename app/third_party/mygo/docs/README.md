# MyGo documentation

MyGo builds desktop apps in Go. Each window shows one of two kinds of
interface, and one app can have windows of both:

- **A web page**, in the webview of the operating system (WKWebView on
  macOS, WebKitGTK on Linux, WebView2 on Windows) instead of a bundled
  browser. The frontend is HTML, CSS and JavaScript built with any tools,
  and calls your Go code through a TypeScript client that MyGo generates
  from it, with the types of your Go structs and the documentation of your
  methods.
- **Native UI**, which MyGo draws itself on the GPU, written in Go alone
  with package `ui`: there is no HTML, no JavaScript and no webview to
  start, so the window opens at once and takes little memory.

Windows of both kinds share the window options and events, the menus and
the desktop APIs, and the app is a single Go program of a few megabytes.

A window with a web page, whose frontend calls a Go service:

```go
type Greeter struct{}

// Greet returns a greeting.
func (Greeter) Greet(name string) string { return "Hello, " + name }

func main() {
	mygo.Bind(Greeter{})
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{Title: "Hello", URL: "/"})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
```

```ts
import { Greeter } from "./mygo"; // generated

document.body.textContent = await Greeter.greet("Ada");
```

A window with native UI, whose view is a Go function of the app's state:

```go
type counter struct{ n int }

func (s *counter) view(c *ui.Context) {
	ui.Column(c).Fill().Center().Gap(12).Children(func() {
		ui.Text(c, fmt.Sprint(s.n)).FontSize(40).Bold()
		if ui.PrimaryButton(c, "Increment").Clicked() {
			s.n++
		}
	})
}

func main() {
	s := &counter{}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:   "Counter",
			Content: ui.View(s.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
```

Web pages suit rich documents, existing web code and what only a browser
does; native UI suits tools, settings, inspectors and utilities, and apps
that must start instantly.

## Guides

- [Getting started](getting-started.md): install the tools, then create,
  develop and build an app, with a web frontend or native UI.
- [Windows](windows.md): creating and arranging windows of both kinds,
  their events, and what windows showing web pages do with them:
  navigation, downloads, permissions, printing.
- [The application](app.md): the lifecycle, quitting, a single instance,
  deep links, file associations, starting at login and well-known
  directories.
- [Menus and the tray](menus.md): application, context and Dock menus,
  keyboard shortcuts and tray icons.
- [Desktop APIs](native.md): dialogs, notifications, the clipboard, the
  shell, displays, dark mode, power and global shortcuts.
- [Clipboard and drag data](data-transfer.md): multiple representations,
  file lists, custom formats, lazy providers, and clipboard persistence.
- [Building and distributing](distribution.md): packaged apps for macOS,
  Windows and Linux, signing, installers and disk images.
- [Auto-updates](updates.md): signed updates from GitHub releases, an S3
  bucket or your own server, delta updates, and an update window in the
  manner of Sparkle.
- [GitHub Actions](github-actions.md): a workflow that builds, signs and
  notarizes the apps of every platform, and publishes them with their
  updates, when you push a tag.

## Web frontends

- [The frontend](frontend.md): how pages load during development and in
  builds, the `mygo-runtime` package, custom protocols, custom title bars
  and dropped files.
- [Calling Go from the frontend](bindings.md): bound services, channels,
  typed events and the generated TypeScript client.
- [Plugins](plugins.md): features that come in a Go package and an npm
  package, such as the official plugins, and writing your own.

## Native UI

[Native UI](ui/README.md) has documentation of its own: guides to views,
layout, input, navigation and accessibility, and a page for each of its
components, from buttons to tables.

## Official plugins

- [Fetch](plugins/fetch.md): a `fetch` that makes HTTP requests from Go,
  with no CORS, any header, streamed bodies and cancellation.
- [WebSocket](plugins/websocket.md): a `WebSocket` whose connections Go
  makes, with headers on the handshake.
- [SQLite](plugins/sqlite.md): local databases with parameterized queries
  and atomic transactions, without cgo.
- [Updater](plugins/updater.md): an update window in the manner of
  Sparkle, which checks for updates and offers to install them.
- [Terminal](plugins/terminal.md): a terminal for native UI, which runs
  the shell or any program with Ghostty's terminal emulator.
- [Glass](plugins/glass.md): Liquid Glass for native UI, as macOS draws
  it, on every platform, with macOS's scroll edges and backdrop blurs.

## Reference

- [Configuration](configuration.md): `mygo.config.ts`, or `mygo.json`.
- [The mygo CLI](cli.md): its commands and flags.
- The Go API: `go doc -all github.com/egoist/mygo`, with the documentation
  of every type and method.
- [Architecture](architecture.md): how MyGo works inside, for contributors,
  with a table of the differences between platforms.

## Requirements

To develop apps you need [Go](https://go.dev/dl/) 1.27 or later and, for
the frontend tooling of projects with a web frontend,
[Bun](https://bun.sh); projects of native UI need Go alone. MyGo uses no
cgo, so there is no C toolchain to install, and any machine can compile the
apps of every platform; signing and disk images of macOS apps need a Mac.

Apps run on:

| Platform | Windows with web pages need | Windows of native UI need |
|---|---|---|
| macOS 12 or later | nothing: WKWebView is part of macOS | nothing: they draw with Metal |
| Linux (x64, arm64) | GTK 3 and WebKitGTK 4.1 (or 4.0): `libwebkit2gtk-4.1-0` on Debian and Ubuntu, `webkit2gtk4.1` on Fedora | GTK 3 alone |
| Windows 10 and 11 (x64, arm64) | the [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/), which Windows 11 includes | nothing: they draw with Direct3D 11 |

An app whose windows all show native UI needs neither WebKitGTK nor the
WebView2 Runtime. Tray icons on Linux also need `libayatana-appindicator3`.

`mygo doctor` checks a development machine.
