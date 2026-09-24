package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// helpers writes jsonObject, jsonError, jsonCase and, with a table, jsonRowID: the reads every
// decoder shares, keys matched exactly (log-2026-09-24, gen/go review calls).
func (g *gen) helpers() {
	if len(g.decoded) == 0 {
		return
	}
	rows := false
	for _, v := range g.emitted {
		rows = rows || v.Type.Kind == types.Table
	}
	g.exec(helpersTemplate, struct {
		JSON, FMT, Errors, Strings, RT string
		Rows                           bool
		L                              locals
	}{
		g.use(jsonPath, jsonPkg), g.use(fmtPkg, fmtPkg), g.use(errorsPkg, errorsPkg),
		g.use(stringsPkg, stringsPkg), g.rt(), rows, g.lc,
	})
}

// openObject reads raw as an object and refuses a key matching an expected one only case-insensitively, naming the smallest in byte order (DOCTRINE §5).
func (g *gen) openObject(keys []string) {
	lc := g.lc
	g.printf(openObjectFormat, lc.Obj, lc.Err, helperObject, lc.Name, lc.Path, lc.Raw)
	var b strings.Builder
	g.caseLoop(&b, lc.Obj, lc.Path, keys)
	g.body.WriteString(b.String())
}

// caseLoop fails the load on a key of the map obj that equals one of keys only ignoring case,
// the smallest in byte order; prefix is the Go expression of the map's path, with its dot.
func (g *gen) caseLoop(b *strings.Builder, obj, prefix string, keys []string) {
	quoted := make([]string, 0, len(keys))
	for _, k := range slices.Compact(slices.Sorted(slices.Values(keys))) {
		quoted = append(quoted, strconv.Quote(k))
	}
	if len(quoted) == 0 {
		return
	}
	lc := g.lc
	fmt.Fprintf(b, foldLoopFormat, g.temp(tempKey), obj, strings.Join(quoted, listSep), lc.Err, helperCase, lc.Name, prefix,
		g.temp(tempBad), helperFolds)
}

// expectedKeys are every key a record's or case's object holds: its own, its extras, and the
// keys of the cases of a variant written inline in it.
func (g *gen) expectedKeys(b *body) []string {
	keys := append(g.objectKeys(b), g.objectExtras(b)...)
	for _, s := range b.slots {
		v, ok := s.T.Named.(*ir.Variant)
		if s.src == nil || !s.src.Inline || !ok {
			continue
		}
		for _, c := range v.Cases {
			if len(c.Fields) > 0 {
				keys = append(keys, g.objectKeys(g.caseBody(v, c))...)
			}
		}
	}
	return keys
}

// nullAt is the error of a null the file may not hold at loc.
func (g *gen) nullAt(loc location) string { return g.errAt(loc, nullText, "") }

// bitsMask is the OR of an enum's codes, which a bits value's members are (WIRE.md §5.3); a negative code has no bit.
func (g *gen) bitsMask(e *ir.Enum) string {
	var mask int64
	for _, m := range e.Members {
		if m.Code < 0 {
			g.failf(ErrMalformed, "bits over the negative code of %s.%s", e.QName(), m.Name)
		}
		mask |= m.Code
	}
	return fmt.Sprintf(hexFormat, mask)
}

// checkBits fails the load on a bit no member's code holds, named in hex.
func (g *gen) checkBits(b *strings.Builder, e *ir.Enum, src string, loc location) {
	rest := g.temp(tempValue)
	fmt.Fprintf(b, bitsCheckFormat, rest, src, g.bitsMask(e), g.errAt(loc, unknownBitsText, rest))
}
