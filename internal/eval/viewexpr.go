package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// Magic are the values of id, key and index where a value is shown, nil for none (VIEWMODEL.md §3.4, G12).
type Magic struct {
	ID, Key, Index value.Value
}

// late is what the view model's reads after stage E change (EVALUATION.md §1 phase 8).
type late struct {
	files map[syntax.Expr]*syntax.File // every expression of a view or a translation file, to its file
	free  bool                         // View and ForceAside: the values they force spend no step (DECISIONS 148)
}

// View evaluates x, of a view or translation file, for self in VIEWMODEL.md §3.4's scope: no step, no finding; false fails (X7).
func (e *Evaluator) View(ctx context.Context, x syntax.Expr, self value.Value, m Magic) (value.Value, bool) {
	if e.prog == nil || e.info == nil || ctx.Err() != nil {
		return nil, false
	}
	file := e.viewFile(x)
	if file == nil {
		return nil, false
	}
	defer e.phase8(check.Bags{})()
	e.flush(context.WithoutCancel(ctx))
	r := e.newRun(ctx, charge{pkg: e.index.pkg[file]}, file)
	r.free, r.magic, r.fr.self = true, &m, self
	v := r.eval(x)
	return v, v != nil && !r.failed
}

// ForceAside is Force at no step cost, findings into bags: what a view model reads after stage E (VIEWMODEL.md C3, DECISIONS 148).
func (e *Evaluator) ForceAside(ctx context.Context, root Root, bags check.Bags) (value.Value, bool) {
	defer e.phase8(bags)()
	e.flush(context.WithoutCancel(ctx)) // what a cancelled call left unverified (J5)
	return e.Force(ctx, root)
}

// phase8 sets findings aside, steps free, no budget verdict nor cause kept, until restored (EVALUATION.md §1 phase 8).
func (e *Evaluator) phase8(bags check.Bags) func() {
	aside, free, exhausted, causes, via := e.aside, e.late.free, e.exhausted, e.causes, e.via
	e.aside, e.late.free, e.exhausted, e.causes, e.via = bags, true, false, nil, nil
	return func() {
		e.aside, e.late.free, e.exhausted, e.causes, e.via = aside, free, exhausted, causes, via
	}
}

// viewFile is the file holding x, an expression of a view or of a translation file; nil for any
// other. The index is built on first use.
func (e *Evaluator) viewFile(x syntax.Expr) *syntax.File {
	if e.late.files == nil {
		e.late.files = map[syntax.Expr]*syntax.File{}
		for _, pkg := range e.prog.Packages {
			for _, f := range pkg.Files {
				e.indexViewFile(f)
			}
		}
	}
	return e.late.files[x]
}

// indexViewFile records every expression of f's views and translation entries.
func (e *Evaluator) indexViewFile(f *syntax.File) {
	record := func(n syntax.Node) bool {
		if x, ok := n.(syntax.Expr); ok {
			e.late.files[x] = f
		}
		return true
	}
	for _, d := range f.Decls {
		if v, ok := d.(*syntax.ViewDecl); ok {
			syntax.Inspect(v, record)
		}
	}
	for _, en := range f.Entries {
		syntax.Inspect(en, record)
	}
}

// magicValue is the value of the magic name obj in a view run; a name without one fails the
// run silently (VIEWMODEL.md G12); false when obj is no magic name or the run is no view run.
func (r *run) magicValue(obj check.Object) (value.Value, bool) {
	if r.magic == nil || !isMagic(obj) {
		return nil, false
	}
	var v value.Value
	switch obj.Name() {
	case memberID:
		v = r.magic.ID
	case keyMagic:
		v = r.magic.Key
	case memberIndex:
		v = r.magic.Index
	}
	if v == nil {
		r.stop()
		return nil, true
	}
	return r.read(v), true
}

// isMagic reports a magic name: a local declared on a view, a step text or a translation entry (VIEWMODEL.md §3.4).
func isMagic(obj check.Object) bool {
	if obj.Kind() != check.ObjLocal {
		return false
	}
	switch obj.Decl().(type) {
	case *syntax.ViewDecl, *syntax.TranslationEntry, *syntax.StringLit:
		return true
	}
	return false
}
