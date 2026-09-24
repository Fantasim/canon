package gogen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// hop is one step of a path of self: the storage it reads, of type t, and how its absence is marked: a presence flag's storage, or nil for an optional record (CODEGEN.md §4.3).
type hop struct {
	store, ok string
	nilable   bool
	t         ir.TypeRef
}

// readArgs are the pure function's arguments read from self, in Reads order, and the statements an optional path needs first (CONFORMANCE.md §2.3: `(x T, xOk bool)` in Go).
func (g *gen) readArgs(b *body, p *pure) (pre string, args []string) {
	var sb strings.Builder
	for i, r := range p.fn.Reads {
		hops := g.hops(b, r.Path)
		if len(hops) == 0 {
			args = append(args, nilLit)
			continue
		}
		if !r.Optional {
			steps := []string{selfRecv}
			for _, h := range hops {
				steps = append(steps, h.store)
			}
			args = append(args, g.toPure(hops[len(hops)-1].t, strings.Join(steps, dot)))
			continue
		}
		v, ok := p.plan.Locals[r.Name], p.plan.OKs[i]
		fmt.Fprintf(&sb, varFormat, v, g.pureType(r.Type))
		fmt.Fprintf(&sb, varFormat, ok, goBool)
		g.optionalRead(&sb, p, hops, v, ok)
		args = append(args, v, ok)
	}
	return sb.String(), args
}

// optionalRead assigns v and ok through a path with an optional step: each optional record is
// tested for nil, the last step's flag is ok.
func (g *gen) optionalRead(sb *strings.Builder, p *pure, hops []hop, v, ok string) {
	cur, depth := selfRecv, 0
	for _, h := range hops[:len(hops)-1] {
		step := cur + dot + h.store
		if !h.nilable {
			cur = step
			continue
		}
		cur = p.temp(g, tempElem)
		fmt.Fprintf(sb, ifBindFormat, cur, step, cur)
		depth++
	}
	last := hops[len(hops)-1]
	present := trueLit
	switch {
	case last.ok != "":
		present = cur + dot + last.ok
	case last.nilable:
		g.failf(ErrMalformed, "an optional read of a record or variant in %s", g.at)
	}
	fmt.Fprintf(sb, assignPairFormat, v, ok, g.toPure(last.t, cur+dot+last.store), present)
	sb.WriteString(strings.Repeat(closeBrace, depth))
}

// hops resolves a path: its first step a field or a precomputed method of self, the next ones
// fields of the record the step before holds.
func (g *gen) hops(b *body, path []string) []hop {
	if len(path) == 0 {
		g.failf(ErrMalformed, "a read of self without a path in %s", g.at)
		return nil
	}
	fields := b.fields
	out := make([]hop, 0, len(path))
	for i, seg := range path {
		f := fieldNamed(fields, seg)
		switch {
		case f != nil:
			out = append(out, g.fieldHop(f))
		case i == 0 && methodNamed(b.methods, seg) != nil:
			s := g.names.MethodSlot(methodNamed(b.methods, seg))
			out = append(out, g.slotHop(s))
		default:
			g.failf(ErrMalformed, "a read of %s, which %s does not hold, in %s", strings.Join(path, dot), b.canon, g.at)
			return nil
		}
		if rec, ok := out[i].t.Named.(*ir.Record); ok && out[i].t.Kind == types.Record {
			fields = rec.Fields
		} else {
			fields = nil
		}
	}
	return out
}

func (g *gen) fieldHop(f *ir.Field) hop {
	return g.slotHop(g.names.Slot(f))
}

// slotHop reads a slot's main storage; its flag when it has one (CODEGEN.md §4.3).
func (g *gen) slotHop(s ir.GoSlot) hop {
	h := hop{store: s.Store, t: s.T, nilable: s.Optional && !s.OK}
	if s.OK {
		h.ok = s.OKStore
	}
	return h
}

func fieldNamed(fields []*ir.Field, name string) *ir.Field {
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// methodNamed is the precomputed method name, read like a field (CONFORMANCE.md §2.2).
func methodNamed(fns []*ir.ExportFn, name string) *ir.ExportFn {
	for _, fn := range fns {
		if fn.Name == name && fn.Kind == ir.FnPrecomputed {
			return fn
		}
	}
	return nil
}
