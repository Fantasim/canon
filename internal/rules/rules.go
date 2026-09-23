package rules

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// Evaluator is what the check runner needs: Invalid (IMPLEMENTATION-PLAN §4.8), and a check run.
type Evaluator interface {
	// Invalid reports a value a conversion or verification marked invalid (EVALUATION.md §7.3).
	Invalid(v value.Value) bool
	// Run evaluates check c on self, an instance, or nil at package level. A check of a broken
	// record or case is aborted without a finding.
	Run(ctx context.Context, c *syntax.CheckDecl, self value.Value) Run
}

// Run is the outcome of one check run.
type Run struct {
	Aborted bool     // a hard error, which the evaluator reported unless tainted, or a poisoned read
	Failed  bool     // a one-line check: its condition is false
	Message string   // a one-line check: its message, or its template's text after a hard error in it
	Reports []Report // a block check: its fail and warn calls, in order
}

// Report is one fail(at, message) or warn(at, message) of a block check; a nil At reports
// without a location.
type Report struct {
	Warn    bool
	At      value.Value
	Message string
}

// Runner runs stages C and D over one build's values. It remembers the instances it checked,
// so one goroutine uses it.
type Runner struct {
	ev     Evaluator
	info   *check.Info
	bags   map[string]*diag.Bag
	files  map[*syntax.CheckDecl]*syntax.File
	broken map[types.Type]bool
	seen   map[*value.Record]bool
	below  map[value.Value]bool
	paths  map[value.Value]*verify.Path
}

// New is the check runner of a checked program; bags holds a bag per selected package.
func New(ev Evaluator, prog *check.Program, bags map[string]*diag.Bag) *Runner {
	r := &Runner{
		ev: ev, bags: bags,
		files:  map[*syntax.CheckDecl]*syntax.File{},
		broken: map[types.Type]bool{},
		seen:   map[*value.Record]bool{},
		below:  map[value.Value]bool{},
		paths:  map[value.Value]*verify.Path{},
	}
	if prog == nil {
		return r
	}
	r.info = prog.Info
	for _, pkg := range prog.Packages {
		r.indexBroken(pkg)
		for _, f := range pkg.Files {
			syntax.Inspect(f, func(n syntax.Node) bool {
				if c, ok := n.(*syntax.CheckDecl); ok {
					r.files[c] = f
				}
				return true
			})
		}
	}
	return r
}

// indexBroken records the broken records and variants, whose checks never run (TYPES.md §1).
func (r *Runner) indexBroken(pkg *check.Package) {
	for _, obj := range pkg.Decls {
		if obj.Kind() == check.ObjTypeName && r.isBroken(obj) && obj.Type() != nil {
			r.broken[obj.Type().Base()] = true
		}
	}
}

func (r *Runner) isBroken(obj check.Object) bool {
	return r.info != nil && r.info.Broken[obj]
}

// brokenType: the instance's record, or its case's variant, is broken.
func (r *Runner) brokenType(t types.Type) bool {
	switch d := t.Base().(type) {
	case *types.CaseType:
		return r.broken[d.Variant]
	case *types.AppliedRecord:
		return r.broken[d.Rec]
	}
	return r.broken[t.Base()]
}

// bagOf is the bag of a package, or ErrNoBag.
func (r *Runner) bagOf(pkg string) (*diag.Bag, error) {
	if bag := r.bags[pkg]; bag != nil {
		return bag, nil
	}
	return nil, fmt.Errorf(fmtNoBag, ErrNoBag, pkg)
}
