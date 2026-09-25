package cppgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// checkKeys refuses obj unless it is an object none of whose keys equals one of keys only
// ignoring letter case; orReturn makes a failure leave the decoder.
func (g *gen) checkKeys(depth int, obj string, keys []string, orReturn bool) {
	keys = slices.Compact(slices.Sorted(slices.Values(keys)))
	quoted := make([]string, len(keys))
	for i, k := range keys {
		quoted[i] = quote(k)
	}
	list := fmt.Sprintf(bracedFormat, strings.Join(quoted, listSep))
	switch {
	case len(keys) == 0:
		g.c.linef(depth, objectReturnFormat, obj)
	case orReturn:
		g.c.linef(depth, keysReturnFormat, obj, list)
	default:
		g.c.linef(depth, keysStmtFormat, obj, list)
	}
}

// objectKeys are a record's or case's object keys: a row's `$id`, `$retired`, a case's tag, fields', `$` keys (WIRE.md §5.5.1).
func (g *gen) objectKeys(c class) []string {
	fields, fns := c.shape()
	var keys []string
	if c.cs != nil {
		keys = append(keys, c.variant.Tag)
	}
	if c.rec != nil && g.entries[c.rec] {
		keys = append(keys, dollar+ir.GoIDStore, dollar+ir.GoRetiredStore)
	}
	for _, f := range fields {
		keys = append(keys, g.fieldKeys(f)...)
	}
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			keys = append(keys, dollar+fn.Name)
		}
	}
	return keys
}

// fieldKeys are a field's keys in its record's object: its path's first, its pairs slots, an inline variant's (WIRE.md §5.6).
func (g *gen) fieldKeys(f *ir.Field) []string {
	v, isVariant := f.Type.Named.(*ir.Variant)
	switch {
	case f.Input != nil, f.Type.Kind == types.Never && f.Optional:
		return nil
	case f.Pairs != nil:
		var keys []string
		for _, tmpl := range f.Pairs.Keys {
			for i := range f.Pairs.Slots {
				keys = append(keys, g.slotKey(tmpl, i))
			}
		}
		return keys
	case f.Inline && isVariant:
		keys := []string{v.Tag}
		for _, cs := range v.Cases {
			if len(cs.Fields) > 0 {
				keys = append(keys, g.objectKeys(class{variant: v, cs: cs})...)
			}
		}
		return keys
	case len(f.WirePath) > 0:
		return f.WirePath[:1]
	}
	return nil
}

// nextSegments are the keys of the object at a path prefix: the next segment of every field under it (WIRE.md §5.5.3).
func nextSegments(fields []*ir.Field, prefix []string) []string {
	var keys []string
	for _, f := range fields {
		if !f.Inline && f.Pairs == nil && len(f.WirePath) > len(prefix) && slices.Equal(f.WirePath[:len(prefix)], prefix) {
			keys = append(keys, f.WirePath[len(prefix)])
		}
	}
	return keys
}

// inlineFolds refuses an inline variant a key of which equals one of its parent's other keys but
// for ASCII letter case: its case decoder reads the parent's object, and would take that key for
// a misspelling of its own.
func (g *gen) inlineFolds(c class) {
	fields, _ := c.shape()
	all := g.objectKeys(c)
	for _, f := range fields {
		if !f.Inline {
			continue
		}
		own := g.fieldKeys(f)
		for _, k := range own {
			if slices.ContainsFunc(all, func(p string) bool { return !slices.Contains(own, p) && asciiFold(k, p) }) {
				g.malformed(inlineFoldKeys, c.canonName()+qnameSep+f.Name) // E8019 InlineFoldedKey
				return
			}
		}
	}
}

// cellPaths are a lookup's cells as a load error names them, `$<fn>.<key>...`, in row-major order (WIRE.md §5.11).
func cellPaths(fn string, doms []domain) []string {
	paths := []string{dollar + fn}
	for _, d := range doms {
		next := make([]string, 0, len(paths)*len(d.keys))
		for _, p := range paths {
			for _, k := range d.keys {
				next = append(next, p+qnameSep+k)
			}
		}
		paths = next
	}
	return paths
}

// bitsMask is the OR of an enum's codes, the bits a `bits` value may set (WIRE.md §5.3).
func (g *gen) bitsMask(e *ir.Enum) string {
	var mask int64
	for _, m := range e.Members {
		if m.Code < 0 {
			g.fail(fmt.Errorf("%w: bits over the negative code of %s.%s", ErrMalformed, e.Name, m.Name))
		}
		mask |= m.Code
	}
	return fmt.Sprintf(maskFormat, mask)
}

// asciiFold reports a and b equal but for the case of ASCII letters, the loaders' case rule
// (canon::json::EqualFold).
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
