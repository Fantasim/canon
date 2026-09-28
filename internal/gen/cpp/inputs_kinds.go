package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// inputReads read each input type (EVALUATION.md §11.3's table).
var inputReads = map[types.Kind]func(*gen, *ir.Field) inputRead{
	types.Bool: (*gen).readBool, types.Int: (*gen).readInt, types.Float: (*gen).readFloat,
	types.Duration: (*gen).readDuration, types.String: (*gen).readString, types.Enum: (*gen).readEnum,
}

func (g *gen) readBool(f *ir.Field) inputRead { return g.parsed(f, cppBool) }

// readInt checks the sized type's implicit range, then the field's own (EVALUATION.md §11.3 step 3).
func (g *gen) readInt(f *ir.Field) inputRead {
	r := g.parsed(f, cppInt64)
	if f.Type.Bits != bits64 || !f.Type.Signed {
		basic := types.Basic{K: types.Int, Bits: f.Type.Bits, Signed: f.Type.Signed}
		lo, hi, _ := basic.Limits()
		r.checks = append(r.checks, outside(valVar, intLit(lo), intLit(hi)))
	}
	r.checks = ownRange(r.checks, f.Range, valVar)
	r.stored = fmt.Sprintf(staticCastFormat, g.storage(f.Type), valVar)
	return r
}

// readFloat checks the field's range; a Float32 is rounded first, its overflow outside its range, and the range checked on the rounded value (log-2026-09-24 "W2 runtime inputs").
func (g *gen) readFloat(f *ir.Field) inputRead {
	r := g.parsed(f, cppDouble)
	if f.Type.Bits == float32Bits {
		r.checks = append(r.checks, inputCheck{cond: fmt.Sprintf(float32OverflowFormat, valVar), reason: ir.InputOutsideRange})
		r.stored = fmt.Sprintf(staticCastFormat, cppFloat, valVar)
	}
	if b := f.Range; b != nil && (b.HasLo || b.HasHi) {
		lo, hi := g.floatBounds(b)
		r.checks = append(r.checks, outside(r.stored, lo, hi))
	}
	return r
}

// readDuration checks the field's range in ms; past ±DurationLimit the text is not a valid Duration (log-2026-09-24 "W2 gen/cpp inputs review").
func (g *gen) readDuration(f *ir.Field) inputRead {
	r := g.parsed(f, cppInt64)
	r.checks = ownRange(r.checks, f.Range, valVar)
	r.stored = fmt.Sprintf(millisFormat, valVar)
	return r
}

// readString checks the length in bytes, then the pattern with search semantics, by MatchPattern over the pattern's constant automaton (EVALUATION.md §11.3; CODEGEN.md §5.12, §7.7).
func (g *gen) readString(f *ir.Field) inputRead {
	r := g.parsed(f, cppString)
	r.checks = ownRange(r.checks, f.Range, fmt.Sprintf(sizeFormat, valVar))
	if f.Pattern != nil {
		r.decls = append(g.patternTable(f.Pattern), r.decls...)
		r.checks = append(r.checks, inputCheck{cond: fmt.Sprintf(noMatchFormat, patternVar, valVar), reason: ir.InputNoMatch})
	}
	r.stored = fmt.Sprintf(moveFormat, valVar)
	return r
}

// readEnum reads a member by its wire value through <E>FromWire, an @json(codes) enum's too, and refuses a retired member (EVALUATION.md §11.3; log-2026-09-25 "ir round-2 review calls").
func (g *gen) readEnum(f *ir.Field) inputRead {
	e, ok := f.Type.Named.(*ir.Enum)
	if !ok {
		g.malformed(noDecl, g.at)
		return inputRead{}
	}
	name := g.qualifiedOwn(e)
	cond := fmt.Sprintf(notValFormat, valVar)
	var retired []string
	for _, m := range e.Members {
		if m.Retired {
			retired = append(retired, fmt.Sprintf(isMemberFormat, valVar, name+scopeSep+g.pl.Enumerator(m)))
		}
	}
	if len(retired) > 0 {
		cond = strings.Join(append([]string{cond}, retired...), orSep)
	}
	fromWire := g.enumHelpers(e).FromWire
	if e.Pkg == g.p.Name {
		fromWire = g.own() + fromWire
	}
	return inputRead{
		decls:  []string{fmt.Sprintf(constAutoFormat, valVar, fmt.Sprintf(helperFormat, fromWire, rawVar))},
		checks: []inputCheck{{cond: cond, reason: ir.InputNotMember}},
		stored: fmt.Sprintf(derefFormat, valVar),
	}
}

// ownRange appends the field's own integer, length or Duration range, if any (EVALUATION.md §11.3 step 3).
func ownRange(checks []inputCheck, b *types.Bound, v string) []inputCheck {
	if b == nil || !b.HasLo && !b.HasHi {
		return checks
	}
	lo, hi := intBounds(b)
	return append(checks, outside(v, lo, hi))
}

// outside is the refusal of v outside lo..=hi.
func outside(v, lo, hi string) inputCheck {
	return inputCheck{cond: fmt.Sprintf(outsideFormat, v, lo, v, hi), reason: ir.InputOutsideRange}
}

// qualifiedOwn is a type's C++ name from the global namespace, the package's own included (CODEGEN.md §2.8, §3.5).
func (g *gen) qualifiedOwn(t ir.Type) string {
	name := g.typeName(t)
	if strings.HasPrefix(name, scopeSep) {
		return name
	}
	return g.own() + name
}
