package lint

import (
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// clash is the names of one enum or constant that a header #defines and no guard covers.
type clash struct {
	decl  string
	enum  bool
	names []string
	files []string
}

// textDefines reports W0008 (API.md L9): once per enum or constant of pkg whose C++ names a header of a @text
// output, or a table of load.defines, #defines while the package's guard set lacks them.
func textDefines(p *pass, pkg *check.Package, bag *diag.Bag) {
	ip := p.irOf(pkg.Path)
	if ip == nil {
		return
	}
	declared := ir.CppDeclaredNames(ip)
	if len(declared) == 0 {
		return
	}
	defs := p.headerDefines()
	guard := ir.CppGuardNames(ip)
	var clashes []*clash
	for _, d := range declared {
		files := defs[d.Name]
		if len(files) == 0 || slices.Contains(guard, d.Name) {
			continue
		}
		c := clashOf(&clashes, d)
		c.names = append(c.names, d.Name)
		c.files = append(c.files, files...)
	}
	for _, c := range clashes {
		if span, ok := p.declSpan(pkg, c); ok {
			slices.Sort(c.files)
			diag.W0008.At(span, c.decl, int64(len(c.names)), c.files[0], guardPrefix(c.names)).Report(bag)
		}
	}
}

// clashOf is the clash d belongs to in clashes, which gains one for a declaration not yet there.
func clashOf(clashes *[]*clash, d ir.CppDeclared) *clash {
	decl, isEnum := declOf(d)
	for _, c := range *clashes {
		if c.decl == decl && c.enum == isEnum {
			return c
		}
	}
	c := &clash{decl: decl, enum: isEnum}
	*clashes = append(*clashes, c)
	return c
}

// declOf is the name of the enum or constant d is declared for, and whether it is an enum.
func declOf(d ir.CppDeclared) (string, bool) {
	if d.Enum != nil {
		return d.Enum.Name, true
	}
	return d.Const.Name, false
}

// guardPrefix is the `@cpp(defines:)` prefix that guards names: their longest common prefix cut after its last underscore.
func guardPrefix(names []string) string {
	prefix := names[0]
	for _, n := range names[1:] {
		for !strings.HasPrefix(n, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix[:strings.LastIndexByte(prefix, prefixEnd)+1]
}

// declSpan is the span of the name of the sound enum or constant c is the clash of in pkg.
func (p *pass) declSpan(pkg *check.Package, c *clash) (source.Span, bool) {
	for _, f := range sources(pkg) {
		for _, d := range f.Decls {
			if name := declName(d, c.enum); name != nil && name.Name == c.decl && p.declared(name) != nil {
				return f.Span(name), true
			}
		}
	}
	return source.Span{}, false
}

// declName is the name of d when it is an enum, or a constant when enum is false, else nil.
func declName(d syntax.Decl, enum bool) *syntax.Ident {
	switch d := d.(type) {
	case *syntax.EnumDecl:
		if enum {
			return d.Name
		}
	case *syntax.ConstDecl:
		if !enum {
			return d.Name
		}
	}
	return nil
}

// irOf is the IR of the package named name, nil when the analysis built none.
func (p *pass) irOf(name string) *ir.Package {
	i := slices.IndexFunc(p.a.IR(), func(ip *ir.Package) bool { return ip.Name == name })
	if i < 0 {
		return nil
	}
	return p.a.IR()[i]
}

// headerDefines are the macros the analysis's headers define, each with the files that define it: the object-like
// #defines of every `*.h`, `*.hh`, `*.hpp` or `*.hxx` that a @text output of a package with an `emit text` writes
// (WIRE.md 6.8's reading), and the names of every load.defines table. Made on first use.
func (p *pass) headerDefines() map[string][]string {
	if p.defines != nil {
		return p.defines
	}
	p.defines = map[string][]string{}
	for _, ip := range p.a.IR() {
		p.textHeaders(ip)
	}
	for _, cp := range p.prog.Packages {
		p.defineTables(cp)
	}
	return p.defines
}

// define records that file #defines name.
func (p *pass) define(name, file string) {
	if !slices.Contains(p.defines[name], file) {
		p.defines[name] = append(p.defines[name], file)
	}
}

// textHeaders records the defines of the headers the @text fns of ip write, when ip has an `emit text`.
func (p *pass) textHeaders(ip *ir.Package) {
	cp := packageNamed(p.prog, ip.Name)
	var dirs []string
	for _, e := range ip.Emits {
		if e.Target == ir.TargetText {
			dirs = append(dirs, e.Dir)
		}
	}
	if cp == nil || len(dirs) == 0 {
		return
	}
	files, err := ir.TextFiles(cp, ip)
	if err != nil {
		return
	}
	for _, f := range files {
		if !slices.Contains(headerExts, path.Ext(f.Path)) {
			continue
		}
		names := load.DefineNames(f.Content)
		for _, dir := range dirs {
			p.defineAll(names, path.Join(dir, f.Path))
		}
	}
}

// defineAll records that file #defines each of names.
func (p *pass) defineAll(names []string, file string) {
	for _, name := range names {
		p.define(name, file)
	}
}

// defineTables records the names of the `table Define` lets of cp, each with the header that defined it.
func (p *pass) defineTables(cp *check.Package) {
	for _, f := range sources(cp) {
		for _, d := range f.Decls {
			let, ok := d.(*syntax.LetDecl)
			if !ok || let.Name == nil || !isDefineTable(p.declared(let.Name)) {
				continue
			}
			v, ok := p.a.Loaded(p.ctx, eval.Root{Pkg: cp.Path, Name: let.Name.Name})
			if tbl, isTable := v.(*value.Table); ok && isTable {
				p.tableDefines(tbl)
			}
		}
	}
}

// isDefineTable reports t, the type `table Define` that load.defines gives (WIRE.md §6.8).
func isDefineTable(t types.Type) bool {
	if t == nil {
		return false
	}
	tt, ok := t.Base().(*types.TableType)
	return ok && tt.Elem == types.DefineType
}

// tableDefines records the entries of tbl: the define named by an entry's key, in the file its provenance names.
func (p *pass) tableDefines(tbl *value.Table) {
	for _, rec := range tbl.Entries {
		if rec.Ident != nil && rec.P != nil {
			p.define(rec.Ident.Key.S, p.a.Files().Path(rec.P.Span.File))
		}
	}
}
