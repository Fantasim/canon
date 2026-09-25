package ir

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// GoPure is a translated fn's Go names (CODEGEN.md §3.3, §5.10; CONFORMANCE.md §2.3, §7): its public method or function, its pure function (the public one at package level), its conformance test, the local of each read, parameter and `let` by Canon name, each read's ok local ("" when not optional), and the vector struct's field of each read then parameter with each read's ok field.
type GoPure struct {
	Public, Pure, Test string
	Locals             map[string]string
	OKs                []string
	Fields, FieldOKs   []string
	TestLocal          string // the test's *testing.T
	fn                 *ExportFn
	origin             string
}

// Pure is a translated fn's names, nil for another fn or one the emit does not write.
func (pl *GoNamePlan) Pure(fn *ExportFn) *GoPure { return pl.pures[fn] }

// declarePure names a translated fn of the struct owner ("" at package level): its pure function lowerCamel(owner) + the public name, then its locals and vector fields, each escaped once (CODEGEN.md §3.4; log-2026-09-24 "gen/go translated fns").
func (pl *GoNamePlan) declarePure(top *nameScope, owner, origin string, fn *ExportFn) {
	public := goExported(fn.Go, fn.Name)
	p := &GoPure{Public: public, Pure: public, Test: goTestPrefix + owner + public + goConformanceSuffix, Locals: map[string]string{}, fn: fn, origin: origin}
	if owner != "" {
		p.Pure = goLowerFirst(owner) + public
	}
	pl.declare(top, p.Pure, origin, fn)
	pl.pures[fn] = p
	sc := pl.scope(origin + goParamsSuffix)
	names := make([]string, 0, len(fn.Reads)+len(fn.Params))
	for _, r := range fn.Reads {
		names = append(names, r.Name)
	}
	for _, prm := range fn.Params {
		names = append(names, prm.Name)
	}
	for _, n := range append(names, letNames(fn.Body, nil)...) {
		if _, done := p.Locals[n]; !done {
			p.Locals[n] = pl.pureLocal(sc, p, n)
		}
	}
	for _, r := range fn.Reads {
		ok := ""
		if r.Optional {
			ok = pl.pureLocal(sc, p, p.Locals[r.Name]+goOKSuffix)
		}
		p.OKs = append(p.OKs, ok)
	}
	pl.declareVectorFields(p)
}

// goLowerFirst is a Go type name with its first word lower-cased: Potion, potion (CODEGEN.md §5.10).
func goLowerFirst(goName string) string {
	ws := words(goName)
	if len(ws) == 0 {
		return goName
	}
	return strings.ToLower(ws[0]) + strings.Join(ws[1:], "")
}

// pureLocal is a Canon name as a local of the pure function, escaped once when reserved there: a keyword, a predeclared identifier, a standard package (unconditionally, log-2026-09-24), an imported Canon package, `self`, a package-level name the body may use, or the pure function itself (CODEGEN.md §3.4); an escaped name reserved too is E8005 like an escaped import (decision 182).
func (pl *GoNamePlan) pureLocal(sc *nameScope, p *GoPure, name string) string {
	local := name
	if pl.pureReserved(p, name) {
		local = name + underscore
		if pl.pureReserved(p, local) {
			pl.problems = append(pl.problems, GoNameProblem{Kind: GoCollision, Scope: sc.what, Name: local, First: local, Origin: p.origin + qnameSep + name, Item: p.fn})
		}
	}
	pl.declare(sc, local, p.origin+qnameSep+name, p.fn)
	return local
}

func (pl *GoNamePlan) pureReserved(p *GoPure, name string) bool {
	_, imported := pl.imports[name]
	return goReserved(name) || imported || name == p.Pure || pl.packageNames()[name]
}

// packageNames are the package-level names a translated body or method may name unqualified: types, enum and kind members, table ids, package fns.
func (pl *GoNamePlan) packageNames() map[string]bool {
	if pl.pkgNames != nil {
		return pl.pkgNames
	}
	pl.pkgNames = map[string]bool{}
	for _, t := range pl.p.Types {
		pl.pkgNames[pl.TypeName(t)] = true
		pl.memberNames(t)
	}
	for _, v := range pl.p.Values {
		if rec := tableRecord(v); rec != nil {
			for _, id := range v.IDs {
				pl.pkgNames[pl.IDMemberName(rec, id)] = true
			}
		}
	}
	for _, fn := range pl.p.Fns {
		pl.pkgNames[goExported(fn.Go, fn.Name)] = true
	}
	return pl.pkgNames
}

// memberNames adds an enum's member constants, a variant's kind members.
func (pl *GoNamePlan) memberNames(t Type) {
	switch x := t.(type) {
	case *Enum:
		for _, m := range x.Members {
			pl.pkgNames[pl.MemberName(x, m)] = true
		}
	case *Variant:
		for _, c := range x.Cases {
			pl.pkgNames[pl.KindMemberName(x, c)] = true
		}
	}
}

// declareVectorFields names the test's vector struct fields: each input's local, with `_` when it is want or code, the template's own fields (CONFORMANCE.md §7.2), and the test's *testing.T, escaped once against the imported Canon packages.
func (pl *GoNamePlan) declareVectorFields(p *GoPure) {
	sc, fn := pl.scope(p.Test+goVectorsScope), p.fn
	for _, n := range goVectorOwn {
		pl.declare(sc, n, p.Test, fn)
	}
	field := func(canon, local string) string {
		if slices.Contains(goVectorOwn, local) {
			local += underscore
		}
		pl.declare(sc, local, p.origin+qnameSep+canon, fn)
		return local
	}
	for i, r := range fn.Reads {
		p.Fields = append(p.Fields, field(r.Name, p.Locals[r.Name]))
		ok := ""
		if p.OKs[i] != "" {
			ok = field(p.Locals[r.Name]+goOKSuffix, p.OKs[i])
		}
		p.FieldOKs = append(p.FieldOKs, ok)
	}
	for _, prm := range fn.Params {
		p.Fields = append(p.Fields, field(prm.Name, p.Locals[prm.Name]))
	}
	local, ok := pl.local(goTestingLocal)
	if !ok {
		pl.problems = append(pl.problems, GoNameProblem{Kind: GoCollision, Scope: p.Test, Name: local, First: pl.imports[local], Origin: p.origin, Item: fn})
	}
	p.TestLocal = local
}

// letNames are the names every `let` of a body binds, in order.
func letNames(n PExpr, out []string) []string {
	switch x := n.(type) {
	case *Let:
		return letNames(x.Body, append(out, x.Name))
	case *If:
		return letNames(x.Else, letNames(x.Then, out))
	case *Block:
		return blockLets(x, out)
	}
	return out
}

func blockLets(x *Block, out []string) []string {
	if x == nil {
		return out
	}
	for _, st := range x.Stmts {
		switch s := st.(type) {
		case *LetStmt:
			out = append(out, s.Name)
		case *IfStmt:
			out = blockLets(s.Else, blockLets(s.Then, out))
		}
	}
	return out
}

// declareConformance declares the conformance file's package-level names, the tests in declaration order, canonCatch and canonShow (CONFORMANCE.md §7), and the packages it imports, a scope of that file (CODEGEN.md §2.8).
func (pl *GoNamePlan) declareConformance(top *nameScope) {
	fns := slices.SortedStableFunc(maps.Keys(pl.pures), byOrder)
	if len(fns) == 0 {
		return
	}
	show := false
	for _, fn := range fns {
		pl.declare(top, pl.pures[fn].Test, pl.pures[fn].origin, fn)
		show = show || slices.ContainsFunc(fn.Reads, func(r *Read) bool { return r.Optional })
	}
	pl.declare(top, goCanonCatch, pl.p.Name, nil)
	if show {
		pl.declare(top, goCanonShow, pl.p.Name, nil)
	}
	pl.declareConformanceImports(fns, show)
}

// byOrder sorts export fns by their declaration rank (CONFORMANCE.md §7.2).
func byOrder(a, b *ExportFn) int { return cmp.Compare(a.Order, b.Order) }

// declareConformanceImports declares what the conformance file imports: rt and testing, fmt for canonShow, math for an int64 limit or a float (CONFORMANCE.md §5, §7.2), and each Canon package an input's or result's enum or key belongs to.
func (pl *GoNamePlan) declareConformanceImports(fns []*ExportFn, show bool) {
	u := &goImportUse{own: pl.p.Name, std: map[string]bool{goRT: true, goTesting: true, goFmt: show}, pkgs: map[string]bool{}}
	for _, fn := range fns {
		u.std[goMath] = u.std[goMath] || fn.Result.Kind == types.Float || slices.ContainsFunc(fn.Params, conformanceMath)
		for _, r := range fn.Reads {
			u.pkgRef(&r.Type)
		}
		for _, prm := range fn.Params {
			u.pkgRef(&prm.Type)
		}
		u.pkgRef(&fn.Result)
	}
	pl.declareUsed(pl.scope(goScopeConformance), u)
}

// conformanceMath reports a parameter whose candidates include an int64 limit or -0.0, which the test writes through math (CONFORMANCE.md §6.2): an Int or a Float.
func conformanceMath(p *Param) bool {
	return p.Type.Kind == types.Float || p.Type.Kind == types.Int && p.Type.Bits == goInt64Bits && p.Type.Signed
}
