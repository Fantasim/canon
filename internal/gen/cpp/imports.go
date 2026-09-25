package cppgen

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// typeName is a type's C++ name, qualified when it is imported (CODEGEN.md §2.8, §3.3).
func (g *gen) typeName(t ir.Type) string {
	pkg, name := g.named(t)
	if name == "" {
		// every caller passes a record, enum or variant; a kind without its declaration is validate's.
		g.malformed(fmt.Sprintf(typeFormat, t), g.at)
		return cppInvalid
	}
	g.noteEnum(t)
	return g.qualifier(pkg) + name
}

// named is a type's package and unqualified C++ name, its @cpp(name:) when it has one
// (ir's CppNamePlan.TypeName, pure in its argument: it works for any package's type).
func (g *gen) named(t ir.Type) (pkg, name string) {
	switch x := t.(type) {
	case *ir.Record:
		return x.Pkg, g.pl.TypeName(x)
	case *ir.Enum:
		return x.Pkg, g.pl.TypeName(x)
	case *ir.Variant:
		return x.Pkg, g.pl.TypeName(x)
	default:
		return "", ""
	}
}

// qualifier is `<namespace>::` of an imported package's cpp emit, "" for this package; it
// records the header the generated header then includes.
func (g *gen) qualifier(pkg string) string {
	if pkg == g.p.Name {
		return ""
	}
	i := slices.IndexFunc(g.p.Imports, func(r *ir.PackageRef) bool { return r.Name == pkg })
	if i < 0 {
		g.fail(fmt.Errorf("%w: %s uses package %s, which is not imported", ErrMalformed, g.at, pkg))
		return ""
	}
	e := cppEmitOf(g.p.Imports[i].Emits)
	if e == nil || e.Namespace == "" {
		g.fail(fmt.Errorf("%w: package %s has no cpp emit", ErrMalformed, pkg))
		return ""
	}
	segs := strings.Split(pkg, qnameSep)
	g.imported[path.Join(relPath(g.emit.Dir, e.Dir), segs[len(segs)-1]+genHeaderSuffix)] = true
	return e.Namespace + scopeSep
}

func cppEmitOf(emits []*ir.Emit) *ir.Emit {
	for _, e := range emits {
		if e.Target == ir.TargetCpp {
			return e
		}
	}
	return nil
}

// relPath is the `/` path from directory from to directory to, both project-relative.
func relPath(from, to string) string {
	f, t := strings.Split(path.Clean(from), pathSep), strings.Split(path.Clean(to), pathSep)
	i := 0
	for i < len(f) && i < len(t) && f[i] == t[i] {
		i++
	}
	var parts []string
	for range f[i:] {
		parts = append(parts, parentDir)
	}
	return path.Join(append(parts, t[i:]...)...)
}

// importIncludes are the headers of the imported packages the header uses, in byte order.
func (g *gen) importIncludes() []string {
	var out []string
	for h := range g.imported { //canon:unordered sorted below
		out = append(out, fmt.Sprintf(includeQuotedFormat, h))
	}
	slices.Sort(out)
	return out
}

// decodeFunc is the decoder of a record or variant: its package's detail::Decode (§2.8, §7.2).
func (g *gen) decodeFunc(t ir.TypeRef) string {
	pkg, _ := g.named(t.Named)
	if pkg == g.p.Name {
		return decodeFunc
	}
	return g.qualifier(pkg) + detailPrefix + decodeFunc
}
