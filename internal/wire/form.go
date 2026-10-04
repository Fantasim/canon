package wire

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Formless is a part of a field's value with no wire form, which verify reports (E8102, WIRE.md §5.1).
type Formless struct {
	Value   value.Value // the value with no wire form: the finding names it
	Site    value.Value // where the finding points: Value, or a bits list's repeated member
	Steps   []Step      // from the field's value down to Value, none for the value itself
	finding func(span source.Span, v diag.ValueArg, field string) *diag.Builder
}

// Step is one step down a field's value: element Index of the list In, or the entry of Key in the map In.
type Step struct {
	In    value.Value
	Index int
	Key   value.Value // nil for a list element
}

// At is the E8102 finding about x at span, for the field named field.
func (x Formless) At(span source.Span, field string) *diag.Builder {
	return x.finding(span, x.Value, field)
}

// FieldForms lists the parts of field f's value v, records not entered, that Encode refuses (§5.1, §5.3, §5.4).
func FieldForms(f *types.Field, v value.Value) []Formless {
	if v == nil || f.Input != nil {
		return nil
	}
	if _, none := v.(*value.None); none {
		return nil
	}
	var out []Formless
	if l, ok := v.(*value.List); ok && f.Enc == types.EncBits {
		out = repeatedBits(l)
	} else {
		out = fractions(v, f.Unit, nil, nil)
	}
	if len(out) == 0 && f.NoneWire != nil && encodesAsMarker(f, v) {
		out = append(out, Formless{Value: v, Site: v, finding: noneFinding(string(f.NoneWire))})
	}
	return out
}

// fractions appends each Duration of v, at steps, itself or in a list or map value, finer than unit (§4.1, §5.1).
func fractions(v value.Value, unit types.Unit, steps []Step, out []Formless) []Formless {
	switch x := v.(type) {
	case *value.Dur:
		if x.Ms%unit.Millis() != 0 {
			out = append(out, Formless{Value: x, Site: x, Steps: slices.Clone(steps), finding: unitFinding(unit)})
		}
	case *value.List:
		for i, e := range x.Elems {
			out = fractions(e, unit, append(steps, Step{In: x, Index: i}), out)
		}
	case *value.Map:
		for i, e := range x.Vals {
			if i < len(x.Keys) {
				out = fractions(e, unit, append(steps, Step{In: x, Key: x.Keys[i]}), out)
			}
		}
	}
	return out
}

// repeatedBits is l, a bits list, at the second occurrence of each member it holds twice (§5.3).
func repeatedBits(l *value.List) []Formless {
	var out []Formless
	var seen, reported int64
	for _, e := range l.Elems {
		m, ok := e.(*value.Member)
		if !ok || m.Enum == nil || m.Index < 0 || m.Index >= len(m.Enum.Members) {
			continue
		}
		member := m.Enum.Members[m.Index]
		switch {
		case seen&member.Code == 0:
			seen |= member.Code
		case reported&member.Code == 0:
			reported |= member.Code
			out = append(out, Formless{Value: l, Site: m, finding: bitsFinding(member.Name)})
		}
	}
	return out
}

// encodesAsMarker reports v, a present value of f, encoding as f's none marker by JSON equality (§5.4).
func encodesAsMarker(f *types.Field, v value.Value) bool {
	marker := string(f.NoneWire)
	switch x := v.(type) {
	case *value.Record:
		return marker == openObject+closeObject && emptyRecord(x)
	case *value.Map:
		return marker == openObject+closeObject && len(x.Keys) == 0
	case *value.Table:
		return marker == openObject+closeObject && len(x.Entries) == 0
	case *value.List:
		if f.Enc != types.EncBits {
			return marker == openArray+closeArray && len(x.Elems) == 0
		}
		n, err := bits(x)
		return err == nil && sameJSON(n, f.NoneWire)
	}
	n, err := (&encoder{}).value(v, scope{unit: f.Unit, asInt: f.Enc == types.EncInt})
	return err == nil && sameJSON(n, f.NoneWire)
}

// emptyRecord reports a record Encode writes as `{}`: no tag, only inputs and empty pairs, no stored export fn.
func emptyRecord(r *value.Record) bool {
	fields, c, err := shape(r)
	if err != nil || c != nil {
		return false
	}
	for i, f := range fields {
		l, isList := r.Fields[i].(*value.List)
		switch {
		case f.Input != nil:
		case f.Pairs != nil && isList && len(l.Elems) == 0:
		default:
			return false
		}
	}
	return !exportsMethods(r.T)
}

// exportsMethods reports a record type with a stored export fn, which writes a `$` key (§5.11).
func exportsMethods(t types.Type) bool {
	var methods []*types.Method
	switch d := t.Base().(type) {
	case *types.RecordType:
		methods = d.Methods
	case *types.AppliedRecord:
		methods = d.Rec.Methods
	}
	for _, m := range methods {
		if m.Export && Stored(m.Type) {
			return true
		}
	}
	return false
}

func unitFinding(unit types.Unit) func(source.Span, diag.ValueArg, string) *diag.Builder {
	return func(span source.Span, v diag.ValueArg, field string) *diag.Builder {
		return diag.E8102.AtUnit(span, v, field, unit.String())
	}
}

func noneFinding(marker string) func(source.Span, diag.ValueArg, string) *diag.Builder {
	return func(span source.Span, v diag.ValueArg, field string) *diag.Builder {
		return diag.E8102.AtNone(span, v, field, marker)
	}
}

func bitsFinding(member string) func(source.Span, diag.ValueArg, string) *diag.Builder {
	return func(span source.Span, v diag.ValueArg, field string) *diag.Builder {
		return diag.E8102.AtBits(span, v, field, member)
	}
}

// Stored reports an untranslated export fn: its parameters are all finite, Bool, enum or non-keyed ref (§5.11).
func Stored(sig *types.FuncType) bool {
	return sig == nil || !slices.ContainsFunc(sig.Params, infinite)
}

// infinite reports a parameter that is not finite: a ref into a keyed list or a `local let` table has no id enum to index by (CODEGEN.md §5.10, DECISIONS 296).
func infinite(t types.Type) bool {
	if r, ok := t.Base().(*types.RefType); ok {
		return r.Target == nil || r.Target.KeyedBy != nil || r.Target.Local && r.Target.Kind == types.CollLet
	}
	k := t.Base().Kind()
	return k != types.Bool && k != types.Enum
}
