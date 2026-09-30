package check_test

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// lineageSteps are the edits of one lineage per world: breaks and mends, a reference to an
// entry of the swapped file from a kept one, a duplicate key across files, a later break.
var lineageSteps = []struct {
	name  string
	srcs  map[string]string
	steps []step
}{
	{"shop", shop, []step{
		{"shop/items/apple.canon", "price: 3", "price: 7", true},
		{"shop/items/apple.canon", "price: 7", `price: "seven"`, true},
		{"shop/items/bread.canon", "needs: pear", "needs: apple", true},
		{"shop/items/bread.canon", "needs: apple", `needs: "pear"`, true},
		{"shop/items/apple.canon", "price: 4", "price: [4, 5].len()", true},
		{"shop/items/apple.canon", `price: "seven"`, "price: 3 +", true},
		{"shop/items/apple.canon", "needs: apple", "needs: plum", true},
		{"shop/items/apple.canon", "kind: FOOD {}", "kind: SEED {}", true},
		{"shop/items/bread.canon", "size: 3", "size: 12", true},
		{"shop/items/apple.canon", "price: 3 +", "price: 3", true},
	}},
	{"dups", dups, []step{
		{"dup/a.canon", "n: 1", "n: 11\n\n\n", true},
		{"dup/b.canon", "n: 2", "n: 22", true},
		{"dup/a.canon", "label: \"a\"", "label: 7", true},
	}},
	{"brokenLater", brokenLater, []step{
		{"p/x.canon", "n: 1", "n: 2", true},
		{"p/x.canon", "n: 2", "n: 3", true},
	}},
}

// IMPLEMENTATION-PLAN §4.7, §7.6 NFR-02: a Recheck writes nothing an earlier program holds.
func TestRecheckLeavesEarlierPrograms(t *testing.T) {
	for _, tc := range lineageSteps {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, tc.srcs)
			prog, s, _ := w.session()
			var kept lineagePrints
			kept.add(prog)
			for _, st := range tc.steps {
				next, ns, _, ok := w.recheck(s, w.edit(st.path, st.old, st.new))
				if !ok {
					t.Fatalf("%s: %q to %q: Recheck refused", st.path, st.old, st.new)
				}
				kept.verify(t, st)
				kept.add(next)
				s = ns
			}
		})
	}
}

// IMPLEMENTATION-PLAN §4.7, DECISIONS 104: the fold replay leaves earlier programs, at every budget.
func TestRecheckReplayLeavesEarlierPrograms(t *testing.T) {
	for n := int64(1); n <= budgetTop; n++ {
		t.Run(strconv.FormatInt(n, 10), func(t *testing.T) {
			w := newWorld(t, priced)
			bags := w.bags()
			fold := budgetFolder(bags, n)
			prog, s := check.CheckSession(t.Context(), w.proj, w.list(), bags, fold)
			foldDefaults(fold, prog)
			var kept lineagePrints
			kept.add(prog)
			for _, st := range pricedSteps {
				nf := w.edit(st.path, st.old, st.new)
				nb := check.Bags{}
				w.parseInto(nb, nf)
				nfold := budgetFolder(nb, n)
				next, ns, ok := s.Recheck(t.Context(), []*syntax.File{nf}, nb, nfold)
				if !ok {
					t.Fatalf("budget %d: %s: Recheck refused", n, st.path)
				}
				w.commit(nf.Src.Path, nf)
				foldDefaults(nfold, next)
				kept.verify(t, st)
				kept.add(next)
				s = ns
			}
		})
	}
}

// lineagePrints are the programs of one lineage with their fingerprints taken when returned.
type lineagePrints struct {
	progs  []*check.Program
	prints []string
}

func (l *lineagePrints) add(prog *check.Program) {
	l.progs = append(l.progs, prog)
	l.prints = append(l.prints, fingerprint(prog))
}

// verify fails when an earlier program's fingerprint moved during the step.
func (l *lineagePrints) verify(t *testing.T, st step) {
	t.Helper()
	for i, prog := range l.progs {
		if got := fingerprint(prog); got != l.prints[i] {
			t.Fatalf("%s: %q to %q changed program %d of the lineage:\n%s", st.path, st.old, st.new, i, firstDiff(got, l.prints[i]))
		}
	}
}

// fingerprint prints a program by identity and one level of content: each package, then each
// map of Info entry by entry, keys and objects by pointer, the other values as they read.
func fingerprint(prog *check.Program) string {
	var b strings.Builder
	for _, p := range prog.Packages {
		fmt.Fprintf(&b, "package %p %#v\n", p, *p)
	}
	i := prog.Info
	for _, part := range [][]string{
		entries("types", i.Types, deep), entries("typeexprs", i.TypeExprs, deep),
		entries("defs", i.Defs, objPrint), entries("uses", i.Uses, objPrint),
		entries("nameuses", i.NameUses, objPrint), entries("selections", i.Selections, deep),
		entries("conv", i.Conv, deep), entries("keys", i.Keys, deep),
		entries("symbols", i.Symbols, deep), entries("calls", i.Calls, deep),
		entries("literals", i.Literals, deep), entries("matches", i.Matches, deep),
		entries("broken", i.Broken, deep), entries("brokenviews", i.BrokenViews, deep),
		entries("brokentranslations", i.BrokenTranslations, deep),
	} {
		b.WriteString(strings.Join(part, "\n"))
		b.WriteByte('\n')
	}
	return b.String()
}

// entries are a map's entries, sorted, its length first.
func entries[K comparable, V any](name string, m map[K]V, val func(V) string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m { //canon:unordered sorted below
		out = append(out, fmt.Sprintf("%s %s %s", name, addr(k), val(v)))
	}
	slices.Sort(out)
	return append([]string{fmt.Sprintf("%s len %d", name, len(m))}, out...)
}

// deep is a value's pointer, when it is one, and its content one level down.
func deep[V any](v V) string {
	return addr(v) + fmt.Sprintf(" %#v", v)
}

// addr is the address a value points at, "-" for a value that is no pointer.
func addr(v any) string {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer {
		return fmt.Sprintf("%#x", rv.Pointer())
	}
	return "-"
}

// objPrint is an object's pointer and all its methods read.
func objPrint(o check.Object) string {
	if o == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s %v %q %q %s %s %p", addr(o), o.Kind(), o.Name(), o.Pkg(), deep(o.Type()), addr(o.Decl()), o.File())
}
