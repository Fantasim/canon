package control

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// dependentControl is one control per branch of a `match`-bodied type function, in branch
// order, and the driver that picks it (C16, D6); an application of any other type function is
// its expansion (J11).
func (r *Resolver) dependentControl(c at, t types.Type) vm.Control {
	app := t.Base().(*types.TypeAppType)
	fn := app.Fn
	if !Matches(fn) {
		if fn.Body == nil {
			return vm.Control{Kind: ctlText}
		}
		return r.control(c, shape.Expand(app), "", false)
	}
	ctl := vm.Control{Kind: ctlDependent, Fn: encode.FuncName(fn), On: encode.Driver(Scrutinized(app))}
	for _, arm := range fn.Arms {
		ctl.Branches = append(ctl.Branches, r.control(inner(c), shape.Substitute(arm.Result, fn.Params, app.Args), "", false))
	}
	return ctl
}

// Matches reports a type function whose body is a type-level `match` (TYPES.md 11.2): it is
// kept as a typeFunction definition (J11).
func Matches(fn *types.TypeFunc) bool {
	return len(fn.Arms) > 0 && fn.Scrutinee != nil && fn.Scrutinee.Param != nil
}

// Scrutinized is the argument app passes to the parameter its function's `match` reads, nil
// for a function without one.
func Scrutinized(app *types.TypeAppType) *types.Arg {
	sc := app.Fn.Scrutinee
	if sc == nil || sc.Param == nil || sc.Param.Index >= len(app.Args) {
		return nil
	}
	return app.Args[sc.Param.Index]
}
