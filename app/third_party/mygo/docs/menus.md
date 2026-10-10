# Menus and the tray

Menus are built from templates of `*mygo.MenuItem` and used as the menu
bar, context menus, the Dock menu and the menus of tray icons.

## Templates

```go
menu := mygo.NewMenu([]*mygo.MenuItem{
	{Role: mygo.RoleAppMenu},
	{Label: "File", Submenu: []*mygo.MenuItem{
		{Label: "New Note", Accelerator: "CmdOrCtrl+N", Click: func(item *mygo.MenuItem, win *mygo.Window) {
			newNote()
		}},
		{Label: "Export…", Accelerator: "CmdOrCtrl+E", Click: exportNotes},
		mygo.Separator(),
		{Role: mygo.RoleClose},
	}},
	{Role: mygo.RoleEditMenu},
	{Label: "View", Submenu: []*mygo.MenuItem{
		{ID: "sidebar", Label: "Show Sidebar", Type: mygo.MenuItemCheckbox, Checked: true, Click: toggleSidebar},
		mygo.Separator(),
		{Label: "List", Type: mygo.MenuItemRadio, Checked: true},
		{Label: "Grid", Type: mygo.MenuItemRadio},
		mygo.Separator(),
		{Role: mygo.RoleToggleDevTools},
		{Role: mygo.RoleToggleFullScreen},
	}},
	{Role: mygo.RoleWindowMenu},
})
```

A `MenuItem` has:

- `Label`, and an `Accelerator`, its keyboard shortcut (see
  [accelerators](#accelerators));
- `Click`, called on the main thread when the item is chosen, with the item
  and, on Linux and Windows, the window whose menu bar or context menu was
  chosen. Other menus use the focused window, or nil;
- `Type`: `MenuItemNormal`, `MenuItemCheckbox`, `MenuItemRadio`,
  `MenuItemSeparator` or `MenuItemSubmenu`; items with a `Submenu` are
  submenus, others normal items unless set. Adjacent radio items form a
  group, in which checking one unchecks the others. Checkbox and radio
  items toggle `Checked` before `Click` runs;
- `Disabled`, `Hidden` and `ToolTip`;
- `ID`, to find the item with `Menu.ItemByID`;
- `Role`, a built-in behavior with its label and accelerator.

### Roles

Roles do what users expect of standard items, with their usual label and
shortcut, which `Label` and `Accelerator` override:

- editing: `RoleUndo`, `RoleRedo`, `RoleCut`, `RoleCopy`, `RolePaste`,
  `RoleDelete`, `RoleSelectAll`, and on macOS `RolePasteAndMatchStyle`;
- the page: `RoleReload`, `RoleForceReload`, `RoleToggleDevTools` (in a
  window of native UI, its [inspector](ui/inspector.md)),
  `RoleResetZoom`, `RoleZoomIn`, `RoleZoomOut`;
- the window: `RoleToggleFullScreen`, `RoleMinimize`, `RoleZoom`,
  `RoleClose`;
- the app: `RoleQuit`, and on macOS `RoleAbout`, `RoleHide`,
  `RoleHideOthers`, `RoleUnhide`, `RoleFront`, `RoleServices`,
  `RoleStartSpeaking`, `RoleStopSpeaking`;
- whole menus: `RoleFileMenu`, `RoleEditMenu`, `RoleViewMenu` and
  `RoleWindowMenu`, and on macOS `RoleAppMenu` (the menu named after the
  app), `RoleWindow` (the menu listing the windows) and `RoleHelp` (the
  menu with the search field).

Items with the roles of macOS are left out on other platforms, so one
template serves every platform.

### Changing items

Change items of a menu in use with their methods, so that the native menu
follows: `SetLabel`, `SetEnabled`, `SetVisible`, `SetChecked` and
`SetAccelerator`:

```go
item := menu.ItemByID("sidebar")
item.SetChecked(false)
```

`Menu.Append` and `Menu.Insert` add items.

## The menu bar

```go
mygo.App.SetMenu(menu)
```

On macOS the menu bar belongs to the app. Apps get a default one, with the
app, File, Edit, View and Window menus, until they set their own: keep
`RoleAppMenu` and `RoleEditMenu` in yours, which make Cmd+Q and copy and
paste work.

On Linux and Windows menu bars belong to windows. `App.SetMenu` gives its
menu to every window without one of its own, `Window.SetMenu` gives a
window its own, and windows have none by default. `App.SetMenu(nil)`
removes the menu.

A window with `AutoHideMenuBar` keeps its menu bar out of sight, and the
room for the page. Alt pressed alone, or F10, shows the bar with the
keyboard in it (Windows) or its first menu open (Linux), and the bar hides
again once the user picks an item or leaves it. Its shortcuts work all
along:

```go
mygo.NewWindow(mygo.WindowOptions{URL: "/", AutoHideMenuBar: true})
```

A Windows window without a title bar, frameless or with a hidden title
bar, has no room for a menu bar either: Alt and F10 open its menus in a
popup from the top-left corner, below the title bar the page draws. On
Linux the bar shows above the page.

A window in full screen leaves the whole screen to the page: its menu bar
hides until it leaves full screen. Alt and F10 still open its menus, in a
popup from the top-left corner on Windows, and in the bar, which shows
while they are open, on Linux.

## Context menus

`Menu.Popup` shows a menu at the mouse over a window and returns once it
closes; `PopupAt` shows it at a position in the page. Show one from a bound
method, which the page calls on right-click:

```go
// ShowNoteMenu shows the context menu of a note.
func (n *Notes) ShowNoteMenu(ctx context.Context, id int64) {
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Rename", Click: func(*mygo.MenuItem, *mygo.Window) { n.startRename(id) }},
		{Label: "Delete", Click: func(*mygo.MenuItem, *mygo.Window) { n.Delete(id) }},
	})
	menu.Popup(mygo.CallerWindow(ctx))
}
```

```ts
note.addEventListener("contextmenu", (e) => {
  e.preventDefault();
  Notes.showNoteMenu(id);
});
```

Without a handler of their own, pages show the webview's standard context
menu, with Copy and Paste on text and Inspect Element in development
builds. Windows showing [native UI](ui/context-menu.md) give elements context
menus with `ContextMenu`, built like the rest of their interface.

## The Dock menu

On macOS `App.Dock.SetMenu` adds items above the standard ones of the menu
that right-clicking the Dock icon opens. Like `App.SetMenu`, it may be
called in `main` before `App.Run`:

```go
mygo.App.Dock.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
	{Label: "New Window", Click: func(*mygo.MenuItem, *mygo.Window) { openMainWindow() }},
}))
```

## Tray icons

A tray icon sits in the menu bar on macOS and in the notification area on
Windows and Linux. Create one once the app is ready:

```go
//go:embed resources/tray.png
var trayIcon []byte

mygo.App.WhenReady(func() {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           trayIcon,
		IconIsTemplate: true, // macOS tints it to match the menu bar
		ToolTip:        "Notes",
		Menu: mygo.NewMenu([]*mygo.MenuItem{
			{Label: "Open Notes", Click: func(*mygo.MenuItem, *mygo.Window) { openMainWindow() }},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}),
	})
	if err != nil {
		log.Println("tray:", err)
	}
	_ = tray
})
```

The icon is a PNG, ideally 16×16 points (a 32×32 image for high-resolution
displays); a template icon, black on transparent, lets macOS tint it for
light and dark menu bars. A tray with a menu shows it on click. Without a
menu, clicks reach `OnClick`, and `OnRightClick` gets right-clicks. Change
the tray with `SetIcon`, `SetMenu`, `SetToolTip` and, on macOS,
`SetTitle`, the text next to the icon; `Destroy` removes it.

`Tray.PopUpMenu` opens the menu set with `SetMenu` programmatically. On
macOS, AppKit places it below the status item and highlights its button
while the menu is open. To show a menu only on right-click while keeping
primary clicks available, set the menu in `OnRightClick`, call
`PopUpMenu`, then clear it with `SetMenu(nil)` when the call returns.

On Linux tray icons use AppIndicator, which needs
`libayatana-appindicator3`: they show a menu and report no clicks, and
`NewTray` returns an error without the library.

Apps that live in the menu bar usually drop their Dock icon on macOS:

```go
mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
```

For an icon that opens a window of native UI, see
[the menu bar example](https://github.com/egoist/mygo/tree/main/examples/menubar-native).
It leaves the tray menu unset on macOS and Windows, uses `OnClick` to
toggle a frameless window positioned with `Tray.Bounds`, and hides it on
`OnBlur` or Escape. On Linux, an AppIndicator menu item opens the same
window.

## Accelerators

Accelerators are keys with modifiers joined by `+`, such as `CmdOrCtrl+N`
or `Alt+Shift+F4`, in any case.

Modifiers:

| Modifier | Key |
|---|---|
| `CmdOrCtrl` (`CommandOrControl`) | Command on macOS, Control elsewhere |
| `Cmd` (`Command`), `Super`, `Meta`, `Win` | Command on macOS, the Windows or Super key elsewhere |
| `Ctrl` (`Control`) | Control |
| `Alt` (`Option`, `AltGr`) | Alt, or Option on macOS |
| `Shift` | Shift |

Keys:

- a character: a letter, a digit or a sign such as `,` or `/` (`Plus` for
  `+`);
- `F1` to `F24`;
- `Enter` (`Return`), `Tab`, `Space`, `Backspace`, `Delete` (`Del`),
  `Insert` (`Ins`), `Escape` (`Esc`);
- `Up`, `Down`, `Left`, `Right`, `Home`, `End`, `PageUp`, `PageDown`;
- `Num0` to `Num9`, `NumDec`, `NumAdd`, `NumSub`, `NumMult`, `NumDiv` (the
  numeric keypad);
- `VolumeUp`, `VolumeDown`, `VolumeMute`, `MediaPlayPause`,
  `MediaNextTrack`, `MediaPreviousTrack`, `MediaStop`, `PrintScreen`,
  `CapsLock`, `NumLock`, `ScrollLock`.

The same syntax registers [global shortcuts](native.md#global-shortcuts).
