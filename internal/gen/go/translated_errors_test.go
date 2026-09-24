package gogen_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
)

// twice is a package-level translated fn with one vector.
func twice() *ir.ExportFn {
	return spec{name: "twice", params: params(intT, "x"), result: intT, body: bin(ir.OpMul, intT, prm(0, intT), lit(intT, num(2))),
		vecs: []vec{ok(nil, num(4), num(2))}}.build(0)
}

// A translated fn stage E left without a body, a source file or vectors, or with its internal error, is a stage E defect (CONFORMANCE.md §6, CODEGEN.md §2.5 T3, decision 196).
func TestTranslatedMalformed(t *testing.T) {
	cases := map[string]func(*ir.ExportFn){
		"no body":    func(fn *ir.ExportFn) { fn.Body = nil },
		"no file":    func(fn *ir.ExportFn) { fn.File = "" },
		"no vectors": func(fn *ir.ExportFn) { fn.Vectors = nil },
		"its error":  func(fn *ir.ExportFn) { fn.Err = fmt.Errorf("%w: twice", ir.ErrInternal) },
		"a bad read": func(fn *ir.ExportFn) { fn.Body = rd(0, intT) },
		"a bad let":  func(fn *ir.ExportFn) { fn.Body = lcl("z") },
	}
	for _, name := range []string{"no body", "no file", "no vectors", "its error", "a bad read", "a bad let"} {
		fn := twice()
		cases[name](fn)
		p := pkg()
		p.Fns = []*ir.ExportFn{fn}
		if err := generateErr(p, nil); !errors.Is(err, gogen.ErrMalformed) {
			t.Errorf("%s: got %v, want ErrMalformed", name, err)
		}
	}
	fn := twice()
	fn.Err = fmt.Errorf("%w: twice", ir.ErrInternal)
	p := pkg()
	p.Fns = []*ir.ExportFn{fn}
	if err := generateErr(p, nil); !errors.Is(err, ir.ErrInternal) {
		t.Errorf("stage E's error is not kept: %v", err)
	}
}

// CONFORMANCE.md §2.2: a lookup whose result a pure function cannot hold is refused, naming it.
func TestTranslatedLookupRefused(t *testing.T) {
	c := newCalc()
	p := c.pkg()
	c.loud.Result = optT(boolT)
	_, err := gogen.Generate(p, p.Emits[0])
	var d *gogen.DetailError
	if !errors.Is(err, gogen.ErrUnsupported) || !errors.As(err, &d) || d.Subject != "loud" {
		t.Errorf("got %v, want ErrUnsupported naming loud", err)
	}
}

// CONFORMANCE.md §7.1: two fns whose tests would share a name are a collision, never two tests of one name.
func TestTranslatedTestNamesCollide(t *testing.T) {
	a, ab := record("A"), record("AB")
	m := twice()
	m.Name = "bC"
	n := twice()
	n.Name = "c"
	a.Methods, ab.Methods = []*ir.ExportFn{m}, []*ir.ExportFn{n}
	err := generateErr(pkg(a, ab), nil)
	if !errors.Is(err, gogen.ErrNameCollision) || !strings.Contains(err.Error(), "TestABCConformance") {
		t.Errorf("got %v, want a collision of TestABCConformance", err)
	}
}

// CODEGEN.md §5.10: data mode refuses stored package fns, not translated ones; they get their conformance file.
func TestTranslatedDataMode(t *testing.T) {
	p := dataThing()
	p.Fns = []*ir.ExportFn{twice()}
	files, err := gogen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || files[2].Path != "demo_conformance_test.go" || !strings.Contains(string(files[1].Content), "func Twice(x int64) int64") {
		t.Errorf("got %d files, want the main file with Twice and demo_conformance_test.go", len(files))
	}
}
