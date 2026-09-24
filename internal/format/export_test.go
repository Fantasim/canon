package format

import "github.com/fantasim/canonlang/internal/syntax"

// KeptCommas reports, per token of f, a comma DECISIONS 216 keeps.
func KeptCommas(f *syntax.File) []bool { return keptCommas(f) }
