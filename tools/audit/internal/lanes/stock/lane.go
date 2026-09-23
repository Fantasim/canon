package stock

import (
	"path/filepath"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

type Lane struct{}

func New() *Lane { return &Lane{} }

func (*Lane) Name() string { return laneName }

func (*Lane) Rules() []string { return ruleIDs }

// Run drives golangci-lint (linters + formatters) and x/tools/deadcode in sequence, and
// maps their issues onto the rulebook ids.
func (*Lane) Run(ctx *lane.Context) (lane.Result, error) {
	if ctx.Go == nil {
		return lane.Result{}, nil
	}
	cfgPath := filepath.Join(ctx.Toolchain, golangciConfig)

	var (
		findings []finding.Finding
		skips    []lane.Skip
	)
	fs, skip := runGolangci(ctx, cfgPath)
	collect(&findings, &skips, fs, skip)
	fs, skip = runDeadcode(ctx)
	collect(&findings, &skips, fs, skip)
	return lane.Result{Findings: findings, Skipped: skips}, nil
}

func collect(findings *[]finding.Finding, skips *[]lane.Skip, fs []finding.Finding, skip *lane.Skip) {
	*findings = append(*findings, fs...)
	if skip != nil {
		*skips = append(*skips, *skip)
	}
}
