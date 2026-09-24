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
	Coll    *types.Collection // what a whole decoded table or keyed list is (TYPES.md §6.3)
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
	v := r.value(sel, t, wscope{root: true, fr: &frame{}})
	return r.result(v)
}

// run is one call of the decoder: where it is, and whether it reported or failed.
type run struct {
	ctx    context.Context
	d      *Decoder
	colls  map[collKey]*types.Collection
	trail  []step
	failed bool
	err    error
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
	return &run{ctx: ctx, d: d, colls: fieldCollections(t)}
}

func (r *run) result(v value.Value) (value.Value, bool, error) {
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
	r.failed = true
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

// site is where a value is read: a JSON value, an object key or a CSV cell (EVALUATION.md §13).
type site struct {
	span source.Span
	node *jsonsrc.Node
	kind value.ProvKind
}

func at(n *jsonsrc.Node) site { return site{span: n.Span, node: n, kind: value.ProvJSON} }

func keyAt(m *jsonsrc.Member) site {
	return site{span: m.KeySpan, node: m.Value, kind: value.ProvJSON}
}

func (s site) prov() *value.Prov {
	p := &value.Prov{Kind: s.kind, Span: s.span}
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
