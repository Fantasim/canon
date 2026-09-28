package check

import (
	"context"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Bags holds one bag per loaded package, by path; a map, not a third seam (DECISIONS 34, 100).
type Bags map[string]*diag.Bag

// Check runs phase 2 over the loaded packages' files into their bags (a package without one gets
// a new bag, added to bags), folding constants through fold (eval.NewFolder in a build). A nil
// bags or fold is API misuse: Check then returns nil. The result is safe for concurrent reads.
func Check(ctx context.Context, proj *project.Project, files []*syntax.File, bags Bags, fold Folder) *Program {
	if bags == nil || fold == nil {
		return nil
	}
	c := newChecker(ctx, proj, bags, fold)
	c.loadPackages(files)
	c.syntaxErrors()
	for _, p := range c.sorted {
		c.collect(p)
	}
	for _, p := range c.sorted {
		c.bindImports(p)
	}
	c.orderPackages()
	for _, p := range c.order {
		if ctx.Err() != nil {
			break
		}
		c.resolvePackage(p)
	}
	c.markBodyTables()
	for _, p := range c.order {
		if ctx.Err() != nil {
			break
		}
		c.checkPackage(p)
	}
	c.checkPresentation()
	c.propagateBroken()
	return c.program()
}

// checker holds the state of one Check.
type checker struct {
	ctx        context.Context
	proj       *project.Project
	bags       Bags
	fold       Folder
	info       *Info
	pkgs       map[string]*pkgState
	sorted     []*pkgState // by path
	order      []*pkgState // dependencies first (TYPES.md §3.1)
	universe   map[string]*object
	nextID     int
	pending    map[*types.RefType]*pendingRef
	colls      map[collKey]*types.Collection
	deps       map[*object][]*object
	tableOf    map[*types.RecordType]bool         // records used as the element of a table (TYPES.md §3.6)
	stableOf   map[*types.RecordType]bool         // records used as the element of a stable table
	stableLost map[*pkgState]bool                 // packages with a stable table whose element is in error
	keyedOf    map[*types.RecordType]bool         // records used as the element of a keyed list
	builtins   map[string]*object                 // built-in members and methods, by name
	boolObjs   []*object                          // `false` and `true` as match patterns, in index order
	bodies     bool                               // step 3 has begun: a new ref resolves at once
	constStack []*object                          // the consts being typed, innermost last
	listKeys   map[*object]map[string]source.Span // keys written in a keyed list's literal and entries

	typeObjects  map[types.Type]*object
	members      map[*types.EnumType][]*object
	cases        map[*types.VariantType][]*object
	caseBodies   map[*types.CaseType]*recordCtx
	caseDecls    map[*types.CaseType]*syntax.VariantCase
	variantCases map[*types.VariantType]string // `@json(case:)` of a variant header
	memberChecks map[*syntax.CheckDecl]*object
	fieldObjects map[*types.Field]*object
	typeFuncs    map[*object]*types.TypeFunc
	fnParams     map[*object][]*object
	fieldJobs    map[*types.Field]func()
	initDone     map[*object]bool
	boundSpans   map[*types.Bound]source.Span
	wheres       []whereJob
	unions       []unionJob
	records      int                         // records and variants being completed, innermost last
	funcDepth    map[*object]int             // records being completed when a type function began
	syntaxHeld   map[syntax.Node]bool        // declarations holding a syntax error (DECISIONS 214)
	reported     int                         // the errors report added so far
	unrefined    map[syntax.Node]bool        // refinements dropped for an error (TYPES.md §1)
	refused      map[*types.Param]types.Type // the declared type of a parameter refused by E3806, for messages
	badLits      map[syntax.Node]bool        // literal tokens holding a lexer error (DECISIONS 215)
	unmatchable  map[syntax.Node]bool        // members and cases named by an E1126 word
	layout       *project.Layout             // the roots as written, no --root override (DECISIONS 215)

	views       map[viewKey]*viewCtx       // the first view of each target (VIEWMODEL.md §3.2)
	messageEnvs map[*syntax.CheckDecl]*env // the scope of each one-line check's message (I18N.md T1)
	positions   map[types.Type]*position   // where a view target's values occur (VIEWMODEL.md §3.4)
	lost        map[*pkgState]*position    // by package, the positions collections in error may have given
	steps       map[*object]bool           // the fields a view gives a `step` text (I18N.md §3.3)
}

func newChecker(ctx context.Context, proj *project.Project, bags Bags, fold Folder) *checker {
	c := &checker{
		ctx: ctx, proj: proj, bags: bags, fold: fold,
		info:       newInfo(),
		pkgs:       map[string]*pkgState{},
		pending:    map[*types.RefType]*pendingRef{},
		colls:      map[collKey]*types.Collection{},
		deps:       map[*object][]*object{},
		tableOf:    map[*types.RecordType]bool{},
		stableOf:   map[*types.RecordType]bool{},
		stableLost: map[*pkgState]bool{},
		keyedOf:    map[*types.RecordType]bool{},
		builtins:   map[string]*object{},
		listKeys:   map[*object]map[string]source.Span{},

		typeObjects:  map[types.Type]*object{},
		members:      map[*types.EnumType][]*object{},
		cases:        map[*types.VariantType][]*object{},
		caseBodies:   map[*types.CaseType]*recordCtx{},
		caseDecls:    map[*types.CaseType]*syntax.VariantCase{},
		variantCases: map[*types.VariantType]string{},
		memberChecks: map[*syntax.CheckDecl]*object{},
		fieldObjects: map[*types.Field]*object{},
		typeFuncs:    map[*object]*types.TypeFunc{},
		fnParams:     map[*object][]*object{},
		fieldJobs:    map[*types.Field]func(){},
		initDone:     map[*object]bool{},
		boundSpans:   map[*types.Bound]source.Span{},
		syntaxHeld:   map[syntax.Node]bool{},
		badLits:      map[syntax.Node]bool{},
		unrefined:    map[syntax.Node]bool{},
		refused:      map[*types.Param]types.Type{},
		unmatchable:  map[syntax.Node]bool{},
		funcDepth:    map[*object]int{},
		views:        map[viewKey]*viewCtx{},
		messageEnvs:  map[*syntax.CheckDecl]*env{},
		lost:         map[*pkgState]*position{},
		steps:        map[*object]bool{},
	}
	c.universe = c.newUniverse()
	return c
}

func newInfo() *Info {
	return &Info{
		Types:      map[syntax.Expr]types.Type{},
		TypeExprs:  map[syntax.Type]types.Type{},
		Defs:       map[*syntax.Ident]Object{},
		Uses:       map[*syntax.IdentExpr]Object{},
		NameUses:   map[*syntax.Ident]Object{},
		Selections: map[*syntax.SelectorExpr]*Selection{},
		Conv:       map[syntax.Expr]*Conversion{},
		Keys:       map[syntax.Expr]*types.Collection{},
		Symbols:    map[*syntax.IdentExpr]bool{},
		Calls:      map[*syntax.CallExpr]*Callee{},
		Literals:   map[*syntax.BraceLit]LitKind{},
		Matches:    map[syntax.Node]*MatchInfo{},
		Broken:     map[Object]bool{},
	}
}

// report adds an error of env's declaration, which breaks; a translated template's is E1703's detail.
func (c *checker) report(env *env, b *diag.Builder) {
	c.reported++
	if env.trans != nil {
		diag.E1703.AtType(env.trans.at, env.trans.key, b.Message()).Report(env.pkg.bag)
		return
	}
	b.Report(env.pkg.bag)
	c.breakObj(env.owner)
}

// warn adds a warning; a warning breaks nothing.
func (c *checker) warn(env *env, b *diag.Builder) {
	b.Report(env.pkg.bag)
}

func (c *checker) breakObj(o *object) {
	if o != nil {
		c.info.Broken[o] = true
	}
}

// span locates n in env's file.
func (env *env) span(n syntax.Node) source.Span {
	return env.file.Span(n)
}

// dependsOn records that the declaration checked names o: naming a broken one breaks (TYPES.md §1).
func (c *checker) dependsOn(env *env, o *object) {
	if env.owner == nil || o == nil || o == env.owner || !topLevel(o) {
		return
	}
	c.deps[env.owner] = append(c.deps[env.owner], o)
}

// topLevel reports the objects Broken may hold: declarations with a body of their own.
func topLevel(o *object) bool {
	switch o.kind {
	case ObjConst, ObjLet, ObjFn, ObjMethod, ObjTypeName, ObjCheck, ObjTest, ObjWidget, ObjEntry:
		return true
	default:
		return false
	}
}

// propagateBroken breaks every declaration that names a broken one, until nothing changes.
func (c *checker) propagateBroken() {
	for changed := true; changed; {
		changed = false
		for _, p := range c.order {
			changed = c.propagateIn(p) || changed
		}
	}
}

func (c *checker) propagateIn(p *pkgState) bool {
	changed := false
	for _, o := range p.all {
		if c.info.Broken[o] {
			continue
		}
		for _, d := range c.deps[o] {
			if c.info.Broken[d] {
				c.info.Broken[o] = true
				changed = true
				break
			}
		}
	}
	return changed
}

// program is the result: packages dependencies first, with their declarations and layers.
func (c *checker) program() *Program {
	prog := &Program{Info: c.info}
	for _, p := range c.order {
		prog.Packages = append(prog.Packages, p.pkg)
	}
	for _, p := range c.order {
		for _, imp := range p.imports {
			p.pkg.Imports = append(p.pkg.Imports, imp.pkg)
		}
	}
	return prog
}
