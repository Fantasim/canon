package ir

import (
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// TSImport is one namespace import of a TypeScript file (log-2026-10-06 "U4 (gen/ts) done" (a)): another package's names are written qualified, `Alias.Name`; gen/ts chooses `import type * as` or `import * as` from what it writes ("U1 rounds 3-5").
type TSImport struct {
	Pkg, Alias string
}

// TSImportAlias is the namespace alias of package pkg in every TypeScript file: its path's segments joined by `_` (`game.core` gives game_core), with `_` added to a reserved word (`default` gives default_, CODEGEN.md §3.4).
func TSImportAlias(pkg string) string { return tsEscape(strings.ReplaceAll(pkg, qnameSep, underscore)) }

// TSImports are the packages a ts emit of p imports, one namespace each, in the order of p.Imports: those whose types the file reaches (tsReached; CODEGEN.md §2.8, DECISIONS 279(b), 323).
func TSImports(p *Package) []TSImport {
	reached := map[string]bool{}
	tsReached(p, func(t *TypeRef) {
		if t.Named != nil && pkgOf(t.Named) != p.Name {
			reached[pkgOf(t.Named)] = true
		}
		if r := t.Ref; t.Kind == types.Ref && r != nil && r.Pkg != p.Name && tsTableRef(r) {
			reached[r.Pkg] = true
		}
	})
	var out []TSImport
	for _, ref := range p.Imports {
		if reached[ref.Name] && tsEmitOf(p, ref.Name) != nil {
			out = append(out, TSImport{Pkg: ref.Name, Alias: TSImportAlias(ref.Name)})
		}
	}
	return out
}

// declareImports declares each namespace alias the file imports, before the package's own names, so a name of the package meeting one is E8005 at that name's declaration (CODEGEN.md §3.5); imported names live in their namespace, never in the module's scope. In data and types mode it declares this file's own reader of each class of another package it reaches, read_<alias>_<T> (§2.8).
func (pl *tsNamePlan) declareImports(mod *nameScope) {
	for _, imp := range TSImports(pl.p) {
		pl.declare(mod, imp.Alias, imp.Pkg, nil) // two packages giving one alias: E8005 at the emit (log-2026-10-06 "U1 rounds 3-5" 3)
	}
	named := map[Type]bool{}
	tsReached(pl.p, func(t *TypeRef) {
		if t.Named != nil && pkgOf(t.Named) != pl.p.Name && !named[t.Named] {
			named[t.Named] = true
			pl.declareForeignReaders(mod, t.Named)
		}
	})
}

// tsReached calls visit on every type a code file of p may write a name of (DECISIONS 279(b), 323): those its declarations hold, then, followed through the fields, cases, export fns and dependent branches of every type they reach, of any package, those its literals, decoders and translated reads write.
func tsReached(p *Package, visit func(*TypeRef)) {
	newWalker(nil, visit).pkgRefs(p)
}

// reachImports are the packages the code emits of u reach only through other packages' types (CODEGEN.md §2.8; DECISIONS 279(b), 323): their readers and literals may build any of their values, so each is listed with its emits of the code targets u emits, the generators that read them; u.reach keeps the first type of each reached, for E8004.
func (s *stage) reachImports(u *unit) []*PackageRef {
	u.reach = map[string]string{}
	code := map[Target]bool{}
	for _, es := range u.emits {
		code[es.e.Target] = isCode(es.e.Target)
	}
	if !code[TargetGo] && !code[TargetCpp] && !code[TargetTS] {
		return nil
	}
	reach := func(pkg, name string) {
		_, direct := u.firstUse[pkg]
		if _, seen := u.reach[pkg]; !seen && !direct && pkg != u.p.Name {
			u.reach[pkg] = name
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
	for _, name := range slices.Sorted(maps.Keys(u.reach)) {
		if dep := s.units[name]; dep != nil {
			emits := slices.DeleteFunc(slices.Clone(dep.p.Emits), func(e *Emit) bool { return !code[e.Target] })
			out = append(out, &PackageRef{Name: name, Dir: dep.p.Dir, Emits: emits})
		}
	}
	return out
}

// TSReaderName is a TypeScript file's reader of a record, variant or dependent type t of another package, read_<alias>_<T> (`read_game_core_LevelRange`); a package's own readers keep read<T> (log-2026-10-06 "TS reader names").
func TSReaderName(t Type) string { return tsForeignRead(pkgOf(t), tsName(t)) }

// TSCaseReaderName is the reader of case c of another package's variant v, read_<alias>_<V><Case>.
func TSCaseReaderName(v *Variant, c *Case) string { return tsForeignRead(v.Pkg, tsCaseName(v, c)) }

// tsForeignRead is read_<alias of pkg>_<name>.
func tsForeignRead(pkg, name string) string {
	return tsReadPrefix + underscore + TSImportAlias(pkg) + underscore + name
}

// declareForeignReaders declares, in data and types mode, the reader this file writes for a class of another package (TSReaderName), and for each of its cases that has a type (CODEGEN.md §2.8, §8.1; DECISIONS 323): it never calls that package's decoders.
func (pl *tsNamePlan) declareForeignReaders(mod *nameScope, t Type) {
	if _, enum := t.(*Enum); enum || pl.e.Mode != ModeData && pl.e.Mode != ModeTypes {
		return
	}
	pl.declare(mod, TSReaderName(t), t.QName(), nil)
	if v, ok := t.(*Variant); ok {
		for _, c := range v.Cases {
			if len(c.Fields) > 0 || len(c.Methods) > 0 {
				pl.declare(mod, TSCaseReaderName(v, c), v.QName()+qnameSep+c.Name, nil)
			}
		}
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
