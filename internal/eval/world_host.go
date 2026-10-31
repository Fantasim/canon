package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// literalsInto is a Host that verifies kept literals through ev, reporting into the bags it is given (TYPES.md §11.5).
type literalsInto interface {
	VerifyLiteralsInto(ctx context.Context, ev *Evaluator, root Root, lits []KeptLiteral, bags check.Bags)
}

// worldHost is a world's host: its loads are decoded afresh and its values verified, kept
// literals included, through the world's evaluator into the world's bags.
type worldHost struct {
	load   vectorLoader
	verify vectorVerifier
	lits   literalsInto // nil when the parent verifies no kept literal
	ev     *Evaluator
}

// newWorldHost is the host of a world under parent, nil when parent cannot serve one: a world
// that could not load, verify, or verify the kept literals its parent verifies, would drop findings.
func newWorldHost(parent Host) *worldHost {
	l, loads := parent.(vectorLoader)
	v, verifies := parent.(vectorVerifier)
	if !loads || !verifies {
		return nil
	}
	h := &worldHost{load: l, verify: v}
	if _, kept := parent.(LiteralVerifier); kept {
		li, ok := parent.(literalsInto)
		if !ok {
			return nil
		}
		h.lits = li
	}
	return h
}

// Load decodes a load afresh through the world's evaluator (ADR-0023 §2 Forecast 8).
func (h *worldHost) Load(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool) {
	return h.load.LoadInto(ctx, h.ev, e, expected, h.ev.bags)
}

// Verify runs stage B on a world value through the world's evaluator (EVALUATION.md §10.5 item 3).
func (h *worldHost) Verify(ctx context.Context, root Root, v value.Value) bool {
	return h.verify.VerifyInto(ctx, h.ev, root, v, h.ev.bags)
}

// VerifyLiterals verifies the kept literals a world value's evaluation built (EVALUATION.md §5).
func (h *worldHost) VerifyLiterals(ctx context.Context, root Root, lits []KeptLiteral) {
	h.lits.VerifyLiteralsInto(ctx, h.ev, root, lits, h.ev.bags)
}
