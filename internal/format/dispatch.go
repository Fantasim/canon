package format

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// buildTable builds the document of a node by its kind (DECISIONS 26). The kinds a file that
// parsed without error never holds (Bad nodes, the doc comment, the interpolation) have none.
var buildTable [syntax.NodeKindCount]func(*builder, syntax.Node) *doc

// on adapts a builder method on a node type to the table.
func on[N syntax.Node](f func(*builder, N) *doc) func(*builder, syntax.Node) *doc {
	return func(b *builder, n syntax.Node) *doc { return f(b, n.(N)) }
}

func leafOf(b *builder, n syntax.Node) *doc { return b.leaf(n) }

func init() {
	for _, k := range []syntax.NodeKind{
		syntax.KindIdent, syntax.KindIdentExpr, syntax.KindIntLit, syntax.KindFloatLit, syntax.KindRegexLit,
		syntax.KindBoolLit, syntax.KindNoneLit, syntax.KindSelfExpr, syntax.KindAnyType, syntax.KindBreakStmt,
		syntax.KindContinueStmt,
	} {
		buildTable[k] = leafOf
	}
	initDecls()
	initExprs()
	initViews()
}

func initDecls() {
	t := &buildTable
	t[syntax.KindQualifiedName], t[syntax.KindImport] = on((*builder).qualifiedName), on((*builder).importDecl)
	t[syntax.KindAnnotation], t[syntax.KindAnnotationArg] = on((*builder).annotation), on((*builder).annotationArg)
	t[syntax.KindAnnotationList], t[syntax.KindModifiers] = on((*builder).annotationList), on((*builder).modifiers)
	t[syntax.KindConstDecl], t[syntax.KindLetDecl] = on((*builder).constDecl), on((*builder).letDecl)
	t[syntax.KindTypeDecl], t[syntax.KindParam] = on((*builder).typeDecl), on((*builder).param)
	t[syntax.KindFnDecl], t[syntax.KindEntryDecl] = on((*builder).fnDecl), on((*builder).entryDecl)
	t[syntax.KindCheckDecl], t[syntax.KindViewDecl] = on((*builder).checkDecl), on((*builder).viewDecl)
	t[syntax.KindWidgetDecl], t[syntax.KindTestDecl] = on((*builder).widgetDecl), on((*builder).testDecl)
	t[syntax.KindEmitDecl], t[syntax.KindRecordDecl] = on((*builder).emitDecl), on((*builder).recordDecl)
	t[syntax.KindRecordBody], t[syntax.KindFieldDecl] = on((*builder).recordBody), on((*builder).fieldDecl)
	t[syntax.KindEnumDecl], t[syntax.KindEnumMember] = on((*builder).enumDecl), on((*builder).enumMember)
	t[syntax.KindVariantDecl], t[syntax.KindVariantCase] = on((*builder).variantDecl), on((*builder).variantCase)
	t[syntax.KindAmendBlock], t[syntax.KindAmendment] = on((*builder).amendBlock), on((*builder).amendment)
	t[syntax.KindAmendSegment], t[syntax.KindTranslationEntry] = on((*builder).amendSegment), on((*builder).translationEntry)
	t[syntax.KindProjectDecl], t[syntax.KindProjectEntry] = on((*builder).project), on((*builder).projectEntry)
	t[syntax.KindProjectList] = on(func(b *builder, n *syntax.ProjectList) *doc {
		return b.parenList(n.First(), n.Last(), partsOf(b, n.Items))
	})
	t[syntax.KindProjectMap] = on(func(b *builder, n *syntax.ProjectMap) *doc {
		return b.braceList(n.First(), n.Last(), entries(b, n.Entries))
	})
	t[syntax.KindBlock], t[syntax.KindLetStmt] = on((*builder).block), on((*builder).letStmt)
	t[syntax.KindVarStmt], t[syntax.KindAssignStmt] = on((*builder).varStmt), on((*builder).assignStmt)
	t[syntax.KindIfStmt], t[syntax.KindForStmt] = on((*builder).ifStmt), on((*builder).forStmt)
	t[syntax.KindWhileStmt], t[syntax.KindReturnStmt] = on((*builder).whileStmt), on((*builder).returnStmt)
	t[syntax.KindExpectStmt], t[syntax.KindMatchStmt] = on((*builder).expectStmt), on((*builder).matchStmt)
	t[syntax.KindStmtArm] = on((*builder).stmtArm)
	t[syntax.KindExprStmt] = on(func(b *builder, n *syntax.ExprStmt) *doc { return b.node(n.X) })
}

func initExprs() {
	t := &buildTable
	t[syntax.KindDurationLit], t[syntax.KindParenExpr] = on((*builder).duration), on((*builder).paren)
	t[syntax.KindStringLit] = on(func(b *builder, n *syntax.StringLit) *doc { return b.stringLit(n, n.Multiline) })
	t[syntax.KindRawStringLit] = on(func(b *builder, n *syntax.RawStringLit) *doc { return b.stringLit(n, n.Multiline) })
	t[syntax.KindUnaryExpr], t[syntax.KindBinaryExpr] = on((*builder).unary), on((*builder).binary)
	t[syntax.KindIsExpr], t[syntax.KindRangeExpr] = on((*builder).isExpr), on((*builder).rangeExpr)
	t[syntax.KindSelectorExpr], t[syntax.KindArg] = on((*builder).selector), on((*builder).arg)
	postfix := func(b *builder, n syntax.Node) *doc { return b.postfix(n.(syntax.Expr)) }
	t[syntax.KindIndexExpr], t[syntax.KindCallExpr], t[syntax.KindForceExpr] = postfix, postfix, postfix
	t[syntax.KindLambdaExpr], t[syntax.KindShorthandLambda] = on((*builder).lambda), on((*builder).shorthand)
	t[syntax.KindListLit], t[syntax.KindListComp] = on((*builder).listLit), on((*builder).listComp)
	t[syntax.KindBraceLit], t[syntax.KindFieldItem] = on((*builder).braceLit), on((*builder).fieldItem)
	t[syntax.KindMapItem], t[syntax.KindEntryItem] = on((*builder).mapItem), on((*builder).entryItem)
	t[syntax.KindSpreadItem], t[syntax.KindCompClause] = on((*builder).spreadItem), on((*builder).compClause)
	t[syntax.KindTypedLit], t[syntax.KindIfExpr] = on((*builder).typedLit), on((*builder).ifExpr)
	t[syntax.KindMatchExpr], t[syntax.KindMatchArm] = on((*builder).matchExpr), on((*builder).matchArm)
	t[syntax.KindPattern], t[syntax.KindLoadExpr] = on((*builder).pattern), on((*builder).loadExpr)
	t[syntax.KindNamedType], t[syntax.KindTypeArgs] = on((*builder).namedType), on((*builder).typeArgList)
	t[syntax.KindListType], t[syntax.KindMapType] = on((*builder).listType), on((*builder).mapType)
	t[syntax.KindDepMapType], t[syntax.KindTableType] = on((*builder).depMapType), on((*builder).tableType)
	t[syntax.KindRefType], t[syntax.KindOptionalType] = on((*builder).refType), on((*builder).optionalType)
	t[syntax.KindKeyedType], t[syntax.KindWhereType] = on((*builder).keyedType), on((*builder).whereType)
	t[syntax.KindUnionType], t[syntax.KindLiteralType] = on((*builder).unionType), on((*builder).literalType)
	t[syntax.KindMatchType], t[syntax.KindTypeArm] = on((*builder).matchType), on((*builder).typeArm)
	t[syntax.KindAssetType], t[syntax.KindFnType] = on((*builder).assetType), on((*builder).fnType)
	t[syntax.KindParenType] = on((*builder).parenType)
}

func initViews() {
	t := &buildTable
	t[syntax.KindViewTitle] = on(func(b *builder, n *syntax.ViewTitle) *doc { return b.viewText(n, n.Text) })
	t[syntax.KindViewSubtitle] = on(func(b *builder, n *syntax.ViewSubtitle) *doc { return b.viewText(n, n.Text) })
	t[syntax.KindViewSingular] = on(func(b *builder, n *syntax.ViewSingular) *doc { return b.viewText(n, n.Text) })
	t[syntax.KindViewPlural] = on(func(b *builder, n *syntax.ViewPlural) *doc { return b.viewText(n, n.Text) })
	t[syntax.KindViewColumns] = on(func(b *builder, n *syntax.ViewColumns) *doc {
		return b.viewBraces(n, n.Braces, entries(b, n.Items))
	})
	t[syntax.KindViewSearch] = on(func(b *builder, n *syntax.ViewSearch) *doc {
		return b.viewBraces(n, n.Braces, entries(b, n.Items))
	})
	t[syntax.KindViewFilters] = on(func(b *builder, n *syntax.ViewFilters) *doc {
		return b.viewBraces(n, n.Braces, entries(b, n.Items))
	})
	t[syntax.KindViewMenu], t[syntax.KindViewColumn] = on((*builder).viewMenu), on((*builder).viewColumn)
	t[syntax.KindViewFilter], t[syntax.KindViewPreview] = on((*builder).viewFilter), on((*builder).viewPreview)
	t[syntax.KindViewShow], t[syntax.KindViewGroup] = on((*builder).viewShow), on((*builder).viewGroup)
	t[syntax.KindViewField] = on((*builder).viewField)
}
