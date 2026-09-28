package views

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/typedef"
)

// Input is what a package's view model is built from (VIEWMODEL.md §12).
type Input struct {
	Program  *check.Program
	Package  string                              // the package whose model is built
	Language string                              // project.canon's `canon` version, "0.1"
	Studio   string                              // project.studio's package, "" for none
	Force    func(eval.Root) (value.Value, bool) // a settled value (build.Analysis.Force); nil: none
	Fold     check.Folder                        // folds constant field defaults; nil writes none
}

// Build is the view model of in.Package (VIEWMODEL.md 12): its envelope, then each section
// by steps, in order.
func Build(ctx context.Context, in Input) (*vm.ViewModel, error) {
	b, err := newBuilder(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	for _, step := range steps {
		step(b)
		if err := ctx.Err(); err != nil { // a step stops early on cancellation: its section is partial
			return nil, fmt.Errorf(fmtWrap, err)
		}
	}
	return b.m, nil
}

// steps fill the sections of the model one after the other.
var steps = []func(*builder){
	(*builder).types,
}

// builder is one model being built, and what its sections share.
type builder struct {
	m     *vm.ViewModel
	colls *encode.Colls
	index *control.Index
	defs  *typedef.Types
}

// newBuilder starts the model of in.Package: its envelope, every member but `studio` present
// and empty (J3's "always present" members).
func newBuilder(ctx context.Context, in Input) (*builder, error) {
	if in.Program == nil || !holds(in.Program, in.Package) {
		return nil, fmt.Errorf(fmtPackage, ErrNoPackage, in.Package)
	}
	b := &builder{colls: encode.NewColls(force(in.Force))}
	b.index = control.NewIndex(in.Program, in.Studio)
	b.defs = typedef.New(ctx, typedef.Input{Program: in.Program, Index: b.index, Colls: b.colls, Fold: in.Fold, Studio: in.Studio}, in.Package)
	b.m = &vm.ViewModel{
		Schema: SchemaVersion, Package: in.Package, Language: in.Language, Requires: []string{},
		Types: map[string]vm.TypeDef{}, Views: map[string]vm.View{}, Values: map[string]vm.Value{},
		Usage: map[string]vm.Usage{}, Search: map[string]vm.SearchIndex{}, Assets: map[string]vm.Asset{},
		Units: map[string]vm.Unit{}, Widgets: map[string]vm.Widget{}, Findings: []vm.Finding{},
	}
	return b, nil
}

// types is the `types` section (§12.3).
func (b *builder) types() { b.m.Types = b.defs.Section() }

// holds reports a program holding the package path.
func holds(prog *check.Program, path string) bool {
	for _, p := range prog.Packages {
		if p.Path == path {
			return true
		}
	}
	return false
}

// force reads a settled let through f, none when f is nil.
func force(f func(eval.Root) (value.Value, bool)) encode.Force {
	if f == nil {
		return nil
	}
	return func(pkg, name string) (value.Value, bool) { return f(eval.Root{Pkg: pkg, Name: name}) }
}
