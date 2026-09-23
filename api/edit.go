package canon

import (
	"context"
	"encoding/json"
)

// Edit is an atomic list of operations (API.md §8.1).
type Edit struct {
	Base        Revision `json:"base"`
	Ops         []Op     `json:"ops"`
	AllowErrors bool     `json:"allowErrors,omitempty"`
	DryRun      bool     `json:"dryRun,omitempty"`
	Normalize   bool     `json:"normalize,omitempty"`
	Evaluate    []string `json:"evaluate,omitempty"`
}

// FileChange is one file an edit wrote, or with DryRun would write.
type FileChange struct {
	Path    string
	Kind    ChangeKind
	OldPath string
	Before  []byte
	After   []byte
}

// Dropped is a value removed by a cascade, in its wire form (rules E14, E15).
type Dropped struct {
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// EditResult is the result of Edit (API.md §8.1).
type EditResult struct {
	Applied  bool
	Revision Revision
	Changes  []FileChange
	Findings []Finding
	Summary  Summary
	Dropped  []Dropped
	Undo     []Op
	Eval     map[string]*EvalResult
}

// Edit applies e atomically: all operations or none (rules E1-E26).
func (p *Project) Edit(ctx context.Context, e Edit) (*EditResult, error) {
	return nil, errUnimplemented()
}
