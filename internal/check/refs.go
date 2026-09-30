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

// resolution is a ref whose target is being found: len(inferring) when it began, the let whose
// inference began it (nil for none), and the lets whose own inference read that target.
type resolution struct {
	depth  int
	reader *object
	cycle  []*object
}

// pendingRef is a `ref X` whose target is found once every type of its package is known.
type pendingRef struct {
	tc   typeCtx
	node *syntax.RefType
}

// fileSpan is a span with the checked file holding it.
type fileSpan struct {
	file *syntax.File
	span source.Span
}

// compareFileSpans orders two spans by their file's path, then by start: a FileID, new for each
// edited file of a persistent file set, never orders.
func compareFileSpans(a, b fileSpan) int {
	return cmp.Or(cmp.Compare(a.file.Src.Path, b.file.Src.Path), cmp.Compare(a.span.Start, b.span.Start))
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
		return compareFileSpans(fileSpan{x.tc.env.file, x.tc.env.span(x.node)}, fileSpan{y.tc.env.file, y.tc.env.span(y.node)})
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
	written := r.Target // its own placeholder, read as the error type until it resolves (TYPES.md §1)
	res := &resolution{depth: len(c.inferring), reader: c.inferred()}
	c.resolving[written] = res
	outer := c.current
	c.current = res
	coll := c.resolveRefName(&pr.tc, pr.node)
	c.current = outer
	delete(c.resolving, written)
	if coll == nil {
		c.targetless[written] = true
		c.breakObj(pr.tc.env.owner)
		for _, o := range res.cycle { // no cycle to report (TYPES.md §10.2): their findings stand
			c.settle(o)
		}
		return r
	}
	c.inferenceCycleFound(res.cycle)
	r.Target = coll
	return r
}

// inferred is the innermost let being inferred, nil for none.
func (c *checker) inferred() *object {
	if len(c.inferring) == 0 {
		return nil
	}
	return c.inferring[len(c.inferring)-1]
}

// coll is the target of a ref, resolving it first if needed.
func (c *checker) coll(r *types.RefType) *types.Collection {
	c.refTarget(r)
	c.inferenceCycle(r)
	return r.Target
}

// brokenRef reports a ref whose target resolved to nothing: it stands for the error type (TYPES.md §1).
func (c *checker) brokenRef(t types.Type) bool {
	r, ok := t.Base().(*types.RefType)
	if !ok {
		return false
	}
	c.refTarget(r) // a pending ref resolves first: its target is needed (TYPES.md §10.2)
	c.inferenceCycle(r)
	return c.noTarget(r.Target)
}

// noTarget reports the target of a ref that resolves to nothing, or not yet.
func (c *checker) noTarget(coll *types.Collection) bool {
	return c.targetless[coll] || c.resolving[coll] != nil
}

// inferenceCycle notes the let whose own inference reads r's target while r resolves.
func (c *checker) inferenceCycle(r *types.RefType) {
	if res := c.resolving[r.Target]; res != nil && len(c.inferring) > res.depth {
		c.readTarget(res, c.inferred())
	}
}

// readTarget notes o as reading a target being found.
func (c *checker) readTarget(res *resolution, o *object) {
	if o != nil && !slices.Contains(res.cycle, o) {
		res.cycle = append(res.cycle, o)
	}
}

// inferenceCycleFound is E3008 alone at each let that read a target being found (TYPES.md §10.2).
func (c *checker) inferenceCycleFound(lets []*object) {
	for _, o := range lets {
		o.typ = types.ErrorType
		c.cycled[o] = true
		env := c.declEnv(o)
		c.deliver(env.pkg, origin{decl: o}, diag.E3008.At(env.span(o.decl.(*syntax.LetDecl).Name)).Report)
		c.counted(env)
		c.settle(o)
	}
}

// settle ends the buffering of o's findings once its inference is over and no resolution under
// way counts it as a reader: dropped for a let that is E3008, else reported in order.
func (c *checker) settle(o *object) {
	if o.state == stateResolving || c.readingNow(o) {
		return
	}
	buf, ok := c.buffered[o]
	if !ok {
		return
	}
	delete(c.buffered, o)
	if c.cycled[o] {
		return
	}
	p := c.pkgs[o.pkg]
	for _, put := range buf {
		c.deliver(p, origin{decl: o}, put)
	}
}

// readingNow reports o among the readers of a target still being found.
func (c *checker) readingNow(o *object) bool {
	for _, res := range c.resolving { //canon:unordered a membership test
		if slices.Contains(res.cycle, o) {
			return true
		}
	}
	return false
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
		if o.kind != ObjLet || (o.local && !own) {
			continue
		}
		if c.onCycle(o) || c.info.Broken[o] && o.typ == nil { // a reader first, broken or not
			continue
		}
		t := c.letType(o)
		if elem, _, ok := collectionElem(t); ok && types.Identical(elem, rec) {
			out = append(out, c.letCollection(env, o, nil, nil))
		}
	}
	return out
}

// onCycle reports a let whose inference led to this search, no candidate (TYPES.md §10.2).
func (c *checker) onCycle(o *object) bool {
	d, _ := o.decl.(*syntax.LetDecl)
	res := c.current
	if d == nil || d.Type != nil || o.state != stateResolving || res == nil || res.reader == nil {
		return false
	}
	c.readTarget(res, res.reader)
	return true
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
			if coll := c.fieldCollection(owner, f); coll != nil && types.Identical(coll.Elem, rec) {
				out = append(out, coll)
			}
		}
	}
	return out
}

// fieldCollection is the collection field f of owner holds, a table or keyed list; nil for none (TYPES.md §9.4).
func (c *checker) fieldCollection(owner *types.RecordType, f *types.Field) *types.Collection {
	elem, keyed, isColl := collectionElem(c.fieldType(f))
	if !isColl {
		return nil
	}
	k := collKey{kind: types.CollField, owner: owner, path: f.Name}
	return c.intern(k, &types.Collection{Kind: types.CollField, Pkg: owner.Pkg, Owner: owner, FieldPath: []string{f.Name}, Elem: elem, KeyedBy: keyed})
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
