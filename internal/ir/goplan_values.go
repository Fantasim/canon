package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// IsContainer reports a value whose Go and C++ form is a container class: a table or a keyed list (CODEGEN.md §5.9).
func IsContainer(v *Value) bool {
	return v.Type.Kind == types.Table || v.Type.Kind == types.List && v.Type.KeyedBy != nil
}

// declareContainers declares the class of every emitted table and keyed list, its methods, and a table's FindBy<F> and their indexes (CODEGEN.md §5.9, decision 193); a value the emit leaves out has no container.
func (pl *GoNamePlan) declareContainers(top *nameScope) {
	for _, v := range pl.emitted {
		if !IsContainer(v) {
			continue
		}
		rec := tableRecord(v)
		if v.Type.Kind == types.Table && rec == nil {
			continue
		}
		name := pl.ContainerName(v)
		pl.declare(top, name, v.Name, v)
		sc := pl.scope(name)
		for _, m := range goContainerMembers {
			pl.declare(sc, m, name, v)
		}
		if rec == nil {
			continue
		}
		pl.declare(sc, GoGet, name, v)
		for _, f := range rec.Fields {
			if f.Stable {
				pl.declare(top, pl.FindByIndex(v, f), name+qnameSep+f.Name, f)
				pl.declare(sc, pl.FindByName(f), name+qnameSep+f.Name, f)
			}
		}
	}
}

// declareValues declares the baked data, each emitted value's accessors (CODEGEN.md §5.9, §6.2): Get<V>, or a slot's getters for a value that is no container, then the data's members, so a cause is reported at its getter (decision 203).
func (pl *GoNamePlan) declareValues(top *nameScope) {
	if len(pl.emitted) == 0 {
		return
	}
	d := pl.Data()
	for _, n := range []string{d.Type, d.Values, d.Build} {
		pl.declare(top, n, pl.p.Name, nil)
	}
	if local, ok := pl.local(goDataLocal); !ok {
		pl.problems = append(pl.problems, GoNameProblem{Kind: GoCollision, Scope: d.Build, Name: local, First: pl.imports[local], Origin: pl.p.Name})
	}
	for _, v := range pl.emitted {
		if IsContainer(v) {
			pl.declare(top, pl.AccessorName(v), v.Name, v)
			continue
		}
		_, getters := pl.ValueSlot(v).members()
		for _, n := range getters {
			pl.declare(top, n, v.Name, v)
		}
	}
	data := pl.scope(d.Type)
	for _, v := range pl.emitted {
		if IsContainer(v) {
			pl.declare(data, pl.ValueStore(v), v.Name, v)
			continue
		}
		stores, _ := pl.ValueSlot(v).members()
		for _, n := range stores {
			pl.declare(data, n, v.Name, v)
		}
	}
}

// declareFns declares each package-level export fn in declaration order: a translated one's function, public and pure at once, and its names (CODEGEN.md §5.10); a stored one's function and the variable of its table (decision 183), which data mode has none of (E8013).
func (pl *GoNamePlan) declareFns(top *nameScope) {
	for _, fn := range pl.p.Fns {
		origin := pl.fnOrigin(fn)
		switch {
		case fn.Kind == FnTranslated:
			pl.declarePure(top, origin, fn, goStruct{})
		case pl.data == nil:
			f := pl.Finite(fn)
			pl.declare(top, f.Name, origin, fn)
			pl.declare(top, f.Store, origin, fn)
			pl.declareParams(f.Store, origin, fn)
		}
	}
}

// declareImports declares the package names the generated file imports (CODEGEN.md §2.8): the standard ones gen/go writes when it needs them, the imported Canon packages whose names it qualifies, then the other packages' rt it passes (`<gopkg>rt`).
func (pl *GoNamePlan) declareImports(top *nameScope) {
	pl.declareUsed(top, pl.importUse())
	pl.declareRTImports(top)
}

// declareUsed declares, in sc, the standard packages u marks, then the imported Canon packages it marks, each once per import path: of a package's copies, the first, since copies share their package name (CODEGEN.md §2.1, DECISIONS 229) and a generator sees only the copy it imports (CopyOf).
func (pl *GoNamePlan) declareUsed(sc *nameScope, u *goImportUse) {
	for _, std := range goStdImports {
		if u.std[std] {
			pl.declare(sc, std, std, nil)
		}
	}
	seen := map[string]bool{}
	for _, ref := range pl.p.Imports {
		i := slices.IndexFunc(ref.Emits, func(e *Emit) bool { return e.Target == TargetGo })
		if i < 0 || !u.pkgs[ref.Name] || seen[ref.Emits[i].GoImport] {
			continue
		}
		e := ref.Emits[i]
		seen[e.GoImport] = true
		pl.declare(sc, e.GoPackage, e.GoImport, nil)
	}
}
