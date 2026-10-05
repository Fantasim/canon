package build

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// declared is a name a declaration introduces, and the kind it declares.
type declared struct {
	id   *syntax.Ident
	kind diag.Kind
}

// namers lists the names each kind of node declares (GRAMMAR.md §9.2).
var namers = map[syntax.NodeKind]func(syntax.Node) []declared{
	syntax.KindFile:        fileNames,
	syntax.KindImport:      func(n syntax.Node) []declared { return one(n.(*syntax.Import).Alias, diag.KindImportAlias) },
	syntax.KindConstDecl:   func(n syntax.Node) []declared { return one(n.(*syntax.ConstDecl).Name, diag.KindConst) },
	syntax.KindLetDecl:     func(n syntax.Node) []declared { return one(n.(*syntax.LetDecl).Name, diag.KindLet) },
	syntax.KindTypeDecl:    typeAliasNames,
	syntax.KindRecordDecl:  recordNames,
	syntax.KindEnumDecl:    func(n syntax.Node) []declared { return one(n.(*syntax.EnumDecl).Name, diag.KindEnum) },
	syntax.KindVariantDecl: func(n syntax.Node) []declared { return one(n.(*syntax.VariantDecl).Name, diag.KindVariant) },
	syntax.KindFieldDecl:   func(n syntax.Node) []declared { return one(n.(*syntax.FieldDecl).Name, diag.KindField) },
	syntax.KindFnDecl:      fnNames,
	syntax.KindWidgetDecl:  widgetNames,
	syntax.KindLetStmt:     func(n syntax.Node) []declared { return one(n.(*syntax.LetStmt).Name, diag.KindLocal) },
	syntax.KindVarStmt:     func(n syntax.Node) []declared { return one(n.(*syntax.VarStmt).Name, diag.KindLocal) },
	syntax.KindForStmt:     func(n syntax.Node) []declared { return many(n.(*syntax.ForStmt).Vars, diag.KindVariable) },
	syntax.KindPattern:     func(n syntax.Node) []declared { return one(n.(*syntax.Pattern).Binder, diag.KindVariable) },
	syntax.KindDepMapType:  func(n syntax.Node) []declared { return one(n.(*syntax.DepMapType).Var, diag.KindVariable) },
	syntax.KindCompClause:  clauseNames,
	syntax.KindLambdaExpr:  func(n syntax.Node) []declared { return many(n.(*syntax.LambdaExpr).Params, diag.KindVariable) },
}

// conventions are the compiled patterns of conventionPatterns.
var conventions = compilePatterns()

func compilePatterns() map[diag.Kind]*regexp.Regexp {
	out := make(map[diag.Kind]*regexp.Regexp, len(conventionPatterns))
	for k, p := range conventionPatterns { //canon:unordered builds a lookup table
		out[k] = regexp.MustCompile(p)
	}
	return out
}

func one(id *syntax.Ident, kind diag.Kind) []declared {
	return many([]*syntax.Ident{id}, kind)
}

func many(ids []*syntax.Ident, kind diag.Kind) []declared {
	var out []declared
	for _, id := range ids {
		if id != nil && id.Name != syntax.Blank {
			out = append(out, declared{id, kind})
		}
	}
	return out
}

func fileNames(n syntax.Node) []declared {
	if p := n.(*syntax.File).Package; p != nil {
		return many(p.Parts, diag.KindPackageSegment)
	}
	return nil
}

func typeAliasNames(n syntax.Node) []declared {
	d := n.(*syntax.TypeDecl)
	return append(one(d.Name, diag.KindTypeAlias), params(d.Params, diag.KindTypeParameter)...)
}

func recordNames(n syntax.Node) []declared {
	d := n.(*syntax.RecordDecl)
	return append(one(d.Name, diag.KindRecord), params(d.Params, diag.KindTypeParameter)...)
}

func fnNames(n syntax.Node) []declared {
	d := n.(*syntax.FnDecl)
	kind := diag.KindFunction
	if d.Self.Valid() {
		kind = diag.KindMethod
	}
	return append(one(d.Name, kind), params(d.Params, diag.KindParameter)...)
}

func widgetNames(n syntax.Node) []declared {
	d := n.(*syntax.WidgetDecl)
	return append(one(d.Name, diag.KindWidget), params(d.Params, diag.KindParameter)...)
}

// clauseNames is a comprehension's `for` variables, or its `let` name.
func clauseNames(n syntax.Node) []declared {
	c := n.(*syntax.CompClause)
	if c.Keyword == syntax.KwLet {
		return many(c.Vars, diag.KindLocal)
	}
	if c.Keyword == syntax.KwFor {
		return many(c.Vars, diag.KindVariable)
	}
	return nil
}

func params(ps []*syntax.Param, kind diag.Kind) []declared {
	var out []declared
	for _, p := range ps {
		out = append(out, one(p.Name, kind)...)
	}
	return out
}

// checkNames is W1003 over the selected packages (GRAMMAR.md §9.2).
func (r *run) checkNames() {
	for _, cp := range r.cps {
		bag := r.bags[cp.Path]
		for _, f := range cp.Files {
			checkFileNames(f, bag)
		}
	}
}

func checkFileNames(f *syntax.File, bag *diag.Bag) {
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n == nil {
			return true
		}
		if namer, ok := namers[n.Kind()]; ok {
			for _, d := range namer(n) {
				reportName(f, d, bag)
			}
		}
		return true
	})
}

func reportName(f *syntax.File, d declared, bag *diag.Bag) {
	conv := conventionOf[d.kind]
	if conventions[conv].MatchString(d.id.Name) {
		return
	}
	diag.W1003.At(f.Span(d.id), d.kind, d.id.Name, conv).Report(bag)
}
