package edit

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/value"
)

// ValueRefs is what Refs finds for r's target among values and map keys, in R8 order, and
// whether code computed each (API.md R7): the list Graph's edges to it must equal (R11).
func ValueRefs(ctx context.Context, s *Snapshot, r Resolved) ([]Ref, []bool, error) {
	res, err := s.reopen(r)
	if err != nil {
		return nil, nil, err
	}
	tg, ok := s.target(res)
	if !ok {
		return nil, nil, ErrBadOp
	}
	sc := s.newScan(tg)
	if err := sc.lets(ctx); err != nil {
		return nil, nil, err
	}
	all := s.firstPaths(sc.found)
	slices.SortFunc(all, func(a, b found) int { return s.refOrder(a.ref, b.ref) })
	refs, computed := make([]Ref, len(all)), make([]bool, len(all))
	for i, f := range all {
		refs[i], computed[i] = f.ref, f.computed
	}
	return refs, computed, nil
}

// Targets is every identity Refs accepts as a target at a path a walk of the lets' values
// reaches, with the first such path (API.md R7, R11): the values Graph must make nodes of.
func Targets(s *Snapshot) map[any]string {
	out := map[any]string{}
	for _, pkg := range s.pkgs {
		for _, obj := range pkg.Decls {
			r := rootRef{pkg: pkg, obj: obj}
			if v, err := s.force(r); err == nil && obj.Kind() == check.ObjLet {
				s.targetsBelow(Path{Package: pkg.Path, Root: obj.Name()}, v, out)
			}
		}
	}
	return out
}

func (s *Snapshot) targetsBelow(at Path, v value.Value, out map[any]string) {
	for _, c := range children(v) {
		p := Path{Package: at.Package, Root: at.Root, Segs: append(slices.Clip(at.Segs), c.seg)}
		if k, ok := s.identityAt(p, c.v); ok {
			if _, seen := out[k]; !seen {
				out[k] = p.String()
			}
		}
		s.targetsBelow(p, c.v, out)
	}
}

// identityAt is the identity of v at p when Refs accepts p as a target and Resolve reads v there.
func (s *Snapshot) identityAt(p Path, v value.Value) (any, bool) {
	res, err := s.open(p)
	if err != nil || res.Target != v {
		return nil, false
	}
	if tg, ok := s.target(res); ok && tg.ident != nil {
		return nodeIdent{coll: tg.ident.Coll, owner: tg.ident.Owner, key: tg.ident.Key}, true
	}
	return nil, false
}

// IdentityAt is the identity of the entry or element at the path id, false for none.
func IdentityAt(s *Snapshot, id string) (any, bool) {
	p, err := Parse(id)
	if err != nil {
		return nil, false
	}
	res, err := s.open(p)
	if err != nil {
		return nil, false
	}
	return s.identityAt(p, res.Target)
}
