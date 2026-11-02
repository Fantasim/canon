package check

import (
	"maps"
	"slices"
)

// What internal/project's out_templates must agree with (CODEGEN.md §2.1, DECISIONS 424, 439).
const (
	OutCharset = outCharset
	TSSuffix   = tsSuffix
)

func EmitTargets() []string { return slices.Sorted(maps.Keys(emitSpecs)) }
