package types

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/syntax"
)

// Kind is one kind of TYPES.md §2, which is authoritative.
type Kind uint8

// Type is a Canon type. Implementations are the pointer types of this package, Basic and the
// kind singletons; the checker interns named types, so a declaration is one pointer.
type Type interface {
	Kind() Kind
	String() string   // canonical type text, qualified names (API.md TypeInfo.Expr)
	Underlying() Type // alias layers at the top removed; refinements kept
	Base() Type       // alias and refinement layers at the top removed (TYP-04)
}

// Basic is a scalar type: Bool, String, Duration, or an Int or Float of a width (TYP-03).
type Basic struct {
	K      Kind
	Bits   int
	Signed bool
}

func (b Basic) Kind() Kind { return b.K }

func (b Basic) String() string { return basicNames[b] }

func (b Basic) Underlying() Type { return b }

func (b Basic) Base() Type { return b }

// Limits is the implicit range of an integer type or of a stored Duration (TYPES.md §7.2).
func (b Basic) Limits() (lo, hi int64, ok bool) {
	switch {
	case b.K == Duration:
		return -DurationLimit, DurationLimit, true
	case b.K != Int:
		return 0, 0, false
	case b.Bits == bitsDefault:
		if b.Signed {
			return -maxInt - 1, maxInt, true
		}
		return 0, maxInt, true
	case b.Signed:
		return -(1 << (b.Bits - 1)), 1<<(b.Bits-1) - 1, true
	}
	return 0, 1<<b.Bits - 1, true
}

// builtin is a kind that carries nothing: Any, None, Error, Never, Range.
type builtin struct {
	k    Kind
	text string
}

func (b builtin) Kind() Kind { return b.k }

func (b builtin) String() string { return b.text }

func (b builtin) Underlying() Type { return b }

func (b builtin) Base() Type { return b }

// Refined is Of with refinements; it never changes static identity, so its Kind is Of's.
// A Refined holds one written refinement; a refined named type nests them (TYP-04).
type Refined struct {
	Of      Type
	Range   *Bound // a value range, or the length of a string, list or map
	Pattern *regexp.Regexp
	Where   *Predicate
	Asset   *AssetSpec // TYPES.md §13.4
}

func (r *Refined) Kind() Kind { return r.Of.Kind() }

func (r *Refined) Underlying() Type { return r }

func (r *Refined) Base() Type { return r.Of.Base() }

// Bound is a range refinement: lo..hi, lo..=hi, lo.., ..=hi or ..hi.
type Bound struct {
	Lo, Hi       Limit
	HasLo, HasHi bool
	HiIncluded   bool
}

// Limit is one end of a Bound: F for a Float base, I otherwise (ms for a Duration).
type Limit struct {
	I int64
	F float64
}

// Predicate is a `where` refinement: Expr runs with `it` bound; Text prints it in type text.
type Predicate struct {
	Expr syntax.Expr
	Text string
}

// AssetSpec is the argument list of asset(root, ext: […]).
type AssetSpec struct {
	Root string
	Exts []string
}

// Alias is a declared name for a type (TYPES.md §13.1); it is the same type as Def.
type Alias struct {
	Pkg, Name, Doc string
	Def            Type
	Decl           *syntax.TypeDecl
}

func (a *Alias) Kind() Kind { return a.Def.Kind() }

func (a *Alias) String() string { return qualify(a.Pkg, a.Name) }

func (a *Alias) Underlying() Type { return a.Def.Underlying() }

func (a *Alias) Base() Type { return a.Def.Base() }

// Unit is the wire unit of a Duration (WIRE.md §5.1).
type Unit uint8

// String is the unit's symbol: ms, s, m, h or d.
func (u Unit) String() string { return unitNames[u] }

// Millis is the number of milliseconds in one unit.
func (u Unit) Millis() int64 { return unitMillis[u] }

// Enc is the wire encoding a field gives its Bool or enum list: @json(int) or @json(bits).
type Enc uint8

func qualify(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + textDot + name
}
