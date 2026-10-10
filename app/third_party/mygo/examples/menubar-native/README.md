# Native menu bar window

Run from the repository root:

```sh
go run ./examples/menubar-native
```

Click the note icon in the menu bar (macOS) or notification area (Windows)
to open **Quick Notes**, a frameless window drawn by package `ui`. Click
the icon again, click outside the window, press Escape, or use Hide to
dismiss it. The panel opens beside the icon and stays within its display's
work area. There is no Dock icon on macOS or taskbar button on Windows.

Type a note and use Copy note to put it on the clipboard. Hiding and
reopening the window preserves its text, selection, and undo history;
notes stay in memory until you quit. Quit is available in the window and
in the icon's right-click menu, which opens below the menu bar on macOS
with AppKit's status-item placement and highlighting. Cmd+W/Ctrl+W hides
the panel, and Cmd+Q/Ctrl+Q quits while it has focus.

Linux uses AppIndicator, which supports menus but no click events or icon
bounds. Choose **Open Quick Notes** from the tray menu to open the same
native window, centered on screen. Install `libayatana-appindicator3` if
`NewTray` reports it missing. Wayland compositors choose window positions.

The example needs Go alone, with no frontend build or webview. To develop
with live reload or package it:

```sh
go run ./cmd/mygo dev examples/menubar-native
go run ./cmd/mygo build examples/menubar-native
```

The macOS bundle declares `LSUIElement` so it starts as a menu bar app;
`ActivationPolicyAccessory` also handles running it with `go run`.
