package progen

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
)

// Finding is a finding reduced to what a property compares: its code and where it starts.
type Finding struct {
	Code      diag.Code
	Path      string
	Line, Col int
	Start     int
	Message   string
}

// String is the finding as "CODE path:line:col".
func (f Finding) String() string {
	return fmt.Sprintf(findingFormat, f.Code, f.Path, f.Line, f.Col)
}

// RunOptions say what Run does with a project.
type RunOptions struct {
	Packages []string          // selectors; none selects every package
	Roots    map[string]string // --root overrides, as build.Options.Roots
	Layers   []string          // active layers, in order
	Build    bool              // Build in check mode (nothing written) rather than Check
	Targets  []ir.Target       // Build's targets; none is every target
}

// Outcome is what one run produced: its findings in the build's order, the outputs a Build call
// wrote (nil for Check), the error that stopped it, and a recovered panic with its stack ("" when
// the compiler did not panic).
type Outcome struct {
	Findings []Finding
	Outputs  []build.Output
	Err      error
	Panic    string
}

// Run opens p at projectDir and checks (or builds) it. A panic anywhere in the compiler is
// recovered into Outcome.Panic: the property suites report it, never crash on it.
func Run(ctx context.Context, p *Project, opt RunOptions) (out Outcome) {
	defer func() {
		if r := recover(); r != nil {
			out = Outcome{Panic: fmt.Sprintf(panicFormat, r, debug.Stack())}
		}
	}()
	proj, err := build.Open(p.fsys(), projectDir, build.Options{Roots: opt.Roots, Layers: opt.Layers})
	if err != nil {
		return failed(err)
	}
	if opt.Build {
		res, err := proj.Build(ctx, build.BuildOptions{Packages: opt.Packages, Targets: opt.Targets, Check: true})
		if err != nil {
			return failed(err)
		}
		return Outcome{Findings: reduce(res.Findings), Outputs: res.Outputs}
	}
	res, err := proj.Check(ctx, opt.Packages)
	if err != nil {
		return failed(err)
	}
	return Outcome{Findings: reduce(res.Findings)}
}

// failed is the outcome of a run err stopped, with the findings an *build.OpenError carries.
func failed(err error) Outcome {
	var oe *build.OpenError
	if errors.As(err, &oe) {
		return Outcome{Findings: reduce(oe.Findings), Err: err}
	}
	return Outcome{Err: err}
}

// Unit is a package of a project: its name and its files' project-relative paths.
type Unit struct {
	Name  string
	Files []string
}

// Units lists the packages of p in name order.
func Units(ctx context.Context, p *Project, roots map[string]string) ([]Unit, error) {
	proj, err := build.Open(p.fsys(), projectDir, build.Options{Roots: roots})
	if err != nil {
		return nil, fmt.Errorf("progen: %w", err)
	}
	us, err := proj.Packages(ctx)
	if err != nil {
		return nil, fmt.Errorf("progen: %w", err)
	}
	out := make([]Unit, 0, len(us.Units))
	for _, u := range us.Units {
		unit := Unit{Name: u.Name}
		for _, f := range u.Files {
			unit.Files = append(unit.Files, f.Src.Path)
		}
		out = append(out, unit)
	}
	return out, nil
}

// reduce keeps each finding's code, severity and resolved start.
func reduce(fs build.Findings) []Finding {
	out := make([]Finding, 0, len(fs.List))
	for _, f := range fs.List {
		r := Finding{Code: f.Code, Start: int(f.Span.Start), Message: f.Message}
		if fs.Files != nil {
			r.Path = fs.Files.Path(f.Span.File)
			if r.Path != "" {
				r.Line, r.Col = fs.Files.Position(f.Span.File, f.Span.Start)
			}
		}
		out = append(out, r)
	}
	return out
}
