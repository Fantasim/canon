package diagnostics

import (
	"path"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

type Lane struct{}

func New() *Lane { return &Lane{} }

func (*Lane) Name() string { return laneName }

func (*Lane) Rules() []string { return ruleIDs }

// scan is one run: the Go files diag-message-inline judges, the catalogue, the findings.
type scan struct {
	ctx      *lane.Context
	cat      *catalogue
	files    []*gosrc.File
	diagPath string
	out      []finding.Finding
}

// Run checks literals and diag calls when Go code exists, and the catalogue's coverage when
// the repository has one.
func (*Lane) Run(ctx *lane.Context) (lane.Result, error) {
	var res lane.Result
	cat, err := readCatalogue(ctx.Repo.Abs(catalogueFile))
	if err != nil {
		res.Skipped = append(res.Skipped, lane.Skip{What: laneName, Reason: err.Error()})
	}
	s := &scan{ctx: ctx, cat: cat, diagPath: ctx.Repo.Module + pathSep + diagDir}
	if ctx.Go != nil {
		s.files = judgedFiles(ctx.Go)
	}
	if ctx.On(ruleInline) && ctx.Go != nil {
		s.literals()
		if sk := s.diagCalls(); sk != nil {
			res.Skipped = append(res.Skipped, *sk)
		}
	}
	if cat != nil && (ctx.On(ruleUntested) || ctx.On(ruleUnreported)) {
		s.coverage()
	}
	res.Findings = s.out
	return res, nil
}

// judgedFiles are the hand-written Go files outside the registry package, tests included.
func judgedFiles(tree *gosrc.Tree) []*gosrc.File {
	var out []*gosrc.File
	for _, f := range tree.Files {
		if !f.Generated && !inDiag(f.Dir) {
			out = append(out, f)
		}
	}
	return out
}

// inDiag reports the registry package or a package below it (its generator).
func inDiag(dir string) bool {
	return dir == diagDir || strings.HasPrefix(dir, diagDir+pathSep)
}

func (s *scan) emit(f finding.Finding) {
	if s.ctx.On(f.Rule) {
		s.out = append(s.out, f)
	}
}

// findingsDirOf is where a code's per-code tests live: its owning package's testdata, or any
// generator's for a code of the generators.
func findingsDirOf(pkg string) string {
	if pkg == genPackage {
		return internalDir + anyGenerator + findingsDir
	}
	return path.Clean(internalDir + pkg + findingsDir)
}
