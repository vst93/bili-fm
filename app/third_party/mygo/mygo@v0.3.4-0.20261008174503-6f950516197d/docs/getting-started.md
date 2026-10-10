# Getting started

A MyGo app shows its windows with a web frontend, in the system's webview,
or with [native UI](ui/README.md) that MyGo draws itself, written in Go. `mygo
init` starts a project of either kind, and an app can add windows of the
other kind later.

## Install

Install [Go](https://go.dev/dl/) 1.27 or later. A project with a web
frontend also needs [Bun](https://bun.sh), which runs its tools, and apps
with web pages need the system's webview:

- on Linux, GTK 3 and WebKitGTK 4.1, e.g.
  `sudo apt install libwebkit2gtk-4.1-0` on Debian and Ubuntu;
- on Windows 10, the
  [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/),
  which Windows 11 includes.

Native UI needs nothing more: Go builds the app, and it runs on what the
system has (GTK 3 on Linux).

The `mygo` command line tool creates, runs and packages apps. It is a Go
program, which Go runs without installing it:

```sh
go run github.com/egoist/mygo/cmd/mygo@latest init my-app
```

Projects with a web frontend also depend on it through npm, as the
`mygo-cli` package, so Bun or npm create one too:

```sh
bunx mygo-cli init my-app      # or: npx mygo-cli init my-app
```

You can also install the CLI with Go, which puts `mygo` on your `PATH`:

```sh
go install github.com/egoist/mygo/cmd/mygo@latest
mygo init my-app
```

`mygo doctor` checks that the machine has what MyGo needs.

## Create a project

### A web frontend

`mygo init my-app` creates a Go module and a TypeScript frontend built with
[Vite](https://vite.dev), side by side, and installs their dependencies:

```
my-app/
├── main.go          the app: its windows and the Go code the page calls
├── go.mod
├── mygo.config.ts   the app's name, identifier and version, and how to build it
├── package.json     the scripts, and the frontend's dependencies
├── index.html       the page
├── src/
│   ├── main.ts      the page's code
│   ├── style.css
│   └── mygo.ts      the typed client of the Go code, generated
├── vite.config.ts
├── tsconfig.json
└── resources/
    └── icon.png     the app icon, a 1024×1024 PNG
```

Builds go to `dist/` (the frontend) and `build/` (the packaged apps), and
the development app to `.mygo/`; `.gitignore` leaves them out. See
[configuration](configuration.md) for the fields of `mygo.config.ts`.

### Native UI

`mygo init -template native my-app` creates an app whose window shows
[native UI](ui/README.md): a Go module alone, with no frontend and no Bun.

```
my-app/
├── main.go          the app: its state, its view and its window
├── main_test.go     a test of the view, which runs without a window
├── go.mod           with the CLI as a tool
├── mygo.json        the app's name, identifier and version
└── resources/
    └── icon.png     the app icon, a 1024×1024 PNG
```

The module has the CLI as a
[tool](https://go.dev/doc/modules/managing-dependencies#tools): `go tool
mygo` runs it at the version `go.mod` pins, which
`go get -tool github.com/egoist/mygo/cmd/mygo@latest` updates with MyGo.

## Develop

```sh
cd my-app
bun run dev        # a web frontend: runs mygo dev
go tool mygo dev   # native UI
```

`mygo dev` builds a development version of the app and starts it, then
rebuilds and restarts it when you edit a `.go` file, the configuration, the
icon or a resource. A build that fails, or crashes on start, keeps the
previous one running. Quit the app, or press Ctrl+C, to stop.

With a web frontend, it also writes `src/mygo.ts` and starts the Vite dev
server, from which the development app loads its pages: edit `src/main.ts`
or `src/style.css` and Vite updates the page right away. Development builds
have the web inspector: right-click the page and choose Inspect Element
(Inspect on Windows), or call `win.Page().OpenDevTools()`.

On macOS the development app is a real app bundle, `My App Dev` with the
identifier of the app plus `.dev`, so that it keeps its data, preferences
and permissions apart from the installed app.

## Call Go

### From the page

`main.go` binds a Go value, whose exported methods the page can call:

```go
// Greeter is callable from the frontend: `mygo generate` turns its methods
// into typed TypeScript functions in src/mygo.ts.
type Greeter struct{}

// Greet returns a greeting for name.
func (Greeter) Greet(name string) string {
	if name == "" {
		name = "stranger"
	}
	return "Hello, " + name + "! This message comes from Go."
}

// Tick is sent to the page every second.
var Tick = mygo.NewEvent[time.Time]("tick")

func main() {
	mygo.Bind(Greeter{})
	// ...
}
```

`src/mygo.ts`, which mygo dev keeps up to date, turns them into typed
functions and events:

```ts
import { Greeter, events } from "./mygo";

const greeting = await Greeter.greet("Ada"); // Promise<string>
events.tick.on((time) => console.log(time)); // time: string
```

Add a method to `Greeter`, save, and it is there to call once the app has
restarted. [Calling Go from the frontend](bindings.md) covers what methods
can take and return, errors, events and security.

### From native UI

Native UI is Go, so it calls your code directly, with no bindings. The view
is a function of your app's state that MyGo calls to build each frame; it
asks its elements what happened since the last one:

```go
func (a *app) view(c *ui.Context) {
	ui.Column(c).Fill().Center().Gap(16).Children(func() {
		ui.TextInput(c, &a.name).Placeholder("Your name").Width(240)
		if ui.PrimaryButton(c, "Greet").Clicked() {
			a.greeting = greet(a.name) // your Go code
		}
		ui.Text(c, a.greeting)
	})
}
```

`main_test.go` clicks and types into the view without a window, as fast as
a unit test: run `go test`. [Native UI](ui/README.md) covers layout, widgets,
text, input, drawing and changing the state from other goroutines.

## Build

```sh
bun run build        # a web frontend: runs mygo build
go tool mygo build   # native UI
```

`mygo build` compiles the app, with the frontend built by Vite embedded in
it when there is one, and packages it for the machine's platform in
`build/<os>-<arch>/`:

| Platform | Output |
|---|---|
| macOS | `My App.app`, and a disk image `My App 0.1.0.dmg` |
| Windows | `My App.exe`, and an installer `My App Setup 0.1.0.exe` |
| Linux | the executable `my-app` with its desktop entry and icon, their archive with `install.sh`, which installs it for the user, and a `.deb` package |

MyGo needs no cgo, so any machine builds for every platform:

```sh
bun run build -- -platform darwin/universal,windows/amd64,linux/amd64
go tool mygo build -platform darwin/universal,windows/amd64,linux/amd64
```

The apps run where they are, but to ship them to users, sign them: see
[Building and distributing](distribution.md).

## Next steps

- [Windows](windows.md) and [the application](app.md), to shape the app.
- [Native UI](ui/README.md), to build interfaces in Go, or
  [the frontend](frontend.md), for web pages.
- [Menus and the tray](menus.md) and the [desktop APIs](native.md).
- The examples in the repository:
  - with web pages: `examples/hello` (the smallest app), `examples/todo`
    (typed services and events, persistence, dialogs, menus, several
    windows), `examples/frameless` (a custom title bar) and
    `examples/native` (menus, a tray icon, dialogs, notifications, a
    global shortcut, the clipboard and dark mode);
  - with native UI: `examples/clipboard` (copying and dragging text, HTML,
    lazy JSON, and file lists), `examples/counter-native` (a counter with a test of its
    view), `examples/vibrancy` (a translucent sidebar under an inset title
    bar, its material picked in it), `examples/effort-slider` (a web
    demo's animated slider redrawn: letters blurring in, a burst of
    pixels, a shaking card) and `examples/gallery` (a tour of the
    toolkit: layout, widgets, text editing, a list of ten thousand rows,
    drawing and overlays).
