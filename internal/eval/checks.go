package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// CheckRun is one run of one check: rules.Run's fields (DECISIONS 148, 186).
type CheckRun struct {
	Aborted bool          // a hard error, reported unless the run was tainted, or a poisoned read
	Failed  bool          // a one-line check: its condition is false
	Message string        // a one-line check: its message, or its template's text after a hard error in it
	Reports []CheckReport // a block check: its fail and warn calls, in order
}

// CheckReport is one fail(at, message) or warn(at, message) of a block check.
type CheckReport struct {
	Warn    bool
	At      value.Value
	Message string
}

// Run evaluates check c on self, nil at package level (EVALUATION.md §8, DECISIONS 148).
func (e *Evaluator) Run(ctx context.Context, c *syntax.CheckDecl, self value.Value) CheckRun {
	file := e.index.file[c]
	if file == nil || e.exhausted || e.brokenCheck(c) {
		return CheckRun{Aborted: true}
	}
	r := e.newRun(ctx, e.checkCharge(c, file), file)
	r.fr.self = self
	if c.Body != nil {
		var reports []CheckReport
		r.reports = &reports
		if r.block(c.Body) == flowAbort || r.failed {
			return CheckRun{Aborted: true}
		}
		return CheckRun{Reports: reports}
	}
	holds, ok := r.truth(c.Cond)
	if !ok {
		return CheckRun{Aborted: true}
	}
	if holds {
		return CheckRun{}
	}
	msg := r.eval(c.Message)
	if text, isStr := msg.(*value.Str); isStr && !r.failed {
		return CheckRun{Failed: true, Message: text.V}
	}
	return CheckRun{Failed: true, Message: templateText(file, c.Message)}
}

// brokenCheck reports a check which is broken (one naming a broken fn included), or whose record or variant is (TYPES.md §1).
func (e *Evaluator) brokenCheck(c *syntax.CheckDecl) bool {
	if e.info == nil || e.index.broken[c] {
		return true
	}
	owner := e.index.owner[c]
	if owner == nil {
		return e.broken(e.index.decls[c])
	}
	switch d := owner.(type) {
	case *syntax.RecordDecl:
		return e.broken(e.info.Defs[d.Name])
	case *syntax.VariantDecl:
		return e.broken(e.info.Defs[d.Name])
	}
	return false
}

// checkCharge names a check for the budget (EVALUATION.md §12.2, DECISIONS 185).
func (e *Evaluator) checkCharge(c *syntax.CheckDecl, file *syntax.File) charge {
	ch := charge{pkg: e.index.pkg[file], name: checkKeyword}
	switch d := e.index.owner[c].(type) {
	case *syntax.RecordDecl:
		ch.name = d.Name.Name
	case *syntax.VariantDecl:
		ch.name = d.Name.Name
	}
	if c.Name != nil {
		ch.name = c.Name.Name
	}
	return ch
}

// failWarn is fail(at, m) or warn(at, m): a report of the check run (EVALUATION.md §8.3).
func (r *run) failWarn(x *syntax.CallExpr, warn bool) value.Value {
	var at, msg value.Value
	for i, a := range x.Args {
		v := r.eval(a.Value)
		if v == nil {
			return nil
		}
		if i == 0 && a.Name == nil || a.Name != nil && a.Name.Name != messageParam {
			at = v
		} else {
			msg = v
		}
	}
	text, _ := msg.(*value.Str)
	if r.reports != nil && text != nil {
		*r.reports = append(*r.reports, CheckReport{Warn: warn, At: at, Message: text.V})
	}
	return &value.None{T: r.typeOf(x), P: r.prov(x, value.ProvComputed)}
}
