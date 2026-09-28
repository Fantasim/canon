package cli

import (
	"encoding/json"

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

// objectOf is v's JSON object with its parts down to depth levels (-1: every part).
func objectOf(v *canon.Value, depth int) explainObject {
	out := explainObject{Path: v.Path, Type: v.Type.Expr, Text: v.Text, Value: v.JSON(), Origin: originJSON(&v.Origin)}
	if depth == 0 {
		return out
	}
	for _, c := range v.Children() {
		out.Parts = append(out.Parts, objectOf(c, depth-1))
	}
	return out
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
