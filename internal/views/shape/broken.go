package shape

import "github.com/fantasim/canonlang/internal/types"

// IsError reports the error type (TYPES.md 1).
func IsError(t types.Type) bool { return t != nil && t.Kind() == types.Error }
