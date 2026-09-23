package ir

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
)

// block appends `type @N …` and the lines of the named type numbered n (FINGERPRINT.md §4.5).
func (fp *fingerprint) block(b []byte, n int, t Type) ([]byte, error) {
	b = strconv.AppendInt(append(b, fpType...), int64(n), decimalBase)
	switch x := t.(type) {
	case *Record:
		b = append(strconv.AppendInt(append(b, fpRecord...), int64(x.Params), decimalBase), fpNewline...)
		return fp.body(b, fpIndent, x.Fields, x.Methods)
	case *Variant:
		b = append(diag.AppendJSONString(append(b, fpVariant...), x.Tag), fpNewline...)
		var err error
		for _, c := range x.Cases {
			b = append(diag.AppendJSONString(append(b, fpIndent+fpCase...), c.Wire), fpNewline...)
			if b, err = fp.body(b, fpIndent+fpIndent, c.Fields, c.Methods); err != nil {
				return nil, err
			}
		}
		return b, nil
	case *Enum:
		return enumBlock(b, x)
	}
	return nil, fmt.Errorf("%w: named type %T", ErrFingerprint, t)
}

// enumBlock appends an enum's head and one member line per member, retired included.
func enumBlock(b []byte, e *Enum) ([]byte, error) {
	wire, codes := fpWireString, fpNone
	if e.JSONCodes {
		wire = fpWireCode
	}
	if e.Codes != nil {
		codes = types.Basic{K: e.Codes.Kind, Bits: e.Codes.Bits, Signed: e.Codes.Signed}.String()
		if e.Codes.Kind != types.Int || codes == "" {
			return nil, fmt.Errorf("%w: @codes type of %s", ErrFingerprint, e.QName())
		}
	}
	b = append(append(append(append(b, fpEnum...), wire...), fpCodes...), codes+fpNewline...)
	for _, m := range e.Members {
		b = diag.AppendJSONString(append(b, fpIndent+fpMember...), m.Wire)
		b = append(b, fpCode...)
		if e.Codes == nil {
			b = append(b, fpNone...)
		} else {
			b = strconv.AppendInt(b, m.Code, decimalBase)
		}
		b = append(b, fpNewline...)
	}
	return b, nil
}

// body appends the field lines, then the `fn` lines, of a record or case at indent.
func (fp *fingerprint) body(b []byte, indent string, fields []*Field, methods []*ExportFn) ([]byte, error) {
	var err error
	for _, f := range fields {
		if f.Input != nil {
			continue
		}
		if b, err = fp.field(b, indent, f); err != nil {
			return nil, fmt.Errorf("field %s: %w", f.Name, err)
		}
	}
	for _, m := range precomputed(methods) {
		if b, err = fp.fnLine(b, indent, fpDollar+m.Name, m); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// field appends `field <path|inline|pairs> type opt= none= unit= enc=` (§4.5).
func (fp *fingerprint) field(b []byte, indent string, f *Field) ([]byte, error) {
	if int(f.Enc) >= len(fpEncNames) {
		return nil, fmt.Errorf("%w: encoding %d", ErrFingerprint, f.Enc)
	}
	b = append(b, indent+fpField...)
	switch {
	case f.Inline:
		b = append(b, fpInline...)
	case f.Pairs != nil:
		b = diag.AppendJSONString(append(b, fpPairs...), f.Pairs.Keys[0])
		b = diag.AppendJSONString(append(b, fpComma...), f.Pairs.Keys[1])
		b = append(strconv.AppendInt(append(b, fpComma...), int64(f.Pairs.Slots), decimalBase), fpClose...)
	case len(f.WirePath) == 0:
		return nil, fmt.Errorf("%w: no wire path", ErrFingerprint)
	default:
		b = appendPath(b, f.WirePath)
	}
	b, err := fp.typ(append(b, fpSpace...), &f.Type)
	if err != nil {
		return nil, err
	}
	opt, none, unit := fpOptNo, fpNone, fpNone
	if f.Optional {
		opt, none = fpOptYes, fpNull
		if f.NoneWire != nil {
			none = string(f.NoneWire)
		}
	}
	if holdsDuration(&f.Type) {
		unit = f.Unit.String()
	}
	b = append(b, fpOpt+opt+fpNoneKey+none+fpUnit+unit+fpEnc...)
	return append(b, fpEncNames[f.Enc]+fpNewline...), nil
}

// fnLine appends `fn <key> <type>`: the result, under one map level per finite parameter.
func (fp *fingerprint) fnLine(b []byte, indent, key string, fn *ExportFn) ([]byte, error) {
	b = append(diag.AppendJSONString(append(b, indent+fpFn...), key), fpSpace...)
	var err error
	for _, p := range fn.Params {
		if b, err = fp.typ(append(b, fpMap...), &p.Type); err != nil {
			return nil, err
		}
		b = append(b, fpComma...)
	}
	if b, err = fp.typ(b, &fn.Result); err != nil {
		return nil, err
	}
	for range fn.Params {
		b = append(b, fpClose...)
	}
	return append(b, fpNewline...), nil
}

// holdsDuration says whether t holds a Duration outside named types (FINGERPRINT.md §4.5).
func holdsDuration(t *TypeRef) bool {
	switch t.Kind {
	case types.Duration:
		return true
	case types.Enum, types.Record, types.Variant:
		return false
	case types.TypeApp:
		d, ok := t.Named.(*Dependent)
		for i := 0; ok && i < len(d.Branches); i++ {
			if holdsDuration(&d.Branches[i].Type) {
				return true
			}
		}
		return false
	default:
		return t.Elem != nil && holdsDuration(t.Elem)
	}
}
