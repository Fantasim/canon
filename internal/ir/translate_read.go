package ir

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// readCtx is where a path of self is read: as a value, left of `??`, or left of `is`.
type readCtx uint8

// pendingRead is a read of self before its index is known: key orders it, the fields' indexes
// in declaration order then, for a precomputed method, the fields' count plus its index; at is
// where it is first read.
type pendingRead struct {
	read *Read
	key  []int
	elem types.Type
	at   syntax.Expr
}

// readUse is a ReadRef waiting for its read's index.
type readUse struct {
	ref  *ReadRef
	read *pendingRead
}

// selfRead is e as a read of self: fields through records, or a precomputed method of self read like a field (CONFORMANCE.md §2.2, meta/decisions/log-2026-09-24.md); isRead is false for anything else.
func (t *translator) selfRead(e syntax.Expr, ctx readCtx) (PExpr, bool) {
	var pr *pendingRead
	switch x := e.(type) {
	case *syntax.IdentExpr, *syntax.SelectorExpr:
		names, ok := t.selfPath(x)
		if !ok {
			return nil, false
		}
		pr = t.fieldRead(e, names)
	case *syntax.CallExpr:
		site, ok := t.selfMethod(x)
		if !ok {
			return nil, false
		}
		pr = t.methodRead(site)
	default:
		return nil, false
	}
	if pr == nil {
		return nil, true
	}
	return t.use(e, pr, ctx), true
}

// selfPath is the field names of a chain of `.f` and `?.f` rooted at self or at a bare field
// name in a method; false for any other chain.
func (t *translator) selfPath(e syntax.Expr) ([]string, bool) {
	var names []string
	for {
		switch x := e.(type) {
		case *syntax.SelfExpr:
			slices.Reverse(names)
			return names, len(names) > 0
		case *syntax.IdentExpr:
			o := t.s.info.Uses[x]
			if o == nil || o.Kind() != check.ObjField {
				return nil, false
			}
			names = append(names, x.Name)
			slices.Reverse(names)
			return names, true
		case *syntax.SelectorExpr:
			sel := t.s.info.Selections[x]
			if x.X == nil || x.Name == nil || sel == nil || sel.Kind != check.SelField || sel.Deref {
				return nil, false
			}
			names = append(names, x.Name.Name)
			e = syntax.Unparen(x.X)
		default:
			return nil, false
		}
	}
}

// fieldRead resolves a field path from self's type: every step but the last must hold a record
// (a path through a ref, a variant or a collection is outside the subset); nil when refused.
func (t *translator) fieldRead(e syntax.Expr, names []string) *pendingRead {
	pr := &pendingRead{read: &Read{Path: names}}
	fields := ownFields(t.site.recv)
	var f *types.Field
	for i, name := range names {
		if f = fieldNamed(fields, name); f == nil {
			t.refuse(e)
			return nil
		}
		pr.key = append(pr.key, f.Index)
		elem := f.Type
		if o, ok := elem.Base().(*types.OptionalType); ok {
			elem, pr.read.Optional = o.Elem, true
		}
		pr.elem = elem
		if i+1 < len(names) {
			fields = recordFields(elem)
		}
	}
	if f.Input != nil {
		t.broken = true // E3313
		return nil
	}
	if hasFlag(annotation(f.Annotations, syntax.AnnTS), syntax.ArgBigint) {
		t.badRead(e, pr) // CONFORMANCE.md §4
		return nil
	}
	return pr
}

// ownFields are the fields of a record or case type, nil for any other.
func ownFields(ty types.Type) []*types.Field {
	switch x := ty.Base().(type) {
	case *types.CaseType:
		return x.Fields
	default:
		return recordFields(ty)
	}
}

// recordFields are the fields of a record type, nil for any other (a path does not go through a case).
func recordFields(ty types.Type) []*types.Field {
	switch x := ty.Base().(type) {
	case *types.RecordType:
		return x.Fields
	case *types.AppliedRecord:
		return x.Rec.Fields
	}
	return nil
}

func fieldNamed(fields []*types.Field, name string) *types.Field {
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// selfMethod is the export method a call of self names, when it is precomputed and takes no
// argument; any other method call is not a read.
func (t *translator) selfMethod(x *syntax.CallExpr) (*fnSite, bool) {
	c := t.s.info.Calls[x]
	if c == nil || c.Kind != check.CalleeMethod || len(x.Args) != 0 || !onSelf(x) {
		return nil, false
	}
	site := t.s.fnByObj[c.Obj]
	if site == nil || site.recv != t.site.recv || site.fn.Kind != FnPrecomputed || len(site.fn.Params) != 0 {
		return nil, false
	}
	return site, true
}

// onSelf reports a method call on self: bare `m()` in a body, or `self.m()`.
func onSelf(x *syntax.CallExpr) bool {
	switch f := x.Fun.(type) {
	case *syntax.IdentExpr:
		return true
	case *syntax.SelectorExpr:
		_, self := syntax.Unparen(f.X).(*syntax.SelfExpr)
		return self
	}
	return false
}

// methodRead is the read of a precomputed method of self, keyed after every field.
func (t *translator) methodRead(site *fnSite) *pendingRead {
	pr := &pendingRead{read: &Read{Path: []string{site.fn.Name}}, elem: site.sig.Result}
	if o, ok := pr.elem.Base().(*types.OptionalType); ok {
		pr.elem, pr.read.Optional = o.Elem, true
	}
	pr.key = []int{len(ownFields(t.site.recv)) + methodIndex(t.site.recv, site.fn.Name)}
	return pr
}

// methodIndex is the index of the method name among a record's or case's methods.
func methodIndex(ty types.Type, name string) int {
	var ms []*types.Method
	switch x := ty.Base().(type) {
	case *types.RecordType:
		ms = x.Methods
	case *types.CaseType:
		ms = x.Methods
	}
	return slices.IndexFunc(ms, func(m *types.Method) bool { return m.Name == name })
}

// use records a read where CONFORMANCE.md §2.2 allows it: optional left of `??`, variant left of `is`, else a scalar; a ref, a pure-function parameter with no candidates (§2.3, §6.2), is E9006.
func (t *translator) use(e syntax.Expr, pr *pendingRead, ctx readCtx) PExpr {
	k := pr.elem.Base().Kind()
	switch {
	case k == types.Ref:
		t.badRead(e, pr)
		return nil
	case pr.read.Optional != (ctx == ctxCoalesce), (k == types.Variant) != (ctx == ctxIs):
		t.refuse(e)
		return nil
	case k != types.Variant && !scalarKinds[k]:
		t.refuse(e)
		return nil
	}
	pr = t.register(pr, e)
	ref := &ReadRef{T: pr.read.Type, Index: -1}
	t.uses = append(t.uses, readUse{ref: ref, read: pr})
	return ref
}

// badRead is E9006 on a read the pure function cannot take as a parameter.
func (t *translator) badRead(e syntax.Expr, pr *pendingRead) {
	name, ty := strings.Join(pr.read.Path, underscore), pr.elem
	t.report(t.file.Span(e), func(sp source.Span) *diag.Builder { return diag.E9006.At(sp, name, t.site.label, ty) })
}

// register is the one pending read of a path, the first met kept with where it was met.
func (t *translator) register(pr *pendingRead, at syntax.Expr) *pendingRead {
	id := strings.Join(pr.read.Path, qnameSep)
	if known := t.reads[id]; known != nil {
		return known
	}
	pr.read.Type, pr.at = t.s.ref(pr.elem), at
	t.reads[id] = pr
	t.order = append(t.order, pr)
	return pr
}

// nameReads orders the reads by declaration, depth first, and names them (CONFORMANCE.md §2.3); a name another read, a parameter or a `let` local already holds is refused, false then.
func (t *translator) nameReads() bool {
	slices.SortStableFunc(t.order, func(a, b *pendingRead) int { return slices.Compare(a.key, b.key) })
	params, taken := map[string]bool{}, t.locals()
	for _, p := range t.site.fn.Params {
		params[p.Name], taken[p.Name] = true, true
	}
	for _, pr := range t.order {
		pr.read.Name = strings.Join(pr.read.Path, underscore)
		if params[pr.read.Name] {
			pr.read.Name = selfPrefix + pr.read.Name
		}
		if taken[pr.read.Name] {
			t.refuse(pr.at)
		}
		taken[pr.read.Name] = true
	}
	return !t.refused
}

// locals are the names the body's `let` statements bind.
func (t *translator) locals() map[string]bool {
	out := map[string]bool{}
	syntax.Inspect(t.site.decl.Body, func(n syntax.Node) bool {
		if l, ok := n.(*syntax.LetStmt); ok && l.Name != nil {
			out[l.Name.Name] = true
		}
		return true
	})
	return out
}

// finishReads gives every ReadRef its read's index, the reads named and ordered.
func (t *translator) finishReads() []*Read {
	out := make([]*Read, len(t.order))
	index := map[*pendingRead]int{}
	for i, pr := range t.order {
		out[i] = pr.read
		index[pr] = i
	}
	for _, u := range t.uses {
		u.ref.Index = index[u.read]
	}
	return out
}
