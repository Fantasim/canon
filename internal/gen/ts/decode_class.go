package tsgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// reading is the statements of a reader so far: its locals, and the properties of the object it returns.
type reading struct {
	stmts  []string
	props  []string
	locals map[*ir.Field]string // the local holding a field's decoded value: a dependent field's discriminant reads it
	fields []*ir.Field
}

// local is a fresh numbered local of the reader being written.
func (g *gen) local() string {
	g.temps++
	return localPrefix + strconv.Itoa(g.temps)
}

// recordReader is `function readT(raw, path[, id])`: the object, then every field in declaration order, then the `$` keys of its fns (WIRE.md §5.5).
func (g *gen) recordReader(r *ir.Record) string {
	name := g.declare(readerName(r), r.QName())
	rd := &reading{locals: map[*ir.Field]string{}, fields: r.Fields}
	if g.entries[r] || len(r.Fields)+len(r.Methods) > 0 {
		rd.stmts = append(rd.stmts, fmt.Sprintf(localFormat, objVar, g.call(decObjectName, rawParam, pathParam)))
	}
	if g.entries[r] {
		g.entryRead(rd, g.loose[r])
	}
	g.bodyRead(rd, r.Fields, r.Methods)
	obj := g.objectText(rd)
	params := rawParam + keyValueSep + tsUnknown + listSep + pathParam + keyValueSep + tsString
	switch {
	case g.loose[r]:
		params += listSep + idParam + optionalMark + keyValueSep + tsString + unionSep + tsNull
	case g.entries[r]:
		params += listSep + idParam + optionalMark + keyValueSep + tsString
	}
	return fmt.Sprintf(readerFormat, name, params, typeName(r), g.readerBody(rd, obj))
}

// readerBody is the statements of a reader and its return.
func (g *gen) readerBody(rd *reading, obj string) string {
	var b strings.Builder
	for _, s := range rd.stmts {
		b.WriteString(indent + s + newline)
	}
	fmt.Fprintf(&b, returnFormat, indent, obj)
	return b.String()
}

// objectText is the returned object literal.
func (g *gen) objectText(rd *reading) string {
	if len(rd.props) == 0 {
		return emptyObject
	}
	return lbrace + space + strings.Join(rd.props, listSep) + space + rbrace
}

// entryRead reads a table row's `$id` (its key when it is a nested table's) and `$retired` (WIRE.md §5.7); a loose row's reader reads them only in table position, where id is given, null for a top-level table (CODEGEN.md §5.4).
func (g *gen) entryRead(rd *reading, loose bool) {
	id, retired := g.local(), g.local()
	g.use(decStringName, decGetName, decBoolName)
	if !loose {
		rd.stmts = append(rd.stmts, fmt.Sprintf(localFormat, id, idReadFormat), fmt.Sprintf(localFormat, retired, retiredReadFormat))
		rd.props = append(rd.props, idProp+keyValueSep+id, retiredProp+keyValueSep+retired)
		return
	}
	rd.stmts = append(rd.stmts, fmt.Sprintf(localFormat, id, looseIDFormat+idReadFormat), fmt.Sprintf(localFormat, retired, looseIDFormat+retiredReadFormat))
	rd.props = append(rd.props, fmt.Sprintf(looseEntryFormat, id, id, retired))
}

// bodyRead reads the fields and then the fn keys of a record or case.
func (g *gen) bodyRead(rd *reading, fields []*ir.Field, fns []*ir.ExportFn) {
	for _, f := range fields {
		if f.Input != nil || f.Optional && f.Type.Kind == types.Never {
			continue
		}
		g.fieldRead(rd, f)
	}
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			g.fnRead(rd, fn)
		}
	}
}

// variantReader reads the tag, then the case's fields from the same object (WIRE.md §5.6).
func (g *gen) variantReader(v *ir.Variant) string {
	name := g.declare(readerName(v), v.QName())
	if v.Tag == "" {
		g.failf(ErrMalformed, malformedNoTag, v.QName())
	}
	var arms, cases []string
	for _, c := range v.Cases {
		arms = append(arms, g.caseBranch(v, c))
		if hasInterface(c) {
			cases = append(cases, g.caseReader(v, c))
		}
	}
	tagPath := pathParam + plusSep + quote(pathSep+v.Tag)
	body := fmt.Sprintf(variantBodyFormat, objVar, g.call(decObjectName, rawParam, pathParam), tagVar, g.call(decStringName, g.getKey(objVar, v.Tag), tagPath),
		tagVar, strings.Join(arms, ""), g.call(decFailName, tagPath, quote(unknownCase)))
	params := rawParam + keyValueSep + tsUnknown + listSep + pathParam + keyValueSep + tsString
	return strings.Join(append([]string{fmt.Sprintf(readerFormat, name, params, typeName(v), body)}, cases...), newline)
}

// caseBranch is a `case "wire":` of a variant reader: a case with an interface is read by its own reader, its `$` keys included, one without is its kind alone.
func (g *gen) caseBranch(v *ir.Variant, c *ir.Case) string {
	if !hasInterface(c) {
		return fmt.Sprintf(caseArmFormat, quote(c.Wire), fmt.Sprintf(bareLitFormat, quote(c.Wire)))
	}
	return fmt.Sprintf(caseArmFormat, quote(c.Wire), call1(readPrefix+caseName(v, c), objVar, pathParam))
}

// caseReader is `function readVCase(o, path)`: the case's fields from the variant's object.
func (g *gen) caseReader(v *ir.Variant, c *ir.Case) string {
	g.temps = 0
	name := g.declare(readPrefix+caseName(v, c), v.QName()+dot+c.Name)
	rd := &reading{locals: map[*ir.Field]string{}, fields: c.Fields}
	rd.props = append(rd.props, kindProp+keyValueSep+quote(c.Wire))
	g.bodyRead(rd, c.Fields, c.Methods)
	body := g.readerBody(rd, g.objectText(rd))
	params := objVar + keyValueSep + objectType + listSep + pathParam + keyValueSep + tsString
	return fmt.Sprintf(readerFormat, name, params, caseName(v, c), body)
}

// dependentReader is `function readD(raw, path, disc)`: the untagged wire read in the branch the discriminant selects; a member no branch covers is refused (CODEGEN.md §5.6).
func (g *gen) dependentReader(d *ir.Dependent) string {
	name := g.declare(readerName(d), d.QName())
	discType, labels := g.discLabels(d)
	if labels == nil {
		g.failf(ErrMalformed, malformedNoDisc, d.QName())
		return ""
	}
	var arms strings.Builder
	for i, b := range d.Branches {
		cases := make([]string, len(b.Members))
		for j, m := range b.Members {
			cases[j] = fmt.Sprintf(caseLabelFormat, labels[m])
		}
		read := g.dec(b.Type, rawParam, pathParam, decCtx{})
		arms.WriteString(strings.Join(cases, "") + fmt.Sprintf(branchReturnFormat, fmt.Sprintf(branchValueFormat, quote(d.Branches[i].Name), read)))
	}
	g.use(decFailName)
	body := fmt.Sprintf(dependentBodyFormat, arms.String())
	params := rawParam + keyValueSep + tsUnknown + listSep + pathParam + keyValueSep + tsString + listSep + discVar + keyValueSep + discType
	return fmt.Sprintf(readerFormat, name, params, typeName(d), body)
}

// discLabels are the type of a dependent type's discriminant and each member's case label, in member order: `false` and `true` for a Bool, the members' wires for an enum.
func (g *gen) discLabels(d *ir.Dependent) (string, []string) {
	if d.Disc == nil {
		return "", nil
	}
	if d.Disc.Kind == types.Bool {
		return tsBoolean, []string{strconv.FormatBool(false), strconv.FormatBool(true)}
	}
	e, ok := d.Disc.Named.(*ir.Enum)
	if !ok || d.Disc.Kind != types.Enum {
		return "", nil
	}
	labels := make([]string, len(e.Members))
	for i, m := range e.Members {
		labels[i] = quote(m.Wire)
	}
	return g.named(e), labels
}
