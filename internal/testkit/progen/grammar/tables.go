package grammar

import "github.com/fantasim/canonlang/internal/syntax"

// fixedTokens are the token kinds whose text is fixed: punctuation, then keywords.
var fixedTokens []syntax.TokenKind

// levels write an expression of each precedence level (GRAMMAR.md §5.11).
var levels [levelCount]func(*gen)

// postfixOps are the steps after a primary.
var postfixOps []func(*gen)

// primaries write a primary and tell a number, which takes no postfix step.
var primaries []func(*gen) bool

// stmtKinds write a statement (GRAMMAR.md §5.10), or refuse where the context forbids it.
var stmtKinds []func(*gen) bool

// primTypes are the nesting primTypes (GRAMMAR.md §5.9).
var primTypes []func(*gen)

// declKinds are the top-level declarations of GRAMMAR.md §5.3, one writer each.
var declKinds []func(*gen)

// viewItems are the items of a view body (GRAMMAR.md §5.6).
var viewItems []func(*gen)

// init fills the tables, whose writers reach back into them, which an initializer cannot.
func init() {
	for k := syntax.TokEllipsis; k < syntax.TokenKindCount; k++ {
		fixedTokens = append(fixedTokens, k)
	}
	levels = [levelCount]func(*gen){
		(*gen).lambda, (*gen).coalesce, (*gen).orExpr, (*gen).andExpr, (*gen).notExpr,
		(*gen).compare, (*gen).rangeExpr, (*gen).additive, (*gen).multiplicative, (*gen).unary,
		(*gen).postfix,
	}
	postfixOps = []func(*gen){
		(*gen).member, (*gen).optMember, (*gen).index, (*gen).call, (*gen).force,
	}
	primaries = []func(*gen) bool{
		(*gen).number, (*gen).stringPrimary, (*gen).namePrimary, (*gen).parenPrimary,
		(*gen).listPrimary, (*gen).bracePrimary, (*gen).typedPrimary, (*gen).ifPrimary,
		(*gen).matchPrimary, (*gen).loadPrimary, (*gen).constPrimary,
	}
	stmtKinds = []func(*gen) bool{
		(*gen).letStmt, (*gen).ifStmt, (*gen).forStmt, (*gen).whileStmt, (*gen).matchStmt,
		(*gen).jumpStmt, (*gen).returnStmt, (*gen).expectStmt, (*gen).simpleStmt, (*gen).reportStmt,
	}
	primTypes = []func(*gen){
		(*gen).listType, (*gen).mapType, (*gen).depMapType, (*gen).tableType, (*gen).refType,
		(*gen).funcType, (*gen).assetType, (*gen).matchType, (*gen).literalType, (*gen).parenType,
	}
	declKinds = []func(*gen){
		(*gen).constDecl, (*gen).letDecl, (*gen).typeDecl, (*gen).enumDecl, (*gen).recordDecl,
		(*gen).variantDecl, (*gen).fnDecl, (*gen).entryDecl, (*gen).topCheck, (*gen).viewDecl,
		(*gen).widgetDecl, (*gen).testDecl, (*gen).emitDecl,
	}
	viewItems = []func(*gen){
		(*gen).viewText, (*gen).viewMenu, (*gen).viewPreview, (*gen).viewSearch, (*gen).viewFilters,
		(*gen).viewColumns, (*gen).viewGroup, (*gen).viewShow, (*gen).viewField,
	}
}
