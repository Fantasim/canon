package edit

import (
	"context"

	"github.com/fantasim/canonlang/internal/value"
)

// SourceOf is v as the Source literal an Undo carries (API.md E23), printed against s: the
// minimal-write fuzz copies compound values with it.
func SourceOf(ctx context.Context, env Env, s *Snapshot, v value.Value) (Lit, error) {
	return newApplier(ctx, env, s).sourceLit(v, nil, stated{})
}
