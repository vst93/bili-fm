package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

// center returns the center of a box.
func center(r Rect) (x, y float32) { return r.X + r.W/2, r.Y + r.H/2 }

func TestInlineLink(t *testing.T) {
	const url = "https://example.com/guide"
	var para, link *node
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).Children(func() {
			para = coreRichText(c).Children(func() {
				coreText(c, "Read ")
				link = coreLink(c, "the documentation of the toolkit", url)
				coreText(c, " to get started.")
			})
		})
	}, 200, 200)
	if !tt.HasText("Read the documentation of the toolkit to get started.") {
		t.Fatalf("the paragraph shows %q", tt.Texts())
	}
	// The link continues the line of "Read " and wraps with the paragraph.
	if len(link.frags) < 2 {
		t.Fatalf("the link takes %d boxes in a narrow paragraph", len(link.frags))
	}
	first, last := link.frags[0], link.frags[len(link.frags)-1]
	if first.X <= para.x+para.contentX() || first.Y != para.y+para.contentY() || last.Y <= first.Y {
		t.Errorf("the link's boxes %v in the paragraph at %v,%v", link.frags, para.x, para.y)
	}

	tt.Move(center(last))
	if tt.Cursor() != CursorPointer {
		t.Errorf("the pointer is %v over the link", tt.Cursor())
	}
	tt.Move(para.x+para.contentX()+5, first.Y+first.H/2) // over "Read"
	if tt.Cursor() != CursorDefault {
		t.Errorf("the pointer is %v over text", tt.Cursor())
	}
	// A click on any line of the link opens it, and on the text around it
	// does not.
	tt.ClickAt(para.x+para.contentX()+5, first.Y+first.H/2)
	tt.ClickAt(center(last))
	if got := tt.OpenedURLs(); !slices.Equal(got, []string{url}) {
		t.Fatalf("opened %v", got)
	}
	if err := tt.Click("the documentation of the toolkit"); err != nil {
		t.Fatal(err)
	}
	if got := tt.OpenedURLs(); len(got) != 2 {
		t.Fatalf("a click on the link found by its text opened %v", got)
	}

	// The keyboard reaches it.
	tt.Key(0, KeyTab)
	if !tt.Focused("the documentation of the toolkit") {
		t.Fatal("Tab does not focus the link")
	}
	// The ring goes around its words, on each line.
	for _, f := range link.frags {
		if c := tt.Image().RGBAAt(int(f.X+f.W/2), int(f.Y-2)); int(c.B) < int(c.R)+40 {
			t.Errorf("no focus ring above the link's box %v: %v", f, c)
		}
	}
	tt.Key(0, KeyEnter)
	if got := tt.OpenedURLs(); len(got) != 3 {
		t.Fatalf("Enter on the link opened %v", got)
	}

	// Assistive technology sees the paragraph, and the link inside it.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tree := tt.h.access
	text := accessNode(t, tree, platform.RoleText, "Read the documentation")
	ln := accessNode(t, tree, platform.RoleLink, "the documentation of the toolkit")
	if tree.Nodes[ln.Parent].ID != text.ID || ln.Actions&platform.ActionPress == 0 || ln.Bounds.W <= 0 {
		t.Errorf("the link's node %+v under %+v", ln, tree.Nodes[ln.Parent])
	}
	if text.Label != "Read the documentation of the toolkit to get started." {
		t.Errorf("the paragraph reads %q", text.Label)
	}
	for _, n := range tree.Nodes {
		if n.Label == "Read " || n.Label == " to get started." {
			t.Errorf("plain text inside the paragraph has a node of its own: %+v", n)
		}
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessAction, ID: ln.ID, Action: platform.AccessPress})
	if got := tt.OpenedURLs(); len(got) != 4 {
		t.Fatalf("pressing the link's node opened %v", got)
	}
}

func TestInlineStyles(t *testing.T) {
	red, gray := RGB(255, 0, 0), RGB(200, 200, 200)
	var para, plain, code *node
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).AlignItems(Start).Children(func() {
			para = coreRichText(c, Span{Text: "Own "}).FontSize(20).Children(func() {
				coreText(c, "bold ").Bold()
				plain = coreText(c, "red ").TextColor(red)
				coreRichText(c).Italic().Children(func() {
					coreText(c, "nested ")
					code = coreText(c, "code").Font("monospace").Background(gray)
				})
			})
		})
	}, 600, 200)
	if para.text != "Own bold red nested code" {
		t.Fatalf("text %q", para.text)
	}
	want := []Span{
		{Text: "Own "},
		{Text: "bold ", Weight: 700},
		{Text: "red ", Color: red},
		{Text: "nested ", Italic: true},
		{Text: "code", Italic: true, Font: "monospace", Background: gray},
	}
	if !slices.Equal(para.spans, want) {
		t.Errorf("spans\n got %+v\nwant %+v", para.spans, want)
	}
	// The paragraph's size applies to all of it, and the nested text has
	// boxes where its own text shows.
	if code.frags[0].H < 20 || code.frags[0].X <= plain.frags[0].X {
		t.Errorf("code at %v, red at %v", code.frags, plain.frags)
	}
	img := tt.Image()
	reds := 0
	r := plain.frags[0]
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if c := img.RGBAAt(x, y); c.R > 200 && c.G < 80 && c.B < 80 {
				reds++
			}
		}
	}
	if reds < 20 {
		t.Errorf("%d red pixels in the red text", reds)
	}
	if c := img.RGBAAt(int(code.frags[0].X+1), int(code.frags[0].Y+1)); c.R != 200 || c.G != 200 {
		t.Errorf("the code's background is %v", c)
	}
}

func TestInlineInteraction(t *testing.T) {
	clicks, disabled := 0, false
	var more *node
	tt := coreNewTester(func(c *context) {
		coreRichText(c).Selectable().Children(func() {
			coreText(c, "Twelve files changed. ")
			more = coreText(c, "Show all").TextColor(c.Theme().Accent).Tooltip("Lists every file").Disabled(disabled)
			if more.Clicked() {
				clicks++
			}
		})
	}, 400, 100)
	tt.ClickAt(center(more.frags[0]))
	if clicks != 1 {
		t.Fatalf("%d clicks on inline text", clicks)
	}
	// Its tooltip shows when the pointer rests on its words, once it came
	// back after the click.
	tt.Move(0, 90)
	tt.Move(center(more.frags[0]))
	tt.rt.tips.hoverSince = time.Now().Add(-time.Second)
	tt.Frame()
	if !tt.HasText("Lists every file") {
		t.Error("no tooltip over inline text")
	}
	// Selectable before Children selects in the whole paragraph.
	if !tt.HasText("Twelve files changed. Show all") || tt.rt.states[more.parent.id].editor.source != "Twelve files changed. Show all" {
		t.Error("the paragraph does not select its whole text")
	}
	disabled = true
	tt.Frame()
	tt.ClickAt(center(more.frags[0]))
	if clicks != 1 {
		t.Error("disabled inline text was clicked")
	}
}

func TestInlineOnlyText(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a box inside a text did not panic")
		}
	}()
	coreRender(func(c *context) {
		coreText(c, "Save").Children(func() { coreButton(c, "Now") })
	}, 100, 100, 1)
}
