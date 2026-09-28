package table

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// columns are a table's columns (VIEWMODEL.md 7.2): the entry column when the collection is
// keyed or the element view has a title (T6), then the declared columns, a column naming the key
// field giving its width to the entry column, else auto (T7).
func (t *Tables) columns(named namer, auto []vm.Column, key *types.Field, keyed bool, v control.View) []vm.Column {
	entry, width := keyed || v.Item(syntax.KindViewTitle) != nil, 0
	var cols []vm.Column
	declared, ok := v.Item(syntax.KindViewColumns).(*syntax.ViewColumns)
	if !ok {
		return append(t.entry(entry, 0), auto...)
	}
	for _, c := range declared.Items {
		if c.Name == nil {
			continue
		}
		w := widthOf(c.Width)
		if key != nil && c.Name.Name == key.Name {
			width = w
			continue
		}
		for _, k := range named(c.Name.Name) {
			cols = append(cols, t.column(k, w))
		}
	}
	return append(t.entry(entry, width), cols...)
}

// entry is the entry column when there is one (T6).
func (t *Tables) entry(entry bool, width int) []vm.Column {
	if !entry {
		return []vm.Column{}
	}
	return []vm.Column{{Field: entryColumn, Width: width, Mode: modeEntry}}
}

// auto are the first 6 scalar fields in declaration order, but the key field, hidden fields and
// deprecated fields (T7).
func (t *Tables) auto(keys []encode.Keyed, key *types.Field) []vm.Column {
	var cols []vm.Column
	for _, k := range keys {
		f := k.Field
		if len(cols) == autoColumns {
			break
		}
		if k.Case != "" || f == key || f.Deprecated != nil || !scalar(f.Type) || t.index.Field(f).True(syntax.PropHidden) {
			continue
		}
		cols = append(cols, t.column(k, 0))
	}
	return cols
}

// widthOf is a declared width in pixels, 0 (none) when absent or out of range (E1613).
func widthOf(w *syntax.IntLit) int {
	if w == nil || w.Value == nil || !w.Value.IsInt64() {
		return 0
	}
	if n := w.Value.Int64(); n >= minWidth && n <= maxWidth {
		return int(n)
	}
	return 0
}

// column is the column of a field with its mode (T8), and its cell control in `edit` mode (C46).
func (t *Tables) column(k encode.Keyed, width int) vm.Column {
	reason, _ := t.res.ReadOnly(k.Field)
	col := vm.Column{Field: k.Key, Width: width, Mode: mode(k.Field.Type, reason)}
	if col.Mode == modeEdit {
		cell := control.Cell(t.res.Field(k.Decl, k.Field))
		col.Cell = &cell
	}
	return col
}

// mode is a field's column mode by its type, the optional stripped (T8): `count` for a
// collection, `case` for a variant, `edit` for a scalar that is not read-only, `value` for one
// that is, `text` for a record, a dependent value or a literal union.
func mode(ft types.Type, readonly string) string {
	switch t := shape.StripOptional(ft); {
	case shape.KindIn(t, types.List, types.Map, types.Table, types.DepMap):
		return modeCount
	case shape.KindIn(t, types.Variant): // a fixed case `V.c` is a record: text (T8)
		return modeCase
	case !scalar(t) || shape.KindIn(t, types.LitUnion):
		return modeText
	case readonly != "":
		return modeValue
	}
	return modeEdit
}

// scalar is a scalar of VIEWMODEL.md 4.3, the optional stripped.
func scalar(t types.Type) bool { return control.Scalar(shape.StripOptional(t)) }
