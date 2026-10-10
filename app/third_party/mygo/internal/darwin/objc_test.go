//go:build darwin

package darwin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestSuperFromDefinedClass rejects purego's objc.ID.SendSuper, which
// resolves super from the object's class: once key-value observing gives
// the object a subclass of its own, it calls the override again, until the
// stack overflows. Overrides call sendSuper with the class they are defined
// in.
func TestSuperFromDefinedClass(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "SendSuper" {
				t.Errorf("%s: SendSuper resolves super from the object's class; call sendSuper", fset.Position(s.Pos()))
			}
			return true
		})
	}
}
