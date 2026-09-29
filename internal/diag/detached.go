package diag

import "slices"

// Detached is a copy of b that holds no argument by reference: each Value, Type and Types
// argument is kept as the text it renders, and each Message argument is detached in turn. The
// copy reports the same finding as b, and keeps no value or type of the program alive.
func (b *Builder) Detached() *Builder {
	c := *b
	c.args = detachedArgs(b.def.Variants[b.variant].Args, b.args)
	c.related = slices.Clone(b.related)
	c.stack = slices.Clone(b.stack)
	c.reads = slices.Clone(b.reads)
	return &c
}

// detachedArgs is args with every argument that references program data replaced by its text.
func detachedArgs(params []Arg, args []any) []any {
	out := slices.Clone(args)
	for i, p := range params {
		if i >= len(out) {
			break
		}
		out[i] = detachedArg(p.Type, out[i])
	}
	return out
}

// detachedArg is one argument as its text, or as it is when it holds only data.
func detachedArg(t ArgType, a any) any {
	switch t {
	case ArgTypeValue:
		v, _ := a.(ValueArg)
		return textValue(valueText(v))
	case ArgTypeType:
		return textType(typeText(a))
	case ArgTypeTypes:
		ts, _ := a.([]TypeArg)
		out := make([]TypeArg, len(ts))
		for i, typ := range ts {
			out[i] = textType(typeText(typ))
		}
		return out
	case ArgTypeMessage:
		if m, ok := a.(Message); ok && m.finding != nil {
			return Message{finding: m.finding.Detached()}
		}
	case ArgTypeName, ArgTypeNames, ArgTypeChain, ArgTypeExpr, ArgTypeInt, ArgTypeRune, ArgTypePath,
		ArgTypeLoc, ArgTypePointer, ArgTypeKind, ArgTypeText: // data already: strings, numbers, spans
	}
	return a
}

// textValue is a Value argument kept as its canonical text.
type textValue string

// CanonText is the text kept.
func (v textValue) CanonText() string { return string(v) }

// textType is a Type argument kept as its canonical text.
type textType string

// String is the text kept.
func (t textType) String() string { return string(t) }
