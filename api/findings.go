package canon

import (
	"context"
	"time"
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
	Pointer string    `json:"pointer,omitempty"`
	Package string    `json:"package"`
	Path    string    `json:"path,omitempty"`
	Message string    `json:"message"`
	Check   string    `json:"check,omitempty"`
	Layer   string    `json:"layer,omitempty"`
	Related []Related `json:"related,omitempty"`
	Stack   []Frame   `json:"stack,omitempty"`
	Reads   []string  `json:"reads,omitempty"`
}

// MarshalJSON writes the finding with the key order and omissions of rule F5.
func (f Finding) MarshalJSON() ([]byte, error) {
	return nil, errUnimplemented()
}

// UnmarshalJSON reads the form written by MarshalJSON (rule F6).
func (f *Finding) UnmarshalJSON(data []byte) error {
	return errUnimplemented()
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
