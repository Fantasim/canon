package tsgen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// recordLit is a record value outside a table as an object literal: the fields in order, then the precomputed fns; a row record held there, a keyed list's element included, has no `id` or `retired` (CODEGEN.md §5.4, DECISIONS 279(c)).
func (g *gen) recordLit(rec *ir.Record, r *value.Record) string {
	return g.objectLit(nil, rec.Fields, rec.Methods, r)
}

// rowLit is a table's row as an object literal: `id` and `retired` first, then what recordLit writes; they are written only where the value sits in a table, whichever package's table it is: this one's, or one inside another package's record (CODEGEN.md §5.4, §5.9, DECISIONS 279(c), 323).
func (g *gen) rowLit(rec *ir.Record, r *value.Record) string {
	return g.objectLit(g.entryParts(r), rec.Fields, rec.Methods, r)
}

// entryParts are `id: "open", retired: false` from the row's identity.
func (g *gen) entryParts(r *value.Record) []string {
	if r.Ident == nil {
		g.failf(ErrMalformed, malformedNoIdent, g.at)
		return nil
	}
	return []string{
		idProp + keyValueSep + g.keyLit(ir.TypeRef{}, r.Ident.Key, false),
		retiredProp + keyValueSep + strconv.FormatBool(r.Ident.Retired),
	}
}

// objectLit is `{ parts, fields, fns }` for a record or case value.
func (g *gen) objectLit(parts []string, fields []*ir.Field, fns []*ir.ExportFn, r *value.Record) string {
	if len(r.Fields) != len(fields) {
		g.failf(ErrMalformed, malformedFields, g.at, len(r.Fields), len(fields))
		return tsUndefined
	}
	o := &owner{fields: fields, rec: r}
	for i, f := range fields {
		if f.Input != nil || f.Optional && f.Type.Kind == types.Never {
			continue
		}
		parts = append(parts, property(fieldProp(f))+keyValueSep+g.fieldLit(f, r.Fields[i], o))
	}
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			parts = append(parts, property(fnProp(fn))+keyValueSep+g.methodLit(fn, r))
		}
	}
	if len(parts) == 0 {
		return emptyObject
	}
	return lbrace + space + strings.Join(parts, listSep) + space + rbrace
}

// fieldLit is a field's value: null for none, a dependent value in the branch its record's discriminant selects, else the literal of its type.
func (g *gen) fieldLit(f *ir.Field, v value.Value, o *owner) string {
	if v == nil {
		g.failf(ErrMalformed, malformedNoValue, f.Name, g.at)
		return tsUndefined
	}
	if _, none := v.(*value.None); none {
		return tsNull
	}
	if app := ir.HeldApp(f.Type); app != nil {
		return g.dependentLit(f.Type, v, o)
	}
	return g.lit(f.Type, v, f.BigInt)
}

// variantLit is `{ kind: "wire", ...fields }`; a case without fields is the kind alone (CODEGEN.md §5.5).
func (g *gen) variantLit(t ir.TypeRef, r *value.Record) string {
	v, ok := t.Named.(*ir.Variant)
	var ct *types.CaseType
	isCase := false
	if r.T != nil {
		ct, isCase = r.T.Base().(*types.CaseType)
	}
	if !ok || !isCase || ct.Index < 0 || ct.Index >= len(v.Cases) {
		g.failf(ErrMalformed, malformedVariant, g.at)
		return tsUndefined
	}
	c := v.Cases[ct.Index]
	return g.objectLit([]string{kindProp + keyValueSep + quote(c.Wire)}, c.Fields, c.Methods, r)
}

// dependentLit is a dependent value, or a list of them, as `{ branch, value }` (CODEGEN.md §5.6).
func (g *gen) dependentLit(t ir.TypeRef, v value.Value, o *owner) string {
	if t.Kind == types.List {
		elems := as[value.List](g, v).Elems
		items := make([]string, len(elems))
		for i, x := range elems {
			items[i] = g.dependentLit(g.elem(t), x, o)
		}
		return lbracket + strings.Join(items, listSep) + rbracket
	}
	d, ok := t.Named.(*ir.Dependent)
	if !ok {
		g.failf(ErrMalformed, malformedDependent, g.at)
		return tsUndefined
	}
	br := g.bakedBranch(d, t, o)
	if br < 0 {
		return tsUndefined
	}
	b := d.Branches[br]
	return fmt.Sprintf(branchValueFormat, quote(b.Name), g.lit(b.Type, v, false))
}

// bakedBranch is the index of the branch the record's discriminant selects, read down ir.DiscFields' path; -1 after a failure.
func (g *gen) bakedBranch(d *ir.Dependent, app ir.TypeRef, o *owner) int {
	path := ir.DiscFields(o.fields, app)
	if path == nil {
		g.failf(ErrMalformed, malformedNoDisc, g.at)
		return -1
	}
	disc := value.Value(o.rec)
	fields := o.fields
	for _, f := range path {
		rec := as[value.Record](g, disc)
		i := slices.Index(fields, f)
		if i < 0 || i >= len(rec.Fields) {
			g.failf(ErrMalformed, malformedNoDisc, g.at)
			return -1
		}
		disc = rec.Fields[i]
		fields = nil
		if r, ok := f.Type.Named.(*ir.Record); ok {
			fields = r.Fields
		}
	}
	m := -1
	switch x := disc.(type) {
	case *value.Member:
		m = x.Index
	case *value.Bool:
		m = boolIndex(x.V)
	}
	if m < 0 || m >= len(d.ByMember) || d.ByMember[m] < 0 || d.ByMember[m] >= len(d.Branches) {
		g.failf(ErrMalformed, malformedBranch, d.QName())
		return -1
	}
	return d.ByMember[m]
}

// boolIndex is a Bool's place in a domain: false, then true (CODEGEN.md §5.10).
func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}

// methodLit is the value of a precomputed or finite-parameter fn on the receiver r: its result, or a frozen nested object keyed by the arguments' wire keys (CODEGEN.md §5.10).
func (g *gen) methodLit(fn *ir.ExportFn, r *value.Record) string {
	in := g.instanceOf(fn, r)
	if len(fn.Params) == 0 {
		return g.lit(fn.Result, in.Result, false)
	}
	if in.Table == nil {
		g.failf(ErrMalformed, malformedNoTable, fn.Name, g.at)
		return tsUndefined
	}
	return g.cellsLit(fn, in.Table, 0, 0)
}

// cellsLit is the nested object of a lookup's cells from parameter dim on, the cells starting at offset.
func (g *gen) cellsLit(fn *ir.ExportFn, tab *ir.LookupTable, dim, offset int) string {
	if dim == len(fn.Params) {
		return g.lit(fn.Result, tab.Cells[offset], false)
	}
	stride := 1
	for _, d := range tab.Domains[dim+1:] {
		stride *= len(d)
	}
	items := make([]string, len(tab.Domains[dim]))
	for i, key := range tab.Domains[dim] {
		items[i] = property(g.domainKey(fn.Params[dim].Type, key)) + keyValueSep + g.cellsLit(fn, tab, dim+1, offset+i*stride)
	}
	return lbrace + space + strings.Join(items, listSep) + space + rbrace
}

// domainKey is the wire key of a finite parameter's value (WIRE.md §5.8): an enum's wire, "true" or "false", a ref's key.
func (g *gen) domainKey(t ir.TypeRef, v value.Value) string {
	switch x := v.(type) {
	case *value.Bool:
		return strconv.FormatBool(x.V)
	case *value.Member:
		return g.memberWire(t.Named, x.Index)
	case *value.Ref:
		return x.Key.Text()
	}
	g.failf(ErrMalformed, malformedDomain, v, g.at)
	return ""
}
