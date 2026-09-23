package integrity

import (
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

func (*Lane) Run(ctx *lane.Context) (lane.Result, error) {
	var res lane.Result
	if ctx.On(ruleBaselineGuard) {
		res.Findings = append(res.Findings, baselineLoosened(ctx)...)
		res.Findings = append(res.Findings, statesLoosened(ctx)...)
		res.Findings = append(res.Findings, thresholdsRaised(ctx)...)
	}
	if ctx.On(ruleDecision) {
		res.Findings = append(res.Findings, decisionsDropped(ctx)...)
	}
	if ctx.On(ruleState) {
		res.Findings = append(res.Findings, badStates(ctx)...)
	}
	if ctx.On(ruleReason) || ctx.On(ruleCount) {
		res.Findings = append(res.Findings, ignores(ctx)...)
	}
	return res, nil
}

// headText is a file's content at the guarded revision; ok is false when it lacks the file.
// The path is read relative to the repo root given, which may sit below git's top level.
func headText(ctx *lane.Context, rel string) (string, bool) {
	out, err := ctx.Repo.Git(gitShow, guardRev(ctx.Repo.Git, ctx.Repo.Base())+gitRevPathHere+rel)
	return out, err == nil
}

// guardRev is base, or HEAD when every commit since base that touched the ratchet touched
// only ratchet files: a committed re-record stands on its own in history, visible to review.
func guardRev(git func(...string) (string, error), base string) string {
	out, err := git(gitLog, gitHashFormat, base+gitRange+repo.DefaultBase, gitPathSep, repo.AuditDir)
	if err != nil || strings.TrimSpace(out) == "" {
		return base
	}
	for h := range strings.FieldsSeq(out) {
		files, err := git(gitShow, repo.GitNameOnly, gitNoFormat, h)
		if err != nil || !onlyRatchetFiles(files) {
			return base
		}
	}
	return repo.DefaultBase
}

func onlyRatchetFiles(files string) bool {
	for f := range strings.FieldsSeq(files) {
		if !strings.HasPrefix(f, auditSegment) && !strings.Contains(f, "/"+auditSegment) {
			return false
		}
	}
	return true
}

func baselineLoosened(ctx *lane.Context) []finding {
	old, ok := headText(ctx, repo.BaselineFile)
	if !ok {
		return nil
	}
	cur, has, err := ratchet.Load(ctx.Repo.Abs(repo.BaselineFile))
	if err != nil || !has {
		return nil
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
	return out
}
