package live

import (
	"encoding/json"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/render"
)

// Target is the value at the request's path (API.md 11); Field holds it, nil for a top-level
// value or an element, and is read only with the record or case Decl declaring it.
type Target struct {
	Value value.Value
	Magic render.Magic // its `key` in a map, its `index` in a list; an entry's `id` is read from it
	Name  string       // its title without a view (V7): its key's text, `#<n>`, or its value name
	Lang  string       // the language of its texts; "" or the source language: the source (V8)
	Field *types.Field
	Decl  types.Type
}

// Text is an evaluated text (API.md V8, V11).
type Text struct {
	Value    string
	OK       bool
	Fallback bool
}

// ShowLine is an evaluated `show` line or view-named method (API.md V10).
type ShowLine struct {
	Owner, Key, Label string
	Text              Text
}

// Heading is how an element of a collection is listed (API.md V6, V6a).
type Heading struct {
	Title, Subtitle Text
	Preview         string
	Retired         bool
	Cells           map[string]Text
}

// Result is the live view state of a value (API.md 11), its keys relative to it (V9).
type Result struct {
	Title, Subtitle Text
	Preview         string
	When            map[string]bool
	Show            []ShowLine
	Headings        map[string]Heading
	Types           map[string]json.RawMessage
}
