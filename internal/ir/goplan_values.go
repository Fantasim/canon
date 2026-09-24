package ir

import (
	"github.com/fantasim/canonlang/internal/types"
)

// isGoContainer reports a value whose Go form is a container class: a table or a keyed list (CODEGEN.md §5.9).
func isGoContainer(v *Value) bool {
	return v.Type.Kind == types.Table || v.Type.Kind == types.List && v.Type.KeyedBy != nil
}

// declareContainers declares the class of every emitted table and keyed list, its methods, and a table's FindBy<F> and their indexes (CODEGEN.md §5.9, decision 193); a value the emit leaves out has no container.
func (pl *GoNamePlan) declareContainers(top *goScope) {
	for _, v := range pl.emitted {
		if !isGoContainer(v) {
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
func (pl *GoNamePlan) declareValues(top *goScope) {
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
		if isGoContainer(v) {
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
		if isGoContainer(v) {
			pl.declare(data, pl.ValueStore(v), v.Name, v)
			continue
		}
		stores, _ := pl.ValueSlot(v).members()
		for _, n := range stores {
			pl.declare(data, n, v.Name, v)
		}
	}
}

// declareFns declares each stored package-level export fn and the variable of its table (CODEGEN.md §5.10, decision 183); a translated one, which baked gen/go refuses (decision 124), declares nothing yet.
func (pl *GoNamePlan) declareFns(top *goScope) {
	for _, fn := range pl.p.Fns {
		if fn.Kind == FnTranslated {
			continue
		}
		f := pl.Finite(fn)
		pl.declare(top, f.Name, fn.Name, fn)
		pl.declare(top, f.Store, fn.Name, fn)
		pl.declareParams(f.Store, fn.Name, fn)
	}
}

// declareImports declares the package names the generated file imports (CODEGEN.md §2.8): the standard ones gen/go writes when it needs them, then the imported Canon packages whose names it qualifies, each once per import path.
func (pl *GoNamePlan) declareImports(top *goScope) {
	u := pl.importUse()
	for _, std := range goStdImports {
		if u.std[std] {
			pl.declare(top, std, std, nil)
		}
	}
	seen := map[string]bool{}
	for _, ref := range pl.p.Imports {
		for _, e := range ref.Emits {
			if e.Target != TargetGo || !u.pkgs[ref.Name] || seen[e.GoImport] {
				continue
			}
			seen[e.GoImport] = true
			pl.declare(top, e.GoPackage, e.GoImport, nil)
		}
	}
}
