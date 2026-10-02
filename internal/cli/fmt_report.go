package cli

import (
	"bytes"
	"fmt"
	"io"
	"time"

	udiff "github.com/aymanbagabas/go-udiff"

	"github.com/fantasim/canonlang/internal/diag"
)

// fileLine is the JSON line of a file not in the canonical layout, with its diff under --diff (IMPLEMENTATION-PLAN.md §8.1).
type fileLine struct {
	File      string `json:"file"`
	Formatted bool   `json:"formatted"`
	Diff      string `json:"diff,omitempty"`
}

// fmtSummary is the closing JSON line: the files visited and how many were not formatted (IMPLEMENTATION-PLAN.md §8.1).
type fmtSummary struct {
	Summary struct {
		Files       int `json:"files"`
		Unformatted int `json:"unformatted"`
	} `json:"summary"`
}

// report prints the result: text lists or diffs the changes, then the findings; JSON, findings, files, summary (CLI.md §3.6).
func (r *fmtRun) report(start time.Time) error {
	if r.inv.opt.format == formatJSON {
		return r.reportJSON()
	}
	if r.inv.opt.diff || r.inv.opt.checkFlag {
		for _, c := range r.changes {
			text, err := r.changeText(c)
			if err != nil {
				return err
			}
			if err := writeText(r.inv.env.Stdout, text); err != nil {
				return err
			}
		}
	}
	return r.writeFindings(time.Since(start))
}

// changeText is a change as the text report prints it: its diff under --diff, else its path.
func (r *fmtRun) changeText(c fmtChange) (string, error) {
	if r.inv.opt.diff {
		return unifiedDiff(c.display, c.old, c.updated)
	}
	return c.display + lineBreak, nil
}

// reportJSON prints the findings, a line per file not formatted, whatever the mode, and the summary.
func (r *fmtRun) reportJSON() error {
	if err := r.writeFindingLines(); err != nil {
		return err
	}
	for _, c := range r.changes {
		line := fileLine{File: c.display}
		if r.inv.opt.diff {
			var err error
			if line.Diff, err = unifiedDiff(c.display, c.old, c.updated); err != nil {
				return err
			}
		}
		if err := r.inv.writeJSONLine(line); err != nil {
			return err
		}
	}
	var sum fmtSummary
	sum.Summary.Files, sum.Summary.Unformatted = r.visited, len(r.changes)
	return r.inv.writeJSONLine(sum)
}

// unifiedDiff is old to updated as a unified diff of the file name, "" when equal. CLI.md §3.6
func unifiedDiff(name string, old, updated []byte) (string, error) {
	before := string(old)
	d, err := udiff.ToUnified(name, name, before, udiff.Lines(before, string(updated)), udiff.DefaultContextLines)
	if err != nil {
		return "", fmt.Errorf(fmtWrap, err)
	}
	return fixHunkStarts(d), nil
}

func writeText(w io.Writer, s string) error {
	if _, err := io.WriteString(w, s); err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return nil
}

// writeFindings prints the findings through the one writer of diag, its summary line after them;
// a run without findings prints none.
func (r *fmtRun) writeFindings(d time.Duration) error {
	if len(r.findings) == 0 {
		return nil
	}
	opt := diag.RenderOptions{Duration: d}
	for _, f := range r.findings {
		if f.Severity == diag.Error {
			opt.Summary.Errors++
		} else {
			opt.Summary.Warnings++
		}
	}
	if err := diag.Write(r.inv.env.Stdout, diag.Locate(&r.set, r.findings), opt); err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return nil
}

// writeFindingLines prints the findings as JSON lines, diag's summary line left off: fmt has its own.
func (r *fmtRun) writeFindingLines() error {
	if len(r.findings) == 0 {
		return nil
	}
	var b bytes.Buffer
	opt := diag.RenderOptions{Format: diag.FormatJSON}
	if err := diag.Write(&b, diag.Locate(&r.set, r.findings), opt); err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return writeText(r.inv.env.Stdout, r.inv.withoutSummary(b.String()))
}
