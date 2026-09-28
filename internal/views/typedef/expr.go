package typedef

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// at is where a type expression is written: the record or case declaring it (a ref into an
// enclosing record, J12) and its field's @json(bits) (C32).
type at struct {
	decl types.Type
	enc  types.Enc
}

// exprs write a type expression by the kind of the type, aliases expanded (J11).
var exprs map[types.Kind]func(*Types, at, types.Type) vm.TypeExpr

func init() {
	exprs = map[types.Kind]func(*Types, at, types.Type) vm.TypeExpr{
		types.Bool:     func(*Types, at, types.Type) vm.TypeExpr { return vm.TypeExpr{Kind: exprBool} },
		types.Int:      func(_ *Types, _ at, t types.Type) vm.TypeExpr { return intExpr(t) },
		types.Float:    func(_ *Types, _ at, t types.Type) vm.TypeExpr { return floatExpr(t) },
		types.String:   func(_ *Types, _ at, t types.Type) vm.TypeExpr { return stringExpr(t) },
		types.Duration: func(_ *Types, _ at, t types.Type) vm.TypeExpr { return bounded(exprDuration, t) },
		types.Enum:     func(_ *Types, _ at, t types.Type) vm.TypeExpr { return named(defEnum, t) },
		types.Record:   (*Types).recordExpr,
		types.Variant:  func(_ *Types, _ at, t types.Type) vm.TypeExpr { return named(defVariant, t) },
		types.Case:     func(_ *Types, _ at, t types.Type) vm.TypeExpr { return caseExpr(t) },
		types.Optional: (*Types).optionalExpr,
		types.List:     (*Types).listExpr,
		types.Table:    (*Types).tableExpr,
		types.Map:      (*Types).mapExpr,
		types.DepMap:   (*Types).depMapExpr,
		types.Ref:      (*Types).refExpr,
		types.LitUnion: (*Types).unionExpr,
		types.Never:    func(*Types, at, types.Type) vm.TypeExpr { return vm.TypeExpr{Kind: exprNever} },
		types.TypeApp:  (*Types).appExpr,
	}
}

// Expr is t's type expression (VIEWMODEL.md 12.3); decl is the record or case whose field or method
// has it, nil for none. A type without a row (`_`, a function) is `any`.
func (s *Types) Expr(decl, t types.Type) vm.TypeExpr { return s.expr(at{decl: decl}, t) }

// expr is t's type expression with the `where` of its outermost layer as `predicate`.
func (s *Types) expr(c at, t types.Type) vm.TypeExpr {
	e := vm.TypeExpr{Kind: exprAny}
	if write, ok := exprs[t.Base().Kind()]; ok {
		e = write(s, c, t)
	}
	if w := shape.LayersOf(t).Where; len(w) > 0 {
		text := w[0].Text
		e.Predicate = &text
	}
	return e
}

// inner is the context of what c's type holds: the field's @json(bits) is its own.
func inner(c at) at { return at{decl: c.decl} }

func intExpr(t types.Type) vm.TypeExpr {
	b := t.Base().(types.Basic)
	e := bounded(exprInt, t)
	e.Bits, e.Signed = b.Bits, &b.Signed
	return e
}

func floatExpr(t types.Type) vm.TypeExpr {
	e := bounded(exprFloat, t)
	e.Bits = t.Base().(types.Basic).Bits
	return e
}

// bounded is a number expression with its bounds (§12.3 int, float, duration).
func bounded(kind string, t types.Type) vm.TypeExpr {
	b := encode.NumberBounds(t)
	return vm.TypeExpr{Kind: kind, Min: b.Min, Max: b.Max, MinExclusive: b.MinExclusive, MaxExclusive: b.MaxExclusive}
}

// stringExpr is a string with its byte lengths and pattern, or an asset (§12.3).
func stringExpr(t types.Type) vm.TypeExpr {
	if a := shape.LayersOf(t).Asset; a != nil {
		return vm.TypeExpr{Kind: exprAsset, Root: a.Root, Ext: a.Exts}
	}
	lo, hi := encode.Lengths(t)
	return vm.TypeExpr{Kind: exprString, MinLen: lo, MaxLen: hi, Pattern: encode.Pattern(t)}
}

// named is an enum, a record or a variant by qualified name.
func named(kind string, t types.Type) vm.TypeExpr {
	return vm.TypeExpr{Kind: kind, Ref: encode.Name(t)}
}

// caseExpr is a variant case used as a type: its variant and the fixed case (12.3 `variant`).
func caseExpr(t types.Type) vm.TypeExpr {
	e := named(defVariant, t)
	e.Case = t.Base().(*types.CaseType).Name
	return e
}

// recordExpr is a record, with `bind` mapping each parameter of an applied record to its
// driver (J13).
func (s *Types) recordExpr(_ at, t types.Type) vm.TypeExpr {
	e := named(defRecord, t)
	if a, ok := t.Base().(*types.AppliedRecord); ok && len(a.Args) > 0 {
		e.Bind = map[string]vm.Driver{}
		for i, p := range a.Rec.Params {
			if i < len(a.Args) {
				e.Bind[p.Name] = *encode.Driver(a.Args[i])
			}
		}
	}
	return e
}

func (s *Types) optionalExpr(c at, t types.Type) vm.TypeExpr {
	of := s.expr(c, t.Base().(*types.OptionalType).Elem)
	return vm.TypeExpr{Kind: exprOptional, Of: &of}
}

// listExpr is a list with its length bounds, key field and whether it is a set (C32).
func (s *Types) listExpr(c at, t types.Type) vm.TypeExpr {
	l := t.Base().(*types.ListType)
	of := s.expr(inner(c), l.Elem)
	e := lengths(vm.TypeExpr{Kind: exprList, Of: &of, Unique: control.IsSet(t, c.enc)}, t)
	if l.KeyedBy != nil {
		e.KeyedBy = l.KeyedBy.Name
	}
	return e
}

// lengths adds a list's or map's length bounds.
func lengths(e vm.TypeExpr, t types.Type) vm.TypeExpr {
	lo, hi := encode.Lengths(t)
	if lo != nil {
		e.Min = vm.Int(int64(*lo))
	}
	if hi != nil {
		e.Max = vm.Int(int64(*hi))
	}
	return e
}

func (s *Types) tableExpr(c at, t types.Type) vm.TypeExpr {
	tt := t.Base().(*types.TableType)
	of := s.expr(inner(c), tt.Elem)
	return vm.TypeExpr{Kind: exprTable, Of: &of, Stable: tt.Stable}
}

func (s *Types) mapExpr(c at, t types.Type) vm.TypeExpr {
	m := t.Base().(*types.MapType)
	k, v := s.expr(inner(c), m.Key), s.expr(inner(c), m.Value)
	return lengths(vm.TypeExpr{Kind: exprMap, Key: &k, Value: &v}, t)
}

// depMapExpr is a dependent map `{k in c: T(k)}`: keyed by refs into c (§12.3 `dependent`).
func (s *Types) depMapExpr(c at, t types.Type) vm.TypeExpr {
	d := t.Base().(*types.DepMapType)
	k, v := s.expr(inner(c), &types.RefType{Target: d.Coll}), s.expr(inner(c), d.Value)
	return lengths(vm.TypeExpr{Kind: exprMap, Key: &k, Value: &v, Dependent: true}, t)
}

// refExpr is a ref: its target's value id, or its enclosing-record field (J12), its element,
// key type, and entry counts in this build.
func (s *Types) refExpr(c at, t types.Type) vm.TypeExpr {
	coll := t.Base().(*types.RefType).Target
	count, active := s.in.Colls.Counts(coll)
	e := vm.TypeExpr{Kind: exprRef, Element: encode.Element(coll), KeyType: encode.KeyType(coll), Count: &count, Active: &active}
	if s, perInstance := encode.Sibling(c.decl, coll); perInstance {
		e.Sibling = s
	} else {
		e.Collection = encode.CollectionID(coll)
	}
	return e
}

func (s *Types) unionExpr(c at, t types.Type) vm.TypeExpr {
	u := t.Base().(*types.LitUnionType)
	of := s.expr(c, u.Of)
	return vm.TypeExpr{Kind: exprUnion, Of: &of, Literals: u.Literals}
}

// appExpr is an application of a `match`-bodied type function and its driver; any other
// type function's application is its expansion (J11).
func (s *Types) appExpr(c at, t types.Type) vm.TypeExpr {
	app := t.Base().(*types.TypeAppType)
	if !control.Matches(app.Fn) {
		if app.Fn.Body == nil {
			return vm.TypeExpr{Kind: exprAny}
		}
		return s.expr(c, shape.Expand(app))
	}
	return vm.TypeExpr{Kind: exprDependent, Fn: encode.FuncName(app.Fn), On: encode.Driver(control.Scrutinized(app))}
}
