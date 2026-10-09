package gogen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// typesSections are a types-mode file's sections in CODEGEN.md §2.7's order: types, hooks and translated fns, then the decoders, public ones first; no value, container, loader or snapshot (§2.2, §5.13).
func (g *gen) typesSections() []func() {
	return []func(){
		g.typesRefusals, g.constants, g.enums, g.kindEnums, g.branchEnums, g.idEnums, g.types, g.rowTypes, g.hooks,
		g.defineTables, g.fns, g.decoders,
	}
}

// typesRefusals refuses what stage E refuses first in types mode: a stored export fn or a computed default (E8014, no data to read them from), an input field (E8019 InputField, no LoadInputs; CODEGEN.md §2.2, §5.12, §5.13).
func (g *gen) typesRefusals() {
	for _, fn := range g.p.Fns {
		g.refuseStored(g.p.Name, fn)
	}
	for _, t := range g.p.Types {
		switch x := t.(type) {
		case *ir.Record:
			g.refuseTypes(x.QName(), x.Fields, x.Methods)
		case *ir.Variant:
			for _, c := range x.Cases {
				g.refuseTypes(x.QName()+dot+c.Name, c.Fields, c.Methods)
			}
		}
	}
}

// refuseTypes refuses a class's stored fns, computed defaults and input fields.
func (g *gen) refuseTypes(owner string, fields []*ir.Field, fns []*ir.ExportFn) {
	for _, fn := range fns {
		g.refuseStored(owner, fn)
	}
	for _, f := range fields {
		at := owner + dot + f.Name
		switch {
		case f.Computed:
			g.fail(newDetail(ErrMalformed, at, typesComputedFormat, at))
		case f.Input != nil:
			g.fail(newDetail(ErrMalformed, at, typesInputFormat, at))
		}
	}
}

// refuseStored refuses a precomputed or lookup fn of owner.
func (g *gen) refuseStored(owner string, fn *ir.ExportFn) {
	if fn.Kind != ir.FnTranslated {
		at := owner + dot + fn.Name
		g.fail(newDetail(ErrMalformed, at, typesStoredFnFormat, at))
	}
}

// publicDecoders writes Decode<T> of each record and variant, in declaration order: it reads the whole document, its paths from `$`, and returns no value unless all of it decodes (CODEGEN.md §5.13).
func (g *gen) publicDecoders() {
	for _, t := range g.p.Types {
		switch t.(type) {
		case *ir.Record, *ir.Variant:
			name := g.goName(t)
			g.exec(tmplPublicDecoder, struct {
				Name, Type, Func, Quoted string
				L                        locals
			}{g.names.PublicDecoder(t), name, g.decodeFunc(t), strconv.Quote(name), g.lc})
		}
	}
}

// textDecoders writes Decode<Fn>File of each decoded @text fn, in source order: the document's byte rules, then its result read from the root as a field of that type is, returning no value unless all of it decodes (DECISIONS 340, CODEGEN.md §5.13).
func (g *gen) textDecoders() {
	for _, fn := range g.names.TextDecoders() {
		g.textDecoder(fn)
	}
}

func (g *gen) textDecoder(fn *ir.ExportFn) {
	defer g.enter(g.p.Name + dot + fn.Name)()
	g.temps = 0
	t := ir.TextResult(fn)
	var b strings.Builder
	x := g.readValue(&b, leaf{t: t}, g.lc.R, g.root())
	g.exec(tmplTextDecoder, struct {
		Name, Type, Quoted, Body, Value string
		L                               locals
	}{g.names.TextDecoder(fn), g.goType(t), strconv.Quote(g.names.TextFile(fn)), b.String(), x, g.lc})
}

// bodyKeys are the keys a decoder of b checks its object's against (expectedKeys); in types mode, which checks none (CODEGEN.md §5.13: unknown keys are ignored), the wire keys its fields read from that object, so that an object no field reads is never bound.
func (g *gen) bodyKeys(b *body) []string {
	if !g.isTypes() {
		return g.expectedKeys(b)
	}
	var keys []string
	for _, s := range b.slots {
		switch f := s.src; {
		case s.fn != nil, f == nil, s.isInput(), f.Inline:
		case f.Pairs != nil:
			keys = append(keys, g.pairsKeys(f)...)
		case len(f.WirePath) > 0:
			keys = append(keys, f.WirePath[0])
		}
	}
	return keys
}

// fieldDefault is the constant default a types-mode decoder gives s's field when its key is absent, nil for none or without one (CODEGEN.md §5.13; WIRE.md §5.4).
func (g *gen) fieldDefault(s *slot) value.Value {
	if !g.isTypes() || s.src == nil || s.fn != nil {
		return nil
	}
	if _, none := s.src.Default.(*value.None); none {
		return nil
	}
	return s.src.Default
}

// absentDefault writes, for a field with a default, the branch an absent key takes and returns the `} else ` its read continues with; "" for a field without one.
func (g *gen) absentDefault(s *slot, obj, quoted string) string {
	def := g.fieldDefault(s)
	if def == nil {
		return ""
	}
	g.printf(absentOpenFormat, g.temp(tempOK), obj, quoted)
	g.writeDefault(s, def)
	return strings.TrimSuffix(elseLine, lbrace+newline)
}

// writeDefault stores v, s's default, as a read value is stored (store): a ref's key, presence when optional; the define value of a ref into a load.defines table is looked up after it, as for a read key (readDefine).
func (g *gen) writeDefault(s *slot, v value.Value) {
	defer g.enter(s.origin)()
	var b strings.Builder
	g.store(&b, s, g.lc.Out, g.expr(s.T, v))
	g.body.WriteString(b.String())
}

// sourceDuration reads a source-wire Duration: any number token whose exact value in the field's unit is a whole number of milliseconds within the Duration range (WIRE.md §5.1, §5.13).
func (g *gen) sourceDuration(b *strings.Builder, raw string, loc location, unit types.Unit) string {
	n := g.temp(tempInt)
	g.called[helperDuration] = true
	prefix, key := g.splitLoc(loc)
	fmt.Fprintf(b, durationReadFormat, n, g.lc.Err, g.helper(helperDuration), g.lc.Name, prefix, key, raw, unit.Millis())
	return g.ownRT() + durationFromMs + n + rparen
}
