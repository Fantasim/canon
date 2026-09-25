package ir

import (
	"fmt"
	"strconv"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
)

// typ appends the `type` production of FINGERPRINT.md §4.2 for t (§4.4).
func (fp *fingerprint) typ(b []byte, t *TypeRef) ([]byte, error) {
	switch t.Kind {
	case types.Enum, types.Record, types.Variant:
		return fp.namedRef(b, t)
	case types.TypeApp:
		return fp.dep(b, t)
	case types.Case:
		return fp.caseRef(b, t)
	case types.Never:
		return append(b, fpNever...), nil
	case types.List, types.Table, types.Optional, types.Map, types.DepMap, types.Ref, types.LitUnion:
		return fp.composite(b, t)
	default:
	}
	name := types.Basic{K: t.Kind, Bits: t.Bits, Signed: t.Signed}.String()
	if name == "" {
		return nil, fmt.Errorf("%w: kind %d, %d bits", ErrFingerprint, t.Kind, t.Bits)
	}
	return append(b, name...), nil
}

// composite appends list(T), keyed(path,T), table(T), opt(T), map(K,V), ref(K) or union(T,"l"…).
func (fp *fingerprint) composite(b []byte, t *TypeRef) ([]byte, error) {
	subs := []*TypeRef{t.Elem}
	switch t.Kind {
	case types.Map, types.DepMap:
		subs = []*TypeRef{t.Key, t.Elem}
	case types.Ref:
		subs = []*TypeRef{t.Key}
	default:
	}
	if t.Kind == types.List && t.KeyedBy != nil {
		b = append(appendPath(append(b, fpKeyed...), t.KeyedBy.WirePath), fpComma...)
	} else {
		b = append(b, fpComposites[t.Kind]...)
	}
	var err error
	for i, sub := range subs {
		if sub == nil {
			return nil, fmt.Errorf("%w: kind %d without its type", ErrFingerprint, t.Kind)
		}
		if i > 0 {
			b = append(b, fpComma...)
		}
		if b, err = fp.typ(b, sub); err != nil {
			return nil, err
		}
	}
	for _, lit := range t.Literals {
		b = diag.AppendJSONString(append(b, fpComma...), lit)
	}
	return append(b, fpClose...), nil
}

// namedRef appends @N, with its parameter bindings <s1,…> when applied (§4.3, §4.4).
func (fp *fingerprint) namedRef(b []byte, t *TypeRef) ([]byte, error) {
	n, ok := fp.num[t.Named]
	if !ok {
		return nil, fmt.Errorf("%w: unnumbered %v", ErrFingerprint, t.Named)
	}
	b = strconv.AppendInt(append(b, fpAt...), int64(n), decimalBase)
	if len(t.Args) == 0 {
		return b, nil
	}
	b = append(b, fpArgsOpen...)
	for i, s := range t.Args {
		if i > 0 {
			b = append(b, fpComma...)
		}
		b = appendSource(b, s)
	}
	return append(b, fpArgsClose...), nil
}

// caseRef appends case(@N,"<case wire tag>"): a case used as a type, N its variant's number (FINGERPRINT.md §4.4, decision 219).
func (fp *fingerprint) caseRef(b []byte, t *TypeRef) ([]byte, error) {
	if t.Case == nil {
		return nil, fmt.Errorf("%w: case type without its case", ErrFingerprint)
	}
	b, err := fp.namedRef(append(b, fpCaseType...), &TypeRef{Kind: types.Variant, Named: t.Named})
	if err != nil {
		return nil, err
	}
	return append(diag.AppendJSONString(append(b, fpComma...), t.Case.Wire), fpClose...), nil
}

// dep appends dep(source,path,"m1"=T1,…), one arm per discriminant member (FINGERPRINT.md §4.4).
func (fp *fingerprint) dep(b []byte, t *TypeRef) ([]byte, error) {
	d, ok := t.Named.(*Dependent)
	if !ok {
		return nil, fmt.Errorf("%w: type application of %T", ErrFingerprint, t.Named)
	}
	wires, err := discWires(d.Disc)
	if err != nil || d.DiscParam < 0 || d.DiscParam >= len(t.Args) || len(wires) != len(d.ByMember) {
		return nil, fmt.Errorf("%w: dependent %s", ErrFingerprint, d.QName())
	}
	b = appendSource(append(b, fpDep...), t.Args[d.DiscParam])
	b = appendPath(append(b, fpComma...), d.DiscPath)
	for i, w := range wires {
		b = append(diag.AppendJSONString(append(b, fpComma...), w), fpEquals...)
		arm := d.ByMember[i]
		switch {
		case arm == NoBranch:
			b = append(b, fpNever...)
		case arm < 0 || arm >= len(d.Branches):
			return nil, fmt.Errorf("%w: arm %d of %s", ErrFingerprint, arm, d.QName())
		default:
			if b, err = fp.typ(b, &d.Branches[arm].Type); err != nil {
				return nil, err
			}
		}
	}
	return append(b, fpClose...), nil
}

// discWires is the wire value of each discriminant member: an enum's, or false and true.
func discWires(disc *TypeRef) ([]string, error) {
	if disc == nil {
		return nil, fmt.Errorf("%w: no discriminant", ErrFingerprint)
	}
	if disc.Kind == types.Bool {
		return []string{strconv.FormatBool(false), strconv.FormatBool(true)}, nil
	}
	enum, ok := disc.Named.(*Enum)
	if !ok {
		return nil, fmt.Errorf("%w: discriminant %T", ErrFingerprint, disc.Named)
	}
	wires := make([]string, len(enum.Members))
	for i, m := range enum.Members {
		wires[i] = m.Wire
	}
	return wires, nil
}

// appendSource appends field<path>, param<i> or key (§4.4).
func appendSource(b []byte, s *Source) []byte {
	switch s.From {
	case types.ArgField:
		return appendPath(append(b, fpSourceField...), s.WirePath)
	case types.ArgParam:
		return strconv.AppendInt(append(b, fpSourceParam...), int64(s.Param), decimalBase)
	default:
		return append(b, fpSourceKey...)
	}
}

// appendPath appends a JSON array of JSON strings with no spaces (§4.1).
func appendPath(b []byte, path []string) []byte {
	b = append(b, fpPathOpen...)
	for i, p := range path {
		if i > 0 {
			b = append(b, fpComma...)
		}
		b = diag.AppendJSONString(b, p)
	}
	return append(b, fpPathClose...)
}
