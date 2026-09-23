package stock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestIsWholeBodyLoop(t *testing.T) {
	src := `package p
func has(xs []int, x int) bool { for _, v := range xs { if v == x { return true } }; return false }
func mixed(xs []int, x int) bool { n := 0; for _, v := range xs { if v == x { n++ } }; return n > 0 }`
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"has": true, "mixed": false}
	for _, d := range f.Decls {
		fd := d.(*ast.FuncDecl)
		if got := isWholeBodyLoop(fd.Body.List); got != want[fd.Name.Name] {
			t.Errorf("%s: got %v", fd.Name.Name, got)
		}
	}
}
