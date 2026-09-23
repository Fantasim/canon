package gostyle

import (
	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

type Lane struct{}

func New() *Lane { return &Lane{} }

func (*Lane) Name() string { return laneName }

func (*Lane) Rules() []string { return ruleIDs }

func (*Lane) Run(ctx *lane.Context) (lane.Result, error) {
	if ctx.Go == nil {
		return lane.Result{}, nil
	}
	var fs []finding.Finding
	fs = append(fs, sizeFindings(ctx)...)
	fs = append(fs, commentFindings(ctx)...)
	fs = append(fs, pkgDocFindings(ctx)...)
	return lane.Result{Findings: fs}, nil
}
