package tsgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// schemas writes each emitted value's schema constant: the fingerprint its data file must carry (CODEGEN.md §3.3, FINGERPRINT.md §2).
func (g *gen) schemas() {
	for _, v := range g.emitted {
		name := g.declare(effective(v.TS, v.Name)+schemaSuffix, v.Name)
		g.add(fmt.Sprintf(constFormat, name, quote(v.Schema)))
	}
}

// decoders writes the readers of every class a decoder reaches and the public decoders: `decode<V>` and `parse<V>` per emitted value in data mode, `decode<T>` and `parse<T>` per public record and variant in types mode (CODEGEN.md §5.9, §5.13, §8.1); this package's classes in declaration order, then this file's readers of other packages' classes (§2.7, §2.8).
func (g *gen) decoders() {
	g.decoderStart = len(g.items)
	for _, v := range g.emitted {
		g.valueDecoder(v)
	}
	if g.isTypes() {
		for _, t := range g.p.Types {
			if docOwner(t) {
				g.need(t)
			}
		}
	}
	readers := g.readers()
	for _, t := range g.p.Types {
		if text, ok := readers[t]; ok {
			g.add(text)
			if g.isTypes() && docOwner(t) {
				g.add(g.publicDecoder(t))
			}
		}
	}
	for _, t := range g.foreignClasses() {
		g.add(readers[t])
	}
}

// readers writes the reader of each needed class once, until no new class is needed: a reader may need others; a class whose reader came out empty (malformed IR, g.err set) is not written again.
func (g *gen) readers() map[ir.Type]string {
	out := map[ir.Type]string{}
	for grown := true; grown; {
		grown = false
		for _, t := range append(slices.Clone(g.p.Types), g.foreignClasses()...) {
			if _, done := out[t]; done || !g.decoded[t] {
				continue
			}
			out[t], grown = g.reader(t), true
		}
	}
	return out
}

// foreignClasses are the classes of other packages this file reads, each once: in the order ir.ForeignUses reaches them, then any other in the order the readers needed them (CODEGEN.md §2.7).
func (g *gen) foreignClasses() []ir.Type {
	if !g.foreignDone {
		g.foreignRead, g.foreignDone = ir.ForeignUses(g.p, g.e).Read, true
	}
	var out []ir.Type
	for _, c := range g.foreignRead {
		if t, ok := c.(ir.Type); ok && g.decoded[t] && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	for _, t := range g.needed {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// reader is the private reader of a class: the object it reads, a variant's tag, a dependent type's discriminant.
func (g *gen) reader(t ir.Type) string {
	defer g.enter(t.QName())()
	g.temps = 0
	switch x := t.(type) {
	case *ir.Record:
		return g.recordReader(x)
	case *ir.Variant:
		return g.variantReader(x)
	case *ir.Dependent:
		return g.dependentReader(x)
	}
	return ""
}

// publicDecoder is `export function decodeT(json: unknown): T` of a types-mode emit, then `parseT(text: string)`, which reads the text with the file's own tokenizer (CODEGEN.md §5.13, §8.1, DECISIONS 278).
func (g *gen) publicDecoder(t ir.Type) string {
	name := g.declare(decodePrefix+typeName(t), t.QName())
	call := fmt.Sprintf(callFormat, g.readerName(t), joinArgs([]string{jsonParam, emptyString}))
	g.use(decParseName)
	parse := fmt.Sprintf(parseFormat, g.declare(parsePrefix+typeName(t), t.QName()), typeName(t), name)
	return docComment("", docOf(t)) + fmt.Sprintf(publicDecoderFormat, name, typeName(t), g.helper(canonFreezeName), call) + newline + parse
}

// docOwner reports a record or variant: the types a types-mode emit decodes publicly (CODEGEN.md §5.13).
func docOwner(t ir.Type) bool {
	switch t.(type) {
	case *ir.Record, *ir.Variant:
		return true
	}
	return false
}

// docOf is the doc of a record or variant.
func docOf(t ir.Type) string {
	switch x := t.(type) {
	case *ir.Record:
		return x.Doc
	case *ir.Variant:
		return x.Doc
	}
	return ""
}

// valueDecoder is `export function decode<V>(json: unknown)` and `parse<V>(text: string)` of a data-mode value: they check the envelope and read the rows or the value, another package's record through this file's own reader; parse reads the text with the file's own tokenizer (CODEGEN.md §2.8, §8.1, WIRE.md §8.2, DECISIONS 278, 323).
func (g *gen) valueDecoder(v *ir.Value) {
	defer g.enter(v.Name)()
	base := upperCamel(effective(v.TS, v.Name))
	name := g.declare(decodePrefix+base, v.Name)
	schema := effective(v.TS, v.Name) + schemaSuffix
	file := quote(v.Name + ir.JSONExt)
	g.use(canonEnvelopeName, decGetName)
	var ret string
	switch {
	case v.Type.Kind == types.Table:
		rec := g.recordOf(g.tableElem(v))
		g.use(canonTableName, canonFreezeName, decListName)
		row := g.rowType(rec, idName(rec))
		ret = fmt.Sprintf(tableTypeFormat, idName(rec), row)
		g.add(fmt.Sprintf(tableDecoderFormat, name, ret, schema, file, g.rowRead(rec, row), idProp))
	case v.Type.Kind == types.List && v.Type.KeyedBy != nil:
		rec := g.recordOf(g.elem(v.Type).Named)
		key := g.keyField(v.Type, rec)
		g.use(canonTableName, canonFreezeName, decListName)
		ret = fmt.Sprintf(tableTypeFormat, g.tsType(key.Type, key.BigInt), g.named(rec))
		g.add(fmt.Sprintf(tableDecoderFormat, name, ret, schema, file, g.decClass(rec, lambdaRaw, lambdaPath), fieldProp(key)))
	case v.Type.Kind == types.Record:
		rec := g.recordOf(v.Type.Named)
		g.use(canonFreezeName)
		ret = g.named(rec)
		g.add(fmt.Sprintf(valueDecoderFormat, name, ret, schema, file, g.decClass(rec, valueRaw, valuePath)))
	default:
		g.failf(ErrMalformed, malformedDataValue, v.Name)
		return
	}
	g.use(decParseName)
	g.add(fmt.Sprintf(parseFormat, g.declare(parsePrefix+base, v.Name), ret, name))
}

// call1 is a call with its arguments joined.
func call1(fn string, args ...string) string {
	return fmt.Sprintf(callFormat, fn, joinArgs(args))
}

func joinArgs(args []string) string { return strings.Join(args, listSep) }

// getKey reads a key of an object, own properties only.
func (g *gen) getKey(obj, key string) string {
	return g.call(decGetName, obj, quote(key))
}

// rowRead is the expression reading a top-level table's row x at p as row, its CanonRow type: a record of this package through its reader, which reads `$id` and `$retired` (asserted present, as in table position they are); another package's through this file's reader of it, then `$id` and `$retired` here (CODEGEN.md §2.8, §5.9, WIRE.md §5.7; DECISIONS 323).
func (g *gen) rowRead(rec *ir.Record, row string) string {
	if rec.Pkg != g.p.Name {
		g.use(decStringName)
		return g.foreignRowRead(rec, foreignRowID)
	}
	read := g.decClass(rec, lambdaRaw, lambdaPath)
	if g.loose[rec] {
		read = call1(g.readerName(rec), lambdaRaw, lambdaPath, tsNull)
	}
	return fmt.Sprintf(ownRowFormat, read, row)
}

// foreignRowRead is the block reading a row of another package's record: the record through this file's reader, then the row's id (id, an expression of the block) and `$retired` (WIRE.md §5.7).
func (g *gen) foreignRowRead(rec *ir.Record, id string) string {
	g.need(rec)
	g.use(decObjectName, decGetName, decBoolName)
	return fmt.Sprintf(foreignRowFormat, g.readerName(rec), id)
}
