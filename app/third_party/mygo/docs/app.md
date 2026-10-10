# The application

`mygo.App` is the application: its lifecycle, its identity, and how it
meets the rest of the system.

## The main function

```go
func main() {
	mygo.Bind(&Notes{})                // services the frontend calls
	mygo.App.WhenReady(func() {        // once the app has launched
		mygo.NewWindow(mygo.WindowOptions{URL: "/"})
	})
	if err := mygo.App.Run(); err != nil { // the event loop, until the app quits
		log.Fatal(err)
	}
}
```

`App.Run` runs the native event loop and returns when the app quits. It
must be called from the main goroutine, because the UI toolkits of every
platform must run on the thread that started the process; MyGo keeps the
main goroutine on it. Windows exist only once the app is ready: create them
in `WhenReady`, or from any goroutine after that.

Before `Run`, `main` sets the app up: it binds services, handles protocols,
adds listeners, and makes settings such as `App.SetName`, `App.SetMenu`,
`App.Dock.SetMenu` and `Theme.SetSource`, which take effect when the app
starts. It may also read what needs no running app, such as `App.Path`,
`App.Locale` or `Power.IsOnBattery`, and register the app with the system
(`App.SetOpenAtLogin`, `App.RegisterURLScheme`). Everything else needs the
running app: when `main` calls the clipboard, displays, dialogs,
notifications, global shortcuts, trays, Dock badges, `Theme.IsDark` and the
like before `Run`, they panic with a message naming the call, on every
platform. Use them from `WhenReady` on; other goroutines may call them
earlier, and wait for the app to start.

## Threads

Every function and method of MyGo is safe to call from any goroutine: calls
made off the main thread are sent to it and wait for the result. What runs
where:

- Event listeners, such as `OnClose` or `OnActivate`, menu item clicks and
  global shortcuts run on the main thread, which also draws the UI: keep
  them short, and start goroutines for slow work.
- Bound methods run on goroutines of their own, one per call: they may
  block.
- `mygo.RunOnMain(fn)` runs `fn` on the main thread, for native APIs called
  through `Window.NativeHandle`.

## Lifecycle

| Event | When |
|---|---|
| `OnReady` | the app has launched; `WhenReady` also runs its function right away when the app is already ready |
| `OnActivate(func(hasVisibleWindows bool))` | the app was activated again, e.g. by a click on its Dock icon (macOS) |
| `OnDidBecomeActive`, `OnDidResignActive` | the app became, or stopped being, the active app |
| `OnWindowCreated(func(w *mygo.Window))` | a window was created |
| `OnWindowAllClosed` | the last window closed, unless the app is quitting |
| `OnBeforeQuit(func(e *mygo.QuitEvent))` | a quit started, before the windows close; `e.PreventDefault()` cancels it |
| `OnWillQuit(func(e *mygo.QuitEvent))` | every window has closed and the app is about to quit; it can still be canceled |
| `OnQuit` | the event loop stopped, right before `Run` returns |

Each returns a function that removes the listener.

### Quitting

When its last window closes, an app quits, unless `OnWindowAllClosed` has a
listener. On macOS apps usually keep running with their Dock icon, and open
a window again when it is clicked:

```go
mygo.App.OnWindowAllClosed(func() {
	if runtime.GOOS != "darwin" {
		mygo.App.Quit()
	}
})
mygo.App.OnActivate(func(hasVisibleWindows bool) {
	if !hasVisibleWindows {
		openMainWindow()
	}
})
```

- `App.Quit()` closes every window, which `OnClose` listeners can refuse,
  then quits; `OnBeforeQuit` and `OnWillQuit` listeners can cancel it too.
  Cmd+Q, the Quit menu item and Ctrl+C in the terminal (SIGINT and SIGTERM)
  quit the same way; a second signal exits at once.
- `App.Exit(code)` exits right away, without events.
- `App.Relaunch()` quits, then starts the app again with the same
  arguments, for example after a setting that needs a restart.

## A single instance

Most desktop apps run once: starting them again brings the running
instance to the front. `RequestSingleInstanceLock` returns false in a
second instance, after passing its command line to the first:

```go
func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return // the running instance takes over
	}
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		if w := mainWindow(); w != nil {
			w.Restore()
			w.Focus()
		}
	})
	// ...
}
```

## Deep links

Apps open URLs of their own schemes, such as `myapp://open?note=42`. List
the schemes in mygo.config.ts, which registers them with the system when the
app is installed:

```ts
export default defineConfig({
  urlSchemes: ["myapp"],
});
```

and handle the URLs with `OnOpenURL`, registered before `Run` so it also
gets the URL the app was started with:

```go
mygo.App.OnOpenURL(func(rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	openNote(u.Query().Get("note"))
})
```

On Windows and Linux a URL starts a new instance of the app: request the
[single instance lock](#a-single-instance), and the URLs of later instances
reach `OnOpenURL` in the first one. Apps that are not installed with the
package `mygo build` makes, a portable executable for example, register
their schemes at run time with `App.RegisterURLScheme("myapp")`, which is
cheap to call at every launch; `UnregisterURLScheme` and
`IsURLSchemeRegistered` go with it. On macOS schemes must be listed in
the configuration.

## File associations

Apps open files of their types from the file manager. Declare the types in
mygo.config.ts:

```ts
export default defineConfig({
  fileAssociations: [
    {
      ext: ["md", "markdown"],
      name: "Markdown Document",
      mimeType: "text/markdown",
    },
  ],
});
```

and open them with `OnOpenFile`, registered before `Run` so it also gets
the files the app was started with:

```go
mygo.App.OnOpenFile(func(path string) {
	openDocument(path)
})
```

It also gets files dropped on the app's Dock icon (macOS) and files passed
on the command line. As with deep links, the single instance lock makes the
first instance get the files of later ones on Windows and Linux.

## Start at login

```go
if err := mygo.App.SetOpenAtLogin(true); err != nil {
	log.Println(err)
}
enabled := mygo.App.OpenAtLogin()
```

Offer it as a setting. The app becomes a login item of the user on macOS
13 and later (which needs the packaged app), a launch agent on macOS 12, an
XDG autostart entry on Linux and a Run entry of the user on Windows, which
the system's settings list and can turn off. `WasOpenedAtLogin` tells a
launch at login apart, to start in the background:

```go
mygo.NewWindow(mygo.WindowOptions{URL: "/", Hidden: mygo.App.WasOpenedAtLogin()})
```

## Directories

`App.Path` returns well-known directories:

| Name | Directory |
|---|---|
| `PathUserData` | the app's data: `~/Library/Application Support/<Name>`, `~/.config/<Name>` (`$XDG_CONFIG_HOME`) or `%AppData%\<Name>`; created on first use |
| `PathCache` | the app's cache: `~/Library/Caches/<Name>`, `~/.cache/<Name>` or `%LocalAppData%\<Name>`; created on first use |
| `PathLogs` | where to write logs: `~/Library/Logs/<Name>`, else `logs` in the user data directory; created on first use |
| `PathResources` | the files the app ships with, see [resources](distribution.md#resources) |
| `PathAppData` | the per-user configuration directory, without the app's name |
| `PathHome`, `PathTemp`, `PathExe` | the home directory, the temporary directory, the executable |
| `PathDesktop`, `PathDocuments`, `PathDownloads`, `PathMusic`, `PathPictures`, `PathVideos` | the user's folders |

```go
dir, err := mygo.App.Path(mygo.PathUserData)
db := filepath.Join(dir, "notes.db")
```

`App.SetPath` overrides a directory, for example to keep test data apart.

## Name and version

A packaged app knows its name and version from its configuration, which
`mygo build` links into it: `App.Name()` and `App.Version()`. A program run with
`go run` has the executable's name and no version, unless it calls
`App.SetName` and `App.SetVersion`, before `Run`. The name sets the user
data, cache and log directories and the single instance lock. Development
builds of `mygo dev` are named `<Name> Dev`, so that they keep their data
apart from the installed app's; calling `SetName` ties them together
again.

`mygo.IsDev()` reports whether the app is a development build: `go run`,
`go build` and `mygo dev` builds are, `mygo build` builds are not, and
`MYGO_ENV=production` makes any build behave like one. `App.IsPackaged()`
reports whether it runs from an app bundle, and `App.Locale()` returns the
user's language, such as `"en-US"`.

## The Dock and the taskbar

- `App.SetBadgeCount(n)` shows a count on the app's icon: the Dock (macOS)
  or the launcher entry (Linux docks that support it); zero clears it.
- `App.Dock` controls the Dock icon on macOS: `SetBadge` shows any text,
  `Bounce` asks for attention, `SetIcon` replaces the icon, `SetMenu` adds
  items to its menu (see [the Dock menu](menus.md#the-dock-menu)), and
  `Hide` and `Show` remove and restore it.
- `App.SetActivationPolicy(mygo.ActivationPolicyAccessory)` removes the
  Dock icon of an app that lives in the menu bar, see
  [tray icons](menus.md#tray-icons); its windows can still show.
- `App.Focus` brings the app to the front. On macOS, `App.Hide` and
  `App.Show` hide and show all its windows, and `App.ShowAboutPanel` shows
  the standard about panel.

## Browsing data

`App.ClearBrowsingData()` deletes what the app's pages stored: cookies,
local and session storage, IndexedDB, service workers and caches, for
example when the user signs out. Open pages keep what they hold in memory
until they reload.
