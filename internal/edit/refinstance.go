package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// sharedUse is a shared token, by the span the values it states carry, and what the instances
// taking it make of it: the paths where it names the target, and whether one names another entry
// of the target's collection (API.md R7; DECISIONS 483).
type sharedUse struct {
	span  source.Span
	at    []string
	other bool
}

// perInstance reports t a ref into the target's collection that each instance holds its own of:
// a record field's keyed list or table, whose same key names another entry in each instance.
func (tg *target) perInstance(t types.Type) (*types.RefType, bool) {
	rt, ok := baseOf(t).(*types.RefType)
	if !ok || rt.Target == nil || rt.Target.Kind != types.CollField || len(rt.Target.FieldPath) == 0 {
		return nil, false
	}
	return rt, tg.names(rt.Target)
}

// used notes v as a use of the shared token stating it: a per-instance ref a default, a constant,
// a function or a lambda states, which names the target in some instances and another entry in
// others; true when that token is the only reference, a computed value being also one (R7).
func (sc *refScan) used(r rootRef, v, parent value.Value, segs []Seg) bool {
	x, isRef := v.(*value.Ref)
	if !isRef || x.P == nil || x.P.Kind == value.ProvLiteral && sc.notToken(x.P) {
		return false // no token states it (an entry, a let or a field read): a computed value (R7)
	}
	if _, ok := sc.tg.perInstance(x.T); !ok {
		return false
	}
	only, shared := sc.sharedProv(x.P)
	if !shared {
		return false
	}
	i, ok := sc.useAt[x.P.Span]
	if !ok {
		i = len(sc.uses)
		sc.useAt[x.P.Span] = i
		sc.uses = append(sc.uses, sharedUse{span: x.P.Span})
	}
	if sc.names(v, parent) {
		sc.uses[i].at = append(sc.uses[i].at, Path{Package: r.pkg.Path, Root: r.obj.Name(), Segs: segs}.String())
	} else {
		sc.uses[i].other = true
	}
	return only
}

// sharedRegion is the source of a default, constant, function or lambda, whose tokens every
// instance it builds shares; only reports such a token the sole reference: a default's or a
// constant's, while a function's or a lambda's values are computed references too (R7).
type sharedRegion struct {
	span source.Span
	only bool
}

// sharedProv reports a provenance every instance taking it shares, a default's or one stated in a
// shared region, and whether its token is then the only reference (API.md R7; DECISIONS 483).
func (sc *refScan) sharedProv(p *value.Prov) (only, shared bool) {
	if p.Kind == value.ProvDefault {
		return true, true
	}
	if p.Kind != value.ProvLiteral && p.Kind != value.ProvComputed {
		return false, false
	}
	regions, ok := sc.regions[p.Span.File]
	if !ok {
		regions = sharedRegions(sc.s.files[p.Span.File])
		sc.regions[p.Span.File] = regions
	}
	for _, g := range regions {
		if contains(g.span, p.Span) {
			return g.only, true
		}
	}
	return false, false
}

// sharedRegions are a file's field defaults, constants, functions, methods and lambdas.
func sharedRegions(f *syntax.File) []sharedRegion {
	var out []sharedRegion
	if f == nil {
		return out
	}
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.FnDecl, *syntax.LambdaExpr:
			out = append(out, sharedRegion{span: f.Span(x)})
			return false
		case *syntax.ConstDecl:
			out = append(out, sharedRegion{span: f.Span(x), only: true})
			return false
		case *syntax.FieldDecl:
			if x.Default != nil {
				out = append(out, sharedRegion{span: f.Span(x.Default), only: true})
			}
			return false
		}
		return true
	})
	return out
}

// instanceHolds reports a per-instance ref token at sp naming the target in the instance it is
// bound to: the instance an amend line's value amends or its path above a key reaches (lineHolds,
// segHolds); a default's, constant's, function's or lambda's, some instance taking it (sharedHolds).
func (sc *nameScan) instanceHolds(tok syntax.Node, sp source.Span, at scanAt) bool {
	e, isExpr := tok.(syntax.Expr)
	if !isExpr {
		return true
	}
	rt, ok := sc.tg.perInstance(sc.s.info.Types[e])
	switch {
	case !ok:
		return true
	case at.line != nil && at.line.Value != nil && at.block != nil && contains(sc.file.Span(at.line.Value), sp):
		return sc.lineHolds(at, sp, rt)
	case at.line != nil && at.block != nil && tok == at.segKey:
		return sc.segHolds(at, rt)
	case at.field != nil && sc.transient(at):
		sc.refuse(sp) // a default some evaluation no walk stores takes (DECISIONS 483)
		return true
	case at.shared:
		return sc.sharedHolds(sp)
	}
	return true
}

// sharedHolds reports a shared token at sp naming the target in some instance taking it; one
// that also names another entry in another instance is kept in shared, which refuses a rename.
func (sc *nameScan) sharedHolds(sp source.Span) bool {
	held := false
	for _, u := range sc.uses {
		if len(u.at) == 0 || !contains(u.span, sp) {
			continue
		}
		held = true
		if u.other {
			sc.shared = append(sc.shared, u.at...)
		}
	}
	return held
}

// lineHolds reports a ref token typed rt at sp in an amendment's right-hand side naming the target:
// its bound instance, the nearest of rt's record, is built by the line's literal at the target's
// instance path, or met on the line's path holding the target's entry (API.md R7; DECISIONS 483).
func (sc *nameScan) lineHolds(at scanAt, sp source.Span, rt *types.RefType) bool {
	chain := chainTo(sc.file, syntax.Unparen(at.line.Value), sp)
	for i := len(chain) - 1; i > 0; i-- { // the token's enclosing nodes, innermost first
		if sc.s.recordOf(chain[i-1]) == rt.Target.Owner {
			return sc.builtHolds(at, braceOf(chain[i-1]), rt)
		}
	}
	return sc.pathHolds(at, rt)
}

// segHolds reports an amend segment's key typed rt, a ref into each instance's own collection,
// naming the target in the instance the line's path reaches above the segment, by identity: the
// nearest of rt's record (API.md R7; DECISIONS 483, 484, 487).
func (sc *nameScan) segHolds(at scanAt, rt *types.RefType) bool {
	i := slices.IndexFunc(at.line.Path, func(seg *syntax.AmendSegment) bool {
		holder, _ := segToken(seg)
		return holder == at.segKey
	})
	return i >= 0 && sc.prefixHolds(at, at.line.Path[:i], rt)
}

// builtHolds reports lit, the instance a line's literal builds, at the path of the target's
// instance in the target's root: the layered value holds the target there.
func (sc *nameScan) builtHolds(at scanAt, lit *syntax.BraceLit, rt *types.RefType) bool {
	home := sc.tg.res
	m := sc.tg.ownerSteps(rt)
	if lit == nil || m < 0 || sc.s.info.NameUses[at.block.Target] != home.root.obj {
		return false
	}
	home.Steps = home.Steps[:m]
	k, ok := sc.s.prefixOf(at.line.Path, home)
	if !ok {
		return false
	}
	node := syntax.Node(syntax.Unparen(at.line.Value))
	for _, st := range home.Steps[k:] {
		if node, ok = childNode(node, st); !ok {
			return false
		}
	}
	return braceOf(node) == lit
}

// pathHolds reports the nearest instance of rt's record on the line's path holding the target's
// entry; a path s does not resolve reaches none.
func (sc *nameScan) pathHolds(at scanAt, rt *types.RefType) bool {
	return sc.prefixHolds(at, at.line.Path, rt)
}

// prefixHolds reports the nearest instance of rt's record that segs, a line's first segments, read
// from the amended root holding the target's entry, by identity; a path s does not resolve reaches none.
func (sc *nameScan) prefixHolds(at scanAt, segs []*syntax.AmendSegment, rt *types.RefType) bool {
	obj := sc.s.info.NameUses[at.block.Target]
	if obj == nil || sc.s.scopedSegs(segs) {
		return false
	}
	p, err := Parse(sc.s.linePathOf(sc.file, Path{Package: obj.Pkg(), Root: obj.Name()}, segs))
	if err != nil {
		return false
	}
	res, err := sc.s.open(p)
	if err != nil {
		return false
	}
	owner := ownerOn(res, rt.Target.Owner)
	return owner != nil && sc.tg.holds(owner, rt)
}

// ownerOn is the last record of type owner res reads, its root's value included; nil for none.
func ownerOn(res resolution, owner *types.RecordType) *value.Record {
	for i := len(res.Steps); i >= 0; i-- {
		if rec, ok := res.parent(i).(*value.Record); ok && recordType(rec.T) == owner {
			return rec
		}
	}
	return nil
}

// placeIdent is the entry a per-instance ref x names in parent, the record holding it: an
// amendment's replaced value is judged in the record now at its place (API.md R7: base and layered
// refs merge by span); nil when parent is no instance of x's record or holds no such entry.
func placeIdent(x *value.Ref, parent value.Value) *value.Identity {
	rt, isRef := baseOf(x.T).(*types.RefType)
	owner, isRec := parent.(*value.Record)
	if !isRef || !isRec || rt.Target == nil || rt.Target.Kind != types.CollField || len(rt.Target.FieldPath) == 0 {
		return nil
	}
	i := fieldIndex(fieldsOf(owner.T), rt.Target.FieldPath[0])
	if recordType(owner.T) != rt.Target.Owner || i < 0 || i >= len(owner.Fields) {
		return nil
	}
	return keyedIdent(owner.Fields[i], x.Key)
}

// holds reports owner's field for rt holding the target's entry, judged by identity.
func (tg *target) holds(owner *value.Record, rt *types.RefType) bool {
	i := fieldIndex(fieldsOf(owner.T), rt.Target.FieldPath[0])
	if i < 0 || i >= len(owner.Fields) {
		return false
	}
	return tg.isEntry(keyedIdent(owner.Fields[i], tg.ident.Key))
}

// ownerSteps is the number of the target's steps reaching the instance whose field, rt's, holds
// it; -1 when the target's path does not end in that field and a key.
func (tg *target) ownerSteps(rt *types.RefType) int {
	m := len(tg.res.Steps) - 1 - len(rt.Target.FieldPath)
	if m < 0 {
		return -1
	}
	for _, st := range tg.res.Steps[m : len(tg.res.Steps)-1] {
		if st.Seg.Kind != SegField {
			return -1
		}
	}
	return m
}

// chainTo is the nodes from n down to the innermost one holding sp, n first; none when n does not.
func chainTo(f *syntax.File, n syntax.Node, sp source.Span) []syntax.Node {
	var out []syntax.Node
	for n != nil && contains(f.Span(n), sp) {
		out = append(out, n)
		var next syntax.Node
		for c := range syntax.Children(n) {
			if contains(f.Span(c), sp) {
				next = c
				break
			}
		}
		n = next
	}
	return out
}

// recordOf is the record type the checker gave expression n, nil for none.
func (s *Snapshot) recordOf(n syntax.Node) *types.RecordType {
	e, ok := n.(syntax.Expr)
	if !ok {
		return nil
	}
	return recordType(s.info.Types[e])
}

// recordType is the record t is, an applied record's own included; nil for another type.
func recordType(t types.Type) *types.RecordType {
	switch b := baseOf(t).(type) {
	case *types.RecordType:
		return b
	case *types.AppliedRecord:
		return b.Rec
	}
	return nil
}
