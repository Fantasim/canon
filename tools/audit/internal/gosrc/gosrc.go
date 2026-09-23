package gosrc

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

type File struct {
	Path string
	Abs  string
	Dir  string
	AST  *ast.File
	Src  []byte
	Test bool
	// Harness is a non-test file of an e2e harness or a *stub package: test code all the same.
	Harness   bool
	Generated bool
	decls     []span
}

// TestCode reports a _test.go file or a harness file.
func (f *File) TestCode() bool { return f.Test || f.Harness }

// harness reports a dir under an e2e/ directory or a package named *stub: a stand-in for a
// third party, or the driver of an end-to-end test.
func harness(dir string) bool {
	return slices.ContainsFunc(strings.Split(dir, pathSep), func(seg string) bool {
		return seg == e2eDirName || strings.HasSuffix(seg, stubSuffix)
	})
}

type span struct {
	from, to int
	name     string
}

type Tree struct {
	Root  string
	Fset  *token.FileSet
	Files []*File

	byPath    map[string]*File
	typedOnce sync.Once
	typed     []*packages.Package
	typedErr  error
}

// Parse reads the given repo-relative .go files under root. Unparsable files are skipped
// and returned as errors so the caller can report them without losing the rest.
func Parse(root string, rel []string) (*Tree, []error) {
	t := &Tree{Root: root, Fset: token.NewFileSet(), byPath: map[string]*File{}}
	var errs []error
	for _, p := range rel {
		abs := root + "/" + p
		src, err := os.ReadFile(abs)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		f, err := parser.ParseFile(t.Fset, abs, src, parser.ParseComments)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p, err))
			continue
		}
		gf := &File{
			Path: p, Abs: abs, Dir: path.Dir(p), AST: f, Src: src,
			Test: strings.HasSuffix(p, testSuffix), Harness: harness(path.Dir(p)),
			Generated: ast.IsGenerated(f) || strings.HasSuffix(p, genSuffix),
		}
		gf.decls = t.index(f)
		t.Files = append(t.Files, gf)
		t.byPath[p] = gf
	}
	return t, errs
}

func (t *Tree) File(rel string) (*File, bool) {
	f, ok := t.byPath[rel]
	return f, ok
}

// Line is the 1-based line of pos.
func (t *Tree) Line(pos token.Pos) int { return t.Fset.Position(pos).Line }

// Enclosing names the top-level declaration covering line: "Func", "Type.Method",
// "type T", "const X", "var X"; "" outside every declaration.
func (t *Tree) Enclosing(rel string, line int) string {
	f, ok := t.byPath[rel]
	if !ok {
		return ""
	}
	i := sort.Search(len(f.decls), func(i int) bool { return f.decls[i].to >= line })
	if i < len(f.decls) && f.decls[i].from <= line {
		return f.decls[i].name
	}
	return ""
}

// CommentLines is rel's comment text only, each comment on its source line and every other
// line blank, so line numbers still match the file.
func (t *Tree) CommentLines(rel string) string {
	f, ok := t.byPath[rel]
	if !ok {
		return ""
	}
	lines := make([]string, t.Line(f.AST.End())+1)
	for _, g := range f.AST.Comments {
		t.fillCommentGroup(lines, g)
	}
	return strings.Join(lines, "\n")
}

// fillCommentGroup writes g's comment text into lines, one physical line per source line.
func (t *Tree) fillCommentGroup(lines []string, g *ast.CommentGroup) {
	for _, c := range g.List {
		first := t.Line(c.Pos())
		for i, l := range strings.Split(c.Text, "\n") {
			if first+i-1 < len(lines) {
				lines[first+i-1] = l
			}
		}
	}
}

// Packages groups the non-generated files by directory.
func (t *Tree) Packages() map[string][]*File {
	out := map[string][]*File{}
	for _, f := range t.Files {
		if !f.Generated {
			out[f.Dir] = append(out[f.Dir], f)
		}
	}
	return out
}

// Typed loads the module with full type information, tests included, once. "./..." also
// matches a vendored Go source tree (e.g. an npm package under node_modules/ that ships Go
// sources); those packages are dropped the same way the file walk drops them, by repo.Skipped.
func (t *Tree) Typed() ([]*packages.Package, error) {
	t.typedOnce.Do(func() {
		cfg := &packages.Config{Mode: typedMode, Dir: t.Root, Tests: true, Fset: token.NewFileSet()}
		pkgs, err := packages.Load(cfg, "./...")
		t.typed, t.typedErr = t.dropSkipped(pkgs), err
	})
	return t.typed, t.typedErr
}

// TypedClean is Typed, failing when the load or any loaded package has an error: a rule
// that must not guess on a module that does not type-check skips instead.
func (t *Tree) TypedClean() ([]*packages.Package, error) {
	pkgs, err := t.Typed()
	if err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("%w: %d, first: %w", errTyped, n, first)
	}
	return pkgs, nil
}

// CalledFunc is the function or method a call statically names, nil for a func value.
func CalledFunc(info *types.Info, c *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.SelectorExpr:
		id = fun.Sel
	case *ast.Ident:
		id = fun
	default:
		return nil
	}
	fn, _ := info.Uses[id].(*types.Func)
	return fn
}

// dropSkipped removes every loaded package whose source files sit under a repo.Skipped path.
func (t *Tree) dropSkipped(pkgs []*packages.Package) []*packages.Package {
	out := pkgs[:0]
	for _, p := range pkgs {
		if !t.pkgSkipped(p) {
			out = append(out, p)
		}
	}
	return out
}

// pkgSkipped reports a package with at least one file and every file under a skipped path.
func (t *Tree) pkgSkipped(p *packages.Package) bool {
	files := p.GoFiles
	if len(files) == 0 {
		files = p.CompiledGoFiles
	}
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !repo.Skipped(t.relPath(f)) {
			return false
		}
	}
	return true
}

func (t *Tree) relPath(abs string) string {
	rel, err := filepath.Rel(t.Root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

// DeclName is the symbol a declaration is known by in findings and baselines.
func DeclName(d ast.Decl) string {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil && len(d.Recv.List) > 0 {
			return recvName(d.Recv.List[0].Type) + "." + d.Name.Name
		}
		return d.Name.Name
	case *ast.GenDecl:
		if len(d.Specs) == 0 {
			return ""
		}
		return d.Tok.String() + " " + specName(d.Specs[0])
	}
	return ""
}

// Lines counts the lines a node spans.
func (t *Tree) Lines(n ast.Node) int {
	return t.Fset.Position(n.End()).Line - t.Fset.Position(n.Pos()).Line + 1
}

// IsBlank reports whether line i (0-based) of src is blank.
func IsBlank(line []byte) bool { return len(bytes.TrimSpace(line)) == 0 }

func (t *Tree) index(f *ast.File) []span {
	var out []span
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			continue
		}
		from := t.Fset.Position(d.Pos()).Line
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Doc != nil {
			from = t.Fset.Position(fd.Doc.Pos()).Line
		}
		if g, ok := d.(*ast.GenDecl); ok && g.Doc != nil {
			from = t.Fset.Position(g.Doc.Pos()).Line
		}
		if g, ok := d.(*ast.GenDecl); ok && g.Lparen.IsValid() {
			out = append(out, t.specSpans(g)...)
			continue
		}
		out = append(out, span{from: from, to: t.Fset.Position(d.End()).Line, name: DeclName(d)})
	}
	return out
}

// specSpans names each spec of a grouped const/var/type block on its own.
func (t *Tree) specSpans(g *ast.GenDecl) []span {
	out := make([]span, 0, len(g.Specs))
	for _, sp := range g.Specs {
		out = append(out, span{
			from: t.Fset.Position(sp.Pos()).Line, to: t.Fset.Position(sp.End()).Line,
			name: g.Tok.String() + " " + specName(sp),
		})
	}
	return out
}

func recvName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return recvName(x.X)
	case *ast.IndexExpr:
		return recvName(x.X)
	case *ast.IndexListExpr:
		return recvName(x.X)
	case *ast.Ident:
		return x.Name
	}
	return ""
}

func specName(s ast.Spec) string {
	switch s := s.(type) {
	case *ast.TypeSpec:
		return s.Name.Name
	case *ast.ValueSpec:
		if len(s.Names) > 0 {
			return s.Names[0].Name
		}
	}
	return ""
}
