package canon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// API.md X2, DECISIONS 195, DECISIONS 196: Evaluate's failures (Analysis.ViewErr) as the API
// reports them: an unsupported load as itself, a compiler bug or a panic as an *InternalError,
// a stale base as a *StaleError, a closed project as ErrClosed, a cancelled call as ctx.Err().
func TestEvalError(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, c := range []struct {
		ctx      context.Context
		err      error
		want     error
		internal bool
	}{
		{context.Background(), fmt.Errorf("x: %w", build.ErrLoad), build.ErrLoad, false},
		{context.Background(), fmt.Errorf("x: %w", build.ErrInternal), ErrInternal, true},
		{context.Background(), &safego.PanicError{Value: "bug"}, ErrInternal, true},
		{context.Background(), errors.New("live: a type that does not encode"), ErrInternal, true},
		{context.Background(), workspace.ErrClosed, ErrClosed, false},
		{context.Background(), &workspace.StaleError{}, ErrStale, false},
		{context.Background(), errors.New("edit: a canonical path that does not parse"), ErrInternal, true},
		{cancelled, fmt.Errorf("live: %w", context.Canceled), context.Canceled, false},
	} {
		got := evalError(c.ctx, c.err)
		var ie *InternalError
		if !errors.Is(got, c.want) || errors.As(got, &ie) != c.internal {
			t.Errorf("evalError(%v) = %v, want %v (internal %v)", c.err, got, c.want, c.internal)
		}
	}
}

// API.md S4, API.md S5: workspace's staleness, through apiError, is a *StaleError, same files.
func TestStaleErrorMapping(t *testing.T) {
	var se *StaleError
	err := apiError(&workspace.StaleError{Files: []string{"a/a.canon"}})
	if !errors.As(err, &se) || !errors.Is(err, ErrStale) || !slices.Equal(se.Files, []string{"a/a.canon"}) {
		t.Errorf("staleError: %v", err)
	}
	if staleError(nil) != nil {
		t.Error("staleError(nil) is not nil")
	}
}
