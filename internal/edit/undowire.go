package edit

import (
	"encoding/json"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// readWire is v's wire in scope's rules (nil for none), re-encoded with each symbol as the token
// it was read from, when a JSON source states v and v holds a symbol the decoder read there
// (DECISIONS 175); false otherwise.
func (a *applier) readWire(v value.Value, scope *types.Field) (json.RawMessage, bool, error) {
	p := provOf(v)
	if p == nil || p.Kind != value.ProvJSON || !holdsDecoded(v) {
		return nil, false, nil
	}
	raw, err := a.wireText(v, scope)
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

// holdsDecoded reports v holding a symbol read from a JSON source (a symbol with provenance).
func holdsDecoded(v value.Value) bool {
	switch x := v.(type) {
	case *value.Symbol:
		return x.P != nil
	case *value.Record:
		return anyDecoded(x.Fields)
	case *value.List:
		return anyDecoded(x.Elems)
	case *value.Map:
		return anyDecoded(x.Keys) || anyDecoded(x.Vals)
	case *value.Table:
		for _, e := range x.Entries {
			if holdsDecoded(e) {
				return true
			}
		}
	}
	return false
}

func anyDecoded(vs []value.Value) bool {
	for _, v := range vs {
		if v != nil && holdsDecoded(v) {
			return true
		}
	}
	return false
}
