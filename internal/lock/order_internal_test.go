package lock

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	orderPkg   = "p"
	otherPkg   = "q"
	orderTable = "p.t"
	orderSeed  = 11
	orderRuns  = 400
	orderKeys  = 9
)

// row is a stable table's element: a @stable code and a plain weight.
var row = &types.RecordType{Pkg: orderPkg, Name: "Row", Fields: []*types.Field{
	{Name: "code", Index: 0, Type: types.IntType, Stable: true},
	{Name: "weight", Index: 1, Type: types.IntType},
}}

// entry is one entry of a test table: key, retirement, @stable code, and where it starts.
type entry struct {
	key     string
	retired bool
	code    int64
	at      source.Pos
}

// tableOf is a stable table of the entries, in their order.
func tableOf(entries []entry) *value.Table {
	t := &value.Table{T: &types.TableType{Elem: row, Stable: true}}
	for _, e := range entries {
		p := &value.Prov{Span: source.Span{File: 1, Start: e.at, End: e.at + 1}}
		t.Entries = append(t.Entries, &value.Record{
			T:      row,
			Fields: []value.Value{&value.Int{V: e.code, T: types.IntType, P: p}, nil},
			Ident:  &value.Identity{Key: value.Key{S: e.key}, Retired: e.retired},
			P:      p,
		})
	}
	return t
}

func shifted(entries []entry, by source.Pos) []entry {
	out := slices.Clone(entries)
	for i := range out {
		out[i].at += by
	}
	return out
}

// LOCK.md §2.3, §3: AddTable from an Order adds a cold AddTable's facts, reusing only equal ones.
func TestAddTableOrder(t *testing.T) {
	base := []entry{{"open", false, 1, 10}, {"done", false, 2, 20}, {"wont_do", true, 3, 30}, {"blocked", false, 4, 40}}
	for _, c := range []struct {
		name   string
		now    []entry
		reused bool
	}{
		{"same facts, spans moved", shifted(base, 7), true},
		{"entry added", append(slices.Clone(base), entry{"late", false, 5, 50}), false},
		{"entry removed", base[1:], false},
		{"entry renamed", []entry{{"opened", false, 1, 10}, base[1], base[2], base[3]}, false},
		{"entry retired", []entry{{"open", true, 1, 10}, base[1], base[2], base[3]}, false},
		{"entry unretired", []entry{base[0], base[1], {"wont_do", false, 3, 30}, base[3]}, false},
		{"stable value changed", []entry{{"open", false, 9, 10}, base[1], base[2], base[3]}, false},
		{"entries reordered", []entry{base[3], base[2], base[1], base[0]}, false},
		{"key refused", []entry{base[0], {"9lives", false, 6, 60}, base[2]}, false},
	} {
		prev, err := NewSources(orderPkg).AddTable(orderTable, tableOf(base), nil)
		if err != nil {
			t.Fatal(err)
		}
		warm, cold := NewSources(orderPkg), NewSources(orderPkg)
		wo, werr := warm.AddTable(orderTable, tableOf(c.now), prev)
		co, cerr := cold.AddTable(orderTable, tableOf(c.now), nil)
		if fmt.Sprint(werr) != fmt.Sprint(cerr) || !slices.Equal(warm.facts.facts, cold.facts.facts) {
			t.Errorf("%s: facts %v (%v), cold %v (%v)", c.name, warm.facts.facts, werr, cold.facts.facts, cerr)
		}
		if (wo == nil) != (co == nil) || wo != nil && (!slices.Equal(wo.kept, co.kept) || !slices.Equal(wo.retired, co.retired)) {
			t.Errorf("%s: order %v, cold %v", c.name, wo, co)
		}
		if reused := wo != nil && &wo.kept[0] == &prev.kept[0]; reused != c.reused {
			t.Errorf("%s: order reused %t, want %t", c.name, reused, c.reused)
		}
	}
}

// LOCK.md §2.2, §3: an Order of another package is not reused: the facts are checked, and refused.
func TestAddTableOrderOtherPackage(t *testing.T) {
	prev, err := NewSources(orderPkg).AddTable(orderTable, tableOf([]entry{{"open", false, 1, 10}}), nil)
	if err != nil {
		t.Fatal(err)
	}
	o, err := NewSources(otherPkg).AddTable(orderTable, tableOf([]entry{{"open", false, 1, 10}}), prev)
	if o != nil || !errors.Is(err, ErrBadFact) {
		t.Errorf("order %v, error %v; want the cold refusal", o, err)
	}
}

// LOCK.md §2.4: a table's first fact equal to the last one held merges into it, retired winning.
func TestAddTableMergesAtTheSeam(t *testing.T) {
	held := Fact{Kind: KindField, Name: orderTable, Field: "code", Value: Value{Int: 1}, Holder: "a", Span: source.Span{File: 1}}
	s := NewSources(orderPkg)
	s.facts.mergeAll([]Fact{held})
	now := []entry{{"a", true, 1, 5}, {"b", false, 2, 6}}
	if _, err := s.AddTable(orderTable, tableOf(now), nil); err != nil {
		t.Fatal(err)
	}
	want := New(orderPkg)
	for _, fact := range append([]Fact{held}, appendAll(tableOf(now))...) {
		referenceMerge(want, fact)
	}
	if !slices.Equal(s.facts.facts, want.facts) {
		t.Errorf("facts %v\nwant %v", s.facts.facts, want.facts)
	}
}

// LOCK.md §2.3: byColl groups as appending each fact to its collection does, names prefixing others.
func TestByCollIsAppend(t *testing.T) {
	var batch []Fact
	for i, name := range []string{"p.a", "p.ab", "p.a_"} {
		for _, holder := range []string{"k", "l"} {
			batch = append(batch, Fact{Kind: KindTable, Name: name, Holder: holder},
				Fact{Kind: KindEnum, Name: name, Value: Value{Int: int64(i)}, Holder: holder})
			for j, field := range []string{"x", "y", "x_"} {
				batch = append(batch, Fact{Kind: KindField, Name: name, Field: field, Value: Value{Int: int64(j)}, Holder: holder})
			}
		}
	}
	f := New(orderPkg)
	f.mergeAll(batch)
	want := map[coll][]Fact{}
	for _, fact := range f.facts {
		c := coll{kind: fact.Kind, name: fact.writtenName()}
		want[c] = append(want[c], fact)
	}
	if got := f.byColl(); !reflect.DeepEqual(got, want) {
		t.Errorf("byColl %v\nwant %v", got, want)
	}
}

// LOCK.md §2.3, §2.4, §3: AddTable, cold or from an Order, merges as one fact at a time does.
func TestAddTableIsSequentialMerge(t *testing.T) {
	rnd := rand.New(rand.NewPCG(orderSeed, orderRuns))
	for run := range orderRuns {
		before, now := randomEntries(rnd), randomEntries(rnd)
		if run%2 == 0 {
			now = shifted(before, orderKeys)
		}
		prior := randomTable(rnd, true).facts
		for i := range prior {
			prior[i].Name = orderTables[rnd.IntN(len(orderTables))]
		}
		prev, err := NewSources(orderPkg).AddTable(orderTable, tableOf(before), nil)
		if err != nil {
			t.Fatal(err)
		}
		want := New(orderPkg)
		for _, fact := range slices.Concat(prior, appendAll(tableOf(now))) {
			referenceMerge(want, fact)
		}
		for _, from := range []*Order{nil, prev} {
			s := NewSources(orderPkg)
			s.facts.mergeAll(slices.Clone(prior))
			if _, err := s.AddTable(orderTable, tableOf(now), from); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(s.facts.facts, want.facts) {
				t.Fatalf("run %d, from %v:\n%v\nwant\n%v", run, from != nil, s.facts.facts, want.facts)
			}
		}
	}
}

// orderTables are tables whose facts sort before, after and among those of orderTable.
var orderTables = [...]string{"p.a", orderTable, "p.z"}

// randomEntries is a few entries with keys drawn from a small set, so that some repeat, some
// retired.
func randomEntries(rnd *rand.Rand) []entry {
	out := make([]entry, rnd.IntN(orderKeys))
	for i := range out {
		out[i] = entry{key: fmt.Sprintf("k%d", rnd.IntN(orderKeys)), retired: rnd.IntN(3) == 0, code: int64(rnd.IntN(orderKeys)), at: source.Pos(i)}
	}
	return out
}

// appendAll is every fact of a table's entries, in entry order.
func appendAll(t *value.Table) []Fact {
	var out []Fact
	for _, e := range t.Entries {
		out = appendEntry(out, orderTable, e, stableFields(row))
	}
	return out
}

// LOCK.md §4.1, §4.2: the sorted join of a table's keys reports what looking each key up did.
func TestTableRulesJoin(t *testing.T) {
	rnd := rand.New(rand.NewPCG(orderSeed, orderSeed))
	seen := map[string]bool{}
	for run := range orderRuns {
		lockFile, cur := randomTable(rnd, true), randomTable(rnd, false)
		got, want := diag.NewBag(&source.FileSet{}, orderPkg), diag.NewBag(&source.FileSet{}, orderPkg)
		for _, bag := range []*diag.Bag{got, want} {
			lockAll, curAll := lockFile.byColl(), cur.byColl()
			c := &comparison{lock: lockAll[tableColl], cur: curAll[tableColl], lockAll: lockAll, curAll: curAll, name: orderTable, bag: bag}
			if bag == got {
				tableRules(c)
			} else {
				tableRulesByMaps(c)
			}
		}
		if !reflect.DeepEqual(got.Findings(), want.Findings()) {
			t.Fatalf("run %d:\nlock %v\ncur %v\njoin %v\nmaps %v", run, lockFile.facts, cur.facts, got.Findings(), want.Findings())
		}
		for _, sit := range situations(lockFile.facts, cur.facts) {
			seen[sit] = true
		}
	}
	if len(seen) != len(situationNames) {
		t.Errorf("the runs met only %v of %v: the test is partly vacuous", seen, situationNames)
	}
}

// situationNames are the cases tableRules tells apart: E6001 removed, renamed, held; E6002.
var situationNames = [...]string{sitGone: "gone", sitOneForOne: "one for one", sitHeld: "held", sitBack: "back"}

// The indexes of situationNames.
const (
	sitGone = iota
	sitOneForOne
	sitHeld
	sitBack
)

// situations are the cases of situationNames a lock and current facts of orderTable meet.
func situations(lockFacts, cur []Fact) []string {
	keys := func(facts []Fact, kind Kind) map[string]Fact {
		out := map[string]Fact{}
		for _, f := range facts {
			if f.Kind == kind {
				out[f.Holder] = f
			}
		}
		return out
	}
	locked, now, lockedCodes, codes := keys(lockFacts, KindTable), keys(cur, KindTable), keys(lockFacts, KindField), keys(cur, KindField)
	var out []string
	gone, added, held := 0, 0, false
	for k, l := range locked { //canon:unordered the situations are a set
		f, ok := now[k]
		gone += boolInt(!ok)
		if ok && l.Retired && !f.Retired {
			out = append(out, situationNames[sitBack])
		}
		for h, c := range codes { //canon:unordered a flag
			if _, known := locked[h]; !ok && !known && c.Value == lockedCodes[k].Value {
				held = true
			}
		}
	}
	for k := range now { //canon:unordered a count
		_, ok := locked[k]
		added += boolInt(!ok)
	}
	if gone > 0 {
		out = append(out, situationNames[sitGone])
	}
	switch {
	case held:
		out = append(out, situationNames[sitHeld])
	case gone == 1 && added == 1:
		out = append(out, situationNames[sitOneForOne])
	}
	return out
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var tableColl = coll{kind: KindTable, name: orderTable}

// randomTable is a random stable table's facts, keys and codes drawn from small ranges so that
// keys go, come back, get retired and share codes; a lock's lines are located.
func randomTable(rnd *rand.Rand, located bool) *File {
	f := New(orderPkg)
	var batch []Fact
	for k := range orderKeys {
		if rnd.IntN(3) == 0 {
			continue
		}
		key, span := fmt.Sprintf("k%d", k), source.Span{File: 1, Start: source.Pos(rnd.IntN(orderRuns))}
		if !located {
			span.File = 2
		}
		batch = append(batch, Fact{Kind: KindTable, Name: orderTable, Holder: key, Retired: rnd.IntN(3) == 0, Span: span},
			Fact{Kind: KindField, Name: orderTable, Field: "code", Value: Value{Int: int64(rnd.IntN(orderKeys))}, Holder: key, Span: span})
	}
	f.mergeAll(batch)
	return f
}

// tableRulesByMaps is tableRules as it was before the join (log-2026-09-29 M4 P18 C5): the
// oracle the join must equal.
func tableRulesByMaps(c *comparison) {
	locked, now := holders(c.lock), holders(c.cur)
	var gone, added []Fact
	for _, l := range c.lock {
		if _, ok := now[l.Holder]; !ok {
			gone = append(gone, l)
		}
	}
	for _, f := range c.cur {
		if _, ok := locked[f.Holder]; !ok {
			added = append(added, f)
		}
	}
	for _, l := range gone {
		c.gone(l, locked, len(gone) == 1 && len(added) == 1, added)
	}
	for _, l := range c.lock {
		if f, ok := now[l.Holder]; ok && l.Retired && !f.Retired {
			diag.E6002.AtUnretire(f.Span, diag.KindEntry, l.Holder).Report(c.bag)
		}
	}
}
