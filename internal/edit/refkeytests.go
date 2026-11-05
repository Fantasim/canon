package edit

import (
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keyTest judges a never-failing key test of the target's key on its home collection, a method
// answering a Bool or an optional (hasKey, get, find): a code ref when its receiver reaches the
// target's instance statically, none when it reaches another's, else a refusal (DECISIONS 483).
func (sc *nameScan) keyTest(n syntax.Node, at scanAt) {
	call, isCall := n.(*syntax.CallExpr)
	if !isCall || len(call.Args) != 1 || call.Args[0].Value == nil || !neverFails(sc.s.info.Types[call]) {
		return
	}
	sel, isSel := syntax.Unparen(call.Fun).(*syntax.SelectorExpr)
	if !isSel {
		return
	}
	arg := syntax.Unparen(call.Args[0].Value)
	match, literal := sc.targetKey(arg)
	if _, named := sc.names(arg); named || !match || !sc.atHome(sel.X) {
		return
	}
	sp := sc.file.Span(arg)
	entry, static := sc.reaches(sel.X)
	switch target := sc.tg.isEntry(entry); {
	case !static, target && !literal: // a constant's key is shared: no rewrite renames this test alone
		sc.refuse(sp)
	case target:
		sc.out = append(sc.out, Ref{Kind: at.kind, Package: sc.pkg, Span: sp})
	}
}

// neverFails reports t a test's answer: a Bool, or an optional none when nothing matches.
func neverFails(t types.Type) bool {
	if t == nil {
		return false
	}
	k := t.Base().Kind()
	return k == types.Bool || k == types.Optional
}

// targetKey reports e the target's key, and whether e is a literal the rename rewrites; a
// constant's value is folded as a literal's, its declaration shared by every use.
func (sc *nameScan) targetKey(e syntax.Expr) (match, literal bool) {
	id := sc.tg.ident
	if id == nil {
		return false, false
	}
	if v, ok := sc.constKey(e); ok {
		return keyIs(v, id.Key), false
	}
	if n, isInt := e.(*syntax.IntLit); isInt {
		return id.Key.IsInt && n.Value.IsInt64() && n.Value.Int64() == id.Key.I, true
	}
	text, ok := constString(e)
	return ok && !id.Key.IsInt && text == id.Key.S, true
}

// constKey is the value of the constant e names, if it names one.
func (sc *nameScan) constKey(e syntax.Expr) (value.Value, bool) {
	var obj check.Object
	switch x := e.(type) {
	case *syntax.IdentExpr:
		obj = sc.s.info.Uses[x]
	case *syntax.SelectorExpr:
		obj = sc.s.info.NameUses[x.Name]
	}
	if obj == nil || obj.Kind() != check.ObjConst || sc.s.byPath[obj.Pkg()] == nil {
		return nil, false
	}
	v, err := sc.s.force(rootRef{pkg: sc.s.byPath[obj.Pkg()], obj: obj})
	return v, err == nil
}

// keyIs reports v a String or Int value equal to key k.
func keyIs(v value.Value, k value.Key) bool {
	switch x := v.(type) {
	case *value.Str:
		return !k.IsInt && x.V == k.S
	case *value.Int:
		return k.IsInt && x.V == k.I
	}
	return false
}

// home is the record type and field whose per-instance collection holds the target.
func (tg *target) home() (*types.RecordType, string, bool) {
	switch {
	case tg.held != nil:
		return tg.held.owner, tg.held.field, true
	case tg.ident != nil && tg.ident.Coll != nil && tg.ident.Coll.Kind == types.CollField && len(tg.ident.Coll.FieldPath) == 1:
		return tg.ident.Coll.Owner, tg.ident.Coll.FieldPath[0], true
	}
	return nil, "", false
}

// atHome reports e, by its static type, a collection of the target's home field's shape: a keyed
// list of the same records and key, or a table of the same records, wherever it is held.
func (sc *nameScan) atHome(e syntax.Expr) bool {
	owner, field, ok := sc.tg.home()
	if !ok {
		return false
	}
	i := fieldIndex(owner.Fields, field)
	return i >= 0 && sameCollection(sc.s.info.Types[e], owner.Fields[i].Type)
}

// sameCollection reports a and b, optionals aside, keyed lists of one record by one key field, or
// tables of one record.
func sameCollection(a, b types.Type) bool {
	if a == nil || b == nil {
		return false
	}
	switch x := present(a).Base().(type) {
	case *types.ListType:
		y, ok := present(b).Base().(*types.ListType)
		return ok && x.KeyedBy != nil && y.KeyedBy != nil && x.KeyedBy.Name == y.KeyedBy.Name && x.Elem.Base() == y.Elem.Base()
	case *types.TableType:
		y, ok := present(b).Base().(*types.TableType)
		return ok && x.Elem.Base() == y.Elem.Base()
	}
	return false
}

// recordDecl is the record type d declares, nil for none.
func (s *Snapshot) recordDecl(d *syntax.RecordDecl) *types.RecordType {
	if d == nil || d.Name == nil || s.info.Defs[d.Name] == nil {
		return nil
	}
	return recordType(s.info.Defs[d.Name].Type())
}

// reaches is the target-key entry of the collection e reads statically, and whether e is static.
func (sc *nameScan) reaches(e syntax.Expr) (*value.Identity, bool) {
	p, ok := sc.s.StaticPath(e)
	if !ok {
		return nil, false
	}
	p = sc.s.unalias(p)
	res, err := Resolve(sc.s, p)
	if err != nil {
		return nil, true // a read through a none, or of nothing: no entry
	}
	for _, c := range children(res.Target) {
		rec, isRec := c.v.(*value.Record)
		if !isRec || rec.Ident == nil || rec.Ident.Key != sc.tg.ident.Key {
			continue
		}
		if sc.tg.isEntry(rec.Ident) {
			return rec.Ident, true
		}
		_, stated := sc.s.structural(Path{Package: p.Package, Root: p.Root, Segs: append(slices.Clip(p.Segs), c.seg)})
		return rec.Ident, stated // another instance's only when stated in its own source: a copied key moves with the rename
	}
	return nil, true
}

// unalias is p read through the lets whose value is a static path, each root once: `s2.k` of
// `let s2 = aa.second.oo` is `aa.second.oo.k`, the instance the let's re-adopted copy holds (DECISIONS 315).
func (s *Snapshot) unalias(p Path) Path {
	seen := map[string]bool{}
	for !seen[p.Package+packageMark+p.Root] {
		seen[p.Package+packageMark+p.Root] = true
		r, err := s.lookup(p)
		if err != nil || r.obj.Kind() != check.ObjLet {
			return p
		}
		init := initializer(r.obj.Decl())
		q, ok := s.StaticPath(syntax.Unparen(init))
		if init == nil || !ok {
			return p
		}
		p = Path{Package: q.Package, Root: q.Root, Segs: slices.Concat(q.Segs, p.Segs)}
	}
	return p
}

// refuse keeps the token at sp among those no rename can judge: the rename is refused `computed`.
func (sc *nameScan) refuse(sp source.Span) {
	line, col := sc.s.a.Files().Position(sp.File, sp.Start)
	sc.shared = append(sc.shared, sc.s.display(sp.File)+packageMark+strconv.Itoa(line)+packageMark+strconv.Itoa(col))
}

// transient reports the record declaring at's field built in a function, a method or a lambda
// without that field: those evaluations take its default, and no walk stores them (DECISIONS 483).
func (sc *nameScan) transient(at scanAt) bool {
	rec := sc.s.recordDecl(at.record)
	if rec == nil || at.field == nil || at.field.Name == nil {
		return false
	}
	k := transientKey{rec: rec, field: at.field.Name.Name}
	if known, ok := sc.built[k]; ok {
		return known
	}
	found := false
	for _, pkg := range sc.s.pkgs {
		for _, f := range pkg.Files {
			found = found || sc.buildsWithout(f, k)
		}
	}
	sc.built[k] = found
	return found
}

// transientKey is a record type and one of its fields.
type transientKey struct {
	rec   *types.RecordType
	field string
}

// buildsWithout reports a literal of k's record, in a function, method or lambda of f, stating
// neither k's field nor a spread.
func (sc *nameScan) buildsWithout(f *syntax.File, k transientKey) bool {
	found := false
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.FnDecl:
			found = found || sc.called(x) && sc.literalWithout(x, k)
			return false
		case *syntax.LambdaExpr:
			found = found || sc.literalWithout(x, k)
			return false
		}
		return !found
	})
	return found
}

// called reports a function or method some name or selection of the program uses: one no code
// calls produces no evaluation to judge.
func (sc *nameScan) called(fn *syntax.FnDecl) bool {
	obj := sc.s.info.Defs[fn.Name]
	if obj == nil {
		return false
	}
	if known, ok := sc.calls[obj]; ok {
		return known
	}
	used := false
	for _, o := range sc.s.info.Uses { //canon:unordered a membership test
		used = used || o == obj
	}
	for _, o := range sc.s.info.NameUses { //canon:unordered a membership test
		used = used || o == obj
	}
	for _, sel := range sc.s.info.Selections { //canon:unordered a membership test
		used = used || sel != nil && sel.Obj == obj
	}
	sc.calls[obj] = used
	return used
}

// literalWithout reports a literal of k's record under region stating neither k's field nor a spread.
func (sc *nameScan) literalWithout(region syntax.Node, k transientKey) bool {
	found := false
	syntax.Inspect(region, func(n syntax.Node) bool {
		if lit := braceOf(n); lit != nil && sc.s.recordOf(n) == k.rec && fieldItem(lit, k.field) == nil && !hasSpread(lit) {
			found = true
		}
		return !found
	})
	return found
}
