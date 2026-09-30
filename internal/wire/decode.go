package wire

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Host is what decoding asks of the evaluator, which reports its false results (§4.8 of the plan).
type Host interface {
	// Default is f's default in the record `in`, its provenance `default` via `via` (EVALUATION §13).
	Default(ctx context.Context, f *types.Field, in Instance, via *value.Prov) (value.Value, bool)
	// Deref is the entry r names; false when there is none or its collection is poisoned.
	Deref(ctx context.Context, r *value.Ref) (*value.Record, bool)
	// Bind keeps the arguments the decoder bound to an applied record instance (TYPES.md §11.1).
	Bind(rec *value.Record, params map[*types.Param]value.Value)
	// Cycle reports that the entry r names is needed to decode itself (EVALUATION.md §3.2).
	Cycle(ctx context.Context, r *value.Ref)
	// Reads is the fields of fields, f's record, that f's default reads (TYPES.md §15).
	Reads(f *types.Field, fields []*types.Field) []int
	// Savepoint marks a decoding attempt; end(true) takes back its steps and bindings.
	Savepoint() (end func(undo bool))
}

// Instance is what a default sees of the record being decoded: the fields decoded so far,
// later ones nil, and the values bound to its parameters.
type Instance struct {
	Record *value.Record
	Params map[*types.Param]value.Value
}

// Decoder reads JSON values and CSV cells as their expected types (WIRE.md §5, §6.6).
type Decoder struct {
	Bag     *diag.Bag
	Pkg     string            // whose names findings print unqualified
	Host    Host              // nil: a default or a dereference is ErrNoHost
	Partial bool              // WIRE.md §6.4
	Keep    bool              // report E3302, E3201, E3202, E3102 and E3317 but keep the value (API.md V2)
	Coll    *types.Collection // what a whole decoded table or keyed list is (TYPES.md §6.3)
	Outer   Outer             // what the decoded value's type arguments name around it
	Field   *types.Field      // the field the decoded whole is the value of: its unit, int, bits and none marker (§4.1)
	Elems   Elems             // load.dir's elements kept apart, which Dir asks for (decode_elems.go)
}

// Outer is what a loaded value's type arguments may name around it: the record whose field the
// load gives, that record's arguments, the dependent map binders.
type Outer struct {
	Record  *value.Record
	Params  map[*types.Param]value.Value
	Binders map[string]value.Value
}

// Selection is what `at:` selects: a value, or under a `*` one item per member (WIRE.md §6.3).
type Selection struct {
	Node  *jsonsrc.Node
	Star  bool
	Items []Selection
}

// Decode reads sel as t: not ok after a finding (WIRE.md §3.4); err is the caller's misuse.
func (d *Decoder) Decode(ctx context.Context, sel Selection, t types.Type) (value.Value, bool, error) {
	r := d.start(ctx, t)
	return r.result(r.whole(sel, t))
}

// whole decodes the decoded whole: in the scope of Decoder.Field when it names one, its none
// marker none as for a member (WIRE.md 4.1, 5.4).
func (r *run) whole(sel Selection, t types.Type) value.Value {
	sc := wscope{root: true, fr: r.rootFrame()}
	f := r.d.Field
	if f == nil {
		return r.value(sel, t, sc)
	}
	sc.unit, sc.asInt, sc.bits = f.Unit, f.Enc == types.EncInt, f.Enc == types.EncBits
	if n := sel.Node; !sel.Star && t.Kind() == types.Optional && f.NoneWire != nil && isMarker(n, f.NoneWire) {
		return &value.None{T: t, P: prov(n)}
	}
	return r.value(sel, t, sc)
}

// run is one call of the decoder: where it is, and whether it reported or failed; the records
// whose dependent fields wait for the second pass are in later (decode_later.go).
type run struct {
	ctx      context.Context
	d        *Decoder
	colls    map[collKey]*types.Collection
	trail    []step
	failed   bool
	err      error
	entryKey *types.Field // the key field of the keyed-list element decoded next
	later    []*pending
	waiting  map[*value.Record]*pending
	local    map[value.Key]*value.Record // Decoder.Coll's entries by key
	pass2    bool
	needs    []want            // the records an attempt of the second pass needs
	needed   map[*pending]bool // the same, as a set; nil outside an attempt
	apps     map[types.Type]bool
	deps     map[types.Type]bool
	elems    Elems // Decoder.Elems, asked by Dir only
	reported int   // the findings reported, and the host calls an element decoded purely never makes
	hosted   int
}

// rootFrame is what the decoded whole's type arguments name: the Decoder's Outer.
func (r *run) rootFrame() *frame {
	o := r.d.Outer
	return &frame{params: o.Params, binders: o.Binders, rec: o.Record}
}

// wscope is what a value inherits from its field (WIRE.md §4.1) and the names its type may use.
type wscope struct {
	unit  types.Unit
	asInt bool
	bits  bool
	root  bool // the value is the decoded whole, which Decoder.Coll names when a collection
	fr    *frame
}

// fieldScope is what a field gives its value: its unit and encoding, its record's frame.
func fieldScope(f *types.Field, fr *frame) wscope {
	return wscope{unit: f.Unit, asInt: f.Enc == types.EncInt, bits: f.Enc == types.EncBits, fr: fr}
}

// inner is the scope of an element or map value: the unit and int form carry, bits does not.
func (s wscope) inner() wscope {
	return wscope{unit: s.unit, asInt: s.asInt, fr: s.fr}
}

func (d *Decoder) start(ctx context.Context, t types.Type) *run {
	return &run{
		ctx: ctx, d: d, colls: fieldCollections(t), waiting: map[*value.Record]*pending{},
		apps: map[types.Type]bool{}, deps: map[types.Type]bool{},
	}
}

func (r *run) result(v value.Value) (value.Value, bool, error) {
	if !r.finishAll() {
		r.failed = true
	}
	if r.err != nil {
		return nil, false, r.err
	}
	if r.failed || v == nil {
		return nil, false, nil
	}
	return v, true, nil
}

// decoders dispatches a value on its type's kind (DECISIONS 26); a nil entry has no wire form.
var decoders [types.Error + 1]func(*run, Selection, types.Type, wscope) value.Value

func init() {
	decoders[types.Bool], decoders[types.Int] = (*run).boolean, (*run).integer
	decoders[types.Float], decoders[types.String] = (*run).float, (*run).str
	decoders[types.Duration], decoders[types.Enum] = (*run).duration, (*run).enum
	decoders[types.Record], decoders[types.Define] = (*run).record, (*run).record
	decoders[types.Variant], decoders[types.Optional] = (*run).variant, (*run).optional
	decoders[types.List], decoders[types.Map] = (*run).list, (*run).mapValue
	decoders[types.DepMap], decoders[types.Table] = (*run).depMap, (*run).table
	decoders[types.Ref], decoders[types.LitUnion] = (*run).ref, (*run).litUnion
	decoders[types.TypeApp], decoders[types.Never] = (*run).dependent, (*run).never
}

// value decodes sel as t: null only as none, a `*` only into a list or map (WIRE.md §5.4).
func (r *run) value(sel Selection, t types.Type, sc wscope) value.Value {
	k := t.Kind()
	if int(k) >= len(decoders) || decoders[k] == nil {
		r.misuse(ErrNoWireType, t)
		return nil
	}
	if sel.Star && (!starKinds[k] || len(sel.Items) != len(sel.Node.Elems)+len(sel.Node.Members)) {
		r.misuse(ErrStar, t)
		return nil
	}
	if n := sel.Node; n.Kind == jsonsrc.Null && !nullKinds[k] {
		r.report(diag.E3315.AtType(n.Span, t), n)
		return nil
	}
	return decoders[k](r, sel, t, sc)
}

func (r *run) optional(sel Selection, t types.Type, sc wscope) value.Value {
	if sel.Node.Kind == jsonsrc.Null {
		return &value.None{T: t, P: prov(sel.Node)}
	}
	return r.value(sel, t.Base().(*types.OptionalType).Elem, sc)
}

// never is a plain Never, which only `none` has (TYPES.md §13.5).
func (r *run) never(sel Selection, t types.Type, _ wscope) value.Value {
	return r.mismatch(sel.Node, diag.KindNull, t)
}

// report adds a finding located at n (with its RFC 6901 pointer) and fails the decode.
func (r *run) report(b *diag.Builder, n *jsonsrc.Node) {
	if n != nil {
		b.Pointer(n.Pointer())
	}
	b.Report(r.d.Bag)
	r.reported++
	r.failed = true
}

// soft reports a finding about the value that the re-check owns: with Decoder.Keep the decode
// goes on and keeps the value, which soft reports; without, it is report.
func (r *run) soft(b *diag.Builder, n *jsonsrc.Node) bool {
	if !r.d.Keep {
		r.report(b, n)
		return false
	}
	if n != nil {
		b.Pointer(n.Pointer())
	}
	b.Report(r.d.Bag)
	r.reported++
	return true
}

// mismatch is E7110: n is not of the JSON kind t needs (WIRE.md §5).
func (r *run) mismatch(n *jsonsrc.Node, want diag.Kind, t types.Type) value.Value {
	r.report(diag.E7110.AtKind(n.Span, want, t, jsonKinds[n.Kind]), n)
	return nil
}

// misuse records the first error that is the caller's, not the data's.
func (r *run) misuse(err error, t types.Type) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: %v", err, t)
	}
}

// name is a declaration's name as findings print it: bare in the decoder's package.
func (r *run) name(pkg, name string) string {
	if pkg == "" || pkg == r.d.Pkg {
		return name
	}
	return pkg + pointSep + name
}

// site is where a value is read: a JSON value, an object key or a CSV cell's own pointer (EVALUATION.md §13).
type site struct {
	span    source.Span
	node    *jsonsrc.Node
	kind    value.ProvKind
	pointer string
}

func at(n *jsonsrc.Node) site { return site{span: n.Span, node: n, kind: value.ProvJSON} }

func keyAt(m *jsonsrc.Member) site {
	return site{span: m.KeySpan, node: m.Value, kind: value.ProvJSON}
}

func (s site) prov() *value.Prov {
	p := &value.Prov{Kind: s.kind, Span: s.span, Pointer: s.pointer}
	if s.node != nil {
		p.Pointer = s.node.Pointer()
	}
	return p
}

// prov is the provenance of the JSON value n.
func prov(n *jsonsrc.Node) *value.Prov { return at(n).prov() }

// child is element or member i of sel: its item under a `*`, else the node's own.
func (sel Selection) child(i int) Selection {
	if sel.Star {
		return sel.Items[i]
	}
	if sel.Node.Kind == jsonsrc.Array {
		return Selection{Node: sel.Node.Elems[i]}
	}
	return Selection{Node: sel.Node.Members[i].Value}
}

// literal is a text from a file as a finding's value argument (ERRORS.md §1.3).
type literal string

func (l literal) CanonText() string { return string(l) }

// spanOf is the span from a's start to b's end.
func spanOf(a, b source.Span) source.Span {
	return source.Span{File: a.File, Start: a.Start, End: b.End}
}
