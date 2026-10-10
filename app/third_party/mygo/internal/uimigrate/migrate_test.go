package uimigrate

import (
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestMigrationPreservesLocalValuePolling(t *testing.T) {
	source := `package app
import native "github.com/egoist/mygo/ui"
var shared bool
type app struct { checked bool }
func (a *app) view(c *native.Context, persistent *bool) {
 checked := a.checked
 if native.Checkbox(c, &checked, "Derived").Disabled(false).Changed() {
  a.checked = checked
  return
 }
 var draft string
 input := native.TextInput(c, &draft)
 alias := input
 if alias.Submitted() { a.send(draft) }
 var choice = 0
 element := native.Segmented[int](c, &choice)
 if element.Changed() { a.choose(choice) }
 native.Checkbox(c, &checked, "Callback").OnChange(func() { a.checked = checked })
 if native.Checkbox(c, &a.checked, "Field").Changed() { a.save() }
 if native.Checkbox(c, &shared, "Global").Changed() { a.save() }
 if native.Checkbox(c, persistent, "Parameter").Changed() { a.save() }
 other := foreign()
 if other.Changed() { a.save() }
}
`
	formatted, err := format.Source([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	r, err := File("app.go", formatted)
	if err != nil {
		t.Fatal(err)
	}
	if r.Changed || string(r.Source) != string(formatted) {
		t.Fatalf("polling control flow was rewritten:\n%s", r.Source)
	}
	if len(r.Notes) != 0 {
		t.Fatalf("same-pass local-value polling needs no review notes: %q", r.Notes)
	}
	again, err := File("app.go", r.Source)
	if err != nil || again.Changed || strings.Join(again.Notes, "\n") != strings.Join(r.Notes, "\n") {
		t.Fatal("review notes were not stable on a repeated migration", err)
	}
}

func TestMigrationAliasesScopesAndNilValues(t *testing.T) {
	source := `package app
import native "github.com/egoist/mygo/ui"
type app struct { saved *native.Element; other *int }
func (a *app) view(c *native.Context) {
 native.Column(c).Children(func() {
  e := native.Text(c, "Hello")
  if e != nil { e.Focus() }
  if a.saved == nil { a.saved = e }
  files := e.DroppedFiles()
  if files != nil { println(files) }
  if a.other != nil { println(*a.other) }
 })
 a.saved = nil
}
func missing() *native.Element { return nil }
`
	r, err := File("app.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"view(c *native.Context)", "Children(func()", "e.Valid()", "!a.saved.Valid()", "a.saved = native.Element{}", "return native.Element{}", "files != nil", "a.other != nil"} {
		if !strings.Contains(string(r.Source), want) {
			t.Fatalf("missing %q:\n%s", want, r.Source)
		}
	}
	if len(r.Notes) == 0 {
		t.Fatal("stored element fields were not reported")
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "app.go", r.Source, 0); err != nil {
		t.Fatal(err)
	}
	again, err := File("app.go", r.Source)
	if err != nil {
		t.Fatal(err)
	}
	if again.Changed {
		t.Fatalf("migration was not idempotent:\n%s", again.Source)
	}
}

func TestMigrationPreservesNonUIAliases(t *testing.T) {
	source := `package app
import ui "example.org/other/ui"
func render(c *ui.Context) *ui.Element { return nil }
`
	r, err := File("app.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if r.Changed || string(r.Source) != source {
		t.Fatal("an unrelated UI library was modified")
	}
}

func TestMigrationBuilderCallbacks(t *testing.T) {
	source := `package app
import "github.com/egoist/mygo/ui"
func view(c *ui.Context) {
 ui.List(c, nil, 5, func(i int) { ui.Text(c, "row") })
 ui.Table(c, nil, nil, 5, func(row, col int) { ui.Text(c, "cell") })
 ui.Button(c, "menu").ContextMenu(func(m *ui.Menu) { m.Item("Open") })
 ui.Box(c).Rows(3)
}
`
	r, err := File("app.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"func(i int)", "func(row, col int)", "func(m *ui.Menu)", "GridRows(3)"} {
		if !strings.Contains(string(r.Source), want) {
			t.Fatalf("missing %q:\n%s", want, r.Source)
		}
	}
}

func TestMigrationDotImportRequiresReview(t *testing.T) {
	r, err := File("app.go", []byte("package app\nimport . \"github.com/egoist/mygo/ui\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Changed || len(r.Notes) != 1 {
		t.Fatal("dot import was guessed instead of reported")
	}
}

func TestMigrationDoesNotChangeOtherBuilders(t *testing.T) {
	source := `package app
import "github.com/egoist/mygo/ui"
func render(c *ui.Context, foreign Other) {
 foreign.Children(func() { println("foreign") })
 ui.Column(c).Children(func() { ui.Text(c,"UI") })
}
`
	r, err := File("app.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(r.Source), "foreign.Children(func()") {
		t.Fatalf("unrelated builder changed:\n%s", r.Source)
	}
}

func TestMigrationKeysBeforeConstruction(t *testing.T) {
	source := `package app
import native "github.com/egoist/mygo/ui"
func view(c *native.Context,v *bool){
 native.Checkbox(c,v,"Done").Disabled(false).Key("done").OnChange(func(){})
 if c!=nil { println("context") }
}
`
	r, err := File("app.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(r.Source), `Checkbox(c.Key("done"), v, "Done").Disabled(false).OnChange`) {
		t.Fatal(string(r.Source))
	}
	if !strings.Contains(string(r.Source), "c != nil") {
		t.Fatal("context nil check changed")
	}
	again, err := File("app.go", r.Source)
	if err != nil || again.Changed {
		t.Fatal("key migration is not idempotent", err, string(again.Source))
	}
}

func TestMigrationSharesPackageFieldsAndTupleHelpers(t *testing.T) {
	sources := map[string][]byte{
		"app/window.go": []byte(`package app
import native "github.com/egoist/mygo/ui"
type window struct { element *native.Element; other *int }
`),
		"app/view.go": []byte(`package app
import "github.com/egoist/mygo/ui"
func(w *window)view(c *ui.Context){if w.element!=nil{w.element.Focus()};if w.other!=nil{println(*w.other)}}
func newWindow()(*window,int){return &window{},0}
func makeElement(c *ui.Context)*ui.Element{return ui.Text(c,"hello")}
`),
		"app/view_test.go": []byte(`package app
import "github.com/egoist/mygo/ui"
func check(c *ui.Context){w,_:=newWindow();if w.element==nil{w.element=makeElement(c)};e:=makeElement(c);if e!=nil{e.Focus()}}
`),
	}
	results, err := Files(sources)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(results["app/view.go"].Source), "w.element.Valid()") || !strings.Contains(string(results["app/view.go"].Source), "w.other != nil") {
		t.Fatal(string(results["app/view.go"].Source))
	}
	for _, want := range []string{"!w.element.Valid()", "e.Valid()"} {
		if !strings.Contains(string(results["app/view_test.go"].Source), want) {
			t.Fatal(string(results["app/view_test.go"].Source))
		}
	}
	migrated := map[string][]byte{}
	for name, r := range results {
		migrated[name] = r.Source
	}
	again, err := Files(migrated)
	if err != nil {
		t.Fatal(err)
	}
	for name, r := range again {
		if r.Changed {
			t.Fatalf("second migration changed %s", name)
		}
	}
}
