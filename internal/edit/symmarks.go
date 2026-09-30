package edit

import (
	"encoding/json"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// symMarks is what the typer knew of the symbols it made: the JSON token a FromJSON value wrote
// each decoded one with (DECISIONS 175), and the keys given as path keys (API.md E25).
type symMarks struct {
	tokens   map[*value.Symbol]json.RawMessage
	pathKeys map[*value.Symbol]bool
}

func newSymMarks() *symMarks {
	return &symMarks{tokens: map[*value.Symbol]json.RawMessage{}, pathKeys: map[*value.Symbol]bool{}}
}

// noteTokens notes the token of every symbol of v the decoder read from f.
func (m *symMarks) noteTokens(v value.Value, f *source.File) {
	if m == nil {
		return
	}
	switch x := v.(type) {
	case *value.Symbol:
		if x.P != nil && x.P.Span.File == f.ID && int(x.P.Span.End) <= len(f.Content) {
			m.tokens[x] = f.Content[x.P.Span.Start:x.P.Span.End]
		}
	case *value.Record:
		m.noteAll(x.Fields, f)
	case *value.List:
		m.noteAll(x.Elems, f)
	case *value.Map:
		m.noteAll(x.Keys, f)
		m.noteAll(x.Vals, f)
	case *value.Table:
		for _, e := range x.Entries {
			m.noteTokens(e, f)
		}
	}
}

func (m *symMarks) noteAll(vs []value.Value, f *source.File) {
	for _, v := range vs {
		m.noteTokens(v, f)
	}
}

// pathKey reports s given as a path key: matched as a Canon name, then as a wire value (E25).
func (m *symMarks) pathKey(s *value.Symbol) bool {
	return m != nil && m.pathKeys[s]
}

// dependentKey is a name given as a key of a dependent key type, a symbol (TYPES.md 11.5).
func (tc *typing) dependentKey(lit Lit, kt types.Type) (value.Value, bool) {
	if !dependent(kt) {
		return nil, false
	}
	switch k := lit.(type) {
	case Key:
		return &value.Symbol{Name: string(k), T: kt}, true
	case PathKey:
		s := &value.Symbol{Name: string(k), T: kt}
		if tc.ty.marks != nil {
			tc.ty.marks.pathKeys[s] = true
		}
		return s, true
	}
	return nil, false
}
