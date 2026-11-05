package edit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// canonPrinter prints a value as a Canon literal on one line, with contextual names (API.md
// M7): members, cases and keys bare or quoted, records without the fields left or equal to their
// defaults. The text is laid out canonically where it is spliced (API-03).
type canonPrinter struct {
	a      *applier
	b      strings.Builder
	err    error
	notes  spellings   // the old value's tokens as written, for an Undo (DECISIONS 337)
	fr     depFrame    // the record the value printed lies in, its parameters bound (TYPES.md 11.1)
	at     types.Type  // the declared type of the position printed, nil when unknown (typedName)
	names  *typeNaming // the file a typed literal's name is printed in; nil for a Source
	unread types.Type  // the first dependent position whose text reads another branch (unreadAt)
}

// canonTextAt is v as a Canon literal (API.md M7) at a position of declared type at, in the file
// the operation writes: the same text in every file but for a typed literal's name (typedName).
func (a *applier) canonTextAt(v value.Value, at types.Type) (string, error) {
	p := &canonPrinter{a: a, at: at, names: a.names}
	p.value(v)
	return p.b.String(), p.err
}

// mapItemText is `key: value` as a new item of a map literal whose values are declared vt.
func (a *applier) mapItemText(k, v value.Value, vt types.Type) (string, error) {
	p := &canonPrinter{a: a, names: a.names}
	p.key(k)
	p.at = vt
	p.b.WriteString(colonSp)
	p.value(v)
	return p.b.String(), p.err
}

// spelling is the token v is printed as, when not canonical: as the restored source or an
// operation's Source wrote it.
func (p *canonPrinter) spelling(v value.Value) (string, bool) {
	if text, ok := p.notes[v]; ok {
		return text, true
	}
	text, ok := p.a.spelled[v]
	return text, ok
}

func (p *canonPrinter) value(v value.Value) {
	if text, ok := p.spelling(v); ok {
		p.b.WriteString(text)
		return
	}
	switch x := v.(type) {
	case *value.Str:
		p.b.WriteString(canonQuote(x.V))
	case *value.Ref:
		p.b.WriteString(p.scope().keyText(x.Key, x.T, refText(x.Key)))
	case *value.Member, *value.Symbol, *value.CaseKind:
		p.b.WriteString(p.nameText(x))
	case *value.Record:
		p.record(x)
	case *value.List:
		p.list(x)
	case *value.Map:
		p.mapValue(x)
	case *value.Table:
		p.table(x)
	case *value.Bool, *value.Int, *value.Float, *value.Dur, *value.None:
		p.b.WriteString(v.CanonText())
	default:
		p.err = fmt.Errorf(fmtUnprinted, errUnprinted, v)
	}
}

// refText is a ref's key in value position: digits, a word bare or quoted, else a string (API.md M7).
func refText(k value.Key) string {
	if k.IsInt {
		return strconv.FormatInt(k.I, decimalBase)
	}
	if text, ok := syntax.ValueText(k.S); ok {
		return text
	}
	return canonQuote(k.S)
}

// key is a map item's key: a member, case kind, symbol or ref's word key bare, the WORD before `:` (M7).
func (p *canonPrinter) key(k value.Value) {
	if text, ok := p.spelling(k); ok {
		p.b.WriteString(text)
		return
	}
	switch x := k.(type) {
	case *value.Member, *value.CaseKind:
		p.b.WriteString(x.CanonText())
		return
	case *value.Ref:
		if !x.Key.IsInt && isWord(x.Key.S) {
			p.b.WriteString(p.scope().keyText(x.Key, x.T, x.Key.S))
			return
		}
	case *value.Symbol:
		if isWord(x.Name) {
			p.b.WriteString(x.Name)
			return
		}
	}
	p.value(k)
}

// record is `{ f: v, … }`, a case `name { … }` or its bare name when it writes no field, a
// typed record literal `T { … }` where a bare brace reads another branch (typedName).
func (p *canonPrinter) record(r *value.Record) {
	c, isCase := r.T.Base().(*types.CaseType)
	f := p.in(r)
	shown := p.shown(f)
	if isCase {
		p.b.WriteString(caseText(c))
		if len(shown) == 0 {
			return
		}
		if p.atDependent() {
			p.unreadAt() // a case is printed bare (DECISIONS 378), never read there (475)
		}
		p.b.WriteString(space)
	} else if name, typed := p.typedName(r); typed {
		p.b.WriteString(name + space)
	}
	p.body(f, fieldsOf(r.T), shown)
}

// in is the frame of r's fields, r printed inside p's frame: its parameters bound (depFrame.enter).
func (p *canonPrinter) in(r *value.Record) depFrame {
	fr := p.fr
	fr.a = p.a
	return fr.enter(r, nil)
}

// body is the brace of fr's record: its shown fields, each printed in fr, or `{}`.
func (p *canonPrinter) body(fr depFrame, fields []*types.Field, shown []int) {
	if len(shown) == 0 {
		p.b.WriteString(emptyBrace)
		return
	}
	saved, at := p.fr, p.at
	p.fr = fr
	p.b.WriteString(braceOpenSp)
	for k, i := range shown {
		if k > 0 {
			p.b.WriteString(listSep)
		}
		p.b.WriteString(fields[i].Name + colonSp)
		p.at = fields[i].Type
		p.value(fr.rec.Fields[i])
	}
	p.b.WriteString(braceCloseSp)
	p.fr, p.at = saved, at
}

// list is `[e, …]`, each element at the declared element type of the position (or of l).
func (p *canonPrinter) list(l *value.List) {
	p.noteCollection(l, len(l.Elems))
	at := p.at
	p.at = listItemType(declaredOr(at, l))
	p.b.WriteString(string(bracketOpen))
	for i, e := range l.Elems {
		if i > 0 {
			p.b.WriteString(listSep)
		}
		p.value(e)
	}
	p.b.WriteString(bracketClose)
	p.at = at
}

// mapValue is `{ k: v, … }`: a String key is a string, a member, case or ref key a bare name.
func (p *canonPrinter) mapValue(m *value.Map) {
	if len(m.Keys) == 0 {
		p.b.WriteString(emptyBrace)
		return
	}
	p.noteCollection(m, len(m.Keys))
	at := p.at
	vt := mapValueType(declaredOr(at, m))
	p.b.WriteString(braceOpenSp)
	for i, k := range m.Keys {
		if i > 0 {
			p.b.WriteString(listSep)
		}
		p.key(k)
		p.b.WriteString(colonSp)
		p.at = vt
		p.value(m.Vals[i])
	}
	p.b.WriteString(braceCloseSp)
	p.at = at
}

// declaredOr is at, the declared type of a position, else v's own type where at is unknown.
func declaredOr(at types.Type, v value.Value) types.Type {
	if at == nil {
		return v.Type()
	}
	return at
}

// listItemType is the declared element type of list type t, nil for another type.
func listItemType(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	return elemType(present(t))
}

// mapValueType is the declared value type of map type t, nil for another type.
func mapValueType(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	_, vt, _ := mapTypes(present(t))
	return vt
}

// table is `{ key { … }, retired key { … } }`.
func (p *canonPrinter) table(t *value.Table) {
	if len(t.Entries) == 0 {
		p.b.WriteString(emptyBrace)
		return
	}
	p.b.WriteString(braceOpenSp)
	for i, e := range t.Entries {
		if i > 0 {
			p.b.WriteString(listSep)
		}
		p.entry(e.Ident.Key.Text(), e.Ident.Retired, e)
	}
	p.b.WriteString(braceCloseSp)
}

// entry is a table entry: `[retired ]key { … }`.
func (p *canonPrinter) entry(key string, retired bool, e *value.Record) {
	if retired {
		p.b.WriteString(retiredWord + space)
	}
	p.b.WriteString(key + space)
	f := p.in(e)
	p.body(f, fieldsOf(e.T), p.shown(f))
}

// shown are the fields of fr's record a literal writes: written, and not equal to their defaults (M7).
func (p *canonPrinter) shown(fr depFrame) []int {
	var out []int
	for i := range fieldsOf(fr.rec.T) {
		if p.a.printed(fr, i) {
			out = append(out, i)
		}
	}
	return out
}

// entryText is a table entry with its key as a new item of a table literal.
func (a *applier) entryText(key string, rec *value.Record, retired bool) (string, error) {
	p := &canonPrinter{a: a, names: a.names}
	p.entry(key, retired, rec)
	return p.b.String(), p.err
}
