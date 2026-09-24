package cli

import canon "github.com/fantasim/canonlang/api"

// outputLine is one JSON line reporting a changed output (IMPLEMENTATION-PLAN.md §8.1).
type outputLine struct {
	Output struct {
		Path    string `json:"path"`
		Target  string `json:"target"`
		Package string `json:"package"`
		Status  string `json:"status"`
	} `json:"output"`
}

func newOutputLine(o canon.Output) outputLine {
	var l outputLine
	l.Output.Path, l.Output.Target = o.Path, string(o.Target)
	l.Output.Package, l.Output.Status = o.Package, string(o.Status)
	return l
}

// lockLine is one appended canon.lock line (IMPLEMENTATION-PLAN.md §8.1).
type lockLine struct {
	Lock struct {
		Package string `json:"package"`
		File    string `json:"file"`
		Line    string `json:"line"`
	} `json:"lock"`
}

func newLockLine(l canon.LockChange, line string) lockLine {
	var out lockLine
	out.Lock.Package, out.Lock.File, out.Lock.Line = l.Package, l.File, line
	return out
}

// buildSummary is canon build's final JSON summary line (IMPLEMENTATION-PLAN.md §8.1).
type buildSummary struct {
	Summary struct {
		Errors   int `json:"errors"`
		Warnings int `json:"warnings"`
		Packages int `json:"packages"`
		Ms       int `json:"ms"`
		Written  int `json:"written"`
		Stale    int `json:"stale"`
	} `json:"summary"`
}

func newBuildSummary(check *canon.CheckResult, written, stale int) buildSummary {
	var s buildSummary
	s.Summary.Errors, s.Summary.Warnings, s.Summary.Packages = check.Summary.Errors, check.Summary.Warnings, check.Summary.Packages
	s.Summary.Ms = int(check.Duration.Milliseconds())
	s.Summary.Written, s.Summary.Stale = written, stale
	return s
}
