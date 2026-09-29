package cli

import (
	"cmp"
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

// writeCycleText is a cycle in text: the header, the new findings, one `fixed:` line per finding
// gone, the summary, and for build its report of the outputs it changed; a blank line sets the
// findings and the summary apart as API.md F14 does.
func (inv *invocation) writeCycleText(ev canon.Event, added, fixed []canon.Finding, cur *cycleState) error {
	out := inv.env.Stdout
	writeLine(out, fmt.Sprintf(fmtWatchHead, len(ev.Files), len(ev.Packages)))
	if len(added) > 0 {
		if err := inv.writeFindingsOnly(added); err != nil {
			return err
		}
	}
	if len(fixed) > 0 && len(added) > 0 {
		writeLine(out, watchBlank)
	}
	for _, f := range fixed {
		writeLine(out, fixedLine(f))
	}
	if len(added) > 0 || len(fixed) > 0 {
		writeLine(out, watchBlank)
	}
	if err := inv.writeFindings(nil, cur.check.Summary, cur.check.Duration); err != nil {
		return err
	}
	if cur.build == nil || inv.opt.quiet {
		return nil
	}
	return inv.writeBuildText(changedOutputs(cur.build.Outputs), cur.build.Lock)
}

// fixedLine is `fixed: <severity>[<CODE>]  <file:line:col>` for a finding that disappeared; a
// finding without a file has no location.
func fixedLine(f canon.Finding) string {
	line := fmt.Sprintf(fmtWatchFixed, f.Severity, f.Code)
	if f.File == "" {
		return line
	}
	return line + outputIndent + fmt.Sprintf(fmtWatchAt, f.File, f.Line, f.Col)
}

// cycleLine is a cycle's first JSON line (IMPLEMENTATION-PLAN.md §8.1).
type cycleLine struct {
	Cycle struct {
		Revision canon.Revision `json:"revision"`
		Files    []string       `json:"files"`
		Packages []string       `json:"packages"`
		Settled  *bool          `json:"settled,omitempty"`
	} `json:"cycle"`
}

// newCycleLine is the cycle line of ev, naming the revision of the result the cycle shows (meta/decisions/log-2026-09-29.md "M4 U6-r").
func newCycleLine(ev canon.Event, rev canon.Revision) cycleLine {
	var l cycleLine
	l.Cycle.Revision = rev
	l.Cycle.Files, l.Cycle.Packages = nonNil(ev.Files), nonNil(ev.Packages)
	return l
}

// writeUnsettled says that builds answering each other's writes stopped: text one line, JSON the cycle line with `settled` false and the summary of the last build (meta/decisions/log-2026-09-29.md "M4 U6-r").
func (inv *invocation) writeUnsettled(ev canon.Event, last *cycleState) error {
	if inv.opt.format != formatJSON {
		writeLine(inv.env.Stdout, watchUnsettled)
		return nil
	}
	line := newCycleLine(ev, ev.Revision)
	settled := false
	line.Cycle.Settled = &settled
	if err := inv.writeJSONLine(line); err != nil {
		return err
	}
	return inv.writeSummaryJSON(last)
}

// changedFinding is a finding's JSON line with the `change` it went through, written last.
type changedFinding struct {
	finding canon.Finding
	change  string
}

// MarshalJSON is the finding's own line (API.md F5) with `"change":"added"` or `"removed"` after
// its last member.
func (c changedFinding) MarshalJSON() ([]byte, error) {
	b, err := c.finding.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	return append(b[:len(b)-1], fmt.Sprintf(fmtWatchChange, c.change)...), nil
}

// writeCycleJSON is a cycle in JSON lines: the cycle, each changed finding, for build its outputs and locks, then the summary (IMPLEMENTATION-PLAN.md §8.1).
func (inv *invocation) writeCycleJSON(ev canon.Event, added, fixed []canon.Finding, cur *cycleState) error {
	head := newCycleLine(ev, cmp.Or(cur.check.Revision, ev.Revision))
	if err := inv.writeJSONLine(head); err != nil {
		return err
	}
	for _, f := range added {
		if err := inv.writeJSONLine(changedFinding{finding: f, change: watchAdded}); err != nil {
			return err
		}
	}
	for _, f := range fixed {
		if err := inv.writeJSONLine(changedFinding{finding: f, change: watchRemoved}); err != nil {
			return err
		}
	}
	if cur.build != nil && !inv.opt.quiet {
		if err := inv.writeBuildJSONLines(cur.build); err != nil {
			return err
		}
	}
	return inv.writeSummaryJSON(cur)
}

// writeSummaryJSON ends a cycle: check's summary line, or build's with what it wrote.
func (inv *invocation) writeSummaryJSON(cur *cycleState) error {
	if cur.build == nil {
		return inv.writeFindings(nil, cur.check.Summary, cur.check.Duration)
	}
	written, stale := countChanges(cur.build, inv.opt.checkFlag)
	return inv.writeJSONLine(newBuildSummary(cur.check, written, stale))
}

// nonNil is s, or an empty list where it is nil: a JSON line writes `[]`, never `null`.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
