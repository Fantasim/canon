package check

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// pathJob is an amend path or a `@files` template name path, resolved once every field is known:
// from the types start may hold, each name in each type its path may have reached.
type pathJob struct {
	file  *syntax.File
	site  OccSite
	kind  OccKind
	start []types.Type
	steps []pathStep
	sets  func(types.Type) [][]*types.Field // an amend path's sets skip a dependent field's branches (DECISIONS 280)
}

// pathStep is one name of a path and the object Info records for it; no node: a `[key]` or
// `[#position]` segment, which reaches the elements.
type pathStep struct {
	node     syntax.Node
	recorded Object
}

// noteField keeps a declared field's object, to name the candidates of a path.
func (b *occBuild) noteField(o Object) {
	if x, ok := o.(*object); ok && x.field != nil {
		b.fields[x.field] = o
	}
}

// amendPath queues a's path, read from the type of its block's target.
func (w *occWalk) amendPath(a *syntax.Amendment, ctx occCtx) {
	blk, ok := ctx.parent.(*syntax.AmendBlock)
	if !ok {
		return
	}
	j := pathJob{file: w.file, site: ctx.site, kind: OccAmend, sets: caseFieldSets}
	if target := w.info.NameUses[blk.Target]; target != nil && target.Type() != nil {
		j.start = []types.Type{target.Type()}
	}
	for _, seg := range a.Path {
		if seg.Name == nil {
			j.steps = append(j.steps, pathStep{})
			continue
		}
		j.steps = append(j.steps, pathStep{node: seg.Name, recorded: w.info.NameUses[seg.Name]})
		w.handled[seg.Name] = true
	}
	w.paths = append(w.paths, j)
}

// templatePaths queues each name path of a let's `@files` template, read from its elements' type.
func (w *occWalk) templatePaths(a *syntax.Annotation, ctx occCtx) {
	d, isLet := ctx.parent.(*syntax.LetDecl)
	tpl, isStr := firstArg(a).(*syntax.StringLit)
	if !isLet || !isStr || a.Name.Name != syntax.AnnFiles {
		return
	}
	var start []types.Type
	if o := w.info.Defs[d.Name]; o != nil && o.Type() != nil {
		if elem, _, isColl := collectionElem(o.Type()); isColl {
			start = []types.Type{elem}
		}
	}
	for _, part := range tpl.Parts {
		if part.Interp == nil {
			continue
		}
		j := pathJob{file: w.file, site: ctx.site, kind: OccFilesVar, start: start, sets: fieldSets}
		for _, n := range templatePath(part.Interp.X) {
			j.steps = append(j.steps, pathStep{node: n, recorded: w.info.ObjectOf(n)})
			w.handled[n] = true
		}
		w.paths = append(w.paths, j)
	}
}

// resolvePath lists each name of j: as Info records it while the path has one type that is not a
// variant; else under every field it names across the path's types, Ambiguous when several.
func (b *occBuild) resolvePath(j pathJob) {
	ts := j.start
	for _, s := range j.steps {
		if s.node == nil {
			ts = elementTypes(ts)
			continue
		}
		occ := Occurrence{File: j.file, Span: j.file.Span(s.node), Kind: j.kind, Site: j.site}
		fields := pathFields(ts, nameText(s.node), j.sets)
		objs := b.fieldObjects(fields)
		if s.recorded != nil && (onePlainType(ts) || len(objs) == 0) {
			b.index[s.recorded] = append(b.index[s.recorded], occ)
			ts = typesAfter(s.recorded, ts)
			continue
		}
		occ.Ambiguous = len(objs) > 1
		for _, o := range objs {
			b.index[o] = append(b.index[o], occ)
		}
		ts = fieldTypes(fields)
	}
}

func (b *occBuild) fieldObjects(fields []*types.Field) []Object {
	var out []Object
	for _, f := range fields {
		if o := b.fields[f]; o != nil {
			out = append(out, o)
		}
	}
	return out
}

// onePlainType reports a path that reached at most one type, and not a variant: what Info
// records for its next name is that name's one declaration.
func onePlainType(ts []types.Type) bool {
	if len(ts) != 1 {
		return len(ts) == 0
	}
	_, isVariant := unwrapOptional(ts[0]).Base().(*types.VariantType)
	return !isVariant
}

// typesAfter is what a path reaches through o: a field's type, an entry's element; else nothing.
func typesAfter(o Object, ts []types.Type) []types.Type {
	if o.Kind() == ObjField {
		return []types.Type{o.Type()}
	}
	if o.Kind() == ObjEntry {
		return elementTypes(ts)
	}
	return nil
}

// elementTypes are the elements of each list, table or map of ts, through optionals.
func elementTypes(ts []types.Type) []types.Type {
	var out []types.Type
	for _, t := range ts {
		switch x := unwrapOptional(t).Base().(type) {
		case *types.ListType:
			out = append(out, x.Elem)
		case *types.TableType:
			out = append(out, x.Elem)
		case *types.MapType:
			out = append(out, x.Value)
		}
	}
	return out
}
