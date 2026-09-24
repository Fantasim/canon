package integrity

import (
	"fmt"
	"slices"
	"strings"

	fpkg "github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/ratchet"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

type finding = fpkg.Finding

type Lane struct{}

func New() *Lane { return &Lane{} }

func (*Lane) Name() string { return laneName }

func (*Lane) Rules() []string { return ruleIDs }

// check is one rule group of this lane: run returns its findings, or the error that leaves it
// unmeasured. A git check compares against a revision: outside a checkout there is nothing to
// compare, which is no failure (README.md).
type check struct {
	rules []string
	skip  string
	git   bool
	run   func(*lane.Context) ([]finding, error)
}

var checks = []check{
	{rules: []string{ruleBaselineGuard}, skip: skipGuard, git: true, run: guardFindings},
	{rules: []string{ruleDecision}, skip: skipDecision, git: true, run: decisionsDropped},
	{rules: []string{ruleState}, skip: skipState, run: badStates},
	{rules: []string{ruleReason, ruleCount}, skip: skipIgnores, run: ignoreFindings},
}

func (*Lane) Run(ctx *lane.Context) (lane.Result, error) {
	var res lane.Result
	inGit := inGitCheckout(ctx)
	for _, c := range checks {
		if !slices.ContainsFunc(c.rules, ctx.On) || (c.git && !inGit) {
			continue
		}
		fs, err := c.run(ctx)
		if err != nil {
			res.Skipped = append(res.Skipped, lane.Skip{What: c.skip, Reason: err.Error()})
			continue
		}
		res.Findings = append(res.Findings, fs...)
	}
	return res, nil
}

// inGitCheckout tells a real git failure from simply being outside a checkout.
func inGitCheckout(ctx *lane.Context) bool {
	out, err := ctx.Repo.Git(gitRevParse, gitInsideWorkTree)
	return err == nil && strings.TrimSpace(out) == gitTrue
}

func ignoreFindings(ctx *lane.Context) ([]finding, error) { return ignores(ctx), nil }

// guardFindings is baseline-guard: the baseline, the states and the thresholds, each against
// the guarded revision.
func guardFindings(ctx *lane.Context) ([]finding, error) {
	var out []finding
	for _, guard := range []func(*lane.Context) ([]finding, error){baselineLoosened, statesLoosened, thresholdsRaised} {
		fs, err := guard(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, fs...)
	}
	return out, nil
}

// headText is a file's content at the guarded revision; ok is false when that revision lacks
// the file (ls-tree lists nothing there), while any git failure is an error, never "absent".
// The path is read relative to the repo root given, which may sit below git's top level.
func headText(ctx *lane.Context, rel string) (string, bool, error) {
	rev, err := guardRev(ctx.Repo.Git, ctx.Repo.Base())
	if err != nil {
		return "", false, err
	}
	listed, err := ctx.Repo.Git(gitLsTree, rev, gitPathSep, rel)
	if err != nil {
		return "", false, fmt.Errorf(fmtGitErr, gitLsTree, rev, err)
	}
	if strings.TrimSpace(listed) == "" {
		return "", false, nil
	}
	text, err := ctx.Repo.Git(gitShow, rev+gitRevPathHere+rel)
	if err != nil {
		return "", false, fmt.Errorf(fmtGitErr, gitShow, rev, err)
	}
	return text, true, nil
}

// guardRev is base, or HEAD when every commit since base that touched the ratchet touched
// only ratchet files: a committed re-record stands on its own in history, visible to review.
func guardRev(git func(...string) (string, error), base string) (string, error) {
	out, err := git(gitLog, gitHashFormat, base+gitRange+repo.DefaultBase, gitPathSep, repo.AuditDir)
	if err != nil {
		return "", fmt.Errorf(fmtGitErr, gitLog, base, err)
	}
	if strings.TrimSpace(out) == "" {
		return base, nil
	}
	for h := range strings.FieldsSeq(out) {
		files, err := git(gitShow, repo.GitNameOnly, gitNoFormat, h)
		if err != nil {
			return "", fmt.Errorf(fmtGitErr, gitShow, h, err)
		}
		if !onlyRatchetFiles(files) {
			return base, nil
		}
	}
	return repo.DefaultBase, nil
}

func onlyRatchetFiles(files string) bool {
	for f := range strings.FieldsSeq(files) {
		if !strings.HasPrefix(f, auditSegment) && !strings.Contains(f, "/"+auditSegment) {
			return false
		}
	}
	return true
}

func baselineLoosened(ctx *lane.Context) ([]finding, error) {
	old, ok, err := headText(ctx, repo.BaselineFile)
	if err != nil || !ok {
		return nil, err
	}
	cur, has, err := ratchet.Load(ctx.Repo.Abs(repo.BaselineFile))
	if err != nil || !has {
		return nil, err
	}
	prev := ratchet.Parse(strings.NewReader(old))
	var out []finding
	for k, e := range cur {
		p, seen := prev[k]
		if seen && e.Count <= p.Count && e.Value <= p.Value {
			continue
		}
		out = append(out, finding{
			Rule: ruleBaselineGuard, File: repo.BaselineFile, Detail: e.Rule + " " + e.File + " " + e.Symbol,
			Message: msgBaselineUp + e.Rule + " at " + e.File,
		})
	}
	return out, nil
}
