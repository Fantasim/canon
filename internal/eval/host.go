package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Host is how the evaluator reaches load and verify; build injects it (IMPLEMENTATION-PLAN §4.8).
type Host interface {
	// Load forces a load expression against its type (WIRE.md §6); false: poisoned (EVALUATION.md §7.2).
	Load(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool)
	// Verify runs stage B on a top-level value (EVALUATION.md §5); false: it is invalid.
	Verify(ctx context.Context, root Root, v value.Value) bool
}

// Root names a top-level value, a const or a let of a package (EVALUATION.md §2.1).
type Root struct {
	Pkg, Name string
}
