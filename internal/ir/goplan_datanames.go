package ir

import (
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// GoSnapshot are the names of a data-mode package's snapshot and store (CODEGEN.md §3.3, §5.11): <P>Snapshot, Load<P>Snapshot, <P>Store and its variable Store.
type GoSnapshot struct {
	Type, Load, Store, Var string
}

// SchemaName is the schema constant of an emitted value, UpperCamel(v) + Schema (CODEGEN.md §3.3, T2).
func (pl *GoNamePlan) SchemaName(v *Value) string { return pl.ContainerName(v) + goSchemaSuffix }

// LoaderName is data mode's Load<V>, or the value's whole @go(name:) override (CODEGEN.md §3.3, §3.5).
func (pl *GoNamePlan) LoaderName(v *Value) string {
	if v.Go.Name != "" {
		return v.Go.Name
	}
	return goLoadPrefix + pl.ContainerName(v)
}

// LoadFunc is the unexported reader of a value's data file, load<V> (CODEGEN.md §6.1).
func (pl *GoNamePlan) LoadFunc(v *Value) string { return goLoadLocalPrefix + pl.ContainerName(v) }

// DecodeFunc is a decoded class's decode<T> (CODEGEN.md §6.1); class is an own *Record, *Variant, *Case or *Dependent.
func (pl *GoNamePlan) DecodeFunc(class any) string { return goDecodePrefix + pl.goNameOf[class] }

// ResolveFunc is the function resolving a class's refs after a load (CODEGEN.md §5.8).
func (pl *GoNamePlan) ResolveFunc(class any) string { return goResolvePrefix + pl.goNameOf[class] }

// Snapshot are the snapshot's names, P the UpperCamel of the package's last segment (CODEGEN.md §3.3).
func (pl *GoNamePlan) Snapshot() GoSnapshot {
	segs := strings.Split(pl.p.Name, qnameSep)
	upper := goUpperCamel(segs[len(segs)-1])
	return GoSnapshot{Type: upper + goSnapshotSuffix, Load: goLoadPrefix + upper + goSnapshotSuffix, Store: upper + goStoreSuffix, Var: goStoreSuffix}
}

// DataLocal is a fixed local of data mode's loaders, escaped once when an imported Canon package is named so (CODEGEN.md §3.4, decision 182); the plan reports the escaped name that is an import too.
func (pl *GoNamePlan) DataLocal(name string) string {
	local, _ := pl.local(name)
	return local
}

// declareSchemas declares each emitted value's schema constant (CODEGEN.md §3.3, T2).
func (pl *GoNamePlan) declareSchemas(top *nameScope) {
	for _, v := range pl.emitted {
		pl.declare(top, pl.SchemaName(v), v.Name, v)
	}
}

// declareDataContainers declares data mode's table and keyed-list classes: rows, a table's FindBy indexes as members, the methods (CODEGEN.md §5.3, §5.9; log-2026-09-24 "gen/go data mode").
func (pl *GoNamePlan) declareDataContainers(top *nameScope) {
	for _, v := range pl.emitted {
		rec := tableRecord(v)
		if !isGoContainer(v) || v.Type.Kind == types.Table && rec == nil {
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
		for _, f := range rec.Fields {
			if f.Stable {
				pl.declare(sc, pl.FindByName(f), name+qnameSep+f.Name, f)
				pl.declare(sc, pl.FindByIndex(v, f), name+qnameSep+f.Name, f)
			}
		}
	}
}

// declareLoaders declares Load<V> of each emitted value that is not @reload (CODEGEN.md §5.9).
func (pl *GoNamePlan) declareLoaders(top *nameScope) {
	for _, v := range pl.emitted {
		if goRootClass(v) != nil && !v.Reload {
			pl.declare(top, pl.LoaderName(v), v.Name, v)
		}
	}
}

// declareSnapshot declares the snapshot (a member and a getter per @reload value), its loader, the store (current, Current, Reload) and its variable (CODEGEN.md §5.11, T4–T6, T9).
func (pl *GoNamePlan) declareSnapshot(top *nameScope) {
	var reload []*Value
	for _, v := range pl.emitted {
		if v.Reload && goRootClass(v) != nil {
			reload = append(reload, v)
		}
	}
	if len(reload) == 0 {
		return
	}
	names := pl.Snapshot()
	pl.declare(top, names.Type, pl.p.Name, nil)
	snap := pl.scope(names.Type)
	for _, v := range reload {
		pl.declare(snap, pl.ContainerName(v), v.Name, v)
	}
	for _, v := range reload {
		pl.declare(snap, pl.ValueStore(v), v.Name, v)
	}
	pl.declare(top, names.Load, pl.p.Name, nil)
	pl.declare(top, names.Store, pl.p.Name, nil)
	store := pl.scope(names.Store)
	for _, n := range goStoreMembers {
		pl.declare(store, n, names.Store, nil)
	}
	pl.declare(top, names.Var, pl.p.Name, nil)
}

// declareDecoders declares the shared JSON helpers, jsonRowID with a table, then decode<T> of each decoded class in declaration order (CODEGEN.md §6.1; log-2026-09-24 "Loader parity").
func (pl *GoNamePlan) declareDecoders(top *nameScope) {
	if len(pl.data.decoded) == 0 {
		return
	}
	for _, h := range goJSONHelpers {
		pl.declare(top, h, pl.p.Name, nil)
	}
	if pl.emitsTable() {
		pl.declare(top, goJSONRowID, pl.p.Name, nil)
	}
	for _, class := range pl.classes() {
		if pl.Decoded(class) {
			pl.declare(top, pl.DecodeFunc(class), pl.goNameOf[class], class)
		}
	}
	pl.declareDependentDecoders(top)
}

// classes are the package's records, and each variant followed by its cases with fields, in declaration order.
func (pl *GoNamePlan) classes() []any { return packageClasses(pl.p) }

func (pl *GoNamePlan) emitsTable() bool {
	for _, v := range pl.emitted {
		if v.Type.Kind == types.Table {
			return true
		}
	}
	return false
}

// classBody is a record's or case's fields and export fns.
func classBody(class any) ([]*Field, []*ExportFn) {
	switch x := class.(type) {
	case *Record:
		return x.Fields, x.Methods
	case *Case:
		return x.Fields, x.Methods
	}
	return nil, nil
}

// declareResolvers declares resolve<T> of each decoded class a load resolves, in declaration order (CODEGEN.md §5.8).
func (pl *GoNamePlan) declareResolvers(top *nameScope) {
	for _, class := range pl.classes() {
		if pl.NeedsWalk(class) {
			pl.declare(top, pl.ResolveFunc(class), pl.goNameOf[class], class)
		}
	}
}

// declareLoads declares load<V> of each emitted value (CODEGEN.md §6.1).
func (pl *GoNamePlan) declareLoads(top *nameScope) {
	for _, v := range pl.emitted {
		if goRootClass(v) != nil {
			pl.declare(top, pl.LoadFunc(v), v.Name, v)
		}
	}
}

// declareDataLocals declares the fixed locals of data mode's loaders, each escaped once (CODEGEN.md §3.4, decision 182): an escaped name that is an import too is E8005.
func (pl *GoNamePlan) declareDataLocals() {
	sc := pl.scope(goScopeLocals)
	for _, n := range goDataLocals {
		pl.declareFixedLocal(sc, n)
	}
}

// declareFixedLocal declares a fixed local of generated code, escaped once when an imported Canon package is named so (CODEGEN.md §3.4, decision 182): an escaped name that is an import too is E8005.
func (pl *GoNamePlan) declareFixedLocal(sc *nameScope, name string) {
	local, ok := pl.local(name)
	if !ok {
		pl.problems = append(pl.problems, GoNameProblem{Kind: GoCollision, Scope: sc.what, Name: local, First: pl.imports[local], Origin: pl.p.Name})
	}
	pl.declare(sc, local, pl.p.Name, nil)
}
