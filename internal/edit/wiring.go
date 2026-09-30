package edit

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// wiring is one value's source wire being built: the JSON token of each symbol a source wrote,
// and the placeholder the encoder writes in its place (log-2026-09-29 M4 B7-r3, B7-r4).
type wiring struct {
	a    *applier
	raws []*jsonsrc.Node
	phs  []*value.Str
}

// restrict is a copy of v whose records keep only the fields a literal writes (M7), for the data
// wire's encoder; a symbol left is its JSON token as read, else the string of its name.
func (w *wiring) restrict(v value.Value) (value.Value, error) {
	switch x := v.(type) {
	case *value.Record:
		return w.restrictRecord(x)
	case *value.List:
		elems, err := w.restrictAll(x.Elems)
		return &value.List{T: x.T, Elems: elems, P: x.P}, err
	case *value.Map:
		vals, err := w.restrictAll(x.Vals)
		return &value.Map{T: x.T, Keys: keyTexts(x.Keys), Vals: vals, P: x.P}, err
	case *value.Table:
		out := &value.Table{T: x.T, P: x.P}
		for _, e := range x.Entries {
			r, err := w.restrictRecord(e)
			if err != nil {
				return nil, err
			}
			out.Entries = append(out.Entries, r)
		}
		return out, nil
	case *value.Symbol:
		return w.token(x)
	}
	return v, nil
}

func (w *wiring) restrictAll(vs []value.Value) ([]value.Value, error) {
	out := make([]value.Value, len(vs))
	for i, v := range vs {
		r, err := w.restrict(v)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}

// restrictRecord is r with only its printed fields, under a copy of its type holding those.
func (w *wiring) restrictRecord(r *value.Record) (*value.Record, error) {
	fields := fieldsOf(r.T)
	var keep []*types.Field
	var vals []value.Value
	for i, f := range fields {
		if !w.a.printed(r, i) {
			continue
		}
		if l, ok := r.Fields[i].(*value.List); ok && f.Pairs != nil {
			keep, vals = append(keep, f), append(vals, w.a.pairsWritten(l)) // both keys of each slot (WIRE.md 5.14)
			continue
		}
		v, err := w.restrict(r.Fields[i])
		if err != nil {
			return nil, err
		}
		keep, vals = append(keep, f), append(vals, v)
	}
	set := make([]bool, len(keep))
	for i := range set {
		set[i] = true
	}
	return &value.Record{T: restrictedType(r.T, keep), Fields: vals, Set: set, Ident: r.Ident, P: r.P}, nil
}

// keyTexts are map keys for the encoder: a symbol key as its text, the member key it was read from.
func keyTexts(keys []value.Value) []value.Value {
	out := make([]value.Value, len(keys))
	for i, k := range keys {
		if s, ok := k.(*value.Symbol); ok {
			k = &value.Str{V: s.Name, T: types.StringType, P: s.P}
		}
		out[i] = k
	}
	return out
}

// token is s for the encoder: a placeholder for the JSON token a source wrote it with (DECISIONS
// 175), the string of its name when no source wrote it; errNoWire when its token is not known.
func (w *wiring) token(s *value.Symbol) (value.Value, error) {
	if s.P == nil {
		return &value.Str{V: s.Name, T: types.StringType}, nil
	}
	n, err := w.a.rawToken(s)
	if err != nil {
		return nil, err
	}
	ph := &value.Str{V: symPlaceholder, T: types.StringType}
	w.raws, w.phs = append(w.raws, n), append(w.phs, ph)
	return ph, nil
}

// encode is rv, restricted, in the source wire at f's place, each placeholder replaced by its
// token. The placeholders are found by identity: rv is encoded a second time with each of them
// numbered, and only the strings that differ between the two encodings are theirs.
func (w *wiring) encode(rv value.Value, f *types.Field) (*jsonsrc.Node, error) {
	n, err := encodeWire(rv, f)
	if err != nil || len(w.phs) == 0 {
		return n, err
	}
	for i, ph := range w.phs {
		ph.V = symPlaceholder + strconv.Itoa(i)
	}
	numbered, err := encodeWire(rv, f)
	if err != nil {
		return nil, err
	}
	return w.substitute(n, numbered), nil
}

// substitute is n with each placeholder, a string numbered differently in numbered, replaced
// by the token it stands for.
func (w *wiring) substitute(n, numbered *jsonsrc.Node) *jsonsrc.Node {
	if n == nil || numbered == nil {
		return n
	}
	if n.Kind == jsonsrc.String && n.Text != numbered.Text {
		i, err := strconv.Atoi(strings.TrimPrefix(numbered.Text, symPlaceholder))
		if err == nil && i >= 0 && i < len(w.raws) {
			return detached(w.raws[i])
		}
	}
	for i := range min(len(n.Elems), len(numbered.Elems)) {
		n.Elems[i] = w.substitute(n.Elems[i], numbered.Elems[i])
	}
	for i := range min(len(n.Members), len(numbered.Members)) {
		n.Members[i].Value = w.substitute(n.Members[i].Value, numbered.Members[i].Value)
	}
	return n
}

// rawToken is the JSON token s was read from: noted by the typer for a FromJSON value, else in
// the snapshot's file its provenance names.
func (a *applier) rawToken(s *value.Symbol) (*jsonsrc.Node, error) {
	raw, ok := a.marks.tokens[s]
	if !ok {
		content := a.snap.a.Files().Content(s.P.Span.File)
		if int(s.P.Span.End) > len(content) || s.P.Span.Start > s.P.Span.End {
			return nil, errNoWire
		}
		raw = content[s.P.Span.Start:s.P.Span.End]
	}
	var fs source.FileSet
	src, err := fs.Add(wireSlot, wireSlot, raw)
	if err != nil {
		return nil, err
	}
	root, err := jsonsrc.Parse(src, diag.NewBag(&fs, ""))
	if err != nil || root == nil {
		return nil, errNoWire
	}
	return detached(root), nil
}
