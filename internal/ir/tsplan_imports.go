package ir

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// declareImports declares the names gen/ts may import from another package for each of its types the file reaches (tsReached; CODEGEN.md §2.8, §3.5, DECISIONS 278, 279(b)): the type, an enum's Members, Names, Index and Codes, a variant's kind union and case types, the public decoder of a types-mode emit's record or variant, a table's id type and its Index. A type's whole family is reserved, imported or not, as the helpers are, so a field or a value added later never breaks a valid name.
func (pl *tsNamePlan) declareImports(mod *nameScope) {
	named, tables := map[Type]bool{}, map[Type]bool{}
	tsReached(pl.p, func(t *TypeRef) {
		if t.Named != nil && pkgOf(t.Named) != pl.p.Name && !named[t.Named] {
			named[t.Named] = true
			pl.declareForeign(mod, t.Named)
		}
		if r := t.Ref; t.Kind == types.Ref && r != nil && r.Pkg != pl.p.Name && tsTableRef(r) && !tables[r.Elem] {
			tables[r.Elem] = true
			pl.declareForeignIDs(mod, r)
		}
	})
}

// tsReached calls visit on every type a TypeScript file of p may write a name of (DECISIONS 279(b)): those its declarations hold, then, followed through the fields, cases, export fns and dependent branches of every type they reach, of any package, those its literals, decoders and translated reads write.
func tsReached(p *Package, visit func(*TypeRef)) {
	newWalker(nil, visit).pkgRefs(p)
}

// tsImports are the packages a ts emit of u reaches only through other packages' types (CODEGEN.md §2.8, DECISIONS 279(b)): the file imports every package whose names it writes, so each is listed with its ts emits alone, the only generator that reads them; u.tsReach keeps the first type of each reached, for E8004.
func (s *stage) tsImports(u *unit) []*PackageRef {
	u.tsReach = map[string]string{}
	if emitFor(u, TargetTS) == nil {
		return nil
	}
	reach := func(pkg, name string) {
		_, direct := u.firstUse[pkg]
		if _, seen := u.tsReach[pkg]; !seen && !direct && pkg != u.p.Name {
			u.tsReach[pkg] = name
		}
	}
	tsReached(u.p, func(t *TypeRef) {
		if t.Named != nil {
			reach(pkgOf(t.Named), t.Named.QName())
		}
		if t.Ref != nil && t.Ref.Coll == types.CollLet {
			reach(t.Ref.Pkg, t.Ref.Pkg+qnameSep+t.Ref.Value)
		}
	})
	var out []*PackageRef
	for _, name := range slices.Sorted(maps.Keys(u.tsReach)) {
		if dep := s.units[name]; dep != nil {
			emits := slices.DeleteFunc(slices.Clone(dep.p.Emits), func(e *Emit) bool { return e.Target != TargetTS })
			out = append(out, &PackageRef{Name: name, Dir: dep.p.Dir, Emits: emits})
		}
	}
	return out
}

// declareForeign declares the names gen/ts may import for a type of another package.
func (pl *tsNamePlan) declareForeign(mod *nameScope, t Type) {
	name, org := tsName(t), t.QName()
	pl.declare(mod, name, org, nil)
	switch x := t.(type) {
	case *Enum:
		for _, suffix := range []string{tsMembersName, tsNamesName, tsIndexName} {
			pl.declare(mod, name+suffix, org, nil)
		}
		if x.Codes != nil {
			pl.declare(mod, name+tsCodesName, org, nil)
		}
	case *Variant:
		pl.declare(mod, name+tsKindSuffix, org, nil)
		for _, c := range x.Cases {
			if len(c.Fields) > 0 || len(c.Methods) > 0 {
				pl.declare(mod, tsCaseName(x, c), org+qnameSep+c.Name, nil)
			}
		}
	}
	if e := tsEmitOf(pl.p, pkgOf(t)); e != nil && e.Mode == ModeTypes && (isVariant(t) || isRecord(t)) {
		pl.declare(mod, tsDecodePrefix+name, org, nil)
	}
}

// declareForeignIDs declares the id type of another package's table, and its Index in a baked or embedded emit (CODEGEN.md §5.3).
func (pl *tsNamePlan) declareForeignIDs(mod *nameScope, r *RefTarget) {
	e := tsEmitOf(pl.p, r.Pkg)
	if e == nil || e.Mode == ModeTypes {
		return
	}
	id, org := tsName(r.Elem)+tsIDSuffix, r.Pkg+qnameSep+r.Value
	pl.declare(mod, id, org, nil)
	if e.Mode != ModeData {
		pl.declare(mod, id+tsIndexName, org, nil)
	}
}

// tsEmitOf is the ts emit of a package p imports, nil when it has none (E8004's).
func tsEmitOf(p *Package, pkg string) *Emit {
	for _, imp := range p.Imports {
		if imp.Name != pkg {
			continue
		}
		for _, e := range imp.Emits {
			if e.Target == TargetTS {
				return e
			}
		}
	}
	return nil
}

// tsTableRef reports a ref into a public table value: gen/ts types its key as that table's id type.
func tsTableRef(r *RefTarget) bool {
	return r.Coll == types.CollLet && !r.Local && !r.Keyed && r.Elem != nil
}

func isRecord(t Type) bool {
	_, ok := t.(*Record)
	return ok
}
