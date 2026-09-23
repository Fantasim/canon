package determinism

import (
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

type Lane struct{}

func New() *Lane { return &Lane{} }

func (*Lane) Name() string { return laneName }

func (*Lane) Rules() []string { return ruleIDs }

// Run judges every hand-written Go file of the type-checked module, tests included; a module
// that does not type-check skips the rule (go vet in make check has already failed).
func (*Lane) Run(ctx *lane.Context) (lane.Result, error) {
	if ctx.Go == nil || !ctx.On(ruleMapRange) {
		return lane.Result{}, nil
	}
	return judge(ctx), nil
}

func judge(ctx *lane.Context) lane.Result {
	pkgs, err := ctx.Go.TypedClean()
	if err != nil {
		return lane.Result{Skipped: []lane.Skip{{What: ruleMapRange, Reason: err.Error()}}}
	}
	s := newScan(ctx)
	s.packages(pkgs)
	return lane.Result{Findings: s.out}
}
