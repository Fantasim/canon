package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Step is one segment of a resolved path: the container it was read in, and the value there.
type Step struct {
	Seg       Seg
	Container types.Type
	Value     value.Value
}

// Resolved is a path read against a snapshot: canonical form, one step per segment, target.
type Resolved struct {
	Canonical string
	Steps     []Step
	Target    value.Value
}
