package load

import (
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// partElems is wire.Elems over the memo, keyed by the parsed tree Parse gives while a file is unchanged (NFR-02).
type partElems struct {
	parts *eval.Parts
}

// dirElems is the kept elements of a load.dir of type t when the evaluator decoding it records
// the load, nil otherwise.
func dirElems(req Request, t types.Type) wire.Elems {
	h, ok := req.Host.(evalHost)
	if !ok {
		return nil
	}
	var elem types.Type
	switch x := t.Base().(type) {
	case *types.ListType:
		elem = x.Elem
	case *types.TableType:
		elem = x.Elem
	default:
		return nil
	}
	if parts := h.ev.LoadParts(elem); parts != nil {
		return partElems{parts: parts}
	}
	return nil
}

func (p partElems) Kept(sel wire.Selection) (*value.Record, bool, bool) {
	return p.parts.Kept(sel.Node)
}

func (p partElems) Start(sel wire.Selection) func(rec *value.Record, retired, pure bool) {
	done := p.parts.Start()
	return func(rec *value.Record, retired, pure bool) {
		done(sel.Node, rec, retired, pure)
	}
}
