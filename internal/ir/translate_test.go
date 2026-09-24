package ir_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
)

const bodiesFile = "bodies.txt"

// CONFORMANCE.md §2.2, §2.3, §7.2 (CODEGEN.md §2.5 T3): each case's translated fns get the portable body and the reads of self gen/go and gen/cpp translate, every export fn its source file and declaration rank; the findings follow (none but the case's own).
func TestTranslatedBodies(t *testing.T) {
	golden.Run(t, "testdata/translate/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := newWorld(t)
		for _, f := range c.Archive.Files {
			if f.Name != bodiesFile {
				w.add(t, f.Name, f.Data)
			}
		}
		w.calls = w.fixtureCalls
		var b strings.Builder
		for _, p := range w.build(t) {
			dumpFns(&b, p)
		}
		b.WriteString(w.findings(t))
		return []byte(b.String())
	}, golden.Expected(bodiesFile))
}

// ownedFn is an export fn with the name messages give it.
type ownedFn struct {
	label string
	fn    *ir.ExportFn
}

// dumpFns prints p's export fns in declaration order (Order), with each translated one's reads and body.
func dumpFns(b *strings.Builder, p *ir.Package) {
	var fns []ownedFn
	for _, ty := range p.Types {
		switch x := ty.(type) {
		case *ir.Record:
			fns = appendOwned(fns, x.Name+".", x.Methods)
		case *ir.Variant:
			for _, c := range x.Cases {
				fns = appendOwned(fns, x.Name+"."+c.Name+".", c.Methods)
			}
		}
	}
	fns = appendOwned(fns, "", p.Fns)
	slices.SortFunc(fns, func(a, b ownedFn) int { return cmp.Compare(a.fn.Order, b.fn.Order) })
	fmt.Fprintf(b, "package %s dir=%s\n", p.Name, p.Dir)
	for _, f := range fns {
		fmt.Fprintf(b, "fn %s file=%s order=%d kind=%s\n", f.label, f.fn.File, f.fn.Order, kindNames[f.fn.Kind])
		for i, r := range f.fn.Reads {
			fmt.Fprintf(b, "  read %d %s %v %s opt=%v\n", i, r.Name, r.Path, typeText(&r.Type), r.Optional)
		}
		if f.fn.Kind == ir.FnTranslated {
			fmt.Fprintf(b, "  body %s\n", pexprText(f.fn.Body, "  "))
		}
	}
}

func appendOwned(out []ownedFn, owner string, fns []*ir.ExportFn) []ownedFn {
	for _, fn := range fns {
		out = append(out, ownedFn{label: owner + fn.Name, fn: fn})
	}
	return out
}

var (
	opNames      = []string{"+", "-", "*", "/", "%", "neg", "not", "and", "or", "==", "!=", "<", "<=", ">", ">="}
	builtinNames = []string{"Float", "Int", "min", "max", "abs", "clamp", "floor", "ceil", "round"}
)

// pexprText prints a body as an s-expression, each node with its type; `let` and `if` break lines.
func pexprText(n ir.PExpr, indent string) string {
	in := indent + "  "
	switch x := n.(type) {
	case nil:
		return "none"
	case *ir.Lit:
		return fmt.Sprintf("%s:%s", valueText(x.V), typeText(&x.T))
	case *ir.ParamRef:
		return fmt.Sprintf("param%d:%s", x.Index, typeText(&x.T))
	case *ir.ReadRef:
		return fmt.Sprintf("read%d:%s", x.Index, typeText(&x.T))
	case *ir.LocalRef:
		return fmt.Sprintf("%s:%s", x.Name, typeText(&x.T))
	case *ir.Unary:
		return fmt.Sprintf("(%s:%s %s)", opNames[x.Op], typeText(&x.T), pexprText(x.X, in))
	case *ir.Binary:
		return fmt.Sprintf("(%s:%s %s %s)", opNames[x.Op], typeText(&x.T), pexprText(x.X, in), pexprText(x.Y, in))
	case *ir.Call:
		return fmt.Sprintf("(%s:%s %s)", builtinNames[x.Fn], typeText(&x.T), listText(x.Args, in))
	case *ir.CallFn:
		return fmt.Sprintf("(call %s:%s %s)", x.Fn.Name, typeText(&x.T), listText(x.Args, in))
	case *ir.If:
		return fmt.Sprintf("(if:%s %s\n%sthen %s\n%selse %s)", typeText(&x.T), pexprText(x.Cond, in), in, pexprText(x.Then, in), in, pexprText(x.Else, in))
	case *ir.Let:
		return fmt.Sprintf("(let %s = %s\n%sin %s)", x.Name, pexprText(x.Value, in), in, pexprText(x.Body, in))
	case *ir.Template:
		parts := make([]string, 0, len(x.Parts))
		for _, p := range x.Parts {
			parts = append(parts, fmt.Sprintf("%q", p.Text))
			if p.X != nil {
				parts = append(parts, pexprText(p.X, in))
			}
		}
		return fmt.Sprintf("(template:%s %s)", typeText(&x.T), strings.Join(parts, " "))
	case *ir.Coalesce:
		return fmt.Sprintf("(??:%s %s %s)", typeText(&x.T), pexprText(x.X, in), pexprText(x.Y, in))
	case *ir.IsCase:
		return fmt.Sprintf("(is case%d %s)", x.Case, pexprText(x.X, in))
	case *ir.Block:
		return blockText(x, indent)
	}
	return fmt.Sprintf("?%T", n)
}

// blockText prints a Block one statement a line, a branch's block indented under its `if`.
func blockText(b *ir.Block, indent string) string {
	if b == nil {
		return "none"
	}
	in := indent + "  "
	out := fmt.Sprintf("{:%s", typeText(&b.T))
	for _, st := range b.Stmts {
		switch x := st.(type) {
		case *ir.LetStmt:
			out += fmt.Sprintf("\n%slet %s = %s", in, x.Name, pexprText(x.Value, in))
		case *ir.IfStmt:
			out += fmt.Sprintf("\n%sif %s then %s else %s", in, pexprText(x.Cond, in), blockText(x.Then, in), blockText(x.Else, in))
		case *ir.ReturnStmt:
			out += fmt.Sprintf("\n%sreturn %s", in, pexprText(x.X, in))
		}
	}
	return out + "\n" + indent + "}"
}

func listText(ns []ir.PExpr, indent string) string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = pexprText(n, indent)
	}
	return strings.Join(out, " ")
}

// CONFORMANCE.md §2.3: a refinement of a refined alias keeps both ranges, so Param.Range and ResultRange are their intersection, integer, Float or Duration (ms); of two equal upper ends the exclusive one holds.
func TestNestedRangesIntersect(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(`package a

/// A level.
type Level = Int(1..=150)

/// A ratio.
type Ratio = Float(0.0..=1.0)

/// A wait.
type Wait = Duration(0s..=10s)

/// The level, scaled.
export fn scaled(level: Level(10..=200), r: Ratio(..0.5), top: Level(..150), w: Wait(2s..)) -> Level(..100) { return level }

emit go { out: "@features/a", package: "a" }
`))
	w.calls = w.fixtureCalls
	pkgs := w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	fn := pkgs[0].Fns[0]
	cases := []struct {
		name      string
		got, want *types.Bound
	}{
		{"level", fn.Params[0].Range, &types.Bound{Lo: types.Limit{I: 10}, Hi: types.Limit{I: 150}, HasLo: true, HasHi: true, HiIncluded: true}},
		{"r", fn.Params[1].Range, &types.Bound{Lo: types.Limit{F: 0}, Hi: types.Limit{F: 0.5}, HasLo: true, HasHi: true}},
		{"result", fn.ResultRange, &types.Bound{Lo: types.Limit{I: 1}, Hi: types.Limit{I: 100}, HasLo: true, HasHi: true}},
		{"equal ends", fn.Params[2].Range, &types.Bound{Lo: types.Limit{I: 1}, Hi: types.Limit{I: 150}, HasLo: true, HasHi: true}},
		{"duration", fn.Params[3].Range, &types.Bound{Lo: types.Limit{I: 2000}, Hi: types.Limit{I: 10000}, HasLo: true, HasHi: true, HiIncluded: true}},
	}
	for _, c := range cases {
		if c.got == nil || *c.got != *c.want {
			t.Errorf("%s: range %+v, want %+v", c.name, c.got, c.want)
		}
	}
}

// guards is a body of n `if` statements whose branch returns only sometimes, each falling through to the next.
func guards(n int) string {
	var b strings.Builder
	b.WriteString("package a\n\n/// Guarded.\nexport fn guarded(x: Int, y: Int) -> Int {\n")
	for i := range n {
		fmt.Fprintf(&b, "  if x == %d {\n    let z = y * %d\n    if z > %d { return z }\n  }\n", i, i+1, i)
	}
	b.WriteString("  return 0\n}\n\nemit go { out: \"@features/a\", package: \"a\" }\n")
	return b.String()
}

// nodes counts the nodes of a body, walking it as a tree: a shared subtree counts each time it is met.
func nodes(n ir.PExpr) int {
	switch x := n.(type) {
	case *ir.Block:
		count := 1
		for _, st := range x.Stmts {
			count += stmtNodes(st)
		}
		return count
	case *ir.Binary:
		return 1 + nodes(x.X) + nodes(x.Y)
	case nil:
		return 0
	}
	return 1
}

func stmtNodes(st ir.Stmt) int {
	switch s := st.(type) {
	case *ir.LetStmt:
		return 1 + nodes(s.Value)
	case *ir.IfStmt:
		n := 1 + nodes(s.Cond) + nodes(s.Then)
		if s.Else != nil {
			n += nodes(s.Else)
		}
		return n
	case *ir.ReturnStmt:
		return 1 + nodes(s.X)
	}
	return 1
}

// CONFORMANCE.md §2.2: 22 guards whose branches fall through give a body linear in the source, even walked as a tree.
func TestGuardsStayLinear(t *testing.T) {
	const n, perGuard = 22, 20
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(guards(n)))
	w.calls = w.fixtureCalls
	pkgs := w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	body := pkgs[0].Fns[0].Body
	if got := nodes(body); body == nil || got > n*perGuard {
		t.Errorf("the body has %d nodes walked as a tree, want at most %d", got, n*perGuard)
	}
}

// decision 196: a translated fn stage E cannot translate while no bag holds an error (here: no folder for its literals) is a visible ErrInternal, never a silent missing body.
func TestUntranslatedWithoutErrorIsInternal(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte("package a\n\n/// Plus one.\nexport fn inc(n: Int) -> Int { return n + 1 }\n\nemit go { out: \"@features/a\", package: \"a\" }\n"))
	w.check(t)
	pkgs := ir.Build(context.Background(), ir.Input{Program: w.prog, Project: w.proj, Bags: w.bags, Host: w})
	fn := pkgs[0].Fns[0]
	if fn.Body != nil || !errors.Is(fn.Err, ir.ErrInternal) {
		t.Errorf("body %v, err %v: want no body and ir.ErrInternal", fn.Body, fn.Err)
	}
}
