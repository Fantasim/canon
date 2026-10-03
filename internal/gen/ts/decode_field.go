package tsgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// fieldRead reads one field where its wire form puts it (WIRE.md §5.5, §5.6, §5.14) into a numbered local, and adds it to the returned object.
func (g *gen) fieldRead(rd *reading, f *ir.Field) {
	local := g.local()
	raw := rawPrefix + strconv.Itoa(g.temps)
	typ := g.fieldType(f)
	var stmts []string
	switch {
	case f.Inline:
		stmts = []string{fmt.Sprintf(typedLocalFormat, local, typ, g.dec(f.Type, objVar, pathParam, decCtx{}))}
	case f.Pairs != nil:
		stmts = []string{fmt.Sprintf(typedLocalFormat, local, typ, g.pairsRead(f))}
	case len(f.WirePath) > 0:
		stmts = g.keyRead(rd, f, local, raw)
	default:
		g.failf(ErrMalformed, malformedNoWire, f.Name, g.at)
	}
	rd.stmts = append(rd.stmts, stmts...)
	rd.props = append(rd.props, property(fieldProp(f))+keyValueSep+local)
	rd.locals[f] = local
}

// keyRead is the statements reading a field's key: a required value, one with a default, or an optional one (WIRE.md §5.4).
func (g *gen) keyRead(rd *reading, f *ir.Field, local, r string) []string {
	raw, path := g.rawOf(f), g.pathOf(f)
	ctx := decCtx{unit: f.Unit, enc: f.Enc, big: f.BigInt}
	if app := ir.HeldApp(f.Type); app != nil {
		ctx.disc = g.discExpr(rd, *app)
	}
	hasDefault := f.Default != nil && !f.Computed
	typ := g.fieldType(f)
	if !f.Optional && !hasDefault {
		return []string{fmt.Sprintf(typedLocalFormat, local, typ, g.dec(f.Type, raw, path, ctx))}
	}
	read := g.dec(f.Type, r, path, ctx)
	stmts := []string{fmt.Sprintf(localFormat, r, raw)}
	absent := tsNull
	if hasDefault {
		absent = g.defaultLit(f, rd)
	}
	switch {
	case f.Optional && absent == tsNull:
		stmts = append(stmts, fmt.Sprintf(typedLocalFormat, local, typ, fmt.Sprintf(condFormat, g.noneTest(r, f, true), tsNull, read)))
	case f.Optional:
		stmts = append(stmts, fmt.Sprintf(typedLocalFormat, local, typ, fmt.Sprintf(defaultedFormat, r, absent, fmt.Sprintf(condFormat, g.noneTest(r, f, false), tsNull, read))))
	default:
		stmts = append(stmts, fmt.Sprintf(typedLocalFormat, local, typ, fmt.Sprintf(defaultedFormat, r, absent, read)))
	}
	return stmts
}

// defaultLit is a field's constant default as a literal.
func (g *gen) defaultLit(f *ir.Field, rd *reading) string {
	if _, none := f.Default.(*value.None); none {
		return tsNull
	}
	return g.fieldLit(f, f.Default, &owner{fields: rd.fields})
}

// rawOf is the key of an object a field reads: `decGet(o, "k")`, or `decAt(o, ["a", "k"], path)` down its @json(path:).
func (g *gen) rawOf(f *ir.Field) string {
	if len(f.WirePath) == 1 {
		return g.getKey(objVar, f.WirePath[0])
	}
	keys := make([]string, len(f.WirePath))
	for i, k := range f.WirePath {
		keys[i] = quote(k)
	}
	return g.call(decAtName, objVar, lbracket+joinArgs(keys)+rbracket, pathParam)
}

// pathOf is the JSON pointer of a field's value, for messages.
func (g *gen) pathOf(f *ir.Field) string {
	return pathParam + plusSep + quote(pathSep+strings.Join(f.WirePath, pathSep))
}

// noneTest is the condition that raw is none: absent when the field has no default, null, or the field's none marker (WIRE.md §5.4).
func (g *gen) noneTest(raw string, f *ir.Field, withAbsent bool) string {
	parts := []string{}
	if withAbsent {
		parts = append(parts, raw+strictEq+tsUndefined)
	}
	parts = append(parts, raw+strictEq+tsNull)
	if f.NoneWire != nil {
		parts = append(parts, g.markerTest(raw, f.NoneWire))
	}
	return strings.Join(parts, orSep)
}

// markerTest is raw JSON-equal to the none marker: a number by exact value, whatever its token (decSame), a string, Bool or null by ===; a non-empty object or array is refused as check's E3316 does (WIRE.md §4.1, §5.4).
func (g *gen) markerTest(raw string, marker []byte) string {
	var v any
	if err := json.Unmarshal(marker, &v); err != nil {
		g.failf(ErrMalformed, malformedMarker, marker, g.at)
		return strconv.FormatBool(false)
	}
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			return g.call(decEmptyObjectName, raw)
		}
	case []any:
		if len(x) == 0 {
			return g.call(decEmptyArrayName, raw)
		}
	case float64:
		return g.call(decSameName, raw, quote(string(bytes.TrimSpace(marker))))
	default:
		return raw + strictEq + string(bytes.TrimSpace(marker))
	}
	g.failf(ErrMalformed, malformedNoneMark, marker, g.at)
	return strconv.FormatBool(false)
}

// pairsRead is `decPairs(o, path, [k0, ...], [v0, ...], (a, b, pa, pb) => ({...}))`: the slots of a @json(pairs:) field (WIRE.md §5.14).
func (g *gen) pairsRead(f *ir.Field) string {
	rec, ok := g.elem(f.Type).Named.(*ir.Record)
	if !ok || len(rec.Fields) != pairFields || f.Pairs == nil || f.Pairs.Slots <= 0 {
		g.failf(ErrMalformed, malformedPairs, g.at)
		return tsUndefined
	}
	keys := [pairFields]string{}
	for k, tmpl := range f.Pairs.Keys {
		keys[k] = lbracket + joinArgs(g.slotKeys(tmpl, f.Pairs.Slots)) + rbracket
	}
	vals := [pairFields]string{pairRawA, pairRawB}
	paths := [pairFields]string{pairPathA, pairPathB}
	props := make([]string, pairFields)
	for k, ef := range rec.Fields {
		ctx := decCtx{unit: ef.Unit, enc: ef.Enc, big: ef.BigInt}
		props[k] = property(fieldProp(ef)) + keyValueSep + g.dec(ef.Type, vals[k], paths[k], ctx)
	}
	read := fmt.Sprintf(pairsLambdaFormat, joinArgs(props))
	return g.call(decPairsName, objVar, pathParam, keys[0], keys[1], read)
}

// slotKeys are a pairs template's keys of slot 0 to n-1, quoted: `{i}` replaced by the slot in decimal.
func (g *gen) slotKeys(tmpl string, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = quote(strings.ReplaceAll(tmpl, "{i}", fmt.Sprint(i)))
	}
	return keys
}

// discExpr is a dependent type's discriminant as the reader already holds it: the local of the first field read, then the properties down ir.DiscFields' path (CODEGEN.md §5.6).
func (g *gen) discExpr(rd *reading, app ir.TypeRef) string {
	path := ir.DiscFields(rd.fields, app)
	if len(path) == 0 || rd.locals[path[0]] == "" {
		g.failf(ErrMalformed, malformedNoDisc, g.at)
		return ""
	}
	parts := []string{rd.locals[path[0]]}
	for _, f := range path[1:] {
		parts = append(parts, fieldProp(f))
	}
	return strings.Join(parts, dot)
}

// fnRead reads the `$<fn>` key of a precomputed fn, or the nested object of a finite one (WIRE.md §5.11).
func (g *gen) fnRead(rd *reading, fn *ir.ExportFn) {
	local := g.local()
	key := dollarKey + fn.Name
	raw := g.getKey(objVar, key)
	path := pathParam + plusSep + quote(pathSep+key)
	var read string
	if len(fn.Params) == 0 {
		read = g.dec(fn.Result, raw, path, decCtx{})
	} else {
		read = g.finiteRead(fn, 0, raw, path)
	}
	rd.stmts = append(rd.stmts, fmt.Sprintf(typedLocalFormat, local, g.fnType(fn), read))
	rd.props = append(rd.props, property(fnProp(fn))+keyValueSep+local)
}

// finiteRead is a nested object, one level per parameter, keyed by the wires of its domain (WIRE.md §5.11).
func (g *gen) finiteRead(fn *ir.ExportFn, level int, raw, path string) string {
	p := fn.Params[level]
	keys, wire := g.domainKeys(p.Type)
	var inner string
	if level+1 == len(fn.Params) {
		inner = g.dec(fn.Result, lambdaRaw, lambdaPath, decCtx{})
	} else {
		inner = g.finiteRead(fn, level+1, lambdaRaw, lambdaPath)
	}
	targs := g.domainType(p.Type) + listSep + g.levelType(fn, level+1)
	return g.helper(decKeyedName) + fmt.Sprintf(typeArgsFormat, targs) + lparen + joinArgs([]string{raw, path, keys, wire, fmt.Sprintf(lambdaFormat, inner)}) + rparen
}

// levelType is the type of what a level of a finite fn's object holds: the result wrapped by the parameters after level.
func (g *gen) levelType(fn *ir.ExportFn, level int) string {
	t := g.tsType(fn.Result, false)
	for _, p := range slices.Backward(fn.Params[level:]) {
		t = fmt.Sprintf(readonlyRecordFormat, g.domainType(p.Type), t)
	}
	return t
}

// domainKeys is a finite parameter's keys as an array and the function from a key to its wire: `["false", "true"]`, or an enum's Members, written by name or, with @json(codes), by code (WIRE.md §5.8, §5.11).
func (g *gen) domainKeys(t ir.TypeRef) (keys, wire string) {
	if t.Kind == types.Bool {
		return boolKeyList, identityWire
	}
	if e, ok := t.Named.(*ir.Enum); ok && t.Kind == types.Enum {
		if e.JSONCodes {
			return g.membersConst(e), fmt.Sprintf(codeWireFormat, g.codesConst(e))
		}
		return g.membersConst(e), identityWire
	}
	g.failf(errUnsupported, unsupportedFinite, g.at)
	return tsUndefined, tsUndefined
}
