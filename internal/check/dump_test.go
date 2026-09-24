package check_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	_ "github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
)

const (
	infoDir  = "testdata/info"
	filePerm = 0o600
	dirPerm  = 0o750
)

// dumper prints Info's facts about the nodes of one file, one line per fact, in source order.
type dumper struct {
	buf  bytes.Buffer
	f    *syntax.File
	info *check.Info
}

func (d *dumper) at(n syntax.Node) string {
	line, col := d.f.Src.Position(d.f.Span(n).Start)
	return fmt.Sprintf("%d:%d", line, col)
}

func (d *dumper) printf(n syntax.Node, format string, args ...any) {
	fmt.Fprintf(&d.buf, "%s %s %s\n", d.at(n), n.Kind(), fmt.Sprintf(format, args...))
}

// objText names an object: its kind and qualified name, and its type when it has one.
func objText(o check.Object) string {
	if o == nil {
		return "<nil>"
	}
	name := o.Name()
	if o.Pkg() != "" {
		name = o.Pkg() + "." + name
	}
	if o.Type() != nil {
		return fmt.Sprintf("%s %s: %s", o.Kind(), name, o.Type())
	}
	return fmt.Sprintf("%s %s", o.Kind(), name)
}

func convText(c *check.Conversion) string {
	if c == nil {
		return ""
	}
	s := fmt.Sprintf("%s(%s -> %s)", c.Kind, c.From, c.To)
	if c.Key != nil {
		s += " key " + convText(c.Key)
	}
	if c.Inner != nil {
		s += " inner " + convText(c.Inner)
	}
	return s
}

func (d *dumper) node(n syntax.Node) bool {
	if n == nil {
		return false
	}
	switch n.(type) {
	case *syntax.Annotation, *syntax.DocComment:
		return false
	}
	d.ident(n)
	if e, ok := n.(syntax.Expr); ok {
		d.expr(e)
	}
	if t, ok := n.(syntax.Type); ok {
		if tt := d.info.TypeExprs[t]; tt != nil {
			d.printf(n, "type %s", tt)
		}
	}
	return true
}

func (d *dumper) ident(n syntax.Node) {
	switch n := n.(type) {
	case *syntax.Ident:
		if o := d.info.Defs[n]; o != nil {
			d.printf(n, "%s def %s", n.Name, objText(o))
		}
		if o := d.info.NameUses[n]; o != nil {
			d.printf(n, "%s names %s", n.Name, objText(o))
		}
	case *syntax.IdentExpr:
		if o := d.info.Uses[n]; o != nil {
			d.printf(n, "%s uses %s", n.Name, objText(o))
		}
		if c := d.info.Keys[n]; c != nil {
			d.printf(n, "%s key of %s", n.Name, c)
		}
		if d.info.Symbols[n] {
			d.printf(n, "%s symbol", n.Name)
		}
	}
}

func (d *dumper) expr(e syntax.Expr) {
	if t := d.info.Types[e]; t != nil {
		d.printf(e, ": %s", t)
	}
	if c := d.info.Conv[e]; c != nil {
		d.printf(e, "conv %s", convText(c))
	}
	switch x := e.(type) {
	case *syntax.SelectorExpr:
		if s := d.info.Selections[x]; s != nil {
			d.printf(e, "select %s %s recv %s deref=%v", s.Kind, objText(s.Obj), s.Recv, s.Deref)
		}
	case *syntax.CallExpr:
		if c := d.info.Calls[x]; c != nil {
			d.printf(e, "call %s %s %s#%d %s", c.Kind, objText(c.Obj), c.Builtin, c.Overload, typeList(c.TypeArgs))
		}
	case *syntax.BraceLit:
		if k, ok := d.info.Literals[x]; ok {
			d.printf(e, "literal %s", k)
		}
	case *syntax.MatchExpr:
		if m := d.info.Matches[x]; m != nil {
			d.printf(e, "match %s covers %v exhaustive=%v unreachable %v", m.Scrutinee, m.Covers, m.Exhaustive, m.Unreachable)
		}
	}
}

func typeList(ts []types.Type) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = t.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// dumpFile prints the facts of one file, then the broken declarations of its package.
func dumpFile(f *syntax.File, prog *check.Program) []byte {
	d := &dumper{f: f, info: prog.Info}
	syntax.Inspect(f, d.node)
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			if o.File() == f && prog.Info.Broken[o] {
				fmt.Fprintf(&d.buf, "broken %s\n", objText(o))
			}
		}
	}
	return d.buf.Bytes()
}

// IMPLEMENTATION-PLAN §4.7.
func TestInfoGoldens(t *testing.T) {
	update := flag.Lookup("update").Value.(flag.Getter).Get().(bool)
	l := loadExamples(t, "teamboard", "sovcommon/ui", "sovcommon/roles")
	prog, _ := l.run(t)
	for _, f := range l.files {
		got := dumpFile(f, prog)
		path := filepath.Join(infoDir, filepath.FromSlash(f.Src.Path)+".txt")
		if update {
			if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, filePerm); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s differs from its golden (go test -update): %v", path, err)
		}
	}
}
