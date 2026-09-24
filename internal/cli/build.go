package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	canon "github.com/fantasim/canonlang/api"
)

// runBuild is `canon build [packages…]` (CLI.md §3.4): check, then write outputs and locks.
func runBuild(inv *invocation) int {
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	selectors := inv.selectors(p.Root())
	res, err := p.Build(inv.ctx, canon.BuildOptions{
		Packages: selectors, Targets: inv.opt.targets, Check: inv.opt.checkFlag, Adopt: inv.opt.adopt,
	})
	if errors.Is(err, canon.ErrUnknownPackage) {
		err = inv.asTyped(p, selectors, err)
	}
	if err != nil {
		return inv.fail(err)
	}
	if err := inv.writeBuild(res); err != nil {
		return inv.fail(err)
	}
	switch {
	case res.Check.Summary.Errors > 0, res.Stale:
		return exitErrors
	case inv.opt.maxWarnings != unlimited && res.Check.Summary.Warnings > inv.opt.maxWarnings:
		return exitWarnings
	}
	return exitOK
}

// writeBuild prints the check's findings, then the build's own report (CLI.md §3.4).
func (inv *invocation) writeBuild(res *canon.BuildResult) error {
	shown := res.Check.Findings
	if inv.opt.quiet {
		shown = slices.DeleteFunc(slices.Clone(shown), func(f canon.Finding) bool { return f.Severity != canon.SeverityError })
	}
	changed := changedOutputs(res.Outputs)
	if inv.opt.format == formatJSON {
		return inv.writeBuildJSON(shown, changed, res)
	}
	if err := inv.writeFindings(shown, res.Check.Summary, res.Check.Duration); err != nil {
		return err
	}
	return inv.writeBuildText(changed, res.Lock)
}

// changedOutputs drops an output a build left untouched (CLI.md §8.5).
func changedOutputs(outputs []canon.Output) []canon.Output {
	out := make([]canon.Output, 0, len(outputs))
	for _, o := range outputs {
		if o.Status != canon.OutputUnchanged {
			out = append(out, o)
		}
	}
	return out
}

// writeBuildText is canon build's own text report (CLI.md §3.4).
func (inv *invocation) writeBuildText(outputs []canon.Output, locks []canon.LockChange) error {
	for _, o := range outputs {
		if o.Status == canon.OutputAdopted {
			writeLine(inv.env.Stdout, fmt.Sprintf(fmtAdopting, o.Path))
		}
	}
	byTarget := map[canon.Target][]string{}
	for _, o := range outputs {
		byTarget[o.Target] = append(byTarget[o.Target], o.Path)
	}
	for _, t := range buildTargets {
		paths := byTarget[t]
		if len(paths) == 0 {
			continue
		}
		writeLine(inv.env.Stdout, string(t)+targetHeaderSuffix)
		for _, path := range paths {
			writeLine(inv.env.Stdout, outputIndent+path)
		}
	}
	if len(locks) == 0 {
		return nil
	}
	writeLine(inv.env.Stdout, lockHeader)
	for _, l := range locks {
		if len(l.Lines) > 0 {
			writeLine(inv.env.Stdout, outputIndent+l.File)
		}
	}
	return nil
}

// writeBuildJSON is canon build's own JSON report (IMPLEMENTATION-PLAN.md §8.1).
func (inv *invocation) writeBuildJSON(findings []canon.Finding, outputs []canon.Output, res *canon.BuildResult) error {
	for _, f := range findings {
		if err := inv.writeJSONLine(f); err != nil {
			return err
		}
	}
	written, stale := 0, 0
	for _, o := range outputs {
		if o.Status == canon.OutputStale {
			stale++
		} else {
			written++
		}
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
	return inv.writeJSONLine(newBuildSummary(res.Check, written, stale))
}

func (inv *invocation) writeJSONLine(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	writeLine(inv.env.Stdout, string(data))
	return nil
}
