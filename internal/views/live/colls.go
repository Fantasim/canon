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
	"github.com/fantasim/canonlang/internal/views/table"
)

// element is an element of a collection, named, and its path.
type element struct {
	Element
	path *verify.Path
}

// Element is a collection's element as the studio names it (API.md P8, V7; VIEWMODEL.md S9):
// Name is its key's text in a table, keyed list or map, else `#<n>` from 1, even for a copy of a
// table entry in a plain list; Magic holds its `index` in a list, its `key` in a map.
type Element struct {
	v      value.Value
	Name   string
	Magic  render.Magic
	form   elemForm
	index  int
	key    value.Key   // an entry's or a keyed element's
	mapKey value.Value // a map entry's
}

// elemForm is how an element's path segment is written (API.md P8).
type elemForm uint8

// ElementAt is element i of the collection c, in collection order; false when c is no list,
// table or map, i is out of range, or the entry has no key.
func ElementAt(c value.Value, i int) (Element, bool) {
	if i < 0 || i >= collLen(c) {
		return Element{}, false
	}
	switch x := c.(type) {
	case *value.Table:
		e := x.Entries[i]
		if e.Ident == nil {
			return Element{}, false
		}
		return Element{v: e, Name: e.Ident.Key.Text(), form: formEntry, key: e.Ident.Key}, true
	case *value.Map:
		k := x.Keys[i]
		return Element{v: x.Vals[i], Name: k.CanonText(), Magic: render.Magic{Key: k}, form: formMap, mapKey: k}, true
	case *value.List:
		return listElement(x, i), true
	}
	return Element{}, false
}

// collLen is the number of elements of a list, table or map; 0 for another value.
func collLen(c value.Value) int {
	switch x := c.(type) {
	case *value.Table:
		return len(x.Entries)
	case *value.List:
		return len(x.Elems)
	case *value.Map:
		return len(x.Keys)
	}
	return 0
}

// path is e's path in its collection's, at (API.md P8).
func (e Element) path(at *verify.Path) *verify.Path {
	return elemPaths[e.form](e, at)
}

func plainPath(e Element, at *verify.Path) *verify.Path { return at.Index(e.index) }

func entryPath(e Element, at *verify.Path) *verify.Path { return at.Entry(e.key) }

func keyedPath(e Element, at *verify.Path) *verify.Path { return at.Key(e.key) }

func mapPath(e Element, at *verify.Path) *verify.Path { return at.Key(verify.KeyOf(e.mapKey)) }

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
		if out[i].Title.OK && seen[sources[i]] >= render.SharedTitle {
			out[i].Title.Value = render.Disambiguated(out[i].Title.Value, e.Name)
		}
		s.out.Headings[rel(e.path)] = out[i]
	}
}

// heading is how e is listed, and its view title in the source language, "" when it has none.
func (s *session) heading(e element, cols []string) (Heading, string) {
	rec, ok := e.v.(*value.Record)
	if !ok {
		return Heading{Title: Text{Value: e.Name, OK: true}, Subtitle: Text{OK: true}, Cells: map[string]Text{}}, ""
	}
	h := s.heads(rec, e.Magic, e.Name)
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
	return out, s.sourceTitle(rec, e.Magic)
}

// elements are the elements of a list, table or map in collection order (API.md P8).
func elements(v value.Value, at *verify.Path) []element {
	var out []element
	for i := range collLen(v) {
		if e, ok := ElementAt(v, i); ok {
			out = append(out, element{Element: e, path: e.path(at)})
		}
	}
	return out
}

// listElement is the element i of a list: a keyed list's by its key, a plain list's by its
// index, `index` counting from 1 (VIEWMODEL.md 3.4); only a keyed list names by key (P8).
func listElement(l *value.List, i int) Element {
	pos := i + 1
	e := Element{v: l.Elems[i], Name: positionTag + strconv.Itoa(pos), index: i, form: formPlain}
	e.Magic = render.Magic{Index: &value.Int{V: int64(pos), T: types.IntType}}
	if r, ok := e.v.(*value.Record); ok && r.Ident != nil && keyedList(l) {
		e.Name, e.key, e.form = r.Ident.Key.Text(), r.Ident.Key, formKeyed
	}
	return e
}

// keyedList reports a list whose type is keyed by a field (API.md P1).
func keyedList(l *value.List) bool {
	if l.T == nil {
		return false
	}
	lt, ok := l.T.Base().(*types.ListType)
	return ok && lt.KeyedBy != nil
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
		if c.Mode == table.ModeText {
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
		return s.headText(s.shown, x, titleItem, x.CanonText(), true)
	case *value.Ref:
		return s.refTitle(x)
	}
	return Text{Value: v.CanonText(), OK: true}
}

// refTitle is a ref's target title in the target's own place, refs in it rendered as keys; its
// key when the target has no view title, is not in this build, or its title fails (VIEWMODEL.md
// S8; log-2026-09-29 M4 U9b-r).
func (s *session) refTitle(r *value.Ref) Text {
	key := Text{Value: r.Key.Text(), OK: true}
	rt, ok := r.T.Base().(*types.RefType)
	if !ok {
		return key
	}
	e, at := s.shown.Target(rt.Target, r.Key)
	if e == nil {
		return key
	}
	defer s.asTarget(e, at)()
	if t := s.headText(s.target.At(e, at), e, titleItem, key.Value, false); t.OK {
		return t
	}
	return key
}
