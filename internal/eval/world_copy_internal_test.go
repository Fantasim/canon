package eval

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// EVALUATION.md §10.5 item 2 (ADR-0023 Forecast 4): the copy carries the construction facts, owners remapped, never a verdict.
func TestWorldCopyCarriesFacts(t *testing.T) {
	from, to := newEvaluator(check.Bags{}, Options{}), newEvaluator(check.Bags{}, Options{})
	owner, orig := &value.Record{}, &value.Record{}
	entry := &value.Record{Ident: &value.Identity{Owner: owner, Key: value.Key{S: "a"}}}
	ref := &value.Ref{Key: value.Key{S: "a"}, Owner: owner}
	lit, old := &value.Int{V: 3}, &value.Int{V: 1}
	list := &value.List{Elems: []value.Value{entry, ref, lit, lit}}
	prev := &value.List{Elems: []value.Value{entry}}
	owner.Fields = []value.Value{list}
	param := &types.Param{}
	from.written[lit], from.history[lit], from.rebuilt[list], from.origin[owner] = true, old, prev, orig
	from.bound[owner] = map[*types.Param]value.Value{param: lit}
	from.invalid[lit], from.verified[owner] = true, true
	cps, ok := copyValues(from, to, []value.Value{owner})
	if !ok {
		t.Fatal("no copy")
	}
	cp := cps[0].(*value.Record)
	cl := cp.Fields[0].(*value.List)
	ce, cr, cv := cl.Elems[0].(*value.Record), cl.Elems[1].(*value.Ref), cl.Elems[2]
	before, _ := to.history[cv].(*value.Int)
	was, _ := to.rebuilt[cl].(*value.List)
	for _, c := range []struct {
		fact string
		ok   bool
	}{
		{"every node anew", cp != owner && cl != list && ce != entry && cr != ref && cv != lit},
		{"sharing kept", cl.Elems[3] == cv},
		{"an entry's owner remapped", ce.Ident.Owner == cp && ce.Ident.Key.S == "a"},
		{"a ref's owner remapped", cr.Owner == cp},
		{"kept as written", to.written[cv]},
		{"what an amendment replaced", before != nil && before != old && before.V == old.V},
		{"the container an amendment copied", was != nil && was != prev && len(was.Elems) == 1 && was.Elems[0] == ce},
		{"the instance it copies", to.origin[cp] != nil && to.origin[cp] != orig},
		{"its arguments", to.bound[cp][param] == cv},
		{"no invalid mark", len(to.invalid) == 0},
		{"no stage-B memo", len(to.verified) == 0},
		{"the original untouched", entry.Ident.Owner == owner && ref.Owner == owner},
	} {
		if !c.ok {
			t.Errorf("the copy lost %s", c.fact)
		}
	}
}
