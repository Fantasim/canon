package check

import (
	"context"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Folder folds a constant expression (TYPES.md §15) in eval's evaluator (IMPLEMENTATION-PLAN §4.7).
type Folder interface {
	Fold(ctx context.Context, owner Object, e syntax.Expr, info *Info) (value.Value, bool) // owner's bag and file take the findings
}

// Program is the checked program, packages dependencies first (TYPES.md §3.1).
type Program struct {
	Packages []*Package
	Info     *Info
}

// Package is a checked package; Decls in (file path, position) order (EVALUATION.md §2.1).
type Package struct {
	Path    string
	Files   []*syntax.File
	Decls   []Object
	Imports []*Package
	Layers  map[string][]*syntax.AmendBlock
}

// Info is every conclusion of the checker, recorded once by node (IMPLEMENTATION-PLAN §4.7).
type Info struct {
	Types       map[syntax.Expr]types.Type // after narrowing, before Conv
	TypeExprs   map[syntax.Type]types.Type
	Defs        map[*syntax.Ident]Object
	Uses        map[*syntax.IdentExpr]Object
	NameUses    map[*syntax.Ident]Object // every other identifier naming an object, `.name` included
	Selections  map[*syntax.SelectorExpr]*Selection
	Conv        map[syntax.Expr]*Conversion
	Keys        map[syntax.Expr]*types.Collection
	Symbols     map[*syntax.IdentExpr]bool // identifiers kept as symbols, not names
	Calls       map[*syntax.CallExpr]*Callee
	Literals    map[*syntax.BraceLit]LitKind
	Matches     map[syntax.Node]*MatchInfo
	Broken      map[Object]bool
	BrokenViews map[*syntax.ViewDecl]bool // a view holding an error (VIEWMODEL.md J4, ADR-0009)

	BrokenTranslations map[*syntax.TranslationEntry]bool // a translation entry holding an error (I18N.md T2, ADR-0009)
}

// ObjectOf is the object an *Ident declares or names, or an *IdentExpr names; else nil.
func (i *Info) ObjectOf(n syntax.Node) Object {
	switch n := n.(type) {
	case *syntax.Ident:
		if obj, ok := i.Defs[n]; ok {
			return obj
		}
		return i.NameUses[n]
	case *syntax.IdentExpr:
		return i.Uses[n]
	}
	return nil
}

// Object is a declared or built-in name, one pointer per declaration; a built-in has no Pkg,
// Decl or File.
type Object interface {
	Kind() ObjKind
	Name() string
	Pkg() string
	Type() types.Type // types.ErrorType when broken; nil for a package, layer, check or test
	Decl() syntax.Node
	File() *syntax.File
}

// Selection is one `.name` on a value, Deref through a ref; Obj is nil for an entry of a
// collection whose keys are dynamic.
type Selection struct {
	Kind  SelKind
	Obj   Object
	Recv  types.Type
	Deref bool
}

// Conversion converts From to To; Inner applies to elements, map values, a pair's second and a
// present value, Key to map keys and a pair's first; nil leaves them unchanged.
type Conversion struct {
	Kind     ConvKind
	From, To types.Type
	Inner    *Conversion
	Key      *Conversion
}

// Callee is a call's callee: Obj, or a built-in with its signature row and type arguments.
type Callee struct {
	Kind     CalleeKind
	Obj      Object
	Builtin  string
	Overload int
	TypeArgs []types.Type
}

// MatchInfo is a checked match: the indexes each arm covers, and the arms reported W3601 or E3602.
type MatchInfo struct {
	Scrutinee   types.Type
	Covers      [][]int
	Exhaustive  bool
	Unreachable []int
}
