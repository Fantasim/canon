package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/lanes"
	"github.com/fantasim/canonlang/tools/audit/internal/ratchet"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/report"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

type opts struct {
	repo, toolchain, thresholds, rule, family, laneName, base string
	raw, quiet, changed, init, tighten                        bool
	perRule                                                   int
}

// printer writes through w and keeps the first error, mirroring package report's own: a
// write failure on the tool's real output becomes a non-zero exit, not a silent drop.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) f(format string, a ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, a...)
}

// warnln writes a diagnostic to stderr; a failure there is not fatal, so the error is
// explicitly discarded rather than silently ignored.
func warnln(w io.Writer, a ...any) { _, _ = fmt.Fprintln(w, a...) }

func run(args []string, out, errw io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		p := &printer{w: out}
		p.f("%s", usage)
		return exitFor(p.err)
	}
	cmd, o, err := parse(args, errw)
	if err != nil {
		warnln(errw, errPrefix, err)
		return exitUsage
	}
	limits, err := threshold.Load(o.thresholds)
	if err != nil {
		warnln(errw, errPrefix, err)
		return exitUsage
	}
	if cmd == cmdRules {
		return exitFor(report.PrintRules(out, o.family, limits))
	}
	s, err := setup(o, limits, errw)
	if err != nil {
		warnln(errw, errPrefix, err)
		return exitUsage
	}
	switch cmd {
	case cmdAudit:
		return s.audit(out)
	case cmdCheck:
		return s.check(out, errw)
	case cmdBaseline:
		return s.baseline(out, errw)
	}
	return exitUsage
}

// exitFor turns a report write error into a non-zero exit; nil is success.
func exitFor(err error) int {
	if err != nil {
		return exitFail
	}
	return exitOK
}

func parse(args []string, errw io.Writer) (string, opts, error) {
	cmd := args[0]
	if !slices.Contains([]string{cmdAudit, cmdCheck, cmdBaseline, cmdRules}, cmd) {
		return "", opts{}, fmt.Errorf("%w: %q (audit | check | baseline | rules)", errUnknownCommand, cmd)
	}
	var o opts
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(errw)
	fs.StringVar(&o.repo, "repo", ".", "repository to audit (any directory inside it)")
	fs.StringVar(&o.toolchain, "toolchain", "", "tools/audit/toolchain (default: next to this source)")
	fs.StringVar(&o.thresholds, "thresholds", besideSource(threshold.FileName), "the rule thresholds file")
	fs.StringVar(&o.rule, "rule", "", "only this rule, every finding")
	fs.StringVar(&o.family, "family", "", "only this family")
	fs.StringVar(&o.laneName, "lane", "", "only the rules of this lane")
	fs.StringVar(&o.base, "base", repo.DefaultBase, "check --changed: the revision to diff against")
	fs.BoolVar(&o.raw, "raw", false, "one tab-separated line per finding")
	fs.BoolVar(&o.quiet, "quiet", false, "no progress lines on stderr")
	fs.BoolVar(&o.changed, "changed", false, "check: judge only files changed against --base, plus untracked")
	fs.BoolVar(&o.init, "init", false, "baseline: write the first baseline (refuses when one exists)")
	fs.BoolVar(&o.tighten, "tighten", false, "baseline: lower the baseline to the tree; never raises it")
	fs.IntVar(&o.perRule, "n", defaultPerRule, "audit: findings shown per rule")
	if err := fs.Parse(args[1:]); err != nil {
		return "", o, fmt.Errorf("parse flags: %w", err)
	}
	if o.rule != "" {
		if _, ok := rules.Lookup(o.rule); !ok {
			return "", o, fmt.Errorf("%w: %q (see the rules command)", errUnknownRule, o.rule)
		}
	}
	return cmd, o, nil
}

type session struct {
	o   opts
	ctx *lane.Context
	all []lane.Lane
}

func setup(o opts, limits threshold.Set, errw io.Writer) (*session, error) {
	debug.SetMemoryLimit(lane.SelfMemLimit)
	if err := lane.Exclusive(func(f string, a ...any) { warnln(errw, fmt.Sprintf(lane.LogPrefix+f, a...)) }); err != nil {
		return nil, err
	}
	r, err := repo.Open(o.repo)
	if err != nil {
		return nil, err
	}
	s := &session{o: o, all: lanes.All()}
	s.ctx = &lane.Context{Repo: r, Toolchain: toolchainDir(o.toolchain), Limits: limits, LimitsFile: absPath(o.thresholds)}
	if !o.quiet {
		s.ctx.Log = errw
	}
	if o.changed {
		if err := r.SetChanged(o.base); err != nil {
			return nil, err
		}
	}
	if gofiles := r.FilesWithExt(repo.GoExt); len(gofiles) > 0 {
		tree, perrs := gosrc.Parse(r.Root, gofiles)
		for _, e := range perrs {
			s.ctx.Logf("parse: %v", e)
		}
		s.ctx.Go = tree
	}
	s.ctx.Enabled = s.enabled()
	return s, nil
}

func (s *session) enabled() map[string]bool {
	var laneRules []string
	for _, l := range s.all {
		if l.Name() == s.o.laneName {
			laneRules = l.Rules()
		}
	}
	on := map[string]bool{}
	for _, rl := range rules.All {
		switch {
		case s.ctx.Repo.Mode(rl) == rules.Off,
			s.o.rule != "" && rl.ID != s.o.rule,
			s.o.family != "" && rl.Family != s.o.family,
			s.o.laneName != "" && !slices.Contains(laneRules, rl.ID):
			continue
		}
		on[rl.ID] = true
	}
	return on
}

func (s *session) findings() ([]finding.Finding, []lane.Skip) { return lane.Run(s.ctx, s.all) }

func (s *session) mode(id string) rules.Mode {
	rl, ok := rules.Lookup(id)
	if !ok {
		return rules.Off
	}
	return s.ctx.Repo.Mode(*rl)
}

func (s *session) audit(out io.Writer) int {
	fs, skips := s.findings()
	if s.o.raw {
		return exitFor(report.PrintRaw(out, fs))
	}
	base, has, err := ratchet.Load(s.ctx.Repo.Abs(repo.BaselineFile))
	if err != nil {
		s.ctx.Logf("baseline: %v", err)
	}
	c := report.Census{
		Repo: s.ctx.Repo, Findings: fs, Skipped: skips, Enabled: s.ctx.Enabled,
		Baseline: base, HasBase: has, PerRule: s.o.perRule, Only: s.o.rule, Limits: s.ctx.Limits,
	}
	if has {
		c.Verdict = ratchet.Compare(fs, base, s.mode, s.ctx.Repo.InScope)
	}
	return exitFor(report.PrintCensus(out, c))
}

func (s *session) check(out, errw io.Writer) int {
	base, has, err := ratchet.Load(s.ctx.Repo.Abs(repo.BaselineFile))
	if err != nil {
		warnln(errw, errPrefix, "baseline:", err)
		return exitUsage
	}
	if !has {
		warnln(errw, errPrefix, "no "+repo.BaselineFile+": every finding counts as new (baseline --init)")
	}
	fs, skips := s.findings()
	v := ratchet.Compare(fs, base, s.mode, s.ctx.Repo.InScope)
	if err := report.PrintCheck(out, s.ctx.Repo, v, skips); err != nil {
		return exitFail
	}
	if v.Failed() {
		return exitFail
	}
	return exitOK
}

func (s *session) baseline(out, errw io.Writer) int {
	path := s.ctx.Repo.Abs(repo.BaselineFile)
	base, has, err := ratchet.Load(path)
	if err != nil {
		warnln(errw, errPrefix, err)
		return exitUsage
	}
	switch {
	case s.o.init && has:
		warnln(errw, errPrefix, "a baseline exists; it only ever shrinks (--tighten). Deleting it is a maintainer's call.")
		return exitUsage
	case s.o.init:
		return s.initBaseline(out, errw, path)
	case s.o.tighten && !has:
		warnln(errw, errPrefix, "no baseline to tighten (--init first)")
		return exitUsage
	case s.o.tighten:
		return s.tightenBaseline(out, errw, path, base)
	}
	warnln(errw, errPrefix, "baseline needs --init or --tighten")
	return exitUsage
}

func (s *session) tightenBaseline(out, errw io.Writer, path string, base ratchet.Baseline) int {
	fs, _ := s.findings()
	nb, n := ratchet.Tighten(base, fs, s.ctx.Repo.InScope)
	if err := nb.Save(path); err != nil {
		warnln(errw, errPrefix, err)
		return exitUsage
	}
	p := &printer{w: out}
	p.f("%s baseline: tightened %d entries, %d remain\n", rules.ToolName, n, len(nb))
	return exitFor(p.err)
}

// initBaseline records every ratchet finding, and demotes to ratchet (in this repo's
// state.tsv) each enforce rule that already has findings, so a first init is never red.
func (s *session) initBaseline(out, errw io.Writer, path string) int {
	if s.o.rule != "" || s.o.family != "" || s.o.laneName != "" || s.o.changed {
		warnln(errw, errPrefix, "--init records the whole repo; drop --rule/--family/--lane/--changed")
		return exitUsage
	}
	fs, skips := s.findings()
	states := s.ctx.Repo.States()
	p := &printer{w: out}
	keep := s.demoteAndKeep(fs, states, p)
	b := ratchet.Build(keep)
	if err := errors.Join(b.Save(path), repo.WriteStates(s.ctx.Repo.Abs(repo.StateFile), states)); err != nil {
		warnln(errw, errPrefix, err)
		return exitUsage
	}
	p.f("%s baseline: %d findings in %d entries -> %s\n", rules.ToolName, len(keep), len(b), repo.BaselineFile)
	for _, k := range skips {
		p.f("  not measured, so not baselined: %s: %s\n", k.What, k.Reason)
	}
	return exitFor(p.err)
}

// demoteAndKeep keeps every ratchet finding, demotes (and reports) an enforce rule that
// already has findings, and drops findings of a rule that neither gates nor ratchets.
func (s *session) demoteAndKeep(fs []finding.Finding, states map[string]rules.Mode, p *printer) []finding.Finding {
	var keep []finding.Finding
	for _, f := range fs {
		switch s.mode(f.Rule) {
		case rules.Enforce:
			if _, ok := states[f.Rule]; !ok {
				p.f("demoted to ratchet in this repo (has findings): %s\n", f.Rule)
			}
			states[f.Rule] = rules.Ratchet
			keep = append(keep, f)
		case rules.Ratchet:
			keep = append(keep, f)
		case rules.Observe, rules.Off:
			// neither baselined nor gated
		}
	}
	return keep
}

// toolchainDir finds tools/audit/toolchain: the flag, else next to this source file (true
// under `go run`).
func toolchainDir(flagVal string) string {
	if flagVal != "" {
		return absPath(flagVal)
	}
	d := besideSource(toolchainName)
	if _, err := os.Stat(d); err != nil {
		return ""
	}
	return d
}

// besideSource is name in this source file's directory, where `go run` keeps it.
func besideSource(name string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return name
	}
	return filepath.Join(filepath.Dir(file), name)
}

func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
