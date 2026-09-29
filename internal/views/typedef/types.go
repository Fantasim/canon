package typedef

import (
	"context"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// Input is what the `types` section reads.
type Input struct {
	Program *check.Program
	Index   *control.Index
	Colls   *encode.Colls
	Texts   *encode.Texts
	Assets  *encode.Assets // asset roots by display path; nil writes them as written
	Fold    check.Folder   // folds constant field defaults (TYPES.md §15); nil writes none
	Drivers Resolve        // the program `drivers` are read over (12.3), on first need; nil: Program and Colls
}

// Resolve is the World drivers are read over, asked only when a type function needs drivers.
type Resolve func() (*World, error)

// Types writes the type definitions and type expressions of one package's view model.
type Types struct {
	in        Input
	ctx       context.Context
	pkg       string
	objects   map[any]check.Object // each declared record, variant, enum or type function
	applied   map[*types.TypeFunc][]*types.Collection
	described []types.Type
	world     *World // in.Drivers resolved, once needed
	err       error  // what resolving it met, or a type function it lacks (Err)
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
	if in.Drivers == nil {
		own := &World{Program: in.Program, Colls: in.Colls}
		in.Drivers = func() (*World, error) { return own, nil }
	}
	return &Types{in: in, ctx: ctx, pkg: pkg, objects: map[any]check.Object{}}
}

// Section is the package's `types` (VIEWMODEL.md 12.3): every type it declares that is public or
// reachable from a public type or value, broken declarations left out (J4), by qualified name.
func (s *Types) Section() map[string]vm.TypeDef {
	out := map[string]vm.TypeDef{}
	p := shape.Package(s.in.Program, s.pkg)
	if p == nil {
		return out
	}
	for _, d := range s.reachable(p) {
		if s.ctx.Err() != nil {
			return out
		}
		out[d.name] = s.def(d)
		if d.t != nil {
			s.described = append(s.described, d.t)
		}
	}
	return out
}

// Described are the records, variants and enums Section wrote, in the order met.
func (s *Types) Described() []types.Type { return s.described }

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
