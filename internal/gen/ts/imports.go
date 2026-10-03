package tsgen

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// importType records a type name of another package's ts emit and returns it (CODEGEN.md §2.8: `import type`).
func (g *gen) importType(pkg, name string) string {
	return g.importName(g.typeImports, pkg, name)
}

// importValue records a value name (a const or function) of another package's ts emit and returns it.
func (g *gen) importValue(pkg, name string) string {
	return g.importName(g.valueImports, pkg, name)
}

// importName adds name to the imports of pkg's file; the name must not collide with another name of the module.
func (g *gen) importName(set map[string]map[string]bool, pkg, name string) string {
	spec, ok := g.specifier(pkg)
	if !ok {
		return name
	}
	if set[spec] == nil {
		set[spec] = map[string]bool{}
	}
	if !set[spec][name] {
		set[spec][name] = true
		g.declare(name, fmt.Sprintf(importOrigin, spec))
	}
	return name
}

// specifier is the relative module path of pkg's ts emit: `.ts` replaced by `.js`, always starting with `./` or `../` (CODEGEN.md §2.8).
func (g *gen) specifier(pkg string) (string, bool) {
	o := g.tsEmit(pkg)
	if o == nil {
		g.failf(ErrMalformed, malformedNoImport, pkg)
		return "", false
	}
	rel := relPath(g.e.Dir, o.Dir)
	file := strings.TrimSuffix(o.FileName, path.Ext(o.FileName)) + jsExt
	spec := path.Join(rel, file)
	if !strings.HasPrefix(spec, parentDir) {
		spec = currentDir + spec
	}
	return spec, true
}

// tsEmit is the ts emit of an imported package, the copy this one uses (ir.CopyOf narrowed it).
func (g *gen) tsEmit(pkg string) *ir.Emit {
	for _, ref := range g.p.Imports {
		if ref.Name != pkg {
			continue
		}
		for _, e := range ref.Emits {
			if e.Target == ir.TargetTS {
				return e
			}
		}
	}
	return nil
}

// foreignHasIDs reports an imported package whose ts emit declares table id types: every mode but types (CODEGEN.md §5.3).
func (g *gen) foreignHasIDs(pkg string) bool {
	o := g.tsEmit(pkg)
	return o != nil && o.Mode != ir.ModeTypes
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

// importLines are the import declarations: per specifier in byte order, the type import then the value import, names sorted.
func (g *gen) importLines() []string {
	specs := map[string]bool{}
	for s := range g.typeImports { //canon:unordered a set, sorted below
		specs[s] = true
	}
	for s := range g.valueImports { //canon:unordered a set, sorted below
		specs[s] = true
	}
	var out []string
	for _, spec := range slices.Sorted(maps.Keys(specs)) {
		if names := g.typeImports[spec]; len(names) > 0 {
			out = append(out, fmt.Sprintf(importTypeFormat, joinSorted(names), spec))
		}
		if names := g.valueImports[spec]; len(names) > 0 {
			out = append(out, fmt.Sprintf(importValueFormat, joinSorted(names), spec))
		}
	}
	return out
}

func joinSorted(set map[string]bool) string {
	return strings.Join(slices.Sorted(maps.Keys(set)), listSep)
}
