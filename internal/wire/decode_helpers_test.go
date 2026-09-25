package wire_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// host serves defaults by field and entries by collection, as build's host would.
type host struct {
	defaults map[*types.Field]value.Value
	entries  map[*types.Collection][]*value.Record
	misses   int // the refs Deref found no entry for, which build's host would report
	calls    int // the defaults evaluated
	bound    map[*value.Record]map[*types.Param]value.Value
	cycles   []*value.Ref           // the refs Cycle reported
	reads    map[*types.Field][]int // what a default reads, by field index
	attempts int                    // the savepoints taken, one per attempt of the second pass
	undone   int                    // the attempts taken back
}

func newHost() *host {
	return &host{
		defaults: map[*types.Field]value.Value{}, entries: map[*types.Collection][]*value.Record{},
		bound: map[*value.Record]map[*types.Param]value.Value{}, reads: map[*types.Field][]int{},
	}
}

// Default serves a registered field's value; an unregistered field with no default gets none.
func (h *host) Default(_ context.Context, f *types.Field, _ wire.Instance, via *value.Prov) (value.Value, bool) {
	h.calls++
	if v, ok := h.defaults[f]; ok {
		return v, true
	}
	if f.Default == nil {
		return &value.None{T: f.Type, P: &value.Prov{Kind: value.ProvDefault, Via: via}}, true
	}
	return nil, false
}

func (h *host) Deref(_ context.Context, r *value.Ref) (*value.Record, bool) {
	for _, e := range h.entries[r.T.Base().(*types.RefType).Target] {
		if e.Ident.Key == r.Key {
			return e, true
		}
	}
	h.misses++
	return nil, false
}

func (h *host) Bind(rec *value.Record, params map[*types.Param]value.Value) {
	h.bound[rec] = params
}

func (h *host) Cycle(_ context.Context, r *value.Ref) {
	h.cycles = append(h.cycles, r)
}

func (h *host) Reads(f *types.Field, _ []*types.Field) []int {
	return h.reads[f]
}

func (h *host) Savepoint() func(bool) {
	h.attempts++
	return func(undo bool) {
		if undo {
			h.undone++
		}
	}
}

// withDefault gives f a default the host evaluates to v.
func (h *host) withDefault(f *types.Field, v value.Value) *types.Field {
	f.Default = &syntax.IntLit{}
	h.defaults[f] = v
	return f
}

// decoded is the outcome of one decode: the value, and the findings as short lines.
type decoded struct {
	v        value.Value
	ok       bool
	err      error
	findings []string
}

// text is the value's canonical text, or its findings.
func (d decoded) text() string {
	if d.err != nil {
		return "error: " + d.err.Error()
	}
	if !d.ok {
		return strings.Join(d.findings, "; ")
	}
	return d.v.CanonText()
}

// decodeJSON decodes src as a.json against t with the decoder's options.
func decodeJSON(t testing.TB, dec wire.Decoder, src string, typ types.Type) decoded {
	t.Helper()
	fs := &source.FileSet{}
	f, err := fs.Add("a.json", "/p/a.json", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	dec.Bag = diag.NewBag(fs, "p")
	if dec.Pkg == "" {
		dec.Pkg = "p"
	}
	root, err := jsonsrc.Parse(f, dec.Bag)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	v, ok, err := dec.Decode(context.Background(), wire.Selection{Node: root}, typ)
	return decoded{v: v, ok: ok, err: err, findings: short(fs, dec.Bag.Findings())}
}

// short writes each finding as at(diag.E3302, "1:3", "/pointer").
func short(fs *source.FileSet, findings []diag.Finding) []string {
	var out []string
	for _, l := range diag.Locate(fs, findings) {
		out = append(out, fmt.Sprintf("%s %d:%d %s", l.Code, l.Loc.Line, l.Loc.Col, l.Pointer))
	}
	return out
}

// coder is a code of the registry, such as diag.E7110.
type coder interface{ Def() *diag.Def }

// at is a finding as short writes it: its code, line:col and pointer.
func at(c coder, pos, pointer string) string {
	return string(c.Def().Code) + " " + pos + " " + pointer
}

// findings is several findings as decoded.text writes them.
func findings(all ...string) string { return strings.Join(all, "; ") }

func opt(t types.Type) *types.OptionalType { return &types.OptionalType{Elem: t} }

func listOf(t types.Type) *types.ListType { return &types.ListType{Elem: t} }

// keyed is `[t] keyed by` its field key.
func keyed(t *types.RecordType, key int) *types.ListType {
	return &types.ListType{Elem: t, KeyedBy: t.Fields[key]}
}

// variantOf builds a variant with tag and cases, each case's wire its name.
func variantOf(pkg, name, tag string, cases ...*types.CaseType) *types.VariantType {
	v := &types.VariantType{Pkg: pkg, Name: name, Tag: tag, Cases: cases}
	for i, c := range cases {
		c.Variant, c.Index = v, i
		if c.Wire == "" {
			c.Wire = c.Name
		}
		for j, f := range c.Fields {
			f.Index = j
		}
	}
	return v
}

// potionFixture is pipeline.Potion (WIRE.md §8.3) with its `stack = 99` default.
func potionFixture(h *host) *types.RecordType {
	id := field("id", &types.Refined{Of: types.StringType}, "dwID")
	cooldown := field("cooldown", types.DurationType, "dwCooldownMs")
	stack := h.withDefault(field("stack", types.IntType, "nStack"), num(99))
	return record("pipeline", "Potion", id, field("name", types.StringType, "szName"),
		field("heal", types.IntType, "nHeal"), cooldown, stack)
}

// talentFixture is rules.canon's GuildTalentTree, whose nodes' parents are level-1 refs.
func talentFixture() (*types.RecordType, *types.Collection) {
	node := record("resource.rules", "TalentNode", field("id", types.IntType))
	tree := record("resource.rules", "GuildTalentTree")
	coll := &types.Collection{Kind: types.CollField, Owner: tree, FieldPath: []string{"nodes"}, Elem: node, KeyedBy: node.Fields[0]}
	parent := field("parent", opt(&types.RefType{Target: coll}))
	parent.Index, parent.NoneWire = 1, []byte("-1")
	node.Fields = append(node.Fields, parent)
	tree.Fields = []*types.Field{field("nodes", keyed(node, 0))}
	return tree, coll
}

// eventFixture is resource.events' Event with its inline variant `kind`, tag "type".
func eventFixture(h *host) *types.RecordType {
	rect := record("resource.events", "Rect", field("left", types.FloatType), field("top", types.FloatType),
		field("right", types.FloatType), field("bottom", types.FloatType))
	items := &types.Collection{Kind: types.CollLet, Pkg: "resource.vocab", Name: "items"}
	monsters := &types.Collection{Kind: types.CollDefines, Pkg: "resource.events", Name: "monsters"}
	lifetime := field("monsterLifetime", opt(types.DurationType), "monsterLifetimeSec")
	lifetime.Unit = types.UnitS
	count := h.withDefault(field("itemCount", listOf(types.IntType)), list(types.IntType, num(1), num(1)))
	kind := variantOf("resource.events", "EventKind", "type",
		&types.CaseType{Name: "spawn_monster", Fields: []*types.Field{field("monsterId", &types.RefType{Target: monsters}),
			field("spawnRegion", rect), lifetime}},
		&types.CaseType{Name: "monster_drop_inject", Methods: []*types.Method{{Name: "isFree", Export: true}}, Fields: []*types.Field{field("itemId", &types.RefType{Target: items}),
			count, h.withDefault(field("levelMin", types.IntType), num(0)), h.withDefault(field("levelMax", types.IntType), num(0))}})
	kindField := field("kind", kind)
	kindField.Inline, kindField.WirePath = true, nil
	weekday := enum("sovcommon.time", "Weekday", "Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat")
	clock := record("sovcommon.time", "TimeOfDay", field("hour", types.IntType), field("minute", types.IntType))
	window := record("sovcommon.time", "Window", field("day", weekday), field("startUtc", clock), field("endUtc", clock))
	rollMode := enum("resource.events", "RollMode", "authoritative", "local_budget")
	return record("resource.events", "Event", field("id", types.StringType), field("worldId", types.UInt32Type),
		field("targetCount", types.IntType), kindField, field("schedule", listOf(window)),
		h.withDefault(field("rollMode", rollMode), member(rollMode, 1)))
}
