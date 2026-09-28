package canon

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/wire"
)

// origin is where r's target comes from, then each value an amendment replaced (EVALUATION.md §13, §9.3).
func (s *snapshot) origin(r edit.Resolved) Origin {
	head := s.originOf(r.Target)
	at := &head
	for _, old := range s.history(r) {
		if old == r.Target {
			continue
		}
		o := s.originOf(old)
		at.Replaced = &o
		at = at.Replaced
	}
	return head
}

// history is the analysis's record of the values amendments replaced at r (build.Analysis.History):
// it walks r from its root value, which an enum member path (API.md P7a) does not have.
func (s *snapshot) history(r edit.Resolved) []value.Value {
	p, err := edit.Parse(r.Canonical)
	if err != nil {
		return nil
	}
	root, ok := s.a.Force(eval.Root{Pkg: p.Package, Name: p.Root})
	if !ok {
		return nil
	}
	chain := make([]value.Value, 0, len(r.Steps)+1)
	chain = append(chain, root)
	for _, st := range r.Steps {
		chain = append(chain, st.Value)
	}
	return s.a.History(chain...)
}

// originOf is v's provenance in the API's form, with the text of v as it produced it; a value
// without provenance (a pseudo-field, an enum member) has the zero Origin.
func (s *snapshot) originOf(v value.Value) Origin {
	p := v.Prov()
	if p == nil {
		return Origin{}
	}
	o := s.provOrigin(p)
	o.Text = s.a.Produced(v).CanonText()
	if o.Via != nil && p.Kind == value.ProvSpread {
		o.Via.Text = o.Text // a spread copies the value its Via produced
	}
	return o
}

// provOrigin is one provenance located in the snapshot's files, Via's value unknown.
func (s *snapshot) provOrigin(p *value.Prov) Origin {
	files := s.a.Files()
	o := Origin{Kind: originKinds[p.Kind], Span: locate(files, p.Span), Pointer: p.Pointer, Layer: p.Layer, MoreFrames: p.MoreFrames}
	for _, fr := range p.Stack {
		o.Stack = append(o.Stack, Frame{Fn: fr.Fn, Span: locate(files, fr.Span)})
	}
	if p.Via != nil {
		via := s.provOrigin(p.Via)
		o.Via = &via
	}
	return o
}

// locate is a span of the snapshot for display (API.md §1.3); a span in no file is the zero Span.
func locate(files diag.Files, sp source.Span) Span {
	file := files.Path(sp.File)
	if file == "" {
		return Span{}
	}
	out := Span{File: file}
	out.Line, out.Col = files.Position(sp.File, sp.Start)
	out.EndLine, out.EndCol = files.Position(sp.File, sp.End)
	return out
}

// editabilityOf is edit's answer in the API's form (API.md §5.2, §7.2).
func editabilityOf(e edit.Editability) Editability {
	return Editability{Mode: editModes[e.Mode], Reason: reasons[e.Reason], File: e.File, Origin: e.Origin, Layer: e.Layer}
}

// kindOf is the ValueKind of v, "" for a value no kind names (a pair).
func kindOf(v value.Value) ValueKind {
	switch x := v.(type) {
	case *value.Record:
		if _, isCase := x.T.Base().(*types.CaseType); isCase {
			return KindVariant
		}
		return KindRecord
	case *value.List:
		if lt, ok := x.T.Base().(*types.ListType); ok && lt.KeyedBy != nil {
			return KindKeyedList
		}
		return KindList
	case *value.Str:
		if isAsset(x.T) {
			return KindAsset
		}
		return KindString
	case *value.Member, *value.CaseKind:
		return KindEnum
	case *value.Bool:
		return KindBool
	case *value.Int:
		return KindInt
	case *value.Float:
		return KindFloat
	case *value.Dur:
		return KindDuration
	case *value.Table:
		return KindTable
	case *value.Map:
		return KindMap
	case *value.Ref:
		return KindRef
	case *value.None:
		return KindNone
	case *value.Range:
		return KindRange
	}
	return ""
}

// isAsset reports a type whose refinements include asset(…).
func isAsset(t types.Type) bool {
	for t != nil {
		r, ok := t.Underlying().(*types.Refined)
		if !ok {
			return false
		}
		if r.Asset != nil {
			return true
		}
		t = r.Of
	}
	return false
}

// childSegs are v's fields but inputs, which have no value (EVALUATION.md §11.2), or its positions.
func childSegs(v value.Value) []edit.Seg {
	var n int
	switch x := v.(type) {
	case *value.Record:
		var out []edit.Seg
		for i, f := range verify.Fields(x.T) {
			if f.Input == nil && i < len(x.Fields) && x.Fields[i] != nil {
				out = append(out, edit.Seg{Kind: edit.SegField, Name: f.Name})
			}
		}
		return out
	case *value.List:
		n = len(x.Elems)
	case *value.Table:
		n = len(x.Entries)
	case *value.Map:
		n = len(x.Keys)
	}
	out := make([]edit.Seg, n)
	for i := range out {
		out[i] = edit.Seg{Kind: edit.SegPos, Pos: i}
	}
	return out
}

// wireJSON is v's wire form as emit json writes it inside `value`, compact (WIRE.md §8.2).
func wireJSON(v value.Value) []byte {
	doc, err := (&wire.Document{Schema: wireSchema, Kind: types.Record, V: v}).Encode()
	if errors.Is(err, wire.ErrNoWire) {
		return nil // WIRE.md §5.9: no wire form
	}
	var file struct {
		Value json.RawMessage `json:"value"`
	}
	if err == nil {
		err = json.Unmarshal(doc, &file)
	}
	var out bytes.Buffer
	if err == nil {
		err = json.Compact(&out, file.Value)
	}
	if err != nil {
		panic(internalError(err))
	}
	return out.Bytes()
}
