package canon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/live"
	"github.com/fantasim/canonlang/internal/workspace"
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

// MarshalJSON writes the heading with its cells as {} when it has none (rule V4a).
func (h Heading) MarshalJSON() ([]byte, error) {
	type plain Heading
	p := plain(h)
	p.Cells = orEmpty(p.Cells)
	return encodeJSON(typeHeading, p)
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

// MarshalJSON writes every key, an empty collection as [] or {}, never null (rule V4a).
func (r EvalResult) MarshalJSON() ([]byte, error) {
	type plain EvalResult
	p := plain(r)
	p.When, p.Headings, p.Types = orEmpty(p.When), orEmpty(p.Headings), orEmpty(p.Types)
	p.Show, p.Findings, p.Dropped = orNone(p.Show), orNone(p.Findings), orNone(p.Dropped)
	return encodeJSON(typeEvalResult, p)
}

// encodeJSON is v's JSON form, a failure naming its API type.
func encodeJSON(typeName string, v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf(fmtEncodeJSON, typeName, err)
	}
	return b, nil
}

func orEmpty[V any](m map[string]V) map[string]V {
	if m == nil {
		return map[string]V{}
	}
	return m
}

func orNone[E any](s []E) []E {
	if s == nil {
		return []E{}
	}
	return s
}

// Evaluate computes the view state of r.Path with r.Draft applied in memory (rules V4a-V14):
// a shared read of the snapshot (S8) that never writes (V13). A draft is not applied yet: a
// request holding one fails with an *InternalError until drafts are implemented (rule X2).
func (p *Project) Evaluate(ctx context.Context, r EvalRequest) (res *EvalResult, err error) {
	defer recoverInternal(&err)
	s, err := p.read(ctx)
	if err != nil {
		return nil, err
	}
	if len(r.Draft) > 0 {
		return nil, errUnimplemented()
	}
	parsed, err := edit.Parse(r.Path)
	if err != nil {
		return nil, syntaxError(r.Path, err, 0)
	}
	a, err := analyze(ctx, s, nil) // every package, as Value: one analysis serves both (S8)
	if err != nil {
		return nil, err
	}
	snap := &snapshot{a: a, s: edit.NewSnapshot(a)}
	at, err := snap.evalTarget(r.Path, parsed)
	if err != nil {
		return nil, err
	}
	lang := r.Lang
	if lang == "" {
		lang = p.lang
	}
	e := workspace.Eval{Analysis: a, Edit: snap.s, At: at, Lang: lang}
	if err := workspace.EvalStale(s, string(r.Base), e); err != nil {
		return nil, evalError(ctx, err)
	}
	ev, err := workspace.Evaluate(ctx, s, e)
	if err != nil {
		return nil, evalError(ctx, err)
	}
	res = evalResultOf(at.Canonical, ev)
	if res.Revision, err = p.revision(ctx, s); err != nil {
		return nil, err
	}
	return res, nil
}

// evalTarget is the value at path: an input field has none (R5), an enum member is no value
// Evaluate reads (P7a).
func (s *snapshot) evalTarget(path string, parsed edit.Path) (edit.Resolved, error) {
	r, err := edit.Resolve(s.s, parsed)
	switch {
	case err != nil:
		return r, s.resolveError(path, parsed, err)
	case r.Target == nil:
		return r, &PathError{Op: -1, Path: path, Err: ErrInputField, Detail: inputEnv(r)}
	case len(r.Steps) == 1 && isEnum(r.Steps[0].Container):
		return r, &PathError{Op: -1, Path: path, Err: ErrBadOp}
	}
	return r, nil
}

func isEnum(t types.Type) bool {
	_, ok := t.(*types.EnumType)
	return ok
}

// evalError is a failed evaluation as the API reports it: the call's cancellation (S11), a
// stale base (S5), an unsupported load (DECISIONS 196), any other failure a compiler bug (X2).
func evalError(ctx context.Context, err error) error {
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, build.ErrLoad), errors.Is(err, workspace.ErrClosed), errors.Is(err, build.ErrInternal),
		errors.Is(err, workspace.ErrStale):
		return apiError(err)
	}
	var ie *InternalError
	if errors.As(panicError(err), &ie) {
		return ie
	}
	return internalError(err)
}

// evalResultOf is ev in the API's form, every collection its own: ev may be shared (S8).
func evalResultOf(path string, ev *workspace.Evaluation) *EvalResult {
	st := ev.State
	out := &EvalResult{
		Path: path, Title: Text(st.Title), Subtitle: Text(st.Subtitle), Preview: st.Preview,
		When: maps.Clone(orEmpty(st.When)), Show: make([]ShowLine, 0, len(st.Show)),
		Headings: make(map[string]Heading, len(st.Headings)), Types: make(map[string]json.RawMessage, len(st.Types)),
		Findings: orNone(fromDiag(ev.Files, ev.Findings)), Summary: summaryOf(ev.Summary), Dropped: []Dropped{},
	}
	for _, l := range st.Show {
		out.Show = append(out.Show, ShowLine{Owner: l.Owner, Key: l.Key, Label: l.Label, Text: Text(l.Text)})
	}
	//canon:unordered each heading is copied into a map under its own key
	for key, h := range st.Headings {
		out.Headings[key] = headingOf(h)
	}
	//canon:unordered each type is copied into a map under its own key
	for key, t := range st.Types {
		out.Types[key] = slices.Clone(t)
	}
	return out
}

func headingOf(h live.Heading) Heading {
	out := Heading{Title: Text(h.Title), Subtitle: Text(h.Subtitle), Preview: h.Preview, Retired: h.Retired, Cells: make(map[string]Text, len(h.Cells))}
	//canon:unordered each cell is copied into a map under its own key
	for key, c := range h.Cells {
		out.Cells[key] = Text(c)
	}
	return out
}
