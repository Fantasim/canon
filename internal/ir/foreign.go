package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// ForeignUse is what one emit builds of other packages' types through their make hooks (CODEGEN.md §2.8, §5.14; DECISIONS 323): Read are the classes its readers read (a data loader's values, every class of a types-mode emit), Written those its literals write (baked values, constants, stored fns' results, translated bodies' literals). Each is a *Record, a *Variant, a *Case (a variant's case with fields, or the type of a field) or a *Dependent, in first-reach order. Tables are the tables those classes hold, whose rows the emit builds through their holder's entry or row hook.
type ForeignUse struct {
	Read, Written []any
	Tables        []TableUse
	variantOf     map[*Case]*Variant
}

// TableUse is a table of Record held by a class of package Holder, another package than the emit's: each row is built by Holder's entry hook when Record is Holder's own (EntryHook), by Holder's row hook when Record is a third package's (RowHook) (CODEGEN.md §5.9, §5.14; log-2026-10-06 "U1 review" 1).
type TableUse struct {
	Holder string
	Record *Record
}

// Entry reports a table of the holder's own records, built through the holder's entry hook.
func (t TableUse) Entry() bool { return t.Record.Pkg == t.Holder }

// VariantOf is the variant of a case the use holds, nil for another case; every case Read, Written and Built return has one.
func (u *ForeignUse) VariantOf(c *Case) *Variant { return u.variantOf[c] }

// Built are Read then Written, each class once: every class the emit calls a make hook of.
func (u *ForeignUse) Built() []any {
	out := slices.Clone(u.Read)
	for _, c := range u.Written {
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}

// ForeignUses is what emit e of p builds of other packages' types (CODEGEN.md §2.8): its readers start from the values a data or embedded emit loads, or from every class and dependent type of a types-mode emit; its literals from a baked emit's values and package fns, from the constants of every mode and from the literals of its translated bodies (foreignRoots).
func ForeignUses(p *Package, e *Emit) *ForeignUse {
	u := &ForeignUse{variantOf: map[*Case]*Variant{}}
	read, written := newForeignWalk(p.Name, u), newForeignWalk(p.Name, u)
	for _, r := range foreignRoots(p, e) {
		if r.read {
			read.root(r)
		} else {
			written.root(r)
		}
	}
	u.Read, u.Written = read.out, written.out
	return u
}

// foreignRoot is one place where an emit starts building values (ForeignUses), and what stage E's findings on the other packages' classes it reaches point at: item is a *Value, *Const, *ExportFn, *Field or *Dependent of the emit's package; read reports a reader (else a literal); body is a translated fn's body, whose literals are written; dep a dependent type a types-mode emit decodes; owners the own classes a types-mode root belongs to, entered already.
type foreignRoot struct {
	item   any
	t      TypeRef
	read   bool
	body   PExpr
	dep    *Dependent
	owners []any
}

// foreignRoots are emit e's roots in ForeignUses' order: the selected values (written in baked mode, read otherwise), a types-mode emit's classes, field by field and stored fn by stored fn, then its dependent types; the constants, a baked emit's stored package fns' results, the translated bodies.
func foreignRoots(p *Package, e *Emit) []foreignRoot {
	var out []foreignRoot
	for _, v := range p.Values {
		if emitSelects(e, v.Name) {
			out = append(out, foreignRoot{item: v, t: v.Type, read: e.Mode != ModeBaked})
		}
	}
	if e.Mode == ModeTypes {
		out = append(out, typesRoots(p)...)
	}
	for _, c := range p.Consts {
		out = append(out, foreignRoot{item: c, t: c.Type})
	}
	for _, fn := range p.Fns {
		if e.Mode == ModeBaked && fn.Kind != FnTranslated {
			out = append(out, foreignRoot{item: fn, t: fn.Result})
		}
	}
	for _, fn := range translatedFns(p) {
		out = append(out, foreignRoot{item: fn, body: fn.Body})
	}
	return out
}

// typesRoots are what a types-mode emit decodes, in declaration order: each record's stored fields and stored fns' results, each case's of each variant, then each dependent type.
func typesRoots(p *Package) []foreignRoot {
	var out []foreignRoot
	for _, t := range p.Types {
		switch x := t.(type) {
		case *Record:
			out = append(out, bodyRoots(x, []any{x})...)
		case *Variant:
			for _, c := range x.Cases {
				out = append(out, bodyRoots(c, []any{x, c})...)
			}
		}
	}
	for _, t := range p.Types {
		if d, ok := t.(*Dependent); ok {
			out = append(out, foreignRoot{item: d, read: true, dep: d})
		}
	}
	return out
}

// bodyRoots are a class's stored fields (no input, no Never?) and its stored fns' results, read.
func bodyRoots(class any, owners []any) []foreignRoot {
	var out []foreignRoot
	fields, fns := classBody(class)
	for _, f := range fields {
		if f.Input == nil && (!f.Optional || f.Type.Kind != types.Never) {
			out = append(out, foreignRoot{item: f, t: f.Type, read: true, owners: owners})
		}
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			out = append(out, foreignRoot{item: fn, t: fn.Result, read: true, owners: owners})
		}
	}
	return out
}

// emitSelects reports an emit that writes value name: a mode that holds values, and its `values` or every one (CODEGEN.md §2.2).
func emitSelects(e *Emit, name string) bool {
	return e.Mode != ModeTypes && (len(e.Values) == 0 || slices.Contains(e.Values, name))
}

// translatedFns are p's translated fns, package-level then each class's methods, in declaration order.
func translatedFns(p *Package) []*ExportFn {
	var out []*ExportFn
	add := func(fns []*ExportFn) {
		for _, fn := range fns {
			if fn.Kind == FnTranslated {
				out = append(out, fn)
			}
		}
	}
	add(p.Fns)
	for _, class := range packageClasses(p) {
		_, fns := classBody(class)
		add(fns)
	}
	return out
}

// foreignWalk follows what a class holds by value, through lists, optionals, maps and tables, own classes and other packages' alike, keeping the other packages' classes; holder is the package of the class being entered.
type foreignWalk struct {
	own, holder string
	use         *ForeignUse
	seen        map[any]bool
	out         []any
}

func newForeignWalk(own string, u *ForeignUse) *foreignWalk {
	return &foreignWalk{own: own, holder: own, use: u, seen: map[any]bool{}}
}

// root walks one root: a translated body's literals, a dependent type, or a type, its own owners entered first (a types-mode emit enters each class once).
func (w *foreignWalk) root(r foreignRoot) {
	for _, o := range r.owners {
		w.seen[o] = true
	}
	switch {
	case r.body != nil:
		w.literals(r.body)
	case r.dep != nil:
		w.dependent(r.dep)
	default:
		w.typ(r.t)
	}
}

// literals walks a translated body: a literal of a record or variant is written through its hook (log-2026-10-06 "U1 review" 5).
func (w *foreignWalk) literals(n PExpr) {
	if n == nil {
		return
	}
	if lit, ok := n.(*Lit); ok {
		w.typ(lit.T)
	}
	for _, sub := range pexprChildren(n) {
		w.literals(sub)
	}
}

func (w *foreignWalk) typ(t TypeRef) {
	w.table(t)
	switch x := t.Named.(type) {
	case *Variant:
		if t.Kind == types.Case && t.Case != nil {
			w.use.variantOf[t.Case] = x
			w.class(t.Case)
		} else {
			w.class(x)
		}
	case *Record:
		w.class(x)
	case *Dependent:
		w.dependent(x)
	}
	if t.Kind == types.Ref {
		return // a key, never a value of the target (CODEGEN.md §5.8)
	}
	for _, sub := range []*TypeRef{t.Elem, t.Key} {
		if sub != nil {
			w.typ(*sub)
		}
	}
}

// table records a table held by a class of another package (TableUse).
func (w *foreignWalk) table(t TypeRef) {
	rec, ok := tableElem(t)
	if !ok || w.holder == w.own {
		return
	}
	if use := (TableUse{Holder: w.holder, Record: rec}); !slices.Contains(w.use.Tables, use) {
		w.use.Tables = append(w.use.Tables, use)
	}
}

// class enters a record, a variant (its cases) or a case: its stored fields and stored fns' results.
func (w *foreignWalk) class(c any) {
	if w.seen[c] {
		return
	}
	w.seen[c] = true
	holder := ""
	switch x := c.(type) {
	case *Record:
		w.keep(x, x.Pkg)
		holder = x.Pkg
	case *Variant:
		w.variant(x)
		return
	case *Case:
		if v := w.use.variantOf[x]; v != nil {
			w.keep(x, v.Pkg)
			holder = v.Pkg
		}
	}
	w.body(c, holder)
}

// variant keeps a variant and enters each of its cases with fields, and the stored results of a case without fields, which its hook takes (log-2026-10-06 "U1 review" 6).
func (w *foreignWalk) variant(v *Variant) {
	w.keep(v, v.Pkg)
	for _, cs := range v.Cases {
		w.use.variantOf[cs] = v
		if len(cs.Fields) > 0 {
			w.class(cs)
		} else {
			w.body(cs, v.Pkg)
		}
	}
}

// dependent keeps a dependent type and enters its branches.
func (w *foreignWalk) dependent(d *Dependent) {
	if w.seen[d] {
		return
	}
	w.seen[d] = true
	w.keep(d, d.Pkg)
	for _, b := range d.Branches {
		w.typ(b.Type)
	}
}

// body enters a record's or case's stored fields (no input, no Never?) and its stored fns' results, holder its package.
func (w *foreignWalk) body(c any, holder string) {
	outer := w.holder
	w.holder = holder
	defer func() { w.holder = outer }()
	fields, fns := classBody(c)
	for _, f := range fields {
		if f.Input == nil && (!f.Optional || f.Type.Kind != types.Never) {
			w.typ(f.Type)
		}
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			w.typ(fn.Result)
		}
	}
}

// keep adds a class of another package, once.
func (w *foreignWalk) keep(c any, pkg string) {
	if pkg != w.own && !slices.Contains(w.out, c) {
		w.out = append(w.out, c)
	}
}

// ownerEmit is the emit of target t of package pkg as p imports it (the copy CopyOf leaves), nil when it has none (E8004's).
func ownerEmit(p *Package, pkg string, t Target) *Emit {
	for _, imp := range p.Imports {
		if imp.Name != pkg {
			continue
		}
		if i := slices.IndexFunc(imp.Emits, func(e *Emit) bool { return e.Target == t }); i >= 0 {
			return imp.Emits[i]
		}
	}
	return nil
}

// ownerResolves reports a ref of a class of package pkg whose owner fills its resolved getter in a make hook: a ref into a value of pkg that pkg's baked or embedded emit of target t selects (CODEGEN.md §5.8, §5.14). A data or types owner writes no hook for a record its loader resolves (log-2026-10-06 "U1 review" 3), which stage E refuses to the package building it.
func ownerResolves(p *Package, pkg string, t Target, r *RefTarget) bool {
	e := ownerEmit(p, pkg, t)
	if e == nil || r == nil || e.Mode != ModeBaked && e.Mode != ModeEmbedded {
		return false
	}
	return r.Coll == types.CollLet && !r.Local && r.Value != "" && r.Pkg == pkg && emitSelects(e, r.Value)
}

// classPkg is the package of a class: a record's, variant's or dependent type's own, a case's variant's.
func (u *ForeignUse) classPkg(c any) string {
	if cs, ok := c.(*Case); ok {
		if v := u.variantOf[cs]; v != nil {
			return v.Pkg
		}
		return ""
	}
	if t, ok := c.(Type); ok {
		return pkgOf(t)
	}
	return ""
}

// classOrigin is a class as messages name it: its qualified name, a case's under its variant's.
func (u *ForeignUse) classOrigin(c any) string {
	if cs, ok := c.(*Case); ok {
		return u.variantOf[cs].QName() + qnameSep + cs.Name
	}
	t, _ := c.(Type)
	return t.QName()
}
