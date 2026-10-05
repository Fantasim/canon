package ir

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Host is what stage E asks of the evaluator (EVALUATION.md §2.3); `build` implements it with eval, which is not in ir's own Consumes column (IMPLEMENTATION-PLAN §3). A false result follows a finding the evaluator or the verifier already reported.
type Host interface {
	// Value is the evaluated, verified top-level const or let name of pkg.
	Value(ctx context.Context, pkg, name string) (value.Value, bool)
	// Call evaluates export fn fn on recv (nil for a package fn) and args, in stage E.
	Call(ctx context.Context, fn check.Object, recv value.Value, args []value.Value) (value.Value, bool)
}

// Input is what stage E reads: the checked program, the project's declared roots and go_module (never a --root override, decision 108), the selected packages (nil: every package of Program), the bags, the host, and the folder of constant field defaults.
type Input struct {
	Program  *check.Program
	Project  *project.Project
	Selected []string
	Bags     check.Bags
	Host     Host
	Fold     check.Folder
}

// Build runs stage E (EVALUATION.md §1 phase 7): it precomputes export fn results, builds the IR of every selected package that declares an emit, and reports every emit rule (CODEGEN.md §12 but E8001, WIRE.md §8.1 but E8152, CONFORMANCE.md §2) to the bags, writing nothing.
func Build(ctx context.Context, in Input) []*Package {
	s := newStage(ctx, in)
	for _, u := range s.order {
		s.emits(u)
	}
	var out []*Package
	for _, u := range s.order {
		if u.selected && len(u.emits) > 0 {
			s.assemble(u)
			s.orderFns(u)
			out = append(out, u.p)
		}
	}
	s.precompute()
	s.lookupDomains()
	for _, u := range s.order {
		if u.selected && len(u.emits) > 0 {
			s.finish(u)
			s.validate(u)
			s.checkMethodCycles(u)
		}
	}
	s.crossPackage()
	return out
}

// stage is one run of stage E. named maps a declaration (*types.RecordType, EnumType,
// VariantType or TypeFunc) to its IR type, shared by every package that reaches it; decls,
// fieldSites, fnObjs and nodeSites locate the IR nodes findings point at; fnByObj is fnObjs reversed.
type stage struct {
	ctx           context.Context
	in            Input
	info          *check.Info
	layout        *project.Layout
	named         map[any]Type
	units         map[string]*unit
	order         []*unit
	decls         map[Type]declSite
	fnObjs        map[*ExportFn]*fnSite
	fnByObj       map[check.Object]*fnSite
	nodeSites     map[any]declSite // enum members, cases, parameters, constants and values
	domainsOf     map[*ExportFn]*fnDomains
	fieldSites    map[*Field]*fieldSite
	depFns        map[*Dependent]*types.TypeFunc
	branchTypes   map[*Branch]types.Type
	members       map[*Variant][]source.Span // a variant's export fns written outside its cases (TYPES.md §12.1)
	variantFns    map[*syntax.FnDecl]bool    // every fn written in a variant body outside its cases
	cyclic        map[*ExportFn]bool         // cyclicFn's verdicts (DECISIONS 284)
	cycleReported map[cycleReport]bool
	owners        map[ownerKey]func(Type, *Case) bool // each owner's Written of its hooks, per target (hookWritten)
}

// unit is one checked package on its way to IR; firstUse is the first type of each imported
// package its IR names, reach that of each package only its code emits reach, for E8004.
type unit struct {
	cp           *check.Package
	p            *Package
	bag          *diag.Bag
	selected     bool
	emits        []*emitSite
	values       []*valueSite
	fns          []*fnSite
	consts       []*constSite
	firstUse     map[string]string
	reach        map[string]string
	cppNames     []cppShared   // the names its data-mode cpp header declares in namespaces other packages share
	variantCalls []source.Span // translated calls of a variant-level export fn (E8019 VariantMethod)
}

// constSite is a public const with its IR and declaration.
type constSite struct {
	c    *Const
	obj  check.Object
	decl *syntax.ConstDecl
}

func (c *constSite) span() source.Span { return c.obj.File().Span(c.decl.Name) }

// declSite locates a declaration for findings.
type declSite struct {
	file *syntax.File
	node syntax.Node
}

func (d declSite) span() source.Span {
	if d.file == nil {
		return source.Span{}
	}
	return d.file.Span(d.node)
}

// fieldSite is a field's declaration and checked type.
type fieldSite struct {
	declSite
	tf *types.Field
}

// valueSite is a public let with its IR value and declaration.
type valueSite struct {
	v    *Value
	obj  check.Object
	decl *syntax.LetDecl
	t    types.Type
}

func newStage(ctx context.Context, in Input) *stage {
	s := &stage{
		ctx: ctx, in: in, info: in.Program.Info, named: map[any]Type{}, units: map[string]*unit{},
		decls: map[Type]declSite{}, fnObjs: map[*ExportFn]*fnSite{}, fnByObj: map[check.Object]*fnSite{}, nodeSites: map[any]declSite{},
		domainsOf: map[*ExportFn]*fnDomains{}, fieldSites: map[*Field]*fieldSite{}, depFns: map[*Dependent]*types.TypeFunc{},
		branchTypes: map[*Branch]types.Type{}, members: map[*Variant][]source.Span{},
		variantFns: map[*syntax.FnDecl]bool{}, cyclic: map[*ExportFn]bool{}, cycleReported: map[cycleReport]bool{},
		owners: map[ownerKey]func(Type, *Case) bool{},
	}
	// nil overrides (decision 108) never name an undeclared root, so NewLayout's ok is always true.
	s.layout, _ = project.NewLayout(in.Project, curDir, nil, nil)
	for _, cp := range in.Program.Packages {
		u := &unit{cp: cp, bag: in.Bags[cp.Path], selected: in.Selected == nil || slices.Contains(in.Selected, cp.Path)}
		u.p = &Package{Name: cp.Path, Dir: packageDir(cp), Doc: packageDoc(cp)}
		s.units[cp.Path] = u
		s.order = append(s.order, u)
	}
	return s
}

// report adds a finding to u's bag; a package without a bag (not loaded by build) takes none.
func (u *unit) report(b *diag.Builder) {
	if u.bag != nil {
		b.Report(u.bag)
	}
}

// sourceFiles yields the source files of cp, in path order (layer and translation files excluded).
func sourceFiles(cp *check.Package) []*syntax.File {
	var out []*syntax.File
	for _, f := range cp.Files {
		if f.FileKind == syntax.FileSource {
			out = append(out, f)
		}
	}
	return out
}
