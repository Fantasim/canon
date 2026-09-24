package eval

import (
	"context"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// vectorLoader is a Host that forces a load reporting into the bags it is given.
type vectorLoader interface {
	LoadInto(ctx context.Context, e *syntax.LoadExpr, expected types.Type, bags check.Bags) (value.Value, bool)
}

// vectorVerifier is a Host that verifies a value through ev, reporting into the bags it is
// given; false: it is invalid.
type vectorVerifier interface {
	VerifyInto(ctx context.Context, ev *Evaluator, root Root, v value.Value, bags check.Bags) bool
}

// vectorHost is a vector's host: a load or a verification reports into throwaway bags, whose
// first error is the vector's (DECISIONS 204); a host that cannot do so serves none.
type vectorHost struct {
	parent Host
	ev     *Evaluator
}

// Load forces a load through the parent's LoadInto; without it the load is poisoned.
func (h *vectorHost) Load(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool) {
	l, ok := h.parent.(vectorLoader)
	if !ok {
		return nil, false
	}
	bags := h.throwaway()
	v, ok := l.LoadInto(ctx, e, expected, bags)
	h.note(bags)
	return v, ok
}

// Verify verifies a value first forced in the vector; without VerifyInto it cannot, and the
// vector has no outcome (EVALUATION.md phase 4 verifies every value).
func (h *vectorHost) Verify(ctx context.Context, root Root, v value.Value) bool {
	vr, ok := h.parent.(vectorVerifier)
	if !ok {
		h.ev.vec.void, h.ev.exhausted = true, true
		return false
	}
	bags := h.throwaway()
	valid := func() bool {
		h.ev.aside = bags // E3505 and evaluation findings of the verification join its own
		defer func() { h.ev.aside = nil }()
		return vr.VerifyInto(ctx, h.ev, root, v, bags)
	}()
	h.note(bags)
	return valid
}

// throwaway is one empty bag per package, discarded after one load or verification.
func (h *vectorHost) throwaway() check.Bags {
	bags := check.Bags{}
	files := h.ev.files()
	for pkg := range h.ev.pkgs { //canon:unordered one bag per package
		bags[pkg] = diag.NewBag(files, pkg)
	}
	return bags
}

// note keeps the first error the bags hold, packages by path, as the vector's.
func (h *vectorHost) note(bags check.Bags) {
	for _, pkg := range slices.Sorted(maps.Keys(bags)) {
		h.ev.vec.noteAll(h.ev, bags[pkg].Findings())
	}
}
