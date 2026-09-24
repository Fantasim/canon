package cppgen_test

import (
	"errors"
	"strings"
	"testing"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// small is a data-mode package with one record Thing of the fields given.
func small(fields ...*ir.Field) (*ir.Package, *ir.Emit) {
	rec := &ir.Record{Pkg: "demo", Name: "Thing", Fields: fields}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/out", Mode: ir.ModeData, Namespace: "demo"}
	return &ir.Package{Name: "demo", Dir: "demo", Types: []ir.Type{rec}, Emits: []*ir.Emit{emit}}, emit
}

func thing(p *ir.Package) *ir.Record { return p.Types[0].(*ir.Record) }

func withField(f *ir.Field) func(*ir.Package, *ir.Emit) {
	return func(p *ir.Package, _ *ir.Emit) { thing(p).Fields = append(thing(p).Fields, f) }
}

func withMethod(fn *ir.ExportFn) func(*ir.Package, *ir.Emit) {
	return func(p *ir.Package, _ *ir.Emit) { thing(p).Methods = append(thing(p).Methods, fn) }
}

// withValues gives the package keyed lists of Thing named names.
func withValues(p *ir.Package, reload bool, names ...string) {
	elem := ir.TypeRef{Kind: types.Record, Named: thing(p)}
	for _, n := range names {
		p.Values = append(p.Values, &ir.Value{Name: n, Reload: reload, Schema: "demo.Thing@00000001", Type: ir.TypeRef{
			Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "a", WirePath: []string{"a"}},
		}})
	}
}

// refusals: one row per construct refused rather than emitted wrongly (log-2026-09-24, gen/cpp).
func refusals() []struct {
	name string
	edit func(*ir.Package, *ir.Emit)
	want error
} {
	tList := listOf(tInt)
	optInt := ir.TypeRef{Kind: types.Optional, Elem: &tInt}
	return []struct {
		name string
		edit func(*ir.Package, *ir.Emit)
		want error
	}{
		{"embedded mode", func(_ *ir.Package, e *ir.Emit) { e.Mode = ir.ModeEmbedded }, cppgen.ErrUnsupported},
		{"types mode", func(_ *ir.Package, e *ir.Emit) { e.Mode = ir.ModeTypes }, cppgen.ErrUnsupported},
		{"baked mode", func(_ *ir.Package, e *ir.Emit) { e.Mode = ir.ModeBaked }, cppgen.ErrUnsupported},
		{"an input field", withField(&ir.Field{Name: "key", Type: tString, Input: &types.Input{Env: "KEY"}}), cppgen.ErrUnsupported},
		{"a dependent type", func(p *ir.Package, _ *ir.Emit) { p.Types = append(p.Types, &ir.Dependent{Pkg: "demo", Name: "D"}) }, cppgen.ErrUnsupported},
		{"a load.defines table", func(p *ir.Package, _ *ir.Emit) { p.Defines = []*ir.DefineTable{{Pkg: "demo", Value: "jobs"}} }, cppgen.ErrUnsupported},
		{"a table-typed field", withField(field("t", "t", "", ir.TypeRef{Kind: types.Table, Elem: &tInt})), cppgen.ErrUnsupported},
		{"a list of optionals", withField(field("o", "o", "", listOf(optInt))), cppgen.ErrUnsupported},
		{"a map field", withField(field("m", "m", "", ir.TypeRef{Kind: types.Map, Key: &tString, Elem: &tInt})), cppgen.ErrUnsupported},
		{"a non-string literal union", withField(field("u", "u", "", ir.TypeRef{Kind: types.LitUnion, Elem: &tInt})), cppgen.ErrUnsupported},
		{"an optional inline field", func(p *ir.Package, _ *ir.Emit) { optionalInline(p) }, cppgen.ErrUnsupported},
		{"an optional @stable field", func(p *ir.Package, _ *ir.Emit) { optionalStable(p) }, cppgen.ErrUnsupported},
		{"a legacy struct", func(p *ir.Package, _ *ir.Emit) { thing(p).Cpp.Struct = "ItemProp" }, cppgen.ErrUnsupported},
		{"a lookup over an Int", withMethod(&ir.ExportFn{Name: "f", Kind: ir.FnLookup, Result: tInt, Params: []*ir.Param{{Name: "n", Type: tInt}}}), cppgen.ErrUnsupported},
		{"a package-level stored fn", func(p *ir.Package, _ *ir.Emit) {
			p.Fns = []*ir.ExportFn{{Name: "f", Kind: ir.FnPrecomputed, Result: tInt}}
		}, cppgen.ErrUnsupported},
		{"a read that is no field", withMethod(translated("f", &ir.Read{Name: "g", Path: []string{"g"}, Type: tInt})), cppgen.ErrUnsupported},
		{"a call to another method", func(p *ir.Package, _ *ir.Emit) { methodCall(p) }, cppgen.ErrUnsupported},
		{"a type that holds itself", func(p *ir.Package, _ *ir.Emit) { selfHolding(p) }, cppgen.ErrUnsupported},
		{"a data value that is a plain list", func(p *ir.Package, _ *ir.Emit) {
			p.Values = []*ir.Value{{Name: "l", Schema: "s", Type: tList}}
		}, cppgen.ErrUnsupported},
		{"methods of a fieldless case", func(p *ir.Package, _ *ir.Emit) { fieldlessMethods(p) }, cppgen.ErrUnsupported},
		{"a pairs record of another package", func(p *ir.Package, _ *ir.Emit) { foreignPairs(p) }, cppgen.ErrUnsupported},
		{"an inline case key equal to a parent key but for case", func(p *ir.Package, _ *ir.Emit) { inlineFold(p, "k", "A") }, cppgen.ErrUnsupported},
		{"an inline tag equal to a parent key but for case", func(p *ir.Package, _ *ir.Emit) { inlineFold(p, "A", "c") }, cppgen.ErrUnsupported},
		{"a json emit", func(_ *ir.Package, e *ir.Emit) { e.Target = ir.TargetJSON }, cppgen.ErrTarget},
		{"no namespace (§2.1)", func(_ *ir.Package, e *ir.Emit) { e.Namespace = "" }, cppgen.ErrMalformed},
		{"a list without its element", withField(field("l", "l", "", ir.TypeRef{Kind: types.List})), cppgen.ErrMalformed},
		{"a ref without its key", withField(field("r", "r", "", ir.TypeRef{Kind: types.Ref})), cppgen.ErrMalformed},
		{"a record kind without its record", withField(field("r", "r", "", ir.TypeRef{Kind: types.Record})), cppgen.ErrMalformed},
		{"a nil field", withField(nil), cppgen.ErrMalformed},
		{"a translated method without its file (T3)", func(p *ir.Package, _ *ir.Emit) {
			fn := translated("f")
			fn.File = ""
			thing(p).Methods = append(thing(p).Methods, fn)
		}, cppgen.ErrMalformed},
		{"bits over a negative code", func(p *ir.Package, _ *ir.Emit) { negativeBits(p) }, cppgen.ErrMalformed},
		{"a keyed list without its key field", func(p *ir.Package, _ *ir.Emit) {
			withValues(p, false, "things")
			p.Values[0].Type.KeyedBy.Name = "nope"
		}, cppgen.ErrMalformed},
		{"a type of a package not imported", withField(field("o", "o", "", ir.TypeRef{Kind: types.Enum, Named: &ir.Enum{Pkg: "other", Name: "E"}})), cppgen.ErrMalformed},
		// The next four are a plan.Problems() stage E should already have refused (E8005/E8011; log-2026-09-24 "Generators trust stage E");
		// gen/cpp trusts it and reports ErrMalformed rather than re-deriving its own diagnosis.
		{"a storage name with __ (§3.4)", withField(field("x_", "x", "", tInt)), cppgen.ErrMalformed},
		{"a field ending in _ (§3.4, §7.2)", withField(field("a_", "b", "", tInt)), cppgen.ErrMalformed},
		{"FindBy names that collide (§3.5)", func(p *ir.Package, _ *ir.Emit) { stableCollision(p) }, cppgen.ErrMalformed},
		{"fields named alike (§3.5)", withField(field("A", "c", "", tInt)), cppgen.ErrMalformed},
	}
}

// TestRefusals: each refusal names its cause through a sentinel.
func TestRefusals(t *testing.T) {
	for _, c := range refusals() {
		t.Run(c.name, func(t *testing.T) {
			p, e := small(field("a", "a", "", tInt))
			c.edit(p, e)
			if _, err := cppgen.Generate(p, e); !errors.Is(err, c.want) {
				t.Errorf("Generate: %v, want %v", err, c.want)
			}
		})
	}
}

// TestSeveralHoldersResolve is CODEGEN.md §5.8, §5.11 (log-2026-09-24 "ir name plans + support plan").
func TestSeveralHoldersResolve(t *testing.T) {
	tests := []struct {
		name                    string
		leftReload, rightReload bool
		resolved                bool
	}{
		{"both @reload: every holder resolves into left", true, true, true},
		{"neither @reload: right does not resolve into left", false, false, false},
		{"target non-@reload, right @reload: right does not resolve into left", false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, e := small(field("a", "a", "", tInt))
			severalHolders(p, tt.leftReload, tt.rightReload)
			files, err := cppgen.Generate(p, e)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			header := string(mustHeader(t, files))
			if strings.Contains(header, "GetPeer()") != tt.resolved {
				t.Errorf("header has a resolved GetPeer(): %v, want %v\n%s", strings.Contains(header, "GetPeer()"), tt.resolved, header)
			}
			if !strings.Contains(header, "GetPeerKey()") {
				t.Errorf("header without the key getter GetPeerKey():\n%s", header)
			}
		})
	}
}

// mustHeader is the .gen.h file's content among files.
func mustHeader(t *testing.T, files []ir.File) []byte {
	t.Helper()
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".gen.h") {
			return f.Content
		}
	}
	t.Fatal("no .gen.h among the generated files")
	return nil
}

func translated(name string, reads ...*ir.Read) *ir.ExportFn {
	return &ir.ExportFn{
		Name: name, File: "thing.canon", Kind: ir.FnTranslated, Result: tInt, Reads: reads,
		Params: []*ir.Param{{Name: "n", Type: tInt}}, Body: &ir.ParamRef{T: tInt, Index: 0},
		Vectors: []*ir.Vector{vec(make([]value.Value, len(reads)), []value.Value{num(1)}, num(1), "")},
	}
}

func optionalInline(p *ir.Package) {
	v := &ir.Variant{Pkg: "demo", Name: "V", Tag: "k", Cases: []*ir.Case{{Name: "a", Wire: "a"}}}
	p.Types = append(p.Types, v)
	thing(p).Fields = append(thing(p).Fields, &ir.Field{Name: "v", Type: ir.TypeRef{Kind: types.Variant, Named: v}, Inline: true, Optional: true})
}

func optionalStable(p *ir.Package) {
	f := field("code", "code", "", tInt)
	f.Stable, f.Optional = true, true
	thing(p).Fields = append(thing(p).Fields, f)
	withValues(p, false, "things")
}

func methodCall(p *ir.Package) {
	other := translated("other")
	caller := translated("caller")
	caller.Body = &ir.CallFn{T: tInt, Fn: other, Args: []ir.PExpr{&ir.ParamRef{T: tInt, Index: 0}}}
	thing(p).Methods = []*ir.ExportFn{other, caller}
}

func selfHolding(p *ir.Package) {
	thing(p).Fields = append(thing(p).Fields, field("me", "me", "", ir.TypeRef{Kind: types.Record, Named: thing(p)}))
}

func fieldlessMethods(p *ir.Package) {
	v := &ir.Variant{Pkg: "demo", Name: "V", Tag: "k", Cases: []*ir.Case{
		{Name: "a", Wire: "a", Methods: []*ir.ExportFn{{Name: "f", Kind: ir.FnPrecomputed, Result: tInt}}},
	}}
	p.Types = append(p.Types, v)
}

// severalHolders gives Thing a "peer" ref into "left", also held by "right" (CODEGEN.md §5.8).
func severalHolders(p *ir.Package, leftReload, rightReload bool) {
	withValues(p, leftReload, "left")
	withValues(p, rightReload, "right")
	key := tInt
	r := ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "demo", Value: "left", Elem: thing(p), Keyed: true}}
	thing(p).Fields = append(thing(p).Fields, field("peer", "peer", "", r))
}

func foreignPairs(p *ir.Package) {
	pair := &ir.Record{Pkg: "other", Name: "Pair", Fields: []*ir.Field{field("k", "k", "", tInt), field("v", "v", "", tInt)}}
	f := &ir.Field{Name: "ps", Type: listOf(ir.TypeRef{Kind: types.Record, Named: pair}), Pairs: &types.Pairs{Keys: [2]string{"k{i}", "v{i}"}, Slots: 2}}
	thing(p).Fields = append(thing(p).Fields, f)
	p.Imports = []*ir.PackageRef{{Name: "other", Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "other/out", Namespace: "other"}}}}
}

func stableCollision(p *ir.Package) {
	a, b := field("fooBar", "x", "", tInt), field("foo_bar", "y", "", tInt)
	a.Stable, b.Stable = true, true
	a.Cpp.Name, b.Cpp.Name = "GetOne", "GetTwo"
	thing(p).Fields = append(thing(p).Fields, a, b)
	withValues(p, false, "things")
}

// inlineFold: Thing, keyed "a", holds inline a variant tagged tag whose case has a key key (WIRE.md §5.6).
func inlineFold(p *ir.Package, tag, key string) {
	v := &ir.Variant{Pkg: "demo", Name: "V", Tag: tag, Cases: []*ir.Case{{Name: "c", Wire: "c", Fields: []*ir.Field{field("x", key, "", tInt)}}}}
	p.Types = append(p.Types, v)
	thing(p).Fields = append(thing(p).Fields, &ir.Field{Name: "v", Type: ir.TypeRef{Kind: types.Variant, Named: v}, Inline: true})
}

// negativeBits is a bits field over a @codes enum with a negative code: no bit holds it (WIRE.md §5.3).
func negativeBits(p *ir.Package) {
	codes := tInt
	e := &ir.Enum{Pkg: "demo", Name: "F", Codes: &codes, Members: []*ir.EnumMember{{Name: "a", Wire: "a", Code: -1}}}
	p.Types = append(p.Types, e)
	f := field("fs", "fs", "", listOf(ir.TypeRef{Kind: types.Enum, Named: e}))
	f.Enc = types.EncBits
	thing(p).Fields = append(thing(p).Fields, f)
}
