package gorules

import (
	"go/token"
	"go/types"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
)

// reachable lists p's named types that externally used objects expose: in a signature, an
// exported field, a variable's type. Unexporting one would leave an exported API returning
// an unexported type.
func reachable(p *packages.Package, used map[string]bool) map[*types.TypeName]bool {
	r := gosrc.NewReacher(func(tn *types.TypeName) bool { return tn.Pkg() == p.Types })
	r.Seed(p.Types, func(obj types.Object) bool { return used[objKey(obj)] })
	return r.Seen
}

// viaInterface keys every method some interface in the load can call: for each module named
// type T and interface I that T or *T implements, the method of T each of I's methods selects,
// promoted ones included.
func viaInterface(pkgs []*packages.Package) map[string]bool {
	byName := interfaces(pkgs)
	out := map[string]bool{}
	for _, p := range pkgs {
		if !plain(p) || p.Types == nil {
			continue
		}
		scope := p.Types.Scope()
		for _, name := range scope.Names() {
			if tn, ok := scope.Lookup(name).(*types.TypeName); ok && !tn.IsAlias() && !types.IsInterface(tn.Type()) {
				implemented(tn, byName, out)
			}
		}
	}
	return out
}

func implemented(tn *types.TypeName, byName map[string][]*types.Interface, out map[string]bool) {
	ptr := types.NewPointer(tn.Type())
	ms := types.NewMethodSet(ptr)
	generic := false
	if n, ok := tn.Type().(*types.Named); ok {
		generic = n.TypeParams().Len() > 0
	}
	checked := map[*types.Interface]bool{}
	for sel := range ms.Methods() {
		for _, it := range byName[sel.Obj().Name()] {
			if checked[it] {
				continue
			}
			checked[it] = true
			if generic || satisfies(ptr, it) {
				markSelected(ms, it, out)
			}
		}
	}
}

func satisfies(t types.Type, it *types.Interface) bool {
	if it.IsMethodSet() {
		return types.Implements(t, it)
	}
	return types.Satisfies(t, it)
}

func markSelected(ms *types.MethodSet, it *types.Interface, out map[string]bool) {
	for m := range it.Methods() {
		if sel := ms.Lookup(m.Pkg(), m.Name()); sel != nil {
			out[objKey(sel.Obj())] = true
		}
	}
}

// interfaces indexes by method name every interface with methods declared at package level
// in the module or anything it imports, transitively, and every interface type the module's
// own code spells.
func interfaces(pkgs []*packages.Package) map[string][]*types.Interface {
	seen := ifaceSet{}
	visited := map[*types.Package]bool{}
	for _, p := range pkgs {
		if p.Types != nil {
			seen.walk(p.Types, visited)
		}
		if p.TypesInfo != nil {
			for _, tv := range p.TypesInfo.Types {
				seen.add(tv.Type)
			}
		}
	}
	byName := map[string][]*types.Interface{}
	for it := range seen {
		for m := range it.Methods() {
			byName[m.Name()] = append(byName[m.Name()], it)
		}
	}
	return byName
}

// ifaceSet collects distinct interface types that have methods.
type ifaceSet map[*types.Interface]bool

// walk adds the package-level interfaces of pkg and of everything it imports.
func (s ifaceSet) walk(pkg *types.Package, visited map[*types.Package]bool) {
	if visited[pkg] {
		return
	}
	visited[pkg] = true
	for _, name := range pkg.Scope().Names() {
		s.add(pkg.Scope().Lookup(name).Type())
	}
	for _, imp := range pkg.Imports() {
		s.walk(imp, visited)
	}
}

func (s ifaceSet) add(t types.Type) {
	if t == nil {
		return
	}
	if it, ok := t.Underlying().(*types.Interface); ok && it.NumMethods() > 0 {
		s[it] = true
	}
}

// unexported proposes the unexported name for obj, or "" for a SCREAMING_SNAKE name or when
// it would collide with a keyword, a predeclared name, an import or a name in its scope.
func unexported(p *packages.Package, obj types.Object) string {
	to := lowerFirst(obj.Name())
	if strings.Contains(to, underscore) || token.IsKeyword(to) || types.Universe.Lookup(to) != nil {
		return ""
	}
	if slices.ContainsFunc(p.Types.Imports(), func(q *types.Package) bool { return q.Name() == to }) {
		return ""
	}
	if fn, ok := obj.(*types.Func); ok && fn.Signature().Recv() != nil {
		if o, _, _ := types.LookupFieldOrMethod(fn.Signature().Recv().Type(), true, p.Types, to); o != nil {
			return ""
		}
		return to
	}
	if p.Types.Scope().Lookup(to) != nil {
		return ""
	}
	return to
}

// lowerFirst lowers a leading word: Foo -> foo, HTTPClient -> httpClient, IDs -> ids,
// TOTPURIScheme -> totpURIScheme, PBKDF2Rounds -> pbkdf2Rounds, ERR_CODE -> err_code.
func lowerFirst(s string) string {
	r := []rune(s)
	n := 0
	for n < len(r) && (unicode.IsUpper(r[n]) || n > 0 && unicode.IsDigit(r[n])) {
		n++
	}
	switch {
	case !slices.ContainsFunc(r, unicode.IsLower):
		n = len(r)
	case n > 1 && !slices.Contains(initialisms, string(r[:n])):
		n = leadingInitialism(r[:n])
	}
	for i := range n {
		r[i] = unicode.ToLower(r[i])
	}
	return string(r)
}

// leadingInitialism is the length of the longest initialism that starts run, or all of the
// run but its last letter, which then begins the next word.
func leadingInitialism(run []rune) int {
	best := 0
	for _, in := range initialisms {
		if len(in) < len(run) && strings.HasPrefix(string(run), in) {
			best = max(best, len(in))
		}
	}
	if best == 0 {
		return len(run) - 1
	}
	return best
}
