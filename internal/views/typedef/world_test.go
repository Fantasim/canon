package typedef_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/typedef"
)

// noFold folds nothing: these fixtures have no consts to fold.
type noFold struct{}

func (noFold) Fold(context.Context, check.Object, syntax.Expr, *check.Info) (value.Value, bool) {
	return nil, false
}

const (
	withFunc = "package a\n\nenum Goal { kill, visit }\n\nrecord Kind {\n  goal: Goal\n}\n\n" +
		"type Target(k: Kind) = match k.goal {\n  kill => String\n  visit => Int\n}\n"
	withoutFunc = "package a\n\nrecord Kind {\n  n: Int\n}\n"
)

// checked is package a of src, checked.
func checked(t *testing.T, src string) *check.Program {
	t.Helper()
	set := &source.FileSet{}
	f, err := set.Add("a/a.canon", "/a/a.canon", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	bags := check.Bags{"a": diag.NewBag(set, "a")}
	file := syntax.Parse(f, syntax.FileSource, bags["a"])
	prog := check.Check(context.Background(), project.New("t", project.Version{Minor: 1}), []*syntax.File{file}, bags, noFold{})
	if prog == nil || bags["a"].Summary().Errors != 0 {
		t.Fatalf("check: %v", bags["a"].Findings())
	}
	return prog
}

// VIEWMODEL.md 12.3, log-2026-09-29 "Drivers review": the drivers' program is asked only for a
// type function; one it lacks is ErrNoTypeFunc (a compiler bug), its own failure is Err.
func TestDriversWorld(t *testing.T) {
	failed := errors.New("resolving failed")
	empty := func(prog *check.Program) typedef.Resolve {
		return func() (*typedef.World, error) {
			return &typedef.World{Program: &check.Program{Info: prog.Info}, Colls: encode.NewColls(nil)}, nil
		}
	}
	for _, c := range []struct {
		name, src string
		resolve   func(*check.Program) typedef.Resolve
		asked     bool
		want      error
	}{
		{"lacks the function", withFunc, empty, true, typedef.ErrNoTypeFunc},
		{"resolving fails", withFunc, func(*check.Program) typedef.Resolve {
			return func() (*typedef.World, error) { return nil, failed }
		}, true, failed},
		{"no type function", withoutFunc, empty, false, nil},
	} {
		prog := checked(t, c.src)
		asked := false
		resolve := c.resolve(prog)
		in := typedef.Input{Program: prog, Drivers: func() (*typedef.World, error) { asked = true; return resolve() }}
		s := typedef.New(context.Background(), in, "a")
		s.Section()
		if err := s.Err(); asked != c.asked || !errors.Is(err, c.want) || (c.want == nil) != (err == nil) {
			t.Errorf("%s: asked %t, Err %v; want asked %t, %v", c.name, asked, err, c.asked, c.want)
		}
	}
}
