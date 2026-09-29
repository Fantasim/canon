package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"

	canon "github.com/fantasim/canonlang/api"
)

// cycleState is what one run of a watched command leaves for the next cycle to compare with: its result, the findings it shows, which -q has thinned (CLI.md §2.3), the files a build wrote, and the failure of a re-check that could not run.
type cycleState struct {
	check   *canon.CheckResult
	build   *canon.BuildResult // nil for check
	shown   []canon.Finding
	good    []canon.Finding // a failed state's findings before the failure
	wrote   map[string]bool
	failure error
}

func (inv *invocation) checkState(res *canon.CheckResult) *cycleState {
	return &cycleState{check: res, shown: inv.shown(res.Findings)}
}

func (inv *invocation) buildState(res *canon.BuildResult) *cycleState {
	s := &cycleState{check: res.Check, build: res, shown: inv.shown(res.Check.Findings), wrote: map[string]bool{}}
	for _, o := range res.Outputs {
		if countsWritten(o.Status) {
			s.wrote[o.Path] = true
		}
	}
	for _, l := range res.Lock {
		s.wrote[l.File] = true
	}
	return s
}

// failedState is the state of a re-check that failed: the findings its error carries, as a project.canon error does, added to the last good findings, which a failure proves nothing about, and the error itself (meta/decisions/log-2026-09-29.md "M4 U6-r").
func (inv *invocation) failedState(err error, prev *cycleState) *cycleState {
	var findings []canon.Finding
	var perr *canon.ProjectError
	if errors.As(err, &perr) {
		findings = perr.Findings
	}
	res := &canon.CheckResult{Findings: findings, Summary: summaryOf(findings)}
	good := prev.shown
	if prev.failure != nil {
		good = prev.good
	}
	shown := append(slices.Clone(good), inv.shown(findings)...)
	return &cycleState{check: res, shown: shown, good: good, failure: err}
}

// startWatch starts watching the project under --watch (API.md W12), its events queued for the command's loop: the callback runs on the watcher's goroutine and never calls the project, and stops sending once stop is called. Without --watch it returns no channel.
func (inv *invocation) startWatch(p *canon.Project) (events <-chan canon.Event, stop func(), err error) {
	if !inv.opt.watch {
		return nil, func() {}, nil
	}
	ctx, stop := context.WithCancel(inv.ctx)
	queue := make(chan canon.Event, watchQueue)
	err = p.Watch(ctx, func(ev canon.Event) {
		select {
		case queue <- ev:
		case <-ctx.Done():
		}
	})
	if err != nil {
		stop()
		return nil, func() {}, fmt.Errorf(fmtWrap, err)
	}
	return queue, stop, nil
}

// findingKey is what matches a finding between two cycles, not its position (IMPLEMENTATION-PLAN.md §8.1).
type findingKey struct {
	code, file, path, message string
}

func keyOf(f canon.Finding) findingKey {
	return findingKey{code: f.Code, file: f.File, path: f.Path, message: f.Message}
}

// unmatched is the findings of a that b has no counterpart for, one counterpart matching one finding.
func unmatched(a, b []canon.Finding) []canon.Finding {
	left := map[findingKey]int{}
	for _, f := range b {
		left[keyOf(f)]++
	}
	var out []canon.Finding
	for _, f := range a {
		if k := keyOf(f); left[k] > 0 {
			left[k]--
			continue
		}
		out = append(out, f)
	}
	return out
}

// writeCycle prints one cycle: its header, the findings that appeared, the ones that disappeared, the summary (IMPLEMENTATION-PLAN.md §8.1); a failed re-check also prints its error.
func (inv *invocation) writeCycle(ev canon.Event, prev, cur *cycleState) error {
	added, fixed := unmatched(cur.shown, prev.shown), unmatched(prev.shown, cur.shown)
	if cur.failure != nil {
		inv.reportError(cur.failure)
	}
	if inv.opt.format == formatJSON {
		return inv.writeCycleJSON(ev, added, fixed, cur)
	}
	return inv.writeCycleText(ev, added, fixed, cur)
}
