package gorules

import (
	"fmt"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// surface is what the type-checked load says about the module's exported API.
type surface struct {
	used  map[string]bool
	iface map[string]bool
}

// exportedLocal reports exported package-level objects and exported methods that no other
// package references; a failed or erroneous load skips the rule rather than guess.
func (s *scan) exportedLocal() *lane.Skip {
	pkgs, err := s.ctx.Go.Typed()
	if err == nil {
		err = loadErrors(pkgs)
	}
	if err != nil {
		return &lane.Skip{What: ruleExportedLocal, Reason: fmt.Sprintf(skipTyped, err)}
	}
	sf := surface{used: externalUses(pkgs), iface: viaInterface(pkgs)}
	for _, p := range pkgs {
		if plain(p) && p.Name != mainPkg && p.Types != nil {
			s.pkgSurface(p, sf)
		}
	}
	return nil
}

// plain is the package itself, not a test variant, external test or test main.
func plain(p *packages.Package) bool {
	return p.ID == p.PkgPath && !strings.HasSuffix(p.PkgPath, testPkgSuffix) && !strings.HasSuffix(p.PkgPath, testMainSuffix)
}

func loadErrors(pkgs []*packages.Package) error {
	var first error
	n := 0
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			if first == nil {
				first = e
			}
			n++
		}
	})
	if first != nil {
		return fmt.Errorf("%w: %d, first: %w", errTyped, n, first)
	}
	return nil
}

// externalUses keys every package-level object and method some package other than its own
// references; an external _test package counts as another package.
func externalUses(pkgs []*packages.Package) map[string]bool {
	used := map[string]bool{}
	for _, p := range pkgs {
		if p.TypesInfo == nil || strings.HasSuffix(p.ID, testMainSuffix) {
			continue
		}
		for _, obj := range p.TypesInfo.Uses {
			if k := objKey(obj); k != "" && obj.Pkg().Path() != p.PkgPath {
				used[k] = true
			}
		}
	}
	return used
}

// objKey names a package-level object "path.Name" and a method "path.Type.Method", the
// same across a package and its test variants; "" for anything else.
func objKey(obj types.Object) string {
	if obj.Pkg() == nil {
		return ""
	}
	if fn, ok := obj.(*types.Func); ok {
		if recv := fn.Origin().Signature().Recv(); recv != nil {
			return strings.Join([]string{obj.Pkg().Path(), recvName(recv.Type()), fn.Name()}, dirHere)
		}
	}
	if obj.Parent() != obj.Pkg().Scope() {
		return ""
	}
	return obj.Pkg().Path() + dirHere + obj.Name()
}

func recvName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Origin().Obj().Name()
	}
	return ""
}

// pkgSurface flags p's exported objects and methods nobody outside p uses.
func (s *scan) pkgSurface(p *packages.Package, sf surface) {
	reach := reachable(p, sf.used)
	scope := p.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if tn, ok := obj.(*types.TypeName); ok && !tn.IsAlias() {
			s.methods(p, tn, sf)
		}
		if !obj.Exported() || sf.used[objKey(obj)] {
			continue
		}
		if tn, ok := obj.(*types.TypeName); ok && reach[tn] {
			continue
		}
		s.flagExported(p, obj, obj.Name(), objKind(obj))
	}
}

func (s *scan) methods(p *packages.Package, tn *types.TypeName, sf surface) {
	named, ok := tn.Type().(*types.Named)
	if !ok || types.IsInterface(named) {
		return
	}
	for m := range named.Methods() {
		k := objKey(m)
		if !m.Exported() || sf.used[k] || sf.iface[k] || slices.Contains(wellKnownMethods, m.Name()) {
			continue
		}
		s.flagExported(p, m, tn.Name()+dirHere+m.Name(), kindMethod)
	}
}

func (s *scan) flagExported(p *packages.Package, obj types.Object, symbol, kind string) {
	pos := p.Fset.Position(obj.Pos())
	rel, err := filepath.Rel(s.ctx.Repo.Root, pos.Filename)
	if err != nil {
		return
	}
	rel = filepath.ToSlash(rel)
	if f, ok := s.ctx.Go.File(rel); !ok || f.Generated {
		return
	}
	fix := ""
	if to := unexported(p, obj); to != "" {
		fix = fmt.Sprintf(fixUnexportName, to)
	}
	s.emit(finding.Finding{
		Rule: ruleExportedLocal, File: rel, Line: pos.Line, Symbol: symbol, Detail: kind,
		Message: fmt.Sprintf(msgExported, kind, symbol), Fix: fix,
	})
}

func objKind(obj types.Object) string {
	switch obj.(type) {
	case *types.Func:
		return token.FUNC.String()
	case *types.TypeName:
		return token.TYPE.String()
	case *types.Const:
		return token.CONST.String()
	}
	return token.VAR.String()
}
