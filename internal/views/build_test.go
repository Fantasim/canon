package views_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views"
)

// VIEWMODEL.md J3: every member but `studio` is present, empty until its section is built.
func TestEnvelope(t *testing.T) {
	m := demo(t, "package a\n", "").model(t, demoPkg)
	got := text(m)
	for _, member := range []string{`"requires":[]`, `"types":{}`, `"views":{}`, `"values":{}`, `"usage":{}`,
		`"search":{}`, `"assets":{}`, `"units":{}`, `"widgets":{}`, `"findings":[]`} {
		if !strings.Contains(got, member) {
			t.Errorf("model %s lacks %s", got, member)
		}
	}
}

// A model is of a package the checked program holds; a cancelled build returns its context's error.
func TestBuildErrors(t *testing.T) {
	x := demo(t, "package a\n", "")
	_, err := views.Build(context.Background(), views.Input{Program: x.a.Program(), Package: "nowhere"})
	if !errors.Is(err, views.ErrNoPackage) {
		t.Errorf("Build(nowhere) = %v, want ErrNoPackage", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := views.Build(ctx, views.Input{Program: x.a.Program(), Package: demoPkg}); !errors.Is(err, context.Canceled) {
		t.Errorf("Build(cancelled) = %v, want context.Canceled", err)
	}
}

// A build cancelled while `types` is written returns the context's error, not a partial model.
func TestBuildCancelledDuringTypes(t *testing.T) {
	x := demo(t, refTarget, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	force := func(r eval.Root) (value.Value, bool) {
		cancel()
		return x.a.Force(r)
	}
	m, err := views.Build(ctx, views.Input{Program: x.a.Program(), Package: demoPkg, Force: force})
	if m != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("Build = %v, %v; want no model and context.Canceled", m != nil, err)
	}
}
