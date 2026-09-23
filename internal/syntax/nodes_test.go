package syntax_test

import (
	"math/big"

	"github.com/fantasim/canonlang/internal/syntax"
)

// gen hands out leaf nodes on increasing tokens, so a node built with its fields in source
// order has children whose first tokens increase.
type gen struct{ tok syntax.Tok }

// sample is a node with every child field set, and how many children that makes.
type sample struct {
	node syntax.Node
	kids int
}

func (g *gen) b() syntax.Bounds {
	t := g.tok
	g.tok++
	return syntax.Bounds{From: t, To: t}
}

func (g *gen) id() *syntax.Ident         { return &syntax.Ident{Bounds: g.b(), Name: "a"} }
func (g *gen) x() syntax.Expr            { return &syntax.IdentExpr{Bounds: g.b(), Name: "x"} }
func (g *gen) ty() syntax.Type           { return &syntax.AnyType{Bounds: g.b()} }
func (g *gen) str() *syntax.StringLit    { return &syntax.StringLit{Bounds: g.b()} }
func (g *gen) doc() *syntax.DocComment   { return &syntax.DocComment{Bounds: g.b(), Text: "d"} }
func (g *gen) ann() *syntax.Annotation   { return &syntax.Annotation{Bounds: g.b()} }
func (g *gen) mods() *syntax.Modifiers   { return &syntax.Modifiers{Bounds: g.b(), Local: g.tok} }
func (g *gen) qn() *syntax.QualifiedName { return &syntax.QualifiedName{Bounds: g.b()} }
func (g *gen) int() *syntax.IntLit       { return &syntax.IntLit{Bounds: g.b(), Value: big.NewInt(1)} }
func (g *gen) brace() *syntax.BraceLit   { return &syntax.BraceLit{Bounds: g.b()} }
func (g *gen) block() *syntax.Block      { return &syntax.Block{Bounds: g.b()} }
func (g *gen) pat() *syntax.Pattern {
	return &syntax.Pattern{Bounds: g.b(), Keyword: syntax.TokUnderscore}
}
func (g *gen) param() *syntax.Param { return &syntax.Param{Bounds: g.b()} }
func (g *gen) arg() *syntax.Arg     { return &syntax.Arg{Bounds: g.b()} }
func (g *gen) clause() *syntax.CompClause {
	return &syntax.CompClause{Bounds: g.b(), Keyword: syntax.KwIf}
}
func (g *gen) targs() *syntax.TypeArgs     { return &syntax.TypeArgs{Bounds: g.b()} }
func (g *gen) body() *syntax.RecordBody    { return &syntax.RecordBody{Bounds: g.b()} }
func (g *gen) entry() *syntax.ProjectEntry { return &syntax.ProjectEntry{Bounds: g.b(), Colon: g.tok} }
func (g *gen) seg() *syntax.AmendSegment   { return &syntax.AmendSegment{Bounds: g.b()} }
func (g *gen) member() *syntax.EnumMember  { return &syntax.EnumMember{Bounds: g.b()} }
func (g *gen) show() *syntax.ViewShow      { return &syntax.ViewShow{Bounds: g.b()} }
func (g *gen) field() *syntax.FieldDecl    { return &syntax.FieldDecl{Bounds: g.b(), Input: syntax.NoTok} }
func (g *gen) vcase() *syntax.VariantCase  { return &syntax.VariantCase{Bounds: g.b()} }
func (g *gen) amend() *syntax.Amendment    { return &syntax.Amendment{Bounds: g.b()} }
func (g *gen) tentry() *syntax.TranslationEntry {
	return &syntax.TranslationEntry{Bounds: g.b()}
}

// samples builds one sample of every node type.
func samples() []sample {
	g := &gen{}
	var all []sample
	for _, part := range []func(*gen) []sample{
		sharedSamples, declSamples, recordSamples, fileSamples, typeSamples,
		exprSamples, exprLitSamples, stmtSamples, viewSamples,
	} {
		all = append(all, part(g)...)
	}
	return all
}

func sharedSamples(g *gen) []sample {
	return []sample{
		{&syntax.File{
			FileKind: syntax.FileSource, Doc: g.doc(), Package: g.qn(),
			Imports: []*syntax.Import{{Bounds: g.b()}}, Decls: []syntax.Decl{&syntax.ConstDecl{Bounds: g.b()}},
			Layer: g.id(), Amends: []*syntax.AmendBlock{{Bounds: g.b()}}, Lang: g.id(),
			Entries: []*syntax.TranslationEntry{g.tentry()}, Project: &syntax.ProjectDecl{Bounds: g.b()},
		}, 9},
		{g.id(), 0},
		{&syntax.QualifiedName{Parts: []*syntax.Ident{g.id(), g.id()}}, 2},
		{g.doc(), 0},
		{&syntax.Annotation{Name: g.id(), Parens: syntax.Delims{Open: g.tok, Close: g.tok}, Args: []*syntax.AnnotationArg{{Bounds: g.b()}}}, 2},
		{&syntax.AnnotationArg{Name: g.id(), Value: &syntax.BoolLit{Bounds: g.b(), Value: true}}, 2},
		{&syntax.AnnotationList{Items: []syntax.AnnValue{&syntax.BoolLit{Bounds: g.b()}, g.qn()}}, 2},
		{&syntax.Modifiers{Local: syntax.NoTok, Export: g.tok, Retired: syntax.NoTok}, 0},
	}
}

func declSamples(g *gen) []sample {
	return []sample{
		{&syntax.Import{Path: g.qn(), Alias: g.id(), Names: []*syntax.Ident{g.id(), g.id()}}, 4},
		{&syntax.ConstDecl{Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Mods: g.mods(), Name: g.id(), Value: g.x()}, 5},
		{&syntax.LetDecl{Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Mods: g.mods(), Name: g.id(), Type: g.ty(), Value: g.x()}, 6},
		{&syntax.TypeDecl{Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Mods: g.mods(), Name: g.id(), Params: []*syntax.Param{g.param()}, Type: g.ty()}, 6},
		{&syntax.Param{Name: g.id(), Type: g.ty(), Default: g.x()}, 3},
		{&syntax.FnDecl{
			Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Mods: g.mods(), Name: g.id(),
			Self: g.tok, Params: []*syntax.Param{g.param()}, Result: g.ty(), Body: g.block(),
		}, 7},
		{&syntax.EntryDecl{Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Mods: g.mods(), Table: g.id(), Key: g.int(), Value: g.brace()}, 6},
		{&syntax.CheckDecl{
			Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Keyword: syntax.KwWarn, Body: g.block(),
			Name: g.id(), Cond: g.x(), At: g.id(), Message: g.str(),
		}, 7},
		{&syntax.ViewDecl{Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Type: g.id(), Case: g.id(), Items: []syntax.ViewItem{&syntax.ViewTitle{Bounds: g.b()}}}, 5},
		{&syntax.WidgetDecl{Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Name: g.id(), Params: []*syntax.Param{g.param()}, Default: syntax.NoTok}, 4},
		{&syntax.TestDecl{Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Name: g.str(), Body: g.block()}, 4},
		{&syntax.EmitDecl{Doc: g.doc(), Annotations: []*syntax.Annotation{g.ann()}, Target: g.id(), Options: g.brace()}, 4},
	}
}

func recordSamples(g *gen) []sample {
	return []sample{
		{&syntax.RecordDecl{Doc: g.doc(), Mods: g.mods(), Name: g.id(), Params: []*syntax.Param{g.param()}, Annotations: []*syntax.Annotation{g.ann()}, Body: g.body()}, 6},
		{&syntax.RecordBody{Items: []syntax.RecordItem{g.field(), &syntax.FnDecl{Bounds: g.b()}, &syntax.CheckDecl{Bounds: g.b()}}}, 3},
		{&syntax.FieldDecl{Doc: g.doc(), Name: g.id(), Input: g.tok, Type: g.ty(), Env: g.str(), Default: g.x(), Annotations: []*syntax.Annotation{g.ann()}}, 6},
		{&syntax.EnumDecl{Doc: g.doc(), Mods: g.mods(), Name: g.id(), Ordered: syntax.NoTok, Annotations: []*syntax.Annotation{g.ann()}, Members: []*syntax.EnumMember{g.member()}}, 5},
		{&syntax.EnumMember{Doc: g.doc(), Mods: g.mods(), Name: g.id(), Value: g.int(), Annotations: []*syntax.Annotation{g.ann()}}, 5},
		{&syntax.VariantDecl{Doc: g.doc(), Mods: g.mods(), Name: g.id(), Annotations: []*syntax.Annotation{g.ann()}, Items: []syntax.VariantItem{g.vcase(), &syntax.FnDecl{Bounds: g.b()}}}, 6},
		{&syntax.VariantCase{Doc: g.doc(), Mods: g.mods(), Name: g.id(), Annotations: []*syntax.Annotation{g.ann()}, Body: g.body()}, 5},
	}
}

func fileSamples(g *gen) []sample {
	return []sample{
		{&syntax.AmendBlock{Doc: g.doc(), Target: g.id(), Items: []*syntax.Amendment{g.amend()}}, 3},
		{&syntax.Amendment{Doc: g.doc(), Path: []*syntax.AmendSegment{g.seg(), g.seg()}, Value: g.x()}, 4},
		{&syntax.AmendSegment{Name: g.id(), Key: g.x(), Position: g.int()}, 3},
		{&syntax.TranslationEntry{Doc: g.doc(), Key: g.qn(), Text: g.str()}, 3},
		{&syntax.ProjectDecl{Doc: g.doc(), Name: g.id(), Items: []*syntax.ProjectEntry{g.entry()}}, 3},
		{&syntax.ProjectEntry{Doc: g.doc(), Key: g.id(), Colon: g.tok, Value: g.int()}, 3},
		{&syntax.ProjectList{Items: []syntax.ProjectValue{g.int(), g.qn(), &syntax.RawStringLit{Bounds: g.b()}}}, 3},
		{&syntax.ProjectMap{Entries: []*syntax.ProjectEntry{g.entry(), g.entry()}}, 2},
	}
}

func typeSamples(g *gen) []sample {
	return []sample{
		{&syntax.NamedType{Name: g.qn(), Args: g.targs()}, 2},
		{&syntax.TypeArgs{Args: []syntax.Expr{&syntax.RegexLit{Bounds: g.b(), Pattern: "^a$"}, g.x()}}, 2},
		{&syntax.ListType{Elem: g.ty(), Args: g.targs()}, 2},
		{&syntax.KeyedType{List: g.ty(), Key: g.id()}, 2},
		{&syntax.MapType{Key: g.ty(), Value: g.ty(), Args: g.targs()}, 3},
		{&syntax.DepMapType{Var: g.id(), Domain: g.x(), Value: g.ty(), Args: g.targs()}, 4},
		{&syntax.TableType{Stable: g.tok, Name: g.qn()}, 1},
		{&syntax.RefType{Name: g.qn()}, 1},
		{&syntax.OptionalType{Elem: g.ty()}, 1},
		{&syntax.WhereType{Base: g.ty(), Pred: g.x()}, 2},
		{&syntax.UnionType{Alts: []syntax.Type{g.ty(), g.ty()}}, 2},
		{&syntax.LiteralType{Value: g.str()}, 1},
		{&syntax.MatchType{Scrutinee: g.x(), Arms: []*syntax.TypeArm{{Bounds: g.b()}}}, 2},
		{&syntax.TypeArm{Patterns: []*syntax.Pattern{g.pat(), g.pat()}, Type: g.ty()}, 3},
		{&syntax.AssetType{Dir: g.str(), Exts: []syntax.NameLit{g.id(), g.str()}}, 3},
		{g.ty(), 0},
		{&syntax.FnType{Params: []syntax.Type{g.ty(), g.ty()}, Result: g.ty()}, 3},
		{&syntax.ParenType{Type: g.ty()}, 1},
	}
}

func exprSamples(g *gen) []sample {
	return []sample{
		{g.x(), 0},
		{g.int(), 0},
		{&syntax.FloatLit{Bounds: g.b(), Coef: big.NewInt(15), Exp: -1}, 0},
		{&syntax.DurationLit{Bounds: g.b(), Millis: 90_000}, 0},
		{&syntax.StringLit{Parts: []syntax.StringPart{{Text: "a"}, {Interp: &syntax.Interp{Bounds: g.b()}}, {Text: "b"}}}, 1},
		{&syntax.Interp{X: g.x(), Spec: &syntax.FormatSpec{Tok: g.tok, Comma: true, Decimals: syntax.NoDecimals}}, 1},
		{&syntax.RawStringLit{Bounds: g.b(), Multiline: true, Value: `C:\x`}, 0},
		{&syntax.RegexLit{Bounds: g.b()}, 0},
		{&syntax.BoolLit{Bounds: g.b()}, 0},
		{&syntax.NoneLit{Bounds: g.b()}, 0},
		{&syntax.SelfExpr{Bounds: g.b()}, 0},
		{&syntax.ParenExpr{X: g.x()}, 1},
		{&syntax.UnaryExpr{Op: syntax.KwNot, X: g.x()}, 1},
		{&syntax.BinaryExpr{X: g.x(), Op: syntax.TokCoalesce, OpTok: g.tok, Y: g.x()}, 2},
		{&syntax.IsExpr{X: g.x(), Target: g.qn()}, 2},
		{&syntax.RangeExpr{Lo: g.x(), Op: syntax.TokRangeIncl, Hi: g.x()}, 2},
		{&syntax.SelectorExpr{X: g.x(), Optional: true, Name: g.id()}, 2},
		{&syntax.IndexExpr{X: g.x(), Index: g.x()}, 2},
		{&syntax.CallExpr{Fun: g.x(), Args: []*syntax.Arg{g.arg(), g.arg()}}, 3},
		{&syntax.Arg{Name: g.id(), Value: g.x()}, 2},
		{&syntax.ForceExpr{X: g.x()}, 1},
	}
}

func exprLitSamples(g *gen) []sample {
	return []sample{
		{&syntax.LambdaExpr{Params: []*syntax.Ident{g.id(), g.id()}, Body: g.x()}, 3},
		{&syntax.ShorthandLambda{Body: g.x()}, 1},
		{&syntax.ListLit{Elems: []syntax.Expr{g.x(), g.x()}}, 2},
		{&syntax.ListComp{Elem: g.x(), Clauses: []*syntax.CompClause{g.clause()}}, 2},
		{&syntax.BraceLit{Items: []syntax.BraceItem{
			&syntax.FieldItem{Bounds: g.b()}, &syntax.MapItem{Bounds: g.b()},
			&syntax.EntryItem{Bounds: g.b()}, &syntax.SpreadItem{Bounds: g.b()},
		}, Clauses: []*syntax.CompClause{g.clause()}}, 5},
		{&syntax.FieldItem{Name: g.id(), Value: g.x()}, 2},
		{&syntax.MapItem{Key: g.x(), Value: g.x()}, 2},
		{&syntax.EntryItem{Doc: g.doc(), Mods: g.mods(), Key: g.id(), Annotations: []*syntax.Annotation{g.ann()}, Value: g.brace()}, 5},
		{&syntax.SpreadItem{X: g.x()}, 1},
		{&syntax.CompClause{Keyword: syntax.KwFor, Vars: []*syntax.Ident{g.id(), g.id()}, X: g.x()}, 3},
		{&syntax.TypedLit{Type: g.qn(), Lit: g.brace()}, 2},
		{&syntax.IfExpr{Cond: g.x(), Then: &syntax.ExprBody{Bounds: g.b()}, ElseIf: &syntax.IfExpr{Bounds: g.b()}, Else: &syntax.ExprBody{Bounds: g.b()}}, 4},
		{&syntax.ExprBody{X: g.x()}, 1},
		{&syntax.MatchExpr{Scrutinee: g.x(), Arms: []*syntax.MatchArm{{Bounds: g.b()}}}, 2},
		{&syntax.MatchArm{Patterns: []*syntax.Pattern{g.pat()}, Body: g.x()}, 2},
		{&syntax.Pattern{Keyword: syntax.TokInvalid, Name: g.qn(), Binder: g.id()}, 2},
		{&syntax.LoadExpr{Method: g.id(), Args: []*syntax.Arg{g.arg()}}, 2},
	}
}

func stmtSamples(g *gen) []sample {
	return []sample{
		{&syntax.Block{Stmts: []syntax.Stmt{&syntax.BreakStmt{Bounds: g.b()}, &syntax.ContinueStmt{Bounds: g.b()}}}, 2},
		{&syntax.LetStmt{Name: g.id(), Type: g.ty(), Value: g.x()}, 3},
		{&syntax.VarStmt{Name: g.id(), Type: g.ty(), Value: g.x()}, 3},
		{&syntax.AssignStmt{Target: g.x(), Op: syntax.TokAddAssign, OpTok: g.tok, Value: g.x()}, 2},
		{&syntax.IfStmt{Cond: g.x(), Then: g.block(), ElseIf: &syntax.IfStmt{Bounds: g.b()}, Else: g.block()}, 4},
		{&syntax.ForStmt{Vars: []*syntax.Ident{g.id()}, Iter: g.x(), Body: g.block()}, 3},
		{&syntax.WhileStmt{Cond: g.x(), Body: g.block()}, 2},
		{&syntax.BreakStmt{Bounds: g.b()}, 0},
		{&syntax.ContinueStmt{Bounds: g.b()}, 0},
		{&syntax.ReturnStmt{Value: g.x()}, 1},
		{&syntax.ExpectStmt{X: g.x(), Outcome: g.id(), Message: g.str()}, 3},
		{&syntax.MatchStmt{Scrutinee: g.x(), Arms: []*syntax.StmtArm{{Bounds: g.b()}}}, 2},
		{&syntax.StmtArm{Patterns: []*syntax.Pattern{g.pat()}, Block: g.block(), X: g.x()}, 3},
		{&syntax.ExprStmt{X: g.x()}, 1},
	}
}

func viewSamples(g *gen) []sample {
	return []sample{
		{&syntax.ViewTitle{Doc: g.doc(), Text: g.str()}, 2},
		{&syntax.ViewSubtitle{Doc: g.doc(), Text: g.str()}, 2},
		{&syntax.ViewSingular{Doc: g.doc(), Text: g.str()}, 2},
		{&syntax.ViewPlural{Doc: g.doc(), Text: g.str()}, 2},
		{&syntax.ViewMenu{Doc: g.doc(), Menu: g.id(), Icon: g.id()}, 3},
		{&syntax.ViewColumns{Doc: g.doc(), Items: []*syntax.ViewColumn{{Bounds: g.b()}}}, 2},
		{&syntax.ViewColumn{Name: g.id(), Width: g.int()}, 2},
		{&syntax.ViewSearch{Doc: g.doc(), Items: []syntax.Expr{g.x(), g.x()}}, 3},
		{&syntax.ViewFilters{Doc: g.doc(), Items: []*syntax.ViewFilter{{Bounds: g.b()}}}, 2},
		{&syntax.ViewFilter{Name: g.id(), Multi: g.tok}, 1},
		{&syntax.ViewPreview{Doc: g.doc(), X: g.x()}, 2},
		{&syntax.ViewShow{Doc: g.doc(), ID: g.id(), Label: g.str(), Template: g.str()}, 4},
		{&syntax.ViewGroup{
			Doc: g.doc(), ID: g.id(), Label: g.str(), Help: g.str(), Advanced: g.tok, When: g.x(),
			Members: []syntax.GroupMember{g.show(), &syntax.ViewField{Bounds: g.b()}},
		}, 7},
		{&syntax.ViewField{Doc: g.doc(), Field: g.tok, Name: g.id(), Label: g.str(), Props: g.brace()}, 4},
	}
}
