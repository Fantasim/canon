package ir

import (
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// translated marks what gen/go writes for a translated fn (CODEGEN.md §5.10; CONFORMANCE.md §2.3, §3): rt for the entry and exit checks, a public method's Duration conversions and every checked operation, time for a method's Duration, strconv for an integer in a template, math for a -0.0, and the package of each enum and key it names.
func (u *goImportUse) translated(fn *ExportFn, method bool) {
	for _, p := range fn.Params {
		u.pureSig(p.Type, method)
		u.std[goRT] = u.std[goRT] || p.Type.Kind == types.Float || goChecksWidth(p.Type) || u.bound(p.Range)
	}
	u.pureSig(fn.Result, method)
	f32 := fn.Result.Kind == types.Float && fn.Result.Bits == goFloat32Bits
	u.std[goRT] = u.std[goRT] || goChecksWidth(fn.Result) || f32 || u.bound(fn.ResultRange)
	for _, r := range fn.Reads {
		u.pkgRef(&r.Type)
		u.std[goRT] = u.std[goRT] || method && r.Type.Kind == types.Duration
	}
	u.pexpr(fn.Body)
}

// pureSig marks a parameter or result: its enum's or key's package; a public method's Duration is a time.Duration it converts through rt.
func (u *goImportUse) pureSig(t TypeRef, method bool) {
	u.pkgRef(&t)
	if method && t.Kind == types.Duration {
		u.mark(goTime, goRT)
	}
}

// goChecksWidth reports a sized integer or a Duration, whose range rt.CheckIntWidth checks.
func goChecksWidth(t TypeRef) bool {
	return t.Kind == types.Duration || t.Kind == types.Int && (t.Bits != goInt64Bits || !t.Signed)
}

// bound reports a range refinement, which rt checks; a -0.0 bound is written through math.
func (u *goImportUse) bound(b *types.Bound) bool {
	if b == nil || !b.HasLo && !b.HasHi {
		return false
	}
	u.std[goMath] = u.std[goMath] || b.HasLo && negZero(b.Lo.F) || b.HasHi && negZero(b.Hi.F)
	return true
}

func negZero(f float64) bool { return f == 0 && math.Signbit(f) }

// pkgRef marks the package of another package's named type or table a type names.
func (u *goImportUse) pkgRef(t *TypeRef) {
	if t == nil {
		return
	}
	u.markPkg(t)
	u.pkgRef(t.Elem)
	u.pkgRef(t.Key)
}

// pexpr marks what each node of a translated body writes.
func (u *goImportUse) pexpr(n PExpr) {
	if n == nil {
		return
	}
	u.node(n)
	for _, c := range pexprChildren(n) {
		u.pexpr(c)
	}
}

// node marks what one node writes: a -0.0 or another package's enum or key, a checked operation's rt helper, an integer interpolation's strconv.
func (u *goImportUse) node(n PExpr) {
	switch x := n.(type) {
	case *Lit:
		u.pkgRef(&x.T)
		f, isFloat := x.V.(*value.Float)
		u.std[goMath] = u.std[goMath] || isFloat && negZero(f.V)
	case *Unary:
		u.std[goRT] = u.std[goRT] || x.Op == OpNeg
	case *Binary:
		u.std[goRT] = u.std[goRT] || goCheckedOps[x.Op]
	case *Call:
		u.std[goRT] = u.std[goRT] || !nativeCall(x)
	case *CallFn:
		u.std[goRT] = u.std[goRT] || x.Fn != nil && x.Fn.Kind == FnLookup && x.Fn.Result.Kind == types.Duration
		u.pkgRef(&x.T)
	case *Template:
		u.std[goStrconv] = u.std[goStrconv] || slices.ContainsFunc(x.Parts, func(p TemplatePart) bool {
			return p.X != nil && p.X.Type().Kind == types.Int
		})
	}
}

// nativeCall reports a built-in gen/go writes as Go's own: min and max of integers.
func nativeCall(x *Call) bool {
	return (x.Fn == BuiltinMin || x.Fn == BuiltinMax) && len(x.Args) > 0 && x.Args[0].Type().Kind != types.Float
}

// pexprChildren are a node's operands and statements' expressions, in order.
func pexprChildren(n PExpr) []PExpr {
	switch x := n.(type) {
	case *Unary:
		return []PExpr{x.X}
	case *Binary:
		return []PExpr{x.X, x.Y}
	case *Call:
		return x.Args
	case *CallFn:
		return x.Args
	case *If:
		return []PExpr{x.Cond, x.Then, x.Else}
	case *Let:
		return []PExpr{x.Value, x.Body}
	case *Coalesce:
		return []PExpr{x.X, x.Y}
	case *IsCase:
		return []PExpr{x.X}
	case *Template:
		var out []PExpr
		for _, p := range x.Parts {
			out = append(out, p.X)
		}
		return out
	case *Block:
		return blockExprs(x, nil)
	}
	return nil
}

// blockExprs are the expressions of a block's statements, nested blocks included.
func blockExprs(b *Block, out []PExpr) []PExpr {
	if b == nil {
		return out
	}
	for _, st := range b.Stmts {
		switch s := st.(type) {
		case *LetStmt:
			out = append(out, s.Value)
		case *ReturnStmt:
			out = append(out, s.X)
		case *IfStmt:
			out = blockExprs(s.Else, blockExprs(s.Then, append(out, s.Cond)))
		}
	}
	return out
}
