package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// failingHost is the test host whose Default of the field named field fails, as a host that
// cannot compute that default.
type failingHost struct {
	evalHost
	field string
}

func (h failingHost) Default(ctx context.Context, f *types.Field, in wire.Instance, via *value.Prov) (value.Value, bool) {
	if f.Name == h.field {
		return nil, false
	}
	return h.evalHost.Default(ctx, f, in, via)
}

// failingOn opens fsys as open does, its host failing the default of field.
func failingOn(t *testing.T, fsys mapFS, layers []string, editLayer, field string) session {
	t.Helper()
	s := open(t, fsys, layers, editLayer, "d")
	s.env.Host = func(a *build.Analysis) wire.Host { return failingHost{newEvalHost(a), field} }
	return s
}

// API.md E6, E22, W5 (EDIT-E6RECORD rulings): a layer Set whose E6 judgement needs a default the
// host cannot compute is refused `computed` with no way out, nothing written; a base Set then writes
// its value, and its Undo restores every value.
func TestE6UndecidedDefault(t *testing.T) {
	fsys := boundLaw(boundSrc, "h: { n: 1, p: { x: true } }")
	op := setTo("d:z.h.p", "{ x: false }")
	s := failingOn(t, fsys, []string{"sb"}, "sb", "p")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	var ne *edit.NotEditableError
	if !errors.As(err, &ne) || ne.Reason != edit.ReasonComputed || ne.Detail != "" || plan != nil {
		t.Errorf("layer Set: %v, %v, want computed with no way out and no plan", plan, err)
	}
	b := failingOn(t, fsys, nil, "", "p")
	plan, err = edit.Apply(context.Background(), b.env, b.snap, edit.Request{Ops: []edit.Operation{op}})
	if err != nil {
		t.Fatalf("base Set: %v", err)
	}
	after := written(fsys, plan)
	u := failingOn(t, after, nil, "", "p")
	undo, err := edit.Apply(context.Background(), u.env, u.snap, edit.Request{Ops: plan.Undo})
	if err != nil {
		t.Fatalf("base Set, Undo %v: %v", plan.Undo, err)
	}
	for _, ls := range [][]string{{"sb"}, nil} {
		sameIn(t, "base Set, Undo", fsys, written(after, undo), ls)
	}
}
