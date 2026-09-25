package wire

import (
	"context"
	"math/big"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Cell is one CSV field, unquoted, and where it is.
type Cell struct {
	Text string
	Span source.Span
}

// CSV reads load.csv records as t: records named by a header, else [[String]] (WIRE.md §6.6).
func (d *Decoder) CSV(ctx context.Context, header []Cell, rows [][]Cell, t types.Type) (value.Value, bool, error) {
	r := d.start(ctx, t)
	var v value.Value
	if header == nil {
		v = r.csvStrings(rows, t)
	} else {
		v = r.csvRecords(header, rows, t)
	}
	return r.result(v)
}

// csvStrings is [[String]]: one list per record, cells verbatim.
func (r *run) csvStrings(rows [][]Cell, t types.Type) value.Value {
	outer, ok := t.Base().(*types.ListType)
	if !ok {
		r.misuse(ErrNoWireType, t)
		return nil
	}
	inner, ok := outer.Elem.Base().(*types.ListType)
	if !ok || inner.Elem.Kind() != types.String {
		r.misuse(ErrNoWireType, t)
		return nil
	}
	l := &value.List{T: t, Elems: make([]value.Value, 0, len(rows))}
	for _, row := range rows {
		rl := &value.List{T: outer.Elem, Elems: make([]value.Value, 0, len(row))}
		for _, c := range row {
			rl.Elems = append(rl.Elems, &value.Str{V: c.Text, T: inner.Elem, P: cellAt(c).prov()})
		}
		l.Elems = append(l.Elems, rl)
	}
	return l
}

func cellAt(c Cell) site { return site{span: c.Span, kind: value.ProvCSV} }

// columns maps each field to its column, -1 for none, and finds a table's `$id` column.
type columns struct {
	refused []bool // a field whose column was refused (E7116): it has a column
	of      []int
	id      int
	span    source.Span
}

// csvRecords reads each record as an element of t, with the columns the header names.
func (r *run) csvRecords(header []Cell, rows [][]Cell, t types.Type) value.Value {
	var elem types.Type
	var lt *types.ListType
	switch x := t.Base().(type) {
	case *types.ListType:
		elem, lt = x.Elem, x
	case *types.TableType:
		elem = x.Elem
	default:
		r.misuse(ErrNoWireType, t)
		return nil
	}
	fields, params, ok := r.shape(elem, r.rootFrame())
	if !ok {
		return nil
	}
	cols, ok := r.columns(header, fields, elem, lt == nil)
	recs := make([]*value.Record, len(rows))
	for i, row := range rows {
		recs[i] = r.csvRecord(row, cols, fields, &frame{params: params}, elem)
		ok = ok && recs[i] != nil
	}
	switch {
	case lt == nil:
		return r.csvTable(recs, rows, cols.id, t, ok)
	case !ok:
		return nil
	}
	return r.csvList(recs, t, lt)
}

// columns reads the header: each name the wire name of a field, once (WIRE.md §6.6).
func (r *run) columns(header []Cell, fields []*types.Field, elem types.Type, table bool) (columns, bool) {
	c := columns{of: make([]int, len(fields)), refused: make([]bool, len(fields)), id: -1}
	for i := range c.of {
		c.of[i] = -1
	}
	if len(header) > 0 {
		c.span = spanOf(header[0].Span, header[len(header)-1].Span)
	}
	seen := map[string]bool{}
	ok := true
	for j, h := range header {
		switch {
		case seen[h.Text]:
			r.report(diag.E7104.AtCsv(h.Span, h.Text), nil)
			ok = false
		case table && h.Text == keyID:
			c.id = j
		default:
			ok = r.column(h, j, fields, elem, &c) && ok
		}
		seen[h.Text] = true
	}
	return c, r.required(fields, elem, c, table) && ok
}

// column maps the field the header cell names to column j.
func (r *run) column(h Cell, j int, fields []*types.Field, elem types.Type, c *columns) bool {
	f := fieldNamed(fields, h.Text)
	switch {
	case f == nil && r.d.Partial:
		return true
	case f == nil:
		r.report(diag.E3301.At(h.Span, elem, h.Text), nil)
	case f.Input != nil:
		r.report(diag.E3312.At(h.Span, f.Name), nil)
	case !cellType(f.Type):
		r.report(diag.E7116.At(h.Span, formCSV, f.Type), nil)
		c.refused[f.Index] = true
	default:
		c.of[f.Index] = j
		return true
	}
	return false
}

// required reports, once at the header, each required field without a column, and a table's
// missing `$id` column.
func (r *run) required(fields []*types.Field, elem types.Type, c columns, table bool) bool {
	ok := true
	for i, f := range fields {
		if c.of[i] < 0 && !c.refused[i] && f.Input == nil && f.Pairs == nil && f.Default == nil && f.Type.Kind() != types.Optional {
			r.report(diag.E3302.At(c.span, elem, f.Name), nil)
			ok = false
		}
	}
	if table && c.id < 0 {
		r.report(diag.E3302.At(c.span, elem, keyID), nil)
		ok = false
	}
	return ok
}

// fieldNamed is the field whose one-key wire name is name.
func fieldNamed(fields []*types.Field, name string) *types.Field {
	for _, f := range fields {
		if !f.Inline && f.Pairs == nil && len(f.WirePath) == 1 && f.WirePath[0] == name {
			return f
		}
	}
	return nil
}

// cellType is a field type a CSV cell can hold, possibly optional (WIRE.md §6.6).
func cellType(t types.Type) bool {
	switch t.Kind() {
	case types.Optional:
		return cellType(t.Base().(*types.OptionalType).Elem)
	case types.LitUnion:
		return cellType(t.Base().(*types.LitUnionType).Of)
	case types.Bool, types.Int, types.Float, types.String, types.Duration, types.Enum, types.Ref:
		return true
	default:
		return false
	}
}

// csvRecord reads one record in fr, with the element's arguments; an empty cell is absent (WIRE.md §6.6).
func (r *run) csvRecord(row []Cell, c columns, fields []*types.Field, fr *frame, elem types.Type) *value.Record {
	rv := &value.Record{T: elem, Fields: make([]value.Value, len(fields)), Set: make([]bool, len(fields))}
	if len(row) > 0 {
		rv.P = cellAt(Cell{Span: spanOf(row[0].Span, row[len(row)-1].Span)}).prov()
	}
	r.bound(rv, fr.params)
	r.enter(rv)
	defer r.leave()
	fr.rec = rv
	ok := true
	for i, f := range fields {
		ok = r.csvField(row, c.of[i], f, i, fr) && ok
		fr.failed = !ok
	}
	if !ok {
		return nil
	}
	return rv
}

// csvField reads field i from column j: a missing column or an empty cell is absent, a cell
// equal to the field's none marker is none.
func (r *run) csvField(row []Cell, j int, f *types.Field, i int, fr *frame) bool {
	rv := fr.rec
	if f.Input != nil {
		return true
	}
	if j < 0 || j >= len(row) || row[j].Text == "" {
		v, required := r.fill(f, rv, fr, rv.P)
		if required && j >= 0 && j < len(row) {
			r.report(diag.E3302.At(row[j].Span, rv.T, f.Name), nil)
		}
		rv.Fields[i] = v
		return v != nil
	}
	cell := row[j]
	rv.Set[i] = true
	if f.Type.Kind() == types.Optional && f.NoneWire != nil && cell.Text == markerText(f.NoneWire) {
		rv.Fields[i] = &value.None{T: f.Type, P: cellAt(cell).prov()}
		return true
	}
	rv.Fields[i] = r.cell(cell, f.Type, fieldScope(f, fr))
	return rv.Fields[i] != nil
}

// csvList is the records as a list; a keyed list's elements carry their identity.
func (r *run) csvList(recs []*value.Record, t types.Type, lt *types.ListType) value.Value {
	l := &value.List{T: t, Elems: make([]value.Value, len(recs))}
	coll, _ := r.collection(lt.Elem, lt.KeyedBy, true)
	for i, rec := range recs {
		if lt.KeyedBy != nil {
			identify(rec, lt.KeyedBy, coll, nil)
		}
		l.Elems[i] = rec
	}
	return l
}

// csvTable is the records keyed by their `$id` cells, each an identifier (E7114) used once
// (E3102, DECISIONS 176).
func (r *run) csvTable(recs []*value.Record, rows [][]Cell, id int, t types.Type, ok bool) value.Value {
	if id < 0 {
		return nil
	}
	coll, _ := r.collection(t.Base().(*types.TableType).Elem, nil, true)
	first := map[string]source.Span{}
	for i, rec := range recs {
		key := Cell{}
		if id < len(rows[i]) {
			key = rows[i][id]
		}
		keyOK := r.stem(File{Stem: key.Text, At: key.Span}, first)
		if rec == nil || !keyOK {
			ok = false
			continue
		}
		rec.Ident = &value.Identity{Coll: coll, Key: value.Key{S: key.Text}}
	}
	if !ok {
		return nil
	}
	return &value.Table{T: t, Entries: recs}
}

// cellDecoders dispatches a cell on its field type's kind (DECISIONS 26).
var cellDecoders [types.Error + 1]func(*run, Cell, types.Type, wscope) value.Value

func init() {
	cellDecoders[types.Bool], cellDecoders[types.Int] = (*run).cellBool, (*run).cellInt
	cellDecoders[types.Float], cellDecoders[types.Duration] = (*run).cellFloat, (*run).cellDuration
	cellDecoders[types.String], cellDecoders[types.Enum] = (*run).cellString, (*run).cellEnum
	cellDecoders[types.Ref], cellDecoders[types.LitUnion] = (*run).cellRef, (*run).cellUnion
	cellDecoders[types.Optional] = (*run).cellOptional
}

// cell reads a cell as text of its field's type; one that does not parse is E7108 (§6.6).
func (r *run) cell(c Cell, t types.Type, sc wscope) value.Value {
	k := t.Kind()
	if int(k) >= len(cellDecoders) || cellDecoders[k] == nil {
		r.misuse(ErrNoWireType, t)
		return nil
	}
	return cellDecoders[k](r, c, t, sc)
}

func (r *run) unparsed(c Cell, t types.Type) value.Value {
	r.report(diag.E7108.At(c.Span, c.Text, t), nil)
	return nil
}

func (r *run) cellOptional(c Cell, t types.Type, sc wscope) value.Value {
	return r.cell(c, t.Base().(*types.OptionalType).Elem, sc)
}

// cellBool is true or false; with @json(int), 0 or 1.
func (r *run) cellBool(c Cell, t types.Type, sc wscope) value.Value {
	yes, no := textTrue, textFalse
	if sc.asInt {
		yes, no = textOne, textZero
	}
	if c.Text != yes && c.Text != no {
		return r.unparsed(c, t)
	}
	return &value.Bool{V: c.Text == yes, P: cellAt(c).prov()}
}

// cellInt is a Canon integer literal with an optional leading `-`, in t's range.
func (r *run) cellInt(c Cell, t types.Type, _ wscope) value.Value {
	text, isInt, ok := canonNumber(c.Text)
	if !ok || !isInt {
		return r.unparsed(c, t)
	}
	i, err := strconv.ParseInt(text, decimalBase, float64Bits)
	if err != nil || !fits(i, t) {
		r.report(diag.E3201.At(c.Span, literal(c.Text), t), nil)
		return nil
	}
	return &value.Int{V: i, T: t, P: cellAt(c).prov()}
}

// cellFloat is a Canon integer or float literal, rounded once to t's width.
func (r *run) cellFloat(c Cell, t types.Type, _ wscope) value.Value {
	text, _, ok := canonNumber(c.Text)
	if !ok {
		return r.unparsed(c, t)
	}
	x, ok := r.floatOf(cellAt(c), text, t)
	if !ok {
		return nil
	}
	return &value.Float{V: x, T: t, P: cellAt(c).prov()}
}

// cellDuration is a Canon integer or float literal in the field's unit (WIRE.md §5.1).
func (r *run) cellDuration(c Cell, t types.Type, sc wscope) value.Value {
	text, _, ok := canonNumber(c.Text)
	if !ok {
		return r.unparsed(c, t)
	}
	ms, ok := r.millis(cellAt(c), c.Text, parseDecimal(text), sc.unit, t)
	if !ok {
		return nil
	}
	return &value.Dur{Ms: ms, P: cellAt(c).prov()}
}

func (r *run) cellString(c Cell, t types.Type, _ wscope) value.Value {
	return &value.Str{V: c.Text, T: t, P: cellAt(c).prov()}
}

// cellEnum is a member's wire value, or its code in decimal with @json(codes).
func (r *run) cellEnum(c Cell, t types.Type, _ wscope) value.Value {
	e := t.Base().(*types.EnumType)
	if !e.WireCodes {
		return r.memberByWire(cellAt(c), c.Text, e)
	}
	if !intKeyPattern.MatchString(c.Text) {
		return r.unparsed(c, t)
	}
	return r.memberByCode(cellAt(c), c.Text, e)
}

// cellRef is a key: a table's entry key, or a keyed list's key field, decimal for an integer.
func (r *run) cellRef(c Cell, t types.Type, _ wscope) value.Value {
	rt := t.Base().(*types.RefType)
	key := value.Key{S: c.Text}
	if rt.Target != nil && rt.Target.KeyedBy != nil {
		if rt.Target.KeyedBy.Type.Kind() == types.Int && !intKeyPattern.MatchString(c.Text) {
			return r.unparsed(c, t)
		}
		k, ok := keyOfValue(r.cell(c, rt.Target.KeyedBy.Type, wscope{}))
		if !ok {
			return nil
		}
		key = k
	}
	return &value.Ref{T: t, Key: key, Owner: r.owner(rt.Target), P: cellAt(c).prov()}
}

// cellUnion is one of the literals, which wins, else a cell of the union's first type.
func (r *run) cellUnion(c Cell, t types.Type, sc wscope) value.Value {
	u := t.Base().(*types.LitUnionType)
	if isLiteral(u, c.Text) {
		return &value.Str{V: c.Text, T: t, P: cellAt(c).prov()}
	}
	return r.cell(c, u.Of, sc)
}

// canonNumber is a Canon number literal with an optional `-`, as decimal text (GRAMMAR §2.4).
func canonNumber(s string) (string, bool, bool) {
	body, neg := strings.CutPrefix(s, minusSign)
	text, isInt, ok := unsignedNumber(body)
	if ok && neg {
		text = minusSign + text
	}
	return text, isInt, ok
}

func unsignedNumber(s string) (string, bool, bool) {
	for _, p := range radixPrefixes {
		if digits, ok := strings.CutPrefix(s, p.prefix); ok {
			n, ok := new(big.Int).SetString(strings.ReplaceAll(digits, underscore, ""), p.base)
			switch {
			case !ok || !validDigits(digits, p.base):
				return "", false, false
			case n.BitLen() > maxExactBits:
				return beyondRange, true, true
			}
			return n.String(), true, true
		}
	}
	mant, exp, hasExp := s, "", false
	if i := strings.IndexAny(s, exponentChars); i >= 0 {
		mant, exp, hasExp = s[:i], stripSign(s[i+1:]), true
	}
	whole, frac, hasPoint := strings.Cut(mant, pointSep)
	valid := validDigits(whole, decimalBase) && (len(whole) == 1 || whole[0] != digitZero) &&
		(!hasPoint || validDigits(frac, decimalBase)) && (!hasExp || validDigits(exp, decimalBase))
	if !valid {
		return "", false, false
	}
	return strings.ReplaceAll(s, underscore, ""), !hasPoint && !hasExp, true
}

// validDigits is a non-empty run of digits of base with `_` only between two digits.
func validDigits(s string, base int) bool {
	if s == "" || strings.HasPrefix(s, underscore) || strings.HasSuffix(s, underscore) ||
		strings.Contains(s, underscore+underscore) {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if c != underscore[0] && strings.IndexByte(radixLower[:base], c) < 0 && strings.IndexByte(radixUpper[:base], c) < 0 {
			return false
		}
	}
	return true
}

// stripSign is s without one leading `+` or `-`.
func stripSign(s string) string {
	if s != "" && strings.IndexByte(signChars, s[0]) >= 0 {
		return s[1:]
	}
	return s
}
