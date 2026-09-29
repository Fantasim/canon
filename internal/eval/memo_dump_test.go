package eval_test

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// deepDump is everything observable of an evaluation's values (log-2026-09-29 M4 U11, EVL-07):
// each node of each value by first reach, its kind, text and provenance, its identity, owner and
// instance, its invalid and written marks and its bound arguments, nodes met again by number.
type deepDump struct {
	ev    *eval.Evaluator
	ids   map[value.Value]int
	queue []value.Value
}

// deep dumps every value of b, forced-set order.
func deep(b *build) string {
	d := &deepDump{ev: b.ev, ids: map[value.Value]int{}}
	var sb strings.Builder
	for _, root := range b.order {
		v, ok := b.values[root]
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "%s.%s = #%d\n", root.Pkg, root.Name, d.id(v))
		for len(d.queue) > 0 {
			n := d.queue[0]
			d.queue = d.queue[1:]
			sb.WriteString(d.line(n))
		}
	}
	return sb.String()
}

// id is n's number, given and queued at its first reach; -1 for none.
func (d *deepDump) id(n value.Value) int {
	if n == nil {
		return -1
	}
	if id, ok := d.ids[n]; ok {
		return id
	}
	d.ids[n] = len(d.ids)
	d.queue = append(d.queue, n)
	return d.ids[n]
}

func (d *deepDump) record(r *value.Record) value.Value {
	if r == nil {
		return nil
	}
	return r
}

// line is one node: its own data, then the numbers of what it holds and names.
func (d *deepDump) line(n value.Value) string {
	s := fmt.Sprintf("#%d %T %q %s invalid=%t written=%t", d.ids[n], n, n.CanonText(), provText(n.Prov()), d.ev.Invalid(n), d.ev.WrittenMark(n))
	var parts []value.Value
	switch x := n.(type) {
	case *value.Record:
		s += fmt.Sprintf(" set=%v%s inst=#%d%s", x.Set, d.ident(x.Ident), d.id(d.ev.InstanceOf(x)), d.args(x))
		parts = x.Fields
	case *value.List:
		parts = x.Elems
	case *value.Map:
		parts = append(slices.Clone(x.Keys), x.Vals...)
	case *value.Table:
		for _, en := range x.Entries {
			parts = append(parts, en)
		}
	case *value.Pair:
		parts = []value.Value{x.A, x.B}
	case *value.Ref:
		s += fmt.Sprintf(" owner=#%d", d.id(d.record(x.Owner)))
	}
	ids := make([]int, len(parts))
	for i, p := range parts {
		ids[i] = d.id(p)
	}
	return fmt.Sprintf("%s parts=%v\n", s, ids)
}

// ident is an identity: its collection by name, key, retirement and owner.
func (d *deepDump) ident(id *value.Identity) string {
	if id == nil {
		return ""
	}
	coll := "none"
	if c := id.Coll; c != nil {
		coll = fmt.Sprintf("%d:%s.%s%v", c.Kind, c.Pkg, c.Name, c.FieldPath)
	}
	return fmt.Sprintf(" ident=%s:%s retired=%t owner=#%d", coll, id.Key.Text(), id.Retired, d.id(d.record(id.Owner)))
}

// args is a record's bound arguments, by parameter order.
func (d *deepDump) args(rec *value.Record) string {
	params := d.ev.BoundArgs(rec)
	order := slices.SortedFunc(maps.Keys(params), func(a, b *types.Param) int { return cmp.Compare(a.Index, b.Index) })
	var sb strings.Builder
	for _, p := range order {
		fmt.Fprintf(&sb, " %s=#%d", p.Name, d.id(params[p]))
	}
	return sb.String()
}

// provText is a provenance and the ones it goes through (EVALUATION.md §13).
func provText(p *value.Prov) string {
	var sb strings.Builder
	for ; p != nil; p = p.Via {
		fmt.Fprintf(&sb, "<%d %v %q %q %v+%d>", p.Kind, p.Span, p.Pointer, p.Layer, p.Stack, p.MoreFrames)
	}
	return sb.String()
}
