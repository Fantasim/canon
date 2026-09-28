package typedef

import (
	"context"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// Input is what the `types` section reads.
type Input struct {
	Program *check.Program
	Index   *control.Index
	Colls   *encode.Colls
	Fold    check.Folder // folds constant field defaults (TYPES.md §15); nil writes none
	Studio  string       // project.studio's package, "" for none
}

// Types writes the type definitions and type expressions of one package's view model.
type Types struct {
	in      Input
	ctx     context.Context
	pkg     string
	objects map[any]check.Object // each declared record, variant, enum or type function
	applied map[*types.TypeFunc][]*types.Collection
}

// New writes the types of the package pkg of in.Program; without an Index no view property is
// read, without Colls every collection is empty.
func New(ctx context.Context, in Input, pkg string) *Types {
	if in.Index == nil {
		in.Index = control.NewIndex(&check.Program{Info: in.Program.Info}, "")
	}
	if in.Colls == nil {
		in.Colls = encode.NewColls(nil)
	}
	return &Types{in: in, ctx: ctx, pkg: pkg, objects: map[any]check.Object{}}
}

// Section is the package's `types` (VIEWMODEL.md 12.3): every type it declares that is public or
// reachable from a public type or value, broken declarations left out (J4), by qualified name.
func (s *Types) Section() map[string]vm.TypeDef {
	out := map[string]vm.TypeDef{}
	p := s.pkgOf()
	if p == nil {
		return out
	}
	for _, d := range s.reachable(p) {
		if s.ctx.Err() != nil {
			return out
		}
		out[d.name] = s.def(d)
	}
	return out
}

// pkgOf is the checked package being written, nil when the program does not hold it.
func (s *Types) pkgOf() *check.Package {
	for _, p := range s.in.Program.Packages {
		if p.Path == s.pkg {
			return p
		}
	}
	return nil
}

// def is one type definition.
func (s *Types) def(d declared) vm.TypeDef {
	switch x := d.t.(type) {
	case *types.RecordType:
		return s.record(x)
	case *types.VariantType:
		return s.variant(x)
	case *types.EnumType:
		return s.enum(x)
	}
	return s.typeFunc(d.fn)
}
