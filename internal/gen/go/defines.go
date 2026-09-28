package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// defineTables writes each define table the fields ref, defines<Table>, sorted by name (CODEGEN.md §5.8).
func (g *gen) defineTables() {
	for _, d := range ir.OwnDefines(g.p) {
		if len(d.Names) != len(d.Values) {
			g.failf(ErrMalformed, "define table %s.%s without a value per name", d.Pkg, d.Value)
			continue
		}
		rows := make([]string, len(d.Names))
		for i, n := range d.Names {
			rows[i] = fmt.Sprintf(defineRowFormat, strconv.Quote(n), d.Values[i])
		}
		g.printf(definesVarFormat, g.names.DefinesVar(d), defineRowType, strings.Join(rows, ""))
	}
}

// defineTable is the table a define slot refs, which the IR must hold.
func (g *gen) defineTable(s *slot) *ir.DefineTable {
	d := ir.DefinesOf(g.p, s.refType().Ref)
	if d == nil {
		g.failf(ErrMalformed, "%s refs a load.defines table the package's IR does not hold", s.origin)
	}
	return d
}

// defineType is what a define slot's value getter returns: int64, or an rt.List of one per key.
func (g *gen) defineType(s *slot) string {
	if s.List {
		return g.rt() + listType + lbracket + goInt64 + rbracket
	}
	return goInt64
}

// defineValue is a baked define ref's value, found at generation time (CODEGEN.md §5.8): baked data needs no lookup.
func (g *gen) defineValue(s *slot, v value.Value) string {
	d := g.defineTable(s)
	if d == nil {
		return zeroLit
	}
	find := func(key value.Key) string {
		if i, ok := slices.BinarySearch(d.Names, key.S); ok {
			return strconv.FormatInt(d.Values[i], decimal)
		}
		g.failf(ErrMalformed, "%s: define %s is not in %s.%s", s.origin, key.S, d.Pkg, d.Value)
		return zeroLit
	}
	if !s.List {
		return find(as[value.Ref](g, v).Key)
	}
	elems := as[value.List](g, v).Elems
	items := make([]string, len(elems))
	for i, x := range elems {
		items[i] = find(as[value.Ref](g, x).Key)
	}
	return g.rt() + makeList + sliceOf + goInt64 + braced(items) + rparen
}

// readDefine looks a define slot's keys on recv up once read, loc naming them (CODEGEN.md §5.8, as C++'s defineLookup).
func (g *gen) readDefine(s *slot, recv string, loc location) {
	if !s.Define {
		return
	}
	d := g.defineTable(s)
	if d == nil {
		return
	}
	lc, keys := g.lc, recv+dot+s.KeyStore
	lookup := func(b *strings.Builder, at location, key string) string {
		v := g.temp(tempValue)
		prefix, k := g.splitLoc(at)
		fmt.Fprintf(b, defineReadFormat, v, lc.Err, g.helper(helperDefine), lc.Name, prefix, k,
			g.names.DefinesVar(d), strconv.Quote(ir.DefineTableName(d)), key)
		return v
	}
	var b strings.Builder
	if s.Optional {
		fmt.Fprintf(&b, ifOpenFormat, recv+dot+s.OKStore)
	}
	if !s.List {
		fmt.Fprintf(&b, assignFormat, recv, s.ValueStore, lookup(&b, loc, keys))
	} else {
		vs, i := g.temp(tempValue), g.temp(tempIndex)
		fmt.Fprintf(&b, defineListOpenFormat, vs, keys, i)
		fmt.Fprintf(&b, defineListElemFormat, vs, i, lookup(&b, loc.index(i), keys+atCall+i+rparen))
		fmt.Fprintf(&b, assignFormat, recv, s.ValueStore, g.rt()+makeList+vs+rparen)
	}
	if s.Optional {
		b.WriteString(closeBrace)
	}
	g.body.WriteString(b.String())
}
