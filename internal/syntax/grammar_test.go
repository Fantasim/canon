package syntax_test

import (
	"bufio"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

const grammarMD = "../../spec/GRAMMAR.md"

var productionLine = regexp.MustCompile(`^([A-Za-z]+)\s+=`)

// grammarBlocks returns the lines of the fenced blocks of GRAMMAR.md: the ebnf blocks when ebnf
// is set, else the first plain block after the heading that starts with section.
func grammarBlocks(t *testing.T, ebnf bool, section string) []string {
	t.Helper()
	f, err := os.Open(grammarMD)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lines []string
	in, armed := false, ebnf
	for sc := bufio.NewScanner(f); sc.Scan(); {
		l := sc.Text()
		switch {
		case !ebnf && strings.HasPrefix(l, "### "+section+" "):
			armed = true
		case strings.HasPrefix(l, "```") && in:
			in = false
			if !ebnf {
				return lines
			}
		case l == "```ebnf" && ebnf, l == "```" && armed && !ebnf:
			in = true
		case in:
			lines = append(lines, l)
		}
	}
	return lines
}

var (
	decls = []syntax.NodeKind{
		syntax.KindConstDecl, syntax.KindLetDecl, syntax.KindTypeDecl, syntax.KindFnDecl,
		syntax.KindEntryDecl, syntax.KindCheckDecl, syntax.KindViewDecl, syntax.KindWidgetDecl,
		syntax.KindTestDecl, syntax.KindEmitDecl, syntax.KindRecordDecl, syntax.KindEnumDecl,
		syntax.KindVariantDecl,
	}
	primTypes = []syntax.NodeKind{
		syntax.KindNamedType, syntax.KindListType, syntax.KindKeyedType, syntax.KindMapType,
		syntax.KindDepMapType, syntax.KindTableType, syntax.KindRefType, syntax.KindFnType,
		syntax.KindAssetType, syntax.KindMatchType, syntax.KindLiteralType, syntax.KindAnyType,
		syntax.KindParenType,
	}
	types    = append([]syntax.NodeKind{syntax.KindUnionType, syntax.KindOptionalType, syntax.KindWhereType}, primTypes...)
	literals = []syntax.NodeKind{
		syntax.KindIntLit, syntax.KindFloatLit, syntax.KindDurationLit, syntax.KindStringLit,
		syntax.KindRawStringLit, syntax.KindBoolLit, syntax.KindNoneLit,
	}
	primaries = append([]syntax.NodeKind{
		syntax.KindIdentExpr, syntax.KindSelfExpr, syntax.KindParenExpr, syntax.KindShorthandLambda,
		syntax.KindListLit, syntax.KindListComp, syntax.KindBraceLit, syntax.KindTypedLit,
		syntax.KindIfExpr, syntax.KindMatchExpr, syntax.KindLoadExpr,
	}, literals...)
	postfixes = []syntax.NodeKind{syntax.KindSelectorExpr, syntax.KindIndexExpr, syntax.KindCallExpr, syntax.KindForceExpr}
	exprs     = slices.Concat(primaries, postfixes, []syntax.NodeKind{
		syntax.KindLambdaExpr, syntax.KindBinaryExpr, syntax.KindUnaryExpr, syntax.KindIsExpr,
		syntax.KindRangeExpr, syntax.KindRegexLit,
	})
	stmts = []syntax.NodeKind{
		syntax.KindLetStmt, syntax.KindVarStmt, syntax.KindIfStmt, syntax.KindForStmt,
		syntax.KindWhileStmt, syntax.KindMatchStmt, syntax.KindBreakStmt, syntax.KindContinueStmt,
		syntax.KindReturnStmt, syntax.KindExpectStmt, syntax.KindExprStmt, syntax.KindAssignStmt,
	}
	viewItems = []syntax.NodeKind{
		syntax.KindViewTitle, syntax.KindViewSubtitle, syntax.KindViewSingular, syntax.KindViewPlural,
		syntax.KindViewMenu, syntax.KindViewPreview, syntax.KindViewSearch, syntax.KindViewFilters,
		syntax.KindViewColumns, syntax.KindViewGroup, syntax.KindViewShow, syntax.KindViewField,
	}
	strs    = []syntax.NodeKind{syntax.KindStringLit, syntax.KindRawStringLit}
	members = []syntax.NodeKind{syntax.KindFnDecl, syntax.KindCheckDecl, syntax.KindAnnotation}
)

// productions maps each production of GRAMMAR.md to its own node, the alternatives it stands
// for, or the node holding it.
var productions = map[string][]syntax.NodeKind{
	"projectFile": {syntax.KindFile, syntax.KindProjectDecl}, "sourceFile": {syntax.KindFile},
	"layerFile": {syntax.KindFile, syntax.KindIdent}, "translationFile": {syntax.KindFile, syntax.KindIdent},
	"packageClause": {syntax.KindQualifiedName}, "importDecl": {syntax.KindImport},
	"topDecl": append([]syntax.NodeKind{syntax.KindDocComment, syntax.KindAnnotation}, decls...), "topDeclBody": decls,
	"constDecl": {syntax.KindConstDecl, syntax.KindModifiers}, "letDecl": {syntax.KindLetDecl, syntax.KindModifiers},
	"typeDecl": {syntax.KindTypeDecl}, "typeParams": {syntax.KindParam}, "param": {syntax.KindParam},
	"fnDecl": {syntax.KindFnDecl, syntax.KindModifiers}, "fnParams": {syntax.KindParam},
	"entryDecl": {syntax.KindEntryDecl}, "entryKey": {syntax.KindIdent, syntax.KindIntLit},
	"testDecl": {syntax.KindTestDecl}, "emitDecl": {syntax.KindEmitDecl}, "widgetDecl": {syntax.KindWidgetDecl},
	"recordDecl": {syntax.KindRecordDecl}, "recordBody": {syntax.KindRecordBody},
	"recordItem": append([]syntax.NodeKind{syntax.KindFieldDecl}, members...), "fieldDecl": {syntax.KindFieldDecl},
	"fieldType": {syntax.KindFieldDecl}, "enumDecl": {syntax.KindEnumDecl}, "enumMember": {syntax.KindEnumMember},
	"variantDecl": {syntax.KindVariantDecl}, "variantItem": append([]syntax.NodeKind{syntax.KindVariantCase}, members...),
	"variantCase": {syntax.KindVariantCase}, "checkDecl": {syntax.KindCheckDecl},
	"viewDecl": {syntax.KindViewDecl}, "viewItem": viewItems, "filterItem": {syntax.KindViewFilter},
	"columnItem": {syntax.KindViewColumn}, "groupItem": {syntax.KindViewGroup},
	"groupMember": {syntax.KindViewShow, syntax.KindViewField}, "showItem": {syntax.KindViewShow},
	"fieldItem": {syntax.KindViewField}, "amendDecl": {syntax.KindAmendBlock}, "amendItem": {syntax.KindAmendment},
	"amendPath": {syntax.KindAmendSegment}, "translationEntry": {syntax.KindTranslationEntry},
	"translationKey": {syntax.KindQualifiedName}, "annotation": {syntax.KindAnnotation},
	"annArg": {syntax.KindAnnotationArg}, "annValue": slices.Concat(strs, []syntax.NodeKind{
		syntax.KindIntLit, syntax.KindFloatLit, syntax.KindDurationLit, syntax.KindBoolLit,
		syntax.KindQualifiedName, syntax.KindAnnotationList, syntax.KindBraceLit,
	}),
	"type": types, "unionType": {syntax.KindUnionType}, "optType": {syntax.KindOptionalType, syntax.KindWhereType},
	"primType": primTypes, "namedType": {syntax.KindNamedType}, "typeArgs": {syntax.KindTypeArgs},
	"typeArg": exprs, "listType": {syntax.KindListType, syntax.KindKeyedType}, "mapType": {syntax.KindMapType},
	"depMapType": {syntax.KindDepMapType}, "tableType": {syntax.KindTableType}, "refType": {syntax.KindRefType},
	"fnType": {syntax.KindFnType}, "assetType": {syntax.KindAssetType},
	"extName": append([]syntax.NodeKind{syntax.KindIdent}, strs...), "matchType": {syntax.KindMatchType},
	"typeArm": {syntax.KindTypeArm}, "block": {syntax.KindBlock}, "statement": stmts,
	"letStmt": {syntax.KindLetStmt}, "varStmt": {syntax.KindVarStmt}, "ifStmt": {syntax.KindIfStmt},
	"forStmt": {syntax.KindForStmt}, "whileStmt": {syntax.KindWhileStmt}, "matchStmt": {syntax.KindMatchStmt},
	"stmtArm": {syntax.KindStmtArm}, "returnStmt": {syntax.KindReturnStmt}, "expectStmt": {syntax.KindExpectStmt},
	"simpleStmt": {syntax.KindExprStmt, syntax.KindAssignStmt}, "assignOp": {syntax.KindAssignStmt},
	"binder": {syntax.KindIdent}, "expr": exprs, "lambda": {syntax.KindLambdaExpr},
	"lambdaParams": {syntax.KindLambdaExpr, syntax.KindIdent}, "coalesce": {syntax.KindBinaryExpr},
	"orExpr": {syntax.KindBinaryExpr}, "andExpr": {syntax.KindBinaryExpr}, "notExpr": {syntax.KindUnaryExpr},
	"compare": {syntax.KindBinaryExpr, syntax.KindIsExpr}, "compOp": {syntax.KindBinaryExpr},
	"range": {syntax.KindRangeExpr}, "rangeOp": {syntax.KindRangeExpr}, "additive": {syntax.KindBinaryExpr},
	"multiplicative": {syntax.KindBinaryExpr}, "unary": {syntax.KindUnaryExpr}, "postfix": postfixes,
	"postfixOp": postfixes, "callArgs": {syntax.KindCallExpr}, "arg": {syntax.KindArg}, "primary": primaries,
	"shorthand": {syntax.KindShorthandLambda}, "typedLit": {syntax.KindTypedLit}, "ifExpr": {syntax.KindIfExpr},
	"exprBody": {syntax.KindExprBody}, "matchExpr": {syntax.KindMatchExpr}, "exprArm": {syntax.KindMatchArm},
	"pattern": {syntax.KindPattern}, "loadCall": {syntax.KindLoadExpr}, "listLit": {syntax.KindListLit, syntax.KindListComp},
	"braceLit": {syntax.KindBraceLit}, "compClause": {syntax.KindCompClause}, "literal": literals, "headerExpr": exprs,
	"braceItem":   {syntax.KindFieldItem, syntax.KindMapItem, syntax.KindEntryItem, syntax.KindSpreadItem},
	"projectItem": {syntax.KindProjectEntry}, "pEntry": {syntax.KindProjectEntry},
	"pValue": slices.Concat(strs, []syntax.NodeKind{
		syntax.KindIntLit, syntax.KindQualifiedName, syntax.KindProjectList, syntax.KindProjectMap,
	}),
}

// lexical maps the token-level constructs that become nodes of their own (GRAMMAR.md §2).
var lexical = map[string][]syntax.NodeKind{
	"DOC":           {syntax.KindDocComment},
	"interpolation": {syntax.KindInterp},
}

// GRAMMAR.md §10: every production of the ebnf blocks maps to node kinds, and every kind to one.
func TestEveryProductionHasANode(t *testing.T) {
	var want []string
	for _, l := range grammarBlocks(t, true, "") {
		if m := productionLine.FindStringSubmatch(l); m != nil && !slices.Contains(want, m[1]) {
			want = append(want, m[1])
		}
	}
	for _, p := range want {
		if len(productions[p]) == 0 {
			t.Errorf("production %s maps to no node kind", p)
		}
	}
	if len(productions) != len(want) {
		t.Errorf("the table maps %d productions, GRAMMAR.md has %d", len(productions), len(want))
	}
	covered := map[syntax.NodeKind]bool{}
	for _, m := range []map[string][]syntax.NodeKind{productions, lexical} {
		for _, kinds := range m {
			for _, k := range kinds {
				covered[k] = true
			}
		}
	}
	for k := syntax.KindInvalid + 1; k < syntax.NodeKindCount; k++ {
		if !covered[k] {
			t.Errorf("node kind %s stands for no production", k)
		}
	}
}
