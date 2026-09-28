package ir

import "github.com/fantasim/canonlang/internal/types"

// classScope opens a class's scope; a member named like a type its body names hides it (CODEGEN.md §3.5).
func (pl *CppNamePlan) classScope(name, origin string, class any) *nameScope {
	sc := pl.scope(name)
	sc.hidden = map[string]string{name: origin}
	switch x := class.(type) {
	case *Variant:
		sc.hidden[pl.KindName(x)] = x.QName()
		for _, c := range casesWithFields(x) {
			sc.hidden[pl.className(c)] = pl.classOrigin(c)
		}
	case *Dependent:
		sc.hidden[pl.Dependent(x).Branch] = x.QName()
		for _, b := range x.Branches {
			pl.namedIn(&b.Type, sc.hidden)
		}
	default:
		fields, fns := classBody(class)
		for _, f := range fields {
			pl.namedIn(&f.Type, sc.hidden)
		}
		for _, fn := range fns {
			pl.namedIn(&fn.Result, sc.hidden)
			for _, p := range fn.Params {
				pl.namedIn(&p.Type, sc.hidden)
			}
		}
	}
	return sc
}

// namedIn adds the own types t names unqualified, itself or through elements, keys and ref targets.
func (pl *CppNamePlan) namedIn(t *TypeRef, out map[string]string) {
	if t == nil {
		return
	}
	if n := t.Named; n != nil && pkgOf(n) == pl.p.Name {
		out[pl.TypeName(n)] = n.QName()
	}
	if v, ok := t.Named.(*Variant); ok && t.Kind == types.Case && t.Case != nil && v.Pkg == pl.p.Name {
		out[pl.CaseName(v, t.Case)] = v.QName() + qnameSep + t.Case.Name
	}
	if t.Ref != nil && t.Ref.Elem != nil && pkgOf(t.Ref.Elem) == pl.p.Name {
		out[pl.TypeName(t.Ref.Elem)] = t.Ref.Elem.QName()
	}
	pl.namedIn(t.Elem, out)
	pl.namedIn(t.Key, out)
}

// snapshotHidden hides the snapshot's own name and each @reload value's type (CODEGEN.md §5.11).
func (pl *CppNamePlan) snapshotHidden(sc *nameScope) {
	sc.hidden = map[string]string{sc.what: pl.p.Name}
	for _, v := range pl.values {
		switch {
		case !v.Reload:
		case v.Type.Kind == types.Record:
			pl.namedIn(&v.Type, sc.hidden)
		default:
			sc.hidden[pl.ContainerName(v)] = pl.valueOrigin(v)
		}
	}
}

// containerScope opens a container's scope, hiding its own name and its rows' types (CODEGEN.md §5.9).
func (pl *CppNamePlan) containerScope(v *Value, name string) *nameScope {
	sc := pl.scope(name)
	sc.hidden = map[string]string{name: pl.valueOrigin(v)}
	pl.rowTypes(v, sc.hidden)
	return sc
}

// rowTypes adds a container's row record and its key and @stable fields' types (CODEGEN.md §5.9).
func (pl *CppNamePlan) rowTypes(v *Value, out map[string]string) {
	rec := containerRecord(v)
	if rec == nil {
		return
	}
	out[pl.TypeName(rec)] = rec.QName()
	for _, f := range rec.Fields {
		if findsBy(f) || v.Type.KeyedBy != nil && f.Name == v.Type.KeyedBy.Name {
			pl.namedIn(&f.Type, out)
		}
	}
}

// accessScope opens detail's access struct, hiding its own name and its loaders' types (CODEGEN.md §7.6).
func (pl *CppNamePlan) accessScope(access string) *nameScope {
	sc := pl.scope(access)
	sc.hidden = map[string]string{access: pl.p.Name}
	for _, v := range pl.values {
		pl.namedIn(&v.Type, sc.hidden)
		if v.Type.Kind != types.Record {
			sc.hidden[pl.ContainerName(v)] = pl.valueOrigin(v)
			pl.rowTypes(v, sc.hidden)
		}
	}
	if pl.reloads() {
		sc.hidden[pl.SnapshotName()] = pl.p.Name
	}
	for _, c := range pl.classes() {
		if pl.walks(c, map[any]bool{}) {
			sc.hidden[pl.className(c)] = pl.classOrigin(c)
		}
	}
	return sc
}

// signatureTypes are the own types fn's C++ signature and conformance vector name.
func (pl *CppNamePlan) signatureTypes(fn *ExportFn) map[string]string {
	out := map[string]string{}
	pl.namedIn(&fn.Result, out)
	for _, r := range fn.Reads {
		pl.namedIn(&r.Type, out)
	}
	for _, p := range fn.Params {
		pl.namedIn(&p.Type, out)
	}
	return out
}

// checkSignatures checks each export fn's parameter list, the methods' then the package fns'.
func (pl *CppNamePlan) checkSignatures() {
	for _, c := range pl.classes() {
		_, fns := classBody(c)
		for _, fn := range fns {
			pl.checkSignature(pl.classOrigin(c)+qnameSep+fn.Name, fn)
		}
	}
	for _, fn := range pl.p.Fns {
		pl.checkSignature(pl.fnOrigin(fn), fn)
	}
}

// checkSignature is E8005 at a parameter of fn named like a type its signature names (CODEGEN.md §3.5).
func (pl *CppNamePlan) checkSignature(origin string, fn *ExportFn) {
	named := pl.signatureTypes(fn)
	params := &nameScope{what: origin + goParamsSuffix}
	for _, r := range fn.Reads {
		if first, ok := named[cppVerbatim(r.Name)]; ok {
			pl.collide(params, cppVerbatim(r.Name), first, origin+qnameSep+r.Name, fn)
		}
	}
	for _, p := range fn.Params {
		if first, ok := named[cppVerbatim(p.Name)]; ok {
			pl.collide(params, cppVerbatim(p.Name), first, origin+qnameSep+p.Name, p)
		}
	}
}
