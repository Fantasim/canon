package cli

import (
	"encoding/json"
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

// explainLine is `canon explain --format json`'s one line (CLI.md §3.7, IMPLEMENTATION-PLAN.md §8.1).
type explainLine struct {
	Explain explainObject `json:"explain"`
}

// explainObject is a value: its qualified path (API.md P10), type, text, wire form, origin, and
// the same object for each part down to --depth; parts is omitted when none are written.
type explainObject struct {
	Path   string          `json:"path"`
	Type   string          `json:"type"`
	Text   string          `json:"text"`
	Value  json.RawMessage `json:"value"`
	Origin *originObject   `json:"origin"`
	Parts  []explainObject `json:"parts,omitempty"`
}

// originObject is Value.Origin in lowerCamel, its fields in declaration order, empty ones omitted.
type originObject struct {
	Kind canon.OriginKind `json:"kind,omitempty"`
	canon.Span
	Pointer    string        `json:"pointer,omitempty"`
	Layer      string        `json:"layer,omitempty"`
	Via        *originObject `json:"via,omitempty"`
	Stack      []canon.Frame `json:"stack,omitempty"`
	Replaced   *originObject `json:"replaced,omitempty"`
	Text       string        `json:"text,omitempty"`
	MoreFrames int           `json:"moreFrames,omitempty"`
}

// object is v's JSON object with its parts down to depth levels (-1: every part), an input field among them as its own object (EVALUATION.md §11.2).
func (s *partSource) object(v *canon.Value, depth int) (explainObject, error) {
	out := explainObject{Path: v.Path, Type: v.Type.Expr, Text: v.Text, Value: v.JSON(), Origin: originJSON(&v.Origin)}
	if depth == 0 {
		return out, nil
	}
	ps, err := s.parts(v)
	if err != nil {
		return out, err
	}
	for _, c := range ps {
		if c.value == nil {
			out.Parts = append(out.Parts, inputObject(c.path, c.env))
			continue
		}
		o, err := s.object(c.value, depth-1)
		if err != nil {
			return out, err
		}
		out.Parts = append(out.Parts, o)
	}
	return out, nil
}

// inputObject is an input field's object: it has no type, value or origin (EVALUATION.md §11.2).
func inputObject(path, env string) explainObject {
	return explainObject{Path: path, Text: fmt.Sprintf(fmtInputFrom, env), Origin: &originObject{}}
}

// originJSON is o's object; the zero Origin of a value without provenance is `{}`.
func originJSON(o *canon.Origin) *originObject {
	out := &originObject{Kind: o.Kind, Span: o.Span, Pointer: o.Pointer, Layer: o.Layer, Stack: o.Stack, Text: o.Text, MoreFrames: o.MoreFrames}
	if o.Via != nil {
		out.Via = originJSON(o.Via)
	}
	if o.Replaced != nil {
		out.Replaced = originJSON(o.Replaced)
	}
	return out
}
