package canon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// Span is a range in a file: 1-based, UTF-8 byte columns, exclusive end (API.md §1.3).
type Span struct {
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Col     int    `json:"col,omitempty"`
	EndLine int    `json:"endLine,omitempty"`
	EndCol  int    `json:"endCol,omitempty"`
}

// Related is another location of a finding, with a note (rule F12).
type Related struct {
	Span
	Note string `json:"note"`
}

// Frame is one frame of a Canon call stack.
type Frame struct {
	Fn string `json:"fn"`
	Span
}

// Finding is a compiler error or warning, or the result of a check or warn (API.md §4.1).
type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Span
	Pointer    string    `json:"pointer,omitempty"`
	Package    string    `json:"package"`
	Path       string    `json:"path,omitempty"`
	Message    string    `json:"message"`
	Check      string    `json:"check,omitempty"`
	Layer      string    `json:"layer,omitempty"`
	Related    []Related `json:"related,omitempty"`
	Stack      []Frame   `json:"stack,omitempty"`
	MoreFrames int       `json:"moreFrames,omitempty"`
	Reads      []string  `json:"reads,omitempty"`
}

// MarshalJSON writes the finding with the key order and omissions of rule F5.
func (f Finding) MarshalJSON() ([]byte, error) {
	l, err := f.located()
	if err != nil {
		return nil, err
	}
	return l.AppendJSON(nil), nil
}

// UnmarshalJSON reads the form written by MarshalJSON (rule F6); an unknown key or severity fails.
func (f *Finding) UnmarshalJSON(data []byte) error {
	type plain Finding
	var p plain
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return fmt.Errorf(fmtDecodeFinding, err)
	}
	if _, ok := severities[p.Severity]; !ok {
		return fmt.Errorf(fmtDecodeFinding, fmt.Errorf(fmtSeverity, errSeverity, p.Severity))
	}
	*f = Finding(p)
	return nil
}

// WriteOptions chooses the form WriteFindings writes and its summary line.
type WriteOptions struct {
	JSON     bool // one JSON object per line (rule F6) instead of the text form (API.md §4.4)
	Summary  Summary
	Duration time.Duration
	Golden   bool // the text form writes the duration (…), as goldens do (IMPLEMENTATION-PLAN.md §7.2)
}

// WriteFindings writes findings in F2 order, then the summary line (API.md §4.2, §4.4).
func WriteFindings(w io.Writer, findings []Finding, o WriteOptions) error {
	located := make([]diag.Located, 0, len(findings))
	for _, f := range findings {
		l, err := f.located()
		if err != nil {
			return err
		}
		located = append(located, l)
	}
	opt := diag.RenderOptions{Summary: o.Summary.diag(), Duration: o.Duration, Golden: o.Golden}
	if o.JSON {
		opt.Format = diag.FormatJSON
	}
	return diag.Write(w, located, opt)
}

// located is the finding as diag writes it; a severity other than error or warning fails.
func (f Finding) located() (diag.Located, error) {
	sev, ok := severities[f.Severity]
	if !ok {
		return diag.Located{}, fmt.Errorf(fmtSeverity, errSeverity, f.Severity)
	}
	l := diag.Located{
		Code: diag.Code(f.Code), Severity: sev, Loc: f.loc(),
		Pointer: f.Pointer, Package: f.Package, Path: f.Path, Message: f.Message,
		Check: f.Check, Layer: f.Layer, MoreFrames: f.MoreFrames, Reads: f.Reads,
	}
	for _, r := range f.Related {
		l.Related = append(l.Related, diag.RelatedLoc{Loc: r.loc(), Note: r.Note})
	}
	for _, fr := range f.Stack {
		l.Stack = append(l.Stack, diag.FrameLoc{Fn: fr.Fn, Loc: fr.loc()})
	}
	return l, nil
}

func (s Span) loc() source.Location {
	return source.Location{Path: s.File, Line: s.Line, Col: s.Col, EndLine: s.EndLine, EndCol: s.EndCol}
}

// Summary counts the findings of a result, those dropped by truncation included (rule F7).
type Summary struct {
	Errors    int          `json:"errors"`
	Warnings  int          `json:"warnings"`
	Packages  int          `json:"packages"`
	Truncated []Truncation `json:"truncated,omitempty"`
}

// Truncation reports findings that were counted but not kept for one package.
type Truncation struct {
	Package  string `json:"package"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
}

// diag is the summary as diag writes it.
func (s Summary) diag() diag.Summary {
	out := diag.Summary{Errors: s.Errors, Warnings: s.Warnings, Packages: s.Packages}
	for _, t := range s.Truncated {
		out.Truncated = append(out.Truncated, diag.Truncation{Package: t.Package, Errors: t.Errors, Warnings: t.Warnings})
	}
	return out
}

// CheckResult is the result of Check and LockCheck (API.md §5.1).
type CheckResult struct {
	Revision Revision
	Packages []string
	Findings []Finding
	Summary  Summary
	Duration time.Duration
}

// HasErrors reports whether the result holds at least one error finding.
func (r *CheckResult) HasErrors() bool {
	return r != nil && r.Summary.Errors > 0
}

// Check runs build phases 1-7 on the selected packages and returns their findings (rules R1-R3).
func (p *Project) Check(ctx context.Context, packages ...string) (*CheckResult, error) {
	return nil, errUnimplemented()
}

// LockCheck verifies canon.lock of the selected packages (rule B4).
func (p *Project) LockCheck(ctx context.Context, packages ...string) (*CheckResult, error) {
	return nil, errUnimplemented()
}
