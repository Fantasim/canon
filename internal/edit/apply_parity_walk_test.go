package edit_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
)

var updateParity = flag.Bool("update-parity", false, "rewrite testdata/parity/mismatches.txt instead of comparing")

// parityGolden lists every operation of the parity walk on which Editable and Apply still differ.
var parityGolden = filepath.Join("testdata", "parity", "mismatches.txt")

// parityLaws are the layer settings the walk opens law with: none, each layer active or not as
// the edit layer, and stacked layers.
var parityLaws = []struct {
	layers    []string
	editLayer string
}{
	{nil, ""}, {[]string{"dev"}, ""}, {[]string{"dev"}, "dev"}, {nil, "dev"}, {nil, "lc"}, {nil, "ld"},
	{nil, "le"}, {nil, "lf"}, {nil, "stage"}, {[]string{"dev"}, "stage"}, {[]string{"dev"}, "lc"},
	{[]string{"la", "lb"}, "la"}, {[]string{"la", "lb"}, "lb"}, {nil, "work"},
	{[]string{"lc"}, "lc"}, {[]string{"ld"}, "ld"}, {[]string{"le"}, "le"}, {[]string{"lf"}, "lf"}, {[]string{"stage"}, "stage"},
	{nil, "lt"}, {[]string{"lt"}, "lt"},
}

// parityTally counts the walk's verdicts, those Apply hid behind ErrBadOp or ErrBadValue, its
// failures inside the compiler, and the Sets whose own value changes a key (keyed).
type parityTally struct{ verdicts, hidden, internal, keyed int }

// letValues are every value of the session's lets, by positional path.
func letValues(s session) []candidate {
	var out []candidate
	for _, pkg := range s.a.Program().Packages {
		for _, obj := range pkg.Decls {
			if obj.Kind() != check.ObjLet {
				continue
			}
			if v, ok := s.a.Force(eval.Root{Pkg: pkg.Path, Name: obj.Name()}); ok {
				out = collect(out, pkg.Path+":"+obj.Name(), v)
			}
		}
	}
	return out
}

// walkOps are parityOps on c, then every operation the fuzz draws on it with values that type
// and differ from c's (API.md M1): a Set that changes the value runs the Undo and line checks.
func walkOps(fz *fuzzProject, s session, c candidate, path string) []edit.Operation {
	ops := parityOps(path, ownLiteral(s, c.v))
	for _, k := range allOps {
		if k.kind == edit.OpUnretire {
			continue
		}
		if op, ok := k.draw(fz, c, sweepDraw(c.v)); ok {
			op.Path = path
			ops = append(ops, op)
		}
	}
	return ops
}

// mismatches are the operations of the walk on every value of s whose two verdicts differ,
// each as `<name>: <op> <path>: Editable <reason>, Apply <reason>`, counted in n.
func mismatches(t *testing.T, name string, s session, n *parityTally) []string {
	var out []string
	cands := letValues(s)
	fz := &fuzzProject{env: s.env, a: s.a, cands: cands, byPath: map[string]int{}, ops: 1, copies: benchCopies}
	for i, c := range cands {
		fz.byPath[c.path] = i
	}
	for _, c := range cands {
		p, _ := edit.Parse(c.path)
		r, err := edit.Resolve(s.snap, p)
		if err != nil {
			continue
		}
		for _, op := range walkOps(fz, s, c, r.Canonical) {
			v := verdictOf(t, s, r, op)
			n.count(v)
			if keyed := op.Kind == edit.OpSet && v.editable == edit.ReasonNone && v.apply == edit.ReasonKey; keyed {
				n.keyed++ // API.md E5: the Set's value changes the key; Editable takes no value
				continue
			}
			if v.editable != v.apply || v.internal || v.misnamed {
				out = append(out, fmt.Sprintf("%s: %s %s: Editable %s, Apply %s", name, parityOpNames[op.Kind], r.Canonical, parityReasons[v.editable], v.applyText()))
			}
		}
	}
	return out
}

func (n *parityTally) count(v verdict) {
	n.verdicts++
	if v.hidden {
		n.hidden++
	}
	if v.internal {
		n.internal++
	}
}

// ownLiteral is v as an operation's value, or none when no Source states it.
func ownLiteral(s session, v value.Value) edit.Lit {
	if lit, err := edit.SourceOf(context.Background(), s.env, s.snap, v); err == nil {
		return lit
	}
	return edit.None{}
}

// API.md W5, E21 (log-2026-11-01 wave G): where Editable gives a reason, Apply refuses the
// operation alone with it, refuses no editable one as not editable, and never fails inside the
// compiler, on every value of law, keptLine and the edit goldens; parityGolden holds the rest.
func TestEditableApplyParity(t *testing.T) {
	var got []string
	var n parityTally
	for _, c := range parityLaws {
		name := fmt.Sprintf("law layers=%s edit=%s", strings.Join(c.layers, ","), c.editLayer)
		got = append(got, mismatches(t, name, open(t, law, c.layers, c.editLayer, "a", "b", "u"), &n)...)
	}
	for _, layers := range [][]string{{"dev"}, nil} {
		name := fmt.Sprintf("keptLine layers=%s edit=dev", strings.Join(layers, ","))
		got = append(got, mismatches(t, name, open(t, keptLine, layers, "dev", "p"), &n)...)
	}
	for _, c := range []struct {
		layers    []string
		editLayer string
	}{{[]string{"dev"}, "dev"}, {[]string{"lx", "dev"}, "dev"}, {nil, "dev"}, {nil, "di"}} {
		name := fmt.Sprintf("mapLines layers=%s edit=%s", strings.Join(c.layers, ","), c.editLayer)
		got = append(got, mismatches(t, name, open(t, mapLines, c.layers, c.editLayer, "p"), &n)...)
	}
	paths, err := filepath.Glob(filepath.Join("testdata", "edits", "*.txtar"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no edit goldens: %v", err)
	}
	for _, p := range paths {
		ar, err := txtar.ParseFile(p)
		if err != nil {
			t.Fatal(err)
		}
		c := readCase(t, ar)
		got = append(got, mismatches(t, strings.TrimSuffix(filepath.Base(p), ".txtar"), open(t, c.fsys, c.layers, c.editLayer, c.pkgs...), &n)...)
	}
	t.Logf("%d verdicts, %d hidden behind ErrBadOp or ErrBadValue, %d Sets changing a key (E5), %d inside the compiler", n.verdicts, n.hidden, n.keyed, n.internal)
	if n.internal > 0 {
		t.Errorf("%d operations fail inside the compiler", n.internal)
	}
	compareParity(t, []byte(strings.Join(got, "\n")+"\n"), n)
}

// compareParity compares the walk's mismatches with parityGolden, or writes them there.
func compareParity(t *testing.T, data []byte, n parityTally) {
	if *updateParity {
		if err := os.WriteFile(parityGolden, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if want, _ := os.ReadFile(parityGolden); !bytes.Equal(data, want) {
		t.Errorf("%d verdicts; the mismatches differ from %s (run with -update-parity and read the diff):\n%s", n.verdicts, parityGolden, data)
	}
}
