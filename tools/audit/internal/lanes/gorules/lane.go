package gorules

import (
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

type Lane struct{}

func New() *Lane { return &Lane{} }

func (*Lane) Name() string { return laneName }

func (*Lane) Rules() []string { return ruleIDs }

// check is one syntax-only rule and the scan method that produces it.
type check struct {
	rule string
	run  func(*scan)
}

var checks = []check{
	{ruleMagicString, (*scan).magicStrings},
	{ruleMagicNumber, (*scan).magicNumbers},
	{ruleConstPlace, (*scan).constPlacement},
	{ruleConstDup, (*scan).constDup},
	{ruleEnvKey, (*scan).envKey},
	{ruleErrInline, (*scan).errInline},
	{ruleErrPlace, (*scan).errPlacement},
	{ruleLogDirect, (*scan).logDirect},
	{ruleBareGo, (*scan).bareGoroutine},
	{ruleStdlib, (*scan).stdlib},
}

func (*Lane) Run(ctx *lane.Context) (lane.Result, error) {
	var res lane.Result
	if ctx.Go == nil {
		return res, nil
	}
	s := newScan(ctx)
	for _, c := range checks {
		if ctx.On(c.rule) {
			c.run(s)
		}
	}
	if ctx.On(ruleImportBound) {
		res.Skipped = append(res.Skipped, s.importBoundary()...)
	}
	if ctx.On(ruleExportedLocal) {
		if sk := s.exportedLocal(); sk != nil {
			res.Skipped = append(res.Skipped, *sk)
		}
	}
	res.Findings = s.out
	return res, nil
}
