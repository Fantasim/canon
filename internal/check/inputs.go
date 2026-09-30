package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// checkInputs is E1903: an input record reached once through fields, in no container, E3022's alone (EVALUATION.md §11.1).
func (c *checker) checkInputs(p *pkgState) {
	var written []types.Type
	for _, o := range p.all {
		r, ok := o.typ.(*types.RecordType)
		if o.kind != ObjTypeName || !ok || !hasInput(r) || reaches(r, r, map[*types.RecordType]bool{}) {
			continue
		}
		if written == nil {
			written = c.writtenTypes(p)
		}
		pc := newPathCounter(r)
		if c.inputPaths(p, pc) != 1 || c.containedIn(p, pc, written) {
			env := c.declEnv(o)
			c.report(env, diag.E1903.At(env.span(r.Decl.Name), o.name))
		}
	}
	c.inputCases(p)
}

// inputCases is E1903 at each variant case that declares an input field itself (EVALUATION.md §11.1).
func (c *checker) inputCases(p *pkgState) {
	for _, o := range p.all {
		v, ok := o.typ.(*types.VariantType)
		if o.kind != ObjTypeName || !ok {
			continue
		}
		for _, ct := range v.Cases {
			if vc := c.caseDecls[ct]; vc != nil && fieldsHaveInput(ct.Fields) {
				env := c.declEnv(o)
				c.report(env, diag.E1903.At(env.span(vc.Name), o.name+dot+ct.Name))
			}
		}
	}
}

// portable is E1904 for each pattern of an input's alias chain (EVALUATION.md §11.3, TYPES.md §7.4).
func (c *checker) portable(env *env, f *types.Field, at syntax.Node) {
	for t := f.Type; t != nil; {
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.OptionalType:
			t = x.Elem
		case *types.Refined:
			c.portablePattern(env, x, at)
			t = x.Of
		default:
			t = nil
		}
	}
}

// portablePattern is E1904 for the pattern of one refinement.
func (c *checker) portablePattern(env *env, r *types.Refined, at syntax.Node) {
	if r.Pattern == nil {
		return
	}
	if construct := unportable(r.Pattern.String()); construct != "" {
		c.report(env, diag.E1904.At(env.span(at), r.Pattern.String(), construct))
	}
}

func hasInput(r *types.RecordType) bool {
	return fieldsHaveInput(r.Fields)
}

func fieldsHaveInput(fields []*types.Field) bool {
	return slices.ContainsFunc(fields, func(f *types.Field) bool { return f.Input != nil })
}

// inputPaths counts the ways public lets of p reach r through record fields and optionals,
// saturated at 2 (more than one is all E1903 needs).
func (c *checker) inputPaths(p *pkgState, pc *pathCounter) int {
	n := 0
	for _, o := range p.all {
		if o.kind == ObjLet && !o.local {
			n = min(n+pc.paths(c.letType(o)), manyPaths)
		}
	}
	return n
}

// pathCounter counts the record-field paths to r, memoized per record; a cycle on the way to r
// is many paths.
type pathCounter struct {
	r      *types.RecordType
	count  map[*types.RecordType]int
	onPath map[*types.RecordType]bool
	holds  map[*types.RecordType]bool // r, or a record whose fields reach r
}

func newPathCounter(r *types.RecordType) *pathCounter {
	return &pathCounter{r: r, count: map[*types.RecordType]int{}, onPath: map[*types.RecordType]bool{}, holds: map[*types.RecordType]bool{}}
}

// paths counts the paths from a value of type t to r through record fields.
func (pc *pathCounter) paths(t types.Type) int {
	rec := requiredRecord(unwrapOptional(t))
	switch {
	case rec == nil:
		return 0
	case rec == pc.r || pc.onPath[rec]:
		if pc.reachesFrom(rec, map[*types.RecordType]bool{}) {
			return manyPaths
		}
		return boolCount(rec == pc.r)
	}
	if n, done := pc.count[rec]; done {
		return n
	}
	pc.onPath[rec] = true
	n := 0
	for _, f := range rec.Fields {
		if !selfContaining(rec, f) {
			n = min(n+pc.paths(f.Type), manyPaths)
		}
	}
	delete(pc.onPath, rec)
	pc.count[rec] = n
	return n
}

func boolCount(b bool) int {
	if b {
		return 1
	}
	return 0
}

// reachesFrom reports r reached from rec's fields through record fields and optionals; E3022's
// cycle is not a way (it adds no path).
func (pc *pathCounter) reachesFrom(rec *types.RecordType, seen map[*types.RecordType]bool) bool {
	for _, f := range rec.Fields {
		next := requiredRecord(unwrapOptional(f.Type))
		if next == nil || seen[next] || selfContaining(rec, f) {
			continue
		}
		seen[next] = true
		if next == pc.r || pc.reachesFrom(next, seen) {
			return true
		}
	}
	return false
}

// holdsR reports r, or a record whose fields reach r: holding one holds r (§11.1 read transitively).
func (pc *pathCounter) holdsR(rec *types.RecordType) bool {
	if rec == pc.r {
		return true
	}
	h, done := pc.holds[rec]
	if !done {
		h = pc.reachesFrom(rec, map[*types.RecordType]bool{})
		pc.holds[rec] = h
	}
	return h
}

// containedIn reports r, or a record holding it, as or inside a list, map, table, case field or
// dependent type anywhere in p: declarations, and every type written (aliases, signatures, bodies).
func (c *checker) containedIn(p *pkgState, pc *pathCounter, written []types.Type) bool {
	return slices.ContainsFunc(p.all, func(o *object) bool { return c.declHolds(o, pc) }) ||
		slices.ContainsFunc(written, func(t types.Type) bool { return pc.holdsIn(t, false) })
}

// writtenTypes are the types of every type expression of p's files.
func (c *checker) writtenTypes(p *pkgState) []types.Type {
	out := []types.Type{}
	for _, f := range p.files {
		syntax.Inspect(f, func(n syntax.Node) bool {
			if t, isType := n.(syntax.Type); isType && c.info.TypeExprs[t] != nil {
				out = append(out, c.info.TypeExprs[t])
			}
			return true
		})
	}
	return out
}

// declHolds reports r held by a container in one declaration: a record's fields, a variant's
// case fields, a let's type.
func (c *checker) declHolds(o *object, pc *pathCounter) bool {
	switch t := o.typ.(type) {
	case *types.RecordType:
		return o.kind == ObjTypeName && pc.fieldsHold(t.Fields, false)
	case *types.VariantType:
		return slices.ContainsFunc(t.Cases, func(ct *types.CaseType) bool { return pc.fieldsHold(ct.Fields, true) })
	}
	return o.kind == ObjLet && pc.holdsIn(c.letType(o), false)
}

func (pc *pathCounter) fieldsHold(fields []*types.Field, inCase bool) bool {
	return slices.ContainsFunc(fields, func(f *types.Field) bool { return pc.holdsIn(f.Type, inCase) })
}

// holdsIn reports r inside a container in t: a list, map, table or dependent type, or directly
// when contained (a case field).
func (pc *pathCounter) holdsIn(t types.Type, contained bool) bool {
	switch x := t.Base().(type) {
	case *types.RecordType:
		return contained && pc.holdsR(x)
	case *types.AppliedRecord:
		return contained && pc.holdsR(x.Rec)
	case *types.OptionalType:
		return pc.holdsIn(x.Elem, contained)
	case *types.ListType:
		return pc.holdsIn(x.Elem, true)
	case *types.TableType:
		return pc.holdsIn(x.Elem, true)
	case *types.MapType:
		return pc.holdsIn(x.Key, true) || pc.holdsIn(x.Value, true)
	case *types.DepMapType:
		return pc.holdsIn(x.Value, true)
	case *types.TypeAppType:
		return pc.typeFuncHolds(x.Fn)
	case *types.DepUnionType:
		return pc.typeFuncHolds(x.Fn)
	case *types.FuncType:
		return pc.holdsIn(x.Result, false) || slices.ContainsFunc(x.Params, func(p types.Type) bool { return pc.holdsIn(p, false) })
	}
	return false
}

// typeFuncHolds reports r among the results of a type function.
func (pc *pathCounter) typeFuncHolds(fn *types.TypeFunc) bool {
	if fn.Body != nil && pc.holdsIn(fn.Body, true) {
		return true
	}
	return slices.ContainsFunc(fn.Arms, func(a *types.TypeArm) bool { return pc.holdsIn(a.Result, true) })
}
