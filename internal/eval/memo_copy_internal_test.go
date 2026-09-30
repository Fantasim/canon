package eval

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// copyCase is a value an entry made, the value it read (its fingerprint walk's nodes) and the same
// value read again on a replay, its nodes at the same places, and the marks its evaluation set.
type copyCase struct {
	name       string
	root       value.Value
	read, live []value.Value
	mark       func(e *Evaluator)
}

// IMPLEMENTATION-PLAN §7.6 (≡ cold): a replay's copy equals the map-based one, node and mark alike.
func TestMemoCopyMatchesTheMapCopy(t *testing.T) {
	for _, tc := range copyCases() {
		t.Run(tc.name, func(t *testing.T) {
			src := newEvaluator(check.Bags{}, Options{})
			if tc.mark != nil {
				tc.mark(src)
			}
			reads := []*readInfo{{nodes: tc.read}}
			f := src.newFreezer(tc.root, reads)
			if !f.walk() {
				t.Fatal("the walk met a function value")
			}
			g := f.kept(tc.root)
			ref := refKeep(f, tc.root)
			if cap(g.parts) != len(g.parts) || cap(g.shared) != len(g.shared) || cap(g.foreign) != len(g.foreign) {
				t.Errorf("kept graph holds spare capacity: parts %d/%d, shared %d/%d, foreign %d/%d",
					len(g.parts), cap(g.parts), len(g.shared), cap(g.shared), len(g.foreign), cap(g.foreign))
			}
			for round := range copyRounds {
				replays := []*readInfo{{nodes: tc.live}}
				gotEv, wantEv := newEvaluator(check.Bags{}, Options{}), newEvaluator(check.Bags{}, Options{})
				tab, ok := g.seed(replays)
				done, refOK := ref.seed(replays)
				if !ok || !refOK {
					t.Fatalf("round %d: seed %t, reference seed %t", round, ok, refOK)
				}
				got, want := gotEv.thaw(&g, tab), wantEv.refThaw(ref, done)
				c := newIsoCheck(tc)
				c.same(got, want)
				c.marks(gotEv, wantEv)
				for _, err := range c.errs {
					t.Errorf("round %d: %v", round, err)
				}
				scribble(c)
			}
		})
	}
}

// scribble writes over every record copied, as later stages write records in place: the next
// copy of the same graph must not see it.
func scribble(c *isoCheck) {
	for n := range c.pairs { //canon:unordered every copy written
		rec, ok := n.(*value.Record)
		if !ok {
			continue
		}
		for i := range rec.Set {
			rec.Set[i] = !rec.Set[i]
		}
		for i := range rec.Fields {
			rec.Fields[i] = nil
		}
		if rec.Ident != nil {
			rec.Ident.Retired = !rec.Ident.Retired
		}
	}
}

// IMPLEMENTATION-PLAN §7.6: a value read that changed kind at a place refuses the replay.
func TestMemoCopySeedRefusesAnotherKind(t *testing.T) {
	read := &value.List{}
	root := &value.Record{Fields: []value.Value{read}}
	f := newEvaluator(check.Bags{}, Options{}).newFreezer(root, []*readInfo{{nodes: []value.Value{read}}})
	if !f.walk() {
		t.Fatal("the walk met a function value")
	}
	g := f.kept(root)
	if _, ok := g.seed([]*readInfo{{nodes: []value.Value{&value.Map{}}}}); ok {
		t.Error("a map where a list was read: want the seed refused")
	}
	if _, ok := g.seed([]*readInfo{{nodes: nil}}); ok {
		t.Error("a read with fewer nodes: want the seed refused")
	}
}

const copyRounds = 2 // each graph is copied twice: the kept graph is never written by a copy

// copyCases are graphs with sharing, aliasing, cycles, values read, empty and nil parts and marks.
func copyCases() []copyCase {
	shared := &value.Int{V: 1}
	loose := &value.Ref{Key: value.Key{S: "k"}}
	alias := &value.List{Elems: []value.Value{shared, nil}}
	parent := &value.Record{Set: []bool{true}}
	entry := &value.Record{Fields: []value.Value{shared}, Set: []bool{}, Ident: &value.Identity{Key: value.Key{S: "one"}, Owner: parent, Retired: true}}
	table := &value.Table{Entries: []*value.Record{entry}}
	owned := &value.Ref{Key: value.Key{S: "one"}, Owner: parent}
	parent.Fields = []value.Value{table, owned, nil}
	read, readElem := &value.List{}, &value.Record{}
	read.Elems = []value.Value{readElem}
	live, liveElem := &value.List{}, &value.Record{}
	live.Elems = []value.Value{liveElem}
	key := &value.Record{}
	root := &value.Record{Fields: []value.Value{alias, alias, loose, parent, readElem, &value.Bool{V: true},
		&value.Map{Keys: []value.Value{key, shared}, Vals: []value.Value{key, alias}},
		&value.Pair{A: read, B: nil}, &value.List{}}}
	old, rebuiltFrom, origin := &value.List{Elems: []value.Value{shared}}, &value.Map{}, &value.Record{}
	param := &types.Param{Name: "p"}
	mark := func(e *Evaluator) {
		e.invalid[alias], e.written[entry] = true, true
		e.history[alias], e.rebuilt[table] = old, rebuiltFrom
		e.origin[entry], e.origin[root] = origin, readElem
		e.bound[parent] = map[*types.Param]value.Value{param: readElem, {Name: "q"}: nil}
	}
	return []copyCase{
		{name: "entry", root: root, read: []value.Value{read, readElem}, live: []value.Value{live, liveElem}, mark: mark},
		{name: "cycle", root: parent, mark: mark},
		{name: "shared root", root: shared},
		{name: "read root", root: readElem, read: []value.Value{read, readElem}, live: []value.Value{live, liveElem}},
		{name: "three reads", root: &value.Record{Fields: []value.Value{read, readElem, key}},
			read: []value.Value{read, readElem, key}, live: []value.Value{live, liveElem, &value.Record{}}},
	}
}

// isoCheck walks two copies side by side: a node reached in both is one pair throughout.
type isoCheck struct {
	pairs, back map[value.Value]value.Value
	outside     map[value.Value]bool // the nodes a copy may hold as they are: shared, or read now
	errs        []error
}

func newIsoCheck(tc copyCase) *isoCheck {
	c := &isoCheck{pairs: map[value.Value]value.Value{}, back: map[value.Value]value.Value{}, outside: map[value.Value]bool{}}
	for _, v := range tc.live {
		c.outside[v] = true
	}
	return c
}

func (c *isoCheck) fail(format string, args ...any) {
	c.errs = append(c.errs, fmt.Errorf(format, args...))
}

// same checks that got and want are the same node, or copies of one at the same places.
func (c *isoCheck) same(got, want value.Value) {
	if got == nil || want == nil || got == want {
		if got != want {
			c.fail("%T %p where the reference holds %T %p", got, got, want, want)
		}
		if _, copied := copyKind(got); copied && !c.outside[got] {
			c.fail("%T %p: a node both copies hold as it is, not copied", got, got)
		}
		return
	}
	if p, seen := c.pairs[got]; seen || c.back[want] != nil {
		if p != want {
			c.fail("%T %p paired with two nodes: one node became two, or two one", got, got)
		}
		return
	}
	c.pairs[got], c.back[want] = want, got
	if c.outside[got] || c.outside[want] || reflect.TypeOf(got) != reflect.TypeOf(want) {
		c.fail("%T %p: a node held as it is in one copy only", got, got)
		return
	}
	c.node(got, want)
}

// node compares two copies' own fields and walks their parts.
func (c *isoCheck) node(got, want value.Value) {
	switch g := got.(type) {
	case *value.Record:
		w := want.(*value.Record)
		c.slices(g.Fields, w.Fields)
		if g.T != w.T || g.P != w.P || !slices.Equal(g.Set, w.Set) || (g.Set == nil) != (w.Set == nil) || (g.Ident == nil) != (w.Ident == nil) {
			c.fail("record %p: fields differ from the reference's", g)
			return
		}
		if g.Ident != nil {
			gi, wi := *g.Ident, *w.Ident
			c.same(recordValue(gi.Owner), recordValue(wi.Owner))
			if gi.Owner, wi.Owner = nil, nil; gi != wi || g.Ident == w.Ident {
				c.fail("record %p: identity differs from the reference's", g)
			}
		}
	case *value.List:
		c.slices(g.Elems, want.(*value.List).Elems)
	case *value.Map:
		w := want.(*value.Map)
		c.slices(g.Keys, w.Keys)
		c.slices(g.Vals, w.Vals)
	case *value.Table:
		w := want.(*value.Table)
		c.slices(recordValues(g.Entries), recordValues(w.Entries))
	case *value.Pair:
		w := want.(*value.Pair)
		c.same(g.A, w.A)
		c.same(g.B, w.B)
	case *value.Ref:
		w := want.(*value.Ref)
		if g.Key != w.Key || g.T != w.T || g.P != w.P {
			c.fail("ref %p differs from the reference's", g)
		}
		c.same(recordValue(g.Owner), recordValue(w.Owner))
	}
}

// slices compares two copied slices: both made (the reference never kept nil), the same length,
// capped at it (no append reaches another's), part by part.
func (c *isoCheck) slices(got, want []value.Value) {
	if got == nil || want == nil || len(got) != len(want) || cap(got) != len(got) {
		c.fail("parts: got %d (cap %d, nil %t), reference %d (nil %t)", len(got), cap(got), got == nil, len(want), want == nil)
		return
	}
	for i := range got {
		c.same(got[i], want[i])
	}
}

// marks checks that both evaluators marked the paired nodes alike.
func (c *isoCheck) marks(got, want *Evaluator) {
	c.links("history", got.history, want.history)
	c.links("rebuilt", got.rebuilt, want.rebuilt)
	c.set("invalid", got.invalid, want.invalid)
	c.set("written", got.written, want.written)
	if len(got.origin) != len(want.origin) || len(got.bound) != len(want.bound) || got.gens.written != want.gens.written {
		c.fail("origin %d, bound %d, written %d; reference %d, %d, %d", len(got.origin), len(got.bound), got.gens.written, len(want.origin), len(want.bound), want.gens.written)
	}
	for rec, of := range got.origin { //canon:unordered every mark compared
		c.same(of, want.origin[c.ref(rec).(*value.Record)])
	}
	for rec, params := range got.bound { //canon:unordered every mark compared
		args := want.bound[c.ref(rec).(*value.Record)]
		if len(params) != len(args) {
			c.fail("bound %p: %d arguments, reference %d", rec, len(params), len(args))
		}
		for p, arg := range params { //canon:unordered every argument compared
			c.same(arg, args[p])
		}
	}
}

// links checks a mark naming another value: each copy marked has its reference marked, naming the pair.
func (c *isoCheck) links(name string, got, want map[value.Value]value.Value) {
	if len(got) != len(want) {
		c.fail("%s: %d marks, reference %d", name, len(got), len(want))
	}
	for k, v := range got { //canon:unordered every mark compared
		w, ok := want[c.ref(k)]
		if !ok {
			c.fail("%s: %T %p marked, its reference not", name, k, k)
		}
		c.same(v, w)
	}
}

// set checks a mark on a node alone.
func (c *isoCheck) set(name string, got, want map[value.Value]bool) {
	if len(got) != len(want) {
		c.fail("%s: %d marks, reference %d", name, len(got), len(want))
	}
	for k := range got { //canon:unordered every mark compared
		if !want[c.ref(k)] {
			c.fail("%s: %T %p marked, its reference not", name, k, k)
		}
	}
}

// ref is k's reference node: its pair when copied, else k itself.
func (c *isoCheck) ref(k value.Value) value.Value {
	if w, ok := c.pairs[k]; ok {
		return w
	}
	return k
}

func recordValue(rec *value.Record) value.Value {
	if rec == nil {
		return nil
	}
	return rec
}

func recordValues(recs []*value.Record) []value.Value {
	if recs == nil {
		return nil
	}
	out := make([]value.Value, len(recs))
	for i, r := range recs {
		out[i] = recordValue(r)
	}
	return out
}
