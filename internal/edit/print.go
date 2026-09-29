package edit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// canonPrinter prints a value as a Canon literal on one line, with contextual names (API.md
// M7): bare members, cases and keys, records without the fields left or equal to their
// defaults. The text is laid out canonically where it is spliced (API-03).
type canonPrinter struct {
	a   *applier
	b   strings.Builder
	err error
}

// canonText is v as a Canon literal (API.md M7).
func (a *applier) canonText(v value.Value) (string, error) {
	p := &canonPrinter{a: a}
	p.value(v)
	return p.b.String(), p.err
}

func (p *canonPrinter) value(v value.Value) {
	switch x := v.(type) {
	case *value.Str:
		p.b.WriteString(canonQuote(x.V))
	case *value.Ref:
		p.b.WriteString(refText(x.Key))
	case *value.Record:
		p.record(x)
	case *value.List:
		p.list(x.Elems)
	case *value.Map:
		p.mapValue(x)
	case *value.Table:
		p.table(x)
	case *value.Bool, *value.Int, *value.Float, *value.Dur, *value.Member, *value.CaseKind, *value.Symbol, *value.None:
		p.b.WriteString(v.CanonText())
	default:
		p.err = fmt.Errorf(fmtUnprinted, errUnprinted, v)
	}
}

// refText is a ref's key as a contextual name: digits, a bare word, else a string (SPEC §6.2).
func refText(k value.Key) string {
	switch {
	case k.IsInt:
		return strconv.FormatInt(k.I, decimalBase)
	case isWord(k.S):
		return k.S
	}
	return canonQuote(k.S)
}

// record is `{ f: v, … }`, a case `name { … }` or its bare name when it writes no field.
func (p *canonPrinter) record(r *value.Record) {
	c, isCase := r.T.Base().(*types.CaseType)
	shown := p.shown(r)
	if isCase {
		p.b.WriteString(c.Name)
		if len(shown) == 0 {
			return
		}
		p.b.WriteString(space)
	}
	p.body(r, fieldsOf(r.T), shown)
}

// body is a record's brace: its shown fields, or `{}`.
func (p *canonPrinter) body(r *value.Record, fields []*types.Field, shown []int) {
	if len(shown) == 0 {
		p.b.WriteString(emptyBrace)
		return
	}
	p.b.WriteString(braceOpenSp)
	for k, i := range shown {
		if k > 0 {
			p.b.WriteString(listSep)
		}
		p.b.WriteString(fields[i].Name + colonSp)
		p.value(r.Fields[i])
	}
	p.b.WriteString(braceCloseSp)
}

func (p *canonPrinter) list(elems []value.Value) {
	p.b.WriteString(string(bracketOpen))
	for i, e := range elems {
		if i > 0 {
			p.b.WriteString(listSep)
		}
		p.value(e)
	}
	p.b.WriteString(bracketClose)
}

// mapValue is `{ k: v, … }`: a String key is a string, a member, case or ref key a bare name.
func (p *canonPrinter) mapValue(m *value.Map) {
	if len(m.Keys) == 0 {
		p.b.WriteString(emptyBrace)
		return
	}
	p.b.WriteString(braceOpenSp)
	for i, k := range m.Keys {
		if i > 0 {
			p.b.WriteString(listSep)
		}
		p.value(k)
		p.b.WriteString(colonSp)
		p.value(m.Vals[i])
	}
	p.b.WriteString(braceCloseSp)
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
	p.body(e, fieldsOf(e.T), p.shown(e))
}

// shown are the fields of r a literal writes: written, and not equal to their defaults (M7).
func (p *canonPrinter) shown(r *value.Record) []int {
	var out []int
	for i := range fieldsOf(r.T) {
		if p.a.printed(r, i) {
			out = append(out, i)
		}
	}
	return out
}

// entryText is a table entry with its key as a new item of a table literal.
func (a *applier) entryText(key string, rec *value.Record, retired bool) (string, error) {
	p := &canonPrinter{a: a}
	p.entry(key, retired, rec)
	return p.b.String(), p.err
}
