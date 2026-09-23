package project

import (
	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

type Lane struct{}

func New() *Lane { return &Lane{} }

func (*Lane) Name() string { return laneName }

func (*Lane) Rules() []string { return ruleIDs }

func (*Lane) Run(ctx *lane.Context) (lane.Result, error) {
	var out []finding.Finding
	if ctx.On(ruleRootClutter) {
		out = append(out, rootClutter(ctx)...)
	}
	if ctx.On(ruleDeadLink) {
		out = append(out, deadLinks(ctx)...)
	}
	return lane.Result{Findings: out}, nil
}
