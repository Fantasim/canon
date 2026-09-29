package rules_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const retagSource = `package teamboard

record Row {
  k: String?

  check {
    fail(self, "retagged")
  }
  check late: k != "b" else "late"
}

let rows: [Row] keyed by k = []
`

// retagging is the fixture evaluator logging the records its check runs give an identity in
// place, as eval's does (colls.go withIdentity).
type retagging struct {
	*evaluator
	log []*value.Record
}

func (r *retagging) RetagMark() int { return len(r.log) }

func (r *retagging) RetaggedSince(mark int) []*value.Record { return r.log[mark:] }

// API.md P8, EVALUATION.md §8.3 (log-2026-09-29 M4 P12-r): a record retagged in place keeps its path.
func TestRetaggedKeepsItsPath(t *testing.T) {
	fx := newFixture(t, "teamboard/rows.canon", []byte(retagSource))
	key := field("k", &types.OptionalType{Elem: types.StringType})
	row := record("Row", key)
	block, late := fx.check("check {"), fx.check("check late")
	row.Checks = append(row.Checks, block, late)
	keyed := &types.ListType{Elem: row, KeyedBy: key}
	first := &value.Record{T: row, Fields: []value.Value{str("a", fx.lit("k"))}, Ident: &value.Identity{Key: value.Key{S: "a"}}, P: fx.lit("Row")}
	second := &value.Record{T: row, Fields: []value.Value{str("b", fx.lit("String"))}, P: fx.lit("rows")}
	ev := &retagging{evaluator: fx.ev}
	fx.ev.scripts[block] = func(self value.Value) rules.Run {
		if self != first {
			return rules.Run{}
		}
		second.Ident = &value.Identity{Key: value.Key{S: "b"}} // a keyed list built in the run takes it
		ev.log = append(ev.log, second)
		return rules.Run{Reports: []rules.Report{{At: second, Message: "retagged"}}}
	}
	fx.ev.scripts[late] = failsFor("late", second)
	fx.let("rows", &value.List{T: keyed, Elems: []value.Value{first, second}, P: fx.lit("[]")})
	fx.runWith(ev)
	var paths []string
	for _, f := range fx.bag.Findings() {
		paths = append(paths, f.Path)
	}
	if want := []string{"rows[1]", "rows[1]"}; !slices.Equal(paths, want) {
		t.Errorf("paths %v, want %v:\n%s", paths, want, fx.render())
	}
}
