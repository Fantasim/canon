package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// helper is the name of a per-file loader helper; a file with decoders writes every helper.
func (g *gen) helper(name string) string { return name }

// helpers writes the per-file helpers the loaders and decoders call, in a fixed order, jsonRowID
// only beside a table: the reads of canon_runtime_json.h's Decoder, with the same messages.
func (g *gen) helpers() {
	view := struct {
		JSON, FMT, Strings, Slices, RT, UTF8 string
		L                                    locals
	}{g.use(jsonPath, jsonPkg), g.use(fmtPkg, fmtPkg), g.use(stringsPkg, stringsPkg), g.use(slicesPkg, slicesPkg), g.rt(), "", g.lc}
	if g.isTypes() { // CODEGEN.md §5.13: the document check, and a source-wire Duration's exact read when one is read
		view.UTF8 = g.use(utf8Path, utf8Pkg)
		g.exec(helperDocument, view)
	}
	if g.isTypes() && (g.names.ReadsDurations() || g.called[helperDuration]) {
		g.exec(helperDuration, view)
	}
	for _, name := range helperOrder {
		g.exec(name, view)
	}
	if g.names.HasMaps() || g.called[helperMap] {
		g.exec(helperKeyPath, view)
		g.exec(helperMap, view)
	}
	if len(ir.EmitDefines(g.p, g.e)) > 0 {
		g.exec(helperDefine, view)
	}
	if g.names.HasNestedTables() || g.called[helperTable] { // a reader of another package's class may read one (CODEGEN.md §2.8)
		g.exec(helperTable, view)
	}
	for _, v := range g.emitted {
		if v.Type.Kind == types.Table {
			g.exec(helperRowID, view)
			return
		}
	}
}

// openObject reads raw as the decoder's object and refuses a key differing from one of keys
// only in letter case.
func (g *gen) openObject(keys []string) {
	lc := g.lc
	if len(keys) == 0 {
		g.printf(objectOnlyFormat, lc.Err, g.helper(helperObject), lc.Name, lc.Path, lc.Raw)
		return
	}
	g.printf(openObjectFormat, lc.Obj, lc.Err, g.helper(helperObject), lc.Name, lc.Path, lc.Raw)
	g.keysCheck(lc.Obj, g.root(), keys)
}

// keysCheck fails the load on a key of obj differing from one of keys only in letter case (WIRE.md §5.5.2); a types-mode decoder ignores unknown keys (CODEGEN.md §5.13).
func (g *gen) keysCheck(obj string, loc location, keys []string) {
	if g.isTypes() {
		return
	}
	quoted := make([]string, 0, len(keys))
	for _, k := range slices.Compact(slices.Sorted(slices.Values(keys))) {
		quoted = append(quoted, strconv.Quote(k))
	}
	if len(quoted) > 0 {
		g.printf(keysFormat, g.lc.Err, g.helper(helperKeys), g.lc.Name, g.locExpr(loc), obj, strings.Join(quoted, listSep))
	}
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

// missing is rt.Missing of the key expression key under the path prefix.
func (g *gen) missing(prefix location, key string) string {
	return fmt.Sprintf(missingFormat, g.ownRT(), g.lc.Name, g.locExpr(prefix), key)
}

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

// asciiFold reports a and b equal but for the case of ASCII letters, the loaders' case rule (rt.EqualFold).
func asciiFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}
