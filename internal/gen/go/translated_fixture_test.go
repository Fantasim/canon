package gogen_test

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// The codes a vector expects (CONFORMANCE.md §3).
var (
	eOverflow  = diag.E4101.Def().Code
	eDivZero   = diag.E4102.Def().Code
	eToInt     = diag.E4103.Def().Code
	eNotFinite = diag.E4104.Def().Code
	eClamp     = diag.E4108.Def().Code
	eWidth     = diag.E3201.Def().Code
	eF32       = diag.E3202.Def().Code
	eRange     = diag.E3204.Def().Code
)

const calcModule = "example.com/calc"

// vec is one vector: the reads of self, the arguments, then a result or a code.
type vec struct {
	recv, args []value.Value
	want       value.Value
	code       diag.Code
}

// spec is a translated fn: its signature, reads, body and hand-computed vectors (CONFORMANCE.md §3).
type spec struct {
	name   string
	params []*ir.Param
	result ir.TypeRef
	rng    *types.Bound
	reads  []*ir.Read
	body   ir.PExpr
	vecs   []vec
}

func (s spec) build(order int) *ir.ExportFn {
	fn := &ir.ExportFn{
		Name: s.name, Doc: "Translated " + s.name + ".", File: "calc.canon", Order: order, Kind: ir.FnTranslated,
		Params: s.params, Result: s.result, ResultRange: s.rng, Reads: s.reads, Body: s.body,
	}
	for _, v := range s.vecs {
		fn.Vectors = append(fn.Vectors, &ir.Vector{Recv: v.recv, Args: v.args, Want: v.want, Code: v.code})
	}
	return fn
}

func num(n int64) *value.Int              { return &value.Int{V: n} }
func flt(f float64) *value.Float          { return &value.Float{V: f} }
func ms(n int64) *value.Dur               { return &value.Dur{Ms: n} }
func str(s string) *value.Str             { return &value.Str{V: s} }
func yes(b bool) *value.Bool              { return &value.Bool{V: b} }
func vals(v ...value.Value) []value.Value { return v }

func ok(recv []value.Value, want value.Value, args ...value.Value) vec {
	return vec{recv: recv, args: args, want: want}
}

func fails(recv []value.Value, code diag.Code, args ...value.Value) vec {
	return vec{recv: recv, args: args, code: code}
}

func lit(t ir.TypeRef, v value.Value) *ir.Lit { return &ir.Lit{T: t, V: v} }
func prm(i int, t ir.TypeRef) *ir.ParamRef    { return &ir.ParamRef{T: t, Index: i} }
func rd(i int, t ir.TypeRef) *ir.ReadRef      { return &ir.ReadRef{T: t, Index: i} }
func lcl(name string) *ir.LocalRef            { return &ir.LocalRef{T: intT, Name: name} }
func bin(op ir.Op, t ir.TypeRef, x, y ir.PExpr) *ir.Binary {
	return &ir.Binary{T: t, Op: op, X: x, Y: y}
}
func blk(stmts ...ir.Stmt) *ir.Block { return &ir.Block{T: intT, Stmts: stmts} }
func ret(x ir.PExpr) *ir.ReturnStmt  { return &ir.ReturnStmt{X: x} }

func builtin(fn ir.Builtin, t ir.TypeRef, args ...ir.PExpr) *ir.Call {
	return &ir.Call{T: t, Fn: fn, Args: args}
}

func params(t ir.TypeRef, names ...string) []*ir.Param {
	out := make([]*ir.Param, len(names))
	for i, n := range names {
		out[i] = &ir.Param{Name: n, Type: t}
	}
	return out
}

// calc is a baked package whose translated methods and package fns use every helper, statement shape and conversion of CONFORMANCE.md §2–§3, in declaration order.
type calc struct {
	tone             *ir.Enum
	shape            *ir.Variant
	bonus, potion    *ir.Record
	size             *ir.Record
	toneT, shapeT    ir.TypeRef
	sizeRef          ir.TypeRef
	fns              []*ir.ExportFn
	loud, pick, incr *ir.ExportFn
}

func newCalc() *calc {
	c := &calc{}
	c.tone = &ir.Enum{Pkg: "calc", Name: "Tone", Doc: "A tone.", Members: []*ir.EnumMember{{Name: "soft", Wire: "soft"}, {Name: "loud", Wire: "loud", Index: 1}}}
	c.toneT = typed(c.tone, types.Enum)
	circle := &ir.Case{Name: "circle", Wire: "circle", Doc: "A circle.", Fields: []*ir.Field{wired("r", "r", "Its radius.", intT)}}
	c.shape = &ir.Variant{Pkg: "calc", Name: "Shape", Doc: "A shape.", Tag: "kind", Cases: []*ir.Case{circle, {Name: "dot", Wire: "dot", Doc: "A dot."}}}
	c.shapeT = typed(c.shape, types.Variant)
	c.bonus = &ir.Record{Pkg: "calc", Name: "Bonus", Doc: "A bonus.", Fields: []*ir.Field{wired("value", "value", "Its value.", intT)}}
	c.potion = &ir.Record{Pkg: "calc", Name: "Potion", Doc: "A potion.", Fields: []*ir.Field{
		wired("name", "name", "Its name.", strT), wired("heal", "heal", "Hit points.", intT),
		wired("tone", "tone", "Its tone.", c.toneT), opt(wired("bonus", "bonus", "A bonus.", typed(c.bonus, types.Record))),
		opt(wired("level", "level", "A level.", intT)), wired("cooldown", "cooldown", "A cooldown.", durT),
		wired("weight", "weight", "A weight.", i32T), wired("ratio", "ratio", "A ratio.", f32T),
		wired("shape", "shape", "A shape.", c.shapeT),
	}}
	c.size = &ir.Record{Pkg: "calc", Name: "Size", Doc: "A size.", Fields: []*ir.Field{wired("cap", "cap", "Its cap.", intT)}}
	c.sizeRef = ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "calc", Value: "sizes", Elem: c.size}}
	return c
}

// pkg is the package: types, the sizes table, then the fns in declaration order.
func (c *calc) pkg() *ir.Package {
	order := 0
	next := func(s spec) *ir.ExportFn { order++; return s.build(order) }
	tenth := &ir.ExportFn{Name: "tenth", Doc: "A tenth.", Kind: ir.FnPrecomputed, Result: intT, File: "calc.canon"}
	for _, s := range c.methods() {
		c.potion.Methods = append(c.potion.Methods, next(s))
	}
	c.potion.Methods = append(c.potion.Methods, tenth)
	c.shape.Cases[0].Methods = []*ir.ExportFn{next(c.caseMethod())}
	c.lookups()
	for _, s := range c.packageFns() {
		c.fns = append(c.fns, next(s))
	}
	emit := &ir.Emit{Target: ir.TargetGo, Out: "out/go/", Dir: "calc/out/go", GoImport: calcModule, Mode: ir.ModeBaked, GoPackage: "calc"}
	return &ir.Package{
		Name: "calc", Dir: "calc", Types: []ir.Type{c.tone, c.shape, c.bonus, c.potion, c.size},
		Values: []*ir.Value{c.sizes()}, Fns: append([]*ir.ExportFn{c.loud, c.pick, c.incr}, c.fns...), Emits: []*ir.Emit{emit},
	}
}

// sizes is a table of two entries, the target of pick's ref result.
func (c *calc) sizes() *ir.Value {
	twin := &types.RecordType{Pkg: "calc", Name: "Size", Fields: []*types.Field{{Name: "cap"}}}
	coll := &types.Collection{Kind: types.CollLet, Pkg: "calc", Name: "sizes"}
	entry := func(id string, n int64) *value.Record {
		return &value.Record{T: twin, Fields: []value.Value{num(n)}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}}}
	}
	return &ir.Value{
		Name: "sizes", Doc: "The sizes.", Type: ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: c.size}},
		IDs: []string{"small", "large"}, V: &value.Table{Entries: []*value.Record{entry("small", 1), entry("large", 9)}},
	}
}

// lookups are the fns translated bodies call: a lookup of a Bool, of a ref, and a translated one.
func (c *calc) lookups() {
	soft, loud := &value.Member{Index: 0}, &value.Member{Index: 1}
	c.loud = &ir.ExportFn{
		Name: "loud", Doc: "Whether t is loud.", Kind: ir.FnLookup, Result: boolT, Params: []*ir.Param{{Name: "t", Type: c.toneT}},
		Table: &ir.LookupTable{Domains: [][]value.Value{{soft, loud}}, Cells: []value.Value{yes(false), yes(true)}},
	}
	c.pick = &ir.ExportFn{
		Name: "pick", Doc: "The size for big.", Kind: ir.FnLookup, Result: c.sizeRef, Params: []*ir.Param{{Name: "big", Type: boolT}},
		Table: &ir.LookupTable{
			Domains: [][]value.Value{{yes(false), yes(true)}},
			Cells:   []value.Value{&value.Ref{Key: value.Key{S: "small"}}, &value.Ref{Key: value.Key{S: "large"}}},
		},
	}
	c.incr = spec{
		name: "incr", params: params(intT, "x"), result: intT, body: bin(ir.OpAdd, intT, prm(0, intT), lit(intT, num(1))),
		vecs: []vec{ok(nil, num(2), num(1)), fails(nil, eOverflow, num(math.MaxInt64))},
	}.build(0)
}
