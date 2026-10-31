package build

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
)

// BuildPackage builds the world of an `expect package` as stages A to D do, into capture (EVALUATION.md §10.5 items 2 to 4).
func (s subjects) BuildPackage(ctx context.Context, w *eval.World, capture *diag.Bag) {
	ev, order := w.Evaluator(), w.Order()
	for _, root := range order { // stage A (§2.1)
		ev.Force(ctx, root)
	}
	ev.BeginVerification(ctx) // stage B (§5), through the world's host into its bags
	runner := rules.NewShared(s.r.rix, checks{Evaluator: ev}, s.captureBags(capture))
	if err := s.worldChecks(ctx, runner, ev, order); err != nil {
		s.host.errs = append(s.host.errs, internal(err))
	}
	s.r.refused = append(s.r.refused, ev.StableAmendments()...) // the run's, never the world's (LOCK.md §6.1)
}

// worldChecks runs stage C on each valid value of order, then stage D (EVALUATION.md §8.1, §8.2, §7.2).
func (s subjects) worldChecks(ctx context.Context, runner *rules.Runner, ev *eval.Evaluator, order []eval.Root) error {
	for _, root := range order {
		if v, ok := ev.Force(ctx, root); ok {
			if err := runner.Instances(ctx, root, v); err != nil {
				return err
			}
		}
	}
	for _, cp := range s.r.prog.Packages {
		if cp.Path == s.pkg {
			return runner.Package(ctx, cp)
		}
	}
	return nil
}

// captureBags maps every package of the program to capture.
func (s subjects) captureBags(capture *diag.Bag) check.Bags {
	bags := check.Bags{}
	for _, cp := range s.r.prog.Packages {
		bags[cp.Path] = capture
	}
	return bags
}
