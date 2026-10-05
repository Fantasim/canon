package types

import (
	"strconv"
	"strings"
)

func (r *Refined) String() string {
	if r.Asset != nil {
		return assetText(r.Asset)
	}
	s := r.Of.String()
	if r.Past {
		s = textPast + s
	}
	if args := refineArgs(r); args != "" {
		if l, ok := r.Of.(*ListType); ok && l.KeyedBy != nil {
			s = textOpenList + l.Elem.String() + textCloseList + textOpen + args + textClose + textKeyedBy + l.KeyedBy.Name
		} else {
			s += textOpen + args + textClose
		}
	}
	if r.Where != nil {
		s += textWhere + r.Where.Text
	}
	return s
}

func (l *ListType) String() string {
	s := textOpenList + l.Elem.String() + textCloseList
	if l.KeyedBy != nil {
		s += textKeyedBy + l.KeyedBy.Name
	}
	return s
}

func (m *MapType) String() string {
	return textOpenMap + m.Key.String() + textColon + m.Value.String() + textCloseMap
}

func (d *DepMapType) String() string {
	return textOpenMap + d.Binder + textIn + d.Coll.String() + textColon + d.Value.String() + textCloseMap
}

func (t *TableType) String() string {
	if t.Stable {
		return textStable + textTable + t.Elem.String()
	}
	return textTable + t.Elem.String()
}

// String prints a ref to a named collection by the collection, and a ref resolved in an
// enclosing record by its element type, as it is written.
func (r *RefType) String() string {
	if r.Target.Kind == CollField {
		return textRef + r.Target.Elem.String()
	}
	return textRef + r.Target.String()
}

// String is the collection's qualified path: the let, or the enclosing record, then fields.
func (c *Collection) String() string {
	root := qualify(c.Pkg, c.Name)
	if c.Kind == CollField {
		root = c.Owner.String()
	}
	return strings.Join(append([]string{root}, c.FieldPath...), textDot)
}

func (o *OptionalType) String() string {
	switch e := o.Elem.(type) {
	case *FuncType, *LitUnionType:
		return textOpen + e.String() + textClose + textOptional
	case *Refined:
		if e.Where != nil {
			return textOpen + e.String() + textClose + textOptional
		}
	}
	return o.Elem.String() + textOptional
}

func (u *LitUnionType) String() string {
	parts := []string{u.Of.String()}
	for _, l := range u.Literals {
		parts = append(parts, QuoteString(l))
	}
	return strings.Join(parts, textUnion)
}

func (f *FuncType) String() string {
	return textFn + typeList(f.Params) + textArrow + f.Result.String()
}

func (p *PairType) String() string {
	return textPair + typeList([]Type{p.A, p.B}) + textClose
}

func (a *AppliedRecord) String() string {
	return a.Rec.String() + textOpen + argList(a.Args) + textClose
}

func (t *TypeAppType) String() string {
	return qualify(t.Fn.Pkg, t.Fn.Name) + textOpen + argList(t.Args) + textClose
}

func (d *DepUnionType) String() string { return qualify(d.Fn.Pkg, d.Fn.Name) + textDepAll }

func typeList(ts []Type) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = t.String()
	}
	return strings.Join(parts, textSep)
}

func argList(args []*Arg) string {
	parts := make([]string, len(args))
	for i, a := range args {
		var names []string
		switch a.Source {
		case ArgParam:
			names = append(names, a.Param.Name)
		case ArgKey:
			names = append(names, a.Binder)
		default:
		}
		for _, f := range a.Path {
			names = append(names, f.Name)
		}
		parts[i] = strings.Join(names, textDot)
	}
	return strings.Join(parts, textSep)
}

func refineArgs(r *Refined) string {
	var parts []string
	if r.Range != nil {
		parts = append(parts, boundText(r.Range, r.Of.Base().Kind()))
	}
	if r.Pattern != nil {
		parts = append(parts, textSlash+r.Pattern.String()+textSlash)
	}
	return strings.Join(parts, textSep)
}

func boundText(b *Bound, base Kind) string {
	s := ""
	if b.HasLo {
		s = limitText(b.Lo, base)
	}
	if !b.HasHi {
		return s + textRange
	}
	if b.HiIncluded {
		return s + textRangeIncl + limitText(b.Hi, base)
	}
	return s + textRange + limitText(b.Hi, base)
}

func limitText(l Limit, base Kind) string {
	switch base {
	case Float:
		return FloatText(l.F, bitsDefault)
	case Duration:
		return DurationText(l.I)
	default:
		return strconv.FormatInt(l.I, 10)
	}
}

func assetText(a *AssetSpec) string {
	s := textAsset + QuoteString(a.Root)
	if len(a.Exts) > 0 {
		exts := make([]string, len(a.Exts))
		for i, e := range a.Exts {
			exts[i] = e
			if !reWord.MatchString(e) {
				exts[i] = QuoteString(e)
			}
		}
		s += textExt + strings.Join(exts, textSep) + textCloseList
	}
	return s + textClose
}
