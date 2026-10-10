# Windows

A window is a native window that shows a web page, loaded from its `URL`,
or [native UI](ui/README.md) that MyGo draws, from its `Content`. Create windows
with `mygo.NewWindow` once the app is ready:

```go
mygo.App.WhenReady(func() {
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     "Editor",
		URL:       "/",
		Width:     1200,
		Height:    800,
		MinWidth:  600,
		MinHeight: 400,
		StateKey:  "editor",
	})
	win.OnClosed(func() { log.Println("closed") })
})
```

A window of native UI takes `Content` in place of `URL`:

```go
mygo.NewWindow(mygo.WindowOptions{Title: "Inspector", Content: ui.View(app.view)})
```

Everything in this guide applies to windows of both kinds, apart from
[pages](#pages), which only windows showing web pages have, and the
options about them. Like every MyGo API, windows can be created and
changed from any goroutine; calls hop to the main thread, which drives the
UI.

## Options

The zero `WindowOptions` is a visible, resizable window of 800×600 centered
on the screen. Sizes and positions are in device-independent pixels.

| Option | |
|---|---|
| `Title` | the window title; defaults to the app name, and a page's `<title>` replaces it |
| `URL` | loaded once the window exists; `/` and other URLs without a scheme are pages of the [frontend](frontend.md#how-pages-load) |
| `Content` | shows [native UI](ui/README.md) that MyGo draws, instead of a page |
| `Width`, `Height` | the size of the window, or of what it shows with `UseContentSize` |
| `X`, `Y` | the top-left corner; the window is centered when both are 0 |
| `MinWidth`, `MinHeight`, `MaxWidth`, `MaxHeight` | limits of resizing; 0 means none |
| `StateKey` | remembers the window's bounds and state under this key, see [below](#remember-where-windows-were) |
| `Hidden` | creates the window without showing it |
| `Maximized`, `FullScreen` | creates it maximized or in full screen |
| `Frameless` | no title bar and borders: the page draws them, see [custom title bars](frontend.md#custom-title-bars), or the [native UI](ui/windows.md) |
| `TitleBarStyle` | hides the title bar but keeps the window buttons over the page, see [custom title bars](frontend.md#custom-title-bars), or over [native UI](ui/windows.md) |
| `TrafficLightPosition` | moves the window buttons of a hidden title bar (macOS) |
| `TitleBarHeight` | the height of the title bar the page draws under the window buttons (Linux, Windows) |
| `Transparent`, `Vibrancy` | a transparent window, and the material behind a transparent page or [native UI](ui/windows.md) (macOS, Windows 11 22H2). On Windows, a window with a material has no menu bar: Alt and F10 open its menus in a popup |
| `BackgroundColor` | fills the window until the page paints, in CSS syntax such as `"#1e1e1e"`, or `"light-dark(#f5f5f7, #1e1e1e)"` to follow the appearance |
| `Opacity` | between 0 and 1 |
| `DisableResize`, `DisableMove`, `DisableMinimize`, `DisableMaximize`, `DisableClose`, `DisableFullScreen`, `DisableShadow` | take abilities away |
| `AlwaysOnTop` | keeps the window above others |
| `SkipTaskbar` | leaves the window out of the taskbar (Linux, Windows) |
| `AutoHideMenuBar` | shows the menu bar only while the keyboard is in it, from Alt or F10 (Linux, Windows), see [the menu bar](menus.md#the-menu-bar) |
| `Parent`, `Modal` | a child window, modal to its parent |
| `Page` | the options of the window's [page](#pages): its preload script, trusted origins, web inspector, zoom and user agent |

## Show windows without flashing

A window showing a web page shows its `BackgroundColor`, white by
default, until the page paints, so make it the page's background. When the page follows dark mode,
give both colors with `light-dark(<light>, <dark>)`: the window switches
with the appearance. To show windows only once their page is ready, create
them hidden and show them from `OnReadyToShow`:

```go
win := mygo.NewWindow(mygo.WindowOptions{URL: "/", Hidden: true, BackgroundColor: "#1e1e1e"})
win.OnReadyToShow(win.Show)
```

Windows of native UI need neither: they have no page to load, and draw
their first frame as they show.

## Remember where windows were

With a `StateKey`, a window remembers its position, size, and maximized and
full screen state in `window-state.json` in the user data directory, which
is written when the window closes and when the app quits. The next window
created with the same key, at the next launch for example, gets them back,
as long as it would show on a connected display. They override `X`, `Y`,
`Width`, `Height`, `UseContentSize`, `Maximized` and `FullScreen`, which
stay the defaults of the first launch. Give each kind of window its own key:
`"main"`, `"preferences"`.

## Size, position and state

```go
win.SetSize(1024, 768)
win.SetContentSize(1024, 700) // the page area
win.SetPosition(100, 100)
win.SetBounds(mygo.Rectangle{X: 100, Y: 100, Width: 1024, Height: 768})
win.Center()

win.Maximize()          // Unmaximize, ToggleMaximize, IsMaximized
win.Minimize()          // Restore, IsMinimized
win.SetFullScreen(true) // ToggleFullScreen, IsFullScreen
win.Hide()              // Show, ShowInactive (without focusing), IsVisible
win.Focus()             // Blur, IsFocused
```

`Bounds`, `ContentBounds`, `Size`, `ContentSize` and `Position` read them
back; [`mygo.Screen`](native.md#displays) tells where the displays are.
On Linux under Wayland the compositor decides where windows go and
ignores the positions apps set.

Other properties:

- `SetTitle`, `SetResizable`, `SetMovable`, `SetMinimizable`,
  `SetMaximizable`, `SetClosable`, `SetMinimumSize`, `SetMaximumSize`,
  `SetAlwaysOnTop`, `SetOpacity`, `SetHasShadow`, `SetBackgroundColor` and
  `SetVibrancy` change what the options set. On Windows, `SetVibrancy`
  changes the material of a window created with one, and shows none in
  other windows.
- `SetIcon(png)` gives the window its own icon in its title bar and taskbar
  button (Linux, Windows).
- `SetSkipTaskbar` hides the window from the taskbar (Linux, Windows).
- `SetAutoHideMenuBar` hides the menu bar until Alt or F10, or shows it
  for good again (Linux, Windows).
- `SetVisibleOnAllWorkspaces` shows it on every Space or virtual desktop
  (macOS, Linux).
- `SetContentProtection(true)` keeps it out of screenshots and screen
  recordings.
- `SetIgnoreMouseEvents(true)` lets clicks through to what is behind it.
- `SetProgressBar` shows the progress of a task on the taskbar button
  (Windows), the Dock icon (macOS) or the launcher entry (Linux docks that
  support it):

  ```go
  win.SetProgressBar(mygo.ProgressBar{Value: 0.4})
  win.SetProgressBar(mygo.ProgressBar{State: mygo.ProgressIndeterminate})
  win.SetProgressBar(mygo.ProgressBar{}) // done
  ```

- `FlashFrame(true)` asks for attention: the taskbar button flashes
  (Windows), the window is marked urgent (Linux) or the Dock icon bounces
  (macOS).

## Child and modal windows

A window with a `Parent` stays above it. With `Modal`, it also blocks its
parent until it closes, like a dialog:

```go
prefs := mygo.NewWindow(mygo.WindowOptions{
	URL:    "/preferences",
	Parent: win,
	Modal:  true,
	Width:  500,
	Height: 400,
})
```

## Events

Listeners run on the main thread: keep them short, and start goroutines for
slow work. Each `On…` method returns a function that removes the listener.

| Event | When |
|---|---|
| `OnClose(func(e *mygo.CloseEvent))` | the window is about to close, by the user or `Close`; `e.PreventDefault()` keeps it open |
| `OnClosed` | the window closed |
| `OnFocus`, `OnBlur` | it gained or lost the keyboard focus |
| `OnShow`, `OnHide` | it was shown or hidden |
| `OnResize`, `OnMove` | it was resized or moved |
| `OnMaximize`, `OnUnmaximize`, `OnMinimize`, `OnRestore` | its state changed |
| `OnEnterFullScreen`, `OnLeaveFullScreen` | it entered or left full screen |
| `OnReadyToShow` | its first page is ready to be displayed (windows showing a web page) |
| `OnFileDrop` | files were dropped on its page, see [dropped files](frontend.md#dropped-files), or on native UI that did not take them, see [dropped files](ui/input.md#dropped-files) |

Ask before closing a window with unsaved changes:

```go
win.OnClose(func(e *mygo.CloseEvent) {
	if !doc.Dirty() {
		return
	}
	e.PreventDefault()
	go func() {
		res, _ := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  win,
			Type:    mygo.MessageQuestion,
			Message: "Save changes before closing?",
			Buttons: []string{"Save", "Don't Save", "Cancel"},
		})
		switch res.Button {
		case 0:
			doc.Save()
			win.Destroy()
		case 1:
			win.Destroy()
		}
	}()
})
```

`Close` closes a window as the user would, asking `OnClose` listeners;
`Destroy` closes it at once.

## Pages

A window shows one page at a time and has a history, like a browser tab.
`win.Page()` returns it, or nil for a window that shows
[native UI](ui/README.md):

```go
page := win.Page()
page.LoadURL("/settings") // LoadHTML, LoadFile
page.Reload()             // ReloadIgnoringCache, Stop
page.GoBack()             // GoForward, CanGoBack, CanGoForward
page.URL()                // the current page
page.SetZoomFactor(1.25)  // ZoomFactor
```

`WindowOptions.Page` sets up the page:

```go
mygo.NewWindow(mygo.WindowOptions{
	URL:  "/",
	Page: mygo.PageOptions{ZoomFactor: 1.25, DevTools: mygo.DevToolsEnabled},
})
```

| Option | |
|---|---|
| `PreloadScript` | JavaScript run before every page's own scripts |
| `TrustedOrigins` | other origins whose pages may call Go, see [who may call](bindings.md#who-may-call) |
| `DevTools` | `DevToolsAuto` (the inspector in development builds), `DevToolsEnabled` or `DevToolsDisabled`; for a window of native UI, its [inspector](ui/inspector.md) |
| `ZoomFactor`, `UserAgent` | the page's zoom (1 is 100%) and user agent |

Page events, on `win.Page()`:

| Event | When |
|---|---|
| `OnWillNavigate(func(e *mygo.NavigateEvent))` | the page is about to navigate, for example after a click on a link; `e.PreventDefault()` cancels it. Not called for `LoadURL` and its siblings. |
| `OnDidNavigate(func(url string))` | a navigation committed |
| `OnDOMReady` | the page's DOM is ready (`DOMContentLoaded`) |
| `OnDidFinishLoad` | the page finished loading |
| `OnDidFailLoad(func(err *mygo.LoadError))` | the page failed to load |
| `OnPageTitleUpdated(func(e *mygo.TitleEvent))` | the page's `<title>` changed; preventing the event keeps the window title as it is |
| `OnRenderProcessGone(func(reason string))` | the web content process crashed or was killed; `Reload` recovers |

Keep a window on the app's own pages and open other links in the browser:

```go
win.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
	if u, err := url.Parse(e.URL); err == nil && (u.Scheme == "http" || u.Scheme == "https") && e.UserInitiated {
		e.PreventDefault()
		go mygo.Shell.OpenExternal(e.URL)
	}
})
```

During development the app's pages come from the dev server, an `http` URL:
check the host, or `mygo.IsDev()`, when your rule depends on the scheme.

Links that open new windows go through the window's open handler, see
[links and new windows](frontend.md#links-and-new-windows).

### Find in page

`FindInPage` highlights the matches of a text and reports how many there
are, like the find bar of a browser; call it as the user types, and with
`FindNext` to go to the next match:

```go
res, err := win.Page().FindInPage("mygo", mygo.FindOptions{FindNext: true})
fmt.Printf("%d of %d\n", res.Active, res.Matches)
win.Page().StopFindInPage()
```

### Printing, PDF and screenshots

```go
win.Page().Print() // the print dialog

pdf, err := win.Page().PrintToPDF(mygo.PDFOptions{
	PageSize:   mygo.PageA4,
	Landscape:  false,
	Margins:    &mygo.Margins{Top: 0.5, Right: 0.5, Bottom: 0.5, Left: 0.5}, // inches
	Background: true, // print background colors and images
})

png, err := win.CapturePage() // what the window shows, as a PNG
```

`PrintToPDF` lays the page out for printing, so its print style sheets
apply. The zero `PDFOptions` prints Letter pages in portrait with margins of
0.4 inch and no backgrounds, like a browser's print dialog.

### Downloads

A page downloads a file when it follows a link with the `download`
attribute, or gets a response it cannot show, such as an attachment. By
default the file goes to the Downloads directory, under the name the page
or the server suggests. `OnWillDownload` chooses another path or cancels
the download, and `OnDownloadDone` reports how it ended:

```go
win.Page().OnWillDownload(func(e *mygo.DownloadEvent) {
	if strings.HasSuffix(e.SuggestedName, ".exe") {
		e.PreventDefault()
		return
	}
	dir, _ := mygo.App.Path(mygo.PathDocuments)
	e.Path = filepath.Join(dir, e.SuggestedName)
})
win.Page().OnDownloadDone(func(d *mygo.Download) {
	if d.Err == nil {
		mygo.Shell.ShowItemInFolder(d.Path)
	}
})
```

### Permissions

Pages ask for the camera, the microphone, the location or notifications.
By default the app's own pages get what they ask for and other pages get
nothing. `Page.SetPermissionHandler` decides instead; it runs on the main
thread:

```go
win.Page().SetPermissionHandler(func(req mygo.PermissionRequest) bool {
	return req.Origin == "https://meet.example.com" &&
		!slices.Contains(req.Permissions, mygo.PermissionGeolocation)
})
```

The operating system may ask the user too. macOS does, once per app, for
the camera and the microphone, and ends apps that use them without
explaining why in `Info.plist`: add usage descriptions to `macos.infoPlist`
in mygo.config.ts:

```ts
export default defineConfig({
  macos: {
    infoPlist: {
      NSCameraUsageDescription: "Scan documents with the camera.",
      NSMicrophoneUsageDescription: "Record voice notes.",
    },
  },
});
```

### The web inspector

`OpenDevTools`, `CloseDevTools`, `ToggleDevTools` and `IsDevToolsOpened`
of a page control its inspector, when it has one, see
[the web inspector](frontend.md#the-web-inspector).

## Finding windows

- `mygo.Windows()` lists the open windows, `mygo.FocusedWindow()` returns
  the focused one and `mygo.WindowByID(id)` the one with an `ID()`, which
  pages know as `runtime().windowId`.
- `mygo.CallerWindow(ctx)` returns the window whose page called a bound
  method.
- `mygo.App.OnWindowCreated` is called for every new window.

## Native access

`win.NativeHandle()` returns the native window, an `NSWindow*` on macOS, a
`GtkWindow*` on Linux and an `HWND` on Windows, for APIs MyGo does not
cover. Call native APIs on the main thread with `mygo.RunOnMain`.
