package vm

import "regexp"

const (
	maxSafeInt = 1<<53 - 1 // VIEWMODEL.md J10: ±(2^53−1)

	fmtBad    = "%w: %q"
	fmtWrap   = "vm: %w"
	fmtWrapIn = "%w: %w"
)

var (
	reNumber  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
	reInteger = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	reDecimal = regexp.MustCompile(`^-?[0-9]+$`) // the schema's decimal-string pattern (J10)
	quote     = []byte(`"`)
)
