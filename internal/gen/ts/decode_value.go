package tsgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// decCtx is what a field adds to how its value is read: its unit, its encoding, @ts(bigint), and a dependent type's discriminant (WIRE.md §4, CODEGEN.md §5.6).
type decCtx struct {
	unit types.Unit
	enc  types.Enc
	big  bool
	disc string
}

// dec is the expression decoding raw, found at path, as a value of type t (WIRE.md §5).
func (g *gen) dec(t ir.TypeRef, raw, path string, c decCtx) string {
	c.big = bigAt(t, c.big)
	switch t.Kind {
	case types.Bool:
		if c.enc == types.EncInt {
			return g.call(decBitName, raw, path)
		}
		return g.call(decBoolName, raw, path)
	case types.Int:
		return g.decInt(t, raw, path, c)
	case types.Float:
		if t.Bits == float32Bits {
			return g.call(decF32Name, raw, path)
		}
		return g.call(decFloatName, raw, path)
	case types.String:
		return g.call(decStringName, raw, path)
	case types.LitUnion:
		return g.call(decStringName, raw, path) + asKw + g.tsType(t, false)
	case types.Duration:
		return g.call(decDurationName, raw, path, intText(c.unit.Millis()))
	case types.Enum:
		return g.decEnum(t, raw, path)
	default:
		return g.decComposite(t, raw, path, c)
	}
}

// call is a call of a decoder helper.
func (g *gen) call(helper string, args ...string) string {
	return fmt.Sprintf(callFormat, g.helper(helper), joinArgs(args))
}

// decInt is an integer checked against its type's range: a number, or a bigint for a @ts(bigint) field.
func (g *gen) decInt(t ir.TypeRef, raw, path string, c decCtx) string {
	lo, hi, _ := types.Basic{K: types.Int, Bits: t.Bits, Signed: t.Signed}.Limits()
	if c.big {
		return g.call(decBigName, raw, path, bigText(lo), bigText(hi))
	}
	return g.call(decIntName, raw, path, intText(lo), intText(hi))
}

// decEnum is an enum member by its wire, or by its code with @json(codes) (WIRE.md §5.3).
func (g *gen) decEnum(t ir.TypeRef, raw, path string) string {
	e, ok := t.Named.(*ir.Enum)
	if !ok {
		g.failf(ErrMalformed, malformedDecl, g.at)
		return tsUndefined
	}
	if e.JSONCodes {
		return g.call(decCodeName, raw, path, g.membersConst(e), g.codesConst(e))
	}
	return g.call(decEnumName, raw, path, g.membersConst(e))
}

// membersConst is the Members table of an enum, imported when another package's.
func (g *gen) membersConst(e *ir.Enum) string {
	return g.enumConst(e, membersSuffix)
}

// codesConst is the Codes table of a `@codes` enum.
func (g *gen) codesConst(e *ir.Enum) string {
	return g.enumConst(e, codesSuffix)
}

func (g *gen) enumConst(e *ir.Enum, suffix string) string {
	name := typeName(e) + suffix
	if e.Pkg != g.p.Name {
		return g.importValue(e.Pkg, name)
	}
	return name
}

// decComposite is a record, variant, list, map or ref.
func (g *gen) decComposite(t ir.TypeRef, raw, path string, c decCtx) string {
	switch t.Kind {
	case types.Record, types.Variant:
		return g.decClass(t.Named, raw, path)
	case types.TypeApp:
		return g.decDependent(t, raw, path, c)
	case types.List:
		return g.decList(t, raw, path, c)
	case types.Table:
		return g.decNested(t, raw, path)
	case types.Map:
		return g.decMap(t, raw, path, c)
	case types.Ref:
		return g.decKey(t, raw, path, c)
	case types.Optional:
		return fmt.Sprintf(nullOrFormat, raw, g.dec(g.elem(t), raw, path, c))
	default:
		g.failf(errUnsupported, unsupportedKind, kindText(t.Kind), g.at)
		return tsUndefined
	}
}

// decClass reads a record or variant through this file's reader of it, another package's included: a package never calls another's public decoders (CODEGEN.md §2.8, §5.13; DECISIONS 323).
func (g *gen) decClass(named ir.Type, raw, path string) string {
	g.need(named)
	return fmt.Sprintf(callFormat, g.readerName(named), joinArgs([]string{raw, path}))
}

// readerName is this file's private reader of a record, variant or dependent type: read<T> for this package's, the plan's read_<alias>_<T> for another package's (ir.TSReaderName; CODEGEN.md §2.8; log-2026-10-06 "TS reader names").
func (g *gen) readerName(t ir.Type) string {
	if pkgOf(t) != g.p.Name {
		return ir.TSReaderName(t)
	}
	return readPrefix + typeName(t)
}

// caseReaderName is the reader of a case with fields: read<V><Case> for this package's variant, the plan's ir.TSCaseReaderName for another package's.
func (g *gen) caseReaderName(v *ir.Variant, c *ir.Case) string {
	if v.Pkg != g.p.Name {
		return ir.TSCaseReaderName(v, c)
	}
	return readPrefix + caseName(v, c)
}

// decDependent is a dependent value, read in the branch its discriminant selects by this file's reader of its type, another package's included (CODEGEN.md §2.8, §5.6).
func (g *gen) decDependent(t ir.TypeRef, raw, path string, c decCtx) string {
	d, ok := t.Named.(*ir.Dependent)
	switch {
	case !ok:
		g.failf(ErrMalformed, malformedDecl, g.at)
		return tsUndefined
	case c.disc == "":
		g.failf(errUnsupported, unsupportedNoDisc, g.at)
		return tsUndefined
	}
	g.need(d)
	return fmt.Sprintf(callFormat, g.readerName(d), joinArgs([]string{raw, path, c.disc}))
}

// decList is an array decoded element by element; @json(bits) reads a list of enum members from one integer.
func (g *gen) decList(t ir.TypeRef, raw, path string, c decCtx) string {
	et := g.elem(t)
	if c.enc == types.EncBits && et.Kind == types.Enum {
		e, _ := et.Named.(*ir.Enum)
		if e == nil {
			g.failf(ErrMalformed, malformedDecl, g.at)
			return tsUndefined
		}
		return g.call(decBitsName, raw, path, g.membersConst(e), g.codesConst(e))
	}
	inner := decCtx{unit: c.unit, big: c.big, disc: c.disc}
	return g.call(decListName, raw, path, fmt.Sprintf(lambdaFormat, g.dec(et, lambdaRaw, lambdaPath, inner)))
}

// decNested is a nested table: an object keyed by id, each row a CanonRow: a record of this package read with its key as its id, one of another package this file's reader's record with the key and `$retired` added (WIRE.md §5.7, CODEGEN.md §4.2, §5.9).
func (g *gen) decNested(t ir.TypeRef, raw, path string) string {
	rec, ok := g.elem(t).Named.(*ir.Record)
	if !ok {
		g.failf(ErrMalformed, malformedDecl, g.at)
		return tsUndefined
	}
	if rec.Pkg != g.p.Name {
		return g.call(decTableName, raw, path, fmt.Sprintf(tableLambdaFormat, g.foreignRowRead(rec, rowIDLocal)))
	}
	g.need(rec)
	call := fmt.Sprintf(callFormat, g.readerName(rec), joinArgs([]string{lambdaRaw, lambdaPath, rowIDLocal}))
	row := fmt.Sprintf(ownRowFormat, call, g.rowType(rec, g.idTypeOf(rec)))
	return g.call(decTableName, raw, path, fmt.Sprintf(tableLambdaFormat, row))
}

// decMap is an object read into a CanonMap: its keys decoded by the key type (WIRE.md §5.8).
func (g *gen) decMap(t ir.TypeRef, raw, path string, c decCtx) string {
	if t.Key == nil {
		g.failf(ErrMalformed, malformedNoKey, g.at)
		return tsUndefined
	}
	inner := decCtx{unit: c.unit, big: c.big}
	key := fmt.Sprintf(keyLambdaFormat, g.decMapKey(*t.Key, lambdaKey, lambdaPath, c.big))
	return g.call(decMapName, raw, path, key, fmt.Sprintf(lambdaFormat, g.dec(g.elem(t), lambdaRaw, lambdaPath, inner)))
}

// decMapKey is a map key's text read as a key of type t; big reads an integer key as a bigint (@ts(bigint), or a ref into a bigint key field).
func (g *gen) decMapKey(t ir.TypeRef, key, path string, big bool) string {
	big = bigAt(t, big)
	switch t.Kind {
	case types.String:
		return key
	case types.LitUnion:
		return key + asKw + g.tsType(t, false)
	case types.Int:
		if big {
			lo, hi, _ := types.Basic{K: types.Int, Bits: t.Bits, Signed: t.Signed}.Limits()
			return g.call(decBigKeyName, key, path, bigText(lo), bigText(hi))
		}
		return g.call(decIntKeyName, key, path)
	case types.Enum:
		e, _ := t.Named.(*ir.Enum)
		if e != nil && e.JSONCodes {
			return g.call(decCodeName, fmt.Sprintf(callFormat, numberFn, key), path, g.membersConst(e), g.codesConst(e))
		}
		return g.decEnum(t, key, path)
	case types.Ref:
		return g.decRefKey(t, key, path, big)
	default:
		g.failf(errUnsupported, unsupportedKind, kindText(t.Kind), g.at)
		return tsUndefined
	}
}

// decRefKey is a map key that is a ref: a table's id string, or the key text read as the key type, an integer as bigAt decided for the ref.
func (g *gen) decRefKey(t ir.TypeRef, key, path string, big bool) string {
	if t.Key == nil {
		g.failf(ErrMalformed, malformedNoKey, g.at)
		return tsUndefined
	}
	if index := g.entryIndex(t.Ref); index != "" {
		return g.call(decEntryName, key, path, index)
	}
	if id := g.tableID(t.Ref); id != "" {
		return key + asKw + id
	}
	return g.decMapKey(*t.Key, key, path, big)
}

// bigAt is the one decision whether the integers at position t of a field are bigint, big being the field's @ts(bigint); the type, the literal and every decoder, map keys included, take it (CODEGEN.md §4.1, DECISIONS 279(a)). A ref takes its target key's form only: bigint when its key field is @ts(bigint) (DECISIONS 278); every other position takes the field's flag.
func bigAt(t ir.TypeRef, big bool) bool {
	if t.Kind == types.Ref {
		return t.Ref != nil && t.Ref.BigInt
	}
	return big
}

// decKey is a ref's key: its table's id string, or a key of the key type the IR gives (CODEGEN.md §5.8).
func (g *gen) decKey(t ir.TypeRef, raw, path string, c decCtx) string {
	if t.Key == nil {
		g.failf(ErrMalformed, malformedNoKey, g.at)
		return tsUndefined
	}
	if index := g.entryIndex(t.Ref); index != "" {
		return g.call(decEntryName, raw, path, index)
	}
	if id := g.tableID(t.Ref); id != "" {
		return g.call(decStringName, raw, path) + asKw + id
	}
	return g.dec(*t.Key, raw, path, c)
}

// need marks a class as read by a decoder; another package's is kept in the order it was needed.
func (g *gen) need(t ir.Type) {
	if !g.decoded[t] && pkgOf(t) != g.p.Name {
		g.needed = append(g.needed, t)
	}
	g.decoded[t] = true
}

// entryIndex is the Index of the id type of a table another package's baked or embedded ts emit holds, which a ref into it is checked against (`no entry <key>`, CODEGEN.md §2.8 Refs); "" when the generator does not know the ids: a table of this package (read in data or types mode, its ids are the file's), or of a data or types owner.
func (g *gen) entryIndex(r *ir.RefTarget) string {
	if !g.knownIDs(r) {
		return ""
	}
	return g.importValue(r.Pkg, idName(r.Elem)+indexSuffix)
}

// knownIDs reports a ref into a table another package's baked or embedded ts emit holds: its ids are that emit's id union, known when this file is generated.
func (g *gen) knownIDs(r *ir.RefTarget) bool {
	if !isTableRef(r) || r.Elem == nil || r.Pkg == g.p.Name {
		return false
	}
	o := g.tsEmit(r.Pkg)
	return o != nil && (o.Mode == ir.ModeBaked || o.Mode == ir.ModeEmbedded)
}
