package eval_test

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
	"golang.org/x/tools/txtar"
)

const (
	memoFile   = "memo"
	memoFormat = "stored %d unkept %d"
)

// memoed is a program parsed and checked once, then evaluated afresh as often as a test likes,
// cold or with a memo: the options, loads and first roots of those evaluations.
type memoed struct {
	p     *program
	prog  *check.Program
	bags  []string
	opt   eval.Options
	load  loader
	first []eval.Root
	as    func(*host) eval.Host // the host the evaluator is given, when not the fixture's (memo_load_test.go)
}

// outcome is what one evaluation produced: each value's text, the findings, the steps spent,
// and the memo's entries replayed, kept and left unkept.
type outcome struct {
	dump, deep, findings string
	steps                int64
	hits, stored, unkept int
	loads                struct{ hits, stored, unkept int } // the same counts for loads
}

// checkOnce checks p for evaluations with opt.
func checkOnce(t testing.TB, p *program, opt eval.Options) *memoed {
	t.Helper()
	bags := check.Bags{}
	prog := check.Check(context.Background(), exampleProject(), p.files, bags, eval.NewFolder(bags, opt))
	return &memoed{p: p, prog: prog, bags: slices.Sorted(maps.Keys(bags)), opt: opt}
}

// evaluate runs stages A to D with a new evaluator and new bags, replaying from memo in epoch
// when memo is set.
func (m *memoed) evaluate(t testing.TB, memo *eval.Memo, epoch uint64) outcome {
	t.Helper()
	b := &build{prog: m.p, bags: check.Bags{}, checked: m.prog, values: map[eval.Root]value.Value{}, order: slices.Clone(m.first)}
	for _, name := range m.bags {
		b.bags[name] = diag.NewBag(m.p.fs, name)
	}
	h := &host{load: m.load}
	if m.as != nil {
		h.as = m.as(h)
	}
	b.evaluate(context.Background(), h, m.opt, nil, func(ev *eval.Evaluator) {
		if memo != nil {
			ev.UseMemo(memo, epoch)
		}
	})
	if err := b.ev.Err(); err != nil {
		t.Errorf("internal error: %v", err)
	}
	out := outcome{dump: b.dump(), deep: deep(b), findings: b.findings(t), steps: b.ev.StepsSpent()}
	out.hits, out.stored, out.unkept = b.ev.MemoCounts()
	out.loads.hits, out.loads.stored, out.loads.unkept = b.ev.LoadMemoCounts()
	return out
}

// same reports where got, an evaluation with a memo, differs from want, a cold one.
func same(t testing.TB, name string, got, want outcome) {
	t.Helper()
	if got.dump != want.dump {
		t.Errorf("%s: values differ from a cold evaluation:\n%s\n--- cold ---\n%s", name, got.dump, want.dump)
	}
	if got.deep != want.deep {
		t.Errorf("%s: provenance, identities, marks or arguments differ from a cold evaluation:\n%s", name, firstDiff(got.deep, want.deep))
	}
	if got.findings != want.findings {
		t.Errorf("%s: findings differ from a cold evaluation:\n%s\n--- cold ---\n%s", name, got.findings, want.findings)
	}
	if got.steps != want.steps {
		t.Errorf("%s: %d steps spent, %d cold", name, got.steps, want.steps)
	}
}

// twice evaluates m cold, then twice with one memo, the second replaying every entry the first kept.
func twice(t testing.TB, name string, m *memoed) (first, again outcome) {
	t.Helper()
	cold := m.evaluate(t, nil, 0)
	memo := eval.NewMemo()
	first = m.evaluate(t, memo, 1)
	again = m.evaluate(t, memo, 1)
	same(t, name+" (recorded)", first, cold)
	same(t, name+" (replayed)", again, cold)
	if again.hits != first.stored || again.stored != 0 || again.unkept != first.unkept {
		t.Errorf("%s: recorded kept %d, left %d; replayed %d, kept %d, left %d",
			name, first.stored, first.unkept, again.hits, again.stored, again.unkept)
	}
	return first, again
}

// archived is an archive's program checked with its budget and layers, its loads served.
func archived(t *testing.T, path string) (*memoed, *txtar.Archive) {
	t.Helper()
	a, err := txtar.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := fromArchive(t, a)
	opt := caseOptions(t, golden.Case{Path: path, Archive: a})
	opt.Layers = strings.Fields(string(archiveFile(a, layersFile)))
	m := checkOnce(t, p, opt)
	m.load = servedJSON(t, p, func(name string) []byte { return archiveFile(a, name) })
	return m, a
}

// IMPLEMENTATION-PLAN §7.6 (NFR-02), EVALUATION.md §12.2 (EVL-03): every program, twice with a memo, as cold.
func TestMemoReplaysEveryProgram(t *testing.T) {
	twice(t, "examples", checkOnce(t, fromExamples(t), eval.Options{}))
	for _, dir := range []string{"memo", "findings", "layers", "prov", "dependent", "static", "tests"} {
		paths, err := filepath.Glob(filepath.Join("testdata", dir, "*.txtar"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			m, _ := archived(t, path)
			twice(t, path, m)
		}
	}
}

// IMPLEMENTATION-PLAN §7.6: each case keeps the entries its memo file counts.
func TestMemoKeeps(t *testing.T) {
	paths, err := filepath.Glob("testdata/memo/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		m, a := archived(t, path)
		want := strings.TrimSpace(string(archiveFile(a, memoFile)))
		if first, _ := twice(t, path, m); want != "" && fmt.Sprintf(memoFormat, first.stored, first.unkept) != want {
			t.Errorf("%s: "+memoFormat+", want %s", path, first.stored, first.unkept, want)
		}
	}
}

// servedJSON serves load("@root/f.json") with files(root/f.json), decoded through the evaluator
// as build's load does; each content is parsed once, so an unchanged file keeps its spans.
func servedJSON(t testing.TB, p *program, files func(name string) []byte) loader {
	var mu sync.Mutex
	srcs := map[string]*source.File{}
	return func(ev *eval.Evaluator, e *syntax.LoadExpr, typ types.Type) (value.Value, bool) {
		lit, ok := e.Args[0].Value.(*syntax.StringLit)
		if !ok || len(lit.Parts) != 1 {
			t.Errorf("load: want one plain string argument")
			return nil, false
		}
		name := lit.Parts[0].Text
		data := files(strings.TrimPrefix(name, rootMark))
		mu.Lock()
		src := srcs[name+string(data)]
		if src == nil {
			var err error
			if src, err = p.fs.Add(name, "/"+name, data); err != nil {
				t.Error(err)
			}
			srcs[name+string(data)] = src
		}
		mu.Unlock()
		bag := diag.NewBag(p.fs, "")
		root, err := jsonsrc.Parse(src, bag)
		if err != nil {
			return nil, false
		}
		req := load.Request{Bag: bag}.Through(ev)
		dec := &wire.Decoder{Bag: req.Bag, Host: req.Host, Coll: req.Coll, Outer: req.Outer, Field: req.Field}
		v, decoded, err := dec.Decode(context.Background(), wire.Selection{Node: root}, typ)
		return v, decoded && err == nil
	}
}

// firstDiff is the first line where got and want differ.
func firstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range min(len(g), len(w)) {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d:\n%s\n--- cold ---\n%s", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("%d lines, %d cold", len(g), len(w))
}
