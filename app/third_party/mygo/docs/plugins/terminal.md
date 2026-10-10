# Terminal

The terminal plugin is a terminal for apps of [native UI](../ui/README.md): a view
that runs the user's shell, or any program, in a pseudo-terminal, with the
terminal emulator of [Ghostty](https://ghostty.org), libghostty-vt. It is
all Go, with no npm package: the emulator is a native library that the
app loads at run time, with no cgo, and that `mygo build` and `mygo dev`
put into the app.

## Set up

Make a terminal, which starts its program, and show it in a window:

```go
import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func main() {
	mygo.App.WhenReady(func() {
		term, err := terminal.New(terminal.Options{})
		if err != nil {
			log.Fatal(err)
		}
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "Terminal",
			Content: ui.View(func(c *ui.Context) {
				terminal.View(c, term).Fill().AutoFocus()
			}),
		})
		win.OnClosed(func() { term.Close() })
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
```

The [terminal example](../../examples/terminal) shows this app. Its window
uses the shell's title and closes when the shell exits.

`terminal.View` is an element like any other: size it with `Fill` or
`Grow`, and put it next to other elements, as in a pane of an editor. Its
size sets the terminal's, in cells of its font, and a click gives it the
keyboard focus. A terminal shows in one view at a time. Its program starts
once a view first shows it, so that it draws its first screen at the
view's size, or half a second after `New` at 80×24 when no view does.

The terminal does what terminal apps do:

- colors (16, 256 and 24-bit), bold, italic, faint, inverse, strikethrough,
  overlines and underlines (single, double, curly, dotted, dashed, in their
  own colors);
- Unicode: wide characters, emoji and grapheme clusters take the cells of
  their width, as in Ghostty, and the system's fonts stand in for what the
  terminal's font lacks;
- box drawing, blocks and Powerline's separators drawn rather than taken
  from the font, so that the lines of full-screen programs join;
- a scrollback that reflows when the terminal resizes, and the alternate
  screen of full-screen programs;
- the keyboard as programs ask: legacy sequences, xterm's modifyOtherKeys
  or the Kitty keyboard protocol, and input methods, which compose at the
  cursor;
- mouse reports, focus reports, bracketed paste, synchronized output,
  hyperlinks (OSC 8), the title (OSC 0 and 2), the working directory
  (OSC 7) and copying to the clipboard (OSC 52).

## Using it

- **Selecting** with the pointer works as in Ghostty: a drag selects
  cells, a double click words and a triple click lines, and dragging past
  the top or the bottom scrolls. Option (Alt elsewhere) drags a rectangle.
  While a program takes the mouse, as editors do, Shift selects.
- **Copy and paste** are Command+C and Command+V on macOS, which the Edit
  menu's roles also send, and Control+Shift+C and Control+Shift+V
  elsewhere. Pastes are bracketed when the program asks, so that pasted
  lines do not run as commands. The context menu has Copy, Paste, Select
  All and Clear Scrollback.
- **Scrolling**: the wheel and the touchpad scroll the scrollback, as do
  Shift+Page Up and Shift+Page Down. Full-screen programs get the wheel as
  arrow keys, or as mouse reports when they take the mouse. Typing scrolls
  back to the bottom.
- **Links**: Command+click (Control+click elsewhere) opens a hyperlink a
  program printed (OSC 8).
- Shortcuts of the app come first: Command shortcuts on macOS are never
  sent to the program, and keys that the window or an element around the
  terminal handles with `Shortcut` go there.

## Options

```go
term, err := terminal.New(terminal.Options{
	Command:  []string{"htop"},          // the user's login shell when empty
	Dir:      "/tmp",                     // the home directory when empty
	Env:      []string{"EDITOR=vim"},     // added to the app's environment
	Font:     terminal.Font{Family: "JetBrains Mono", Size: 14},
	Theme:    terminal.DarkTheme(),        // follows the window when nil
	Cursor:   terminal.CursorBar,          // a block when zero
	OnTitle:  func(title string) { win.SetTitle(title) },
	OnExit:   func(code int) { win.Close() },
})
```

- `Command` runs a program instead of the shell. The shell runs as a login
  shell (`-zsh`), as terminal apps start it, so that it reads the user's
  profile: `$SHELL`, else `/bin/zsh` on macOS and `/bin/sh` elsewhere, and
  PowerShell on Windows.
- `Env` adds to the environment, which has `TERM=xterm-256color`,
  `COLORTERM=truecolor`, and `LANG=en_US.UTF-8` when no locale is set, as
  for apps started from the Finder.
- `Font` sets the font: the system's monospaced font at 13 DIPs by
  default (see [Fonts](#fonts)).
- `Theme` sets the colors: the foreground, the background, the cursor's
  and the text's under it, the selection's and the selected text's, and
  the palette of 16 colors; `DarkTheme`, when set too, sets those of dark
  windows (see [Themes](#themes)). `terminal.DarkTheme()` and
  `terminal.LightTheme()` are the window's background and text with Visual
  Studio Code's palette, and without a theme the terminal takes the one of
  the window's appearance as it changes. Programs may change the colors
  (OSC 4, 10, 11, 12).
- `Cursor` and `NoBlink` set the cursor, which programs may change.
- `Scrollback` is about how many bytes of output to keep above the
  screen: 10 MB by default, none when negative.
- `Transparent` leaves the background undrawn, as Ghostty's
  `background-opacity`, so that what is under the view shows through: the
  window's material, or an element's translucent background. Cells with a
  background color of their own still draw it.
- `OptionAsAlt` makes Option on macOS the Alt key of programs (Meta),
  instead of the key typing accented letters.
- `OnTitle`, `OnExit`, `OnBell` and `OnNotify` run on a goroutine of the
  terminal when the program sets the title, ends, rings the bell or asks
  for a desktop notification. `Done` is closed once the program exited,
  and `ExitCode` returns its code (-1 for a program that could not
  start); the terminal then shows "[Process exited]".

## Fonts

`terminal.Font` holds what Ghostty's font options set:

```go
term, err := terminal.New(terminal.Options{Font: terminal.Font{
	Family:     "JetBrains Mono, Menlo, monospace", // tried in order
	Size:       14,                                  // DIPs, 13 when zero
	Weight:     500,                                 // 400 when zero
	Features:   []string{"-calt", "+ss01"},          // as Ghostty's font-feature
	LineHeight: 1.2,                                 // as adjust-cell-height = 20%
	Thicken:    true,                                // as font-thicken, on macOS
}})
```

- `Family` is a list of families, as `ui.Element.Font` takes it: the
  system's monospaced font when empty. Characters the fonts lack come
  from the system's fallback fonts. A font the app carries, rather than
  one the user installed, is added with `ui.RegisterFont`, its styles
  under one family so that bold and italic text take them:

  ```go
  //go:embed fonts
  var fonts embed.FS

  for _, name := range []string{"Regular", "Bold", "Italic", "BoldItalic"} {
  	data, _ := fonts.ReadFile("fonts/JetBrainsMono-" + name + ".ttf")
  	ui.RegisterFont(data, "JetBrains Mono")
  }
  ```

- `Weight` sets how heavy text is, from 100 to 900, of the weights the
  family has; bold text is 300 heavier, at least 700.
- `Features` turn OpenType features on and off: `"ss01"` or `"+ss01"` on,
  `"calt=0"` or `"-calt"` off, `"cv05=2"` an alternate. Coding fonts make
  most of their ligatures with `calt`, the others with `liga`; a ligature
  spans the cells of its characters.
- `LineHeight` makes rows taller or shorter than the font's line height,
  with the text centered in them; box drawing still joins.
- `Thicken` draws text with a thicker stroke, with Core Text's font
  smoothing at its strongest whatever the colors and the system's
  setting, as Ghostty does. Without it, text is as AppKit draws it:
  smoothed more the lighter it is, while the user leaves smoothing on. It
  changes nothing on Linux and Windows, as in Ghostty.

`SetFont` changes the font of a running terminal; the grid then takes the
new size of its cells.

## Themes

The plugin carries the hundreds of themes Ghostty ships (from
[iTerm2-Color-Schemes](https://github.com/mbadolato/iTerm2-Color-Schemes)),
which `terminal.GhosttyTheme` returns by name, as Ghostty's `theme` option
takes it:

```go
mocha, err := terminal.GhosttyTheme("Catppuccin Mocha")
latte, err := terminal.GhosttyTheme("Catppuccin Latte")
term, err := terminal.New(terminal.Options{
	Theme:     latte, // in light windows
	DarkTheme: mocha, // in dark windows, as theme = light:…,dark:… in Ghostty
})
```

As in Ghostty, a theme file of the user's in Ghostty's themes directory
(`$XDG_CONFIG_HOME/ghostty/themes`, `~/.config/ghostty/themes` by default)
comes first, and an absolute path names a theme file.
`terminal.GhosttyThemes()` lists the names, the user's themes with
Ghostty's, for a picker, and `terminal.ParseGhosttyTheme` reads a theme in
Ghostty's format from bytes:

```
palette = 0=#45475a
palette = 1=#f38ba8
background = #1e1e2e
foreground = #cdd6f4
cursor-color = #f5e0dc
cursor-text = #1e1e2e
selection-background = #f5e0dc
selection-foreground = #1e1e2e
```

`SetTheme(theme, dark)` changes the themes of a running terminal.

## Methods

The terminal's methods are safe from any goroutine:

- `Send` sends bytes to the program as if typed, and `Paste` pastes text.
- `Feed` shows bytes as if the program wrote them, text and escape
  sequences.
- `Title`, `Dir` (the directory the shell reported), `Size` (in cells) and
  `Text` (the screen and its scrollback) read the terminal.
- `SetFont` and `SetTheme` change the font and the colors.
- `Resize` sets the size of a terminal no view shows, and `Snapshot`
  returns what a terminal shows as escape sequences: its scrollback and
  screen with their styles, the cursor and the modes programs set. Fed to
  a new terminal of the same size, it shows the same, as when a server
  keeps a session's screen for the windows that attach to it.
- `Close` hangs up the program, which gets SIGHUP, and frees the terminal.

## Without a program

`Conn` connects the terminal to anything that reads and writes, such as an
SSH session or a serial port, instead of a program: what the terminal reads
from it shows, and what is typed is written to it. A `Conn` with a method
`Resize(cols, rows int) error` is told the size of the terminal:

```go
term, err := terminal.New(terminal.Options{Conn: session})
```

A terminal whose `Conn` never sends anything shows what the app writes
with `Feed`, as a log with colors would.

## The library

libghostty-vt is a native library of Ghostty, built for each platform from
the version of Ghostty the plugin binds, and published with MyGo. The
plugin's package names those builds and their SHA-256 in
`mygo-plugin.json`, and the CLI puts the one of each platform into the
apps it builds, among their resources, as
`libghostty-vt.dylib`, `libghostty-vt.so` or `ghostty-vt.dll`: a macOS app
signs it with the app, and a universal app gets both architectures in one
file.

A program not built by the CLI, as under `go run` and `go test`, downloads
the library once into the user's cache (`<cache>/mygo/natives/`), where
the CLI keeps those it downloads, and checks its SHA-256. Packaged apps
never download it. `terminal.LibraryPath` returns the library a program
loads, and `$MYGO_GHOSTTY_VT` names another library.

<!-- repository-only:start -->

To build libghostty-vt from Ghostty's sources:

```sh
git clone https://github.com/ghostty-org/ghostty && cd ghostty
zig build -Demit-lib-vt -Doptimize=ReleaseFast
MYGO_GHOSTTY_VT=$PWD/zig-out/lib/libghostty-vt.dylib go run ./examples/terminal
```

<!-- repository-only:end -->

`terminal.Load` loads the library, which `New` does too: call it first to
report a missing library before showing a window.

The library runs on macOS 13 or later, as Ghostty does, so apps with a
terminal set `minimumSystemVersion` to 13.0 or later; on Linux with glibc
2.28 or later (Debian 10, Ubuntu 20.04), on x64 and arm64; and on Windows
10 1809 or later, whose ConPTY runs the program, on x64 and arm64.

## Testing

`ui.NewTester` runs a terminal view without a window: type into it with
`Type` and `Key`, and read what the program printed with `Text`:

```go
func TestShell(t *testing.T) {
	term, err := terminal.New(terminal.Options{Command: []string{"/bin/sh"}})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	tt := ui.NewTester(func(c *ui.Context) { terminal.View(c, term).Fill().AutoFocus() }, 640, 400)
	tt.Type("echo hello")
	tt.Key(0, ui.KeyEnter)
	for !strings.Contains(term.Text(), "\nhello") {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
}
```
