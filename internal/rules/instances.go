package rules

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// Instances runs stage C over one top-level value, in forced-set order (EVALUATION.md §8.1).
func (r *Runner) Instances(ctx context.Context, root eval.Root, v value.Value) error {
	bag, err := r.bagOf(root.Pkg)
	if err != nil {
		return err
	}
	t := &traversal{Runner: r, ctx: ctx, bag: bag}
	at := verify.Root(root.Name)
	t.index(v, at)
	t.visit(v, at)
	return nil
}

// traversal is one Instances run.
type traversal struct {
	*Runner
	ctx context.Context
	bag *diag.Bag
}

// visit walks v depth-first, pre-order: an instance's checks run before its parts are visited.
// It follows fields, elements, map keys then values, and entries, never refs.
func (t *traversal) visit(v value.Value, at *verify.Path) {
	if v == nil || t.ctx.Err() != nil {
		return
	}
	rec, isRecord := v.(*value.Record)
	if isRecord {
		if t.seen[rec] {
			return
		}
		t.seen[rec] = true
		if !t.invalidBelow(rec) && !t.brokenType(rec.T) {
			t.runChecks(rec, at)
		}
	}
	for _, p := range parts(v, at) {
		t.visit(p.v, p.at)
	}
}

// index records where each value of the subtree is first reached, the path of its findings.
func (t *traversal) index(v value.Value, at *verify.Path) {
	if _, known := t.paths[v]; known || v == nil {
		return
	}
	t.paths[v] = at
	for _, p := range parts(v, at) {
		t.index(p.v, p.at)
	}
}

// part is a value inside another, with its path.
type part struct {
	v  value.Value
	at *verify.Path
}

// parts is what the traversal and the invalid test follow, in the order of EVALUATION.md §8.1.
func parts(v value.Value, at *verify.Path) []part {
	switch x := v.(type) {
	case *value.Record:
		return recordParts(x, at)
	case *value.List:
		return listParts(x, at)
	case *value.Map:
		return mapParts(x, at)
	case *value.Table:
		return tableParts(x, at)
	case *value.Pair:
		return []part{{x.A, at}, {x.B, at}}
	}
	return nil
}

func recordParts(r *value.Record, at *verify.Path) []part {
	var out []part
	for i, f := range verify.Fields(r.T) {
		if i < len(r.Fields) {
			out = append(out, part{r.Fields[i], at.Field(f.Name)})
		}
	}
	return out
}

// listParts are a list's elements; a keyed list's are named by key.
func listParts(l *value.List, at *verify.Path) []part {
	out := make([]part, 0, len(l.Elems))
	for i, e := range l.Elems {
		p := at.Index(i)
		if r, ok := e.(*value.Record); ok && r.Ident != nil {
			p = at.Key(r.Ident.Key)
		}
		out = append(out, part{e, p})
	}
	return out
}

// mapParts are each key, then its value.
func mapParts(m *value.Map, at *verify.Path) []part {
	var out []part
	for i, k := range m.Keys {
		p := at.Key(verify.KeyOf(k))
		out = append(out, part{k, p})
		if i < len(m.Vals) {
			out = append(out, part{m.Vals[i], p})
		}
	}
	return out
}

func tableParts(t *value.Table, at *verify.Path) []part {
	var out []part
	for _, e := range t.Entries {
		if e != nil && e.Ident != nil {
			out = append(out, part{e, at.Entry(e.Ident.Key)})
		}
	}
	return out
}

// invalidBelow: v or a value under it, refs not followed, is invalid (EVALUATION.md §7.3).
func (t *traversal) invalidBelow(v value.Value) bool {
	if v == nil {
		return false
	}
	if known, ok := t.below[v]; ok {
		return known
	}
	bad := t.ev.Invalid(v)
	for _, p := range parts(v, nil) {
		if bad {
			break
		}
		bad = t.invalidBelow(p.v)
	}
	t.below[v] = bad
	return bad
}

// runChecks runs the checks of an instance's record or case, in declaration order; a case value's variant-level checks first (EVALUATION.md §8.1).
func (t *traversal) runChecks(rec *value.Record, at *verify.Path) {
	for _, c := range t.checksOf(rec.T) {
		if t.ctx.Err() != nil {
			return
		}
		t.report(c, t.ev.Run(t.ctx, c, rec), rec, at)
	}
}

func (r *Runner) checksOf(t types.Type) []*syntax.CheckDecl {
	switch d := t.Base().(type) {
	case *types.RecordType:
		return d.Checks
	case *types.CaseType:
		return append(slices.Clip(r.shared[d.Variant]), d.Checks...)
	case *types.AppliedRecord:
		return d.Rec.Checks
	}
	return nil
}
