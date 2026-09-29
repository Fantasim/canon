package load_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// definesExpr is `load.defines(path[, prefix: p])` (WIRE.md §6.8).
func definesExpr(path string, prefix string) *syntax.LoadExpr {
	e := &syntax.LoadExpr{
		Method: &syntax.Ident{Name: "defines"},
		Args:   []*syntax.Arg{{Value: &syntax.StringLit{Parts: []syntax.StringPart{{Text: path}}}}},
	}
	if prefix != "" {
		e.Args = append(e.Args, &syntax.Arg{
			Name: &syntax.Ident{Name: "prefix"}, Value: &syntax.StringLit{Parts: []syntax.StringPart{{Text: prefix}}},
		})
	}
	return e
}

// definesTableType is `table Define` (WIRE.md §6.8).
func definesTableType() *types.TableType {
	return &types.TableType{Elem: types.DefineType}
}

// defineOf is table's accepted name's value and whether it is present.
func defineOf(table *value.Table, name string) (int64, bool) {
	for _, r := range table.Entries {
		if r.Ident.Key.S == name {
			return r.Fields[0].(*value.Int).V, true
		}
	}
	return 0, false
}

// WIRE.md §6.8's grammar: hex/octal/decimal, suffixes, bitwise and shift, unary -, name refs.
func TestDefinesGrammar(t *testing.T) {
	content := "" +
		"#define A 0x10\n" +
		"#define B 010\n" +
		"#define C 10UL\n" +
		"#define D (A | 1)\n" +
		"#define E (A & 0x11)\n" +
		"#define F (1 << 4)\n" +
		"#define G (A >> 4)\n" +
		"#define H (A + B - 1)\n" +
		"#define I (-A)\n" +
		"#define J A\n" + // a name reference, read after A is already accepted
		"#define K A\n" + // another, distinct name: not a duplicate of J
		"#define GUARD\n" + // an include guard: skipped silently, not counted
		"#define MACRO(x) (x)\n" // a function-like macro: skipped, counted
	l, req := loaderFor(t, map[string]string{"a.h": content})
	v, ok, err := l.Load(context.Background(), req, definesExpr("a.h", ""), definesTableType())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v findings=%+v", ok, err, req.Bag.Findings())
	}
	table := v.(*value.Table)
	for name, want := range map[string]int64{
		"A": 0x10, "B": 010, "C": 10, "D": 0x11, "E": 0x10, "F": 16, "G": 1, "H": 0x10 + 010 - 1, "I": -16, "J": 16, "K": 16,
	} {
		got, ok := defineOf(table, name)
		if !ok || got != want {
			t.Errorf("%s: got=%d ok=%v, want %d", name, got, ok, want)
		}
	}
	if _, ok := defineOf(table, "GUARD"); ok {
		t.Error("GUARD: an empty-body define should not be accepted")
	}
	l.FinishDefines()
	fd := req.Bag.Findings()
	if len(fd) != 1 || fd[0].Code != diag.W7101.Def().Code {
		t.Errorf("findings = %+v, want one %s (MACRO only, GUARD skipped silently)", fd, diag.W7101.Def().Code)
	}
}

// WIRE.md §6.8: a duplicate #define of a different value points back at the first.
func TestDefinesDuplicateDifferentValueIsE7102(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.h": "#define X 1\n#define X 2\n"})
	_, ok, err := l.Load(context.Background(), req, definesExpr("a.h", ""), definesTableType())
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a refused decode", ok, err)
	}
	if len(req.Bag.Findings()) != 1 || req.Bag.Findings()[0].Code != diag.E7102.Def().Code {
		t.Errorf("findings = %+v, want one %s", req.Bag.Findings(), diag.E7102.Def().Code)
	}
}

// WIRE.md §6.8: a "//" or "/* */" inside a string or character literal is not a comment.
func TestDefinesCommentInsideStringLiteral(t *testing.T) {
	content := "#define SLASH \"a // b\"\n#define X 1 /* c */\n#define Y 2 // trailing\n"
	l, req := loaderFor(t, map[string]string{"a.h": content})
	v, ok, err := l.Load(context.Background(), req, definesExpr("a.h", ""), definesTableType())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v findings=%+v", ok, err, req.Bag.Findings())
	}
	table := v.(*value.Table)
	if _, ok := defineOf(table, "SLASH"); ok {
		t.Error("SLASH: a string-valued define is not a supported integer expression")
	}
	for name, want := range map[string]int64{"X": 1, "Y": 2} {
		got, ok := defineOf(table, name)
		if !ok || got != want {
			t.Errorf("%s: got=%d ok=%v, want %d (comment inside SLASH's string must not swallow it)", name, got, ok, want)
		}
	}
}

// WIRE.md §6.8, DECISIONS 195: no depth limit; a 3M-deep expression is accepted with its value.
func TestDefinesDeepNestingAccepted(t *testing.T) {
	const depth = 3_000_000
	nest := strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth)
	l, req := loaderFor(t, map[string]string{"a.h": "#define X " + nest + "\n#define Y " + strings.Repeat("-", depth) + "X\n"})
	v, ok, err := l.Load(context.Background(), req, definesExpr("a.h", ""), definesTableType())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v findings=%+v", ok, err, req.Bag.Findings())
	}
	for name, want := range map[string]int64{"X": 1, "Y": 1} {
		if got, ok := defineOf(v.(*value.Table), name); !ok || got != want {
			t.Errorf("%s: got=%d ok=%v, want %d", name, got, ok, want)
		}
	}
	l.FinishDefines()
	if fd := req.Bag.Findings(); len(fd) != 0 {
		t.Errorf("findings = %+v, want none", fd)
	}
}

// WIRE.md §6.8 steps 2-3: splicing first; an unterminated literal (`don't`) ends at the next LF.
func TestDefinesLiteralsAndSplices(t *testing.T) {
	content := "#error don't use\n" +
		"#define M 1 // note\n" +
		"#define N 2 /* x */\n" +
		"#pragma \"open\n" +
		"#define S \"a\\\nb\" \n" + // a spliced string: one define, skipped
		"#define P 3 // c\\\n#define HIDDEN 4\n" + // the splice continues the comment
		"#define Q \\\n5\n" +
		"#define R \\\\\n\n" // `\\` then LF: the second `\` splices, the first stays
	l, req := loaderFor(t, map[string]string{"a.h": content})
	v, ok, err := l.Load(context.Background(), req, definesExpr("a.h", ""), definesTableType())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v findings=%+v", ok, err, req.Bag.Findings())
	}
	table := v.(*value.Table)
	for name, want := range map[string]int64{"M": 1, "N": 2, "P": 3, "Q": 5} {
		if got, ok := defineOf(table, name); !ok || got != want {
			t.Errorf("%s: got=%d ok=%v, want %d", name, got, ok, want)
		}
	}
	if len(table.Entries) != 4 {
		t.Errorf("accepted %d defines, want M N P Q", len(table.Entries))
	}
	l.FinishDefines()
	if fd := req.Bag.Findings(); len(fd) != 1 || fd[0].Code != diag.W7101.Def().Code || !strings.Contains(fd[0].Message, "2 defines") {
		t.Errorf("findings = %+v, want one %s counting S and R", fd, diag.W7101.Def().Code)
	}
}

// WIRE.md §6.8: a Scratch read is not cached, so the build's own call still reports E7102, W7101.
func TestDefinesScratchReadIsNotCached(t *testing.T) {
	content := "#define X 1\n#define X 2\n#define A_BAD \"s\"\n#define B_BAD \"t\"\n"
	l, build := loaderFor(t, map[string]string{"a.h": content})
	scratch := load.Request{Pkg: "p", Bag: diag.NewBag(l.Set, "p"), Scratch: true}
	for _, req := range []load.Request{scratch, build} {
		prefix := map[bool]string{true: "B_", false: "A_"}[req.Scratch]
		if _, ok, err := l.Load(context.Background(), req, definesExpr("a.h", prefix), definesTableType()); err != nil || ok {
			t.Fatalf("scratch=%v: ok=%v err=%v, want a refused decode", req.Scratch, ok, err)
		}
	}
	l.FinishDefines()
	codes := func(bag *diag.Bag) (out []diag.Code) {
		for _, fd := range bag.Findings() {
			out = append(out, fd.Code)
		}
		return out
	}
	e7102, w7101 := diag.E7102.Def().Code, diag.W7101.Def().Code
	if got := codes(scratch.Bag); len(got) != 1 || got[0] != e7102 {
		t.Errorf("scratch bag = %v, want [%s] (nothing is finished for it)", got, e7102)
	}
	if got := codes(build.Bag); len(got) != 2 || got[0] != e7102 || got[1] != w7101 {
		t.Errorf("build bag = %v, want [%s %s]", got, e7102, w7101)
	}
	if fd := build.Bag.Findings(); len(fd) == 2 && !strings.Contains(fd[1].Message, "A_BAD") {
		t.Errorf("%s = %q, want A_BAD alone (B_ is only the scratch call's prefix)", w7101, fd[1].Message)
	}
}

// API.md S5: a header read once serves the next call from the cache, which Reused is told of.
func TestDefinesCacheHitIsReported(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.h": "#define A 1\n"})
	var reused []string
	l.Reused = func(abs string) { reused = append(reused, abs) }
	for range 2 {
		if _, _, err := l.Load(context.Background(), req, definesExpr("a.h", ""), definesTableType()); err != nil {
			t.Fatal(err)
		}
	}
	if len(reused) != 1 || reused[0] != projectDir+"/a.h" {
		t.Errorf("reused %v, want the header once", reused)
	}
}

// WIRE.md §6.8: two load.defines calls of one file aggregate their skips into one W7101.
func TestDefinesW7101AggregatesAcrossPrefixedCalls(t *testing.T) {
	content := "#define FOO(x) x\n#define A_ONE 1\n#define B_BAD \"s\"\n#define A_BAD \"t\"\n"
	l, req := loaderFor(t, map[string]string{"a.h": content})
	_, _, err := l.Load(context.Background(), req, definesExpr("a.h", "A_"), definesTableType())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = l.Load(context.Background(), req, definesExpr("a.h", "B_"), definesTableType())
	if err != nil {
		t.Fatal(err)
	}
	l.FinishDefines()
	fd := req.Bag.Findings()
	if len(fd) != 1 || fd[0].Code != diag.W7101.Def().Code {
		t.Fatalf("findings = %+v, want one %s (FOO excluded: matches neither prefix)", fd, diag.W7101.Def().Code)
	}
}
