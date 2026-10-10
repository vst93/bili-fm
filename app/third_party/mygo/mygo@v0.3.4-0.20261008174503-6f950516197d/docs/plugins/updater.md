# Updater

The updater plugin gives an app the update window that Mac users know from
Sparkle, on every platform: it checks for updates in the background, shows
the release notes of a new version and offers to install it, skip it or
remind the user later, then downloads it with a progress bar and relaunches
the app into it. It is all Go, with no npm package, and works in apps of
web pages and of [native UI](../ui/README.md) alike. Its window is a web page, or
native UI with [`native.Plugin`](#native-ui), for apps that show no web
page at all.

It is built on `mygo.Updater`, so the app must be built with updates: see
[auto-updates](../updates.md) to sign and publish them.

## Set up

Use the plugin in Go:

```go
import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
)

func main() {
	mygo.Use(updater.Plugin)
	// ...
}
```

- It checks in the background once a day, the first time 10 seconds after
  launch when a check is due. When a new version is out, a window shows its
  release notes and offers **Install Update**, **Remind Me Later** and
  **Skip This Version**. Background checks do not offer a skipped version
  again, but offer the next one.
- Installing downloads the update with a progress bar, then offers to
  relaunch; otherwise the update runs at the next launch.
- With the window's "Automatically download and install updates in the
  future" checked, background checks install updates without asking, and
  they run at the next launch.

Builds that cannot update themselves, such as development builds and apps
installed by a package manager, never check in the background, and the
window says why when the user checks.

## Checking from the menu

`updater.CheckForUpdates()` checks as the user asked: the window shows at
once, and says when the app is up to date or the check failed. It also
shows updates the user skipped. `updater.MenuItem()` is a "Check for
Updates…" item that calls it, which goes after About on macOS:

```go
mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
	{Label: "My App", Submenu: []*mygo.MenuItem{
		{Role: mygo.RoleAbout},
		updater.MenuItem(),
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	}},
	{Role: mygo.RoleEditMenu},
	{Role: mygo.RoleWindowMenu},
}))
```

A button of the app's page can call `CheckForUpdates` through a bound
method, too. While a check is under way, calling it again brings its
window to the front.

## Options

`updater.New` takes options instead of the defaults of `updater.Plugin`:

```go
mygo.Use(updater.New(updater.Options{
	Interval:               12 * time.Hour, // between checks (default a day)
	DisableAutomaticChecks: true,           // until SetAutomaticChecks(true)
	Icon:                   iconPNG,        // default: icon.png among the resources
}))
```

- `Interval` is the time between background checks, a day by default.
- `DisableAutomaticChecks` turns background checks off until
  `SetAutomaticChecks(true)`, for apps that ask the user first or only
  check from the menu.
- `Icon` is the PNG image of the window; by default, `icon.png` among the
  app's resources, the icon that `mygo build` uses, when there is one.
- `Language` and `Strings` choose and change the window's texts (see
  [languages](#languages)).

## Preferences

The user's choices are kept in `updater.json` in `PathUserData`.
`AutomaticChecks` and `AutomaticDownloads` read them and
`SetAutomaticChecks` and `SetAutomaticDownloads` change them, for a
preferences page; `LastCheck` returns when the app last checked.
`OnChange` calls a function on the main thread whenever one of them
changed, after a check or an answer in the update window as well, so the
page can show them again:

```go
// UpdatePreferences tells the preferences page what they are now.
var UpdatePreferences = mygo.NewEvent[Prefs]("update-preferences")

updater.OnChange(func() {
	UpdatePreferences.Broadcast(Prefs{
		AutomaticChecks:    updater.AutomaticChecks(),
		AutomaticDownloads: updater.AutomaticDownloads(),
		LastCheck:          updater.LastCheck(),
	})
})
```

Sparkle asks on the second launch whether to check automatically: apps
that want to ask set `DisableAutomaticChecks` and call `SetAutomaticChecks`
with the answer.

## Languages

The window speaks the user's language (`App.Locale`) when the plugin has
it: English, Chinese (Simplified and Traditional), Dutch, French, German,
Italian, Japanese, Korean, Polish, Portuguese (Brazil), Russian, Spanish,
Turkish and Ukrainian, with their decimal marks and units ("1,5 Mo").
Other languages get English. `Language` picks one, such as the language the
app itself shows, and `Strings` changes texts or adds languages, by language
tag; the fields an app leaves empty keep the plugin's texts:

```go
mygo.Use(updater.New(updater.Options{
	Language: settings.Language, // "" follows the system
	Strings: map[string]updater.Strings{
		"en": {Install: "Update Now"},
		"sv": {Title: "Programuppdatering", Install: "Installera uppdatering" /* … */},
	},
}))
```

Some texts are formats, such as `AvailableMessage`, whose arguments are the
app's name, the new version and the running one: indexed verbs (`%[2]s`)
let a language order them. `mygo.Use` fails when a format does not fit its
arguments. `go doc github.com/egoist/mygo/plugins/updater.Strings` lists
the texts and the arguments of the formats.

Languages written from right to left, such as Arabic and Hebrew, get a
mirrored window, and release notes take the direction of the language
they are written in.

## Native UI

The window of `updater.Plugin` is a web page, so it needs WebKitGTK on
Linux and the WebView2 Runtime on Windows 10 even in an app whose own
windows all show [native UI](../ui/README.md). Package `native` draws the same
window in native UI instead, and needs no webview:

```go
import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
)

func main() {
	mygo.Use(native.Plugin) // or native.New(updater.Options{...})
	// ...
}
```

It takes the same options and speaks the same languages, and everything
else is package `updater`'s: `updater.MenuItem()`,
`updater.CheckForUpdates()` and the preferences work as with
`updater.Plugin`. Use one of the two plugins only. The window looks the
same; its release notes are the same Markdown, with text to select and
links that open in the browser.

## Your own update UI

The window is one way to offer updates. For an interface of your own, such
as a banner in the app's page, use `mygo.Updater` instead of the plugin:
see [your own update UI](../updates.md#your-own-update-ui).
