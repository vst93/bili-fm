package uivet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

type testImporter struct{ ui *types.Package }

func (i testImporter) Import(path string) (*types.Package, error) { return i.ui, nil }
func TestStoredValuesAndGoroutineCaptures(t *testing.T) {
	ui := types.NewPackage(uiPath, "ui")
	for _, name := range []string{"Element", "Context", "Handle", "Services"} {
		obj := types.NewTypeName(token.NoPos, ui, name, nil)
		types.NewNamed(obj, types.NewStruct(nil, nil), nil)
		ui.Scope().Insert(obj)
	}
	ui.Scope().Insert(types.NewFunc(token.NoPos, ui, "Box", types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewVar(token.NoPos, ui, "c", types.NewPointer(ui.Scope().Lookup("Context").Type()))), types.NewTuple(types.NewVar(token.NoPos, ui, "", ui.Scope().Lookup("Element").Type())), false)))
	ui.MarkComplete()
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "app.go", `package app
import native "github.com/egoist/mygo/ui"
type E = native.Element
type app struct { bad E; context *native.Context; safe native.Handle; services native.Services; view func(*native.Context) }
var saved native.Element
func view(c *native.Context){
 e:=native.Box(c)
 go func(){ _=e; _=c }()
 go func(e native.Element){}(e)
 var h native.Handle
 go func(){_=h}()
}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}}
	pkg, err := (&types.Config{Importer: testImporter{ui}}).Check("example/app", fs, []*ast.File{f}, info)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := Analyze(fs, []*ast.File{f}, pkg, info)
	if len(diagnostics) != 6 {
		t.Fatalf("got %d: %#v", len(diagnostics), diagnostics)
	}
	for _, d := range diagnostics {
		if strings.Contains(d.Message, "safe") || strings.Contains(d.Message, "services") {
			t.Fatal(d)
		}
	}
}
