package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

// runBuild is `canon build [packages…] [--watch]` (CLI.md §3.4): check, then write outputs and locks; with --watch, then the same after each change.
func runBuild(inv *invocation) int {
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	events, stop, err := inv.startWatch(p)
	if err != nil {
		return inv.fail(err)
	}
	defer stop()
	selectors := inv.selectors(p.Root())
	res, err := inv.build(p, selectors)
	if err != nil {
		return inv.fail(err)
	}
	if err := inv.writeBuild(res); err != nil {
		return inv.fail(err)
	}
	if events != nil {
		rerun := func() (*cycleState, error) {
			next, err := inv.build(p, selectors)
			if err != nil {
				return nil, err
			}
			return inv.buildState(next), nil
		}
		return inv.watchLoop(events, inv.buildState(res), rerun)
	}
	return inv.buildExit(res)
}

// build is one run of Build over the selected packages, an unknown selector reported as typed.
func (inv *invocation) build(p *canon.Project, selectors []string) (*canon.BuildResult, error) {
	res, err := p.Build(inv.ctx, canon.BuildOptions{
		Packages: selectors, Targets: inv.opt.targets, Check: inv.opt.checkFlag, Adopt: inv.opt.adopt,
	})
	if errors.Is(err, canon.ErrUnknownPackage) {
		err = inv.asTyped(p, selectors, err)
	}
	return res, err
}

// buildExit is build's exit code: errors, or stale outputs under --check, then too many warnings.
func (inv *invocation) buildExit(res *canon.BuildResult) int {
	switch {
	case res.Check.Summary.Errors > 0, res.Stale:
		return exitErrors
	case inv.opt.maxWarnings != unlimited && res.Check.Summary.Warnings > inv.opt.maxWarnings:
		return exitWarnings
	}
	return exitOK
}

// writeBuild prints the check's findings, then the build's own report; -q drops the report too, the error findings and the summary kept (DECISIONS 201, CLI.md §2.3; meta/decisions/log-2026-09-24.md "Chosen while resuming").
func (inv *invocation) writeBuild(res *canon.BuildResult) error {
	shown := inv.shown(res.Check.Findings)
	if inv.opt.format == formatJSON {
		return inv.writeBuildJSON(shown, res)
	}
	if err := inv.writeFindings(shown, res.Check.Summary, res.Check.Duration); err != nil {
		return err
	}
	if inv.opt.quiet {
		return nil
	}
	return inv.writeBuildText(changedOutputs(res.Outputs), res.Lock)
}

// changedOutputs drops an output a build left untouched: text lists changed outputs only, JSON lists every one (IMPLEMENTATION-PLAN.md §8.5, DECISIONS 201).
func changedOutputs(outputs []canon.Output) []canon.Output {
	out := make([]canon.Output, 0, len(outputs))
	for _, o := range outputs {
		if o.Status != canon.OutputUnchanged {
			out = append(out, o)
		}
	}
	return out
}

// writeBuildText is canon build's own text report: outputs grouped by target, a stale one prefixed under --check, then the lock section (IMPLEMENTATION-PLAN.md §8.5, DECISIONS 201).
func (inv *invocation) writeBuildText(outputs []canon.Output, locks []canon.LockChange) error {
	for _, o := range outputs {
		if o.Status == canon.OutputAdopted {
			writeLine(inv.env.Stdout, fmt.Sprintf(fmtAdopting, o.Path))
		}
	}
	byTarget := map[canon.Target][]canon.Output{}
	for _, o := range outputs {
		byTarget[o.Target] = append(byTarget[o.Target], o)
	}
	for _, t := range buildTargets {
		group := byTarget[t]
		if len(group) == 0 {
			continue
		}
		writeLine(inv.env.Stdout, string(t)+targetHeaderSuffix)
		for _, o := range group {
			writeLine(inv.env.Stdout, outputIndent+stalePrefix(o.Status)+o.Path)
		}
	}
	if len(locks) == 0 {
		return nil
	}
	writeLine(inv.env.Stdout, lockHeader)
	prefix := lockPrefix(inv.opt.checkFlag)
	for _, l := range locks {
		writeLine(inv.env.Stdout, outputIndent+prefix+l.File)
	}
	return nil
}

// stalePrefix marks a would-be output change under --check (IMPLEMENTATION-PLAN.md §8.5, DECISIONS 201).
func stalePrefix(s canon.OutputStatus) string {
	if s == canon.OutputStale {
		return stalePathPrefix
	}
	return ""
}

// lockPrefix marks a would-be lock change under --check: every lock a --check build lists is prospective, since nothing is written (DECISIONS 201; meta/decisions/log-2026-09-24.md "Chosen while resuming").
func lockPrefix(check bool) string {
	if check {
		return stalePathPrefix
	}
	return ""
}

// countsWritten reports an output status the report counts as written, an adopted one included (rule B2, DECISIONS 201).
func countsWritten(s canon.OutputStatus) bool {
	return s == canon.OutputWritten || s == canon.OutputAdopted
}

// countChanges counts every output and lock the build changed, or, under --check, would change: canon build's written/stale summary, lock changes counted (DECISIONS 201; meta/decisions/log-2026-09-24.md "Chosen while resuming").
func countChanges(res *canon.BuildResult, check bool) (written, stale int) {
	for _, o := range res.Outputs {
		switch {
		case o.Status == canon.OutputStale:
			stale++
		case countsWritten(o.Status):
			written++
		}
	}
	if check {
		stale += len(res.Lock)
	} else {
		written += len(res.Lock)
	}
	return written, stale
}

// writeBuildJSON is canon build's own JSON report: every finding, then, unless -q, every output and appended lock line, then the summary (IMPLEMENTATION-PLAN.md §8.1, DECISIONS 201).
func (inv *invocation) writeBuildJSON(findings []canon.Finding, res *canon.BuildResult) error {
	for _, f := range findings {
		if err := inv.writeJSONLine(f); err != nil {
			return err
		}
	}
	if !inv.opt.quiet {
		if err := inv.writeBuildJSONLines(res); err != nil {
			return err
		}
	}
	written, stale := countChanges(res, inv.opt.checkFlag)
	return inv.writeJSONLine(newBuildSummary(res.Check, written, stale))
}

// writeBuildJSONLines prints every output, then every appended lock line.
func (inv *invocation) writeBuildJSONLines(res *canon.BuildResult) error {
	for _, o := range res.Outputs {
		if err := inv.writeJSONLine(newOutputLine(o)); err != nil {
			return err
		}
	}
	for _, l := range res.Lock {
		for _, line := range l.Lines {
			if err := inv.writeJSONLine(newLockLine(l, line)); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeJSONLine writes v as one JSON line, `<`, `>` and `&` unescaped as the finding writer leaves them (API.md F5).
func (inv *invocation) writeJSONLine(v any) error {
	enc := json.NewEncoder(inv.env.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return nil
}
