package ir

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// view is u's IR as the generator of emit es sees it, each import narrowed to the copy es uses (CopyOf, CODEGEN.md §2.8): what stage E judges of es is what its generator writes.
func (s *stage) view(u *unit, es *emitSite) *Package { return CopyOf(s.in.Project, u.p, es.e) }

// textImports are the packages, not in have, that a @text result reaches when one copy of a types-mode code emit of u writes its Decode<Fn>File (DECISIONS 340, 363), listed with their emits of the targets decoding it. Decoders are decided first with every package a readable result reaches imported (a fixpoint of one step: a result's decodability reads only the imports of what it reaches), then only the decided results' packages stay: a result going without a decoder changes nothing in the generated code, its names included. A package found here is never a use E8004 or E8007 judges.
func (s *stage) textImports(u *unit, have []*PackageRef) []*PackageRef {
	u.textOnly = map[string]bool{}
	sites := slices.DeleteFunc(slices.Clone(u.emits), func(es *emitSite) bool { return !isCode(es.e.Target) || es.e.Mode != ModeTypes })
	if len(sites) == 0 {
		return nil
	}
	listed := map[string]bool{u.p.Name: true}
	for _, r := range have {
		listed[r.Name] = true
	}
	var tentative []*PackageRef
	for _, pkg := range slices.Sorted(maps.Keys(textReached(u.p, readableResults(u.p)))) {
		if dep := s.units[pkg]; dep != nil && !listed[pkg] {
			tentative = append(tentative, &PackageRef{Name: pkg, Dir: dep.p.Dir, Emits: targetEmits(dep.p.Emits, sites)})
		}
	}
	decided := s.decidedReach(u, sites, append(slices.Clone(have), tentative...))
	return slices.DeleteFunc(tentative, func(r *PackageRef) bool {
		u.textOnly[r.Name] = decided[r.Name]
		return !decided[r.Name]
	})
}

// decidedReach are the packages the results reach that some copy of the go types emit decodes, with imports as u's imports.
func (s *stage) decidedReach(u *unit, sites []*emitSite, imports []*PackageRef) map[string]bool {
	saved := u.p.Imports
	u.p.Imports = imports
	var fns []*ExportFn
	for _, es := range sites {
		fns = append(fns, TextDecoded(s.view(u, es), es.e)...)
	}
	u.p.Imports = saved
	return textReached(u.p, fns)
}

// targetEmits are the emits, every copy, of the targets of sites.
func targetEmits(emits []*Emit, sites []*emitSite) []*Emit {
	return slices.DeleteFunc(slices.Clone(emits), func(e *Emit) bool {
		return !slices.ContainsFunc(sites, func(es *emitSite) bool { return es.e.Target == e.Target })
	})
}

// readableResults are the @text fns of p whose result is a map or a list readable at the top of a file (textReadable).
func readableResults(p *Package) []*ExportFn {
	return slices.DeleteFunc(slices.Clone(p.TextFns), func(fn *ExportFn) bool {
		t := TextResult(fn)
		return t.Kind != types.Map && t.Kind != types.List || !textReadable(p, t)
	})
}

// textReached are the packages but p whose types or collections the results of fns hold, at any depth.
func textReached(p *Package, fns []*ExportFn) map[string]bool {
	out := map[string]bool{}
	w := newWalker(nil, func(t *TypeRef) {
		if t.Named != nil {
			out[pkgOf(t.Named)] = true
		}
		if t.Ref != nil && t.Ref.Coll == types.CollLet {
			out[t.Ref.Pkg] = true
		}
	})
	for _, fn := range fns {
		t := TextResult(fn)
		w.ref(&t)
	}
	delete(out, "")
	delete(out, p.Name)
	return out
}
