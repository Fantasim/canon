package check

import (
	"cmp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// pendingRef is a `ref X` whose target is found once every type of its package is known.
type pendingRef struct {
	tc   typeCtx
	node *syntax.RefType
}

// errorColl is the target of a ref that resolved to nothing: its element is the error type.
var errorColl = &types.Collection{Elem: types.ErrorType}

// compareSpans orders two spans of the checked files: by file, then by start.
func compareSpans(a, b source.Span) int {
	return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Start, b.Start))
}

// isDefines reports a let initialized by `load.defines(…)`, a collection of Define.
func isDefines(let *object) bool {
	d, ok := let.decl.(*syntax.LetDecl)
	if !ok {
		return false
	}
	l, isLoad := d.Value.(*syntax.LoadExpr)
	return isLoad && l.Method != nil && l.Method.Name == loadDefines
}

// collKey interns collections: two refs share a target exactly when their pointers are equal.
type collKey struct {
	kind  types.CollKind
	pkg   string
	name  string
	owner *types.RecordType
	path  string
}

// newRef is `ref X`; its target is resolved at the end of its package's step 2, or at once
// when that step is over (a local annotation in a body).
func (c *checker) newRef(tc *typeCtx, t *syntax.RefType) types.Type {
	r := &types.RefType{Target: &types.Collection{Name: qualified(t.Name), Elem: types.ErrorType}}
	c.pending[r] = &pendingRef{tc: *tc, node: t}
	if c.bodies {
		return c.refTarget(r)
	}
	return r
}

// resolvePending resolves every ref of p still pending, in the order they were written.
func (c *checker) resolvePending(p *pkgState) {
	var refs []*types.RefType
	for r, pr := range c.pending { //canon:unordered sorted by position below
		if pr.tc.env.pkg == p {
			refs = append(refs, r)
		}
	}
	slices.SortFunc(refs, func(a, b *types.RefType) int {
		x, y := c.pending[a], c.pending[b]
		return compareSpans(x.tc.env.span(x.node), y.tc.env.span(y.node))
	})
	for _, r := range refs {
		c.refTarget(r)
	}
}

// refTarget resolves a pending ref (TYPES.md §10.2).
func (c *checker) refTarget(r *types.RefType) types.Type {
	pr, ok := c.pending[r]
	if !ok {
		return r
	}
	delete(c.pending, r)
	r.Target = errorColl // read again while its name resolves (a let inferred on the way), it is the error type (TYPES.md §1)
	coll := c.resolveRefName(&pr.tc, pr.node)
	if coll == nil {
		coll = errorColl
		c.breakObj(pr.tc.env.owner)
	}
	r.Target = coll
	return r
}

// coll is the target of a ref, resolving it first if needed.
func (c *checker) coll(r *types.RefType) *types.Collection {
	c.refTarget(r)
	return r.Target
}

// brokenRef reports a ref whose target resolved to nothing: it stands for the error type (TYPES.md §1).
func (c *checker) brokenRef(t types.Type) bool {
	r, ok := t.Base().(*types.RefType)
	return ok && r.Target == errorColl // a pending ref is resolved where its package says, never here
}

// erroneous reports the error type, or a ref standing for it.
func (c *checker) erroneous(t types.Type) bool {
	return t.Kind() == types.Error || c.brokenRef(t)
}

// resolveRefName finds what `ref X` names: a let or a path through its record fields, or a
// record type searched level by level.
func (c *checker) resolveRefName(tc *typeCtx, t *syntax.RefType) *types.Collection {
	env := tc.env
	q := t.Name
	o := c.lookupGlobal(env, q.Parts[0].Name)
	if o == nil {
		c.unknownName(env, q.Parts[0], q.Parts[0].Name)
		return nil
	}
	c.info.NameUses[q.Parts[0]] = o
	rest := q.Parts[1:]
	if o.kind == ObjPackage && len(rest) > 0 {
		m, ok := o.target.names[rest[0].Name]
		if !ok || m.local {
			c.report(env, diag.E2004.At(env.span(rest[0]), o.target.path, rest[0].Name))
			return nil
		}
		c.info.NameUses[rest[0]] = m
		o, rest = m, rest[1:]
	}
	c.dependsOn(env, o)
	switch o.kind {
	case ObjLet:
		return c.letCollection(env, o, rest, q)
	case ObjTypeName, ObjBuiltin:
		if t := c.typeOfName(env, o, nil); t != nil && len(rest) == 0 {
			if rec, ok := t.Base().(*types.RecordType); ok {
				return c.searchLevels(tc, rec, q)
			}
		}
	default:
	}
	c.report(env, diag.E3504.At(env.span(q), qualified(q)))
	return nil
}

// letCollection is a let, or a path through its record fields, that is a collection, else E3504 (TYPES.md §9.4).
func (c *checker) letCollection(env *env, let *object, path []*syntax.Ident, q *syntax.QualifiedName) *types.Collection {
	if annotating(let) { // named in its own annotation: not a collection (log 2026-09-24, check C2 review)
		c.report(env, diag.E3504.At(env.span(q), qualified(q)))
		return nil
	}
	t := c.letType(let)
	var names []string
	for _, part := range path {
		if t.Kind() == types.Error { // names nothing, reports nothing (TYPES.md §1)
			return nil
		}
		rec, ok := t.Base().(*types.RecordType)
		if !ok {
			c.report(env, diag.E3504.At(env.span(q), qualified(q)))
			return nil
		}
		c.completeRecord(rec)
		f := fieldNamed(rec.Fields, part.Name)
		if f == nil {
			c.report(env, diag.E3003.At(env.span(part), rec, diag.KindField, part.Name))
			return nil
		}
		c.info.NameUses[part] = c.fieldObjects[f]
		t = c.fieldType(f)
		names = append(names, part.Name)
	}
	elem, keyed, ok := collectionElem(t)
	if !ok && t.Kind() == types.Error {
		return nil
	}
	if !ok {
		c.report(env, diag.E3504.At(env.span(q), qualified(q)))
		return nil
	}
	return c.internLet(let, names, elem, keyed)
}

// internLet is the collection a let, or the field path names through its records, holds.
func (c *checker) internLet(let *object, names []string, elem types.Type, keyed *types.Field) *types.Collection {
	kind := types.CollLet
	if isDefines(let) {
		kind = types.CollDefines
	}
	k := collKey{kind: kind, pkg: let.pkg, name: let.name, path: strings.Join(names, dot)}
	return c.intern(k, &types.Collection{Kind: kind, Pkg: let.pkg, Name: let.name, FieldPath: names, Elem: elem, KeyedBy: keyed, Local: let.local})
}

// collectionElem is the element of a table or keyed list type, and its key field.
func collectionElem(t types.Type) (types.Type, *types.Field, bool) {
	switch x := t.Base().(type) {
	case *types.TableType:
		return x.Elem, nil, true
	case *types.ListType:
		if x.KeyedBy != nil {
			return x.Elem, x.KeyedBy, true
		}
	}
	return nil, nil, false
}

func (c *checker) intern(k collKey, coll *types.Collection) *types.Collection {
	if found, ok := c.colls[k]; ok {
		return found
	}
	c.colls[k] = coll
	return coll
}

// searchLevels is the level-by-level search for a collection of rec (TYPES.md §10.2).
func (c *checker) searchLevels(tc *typeCtx, rec *types.RecordType, q *syntax.QualifiedName) *types.Collection {
	env := tc.env
	levels := []func() []*types.Collection{
		func() []*types.Collection { return c.enclosingCandidates(tc, rec) },
		func() []*types.Collection { return c.letCandidates(env, env.pkg, rec, true) },
		func() []*types.Collection { return c.importCandidates(env, rec) },
	}
	for _, level := range levels {
		found := level()
		switch len(found) {
		case 0:
			continue
		case 1:
			return found[0]
		}
		var names []string
		for _, f := range found {
			pkg := f.Pkg
			if f.Kind == types.CollField {
				pkg = ownerPkg(f.Owner)
			}
			names = append(names, env.localName(f.String(), pkg))
		}
		c.report(env, diag.E2103.AtSeveral(env.span(q), qualified(q), names))
		return nil
	}
	c.report(env, diag.E2103.AtNone(env.span(q), qualified(q)))
	return nil
}

// letCandidates are the top-level lets of p whose type is a collection of rec; local ones
// only in the package itself.
func (c *checker) letCandidates(env *env, p *pkgState, rec *types.RecordType, own bool) []*types.Collection {
	var out []*types.Collection
	for _, o := range p.all {
		if o.kind != ObjLet || (o.local && !own) || c.info.Broken[o] && o.typ == nil {
			continue
		}
		t := c.letType(o)
		if elem, _, ok := collectionElem(t); ok && types.Identical(elem, rec) {
			out = append(out, c.letCollection(env, o, nil, nil))
		}
	}
	return out
}

// importCandidates are the public collections of rec in the packages the file imports.
func (c *checker) importCandidates(env *env, rec *types.RecordType) []*types.Collection {
	var out []*types.Collection
	seen := map[*pkgState]bool{}
	for _, e := range env.pkg.edges {
		if e.file != env.file || seen[e.to] {
			continue
		}
		seen[e.to] = true
		out = append(out, c.letCandidates(env, e.to, rec, false)...)
	}
	return out
}

// enclosingCandidates is level 1: fields of records of the package, holding a collection of
// rec, in a record that contains the record or case whose field type holds the ref. A ref in
// a signature or a let annotation has no enclosing record and skips this level.
func (c *checker) enclosingCandidates(tc *typeCtx, rec *types.RecordType) []*types.Collection {
	if tc.encl == nil {
		return nil
	}
	var out []*types.Collection
	for _, o := range tc.env.pkg.all {
		owner, ok := o.typ.(*types.RecordType)
		if o.kind != ObjTypeName || !ok || !c.contains(owner, tc.encl, map[types.Type]bool{}) {
			continue
		}
		for _, f := range owner.Fields {
			if elem, keyed, isColl := collectionElem(c.fieldType(f)); isColl && types.Identical(elem, rec) {
				k := collKey{kind: types.CollField, owner: owner, path: f.Name}
				out = append(out, c.intern(k, &types.Collection{Kind: types.CollField, Pkg: owner.Pkg, Owner: owner, FieldPath: []string{f.Name}, Elem: elem, KeyedBy: keyed}))
			}
		}
	}
	return out
}

// contains reports that a value of type r holds a d, itself included: through fields, lists,
// keyed lists, tables, maps, optionals and variant cases.
func (c *checker) contains(r types.Type, d types.Type, seen map[types.Type]bool) bool {
	if r == d {
		return true
	}
	if seen[r] {
		return false
	}
	seen[r] = true
	for _, t := range c.componentTypes(r) {
		if c.contains(t, d, seen) {
			return true
		}
	}
	return false
}

// componentTypes are the named types a value of type t holds directly.
func (c *checker) componentTypes(t types.Type) []types.Type {
	switch x := t.Base().(type) {
	case *types.RecordType:
		c.completeRecord(x)
		var out []types.Type
		for _, f := range x.Fields {
			out = append(out, namedIn(c.fieldType(f))...)
		}
		return out
	case *types.VariantType:
		c.completeVariant(x)
		var out []types.Type
		for _, ct := range x.Cases {
			out = append(out, ct)
		}
		return out
	case *types.CaseType:
		var out []types.Type
		for _, f := range x.Fields {
			out = append(out, namedIn(c.fieldType(f))...)
		}
		return out
	}
	return nil
}

// namedIn are the records and variants a field type holds through lists, tables, maps and
// optionals.
func namedIn(t types.Type) []types.Type {
	switch x := t.Base().(type) {
	case *types.RecordType, *types.VariantType:
		return []types.Type{x}
	case *types.AppliedRecord:
		return []types.Type{x.Rec}
	case *types.OptionalType:
		return namedIn(x.Elem)
	case *types.ListType:
		return namedIn(x.Elem)
	case *types.TableType:
		return namedIn(x.Elem)
	case *types.MapType:
		return append(namedIn(x.Key), namedIn(x.Value)...)
	}
	return nil
}
