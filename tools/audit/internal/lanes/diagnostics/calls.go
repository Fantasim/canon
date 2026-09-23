package diagnostics

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// typedFile is one judged file as the type-checked load parsed it.
type typedFile struct {
	pkg  *packages.Package
	rel  string
	seen map[string]bool
}

// diagCalls reports, from the type-checked load, what only types can see: fmt or English in
// the arguments of any diag call (a stored builder included), a diag record built by hand,
// and a Finding's message assigned. A module that does not type-check skips it.
func (s *scan) diagCalls() *lane.Skip {
	pkgs, err := s.ctx.Go.TypedClean()
	if err != nil {
		return &lane.Skip{What: ruleInline, Reason: err.Error()}
	}
	judged := map[string]string{}
	for _, f := range s.files {
		judged[filepath.Clean(f.Abs)] = f.Path
	}
	seen := map[string]bool{}
	for _, p := range pkgs {
		for _, file := range p.Syntax {
			rel, ok := judged[filepath.Clean(p.Fset.Position(file.Pos()).Filename)]
			if !ok || p.TypesInfo == nil {
				continue
			}
			tf := typedFile{pkg: p, rel: rel, seen: seen}
			ast.Inspect(file, func(n ast.Node) bool { s.typedNode(tf, n); return true })
		}
	}
	return nil
}

func (s *scan) typedNode(tf typedFile, n ast.Node) {
	switch n := n.(type) {
	case *ast.CallExpr:
		if fn := gosrc.CalledFunc(tf.pkg.TypesInfo, n); fn != nil && s.fromDiag(fn) {
			for _, a := range n.Args {
				s.argument(tf, a)
			}
		}
	case *ast.CompositeLit:
		s.record(tf, n)
	case *ast.AssignStmt:
		for _, lhs := range n.Lhs {
			if sel, ok := lhs.(*ast.SelectorExpr); ok && s.messageField(tf.pkg.TypesInfo, sel) {
				s.typedEmit(tf, lhs.Pos(), detailMessage, fmt.Sprintf(msgMessage, messageField))
			}
		}
	}
}

// record reports a composite literal of a diag type only diag constructs.
func (s *scan) record(tf typedFile, lit *ast.CompositeLit) {
	if name, ok := s.diagType(tf.pkg.TypesInfo.TypeOf(lit)); ok && slices.Contains(handBuilt, name) {
		s.typedEmit(tf, lit.Pos(), detailBuilt+name, fmt.Sprintf(msgBuilt, name))
	}
}

// argument reports a fmt call or a literal holding whitespace anywhere in a diag call's
// argument, outside function literals: a name, a span or a value never has either.
func (s *scan) argument(tf typedFile, arg ast.Expr) {
	ast.Inspect(arg, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if fn := gosrc.CalledFunc(tf.pkg.TypesInfo, n); fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == gosrc.PkgFmt {
				s.typedEmit(tf, n.Pos(), detailFormatted, fmt.Sprintf(msgFormatted, fn.Name()))
			}
		case *ast.BasicLit:
			if v, ok := foldString(n); ok && strings.ContainsFunc(v, unicode.IsSpace) {
				s.typedEmit(tf, n.Pos(), detailPhrase, fmt.Sprintf(msgPhrase, v))
			}
		}
		return true
	})
}

func (s *scan) fromDiag(obj types.Object) bool {
	return obj.Pkg() != nil && obj.Pkg().Path() == s.diagPath
}

// diagType names a (pointer to a) named type of the diag package.
func (s *scan) diagType(t types.Type) (string, bool) {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || !s.fromDiag(named.Obj()) {
		return "", false
	}
	return named.Obj().Name(), true
}

// messageField reports a selector naming the rendered-message field of a diag type.
func (s *scan) messageField(info *types.Info, sel *ast.SelectorExpr) bool {
	selection := info.Selections[sel]
	return selection != nil && selection.Kind() == types.FieldVal &&
		s.fromDiag(selection.Obj()) && selection.Obj().Name() == messageField
}

// typedEmit reports once per position: a file belongs to several test variants of a package.
func (s *scan) typedEmit(tf typedFile, pos token.Pos, detail, msg string) {
	line := tf.pkg.Fset.Position(pos).Line
	key := fmt.Sprint(tf.rel, line, detail, msg)
	if tf.seen[key] {
		return
	}
	tf.seen[key] = true
	s.emit(finding.Finding{Rule: ruleInline, File: tf.rel, Line: line, Detail: detail, Message: msg})
}
