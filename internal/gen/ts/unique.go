package tsgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// unique is rows, the read of a keyed list's or table's rows at path, refused when two rows share a key: the property prop of a row, found on the wire at keyPath, of the type kt (WIRE.md §5.7, CODEGEN.md §5.15).
func (g *gen) unique(rows, path, prop, keyPath string, kt ir.TypeRef) string {
	args := []string{rows, path, fmt.Sprintf(rowKeyFormat, prop), quote(keyPath)}
	if token := g.keyToken(kt); token != "" {
		args = append(args, token)
	}
	return g.call(decUniqueName, args...)
}

// keyedUnique is the read of a keyed list, list, held to unique keys: its element record's key field decides the property and the path.
func (g *gen) keyedUnique(t ir.TypeRef, list, path string) string {
	rec, ok := g.elem(t).Named.(*ir.Record)
	if !ok {
		g.failf(ErrMalformed, malformedDecl, g.at)
		return list
	}
	key := g.keyField(t, rec)
	return g.unique(list, path, fieldProp(key), strings.Join(key.WirePath, dot), key.Type)
}

// keyToken is the function writing a key of type t as the data wire does when the default (a String quoted, a number or bigint in decimal) is not it, else "": an enum with @json(codes) is its code, a ref to a keyed list reads as its key type (WIRE.md §5.3, §7.3, §5.8).
func (g *gen) keyToken(t ir.TypeRef) string {
	if e, ok := t.Named.(*ir.Enum); ok && t.Kind == types.Enum && e.JSONCodes {
		return fmt.Sprintf(codeWireFormat, g.codesConst(e))
	}
	if t.Kind == types.Ref && t.Key != nil && !isTableRef(t.Ref) {
		return g.keyToken(*t.Key)
	}
	return ""
}
