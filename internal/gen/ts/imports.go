package tsgen

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// nsImport is the one namespace import of another package's ts emit: its alias, and whether the file uses a value through it (`import * as`) or types only (`import type * as`).
type nsImport struct {
	alias string
	value bool
}

// importType is a type name of another package's ts emit, qualified through that package's namespace import (CODEGEN.md §2.8; log-2026-10-06 "U4 (gen/ts) done" (a)).
func (g *gen) importType(pkg, name string) string {
	return g.qualify(pkg, name, false)
}

// importValue is a value name (a const or function) of another package's ts emit, qualified the same way; the import then brings values.
func (g *gen) importValue(pkg, name string) string {
	return g.qualify(pkg, name, true)
}

// qualify is alias.name, pkg's namespace import created on first use, so the file imports only what it writes (noUnusedLocals): the alias is the plan's (ir.TSImportAlias), the module's one name for pkg; imported names never enter its scope.
func (g *gen) qualify(pkg, name string, value bool) string {
	spec, ok := g.specifier(pkg)
	if !ok {
		return name
	}
	imp := g.imports[spec]
	if imp == nil {
		imp = &nsImport{alias: g.declare(ir.TSImportAlias(pkg), fmt.Sprintf(importOrigin, spec))}
		g.imports[spec] = imp
	}
	imp.value = imp.value || value
	return imp.alias + dot + name
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

// importLines are the import declarations, one namespace import per package in specifier byte order: `import * as` when the file uses a value of it, else `import type * as`.
func (g *gen) importLines() []string {
	var out []string
	for _, spec := range slices.Sorted(maps.Keys(g.imports)) {
		imp := g.imports[spec]
		format := importTypeFormat
		if imp.value {
			format = importValueFormat
		}
		out = append(out, fmt.Sprintf(format, imp.alias, spec))
	}
	return out
}
