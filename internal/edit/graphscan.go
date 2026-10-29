package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// hitKey groups the paths of one reference as Refs does (refvalues.go firstPaths), per target.
type hitKey struct {
	to   int
	span source.Span
	let  check.Object
}

// instance is an entry's collection instance as a let path reaches it (DECISIONS 315, 316).
type instance struct {
	coll  *types.Collection
	owner *value.Record
}

// graphScan is Graph's one walk of the lets, active and replaced values alike, as Refs walks
// them: every reference found, once per target and span, with its path's pattern and owner.
type graphScan struct {
	s      *Snapshot
	ns     *nodeSet
	refs   *refScan // its stated test and token cache
	w      *valueWalk
	frames frames
	pats   patterns
	groups map[hitKey]int
	judged map[int]bool // a group seen twice: whether its edge's path is final
	edges  []GraphEdge
	insts  map[*types.Collection][]instance
	buf    []int
}

func (s *Snapshot) newGraphScan(ns *nodeSet) *graphScan {
	sc := &graphScan{
		s: s, ns: ns, refs: &refScan{s: s, tokens: map[source.FileID]map[source.Span]bool{}},
		pats: newPatterns(), groups: map[hitKey]int{}, judged: map[int]bool{},
		insts: map[*types.Collection][]instance{},
	}
	sc.frames.trie = &ns.trie
	sc.frames.pats = &sc.pats
	sc.w = s.newWalk(sc.visit(RefValue), true)
	sc.w.onKey = sc.visit(RefKey)
	return sc
}

// visit is the walk's visit: a value or map key naming nodes is noted as kind for each.
func (sc *graphScan) visit(kind RefKind) visitFn {
	return func(r rootRef, v, _ value.Value, segs []Seg) bool {
		sc.frames.at(len(segs), v)
		if to := sc.targets(v); len(to) > 0 {
			sc.note(r, kind, v, segs, to)
		}
		return false
	}
}

// targets are the nodes v names, as target.isValue matches them (refs.go): a ref's entry by
// its collection, owner and key, or through a let path by the instance it reaches; a member.
func (sc *graphScan) targets(v value.Value) []int {
	sc.buf = sc.buf[:0]
	switch x := v.(type) {
	case *value.Ref:
		sc.refTargets(x)
	case *value.Member:
		if i, ok := sc.ns.byMember[nodeMember{enum: x.Enum, index: x.Index}]; ok {
			sc.buf = append(sc.buf, i)
		}
	}
	return sc.buf
}

// refTargets adds the entry x names by its collection, owner and key, and through a let path the
// entries of the instances it reaches with that key.
func (sc *graphScan) refTargets(x *value.Ref) {
	rt, ok := baseOf(x.T).(*types.RefType)
	if !ok {
		return
	}
	sc.add(nodeIdent{coll: rt.Target, owner: x.Owner, key: x.Key})
	if c := rt.Target; c == nil || c.Kind != types.CollLet || len(c.FieldPath) == 0 {
		return
	}
	for _, in := range sc.instance(rt.Target) {
		if in.coll != rt.Target {
			sc.add(nodeIdent{coll: in.coll, owner: in.owner, key: x.Key})
		}
	}
}

func (sc *graphScan) add(k nodeIdent) {
	if i, ok := sc.ns.byIdent[k]; ok && !slices.Contains(sc.buf, i) {
		sc.buf = append(sc.buf, i)
	}
}

// instance is the collection instances whose entries the let path c reaches, once each.
func (sc *graphScan) instance(c *types.Collection) []instance {
	if in, ok := sc.insts[c]; ok {
		return in
	}
	var out []instance
	for _, e := range letPathEntries(sc.s, c) {
		if rec, ok := e.(*value.Record); ok && rec.Ident != nil {
			if in := (instance{coll: rec.Ident.Coll, owner: rec.Ident.Owner}); !slices.Contains(out, in) {
				out = append(out, in)
			}
		}
	}
	sc.insts[c] = out
	return out
}

// note keeps v as Refs keeps it (refvalues.go note): stated or computed, a computed one at its
// let's name; one edge per target, the first path of each group, or its first structural one.
func (sc *graphScan) note(r rootRef, kind RefKind, v value.Value, segs []Seg, to []int) {
	p := v.Prov()
	if p == nil {
		return
	}
	keep, computed := sc.refs.stated(p)
	if !keep {
		return
	}
	key := hitKey{span: p.Span}
	if computed {
		key.span, key.let = nameSpan(r, p.Span), r.obj
	}
	f := sc.frames.reach(r, segs)
	path := ""
	for _, t := range to {
		key.to = t
		if i, dup := sc.groups[key]; dup {
			sc.again(i, r, kind, segs)
			continue
		}
		if path == "" {
			path = Path{Package: r.pkg.Path, Root: r.obj.Name(), Segs: segs}.String()
		}
		sc.groups[key] = len(sc.edges)
		sc.edges = append(sc.edges, GraphEdge{
			Path: path, Package: r.pkg.Path, Span: key.span, From: f.owner, To: t, Pattern: f.bare, Kind: kind, Computed: computed,
		})
	}
}

// again is a later path of an edge's group: it replaces the edge's when the edge's leaves its
// source tree and this one stays in its own (refvalues.go firstPaths, reachesSource).
func (sc *graphScan) again(i int, r rootRef, kind RefKind, segs []Seg) {
	final, judged := sc.judged[i]
	if final {
		return
	}
	if !judged {
		p, err := Parse(sc.edges[i].Path)
		_, kept := sc.s.structural(p)
		if sc.judged[i] = err == nil && kept; sc.judged[i] {
			return
		}
	}
	p := Path{Package: r.pkg.Path, Root: r.obj.Name(), Segs: segs}
	if _, ok := sc.s.structural(p); ok {
		f := sc.frames.reach(r, segs)
		e := &sc.edges[i]
		e.Path, e.Package, e.Kind, e.From, e.Pattern = p.String(), r.pkg.Path, kind, f.owner, f.bare
		sc.judged[i] = true
	}
}
