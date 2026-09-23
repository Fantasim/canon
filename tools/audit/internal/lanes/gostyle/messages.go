package gostyle

import (
	"fmt"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
)

// measured renders "<n> <unit> (max <limit>)": the shape most size/comment findings share.
func measured(n int, unit string, limit int) string { return fmt.Sprintf(fmtMeasured, n, unit, limit) }

// sizeFinding builds a line-anchored finding; Symbol is left for the runner to fill from
// Line, and Fix is left for the rulebook default.
func sizeFinding(rule, file string, line, value int, msg string) finding.Finding {
	return finding.Finding{Rule: rule, File: file, Line: line, Value: value, Message: msg}
}
