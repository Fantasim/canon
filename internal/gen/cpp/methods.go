package cppgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// domain is one finite parameter's values in domain order: their wire keys and its size.
type domain struct {
	keys []string
	enum *ir.Enum // nil for Bool and a table id
	id   bool     // a table id, its own ordinal (CODEGEN.md §5.3)
}

// domains lists each parameter's wire keys in domain order (CODEGEN.md §5.10, WIRE.md §5.11).
func (g *gen) domains(fn *ir.ExportFn) []domain {
	out := make([]domain, 0, len(fn.Params))
	for i, p := range fn.Params {
		ids, isID := g.tableDomain(fn, i)
		switch e, _ := p.Type.Named.(*ir.Enum); {
		case p.Type.Kind == types.Bool:
			out = append(out, domain{keys: []string{strconv.FormatBool(false), strconv.FormatBool(true)}})
		case p.Type.Kind == types.Enum && e != nil:
			d := domain{enum: e}
			for _, m := range e.Members {
				d.keys = append(d.keys, enumKey(e, m))
			}
			out = append(out, d)
		case isID:
			out = append(out, domain{keys: ids, id: true})
		default:
			g.malformed(lookupParams, fn.Name) // E8013 refParam: a data-mode finite parameter is a Bool or an enum
		}
	}
	return out
}

// tableDomain is parameter i's domain when it is a table id with an id enum, the table's keys in entry order: this package's table's ids, another package's from the lookup's Domains, which stage E enumerates whatever receivers exist (CODEGEN.md §5.10). False for any other parameter.
func (g *gen) tableDomain(fn *ir.ExportFn, i int) ([]string, bool) {
	t := fn.Params[i].Type
	if _, id := g.idEnum(t); !id {
		return nil, false
	}
	if t.Ref.Pkg == g.p.Name {
		for _, v := range g.p.Values {
			if v.Name == t.Ref.Value && v.Type.Kind == types.Table {
				return v.IDs, true
			}
		}
		return nil, false
	}
	if i >= len(fn.Domains) { // stage E enumerates every lookup's domains, receivers or not (log-2026-10-06 "U5 review FAIL" 2)
		g.malformed(lookupParams, fn.Name)
		return nil, false
	}
	var keys []string
	for _, k := range fn.Domains[i] {
		if r, ok := k.(*value.Ref); ok {
			keys = append(keys, r.Key.S)
		}
	}
	return keys, true
}

// enumKey is a member's wire key: its wire value, or its code with @json(codes) (WIRE.md §5.8).
func enumKey(e *ir.Enum, m *ir.EnumMember) string {
	if e.JSONCodes {
		return strconv.FormatInt(m.Code, decimalBase)
	}
	return m.Wire
}

func cells(doms []domain) int {
	n := 1
	for _, d := range doms {
		n *= len(d.keys)
	}
	return n
}

// resultType splits an optional result into its element and the flag.
func resultType(t ir.TypeRef) (ir.TypeRef, bool) {
	if t.Kind == types.Optional && t.Elem != nil {
		return *t.Elem, true
	}
	return t, false
}

// publicMethod calls the pure function on self's paths and converted arguments (CONFORMANCE.md §2.3).
func (g *gen) publicMethod(owner string, fields []*ir.Field, fn *ir.ExportFn, name string) string {
	var params, args []string
	for _, r := range fn.Reads {
		args = append(args, g.readExpr(fields, r))
	}
	for _, p := range fn.Params {
		params = append(params, g.publicParam(p.Type)+space+verbatim(p.Name))
		args = append(args, toPure(p.Type, verbatim(p.Name)))
	}
	call := fmt.Sprintf(pureCallFormat, g.pl.PureName(owner, fn), strings.Join(args, listSep))
	return fmt.Sprintf(methodFormat, g.storage(fn.Result), name, strings.Join(params, listSep), g.fromPure(fn.Result, call))
}

// hop is one step of a path of self: the member or getter call reaching it, its field.
type hop struct {
	step  string
	field *ir.Field
}

// readExpr is the pure argument of a path of self, none when an optional step is (CONFORMANCE.md §2.3).
func (g *gen) readExpr(fields []*ir.Field, r *ir.Read) string {
	hops := g.hops(fields, r.Path)
	if hops == nil {
		return cppInvalid
	}
	last := hops[len(hops)-1].field
	if !r.Optional {
		steps := make([]string, len(hops))
		for i, h := range hops {
			steps[i] = h.step
			if h.field.Optional {
				g.fail(fmt.Errorf("%w: read %s through an optional is not optional", ErrMalformed, r.Name))
			}
		}
		return readToPure(last.Type, strings.Join(steps, memberAccess))
	}
	var b block
	cur, sep := "", ""
	for i, h := range hops {
		cur, sep = cur+sep+h.step, memberAccess
		if h.field.Optional {
			v := fmt.Sprintf(stepVarFormat, i)
			b.add(stepFormat, v, cur)
			b.add(noneReturnFormat, v)
			cur, sep = v, arrow
		}
	}
	if last.Optional {
		cur = fmt.Sprintf(derefFormat, cur)
	}
	pure := fmt.Sprintf(optionalFormat, g.pureType(r.Type))
	return fmt.Sprintf(lambdaFormat, pure, strings.Join(b.lines, space), readToPure(last.Type, cur))
}

// hops walks the path: the first step is a member, the next ones getters of a held record.
func (g *gen) hops(fields []*ir.Field, path []string) []hop {
	if len(path) == 0 {
		g.fail(fmt.Errorf("%w: a read of self without a path at %s", ErrMalformed, g.at))
		return nil
	}
	f := fieldNamed(fields, path[0])
	if f == nil {
		g.malformed(unknownReads, path[0])
		return nil
	}
	m, err := g.member(f.Name)
	g.fail(err)
	out := []hop{{step: m, field: f}}
	for _, seg := range path[1:] {
		rec, ok := f.Type.Named.(*ir.Record)
		if f.Type.Kind != types.Record || !ok {
			g.malformed(unknownReads, seg)
			return nil
		}
		if f = fieldNamed(rec.Fields, seg); f == nil {
			g.malformed(unknownReads, seg)
			return nil
		}
		out = append(out, hop{step: g.getterName(f) + callSuffix, field: f})
	}
	return out
}

func fieldNamed(fields []*ir.Field, name string) *ir.Field {
	for _, f := range fields {
		if f != nil && f.Name == name {
			return f
		}
	}
	return nil
}

// readToPure converts what self holds to the pure type: a Duration's count, a variant's kind.
func readToPure(t ir.TypeRef, expr string) string {
	switch t.Kind {
	case types.Duration:
		return expr + countCall
	case types.Variant:
		return expr + kindCall
	default:
		return expr
	}
}

// publicParam is a declared parameter's public type: the getter type, a string_view for String.
func (g *gen) publicParam(t ir.TypeRef) string {
	if t.Kind == types.String {
		return cppStringView
	}
	return g.storage(t)
}

func toPure(t ir.TypeRef, name string) string {
	if t.Kind == types.Duration {
		return name + countCall
	}
	return name
}

// fromPure converts the pure result back: milliseconds, then narrowing a sized result.
func (g *gen) fromPure(t ir.TypeRef, call string) string {
	switch {
	case t.Kind == types.Duration:
		return fmt.Sprintf(millisFormat, call)
	case t.Kind == types.Int && (t.Bits != bits64 || !t.Signed), t.Kind == types.Float && t.Bits == float32Bits:
		return fmt.Sprintf(staticCastFormat, g.storage(t), call)
	default:
		return call
	}
}
