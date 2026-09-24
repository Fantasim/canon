package wire

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// object is a JSON object read as a record, with the members its fields claimed (WIRE.md §5.5).
type object struct {
	n      *jsonsrc.Node
	claims map[*jsonsrc.Node][]bool
	bad    map[*jsonsrc.Node]bool
}

func newObject(n *jsonsrc.Node) *object {
	o := &object{n: n, claims: map[*jsonsrc.Node][]bool{}, bad: map[*jsonsrc.Node]bool{}}
	o.open(n)
	return o
}

// open makes n an object whose unclaimed keys are unknown.
func (o *object) open(n *jsonsrc.Node) {
	if o.claims[n] == nil {
		o.claims[n] = make([]bool, len(n.Members))
	}
}

// claim is the member of n with that key, marked claimed; nil when there is none.
func (o *object) claim(n *jsonsrc.Node, key string) *jsonsrc.Member {
	for i := range n.Members {
		if n.Members[i].Key == key {
			o.claims[n][i] = true
			return &n.Members[i]
		}
	}
	return nil
}

// rowState is what a table row's `$retired` said.
type rowState struct {
	retired bool
}

// record is a record value: every field decoded, in declaration order (WIRE.md §5.5.1).
func (r *run) record(sel Selection, t types.Type, sc wscope) value.Value {
	if v := r.recordAt(sel.Node, t, sc.fr, nil); v != nil {
		return v
	}
	return nil
}

// recordAt decodes the object n as t; a table row also reads its `$retired` (WIRE.md §5.7).
func (r *run) recordAt(n *jsonsrc.Node, t types.Type, outer *frame, rw *rowState) *value.Record {
	if n.Kind != jsonsrc.Object {
		r.mismatch(n, diag.KindObject, t)
		return nil
	}
	fields, params, ok := r.shape(t, outer)
	if !ok {
		return nil
	}
	o := newObject(n)
	ok = rw == nil || r.retired(o, rw)
	rv := &value.Record{T: t, Fields: make([]value.Value, len(fields)), Set: make([]bool, len(fields)), P: prov(n)}
	ok = r.fields(o, fields, rv, &frame{params: params, rec: rv, failed: !ok}) && ok
	if !r.unknown(o, n, t) || !ok {
		return nil
	}
	return rv
}

// shape is the fields of a record type, and the values of its parameters for R(args).
func (r *run) shape(t types.Type, outer *frame) ([]*types.Field, map[*types.Param]value.Value, bool) {
	switch d := t.Base().(type) {
	case *types.RecordType:
		return d.Fields, nil, true
	case *types.AppliedRecord:
		params, ok := r.bind(d.Rec.Params, d.Args, outer)
		return d.Rec.Fields, params, ok
	}
	r.misuse(ErrNoWireType, t)
	return nil, nil, false
}

// retired reads a row's `$retired`, which must be the JSON true (WIRE.md §5.7).
func (r *run) retired(o *object, rw *rowState) bool {
	m := o.claim(o.n, keyRetired)
	switch {
	case m == nil:
		return true
	case m.Value.Kind == jsonsrc.Bool && m.Value.Text == textTrue:
		rw.retired = true
		return true
	case isContainer(m.Value):
		r.mismatch(m.Value, diag.KindBoolean, types.BoolType)
	default:
		r.report(diag.E7110.AtRetired(m.Value.Span, jsonText(m.Value)), m.Value)
	}
	return false
}

// fields decodes each field into rv, reporting every failure before giving up.
func (r *run) fields(o *object, fields []*types.Field, rv *value.Record, fr *frame) bool {
	r.enter(rv)
	defer r.leave()
	ok := true
	for i, f := range fields {
		r.trail = append(r.trail, step{field: f.Name})
		ok = r.field(o, f, i, rv, fr) && ok
		fr.failed = fr.failed || !ok
		r.trail = r.trail[:len(r.trail)-1]
	}
	return ok
}

// field decodes field i: an input, an inline variant, pairs slots, or its key path.
func (r *run) field(o *object, f *types.Field, i int, rv *value.Record, fr *frame) bool {
	switch {
	case f.Input != nil:
		return r.input(o, f, rv.T)
	case f.Inline:
		return r.inline(o, f, i, rv)
	case f.Pairs != nil:
		return r.pairs(o, f, i, rv)
	case len(f.WirePath) == 0:
		r.misuse(ErrNoWireType, f.Type)
		return false
	}
	m, ok := r.lookup(o, f.WirePath, rv.T)
	switch {
	case !ok:
		return false
	case m == nil:
		return r.absent(o.n, f, i, rv, fr)
	}
	rv.Set[i] = true
	rv.Fields[i] = r.present(m.Value, f, fr)
	return rv.Fields[i] != nil
}

// input is a runtime input: it has no wire form, and its key in the data is E3312 (TYP-17).
func (r *run) input(o *object, f *types.Field, t types.Type) bool {
	if len(f.WirePath) == 0 {
		return true
	}
	m, ok := r.lookup(o, f.WirePath, t)
	if ok && m != nil {
		r.report(diag.E3312.At(m.KeySpan, f.Name), m.Value)
		return false
	}
	return ok
}

// lookup follows a key path: nil when absent, not ok past a non-object (WIRE.md §5.5.3).
func (r *run) lookup(o *object, path []string, t types.Type) (*jsonsrc.Member, bool) {
	cur := o.n
	for _, key := range path[:len(path)-1] {
		m := o.claim(cur, key)
		if m == nil {
			return nil, true
		}
		if m.Value.Kind != jsonsrc.Object {
			if !o.bad[m.Value] {
				o.bad[m.Value] = true
				r.mismatch(m.Value, diag.KindObject, t)
			}
			return nil, false
		}
		cur = m.Value
		o.open(cur)
	}
	return o.claim(cur, path[len(path)-1]), true
}

// present decodes a field's value: `null` and the field's none marker are none (WIRE.md §5.4).
func (r *run) present(n *jsonsrc.Node, f *types.Field, fr *frame) value.Value {
	optional := f.Type.Kind() == types.Optional
	switch {
	case n.Kind == jsonsrc.Null && !optional:
		r.report(diag.E3315.AtField(n.Span, f.Name), n)
		return nil
	case optional && (n.Kind == jsonsrc.Null || f.NoneWire != nil && isMarker(n, f.NoneWire)):
		return &value.None{T: f.Type, P: prov(n)}
	}
	return r.value(Selection{Node: n}, f.Type, fieldScope(f, fr))
}

// absent is a field without key: its default, none, or E3302 at the object (WIRE.md §5.4).
func (r *run) absent(n *jsonsrc.Node, f *types.Field, i int, rv *value.Record, fr *frame) bool {
	v, required := r.fill(f, rv, fr, prov(n))
	if required {
		r.report(diag.E3302.At(n.Span, rv.T, f.Name), n)
		return false
	}
	rv.Fields[i] = v
	return v != nil
}

// fill is an absent field's value: none for an optional without default, else its default,
// which the host evaluates, unless the record has failed; required when there is neither.
func (r *run) fill(f *types.Field, rv *value.Record, fr *frame, via *value.Prov) (value.Value, bool) {
	switch {
	case f.Pairs != nil:
		return &value.List{T: f.Type, P: via}, false
	case f.Type.Kind() == types.Optional && f.Default == nil:
		return &value.None{T: f.Type, P: &value.Prov{Kind: value.ProvDefault, Via: via}}, false
	case f.Default == nil:
		return nil, true
	case fr.failed:
		return nil, false
	case r.d.Host == nil:
		r.misuse(ErrNoHost, f.Type)
		return nil, false
	}
	v, ok := r.d.Host.Default(r.ctx, f, Instance{Record: rv, Params: fr.params}, via)
	if !ok {
		r.failed = true
		return nil, false
	}
	return v, false
}

// unknown reports each key no field claimed, unless partial (E3301, WIRE.md §5.5.1, §6.4).
func (r *run) unknown(o *object, n *jsonsrc.Node, t types.Type) bool {
	ok := true
	for i, m := range n.Members {
		switch {
		case o.claims[n][i] && o.claims[m.Value] != nil:
			ok = r.unknown(o, m.Value, t) && ok
		case o.claims[n][i], r.d.Partial, m.Key == keySchema && n.Pointer() == "":
		default:
			r.report(diag.E3301.At(m.KeySpan, t, m.Key), m.Value)
			ok = false
		}
	}
	return ok
}

// variant is a variant value: an object whose tag names the case (WIRE.md §5.6).
func (r *run) variant(sel Selection, t types.Type, _ wscope) value.Value {
	n := sel.Node
	if n.Kind != jsonsrc.Object {
		return r.mismatch(n, diag.KindObject, t)
	}
	o := newObject(n)
	c := r.caseOf(o, t.Base().(*types.VariantType))
	if c == nil {
		return nil
	}
	cv := r.caseValue(o, c)
	if !r.unknown(o, n, c) || cv == nil {
		return nil
	}
	return cv
}

// inline is a variant field whose tag and case fields sit in the parent object (WIRE.md §5.6).
func (r *run) inline(o *object, f *types.Field, i int, rv *value.Record) bool {
	vt, ok := f.Type.Base().(*types.VariantType)
	if !ok {
		r.misuse(ErrNoWireType, f.Type)
		return false
	}
	c := r.caseOf(o, vt)
	if c == nil {
		claimCases(o, vt)
		return false
	}
	rv.Set[i] = true
	cv := r.caseValue(o, c)
	if cv == nil {
		return false
	}
	rv.Fields[i] = cv
	return true
}

// claimCases claims every case's keys once a tag failed: no E3301 follows (EVALUATION.md §7.2).
func claimCases(o *object, vt *types.VariantType) {
	for _, c := range vt.Cases {
		for _, f := range c.Fields {
			switch {
			case f.Pairs != nil:
				slotsOf(o, f.Pairs)
			case !f.Inline && len(f.WirePath) > 0:
				o.claim(o.n, f.WirePath[0])
			}
		}
		for _, m := range c.Methods {
			if m.Export {
				o.claim(o.n, keyDollar+m.Name)
			}
		}
	}
}

// caseOf is the case the object's tag names: E7112 without a tag or for an unknown case.
func (r *run) caseOf(o *object, vt *types.VariantType) *types.CaseType {
	name := r.name(vt.Pkg, vt.Name)
	m := o.claim(o.n, vt.Tag)
	switch {
	case m == nil:
		r.report(diag.E7112.AtTag(o.n.Span, vt.Tag, name), o.n)
		return nil
	case m.Value.Kind != jsonsrc.String:
		r.mismatch(m.Value, diag.KindString, vt)
		return nil
	}
	wires := make([]string, len(vt.Cases))
	for i, c := range vt.Cases {
		if c.Wire == m.Value.Text {
			return c
		}
		wires[i] = c.Wire
	}
	r.report(diag.E7112.AtCase(m.Value.Span, m.Value.Text, name, wires), m.Value)
	return nil
}

// caseValue decodes the case's fields from o; a case sees no enclosing parameter.
func (r *run) caseValue(o *object, c *types.CaseType) *value.Record {
	cv := &value.Record{T: c, Fields: make([]value.Value, len(c.Fields)), Set: make([]bool, len(c.Fields)), P: prov(o.n)}
	if !r.fields(o, c.Fields, cv, &frame{rec: cv}) {
		return nil
	}
	return cv
}

// isMarker is JSON equality with a field's none marker (WIRE.md §5.4).
func isMarker(n *jsonsrc.Node, marker []byte) bool {
	m := string(marker)
	switch {
	case m == openObject+closeObject:
		return n.Kind == jsonsrc.Object && len(n.Members) == 0
	case m == openArray+closeArray:
		return n.Kind == jsonsrc.Array && len(n.Elems) == 0
	case m == textTrue || m == textFalse:
		return n.Kind == jsonsrc.Bool && n.Text == m
	case strings.HasPrefix(m, quote):
		return n.Kind == jsonsrc.String && n.Text == markerText(marker)
	}
	return n.Kind == jsonsrc.Number && parseDecimal(n.Text) == parseDecimal(m)
}

// markerText is a none marker as a CSV cell writes it: a string's text, else its JSON.
func markerText(marker []byte) string {
	if s, _, err := diag.UnquoteJSON(string(marker)); err == nil && strings.HasPrefix(string(marker), quote) {
		return s
	}
	return string(marker)
}
