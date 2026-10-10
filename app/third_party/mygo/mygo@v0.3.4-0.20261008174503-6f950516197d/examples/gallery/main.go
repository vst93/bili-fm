// Gallery tours MyGo's own user interface toolkit: a window drawn on the
// GPU from Go, without a web page. It shows layout, the widgets, text
// editing, a list of ten thousand rows with context menus, a chat of
// messages of every height, styling (grids, borders, gradients, text
// decorations, motion), custom drawing, overlays, file drops and updates
// from other goroutines. Its pages are paths of a router, with a history.
//
//	go run ./examples/gallery
package main

import (
	"fmt"
	"log"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

type gallery struct {
	win *mygo.Window
	// router shows the pages: "/overview", "/list", and a row of the list,
	// "/list/42".
	router *ui.Router

	count    int
	agree    bool
	notify   bool
	size     string
	plan     string
	volume   float64
	name     string
	email    string
	bio      string
	filter   string
	picked   int
	starred  map[int]bool
	tab      int
	split    float32
	copies   float64
	file     int
	chosen   ui.Selection[int]
	tree     map[string]bool
	leaf     string
	birthday time.Time
	dialog   bool
	menu     bool
	files    []string
	now      time.Time
	samples  []float64
	period   int
	pinned   bool
	fruit    string
	// The toolbar's toggles and view.
	bold, italic, underline, inspector bool
	layout                             int
	// The search field, the combobox, the autocomplete and the tags.
	search, font, city string
	tags               []string
	eased              bool
	// The items of the Motion page's list, the next one's number, and
	// whether its panel is open.
	items     []motionItem
	nextItem  int
	panelOpen bool
	// The disclosure, the sections of the accordion, and their choices.
	advanced, verbose, share, news bool
	sections                       [3]bool
	// The files of the table, and how they are sorted.
	tableFiles []tableFile
	sort       ui.SortOrder
	// Which sections of the sidebar show.
	sectionsOpen [2]bool
	// The alert, the notes deleted with an undo, the find bar, the
	// notifications' check boxes, and the path.
	alert, finding                             bool
	notes, findAt                              int
	findQuery                                  string
	notifyMail, notifyCalendar, notifyMessages bool
	pathAt, pathDepth                          int
	// The Glass page's style, what is under its toolbar (a bar of glass,
	// a soft or hard scroll edge, or a progressive blur), and where its
	// lens is.
	glassStyle, glassEdge int
	lens                  [2]float32
	// The meeting's day and time, and the tint of its text.
	meeting time.Time
	tint    ui.Color
	// The indicators' values: the rating, the battery, the quality and
	// the range of prices.
	stars               int
	battery, quality    float64
	priceLow, priceHigh float64
	// The grid of swatches, the one last chosen, those chosen, and their
	// order; the tasks dragged between two columns.
	swatches       ui.GridState
	swatch         int
	swatchesChosen ui.Selection[int]
	swatchOrder    []int
	tasks          []galleryTask
	// The outline, its row chosen, and its order.
	outline     ui.OutlineState[string]
	outlineRow  int
	outlineSort ui.SortOrder
	// rows, chat and table keep the places of the lists of the List page.
	rows     ui.ListState
	chat     ui.ListState
	table    ui.ListState
	messages []message
	draft    string
}

// galleryTask is a task of the Drag and drop card.
type galleryTask struct {
	name string
	done bool
}

// reorder moves items of order before to, as GridState.Reorder asks.
func reorder(order, items []int, to int) []int {
	moved := make([]int, 0, len(items))
	rest := make([]int, 0, len(order))
	at := -1
	for i, v := range order {
		if i == to {
			at = len(rest)
		}
		if slices.Contains(items, i) {
			moved = append(moved, v)
		} else {
			rest = append(rest, v)
		}
	}
	if at < 0 {
		at = len(rest)
	}
	return slices.Insert(rest, at, moved...)
}

// hsl returns the color of hue h in degrees, saturation s and lightness l.
func hsl(h, s, l float64) ui.Color {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	return ui.RGB(uint8((r+m)*255), uint8((g+m)*255), uint8((b+m)*255))
}

// tableFile is a file of the table on the List page.
type tableFile struct {
	id         int
	name, kind string
	size       int
}

// sortedFiles returns the table's files in the order it is sorted by.
func (g *gallery) sortedFiles() []*tableFile {
	if g.tableFiles == nil {
		for i, name := range []string{"report.pdf", "photo.jpg", "notes.md", "budget.xlsx", "slides.key", "song.mp3", "archive.zip", "logo.svg"} {
			kind := map[string]string{"pdf": "PDF document", "jpg": "JPEG image", "md": "Markdown", "xlsx": "Spreadsheet", "key": "Presentation", "mp3": "MP3 audio", "zip": "ZIP archive", "svg": "SVG image"}[name[strings.LastIndexByte(name, '.')+1:]]
			g.tableFiles = append(g.tableFiles, tableFile{id: i, name: name, kind: kind, size: (i*37%11 + 1) * 173})
		}
	}
	rows := make([]*tableFile, len(g.tableFiles))
	for i := range g.tableFiles {
		rows[i] = &g.tableFiles[i]
	}
	slices.SortStableFunc(rows, func(a, b *tableFile) int {
		var d int
		switch g.sort.Column {
		case "Name":
			d = strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
		case "Kind":
			d = strings.Compare(a.kind, b.kind)
		case "Size":
			d = a.size - b.size
		}
		if g.sort.Descending {
			d = -d
		}
		return d
	})
	return rows
}

// message is a message of the chat on the List page; ids grow with time,
// below zero for those loaded later from before.
type message struct {
	id   int
	text string
	mine bool
}

// day returns the day of a message, 12 a day, 0 for today's.
func (m message) day() int {
	if m.id < 0 {
		return (m.id-11)/12 - 16
	}
	return m.id/12 - 16
}

// chatLines are what the messages of the chat say.
var chatLines = []string{
	"Did the list keep its place?",
	"Yes.",
	"It measures each message as it shows, and estimates the others from those it measured.",
	"So a message of ten lines and one of a single word scroll the same.",
	"Load the older ones: the messages in view stay where they are.",
	"Nice!",
	"Scroll up a little and the date of the day you are in stays at the top, until the next day pushes it away.",
	"And at the end, new messages keep coming into view, unless you scrolled up to read.",
	"Then a button takes you back to the latest.",
	"What about a million messages?",
	"Its offsets are float64, so they scroll by fractions of a DIP all the same.",
	"👍",
	"The rows it builds are only those in view, and a few beyond for Tab.",
	"Lunch?",
	"Sure, in ten minutes.",
}

// makeMessage makes up message id, of one to three lines of the chat.
func makeMessage(id int) message {
	r := uint32(id) * 2654435761
	var lines []string
	for k := range 1 + int((r>>3)%5)/2 {
		lines = append(lines, chatLines[(int(r>>9)+k*4)%len(chatLines)])
	}
	return message{id: id, text: strings.Join(lines, " "), mine: r%3 == 0}
}

var pages = []string{"Overview", "Controls", "Text", "List", "Styling", "Drawing", "Glass", "Overlays", "Motion"}

// icon parses the shapes of a 24×24 stroked icon, drawn in currentColor
// as icon sets draw them.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

// The icons of the pages, and two for buttons.
var (
	pageIcons = map[string]*ui.SVG{
		"Overview": icon(`<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>`),
		"Controls": icon(`<path d="M4 7h10M18 7h2M4 17h2M10 17h10"/><circle cx="16" cy="7" r="2"/><circle cx="8" cy="17" r="2"/>`),
		"Text":     icon(`<path d="M5 6V5h14v1M12 5v14M9 19h6"/>`),
		"List":     icon(`<path d="M9 6h11M9 12h11M9 18h11M4 6h.01M4 12h.01M4 18h.01"/>`),
		"Styling":  icon(`<path d="M12 21a9 9 0 1 1 9-9c0 2.5-2 3.5-3.5 3.5H16a2 2 0 0 0-1.5 3.3c.4.5.4 2.2-2.5 2.2z"/><circle cx="7.5" cy="11" r="1"/><circle cx="11" cy="7" r="1"/><circle cx="16" cy="8.5" r="1"/>`),
		"Drawing":  icon(`<path d="M15 5l4 4M4 20l1-4.5L16.5 4a2.1 2.1 0 0 1 3 3L8 18.5z"/>`),
		"Glass":    icon(`<rect x="3" y="6" width="18" height="12" rx="6"/><path d="M7 10.5a3 3 0 0 1 2.5-1.5"/>`),
		"Overlays": icon(`<path d="M12 3 3 8l9 5 9-5z"/><path d="m3 13 9 5 9-5"/>`),
		"Motion":   icon(`<path d="M3 12h4M5 7h6M5 17h6"/><circle cx="16" cy="12" r="5"/>`),
	}
	starIcon    = icon(`<path d="M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1-5.4-2.9-5.4 2.9 1-6.1L3.2 9.5l6.1-.9z"/>`)
	checkIcon   = icon(`<path d="M20 6 9 17l-5-5"/>`)
	chevronIcon = icon(`<path d="m6 9 6 6 6-6"/>`)
	loaderIcon  = icon(`<path d="M21 12a9 9 0 1 1-6.2-8.6"/>`)
	// A picture in its own colors: gradients, a clip path, and a dot in
	// currentColor.
	badge = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120">
	<defs>
		<linearGradient id="bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#6366f1"/><stop offset="1" stop-color="#ec4899"/></linearGradient>
		<radialGradient id="glow" cx=".35" cy=".3" r=".6"><stop offset="0" stop-color="#fff" stop-opacity=".55"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient>
		<clipPath id="round"><rect width="120" height="120" rx="28"/></clipPath>
	</defs>
	<g clip-path="url(#round)"><rect width="120" height="120" fill="url(#bg)"/><circle cx="42" cy="36" r="60" fill="url(#glow)"/></g>
	<path d="M34 78 60 34l26 44z" fill="none" stroke="#fff" stroke-width="9" stroke-linejoin="round"/>
	<circle cx="60" cy="64" r="7" fill="currentColor"/>
</svg>`))
)

// pagePath returns the path of a page, and pageOf the page of a path.
func pagePath(page string) string { return "/" + strings.ToLower(page) }

func pageOf(path string) string {
	for _, p := range pages {
		if path == pagePath(p) || strings.HasPrefix(path, pagePath(p)+"/") {
			return p
		}
	}
	return ""
}

func (g *gallery) view(c *ui.Context) {
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		g.sidebar(c)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			g.toolbar(c)
			// The pages, which keep their place in the history: back on one,
			// it is scrolled where it was.
			g.router.View(c, func(r *ui.Route) {
				ui.Scroll(c).Grow(1).Padding(12, 32, 28).Gap(18).Children(func() {
					if r.Match("/list/{row}") {
						g.listRow(c, r)
						return
					}
					page := pageOf(r.Path())
					if page == "" {
						r.Title("Not found")
						ui.Text(c, "Not found").FontSize(26).Bold()
						return
					}
					r.Title(page)
					ui.Text(c, page).FontSize(26).Bold()
					switch page {
					case "Overview":
						g.overview(c)
					case "Controls":
						g.controls(c)
					case "Text":
						g.text(c)
					case "List":
						g.list(c)
					case "Styling":
						g.styling(c)
					case "Drawing":
						g.drawing(c)
					case "Glass":
						g.glassPage(c)
					case "Overlays":
						g.overlays(c)
					case "Motion":
						g.motion(c)
					}
				})
			})
		})
	})
	// Ctrl+1…7 (Cmd on macOS) switch pages.
	for i, p := range pages {
		if c.Shortcut(ui.Cmd, ui.Key1+ui.Key(i)) {
			g.router.Push(pagePath(p))
		}
	}
}

// toolbar goes back and forward in the history of the pages, and shows
// the path of the page: Cmd+[ and Cmd+] on macOS, Alt+Left and Alt+Right
// elsewhere, and a mouse's side buttons, go back and forward too.
func (g *gallery) toolbar(c *ui.Context) {
	ui.Toolbar(c, func() {
		ui.BackButton(c, g.router)
		ui.ForwardButton(c, g.router)
		page := pageOf(g.router.Path())
		path := []string{page}
		if row := strings.TrimPrefix(g.router.Path(), pagePath(page)+"/"); row != g.router.Path() {
			path = append(path, "Row "+row)
		}
		chosen := -1
		if ui.Breadcrumbs(c, path, &chosen).Label("Path").Changed() {
			g.router.Push(pagePath(page))
		}
	}).Label("Navigation").Padding(8, 24, 0)
}

// listRow shows a row of the list, from the List page: the arrows in the
// toolbar, or the side buttons of a mouse, go back to the list as it was.
func (g *gallery) listRow(c *ui.Context, r *ui.Route) {
	t := c.Theme()
	n, err := strconv.Atoi(r.Param("row"))
	if err != nil || n < 0 || n >= 10000 {
		r.Title("Not found")
		ui.Text(c, "Not found").FontSize(26).Bold()
		return
	}
	r.Title(fmt.Sprintf("Row %d", n))
	ui.Text(c, fmt.Sprintf("Row %d", n)).FontSize(26).Bold()
	card(c, "", func() {
		for _, f := range []struct {
			label string
			value int
		}{{"Square", n * n}, {"Cube", n * n * n}} {
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, f.label).TextColor(t.TextMuted).Width(80)
				ui.Textf(c, "%d", f.value).Font("monospace")
			})
		}
		starred := g.starred[n]
		if ui.Checkbox(c, &starred, "Starred").Changed() {
			g.starred[n] = starred
		}
	}).MaxWidth(420)
	// Links to paths go there in the router, relative to the page as on
	// the web.
	ui.Row(c).Gap(16).Children(func() {
		if n > 0 {
			ui.Link(c, "← Previous row", strconv.Itoa(n-1))
		}
		if n < 9999 {
			ui.Link(c, "Next row →", strconv.Itoa(n+1))
		}
		ui.Link(c, "All rows", "/list")
	})
}

func (g *gallery) sidebar(c *ui.Context) {
	t := c.Theme()
	side := ui.Column(c).Width(200).PaddingY(16).Background(t.Surface).Shrink(0)
	side.Children(func() {
		ui.Text(c, "MyGo UI").FontSize(13).Bold().TextColor(t.TextMuted).Padding(4, 20, 6)
		// The pages, in two sections that hide and show; the arrows choose
		// among them while the sidebar has the focus.
		page := pageOf(g.router.Path())
		if ui.Sidebar(c, &page, func() {
			for k, section := range []struct {
				title string
				pages []string
			}{{"Widgets", pages[:4]}, {"Look", pages[4:]}} {
				ui.SidebarSection(c, section.title, &g.sectionsOpen[k], func() {
					for _, p := range section.pages {
						item := ui.SidebarItem(c, p, pageIcons[p], p)
						if p == "List" {
							item.Children(func() { ui.Badge(c, "10k") })
						}
					}
				})
			}
		}).Grow(1).Label("Pages").Changed() {
			g.router.Push(pagePath(page))
		}
		ui.Text(c, g.now.Format("15:04:05")).FontSize(12).TextColor(t.TextMuted).Padding(0, 20)
	})
}

func card(c *ui.Context, title string, body func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).Padding(18).Gap(12).Radius(10).Background(t.Background).Border(1, t.Border).
		Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.06)).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(15).Bold()
		}
		body()
	})
}

// wideColors draws what each kind of color paints (fills, gradients,
// stripes, text, shadows) in the sRGB color nearest to an Oklch one, on the
// left, and in the Oklch color itself, on the right. Where the window draws
// a wide gamut (a Display P3 screen on macOS) the right column is the more
// vivid; elsewhere the two match.
func wideColors(c *ui.Context) {
	t := c.Theme()
	vivid, warm := ui.Oklch(0.85, 0.3, 145), ui.Oklch(0.7, 0.3, 30)
	black := ui.RGB(0, 0, 0)
	ui.Text(c, "Compare the columns on a Display P3 screen: the right one is more vivid.").FontSize(12).TextColor(t.TextMuted)

	// pair builds a sample twice, in sRGB and then in Oklch, in equal columns.
	pair := func(sample func(wide bool) ui.Element) {
		ui.Row(c).Gap(12).Children(func() {
			for _, wide := range []bool{false, true} {
				sample(wide).Basis(0).Grow(1)
			}
		})
	}
	box := func(label string) ui.Element {
		return ui.Row(c).Height(30).Radius(6).Center().Children(func() {
			ui.Text(c, label).FontSize(12).Bold().TextColor(black)
		})
	}

	pair(func(wide bool) ui.Element {
		if wide {
			return ui.Text(c, "Oklch").FontSize(12).Bold().TextColor(t.TextMuted)
		}
		return ui.Text(c, "sRGB").FontSize(12).Bold().TextColor(t.TextMuted)
	})
	pair(func(wide bool) ui.Element {
		if wide {
			return box("Fill").Background(vivid)
		}
		return box("Fill").Background(vivid.SRGB())
	})
	pair(func(wide bool) ui.Element {
		if wide {
			return box("Gradient").LinearGradient(ui.LinearGradient{From: warm, To: vivid, Angle: 90, Oklab: true})
		}
		return box("Gradient").LinearGradient(ui.LinearGradient{From: warm.SRGB(), To: vivid.SRGB(), Angle: 90, Oklab: true})
	})
	pair(func(wide bool) ui.Element {
		white := ui.RGB(255, 255, 255)
		if wide {
			return box("Stripes").Background(white).Stripes(vivid, 4, 6, 45)
		}
		return box("Stripes").Background(white).Stripes(vivid.SRGB(), 4, 6, 45)
	})
	pair(func(wide bool) ui.Element {
		return ui.Row(c).Height(30).Radius(6).Center().Background(ui.RGB(20, 20, 20)).Children(func() {
			text := ui.Text(c, "Text").FontSize(14).Bold()
			if wide {
				text.TextColor(vivid)
			} else {
				text.TextColor(vivid.SRGB())
			}
		})
	})
	pair(func(wide bool) ui.Element {
		e := box("Shadow").Margin(6).Background(ui.RGB(255, 255, 255))
		if wide {
			return e.Shadow(0, 4, 12, 0, vivid)
		}
		return e.Shadow(0, 4, 12, 0, vivid.SRGB())
	})
}

func (g *gallery) overview(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "Everything here is laid out with flexbox and grids and drawn by MyGo itself: no HTML, no JavaScript, no cgo. "+
		"The view is a Go function of the app's state that runs again after every event.").TextColor(t.TextMuted)
	ui.Row(c).Gap(16).Wrap().AlignItems(ui.Start).Children(func() {
		card(c, "Counter", func() {
			ui.Text(c, fmt.Sprint(g.count)).FontSize(40).Bold()
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "−").Width(44).Clicked() {
					g.count--
				}
				if ui.PrimaryButton(c, "Increment").Clicked() {
					g.count++
				}
			})
		})
		card(c, "Live data", func() {
			ui.Text(c, "A goroutine pushes a sample every second with Window.Update.").TextColor(t.TextMuted).MaxWidth(260)
			g.sparkline(c).Size(260, 80)
		})
		card(c, "Files", func() {
			zone := ui.Column(c).Size(260, 80).Padding(8, 12).Gap(2).Radius(6).Background(t.Surface).
				Border(1, t.Border).Justify(ui.Center).AlignItems(ui.Center)
			if files := zone.DroppedFiles(); files != nil {
				g.files = files
			}
			if zone.FileDragOver() {
				zone.Border(2, t.Accent)
			}
			zone.Children(func() {
				if len(g.files) == 0 {
					ui.Text(c, "Drop files here").TextColor(t.TextMuted)
				}
				for i, f := range g.files {
					if i == 3 {
						ui.Text(c, fmt.Sprintf("and %d more", len(g.files)-i)).FontSize(12).TextColor(t.TextMuted)
						break
					}
					ui.Text(c, filepath.Base(f)).FontSize(12).MaxLines(1)
				}
			})
		})
	})
}

func (g *gallery) sparkline(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Box(c).Radius(6).Background(t.Surface).Draw(func(p *ui.Painter, r ui.Rect) {
		if len(g.samples) < 2 {
			return
		}
		var path ui.Path
		for i, v := range g.samples {
			x := r.X + 6 + float32(i)/float32(len(g.samples)-1)*(r.W-12)
			y := r.Y + r.H - 6 - float32(v)*(r.H-12)
			if i == 0 {
				path.MoveTo(x, y)
			} else {
				path.LineTo(x, y)
			}
		}
		p.StrokePath(&path, 2, t.Accent)
	})
}

func (g *gallery) controls(c *ui.Context) {
	t := c.Theme()
	card(c, "Choices", func() {
		ui.Checkbox(c, &g.agree, "I agree to the terms")
		ui.Row(c).Gap(10).Children(func() {
			ui.Switch(c, &g.notify).Label("Notifications")
			ui.Text(c, map[bool]string{true: "Notifications on", false: "Notifications off"}[g.notify])
		})
		ui.RadioGroup(c, func() {
			for _, p := range []string{"Free", "Pro", "Team"} {
				ui.Radio(c, &g.plan, p, p)
			}
		}).Row().Gap(18).Label("Plan")
		ui.Row(c).Gap(10).Children(func() {
			ui.Text(c, "Size")
			ui.Select(c, &g.size, []string{"Small", "Medium", "Large", "Extra large"})
		})
	})
	card(c, "Search and choose", func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.SearchField(c, &g.search).Label("Search").Width(220)
			ui.Combobox(c, &g.font, []string{"Avenir", "Courier", "Futura", "Georgia", "Gill Sans", "Helvetica", "Menlo", "Optima", "Palatino", "Times"}).Label("Font").Width(200)
			ui.Autocomplete(c, &g.city, []string{"Amsterdam", "Berlin", "Lisbon", "London", "Madrid", "Paris", "Prague", "Rome", "Vienna"}).Label("City").Placeholder("City").Width(200)
		})
		ui.TokenField(c, &g.tags, []string{"design", "go", "native", "performance", "release", "typescript"}).Label("Tags")
		ui.Text(c, "Type to filter; Up and Down move, Enter chooses. In the tags, Enter or a comma adds a tag, and Backspace takes out the last.").TextColor(t.TextMuted)
	})
	card(c, "Drag and drop", func() {
		if g.tasks == nil {
			g.tasks = []galleryTask{{"Write the docs", false}, {"Fix the layout", false}, {"Ship it", false}, {"Plan the release", true}}
		}
		ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
			for col, title := range []string{"To do", "Done"} {
				done := col == 1
				bin := ui.Column(c).Grow(1).Basis(0).Gap(6).Padding(10).Radius(8).MinHeight(150).Background(t.Surface).Border(1, t.Border).Label(title)
				if task, ok := ui.Drop[*galleryTask](bin); ok {
					task.done = done
				}
				if task, ok := ui.DragOver[*galleryTask](bin); ok && task.done != done {
					bin.Border(2, t.Accent)
				}
				bin.Children(func() {
					ui.Text(c, title).FontWeight(600)
					for i := range g.tasks {
						if task := &g.tasks[i]; task.done == done {
							ui.Row(c.Key(task.name)).Padding(7, 10).Radius(6).Background(t.Background).Border(1, t.Border).Cursor(ui.CursorPointer).Drag(task).Children(func() {
								ui.Text(c, task.name)
							})
						}
					}
				})
			}
		})
		ui.Text(c, "Drag a task to the other column; Escape gives up.").FontSize(12).TextColor(t.TextMuted)
	})
	card(c, "Groups and paths", func() {
		ui.CheckboxGroup(c, "Notifications", func() {
			ui.Checkbox(c, &g.notifyMail, "Mail")
			ui.Checkbox(c, &g.notifyCalendar, "Calendar")
			ui.Checkbox(c, &g.notifyMessages, "Messages")
		})
		path := []string{"Macintosh HD", "Users", "ada", "Documents", "Reports"}
		if ui.Breadcrumbs(c, path[:g.pathDepth+1], &g.pathAt).Label("Path").Changed() {
			g.pathDepth = g.pathAt
		}
		if g.pathDepth < len(path)-1 && ui.Button(c, "Open "+path[g.pathDepth+1]).Clicked() {
			g.pathDepth++
		}
	})
	card(c, "Disclosure", func() {
		ui.Collapsible(c, "Advanced options", &g.advanced, func() {
			ui.Checkbox(c, &g.verbose, "Verbose logging")
			ui.Text(c, "The arrow turns and the content grows into view, at once where the desktop asks for less motion.").TextColor(t.TextMuted)
		})
		ui.Accordion(c, func() {
			ui.AccordionItem(c, "General", &g.sections[0], func() {
				ui.Text(c, "Startup, appearance and updates.")
			})
			ui.AccordionItem(c, "Privacy", &g.sections[1], func() {
				ui.Checkbox(c, &g.share, "Share usage data")
			})
			ui.AccordionItem(c, "Keyboard", &g.sections[2], func() {
				ui.Text(c, "Up and Down move between the headers, as do Home and End; Enter and Space open and close them.").TextColor(t.TextMuted)
			})
		})
	})
	card(c, "Ranges", func() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Slider(c, &g.volume, 0, 100).Label("Volume").Grow(1)
			ui.Textf(c, "%3.0f%%", g.volume).Width(48).TextAlign(ui.End)
		})
		ui.Progress(c, g.volume/100)
		ui.Progress(c, -1)
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Copies")
			ui.NumberInput(c, &g.copies, 1, 99, 1).Label("Copies")
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Quality").Width(64)
			ui.StepSlider(c, &g.quality, 0, 100, 25).Label("Quality").Grow(1)
			ui.Textf(c, "%3.0f%%", g.quality).Width(48).TextAlign(ui.End)
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Price").Width(64)
			ui.RangeSlider(c, &g.priceLow, &g.priceHigh, 0, 500, 10).Label("Price").Grow(1)
			ui.Textf(c, "$%.0f–%.0f", g.priceLow, g.priceHigh).Width(72).TextAlign(ui.End)
		})
	})
	card(c, "Dates, times and colors", func() {
		ui.Row(c).Gap(20).AlignItems(ui.Start).Wrap().Children(func() {
			ui.Calendar(c, &g.meeting).Label("Meeting")
			ui.Column(c).Gap(12).Children(func() {
				ui.Form(c, func() {
					ui.Field(c, "Time", func() { ui.TimeInput(c, &g.meeting).Label("Meeting") })
					ui.Field(c, "Tint", func() { ui.ColorWell(c, &g.tint) })
				})
				ui.Text(c, g.meeting.Format("Monday, January 2 at 15:04")).TextColor(g.tint)
			})
		})
	})
	card(c, "Indicators", func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Label("Syncing")
			ui.Text(c, "Syncing…").TextColor(t.TextMuted).Grow(1)
			ui.Rating(c, &g.stars, 5).Label("Rating")
		})
		disk := 412.0
		ui.Textf(c, "Disk: %.0f of 500 GB", disk).FontSize(12).TextColor(t.TextMuted)
		ui.Meter(c, disk, 0, 500, &ui.MeterLevels{Warning: 400, Critical: 475}).Label("Disk")
		ui.Textf(c, "Battery: %.0f%%", g.battery).FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Meter(c, g.battery, 0, 100, &ui.MeterLevels{Warning: 20, Critical: 10}).Label("Battery").Grow(1)
			ui.Stepper(c, &g.battery, 0, 100, 5).Label("Battery")
		})
		ui.Row(c).Gap(8).Children(func() {
			for _, name := range []string{"Ada Lovelace", "Grace Hopper", "Alan Turing", "Margaret Hamilton"} {
				ui.Avatar(c, name, nil).Tooltip(name)
			}
		})
	})
	card(c, "Buttons", func() {
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if ui.PrimaryButton(c, "Save").Clicked() {
				c.Toast("Saved")
			}
			ui.Button(c, "Cancel")
			ui.Button(c, "Disabled").Disabled(true)
			ui.MenuButton(c, "Export", func(m *ui.Menu) {
				for _, as := range []string{"PDF", "PNG", "SVG"} {
					if m.Item("As " + as).Chosen() {
						c.Toast("Exported as " + as)
					}
				}
			})
			ui.Link(c, "Open mygo.dev", "https://github.com/egoist/mygo")
		})
		ui.Text(c, "Tab moves the focus; Enter or Space presses the focused button.").TextColor(t.TextMuted)
	})
	card(c, "Toolbar", func() {
		ui.Toolbar(c, func() {
			ui.Button(c, "New")
			ui.Button(c, "Open")
			ui.ToggleGroup(c, func() {
				ui.Toggle(c, &g.bold, "Bold")
				ui.Toggle(c, &g.italic, "Italic")
				ui.Toggle(c, &g.underline, "Underline")
			}).Label("Style")
			ui.Segmented(c, &g.layout, "List", "Grid", "Columns").Label("View")
			ui.Spacer(c)
			ui.Toggle(c, &g.inspector, "Inspector")
		}).Label("Document").Border(1, t.Border).Radius(8)
		ui.Text(c, "Tab stops once in the toolbar, and the arrows move between its controls. Narrow the window: what does not fit goes into the » menu.").TextColor(t.TextMuted)
	})
	card(c, "Tabs and panes", func() {
		ui.Tabs(c, &g.tab, "Files", "Search", "History")
		ui.Split(c, &g.split, func() {
			ui.Column(c).Fill().Padding(10).Gap(6).Background(t.Surface).Children(func() {
				for _, name := range [][]string{{"main.go", "go.mod", "README.md"}, {"Results"}, {"Yesterday", "Last week"}}[g.tab] {
					ui.Text(c, name).SingleLine()
				}
			})
		}, func() {
			ui.Column(c).Fill().Padding(10).Children(func() {
				ui.Text(c, "Drag the divider, or focus it and press the arrows.").TextColor(t.TextMuted)
			})
		}).Height(140).Border(1, t.Border).Radius(t.Radius).Clip()
	})
	card(c, "Built on bases", func() {
		ui.Text(c, "Bases are the widgets without their look: the pointer, the keys, the focus and accessibility, styled here anew.").TextColor(t.TextMuted)
		ui.Row(c).Gap(16).Wrap().Children(func() {
			// A segmented control on TabsBase.
			tabs := ui.TabsBase(c, &g.period, 3)
			tabs.List.Padding(3).Radius(999).Background(t.Surface).Children(func() {
				for i, name := range []string{"Day", "Week", "Month"} {
					seg := tabs.Tab(i).Padding(5, 14).Radius(999)
					if i == g.period {
						seg.Background(t.Background).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.15))
					}
					seg.Children(func() { ui.Text(c, name) })
				}
			})
			// A pill that toggles, on SwitchBase.
			pill := ui.SwitchBase(c, &g.pinned).Gap(6).Padding(5, 12).Radius(999).Border(1, t.Border)
			if g.pinned {
				pill.Background(t.Accent).TextColor(t.AccentText).Border(1, t.Accent)
			}
			pill.Children(func() {
				ui.Icon(c, starIcon)
				ui.Text(c, "Starred")
			})
			// A select with check marks, on SelectBase.
			sel := ui.SelectBase(c, &g.fruit)
			sel.Trigger.Gap(6).Padding(6, 10).Radius(8).Border(1, t.Border).Children(func() {
				ui.Text(c, g.fruit)
				ui.Icon(c, chevronIcon).TextColor(t.TextMuted)
			})
			sel.Popup(func(panel ui.Element) {
				panel.Margin(4, 0, 0, 0).Padding(4).Radius(10).Background(t.Background).Border(1, t.Border)
				panel.Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.15))
				for _, fruit := range []string{"Apple", "Banana", "Cherry", "Durian"} {
					item := sel.Item(fruit).Gap(8).Padding(6, 10).Radius(6)
					if item.Highlighted() {
						item.Background(t.Accent).TextColor(t.AccentText)
					}
					item.Children(func() {
						check := ui.Icon(c, checkIcon)
						if fruit != g.fruit {
							check.Opacity(0)
						}
						ui.Text(c, fruit)
					})
				}
			})
		})
	})
}

func (g *gallery) text(c *ui.Context) {
	t := c.Theme()
	card(c, "Form", func() {
		ui.Form(c, func() {
			ui.Field(c, "Name", func() { ui.TextInput(c, &g.name).Placeholder("Ada Lovelace") })
			invalid := ""
			if g.email != "" && !strings.Contains(g.email, "@") {
				invalid = "Enter an email address, such as ada@example.com."
			}
			ui.Field(c, "Email", func() {
				if ui.TextInput(c, &g.email).Placeholder("ada@example.com").Submitted() {
					g.dialog = true
				}
			}).Description("Enter opens a dialog.").Error(invalid)
			ui.Field(c, "About you", func() {
				ui.TextArea(c, &g.bio).Placeholder("Multiple lines, with undo, selection and input methods.").Height(110)
			}).Description(fmt.Sprintf("%d characters", len([]rune(g.bio))))
			ui.Fieldset(c, "Optional", func() {
				ui.Field(c, "Birthday", func() { ui.DateInput(c, &g.birthday) })
				ui.Field(c, "Updates", func() { ui.Checkbox(c, &g.news, "Send me the newsletter") })
			})
		})
	})
	card(c, "Find", func() {
		text := "The quick brown fox jumps over the lazy dog. The dog sleeps; the fox runs on."
		matches := strings.Count(strings.ToLower(text), strings.ToLower(g.findQuery))
		if g.findQuery == "" {
			matches = 0
		}
		if c.Shortcut(ui.Cmd, ui.KeyF) {
			g.finding = true
		}
		if !g.finding && ui.Button(c, "Find…").Clicked() {
			g.finding = true
		}
		ui.FindBar(c, &g.finding, &g.findQuery, matches, &g.findAt).Radius(8)
		ui.Text(c, text)
	})
	card(c, "Typography", func() {
		ui.Text(c, "Display 28").FontSize(28).Bold()
		ui.Text(c, "Italic, underlined and struck through").Italic().Underline().Strikethrough()
		ui.RichText(c,
			ui.Span{Text: "Rich text mixes "}, ui.Span{Text: "bold", Weight: 700}, ui.Span{Text: ", "},
			ui.Span{Text: "colored", Color: t.Accent}, ui.Span{Text: ", "}, ui.Span{Text: "large", Size: 20},
			ui.Span{Text: " and "}, ui.Span{Text: "underlined", Underline: true}, ui.Span{Text: " runs in one paragraph."},
		)
		ui.Text(c, "Monospace: func main() {}").Font("monospace")
		ui.Text(c, "SPACED CAPITALS").FontSize(12).Bold().LetterSpacing(2).TextColor(t.TextMuted)
		ui.Text(c, "Tabular digits: 1,111.11 / 8,888.88").FontFeatures("tnum")
		ui.Text(c, "Mixed scripts: English, Ελληνικά, Русский, 日本語, 한국어, العربية, עברית, हिन्दी 🎉").Selectable()
		ui.Text(c, strings.Repeat("Long text wraps to the width it gets. ", 6)).TextColor(t.TextMuted)
		ui.Text(c, strings.Repeat("A single line that ends with an ellipsis when it does not fit. ", 4)).SingleLine()
	})
	card(c, "Text selection", func() {
		ui.Column(c).Selectable().Gap(12).Children(func() {
			ui.Text(c, "Drag from this paragraph into the next, then copy the selected text.")
			ui.RichText(c, ui.Span{Text: "Each paragraph keeps its own layout. "}, ui.Span{Text: "They share one selection.", Weight: 700})
			ui.Text(c, "This hint is excluded from selection.").Unselectable().TextColor(t.TextMuted)
		})
	})
}

func (g *gallery) list(c *ui.Context) {
	t := c.Theme()
	ui.TextInput(c, &g.filter).Placeholder("Filter 10,000 rows").Label("Filter")
	var rows []int
	for i := 0; i < 10000; i++ {
		if g.filter == "" || strings.Contains(fmt.Sprint(i), g.filter) {
			rows = append(rows, i)
		}
	}
	// The row of the number picked: the list chooses rows by their index,
	// which the filter changes.
	at := slices.Index(rows, g.picked)
	ui.Row(c).Gap(12).Children(func() {
		ui.Textf(c, "%d rows; only those in view are built. Pick one with a click or the arrows, open it with a double click or Enter; right-click one for its menu.", len(rows)).TextColor(t.TextMuted).Grow(1)
		if ui.Button(c, "Show picked").Disabled(at < 0).Clicked() {
			// The row may not be built: the list scrolls to it all the same.
			g.rows.ScrollTo(at, ui.Center)
		}
	})
	// A row opens in a page of its own, which slides in; back, the list is
	// as it was.
	open := func(n int) { g.router.Push(fmt.Sprintf("/list/%d", n)) }
	g.rows.Selected = &at
	g.rows.Key = func(i int) any { return rows[i] }
	list := ui.List(c, &g.rows, len(rows), func(i int) {
		n := rows[i]
		row := ui.Row(c).Height(32).PaddingX(12).Gap(10)
		row.ContextMenu(func(m *ui.Menu) {
			if m.Item("Open").Chosen() {
				open(n)
			}
			if m.Item("Pick").Chosen() {
				g.picked = n
			}
			if m.Item("Starred").Checked(g.starred[n]).Chosen() {
				g.starred[n] = !g.starred[n]
			}
			m.Separator()
			if m.Item("Copy Square").Chosen() {
				mygo.Clipboard.WriteText(fmt.Sprint(n * n))
			}
		})
		row.Children(func() {
			label := fmt.Sprintf("Row %d", n)
			if g.starred[n] {
				label += "  ★"
			}
			ui.Text(c, label).Grow(1)
			ui.Textf(c, "%d²  =  %d", n, n*n).Font("monospace").FontSize(12)
		})
	}).Height(420).Border(1, t.Border).Radius(8).Padding(4)
	if list.Changed() {
		g.picked = rows[at]
	}
	if list.Submitted() && at >= 0 {
		open(rows[at])
	}
	g.chatCard(c)
	ui.Row(c).Gap(18).AlignItems(ui.Stretch).Height(260).Children(func() {
		card(c, "Tree", func() {
			item := func(path, label string, children func()) {
				var open *bool
				if children != nil {
					o := g.tree[path]
					open = &o
					defer func() { g.tree[path] = o }()
				}
				if ui.TreeItem(c, label, open, func() {
					if children != nil {
						children()
					}
				}).Selected(g.leaf == path).Clicked() {
					g.leaf = path
				}
			}
			ui.Tree(c, func() {
				item("ui", "ui", func() {
					item("ui/widgets.go", "widgets.go", nil)
					item("ui/text", "text", func() {
						item("ui/text/layout.go", "layout.go", nil)
					})
				})
				item("go.mod", "go.mod", nil)
			})
		}).Width(220)
		card(c, "Table", func() {
			cols := []ui.TableColumn{{Title: "Name", Sortable: true}, {Title: "Kind", Width: 120, Sortable: true}, {Title: "Size", Width: 90, Align: ui.End, Sortable: true}}
			// Several files chosen by their IDs, the one last chosen
			// opening; typing a name goes to it. The names are renamed in
			// place.
			rows := g.sortedFiles()
			g.table.Key = func(i int) any { return rows[i].id }
			g.table.Label = func(i int) string { return rows[i].name }
			g.table.Selected = &g.file
			g.table.Selection = &g.chosen
			g.table.Sort = &g.sort
			if ui.Table(c, &g.table, cols, len(rows), func(row, col int) {
				f := rows[row]
				switch col {
				case 0:
					ui.EditableText(c, &f.name)
				case 1:
					ui.Text(c, f.kind).SingleLine()
				case 2:
					ui.Textf(c, "%d KB", f.size)
				}
			}).Grow(1).Submitted() {
				c.Toast("Opened " + rows[g.file].name)
			}
			cmd, rename := "Ctrl", "F2"
			if runtime.GOOS == "darwin" {
				cmd, rename = "Cmd", "Return"
			}
			ui.Textf(c, "%d chosen. Shift-click or %s-click to choose several; type a name to go to it; %s renames. Click a header to sort, drag it to move the column, and drag its edge to resize it.", g.chosen.Len(), cmd, rename).FontSize(12).TextColor(t.TextMuted)
		}).Grow(1)
	})
	card(c, "Outline", func() {
		// 100 folders of 100 files, in the order the Name column sorts.
		names := func(prefix string, n int) []string {
			items := make([]string, n)
			for i := range items {
				j := i + 1
				if g.outlineSort.Descending {
					j = n - i
				}
				items[i] = fmt.Sprintf("%s %03d", prefix, j)
			}
			return items
		}
		children := func(item string) []string {
			if !strings.HasPrefix(item, "Folder") || strings.Contains(item, "/") {
				return nil
			}
			return names(item+"/File", 100)
		}
		cols := []ui.TableColumn{{Title: "Name", Sortable: true}, {Title: "Kind", Width: 110}}
		g.outline.List.Selected = &g.outlineRow
		g.outline.List.Sort = &g.outlineSort
		ui.OutlineTable(c, &g.outline, cols, names("Folder", 100), children, func(item string, col int) {
			switch {
			case col == 0:
				ui.Text(c, item[strings.LastIndexByte(item, '/')+1:]).SingleLine()
			case children(item) != nil:
				ui.Text(c, "Folder")
			default:
				ui.Text(c, "Document")
			}
		}).Grow(1)
		ui.Text(c, "10,100 items, built only as they show. Right and Left open and close the item chosen, and Option-click or Option with them all inside.").FontSize(12).TextColor(t.TextMuted)
	}).Height(340)
	card(c, "Grid view", func() {
		// 10,000 swatches, as many columns as fit, several chosen by
		// their numbers, in the order the user drags them to.
		if g.swatchOrder == nil {
			g.swatchOrder = make([]int, 10000)
			for i := range g.swatchOrder {
				g.swatchOrder[i] = i
			}
		}
		g.swatches.Key = func(i int) any { return g.swatchOrder[i] }
		g.swatches.Selected = &g.swatch
		g.swatches.Selection = &g.swatchesChosen
		g.swatches.Label = func(i int) string { return fmt.Sprintf("Swatch %d", g.swatchOrder[i]) }
		g.swatches.Reorder = func(items []int, to int) { g.swatchOrder = reorder(g.swatchOrder, items, to) }
		ui.GridView(c, &g.swatches, len(g.swatchOrder), 96, 96, func(i int) {
			n := g.swatchOrder[i]
			ui.Box(c).Grow(1).Margin(6).Radius(6).Background(hsl(float64(n%36)*10, 0.6, 0.65))
			ui.Textf(c, "%d", n).FontSize(12).AlignSelf(ui.Center).Padding(0, 0, 4)
		}).Grow(1)
		ui.Textf(c, "%d chosen of 10,000, built only as they show. The arrows move in both directions; Shift and %s choose several, and dragging moves them.", g.swatchesChosen.Len(), map[bool]string{true: "Cmd", false: "Ctrl"}[runtime.GOOS == "darwin"]).FontSize(12).TextColor(t.TextMuted)
	}).Height(380)
}

// chatCard shows a chat: messages of every height, which the list
// measures as they show, the header of each day pinned at the top, older
// messages loading above without moving those in view, and new ones
// followed at the end.
func (g *gallery) chatCard(c *ui.Context) {
	t := c.Theme()
	if g.messages == nil {
		for id := range 200 {
			g.messages = append(g.messages, makeMessage(id))
		}
	}
	// The rows: a header before each day's messages.
	type item struct {
		header bool
		day    int
		msg    message
	}
	var items []item
	for i, m := range g.messages {
		if i == 0 || m.day() != g.messages[i-1].day() {
			items = append(items, item{header: true, day: m.day()})
		}
		items = append(items, item{day: m.day(), msg: m})
	}
	g.chat.FollowEnd = true
	g.chat.Key = func(i int) any {
		if items[i].header {
			return fmt.Sprint("day ", items[i].day)
		}
		return items[i].msg.id
	}
	g.chat.Header = func(i int) bool { return items[i].header }
	card(c, "Chat", func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Textf(c, "%d messages of every height; older ones load above without moving those in view.", len(g.messages)).TextColor(t.TextMuted).Grow(1)
			if ui.Button(c, "Load older").Clicked() {
				older := make([]message, 0, 24)
				for id := g.messages[0].id - 24; id < g.messages[0].id; id++ {
					older = append(older, makeMessage(id))
				}
				g.messages = append(older, g.messages...)
			}
			if !g.chat.AtEnd() && ui.Button(c, "Jump to latest").Clicked() {
				g.chat.ScrollToEnd()
			}
		})
		ui.List(c, &g.chat, len(items), func(i int) {
			it := items[i]
			if it.header {
				label := "Today"
				if it.day < 0 {
					label = time.Now().AddDate(0, 0, it.day).Format("Monday, January 2")
				}
				ui.Row(c).Justify(ui.Center).PaddingY(6).Children(func() {
					ui.Text(c, label).FontSize(12).Bold().Padding(3, 10).Radius(999).Background(t.Surface).Border(1, t.Border)
				})
				return
			}
			m := it.msg
			ui.Row(c).Padding(3, 12).Children(func() {
				bubble := ui.Box(c).MaxWidthPercent(72).Padding(7, 12).Radius(14).Background(t.Surface)
				if m.mine {
					bubble.Margin(0, 0, 0, ui.Auto).Background(t.Accent).TextColor(t.AccentText)
				}
				bubble.Children(func() { ui.Text(c, m.text) })
			})
		}).Height(380).Justify(ui.End).PaddingY(4).Border(1, t.Border).Radius(8)
		ui.Row(c).Gap(8).Children(func() {
			input := ui.TextInput(c, &g.draft).Placeholder("Message").Label("Message").Grow(1)
			send := ui.Button(c, "Send").Disabled(strings.TrimSpace(g.draft) == "")
			if (send.Clicked() || input.Submitted()) && strings.TrimSpace(g.draft) != "" {
				last := g.messages[len(g.messages)-1]
				g.messages = append(g.messages, message{id: last.id + 1, text: strings.TrimSpace(g.draft), mine: true})
				g.draft = ""
			}
		})
	})
}

func (g *gallery) styling(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "Grids, borders of each side, gradients, stripes, text decorations, and motion along easings.").TextColor(t.TextMuted)
	ui.Grid(c).Columns(2).Gap(16).Children(func() {
		card(c, "Grid", func() {
			ui.Grid(c).ColumnTracks(ui.FitContent(), ui.Fr(1), ui.Fr(1)).Gap(6).Children(func() {
				cell := func(s string) ui.Element {
					return ui.Box(c).Padding(8, 10).Radius(6).Background(t.Surface).Children(func() { ui.Text(c, s).FontSize(12) })
				}
				cell("ColumnSpan(-1)").ColumnSpan(-1).Background(t.Accent).TextColor(t.AccentText)
				cell("FitContent").RowSpan(2)
				cell("Fr(1)")
				cell("Fr(1)")
				cell("ColumnSpan(2)").ColumnSpan(2).JustifySelf(ui.Center)
			})
		})
		card(c, "Borders", func() {
			ui.Row(c).PaddingY(6).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, "A header with a line below").Bold()
			})
			ui.Row(c).Padding(8, 12).Gap(8).BorderWidth(0, 0, 0, 4).BorderColor(t.Accent).Background(t.Surface).Radius(0, 6, 6, 0).Children(func() {
				ui.Text(c, "A note with an accent on its left")
			})
			ui.Column(c).Height(56).Radius(8).Border(2, t.Border).BorderStyle(ui.BorderDashed).Center().Children(func() {
				ui.Text(c, "Dashed, as a place to drop files").TextColor(t.TextMuted)
			})
		})
		card(c, "Fills", func() {
			blue, yellow := ui.Hex("#2563eb"), ui.Hex("#facc15")
			bar := func(label string) ui.Element {
				return ui.Row(c).Height(30).PaddingX(10).Radius(6).Children(func() {
					ui.Text(c, label).FontSize(12).Bold().TextColor(ui.RGB(255, 255, 255))
				})
			}
			bar("sRGB").Gradient(blue, yellow, 90)
			bar("Oklab").LinearGradient(ui.LinearGradient{From: blue, To: yellow, Angle: 90, Oklab: true})
			bar("Stops at 40% and 60%").LinearGradient(ui.LinearGradient{From: blue, To: yellow, Angle: 90, Start: 0.4, End: 0.6})
			ui.Row(c).Height(30).Radius(6).Background(t.Surface).Stripes(t.Border, 4, 6, 45).Center().Children(func() {
				ui.Text(c, "Stripes: unavailable").FontSize(12).TextColor(t.TextMuted)
			})
		})
		card(c, "Wide colors (Oklch)", func() { wideColors(c) })
		card(c, "Text decorations", func() {
			ui.RichText(c, ui.Span{Text: "Spell checkers mark "}, ui.Span{Text: "mispeled", WavyUnderline: true, DecorationColor: t.Danger},
				ui.Span{Text: " words with waves."})
			ui.RichText(c, ui.Span{Text: "Search results "}, ui.Span{Text: "stand out", Background: ui.RGBA(250, 204, 21, 0.45)},
				ui.Span{Text: " with a background."})
			ui.Text(c, "Underlines of their own color and thickness").Underline().DecorationColor(t.Accent).DecorationThickness(2)
			ui.Text(c, "A highlight behind every line").TextBackground(t.Selection)
			ui.Text(c, strings.Repeat("A line cut with an ellipsis of its own. ", 3)).SingleLine().Ellipsis(" →")
		})
		card(c, "Motion", func() {
			ui.Row(c).Gap(14).Children(func() {
				spin := ui.Icon(c, loaderIcon).FontSize(24).TextColor(t.Accent)
				spin.Rotate(spin.Loop("spin", time.Second, ui.Linear) * 360)
				pulse := ui.Box(c).Height(14).Grow(1).Radius(7).Background(t.Border)
				pulse.Opacity(0.4 + 0.6*pulse.Loop("pulse", 1600*time.Millisecond, ui.Bounce(ui.EaseInOut)))
			})
			if ui.Button(c, "Move along each easing").Clicked() {
				g.eased = !g.eased
			}
			for _, e := range []struct {
				name string
				ease ui.Easing
			}{{"Linear", ui.Linear}, {"EaseIn", ui.EaseIn}, {"EaseOut", ui.EaseOut}, {"EaseInOut", ui.EaseInOut}} {
				track := ui.Row(c.Key(e.name)).Height(18)
				to := float32(0)
				if g.eased {
					to = 1
				}
				at := track.AnimateWith("x", to, 900*time.Millisecond, e.ease)
				track.Children(func() {
					ui.Text(c, e.name).FontSize(12).Width(70).TextColor(t.TextMuted)
					ui.Box(c).Grow(1).Height(18).Children(func() {
						ui.Box(c).Size(18, 18).Radius(9).Background(t.Accent).Absolute().LeftPercent(at * 90)
					})
				})
			}
		})
		card(c, "Layout", func() {
			ui.Row(c).Gap(6).Reverse().Children(func() {
				for _, s := range []string{"1", "2", "3"} {
					ui.Box(c).Size(28, 28).Radius(6).Background(t.Surface).Center().Children(func() { ui.Text(c, s) })
				}
				ui.Text(c, "Reverse()").FontSize(12).TextColor(t.TextMuted).Margin(0, ui.Auto, 0, 0)
			})
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, "Margin(…, Auto) pushes to the end").FontSize(12).TextColor(t.TextMuted)
				ui.Button(c, "Save").Margin(0, 0, 0, ui.Auto)
			})
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, "Inbox")
				ui.Text(c, "3").FontSize(10).Bold().Padding(1, 5).Radius(8).Background(t.Danger).TextColor(ui.RGB(255, 255, 255)).Top(-6)
				ui.Text(c, "Top(-6) moves a badge up").FontSize(12).TextColor(t.TextMuted)
			})
		})
		card(c, "Scrolling both ways", func() {
			ui.ScrollBoth(c).Height(150).Radius(6).Border(1, t.Border).Children(func() {
				ui.Grid(c).ColumnTracks(repeat(ui.Fixed(56), 16)...).Gap(4).Padding(6).Children(func() {
					for i := range 16 * 12 {
						ui.Box(c).Height(28).Radius(4).Background(t.Accent.Alpha(0.08 + 0.6*float32(i%16)/16*float32(i/16)/12)).Center().Children(func() {
							ui.Textf(c, "%c%d", 'A'+i%16, i/16+1).FontSize(11)
						})
					}
				})
			})
		})
		card(c, "Cursors", func() {
			ui.Row(c).Wrap().Gap(6).Children(func() {
				for _, k := range []struct {
					name   string
					cursor ui.Cursor
				}{{"ResizeColumn", ui.CursorResizeColumn}, {"ResizeRow", ui.CursorResizeRow}, {"ResizeE", ui.CursorResizeE},
					{"Copy", ui.CursorCopy}, {"Alias", ui.CursorAlias}, {"ContextMenu", ui.CursorContextMenu},
					{"VerticalText", ui.CursorVerticalText}, {"None", ui.CursorNone}} {
					ui.Box(c).Padding(6, 10).Radius(6).Background(t.Surface).Cursor(k.cursor).Children(func() { ui.Text(c, k.name).FontSize(12) })
				}
			})
		})
	})
}

// repeat returns n tracks t.
func repeat(t ui.Track, n int) []ui.Track {
	out := make([]ui.Track, n)
	for i := range out {
		out[i] = t
	}
	return out
}

func (g *gallery) drawing(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "Element.Draw paints with rectangles, shadows, paths and text. This drawing moves with the time of each frame, which paints it again without building the page.").TextColor(t.TextMuted)
	ui.Box(c).Height(320).Radius(10).Background(t.Surface).Draw(func(p *ui.Painter, r ui.Rect) {
		p.AnimationFrame()
		phase := float64(p.Now().UnixMilli()%4000) / 4000 * 2 * math.Pi
		// Bars.
		for i := 0; i < 12; i++ {
			h := float32(60 + 50*math.Sin(phase+float64(i)*0.6))
			x := r.X + 24 + float32(i)*28
			p.Fill(ui.Rect{X: x, Y: r.Y + r.H - 24 - h, W: 18, H: h}, t.Accent.Alpha(0.35+0.05*float32(i)), 4)
		}
		// A sine wave.
		var wave ui.Path
		for i := 0; i <= 100; i++ {
			x := r.X + 380 + float32(i)*3
			y := r.Y + r.H/2 + float32(60*math.Sin(phase*2+float64(i)/12))
			if i == 0 {
				wave.MoveTo(x, y)
			} else {
				wave.LineTo(x, y)
			}
		}
		p.StrokePath(&wave, 3, t.Danger)
		var dot ui.Path
		dot.Circle(r.X+r.W-70, r.Y+70, 36)
		p.FillPath(&dot, t.Accent)
		p.Text(r.X+24, r.Y+20, "Animated at the display's rate", 14, t.Text)
	})
	card(c, "Vector images", func() {
		ui.Text(c, "SVGs stay sharp at any size: icons in the color of the text, pictures in their own colors.").TextColor(t.TextMuted)
		gold := ui.RGB(245, 180, 0)
		ui.Row(c).Gap(14).Children(func() {
			for _, size := range []float32{16, 24, 40} {
				ui.Icon(c, starIcon).FontSize(size).TextColor(gold)
			}
			// A button lays out its children in a row.
			done := ui.PrimaryButton(c, "").Children(func() {
				ui.Icon(c, checkIcon)
				ui.Text(c, "Done").SingleLine()
			})
			if done.Clicked() {
				c.Toast("Done")
			}
			ui.Image(c, badge).Size(64, 64).TextColor(gold)
			ui.Image(c, badge).Size(32, 32).TextColor(gold)
		})
	})
}

// glass shows Liquid Glass: a toolbar floating over content that scrolls
// under it, on a bar of glass, a scroll edge or a progressive blur, a
// tinted button, and a lens to drag around.
func (g *gallery) glassPage(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "The glass plugin's Liquid Glass, a material, as macOS draws it: what is under it shows through, frosted and bent along its edges. Scroll under the toolbar, and drag the lens.").TextColor(t.TextMuted)
	ui.Row(c).Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Segmented(c, &g.glassStyle, "Regular", "Clear").Label("Glass")
		ui.Segmented(c, &g.glassEdge, "Glass bar", "Soft edge", "Hard edge", "Progressive blur").Label("Under the toolbar")
	})
	style := glass.Regular
	if g.glassStyle == 1 {
		style = glass.Clear
	}
	tiles := []ui.Color{ui.Hex("#ef4444"), ui.Hex("#f59e0b"), ui.Hex("#10b981"), ui.Hex("#06b6d4"), ui.Hex("#6366f1"), ui.Hex("#ec4899")}
	area := ui.Box(c).Height(420).Radius(12).Clip().Border(1, t.Border)
	area.Children(func() {
		// What shows through: photos and text, which start below the
		// toolbar, 64 DIPs down, and scroll under it, with the scroll bar
		// below what floats over the content.
		bars := float32(64)
		if g.glassEdge == 3 {
			bars = 88
		}
		ui.Scroll(c).Fill().Padding(76, 16, 16).ScrollbarInsets(bars, 0, 0).Gap(12).Children(func() {
			for i := range 12 {
				ui.Row(c).Gap(14).Children(func() {
					a, b := tiles[i%len(tiles)], tiles[(i+2)%len(tiles)]
					ui.Box(c).Size(180, 96).Radius(12).Gradient(a, b, 135)
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, fmt.Sprintf("Photo %d", i+1)).Bold()
						ui.Text(c, "Glass bends the light along its rim, and frosts what is under its middle.").TextColor(t.TextMuted)
					})
				})
			}
		})
		// A toolbar of buttons floating on glass, or over a scroll edge as
		// macOS's, which fades the content into the background or frosts
		// it, or over a blur, the stronger the nearer the top.
		if g.glassEdge > 0 {
			edge := ui.Box(c).Absolute().Top(0).Left(0).Right(0).PassThrough()
			switch g.glassEdge {
			case 1:
				edge.Height(74).Material(glass.ScrollEdge{})
			case 2:
				edge.Height(64).Material(glass.ScrollEdge{Hard: true})
			case 3:
				edge.Height(88).Material(glass.Blur{Radius: 6, Mask: &ui.LinearGradient{From: ui.RGB(0, 0, 0), To: ui.Transparent, Angle: 180, Start: 0.3, End: 1}})
			}
		}
		bar := ui.Row(c).Absolute().Top(12).Left(12).Right(12).Padding(6, 8).Gap(6).AlignItems(ui.Center).Radius(26)
		if g.glassEdge == 0 {
			bar.Material(glass.Glass{Style: style})
		} else {
			bar.PassThrough()
		}
		bar.Children(func() {
			for _, ic := range []*ui.SVG{chevronIcon, starIcon, checkIcon} {
				b := ui.Box(c).Size(40, 40).Radius(20).Center().Material(glass.Glass{Style: style, Interactive: true}).Children(func() {
					ui.Icon(c, ic).FontSize(18)
				})
				if b.Clicked() {
					c.Toast("Clicked")
				}
			}
			ui.Box(c).Grow(1)
			done := ui.Row(c).Padding(8, 16).Radius(20).Material(glass.Glass{Style: style, Tint: t.Accent, Interactive: true}).Children(func() {
				ui.Text(c, "Done").Bold().TextColor(t.AccentText)
			})
			if done.Clicked() {
				c.Toast("Done")
			}
		})
		// A lens to drag over what is under it.
		lens := ui.Box(c).Absolute().Left(g.lens[0]).Top(g.lens[1]).Size(110, 110).Radius(55).Material(glass.Glass{Style: style, Interactive: true})
		if dx, dy, ok := lens.Dragged(); ok {
			g.lens[0] = max(0, g.lens[0]+dx)
			g.lens[1] = max(0, g.lens[1]+dy)
		}
	})
}

// motionItem is an item of the Motion page's list.
type motionItem struct {
	id   int
	name string
}

// motionColors tint the items of the Motion page.
var motionColors = []ui.Color{ui.Hex("#2563eb"), ui.Hex("#16a34a"), ui.Hex("#d97706"), ui.Hex("#db2777"), ui.Hex("#7c3aed")}

// itemMotion moves the items of the Motion page: they grow in, collapse
// out, and slide to their places.
var itemMotion = ui.ElementTransition{
	Duration: 250 * time.Millisecond,
	Ease:     ui.EaseInOut,
	Enter:    &ui.Motion{Collapse: true},
	Exit:     &ui.Motion{Collapse: true},
}

func (g *gallery) motion(c *ui.Context) {
	t := c.Theme()
	ui.Text(c, "Transitions move elements where the layout puts them, and in and out as they come and go. "+
		"F12 (Alt+Cmd+I) opens the inspector of the window's elements.").TextColor(t.TextMuted)
	ui.Grid(c).Columns(2).Gap(16).Children(func() {
		card(c, "A list that moves", func() {
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "Add").Clicked() {
					g.nextItem++
					at := 0
					if len(g.items) > 0 {
						at = g.nextItem % (len(g.items) + 1)
					}
					g.items = slices.Insert(g.items, at, motionItem{g.nextItem, fmt.Sprintf("Item %d", g.nextItem)})
				}
				if ui.Button(c, "Shuffle").Disabled(len(g.items) < 2).Clicked() {
					for i := range g.items {
						j := (i*7 + g.nextItem) % len(g.items)
						g.items[i], g.items[j] = g.items[j], g.items[i]
					}
					g.nextItem++
				}
				if ui.Button(c, "Sort").Disabled(len(g.items) < 2).Clicked() {
					slices.SortFunc(g.items, func(a, b motionItem) int { return a.id - b.id })
				}
			})
			// Each row has the same transition, so that the rows move in
			// step, and the column grows and shrinks with them; the lines
			// between the rows come with the column.
			ui.Column(c).Radius(8).Border(1, t.Border).Clip().Dividers(1, t.Border).
				Transition(ui.ElementTransition{Size: true, Duration: itemMotion.Duration, Ease: itemMotion.Ease}).Children(func() {
				removed := -1
				for _, it := range g.items {
					ui.Row(c.Key(it.id)).Padding(8, 10).Gap(10).AlignItems(ui.Center).Background(t.Background).Transition(itemMotion).Children(func() {
						ui.Box(c).Size(10, 10).Radius(5).Background(motionColors[it.id%len(motionColors)])
						ui.Text(c, it.name).Grow(1)
						if ui.Button(c, "Remove").Clicked() {
							removed = it.id // once the loop over the items is done
						}
					})
				}
				if removed >= 0 {
					g.items = slices.DeleteFunc(g.items, func(o motionItem) bool { return o.id == removed })
				}
			})
			if len(g.items) == 0 {
				ui.Text(c, "No items: add some.").TextColor(t.TextMuted)
			}
		})
		card(c, "A panel that opens", func() {
			if ui.Button(c, "Toggle the panel").Clicked() {
				g.panelOpen = !g.panelOpen
			}
			ui.Row(c).Height(120).Radius(8).Border(1, t.Border).Clip().Children(func() {
				w := float32(48)
				if g.panelOpen {
					w = 160
				}
				// The panel lays out its content at each width on the way;
				// the content beside it moves with it.
				ui.Column(c).Width(w).Shrink(0).Padding(10).Gap(8).Background(t.Surface).ClipX().
					Transition(ui.ElementTransition{Size: true, Ease: ui.EaseInOut}).Children(func() {
					for _, p := range []string{"Overview", "Styling", "Drawing"} {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, pageIcons[p]).FontSize(18).Shrink(0)
							ui.Text(c, p).SingleLine()
						})
					}
				})
				ui.Column(c).Grow(1).Padding(12).Transition(ui.ElementTransition{Ease: ui.EaseInOut}).Children(func() {
					ui.Text(c, "The content beside the panel.").TextColor(t.TextMuted)
				})
			})
		})
		card(c, "Colors that fade", func() {
			ui.Text(c, "Their backgrounds and borders fade as the pointer comes and goes.").FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(10).Children(func() {
				for i, col := range motionColors[:3] {
					tile := ui.Box(c.Key(i)).Size(64, 48).Radius(8).Border(2, t.Border)
					tile.Background(col.Alpha(0.15))
					if tile.Hovered() {
						tile.Background(col).BorderColor(col.Mix(ui.RGB(0, 0, 0), 0.3))
					}
					tile.Transition(ui.ElementTransition{Colors: true, Duration: 150 * time.Millisecond})
				}
			})
		})
		card(c, "Attached", func() {
			ui.Text(c, "Attach puts a point of an element on a point of its parent.").FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(24).PaddingY(8).Children(func() {
				for i, name := range []string{"Ada Lovelace", "Alan Turing"} {
					ui.Box(c).Children(func() {
						ui.Avatar(c, name, nil)
						ui.Text(c, fmt.Sprint(3+i*9)).FontSize(10).Bold().Padding(1, 5).Radius(8).
							Background(t.Danger).TextColor(ui.RGB(255, 255, 255)).Attach(ui.AnchorTopRight, ui.AnchorCenter)
					})
				}
				ui.Box(c).Size(120, 64).Radius(8).Background(t.Surface).Children(func() {
					ui.Text(c, "Bottom right").FontSize(11).TextColor(t.TextMuted).
						Attach(ui.AnchorBottomRight, ui.AnchorBottomRight).Right(6).Bottom(4)
					ui.Text(c, "Center").FontSize(11).Attach(ui.AnchorCenter, ui.AnchorCenter)
				})
			})
		})
	})
}

func (g *gallery) overlays(c *ui.Context) {
	t := c.Theme()
	card(c, "Overlays", func() {
		ui.Row(c).Gap(10).Children(func() {
			if ui.PrimaryButton(c, "Open dialog").Clicked() {
				g.dialog = true
			}
			menu := ui.Button(c, "Menu ▾")
			if menu.Clicked() {
				g.menu = !g.menu
			}
			ui.Popover(c, menu, &g.menu, func() {
				for _, item := range []string{"New file", "Open…", "Save as…"} {
					entry := ui.Row(c.Key(item)).Padding(6, 12).Radius(5).Width(180)
					if entry.Hovered() {
						entry.Background(t.Accent).TextColor(t.AccentText)
					}
					if entry.Clicked() {
						g.menu = false
					}
					entry.Children(func() { ui.Text(c, item) })
				}
			})
			ui.Button(c, "Hover me").Tooltip("Tooltips show after the pointer rests a moment.")
			if ui.Button(c, "Alert").Clicked() {
				g.alert = true
			}
			if ui.Button(c, "Delete a note").Clicked() {
				g.notes--
				c.ToastAction("Note deleted", "Undo", func() { g.notes++ })
			}
			if ui.Button(c, "Native dialog").Clicked() {
				go mygo.Dialog.Message(mygo.MessageOptions{Parent: g.win, Message: "Native dialogs work from MyGo UI windows too."})
			}
		})
	})
	ui.Textf(c, "%d notes. Undo in the toast brings a deleted one back.", g.notes).FontSize(12).TextColor(t.TextMuted)
	if ui.AlertDialog(c, &g.alert, "Delete “Notes”?", "This deletes the note on all your devices. You can't undo this.", "Cancel", "Delete") == 1 {
		g.notes--
		c.Toast("Note deleted")
	}
	ui.Modal(c, &g.dialog, func() {
		ui.Text(c, "A modal dialog").FontSize(18).Bold()
		ui.Text(c, "Click outside or press Escape to close it.").TextColor(t.TextMuted)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.PrimaryButton(c, "Done").Clicked() {
				g.dialog = false
			}
		})
	})
}

func main() {
	g := &gallery{router: ui.NewRouter("/overview"), items: []motionItem{{1, "Item 1"}, {2, "Item 2"}, {3, "Item 3"}}, nextItem: 3, size: "Medium", fruit: "Apple", plan: "Pro", volume: 35, picked: -1, starred: map[int]bool{}, split: 160, copies: 1, tree: map[string]bool{"ui": true}, birthday: time.Date(1815, 12, 10, 0, 0, 0, 0, time.UTC), now: time.Now(), font: "Helvetica", tags: []string{"go", "native"}, sections: [3]bool{true}, sectionsOpen: [2]bool{true, true}, stars: 4, battery: 35, quality: 75, priceLow: 100, priceHigh: 350, meeting: time.Date(2026, 10, 15, 9, 30, 0, 0, time.Local), tint: ui.Hex("#2563eb"), lens: [2]float32{110, 190}, glassEdge: 1, notes: 12, notifyMail: true, pathDepth: 4}
	mygo.App.WhenReady(func() {
		g.win = mygo.NewWindow(mygo.WindowOptions{
			Title:    "MyGo UI Gallery",
			Width:    980,
			Height:   720,
			MinWidth: 640, MinHeight: 480,
			StateKey: "gallery",
			Content:  ui.View(g.view),
		})
		go func() {
			x, sample := 0.0, func(x float64) float64 { return 0.5 + 0.35*math.Sin(x) + 0.1*math.Sin(x*3.1) }
			// A minute of samples to start with, then one a second, when
			// the clock changes: the window draws nothing in between.
			history := make([]float64, 60)
			for i := range history {
				x += 0.35
				history[i] = sample(x)
			}
			g.win.Update(func() { g.samples = history })
			for {
				time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
				x += 0.35
				v := sample(x)
				g.win.Update(func() {
					g.now = time.Now()
					g.samples = append(g.samples[1:], v)
				})
			}
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
