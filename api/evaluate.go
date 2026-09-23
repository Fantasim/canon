package canon

import (
	"context"
	"encoding/json"
)

// EvalRequest asks for the live view state of a value, with a draft applied (API.md §11).
type EvalRequest struct {
	Base  Revision `json:"base,omitempty"`
	Path  string   `json:"path"`
	Draft []Op     `json:"draft,omitempty"`
	Lang  string   `json:"lang,omitempty"`
}

// Text is an evaluated text (rules V8, V11).
type Text struct {
	Value    string `json:"value"`
	OK       bool   `json:"ok"`
	Fallback bool   `json:"fallback"`
}

// ShowLine is one evaluated show line or view method (rule V10).
type ShowLine struct {
	Owner string `json:"owner"`
	Key   string `json:"key"`
	Label string `json:"label"`
	Text  Text   `json:"text"`
}

// Heading is how an element of a collection is listed (rules V6, V6a).
type Heading struct {
	Title    Text            `json:"title"`
	Subtitle Text            `json:"subtitle"`
	Preview  string          `json:"preview"`
	Retired  bool            `json:"retired"`
	Cells    map[string]Text `json:"cells"`
}

// EvalResult is the live view state of a value (API.md §11).
type EvalResult struct {
	Revision Revision                   `json:"revision"`
	Path     string                     `json:"path"`
	Title    Text                       `json:"title"`
	Subtitle Text                       `json:"subtitle"`
	Preview  string                     `json:"preview"`
	When     map[string]bool            `json:"when"`
	Show     []ShowLine                 `json:"show"`
	Headings map[string]Heading         `json:"headings"`
	Types    map[string]json.RawMessage `json:"types"`
	Findings []Finding                  `json:"findings"`
	Summary  Summary                    `json:"summary"`
	Dropped  []Dropped                  `json:"dropped"`
}

// Evaluate computes the view state of r.Path with r.Draft applied in memory (rules V4a-V14).
func (p *Project) Evaluate(ctx context.Context, r EvalRequest) (*EvalResult, error) {
	return nil, errUnimplemented()
}
