package live

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
)

// element is an element of a collection: its value, path, magic names and its title without a
// view (API.md V7): its key's canonical text, or `#<n>` in a plain list (VIEWMODEL.md S9).
type element struct {
	v     value.Value
	path  *verify.Path
	magic render.Magic
	name  string
}

// isCollection reports a list, table or map value.
func isCollection(v value.Value) bool {
	switch v.(type) {
	case *value.List, *value.Table, *value.Map:
		return true
	}
	return false
}

// collection heads each element of the collection v at at (API.md V6), with the cells of the
// `text` columns of its table control ctl (V6a), equal titles disambiguated (S9).
func (s *session) collection(v value.Value, at *verify.Path, ctl vm.Control) {
	if s.stopped() {
		return
	}
	els := elements(v, at)
	cols := textColumns(ctl)
	out := make([]Heading, len(els))
	seen := map[string]int{}
	sources := make([]string, len(els))
	for i, e := range els {
		out[i], sources[i] = s.heading(e, cols)
		if sources[i] != "" {
			seen[sources[i]]++
		}
	}
	for i, e := range els {
		if out[i].Title.OK && seen[sources[i]] >= sharedTitle {
			out[i].Title.Value = render.Disambiguated(out[i].Title.Value, e.name)
		}
		s.out.Headings[rel(e.path)] = out[i]
	}
}

// heading is how e is listed, and its view title in the source language, "" when it has none.
func (s *session) heading(e element, cols []string) (Heading, string) {
	rec, ok := e.v.(*value.Record)
	if !ok {
		return Heading{Title: Text{Value: e.name, OK: true}, Subtitle: Text{OK: true}, Cells: map[string]Text{}}, ""
	}
	h := s.heads(rec, e.magic, e.name)
	out := Heading{Title: h.title, Subtitle: h.subtitle, Preview: h.preview, Cells: map[string]Text{}}
	out.Retired = rec.Ident != nil && rec.Ident.Retired
	for _, key := range cols {
		if v, ok := cellValue(rec, key); ok {
			out.Cells[key] = s.cell(v)
		}
	}
	if !h.title.OK {
		return out, ""
	}
	return out, s.sourceTitle(rec, e.magic)
}

// elements are the elements of a list, table or map in collection order (API.md P8).
func elements(v value.Value, at *verify.Path) []element {
	var out []element
	switch x := v.(type) {
	case *value.Table:
		for _, e := range x.Entries {
			if e.Ident != nil {
				out = append(out, element{v: e, path: at.Entry(e.Ident.Key), name: e.Ident.Key.Text()})
			}
		}
	case *value.List:
		for i, el := range x.Elems {
			out = append(out, listElement(el, i, at))
		}
	case *value.Map:
		for i, k := range x.Keys {
			out = append(out, element{v: x.Vals[i], path: at.Key(verify.KeyOf(k)), magic: render.Magic{Key: k}, name: k.CanonText()})
		}
	}
	return out
}

// listElement is the element i of a list: a keyed list's by its key, a plain list's by its
// index, `index` counting from 1 (VIEWMODEL.md 3.4).
func listElement(el value.Value, i int, at *verify.Path) element {
	pos := i + 1
	e := element{v: el, path: at.Index(i), magic: render.Magic{Index: &value.Int{V: int64(pos), T: types.IntType}}, name: positionTag + strconv.Itoa(pos)}
	if r, ok := el.(*value.Record); ok && r.Ident != nil {
		e.path, e.name = at.Key(r.Ident.Key), r.Ident.Key.Text()
	}
	return e
}

// textColumns are the field keys of the `text` columns of a table control (VIEWMODEL.md T8),
// or of a widget's fallback table; none for another control.
func textColumns(ctl vm.Control) []string {
	if ctl.Kind != control.CtlTable && ctl.Fallback != nil {
		ctl = *ctl.Fallback
	}
	if ctl.Kind != control.CtlTable {
		return nil
	}
	var out []string
	for _, c := range ctl.Columns {
		if c.Mode == columnText {
			out = append(out, c.Field)
		}
	}
	return out
}

// cellValue is the value of rec a column's field key names: `f`, `v.c.f` for a case field of
// an inline variant, `c.f` in a table of variants (T6a); false when rec's shape lacks it (T9).
func cellValue(rec *value.Record, key string) (value.Value, bool) {
	t := rec.T
	if c, ok := rec.T.Base().(*types.CaseType); ok {
		name, rest, _ := strings.Cut(key, dot)
		if name != c.Name {
			return nil, false
		}
		t, key = c, rest
	}
	for _, k := range encode.Keys(t) {
		if k.Key == key {
			return keyedValue(rec, k)
		}
	}
	return nil, false
}

// keyedValue is k's field of rec when rec's current cases are k's case path (L18).
func keyedValue(rec *value.Record, k encode.Keyed) (value.Value, bool) {
	cur := rec
	for seg := range strings.SplitSeq(k.Case, caseSep) {
		field, caseName, ok := strings.Cut(seg, caseIs)
		if !ok {
			continue
		}
		c, isCase := fieldOf(cur, field).(*value.Record)
		if !isCase || caseOf(c) != caseName {
			return nil, false
		}
		cur = c
	}
	v := fieldOf(cur, k.Field.Name)
	return v, v != nil
}

// caseOf is the case name of a case value, "" for another record.
func caseOf(rec *value.Record) string {
	if c, ok := rec.T.Base().(*types.CaseType); ok {
		return c.Name
	}
	return ""
}

// cell is a `text` cell (API.md V6a): a record's title, a ref's target title (S8), else the
// value's canonical text.
func (s *session) cell(v value.Value) Text {
	switch x := v.(type) {
	case *value.Record:
		return s.headText(s.shown, x, titleItem, x.CanonText())
	case *value.Ref:
		return s.refTitle(x)
	}
	return Text{Value: v.CanonText(), OK: true}
}

// refTitle is a ref's target title, refs in it rendered as keys; its key when the target has no
// view title, is not in this build, or its title fails (VIEWMODEL.md S8).
func (s *session) refTitle(r *value.Ref) Text {
	key := Text{Value: r.Key.Text(), OK: true}
	rt, ok := r.T.Base().(*types.RefType)
	if !ok {
		return key
	}
	e := s.entry(rt.Target, r.Key)
	if e == nil {
		return key
	}
	if t := s.headText(s.target, e, titleItem, key.Value); t.OK {
		return t
	}
	return key
}

// entry is the entry of coll keyed k in this build, nil for none (a field of an enclosing record).
func (s *session) entry(coll *types.Collection, k value.Key) *value.Record {
	byKey, ok := s.keys[coll]
	if !ok {
		byKey = map[value.Key]*value.Record{}
		for _, e := range s.colls.Entries(coll) {
			if e.Ident != nil {
				byKey[e.Ident.Key] = e
			}
		}
		s.keys[coll] = byKey
	}
	return byKey[k]
}
