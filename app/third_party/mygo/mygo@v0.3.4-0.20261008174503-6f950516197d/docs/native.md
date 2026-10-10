# Desktop APIs

MyGo wraps the parts of the operating system desktop apps use most, for
apps whose windows show web pages, native UI or both. Like the rest of
MyGo, each can be called from any goroutine.

## Dialogs

`mygo.Dialog` shows the system's file and message dialogs. Its methods
block until the user answers: call them from bound methods, which run on
goroutines of their own, or start a goroutine for them in event listeners
and menu items, which should return quickly. `Parent` attaches a dialog to a
window, as a sheet on macOS.

```go
// Pick files.
paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
	Parent:   win,
	Title:    "Import",
	Filters:  []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg"}}},
	Multiple: true,
}) // nil when canceled

// Choose where to save.
path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
	Parent:      win,
	DefaultPath: "notes.json",
}) // "" when canceled

// Ask a question.
res, err := mygo.Dialog.Message(mygo.MessageOptions{
	Parent:        win,
	Type:          mygo.MessageWarning,
	Message:       "Delete 3 notes?",
	Detail:        "You cannot undo this.",
	Buttons:       []string{"Delete", "Cancel"},
	CheckboxLabel: "Don't ask again",
})
if res.Button == 0 { /* Delete */ }

// Report an error.
mygo.Dialog.Error("Import failed", err.Error())
```

`OpenDialogOptions` also pick directories (`Directory`), show hidden files
and, on macOS, a message and a New Folder button. In message dialogs Enter
activates `DefaultButton` and Escape `CancelButton`, by default the first
button labeled Cancel or No.

## Notifications

```go
n := mygo.NewNotification(mygo.NotificationOptions{
	Title: "Export finished",
	Body:  "notes.json was saved to Documents.",
})
n.OnClick(func() { mainWindow().Focus() })
if err := n.Show(); err != nil {
	log.Println(err)
}
```

`Close` removes a notification. macOS shows notifications of packaged apps
only, which `mygo dev` and `mygo build` make; `mygo.NotificationsSupported()`
reports whether the app can show them. Linux shows them through the
desktop's notification service, Windows as notification-area balloons.
`Show` returns once the system has the notification. On macOS the first
one asks the user whether to allow notifications, and `Show` waits for the
answer; it returns `mygo.ErrNotificationsDenied` when they are not allowed,
by that answer or later in System Settings.

macOS keeps a notification in Notification Center after it is clicked,
until the app removes it: `Close` it in `OnClick` to take it away, or call
`mygo.ClearNotifications()`, which removes all of the app's notifications,
those of earlier runs too, as the app comes to the front:

```go
n.OnClick(func() {
	mainWindow().Focus()
	n.Close()
})

// What the user has seen in the app does not wait in Notification Center.
mygo.App.OnDidBecomeActive(func() { mygo.ClearNotifications() })
```

Linux asks the desktop's notification service to close them, which may
have closed them already, and Windows hides its balloon.

A notification can outlive the run that showed it, in Notification Center
on macOS, where a click launches the app again. Give it an `ID` of the
app's own, which `App.OnNotificationClick` receives for every click, that
one included when it is registered before `Run`; showing a notification
with the `ID` of one still shown replaces it. `Group` gathers notifications
in one stack of Notification Center, such as the messages of a
conversation (macOS):

```go
// IDs are "<conversation>/<message>".
mygo.App.OnNotificationClick(func(id string) {
	conversation, _, _ := strings.Cut(id, "/")
	openConversation(conversation)
})

mygo.NewNotification(mygo.NotificationOptions{
	ID:    "ada/" + msg.ID,
	Group: "ada",
	Title: "Ada",
	Body:  msg.Text,
}).Show()
```

## Clipboard

```go
mygo.Clipboard.WriteText("hello")
text := mygo.Clipboard.ReadText()

mygo.Clipboard.WriteHTML("<b>hello</b>")
mygo.Clipboard.WriteImage(png) // a PNG
img := mygo.Clipboard.ReadImage()

formats := mygo.Clipboard.AvailableFormats()
mygo.Clipboard.Clear()
```

Pages can use the web's `navigator.clipboard` too.

`Clipboard.Write(transfer.Data)` and `Clipboard.Read(formats...)` share the
native drag/drop model: ordered items, alternative representations, file
lists, custom MIME formats, and lazy providers. `Formats` discovers portable
names without requesting bytes; `ReadFormat`, `ReadFiles`, `WriteFiles`, and
`Flush` provide explicit negotiation, file transfer, and persistence. See
[Clipboard and drag data](data-transfer.md) for provider lifetimes and native
format mappings. The [clipboard example](../examples/clipboard) demonstrates
these operations.

## The shell

```go
mygo.Shell.OpenExternal("https://example.com") // the default browser
mygo.Shell.OpenPath("/Users/ada/report.pdf")   // the default app for the file
mygo.Shell.ShowItemInFolder(path)              // Finder, Explorer, the file manager
mygo.Shell.TrashItem(path)                     // to the trash
mygo.Shell.Beep()
```

## Displays

`mygo.Screen` describes the connected displays, in device-independent
pixels with the origin at the top-left corner of the primary display:

```go
primary := mygo.Screen.PrimaryDisplay()
fmt.Println(primary.Bounds, primary.WorkArea, primary.ScaleFactor)

for _, d := range mygo.Screen.Displays() { /* ... */ }

cursor := mygo.Screen.CursorScreenPoint()
d := mygo.Screen.DisplayNearestPoint(cursor)

mygo.Screen.OnDisplaysChanged(func() { /* rearrange windows */ })
```

`WorkArea` leaves out the menu bar, the Dock and taskbars.
`DisplayMatching(rect)` returns the display a rectangle overlaps most, the
one a window is on.

## Dark mode

`mygo.Theme` is the light or dark appearance, which pages follow through
the `prefers-color-scheme` media query:

```go
dark := mygo.Theme.IsDark()
mygo.Theme.OnUpdated(func() { log.Println("dark:", mygo.Theme.IsDark()) })

mygo.Theme.SetSource(mygo.ThemeDark) // or ThemeLight, or ThemeSystem to follow the system
```

`SetSource` may be called before `App.Run`, for example with an appearance
the user saved in the app's preferences: the app then starts in it.

## Power

```go
// Keep the computer awake during a long task, and the display on with true.
release := mygo.Power.KeepAwake("Exporting the video", false)
defer release()

mygo.Power.OnSuspend(func() { pauseSync() })
mygo.Power.OnResume(func() { resumeSync() })
mygo.Power.OnLockScreen(func() { hideSecrets() })
mygo.Power.OnUnlockScreen(func() { /* ... */ })

onBattery := mygo.Power.IsOnBattery()
idle := mygo.Power.IdleTime() // since the last keyboard or mouse input
```

The reason given to `KeepAwake` shows where the system lists what keeps it
awake. On Linux, `OnLockScreen` is called when the screen saver starts,
which usually locks the screen, and `IdleTime` is 0 on desktops that do not
report it.

## Global shortcuts

Global shortcuts work while the app is in the background, for example to
bring it to the front:

```go
err := mygo.GlobalShortcut.Register("CmdOrCtrl+Shift+Space", func() {
	win.Show()
	win.Focus()
})
```

The function runs on the main thread. Shortcuts use the syntax of
[accelerators](menus.md#accelerators); `Unregister`, `UnregisterAll` and
`IsRegistered` manage them. `Register` fails when the shortcut is taken,
by the app or, where the system tells, by another app.

On Linux under X11 the app grabs the keys itself. Under Wayland, where apps
cannot, the desktop binds shortcuts for the app through the XDG desktop
portal, which KDE Plasma 6 and GNOME 48 and later provide: the desktop may
ask the user to confirm new shortcuts and lets them pick other keys, and it
needs the app installed, since it identifies apps by their desktop entry.
The shortcut works shortly after `Register` returns. Windows the function
shows or focuses take the focus with the key press's activation token where
the desktop provides one, as GNOME does.
