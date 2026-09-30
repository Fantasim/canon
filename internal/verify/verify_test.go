package verify_test

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

const statusSource = `package teamboard

let statuses: stable table Status = {
  open { label: "Open", next: [taken] }
  taken { label: "Taken", next: [open] }
}

let cooldown: Duration(1s..) = 500ms
`

func codes(fx *fixture) []diag.Code {
	var out []diag.Code
	for _, f := range fx.bag.Findings() {
		out = append(out, f.Code)
	}
	return out
}

// EVALUATION.md §5, §7.1: a finding marks the value it is about invalid; a clean value stays valid.
func TestVerifyMarksWhatItReports(t *testing.T) {
	fx := newFixture(t, "teamboard/taxonomy.canon", []byte(statusSource))
	s := newStatuses(fx)
	bad := s.entry("taken", "Taken", "open")
	bad.Fields[1].(*value.List).Elems[0].(*value.Ref).Key = value.Key{S: "closed"}
	s.let(s.entry("open", "Open", "taken"), bad)
	got := fx.verify("statuses")
	ref := bad.Fields[1].(*value.List).Elems[0]
	if got[0].Valid || len(fx.ev.invalid) != 1 || fx.ev.invalid[0] != ref {
		t.Errorf("Verify = %v, invalid %v; want false and the ref", got, fx.ev.invalid)
	}
	fx = newFixture(t, "teamboard/taxonomy.canon", []byte(statusSource))
	s = newStatuses(fx)
	s.let(s.entry("open", "Open", "taken"), s.entry("taken", "Taken", "open"))
	if got := fx.verify("statuses"); !got[0].Valid || len(fx.ev.invalid) != 0 || len(fx.bag.Findings()) != 0 {
		t.Errorf("clean table: Verify = %v, invalid %v, findings %v", got, fx.ev.invalid, fx.bag.Findings())
	}
}

// EVALUATION.md §7.2: a ref into a poisoned collection is invalid, without a finding.
func TestPoisonedTargetIsSilent(t *testing.T) {
	fx := newFixture(t, "teamboard/taxonomy.canon", []byte(statusSource))
	s := newStatuses(fx)
	s.let(s.entry("open", "Open", "taken"))
	initial := &value.Ref{T: s.next.Elem, Key: value.Key{S: "taken"}, P: fx.lit("taken")}
	fx.let("initialStatus", s.next.Elem, initial)
	fx.ev.poisoned[eval.Root{Pkg: pkg, Name: "statuses"}] = true
	if got := fx.verify("initialStatus"); got[0].Valid || len(fx.bag.Findings()) != 0 || fx.ev.invalid[0] != initial {
		t.Errorf("Verify = %v, findings %v, invalid %v", got, fx.bag.Findings(), fx.ev.invalid)
	}
}

// EVALUATION.md §3.4: a level-1 ref resolves in its owner; an unbound one is handed to eval.
func TestLevelOneRefs(t *testing.T) {
	fx := newFixture(t, "teamboard/taxonomy.canon", []byte(statusSource))
	node := &types.RecordType{Pkg: pkg, Name: "Node"}
	tree := &types.RecordType{Pkg: pkg, Name: "Tree"}
	coll := &types.Collection{Kind: types.CollField, Owner: tree, FieldPath: []string{"nodes"}, Elem: node}
	parent := &types.RefType{Target: coll}
	node.Fields = []*types.Field{{Name: "id", Type: types.StringType}, {Name: "parent", Index: 1, Type: &types.OptionalType{Elem: parent}}}
	nodes := &types.ListType{Elem: node, KeyedBy: node.Fields[0]}
	tree.Fields = []*types.Field{{Name: "nodes", Type: nodes}}
	owner := &value.Record{T: tree}
	mk := func(id, up string, bound bool) *value.Record {
		var p value.Value = &value.None{T: node.Fields[1].Type}
		if up != "" {
			r := &value.Ref{T: parent, Key: value.Key{S: up}, P: fx.lit("open")}
			if bound {
				r.Owner = owner
			}
			p = r
		}
		return &value.Record{T: node, Ident: &value.Identity{Coll: coll, Owner: owner, Key: value.Key{S: id}},
			Fields: []value.Value{str(id, types.StringType, nil), p}}
	}
	owner.Fields = []value.Value{&value.List{T: nodes, Elems: []value.Value{mk("a", "", true), mk("b", "a", true), mk("c", "z", true), mk("d", "zz", false)}}}
	fx.let("tree", tree, owner)
	res := fx.verify("tree")[0]
	d := owner.Fields[0].(*value.List).Elems[3].(*value.Record).Fields[1]
	if res.Valid || len(res.Unbound) != 1 || res.Unbound[0].Ref != d || res.Unbound[0].Path != "tree.nodes[d].parent" || !slices.Contains(fx.ev.invalid, d) {
		t.Errorf("result %+v, want d's parent unbound", res)
	}
	if got := codes(fx); !slices.Equal(got, []diag.Code{diag.E3501.Def().Code}) {
		t.Errorf("findings %v, want one: c's parent z", fx.bag.Findings())
	}
	want := diag.NewBag(fx.fs, pkg)
	diag.E3501.At(source.Span{}, &value.Ref{Key: value.Key{S: "z"}}, "Tree.nodes").Report(want)
	if msg := fx.bag.Findings()[0].Message; msg != want.Findings()[0].Message {
		t.Errorf("message %q", msg)
	}
}

// EVALUATION.md §2.1: a Duration is checked against its let's declared type.
func TestDeclaredTypeOfTheLet(t *testing.T) {
	fx := newFixture(t, "teamboard/taxonomy.canon", []byte(statusSource))
	cooldown := fx.written("Duration(1s..)", &types.Refined{Of: types.DurationType, Range: &types.Bound{Lo: types.Limit{I: 1000}, HasLo: true}})
	fx.let("cooldown", cooldown, &value.Dur{Ms: 500, P: fx.lit("500ms")})
	fx.verify("cooldown")
	if got := codes(fx); !slices.Equal(got, []diag.Code{diag.E3204.Def().Code}) {
		t.Errorf("findings %v", fx.bag.Findings())
	}
}

// TYPES.md §7.4, §13.2: none and a union's literal skip the refinements.
func TestNoneAndLiteralsPass(t *testing.T) {
	fx := newFixture(t, "teamboard/taxonomy.canon", []byte(statusSource))
	short := &types.Refined{Of: types.StringType, Range: &types.Bound{Hi: types.Limit{I: 2}, HasHi: true}}
	opt := &types.Refined{Of: &types.OptionalType{Elem: short}, Where: &types.Predicate{Text: "false"}}
	union := &types.LitUnionType{Of: short, Literals: []string{"default"}}
	rec := record("R", field("a", opt), field("b", union), field("c", union))
	fx.let("r", rec, &value.Record{T: rec, Fields: []value.Value{
		&value.None{T: opt}, str("default", union, fx.lit("open")), str("long", union, fx.lit("taken")),
	}})
	fx.verify("r")
	if got := codes(fx); !slices.Equal(got, []diag.Code{diag.E3204.Def().Code}) {
		t.Errorf("findings %v, want one: c is too long", fx.bag.Findings())
	}
}

// EVALUATION.md §7.1, API.md F1: a hard error in `where` poisons the root, the re-run given its value's path.
func TestWhereHardError(t *testing.T) {
	fx := newFixture(t, "teamboard/taxonomy.canon", []byte(statusSource))
	pos := &types.Refined{Of: types.IntType, Where: &types.Predicate{Text: "1 / it > 0"}}
	short := &types.Refined{Of: types.StringType, Range: &types.Bound{Hi: types.Limit{I: 1}, HasHi: true}}
	rec := record("R", field("n", pos), field("s", short))
	fx.let("r", rec, &value.Record{T: rec, Fields: []value.Value{integer(0, pos, fx.lit("500")), str("long", short, fx.lit("open"))}})
	fx.ev.hard = true
	if got := fx.verify("r"); got[0].Valid || !got[0].Poisoned || len(fx.bag.Findings()) != 0 {
		t.Errorf("result %+v, findings %v", got[0], fx.bag.Findings())
	}
	if !slices.Equal(fx.ev.wheres, []string{"r.n"}) {
		t.Errorf("where run at %q, want r.n", fx.ev.wheres)
	}
}

// TYPES.md §7.3: a non-finite value stored (from outside Canon arithmetic) is E3202.
func TestNonFinite(t *testing.T) {
	fx := newFixture(t, "teamboard/taxonomy.canon", []byte(statusSource))
	nan := &value.Float{V: math.NaN(), T: types.FloatType, P: fx.lit("500")}
	fx.let("x", types.FloatType, nan)
	fx.verify("x")
	if got := codes(fx); !slices.Equal(got, []diag.Code{diag.E3202.Def().Code}) {
		t.Errorf("findings %v", fx.bag.Findings())
	}
}

// Check needs a bag for the value's package: its absence is a Go error, not a panic.
func TestNoBag(t *testing.T) {
	v := verify.New(&evaluator{}, nil, nil, nil)
	if _, err := v.Check(context.Background(), eval.Root{Pkg: "p", Name: "x"}, &value.Int{}); !errors.Is(err, verify.ErrNoBag) {
		t.Errorf("err %v", err)
	}
	enum := &types.EnumType{Codes: &types.UInt8Type}
	if _, err := v.Codes(&object{typ: enum}); !errors.Is(err, verify.ErrNoBag) {
		t.Errorf("Codes err %v", err)
	}
}

// API.md §6.5 (P8, P9): map entries and keyed-list elements by key, plain lists by index.
func TestPaths(t *testing.T) {
	root := verify.Root("farm")
	slot := &types.LitUnionType{Of: types.StringType, Literals: []string{"none"}}
	nested := &types.LitUnionType{Of: slot, Literals: []string{"all"}}
	kind := &types.EnumType{Name: "Kind", Members: []*types.Member{{Name: "none"}, {Name: "red", Index: 1}}}
	none, other := &value.Str{V: "none"}, &value.Str{V: "other"}
	for _, c := range []struct {
		got  *verify.Path
		want string
	}{
		{root.Field("modelTypes").Key(value.Key{I: 3, IsInt: true}).Field("levels").Index(1), "farm.modelTypes[3].levels[1]"},
		{root.Key(value.Key{S: "daily"}), "farm[daily]"},
		{root.Key(value.Key{S: "two words"}), `farm["two words"]`},
		{root.Entry(value.Key{S: "open"}), "farm.open"},
		{root.Entry(value.Key{S: "9lives"}), `farm["9lives"]`},
		{root.Entry(value.Key{S: "_"}), `farm["_"]`},
		{(*verify.Path)(nil).Field("x"), ""},
		// API.md P9 (log-2026-09-29 M4 U13-r): the key type declared at the map decides the quoting.
		{root.MapKey(none, slot), `farm["none"]`},
		{root.MapKey(other, slot), "farm[other]"},
		{root.MapKey(none, types.StringType), "farm[none]"},
		{root.MapKey(none, &types.Alias{Name: "Slot", Def: slot}), `farm["none"]`},
		{root.MapKey(none, &types.Refined{Of: slot, Range: &types.Bound{Hi: types.Limit{I: 9}, HasHi: true}}), `farm["none"]`},
		{root.MapKey(&value.Str{V: "all"}, nested), `farm["all"]`},
		{root.MapKey(none, nested), `farm["none"]`},
		{root.MapKey(&value.Member{Enum: kind}, &types.LitUnionType{Of: kind, Literals: []string{"none"}}), "farm[none]"},
		{root.MapKey(&value.Str{V: "two words"}, slot), `farm["two words"]`},
		{root.MapKey(&value.Int{V: 7}, &types.LitUnionType{Of: types.IntType, Literals: []string{"7"}}), "farm[7]"},
	} {
		if s := c.got.String(); s != c.want {
			t.Errorf("path %q, want %q", s, c.want)
		}
	}
}

// API.md P8 (log-2026-09-29 M4 U13-r2): a list's elements are named by key when the list type
// declared at their place is keyed, else when the list is stored keyed; walk, rejudge and rules
// read it alike.
func TestListAt(t *testing.T) {
	row := &types.RecordType{Name: "Row", Fields: []*types.Field{{Name: "id", Type: types.StringType}}}
	plain, keyed := &types.ListType{Elem: row}, &types.ListType{Elem: row, KeyedBy: row.Fields[0]}
	for _, c := range []struct {
		name          string
		declared      types.Type
		stored        types.Type
		want          *types.ListType
		wantDeclElems bool
	}{
		{"declared keyed", &types.OptionalType{Elem: keyed}, plain, keyed, true},
		{"declared plain wins", plain, keyed, plain, true},
		{"declared under an alias", &types.Alias{Name: "Rows", Def: keyed}, plain, keyed, true},
		{"none declared", nil, keyed, keyed, false},
		{"no list declared", types.StringType, keyed, keyed, false},
	} {
		lt, et := verify.ListAt(c.declared, &value.List{T: c.stored})
		if lt != c.want || (et != nil) != c.wantDeclElems {
			t.Errorf("%s: ListAt = %v, %v", c.name, lt, et)
		}
	}
}

// EVALUATION.md §13: a spread copy is reported at the value it was copied from.
func TestSiteOfSpread(t *testing.T) {
	orig := &value.Prov{Kind: value.ProvJSON, Pointer: "/a/0", Layer: "dev"}
	v := &value.Int{V: 1, P: &value.Prov{Kind: value.ProvSpread, Via: orig}}
	if s := verify.SiteOf(v); s.Span != orig.Span {
		t.Errorf("site %v", s)
	}
	if s := verify.SiteOf(&value.Int{}); s.Span.File != 0 {
		t.Errorf("no provenance: %v", s)
	}
}

// A cancelled context stops the walk: nothing is reported.
func TestCancelled(t *testing.T) {
	fx := newFixture(t, "teamboard/taxonomy.canon", []byte(statusSource))
	s := newStatuses(fx)
	s.let(s.entry("open", "Open", "taken"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v := fx.verifier()
	res, err := v.Check(ctx, eval.Root{Pkg: pkg, Name: "statuses"}, fx.ev.values[eval.Root{Pkg: pkg, Name: "statuses"}])
	if err != nil || !res.Valid || len(fx.bag.Findings()) != 0 {
		t.Error("a cancelled walk reported")
	}
}
