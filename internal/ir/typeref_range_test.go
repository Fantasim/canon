package ir_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// rangeLim holds the bound another package's field names (TYPES.md §7.4: bounds are folded constants).
const rangeLim = `/// Limits.
package lim

/// The cap.
const MAX = 120

emit go { out: "@features/lim", package: "lim", mode: types }
`

// rangeSrc refines every position TYPES.md §7.4 allows, through aliases, optionals, unions, branches, refs and fns.
const rangeSrc = `/// A.
package a

import lim { MAX }

/// Coins.
type Penya = Int(0..)

/// A kind.
enum K { one, two }

/// A target.
record Item {
  /// Its key, refined: a ref to it is still an unchecked key.
  id: String(1..=8)
}

/// The items.
let items: [Item] keyed by id = [{ id: "x" }]

/// A payload.
type P(k: K) = match k { one => Int(1..=3), two => String }

/// A probe.
record Probe {
  /// Capped by another package's constant.
  level: Int(1..=MAX)
  /// A stacked alias.
  purse: Penya(..=1000)
  /// A list and its elements.
  tags: [Int(0..=5)](..=3)
  /// A map, its keys and values.
  weights: {String(1..=8): Float(0.0..1.0)}(1..)
  /// An optional field.
  note: String(..=16)?
  /// Optional elements.
  slots: [Int(0..9)?]
  /// A literal union's base.
  label: String(..=4) | "none"
  /// Its kind.
  k: K
  /// A dependent branch.
  p: P(k)
  /// A ref.
  item: ref items
  /// A Float32 bound, binary64.
  ratio: Float32(..=0.1)
  /// A Duration, in ms.
  wait: Duration(1s..=1h)

  /// Twice n.
  export fn twice(self, n: Int(0..=10)) -> Int(0..=20) { return n * 2 }
}

/// Settings.
record Gen {
  /// An input.
  limit: input Int(1..=50)? from env "LIMIT"
}

/// A top-level let.
let lv: Int(1..=9) = 3

/// The settings.
let gen: Gen = {}

/// Half of n.
export fn half(n: Int(-10..=10)) -> Int(-5..=5) { return n / 2 }

emit go { out: "@features/a", package: "a" }
`

// rangePackage is package a of rangeSrc after stage E, with no error.
func rangePackage(t *testing.T) *ir.Package {
	t.Helper()
	w := newWorld(t)
	w.add(t, "lim/lim.canon", []byte(rangeLim))
	w.add(t, "a/a.canon", []byte(rangeSrc))
	w.add(t, "a.items.json", []byte(`[{ "id": "x" }]`))
	w.add(t, "a.lv.json", []byte(`3`))
	w.add(t, "a.gen.json", []byte(`{}`))
	pkgs := w.build(t, "a")
	if out := w.findings(t); !strings.HasPrefix(out, "0 errors") {
		t.Fatalf("findings:\n%s", out)
	}
	for _, p := range pkgs {
		if p.Name == "a" {
			return p
		}
	}
	t.Fatal("no package a")
	return nil
}

// probeField is the field name of a record of p (Probe's, or Gen's input).
func probeField(t *testing.T, p *ir.Package, name string) *ir.Field {
	t.Helper()
	for _, typ := range p.Types {
		r, ok := typ.(*ir.Record)
		if !ok {
			continue
		}
		if i := slices.IndexFunc(r.Fields, func(f *ir.Field) bool { return f.Name == name }); i >= 0 {
			return r.Fields[i]
		}
	}
	t.Fatalf("no field %s", name)
	return nil
}

// incl is lo..=hi of integers (ms for a Duration); upTo ..=hi.
func incl(lo, hi int64) *types.Bound {
	return &types.Bound{Lo: types.Limit{I: lo}, Hi: types.Limit{I: hi}, HasLo: true, HasHi: true, HiIncluded: true}
}

func upTo(hi int64) *types.Bound {
	return &types.Bound{Hi: types.Limit{I: hi}, HasHi: true, HiIncluded: true}
}

// TYPES.md §7.4, CODEGEN.md §5.15, DECISIONS 399: each TypeRef level carries its own range or length refinement, an alias chain's ranges intersected, folded; nil where the level has none and on a ref's key.
func TestTypeRefRange(t *testing.T) {
	p := rangePackage(t)
	f := func(name string) *ir.TypeRef { return &probeField(t, p, name).Type }
	var probe *ir.Record
	for _, typ := range p.Types {
		if r, ok := typ.(*ir.Record); ok && r.Name == "Probe" {
			probe = r
		}
	}
	var dep *ir.Dependent
	for _, typ := range p.Types {
		if d, ok := typ.(*ir.Dependent); ok && d.Name == "P" {
			dep = d
		}
	}
	cases := []struct {
		name string
		got  *ir.TypeRef
		want *types.Bound
	}{
		{"another package's const", f("level"), incl(1, 120)},
		{"stacked alias", f("purse"), incl(0, 1000)},
		{"list length", f("tags"), upTo(3)},
		{"list element", f("tags").Elem, incl(0, 5)},
		{"map count", f("weights"), &types.Bound{Lo: types.Limit{I: 1}, HasLo: true}},
		{"map key", f("weights").Key, incl(1, 8)},
		{"map value", f("weights").Elem, &types.Bound{Lo: types.Limit{F: 0}, Hi: types.Limit{F: 1}, HasLo: true, HasHi: true}},
		{"optional field", f("note"), upTo(16)},
		{"list of optionals", f("slots"), nil},
		{"optional element itself", f("slots").Elem, nil},
		{"optional element", f("slots").Elem.Elem, &types.Bound{Lo: types.Limit{I: 0}, Hi: types.Limit{I: 9}, HasLo: true, HasHi: true}},
		{"literal union", f("label"), nil},
		{"literal union base", f("label").Elem, upTo(4)},
		{"dependent field", f("p"), nil},
		{"dependent branch", &dep.Branches[0].Type, incl(1, 3)},
		{"ref", f("item"), nil},
		{"ref key", f("item").Key, nil},
		{"Float32 in binary64", f("ratio"), &types.Bound{Hi: types.Limit{F: 0.1}, HasHi: true, HiIncluded: true}},
		{"Duration in ms", f("wait"), incl(1000, 3600000)},
		{"input", f("limit"), incl(1, 50)},
		{"fn param", &probe.Methods[0].Params[0].Type, incl(0, 10)},
		{"fn result", &probe.Methods[0].Result, incl(0, 20)},
		{"package fn negative bound", &p.Fns[0].Params[0].Type, incl(-10, 10)},
		{"top-level let", &p.Values[1].Type, incl(1, 9)},
		{"no refinement", f("k"), nil},
	}
	for _, c := range cases {
		if !reflect.DeepEqual(c.got.Range, c.want) {
			t.Errorf("%s: Range = %+v, want %+v", c.name, c.got.Range, c.want)
		}
	}
	if r := f("ratio").Range; r == nil || r.Hi.F != 0.1 {
		t.Errorf("Float32(..=0.1): Hi.F = %+v, want 0.1 exactly", r)
	}
}

// DECISIONS 399 (Decision 1): Field.Range, Param.Range and ResultRange stay, each equal to its TypeRef's Range.
func TestRangeMirrorsOwn(t *testing.T) {
	p := rangePackage(t)
	checked := 0
	mirror := func(what string, own, typ *types.Bound) {
		checked++
		if !reflect.DeepEqual(own, typ) {
			t.Errorf("%s: own %+v, TypeRef %+v", what, own, typ)
		}
	}
	fns := append([]*ir.ExportFn{}, p.Fns...)
	for _, typ := range p.Types {
		r, ok := typ.(*ir.Record)
		if !ok {
			continue
		}
		fns = append(fns, r.Methods...)
		for _, f := range r.Fields {
			if f.Input != nil {
				mirror(r.Name+"."+f.Name, f.Range, f.Type.Range)
			}
		}
	}
	for _, fn := range fns {
		mirror(fn.Name+" result", fn.ResultRange, fn.Result.Range)
		for _, par := range fn.Params {
			mirror(fn.Name+"."+par.Name, par.Range, par.Type.Range)
		}
	}
	if checked < 5 {
		t.Fatalf("checked %d mirrors, want the input, two params and two results", checked)
	}
}

// FINGERPRINT.md §5, DECISIONS 399 (Decision 8): a refinement is outside the hash, so the fingerprint of every value and of every record as a root is the same with each Range stripped.
func TestFingerprintIgnoresRange(t *testing.T) {
	p := rangePackage(t)
	var roots []*ir.TypeRef
	for _, v := range p.Values {
		roots = append(roots, &v.Type)
	}
	for _, typ := range p.Types {
		if r, ok := typ.(*ir.Record); ok {
			roots = append(roots, &ir.TypeRef{Kind: types.Record, Named: r})
		}
	}
	before := make([][]byte, len(roots))
	for i, root := range roots {
		text, err := ir.Fingerprint(root, methodsOf(root))
		if err != nil {
			t.Fatal(err)
		}
		before[i] = text
	}
	stripped := map[*ir.TypeRef]bool{}
	for _, typ := range p.Types {
		stripType(typ, stripped)
	}
	for _, root := range roots {
		stripRef(root, stripped)
	}
	if len(stripped) == 0 {
		t.Fatal("no Range to strip: the test proves nothing")
	}
	for i, root := range roots {
		after, err := ir.Fingerprint(root, methodsOf(root))
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before[i]) {
			t.Errorf("root %d: fingerprint moved with Range stripped:\n%s\nvs\n%s", i, before[i], after)
		}
	}
}

// methodsOf are the methods of a root's record, its `$fns` candidates.
func methodsOf(t *ir.TypeRef) []*ir.ExportFn {
	if t.Elem != nil && (t.Kind == types.List || t.Kind == types.Table) {
		t = t.Elem
	}
	if r, ok := t.Named.(*ir.Record); ok {
		return r.Methods
	}
	return nil
}

func stripType(typ ir.Type, done map[*ir.TypeRef]bool) {
	switch x := typ.(type) {
	case *ir.Record:
		for _, f := range x.Fields {
			stripRef(&f.Type, done)
		}
		for _, fn := range x.Methods {
			stripRef(&fn.Result, done)
		}
	case *ir.Dependent:
		for _, b := range x.Branches {
			stripRef(&b.Type, done)
		}
	}
}

// stripRef clears every Range of t, recording the levels that had one.
func stripRef(t *ir.TypeRef, done map[*ir.TypeRef]bool) {
	if t == nil {
		return
	}
	if t.Range != nil {
		done[t] = true
		t.Range = nil
	}
	stripRef(t.Elem, done)
	stripRef(t.Key, done)
}
