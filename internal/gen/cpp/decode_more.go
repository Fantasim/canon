package cppgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// decodeList reads an array element by element; a keyed list also collects the keys (§4.2).
func (g *gen) decodeList(depth int, src, key string, l leaf) {
	i, e := fmt.Sprintf(indexVarFormat, depth), fmt.Sprintf(elemVarFormat, depth)
	rows, keys := fmt.Sprintf(rowsVarFormat, depth), fmt.Sprintf(keysVarFormat, depth)
	elem := g.storage(*l.t.Elem)
	var kf *ir.Field
	if l.t.KeyedBy != nil {
		if kf = g.keyField(l.t); kf == nil {
			return
		}
	}
	inner, body := depth+1, depth+depthTwo
	g.c.linef(depth, notArrayFormat, src)
	g.c.linef(inner, failArrayFormat, key)
	g.c.linef(depth, elseOpen)
	if kf != nil {
		g.c.linef(inner, vectorDeclFormat, elem, rows)
		g.c.linef(inner, vectorDeclFormat, g.storage(kf.Type), keys)
	}
	g.c.linef(inner, forFormat, i, i, src, i)
	g.c.linef(body, localFormat, elem, e, elemInit(*l.t.Elem))
	g.decodeValue(body, fmt.Sprintf(indexFormat, src, i), elemKey(key, i), leaf{t: *l.t.Elem, unit: l.unit, enc: l.enc, dst: e})
	if kf != nil {
		g.c.linef(body, pushBackKeyFormat, keys, e, g.getterName(kf))
		g.c.linef(body, pushBackFormat, rows, e)
	} else {
		g.c.linef(body, pushBackFormat, l.dst, e)
	}
	g.c.linef(inner, closeBrace)
	if kf != nil {
		g.c.linef(inner, fromRowsFormat, l.dst, g.storage(kf.Type), elem, rows, keys)
	}
	g.c.linef(depth, closeBrace)
}

// elemInit value-initializes a scalar element; a class element is default-constructed.
func elemInit(t ir.TypeRef) string {
	if memberInit(t, false) != "" {
		return initBraces
	}
	return ""
}

// elemKey is the decoder path of element i: `"k[" + std::to_string(i) + "]"`.
func elemKey(key, i string) string {
	if strings.HasPrefix(key, quoteMark) && strings.HasSuffix(key, quoteMark) && !strings.Contains(key, concat) {
		return fmt.Sprintf(elemKeyLitFormat, key[1:len(key)-1], i)
	}
	return fmt.Sprintf(elemKeyExprFormat, key, i)
}

// decodeVariant reads the tag, then the case's fields from the same object (WIRE.md §5.6).
func (g *gen) decodeVariant(v *ir.Variant) {
	if v.Tag == "" {
		g.fail(fmt.Errorf("%w: variant %s without a tag", ErrMalformed, v.Name))
		return
	}
	g.c.linef(1, tagDeclLine)
	g.c.linef(1, tagReadFormat, quote(v.Tag))
	for i, c := range v.Cases {
		open := elseIfTagFormat
		if i == 0 {
			open = ifTagFormat
		}
		g.c.linef(1, open, quote(c.Wire))
		if len(c.Fields) > 0 {
			g.c.linef(depthTwo, emplaceDecodeFormat, i)
		} else {
			g.c.linef(depthTwo, emplaceIndexFormat, i)
		}
	}
	g.c.linef(1, elseOpen)
	g.c.linef(depthTwo, unknownCaseFormat, quote(v.Tag))
	g.c.linef(1, closeBrace)
}

// decodeTable reads a finite-parameter method's nested `$<fn>` object into its table (WIRE.md §5.11).
func (g *gen) decodeTable(fn *ir.ExportFn, l leaf) {
	doms := g.domains(fn)
	obj := fmt.Sprintf(tableVarFormat, 0)
	g.c.linef(1, objectOpenFormat, obj, sourceVar, quote(dollar+fn.Name))
	g.c.linef(depthTwo, pushFormat, quote(dollar+fn.Name))
	for i, d := range doms {
		quoted := make([]string, len(d.keys))
		for k, key := range d.keys {
			quoted[k] = quote(key)
		}
		g.c.linef(depthTwo, keysArrayFormat, fmt.Sprintf(wireKeysFormat, i), strings.Join(quoted, listSep))
	}
	index, depth := "", depthTwo
	for i, d := range doms {
		a := fmt.Sprintf(argVarFormat, i)
		g.c.linef(depth, countForFormat, a, a, len(d.keys), a)
		index = rowMajor(index, len(d.keys), a)
		key := fmt.Sprintf(indexFormat, fmt.Sprintf(wireKeysFormat, i), a)
		if i+1 < len(doms) {
			next := fmt.Sprintf(tableVarFormat, i+1)
			g.c.linef(depth+1, objectOpenFormat, next, fmt.Sprintf(derefFormat, obj), key)
			g.c.linef(depth+depthTwo, pushFormat, key)
			obj, depth = next, depth+depthTwo
			continue
		}
		cell := l
		cell.dst = fmt.Sprintf(indexFormat, l.dst, index)
		g.decodeKey(depth+1, fmt.Sprintf(derefFormat, obj), key, cell)
	}
	for i := len(doms) - 1; i > 0; i-- {
		g.c.linef(depth, closeBrace)
		g.c.linef(depth, popLine)
		g.c.linef(depth-1, closeBrace)
		depth -= depthTwo
	}
	g.c.linef(depth, closeBrace)
	g.c.linef(depthTwo, popLine)
	g.c.linef(1, closeBrace)
}

// rowMajor extends the row-major index with the next argument's ordinal: index × size + ord.
func rowMajor(index string, size int, ord string) string {
	switch {
	case index == "":
		return ord
	case strings.Contains(index, space):
		index = fmt.Sprintf(parenFormat, index)
	}
	return fmt.Sprintf(rowMajorFormat, index, size, ord)
}

// markerTest is the C++ test that *x holds the @json(none:) marker, JSON-equal (WIRE.md §5.4).
func (g *gen) markerTest(x string, marker []byte) string {
	var v any
	dec := json.NewDecoder(bytes.NewReader(marker))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		g.fail(fmt.Errorf("%w: none marker %s: %w", ErrMalformed, marker, err))
		return cppInvalid
	}
	switch m := v.(type) {
	case json.Number:
		if i, err := m.Int64(); err == nil {
			return fmt.Sprintf(markerIntFormat, x, x, intLit(i))
		}
		return fmt.Sprintf(markerNumberFormat, x, x, m.String())
	case string:
		return fmt.Sprintf(markerStringFormat, x, x, quote(m))
	case bool:
		return fmt.Sprintf(markerBoolFormat, x, x, m)
	case map[string]any:
		if len(m) == 0 {
			return fmt.Sprintf(markerEmptyFormat, x, jsonObject, x)
		}
	case []any:
		if len(m) == 0 {
			return fmt.Sprintf(markerEmptyFormat, x, jsonArray, x)
		}
	}
	g.unsupported(fmt.Sprintf(noneMarkerFormat, marker), g.at)
	return cppInvalid
}
