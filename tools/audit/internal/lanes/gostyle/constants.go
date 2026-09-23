package gostyle

import (
	"regexp"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

const laneName = "gostyle"

// Rule ids this lane emits. Every other file refers to these, never the string literal,
// so a rename only touches this list. The ones the web lane also emits are named once by
// package rules.
const (
	ruleFnLength      = "fn-length"
	ruleFnParams      = "fn-params"
	ruleFnResults     = "fn-results"
	ruleFnNesting     = "fn-nesting"
	ruleNakedReturn   = "naked-return"
	ruleFileLength    = "file-length"
	rulePkgSize       = "pkg-size"
	ruleCommentDecl   = rules.IDCommentDecl
	ruleCommentHeader = rules.IDCommentFileHeader
	ruleCommentPkg    = "comment-pkg"
	ruleCommentBlock  = rules.IDCommentBlock
	ruleCommentRatio  = rules.IDCommentRatio
	ruleCommentField  = "comment-field"
	ruleADRNarration  = rules.IDCommentADRNarration
	ruleHistory       = rules.IDCommentHistory
	ruleTODO          = rules.IDTodoInCode
	rulePkgDoc        = "pkg-doc"
	rulePkgExample    = "pkg-example"
)

var ruleIDs = []string{
	ruleFnLength, ruleFnParams, ruleFnResults, ruleFnNesting, ruleNakedReturn,
	ruleFileLength, rulePkgSize, ruleCommentDecl, ruleCommentHeader, ruleCommentPkg,
	ruleCommentBlock, ruleCommentRatio, ruleCommentField, ruleADRNarration, ruleHistory, ruleTODO,
	rulePkgDoc, rulePkgExample,
}

// docGoFile is exempt from comment-file-header; its leading comment is comment-pkg's target.
const docGoFile = "doc.go"

// pkg-doc and pkg-example: the package every command lives in, an example's name prefix.
const (
	mainPkg       = gosrc.MainPkg
	examplePrefix = "Example"
	fmtNoDocGo    = "package %s has no doc.go"
	fmtNoExample  = "package %s has no Example test"
)

const (
	unitLines      = "lines"
	unitStatements = "statements"
	unitParams     = "params"
	unitResults    = "results"
	unitLevels     = "levels"
)

const (
	fmtMeasured        = "%d %s (max %d)"
	fmtMeasuredPercent = "%d%% comments (max %d%%)"
	fmtPkgSize         = "%d files, %d lines (max %d files or %d lines)"
	fmtNakedReturn     = "%d bare return(s) in a %d-line function (max %d)"
	fmtHeaderLines     = "%d header comment line(s) outside doc.go"
)

// commentRatioMinLines is the smallest file, by non-blank lines, the ratio rule judges.
const commentRatioMinLines = 20

const ratioPercentScale = 100

const newline = "\n"

// Directive prefixes never counted toward a doc or block's length.
const (
	directiveGo       = "//go:"
	directiveNolint   = "//nolint"
	directiveSovaudit = "// sovaudit:"
)

// reGeneratedHeader matches the generated-file marker line ast.IsGenerated also looks for.
var reGeneratedHeader = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// reADRCite matches an ADR/DOCTRINE/§/L-nnnn/"decision N"/Amendment citation.
var reADRCite = regexp.MustCompile(`ADR-?\d+|\bADR\b|DOCTRINE|§|\bL-\d{4}\b|decision \d|Amendment`)

// reTODO matches an uppercase code-marker comment.
var reTODO = regexp.MustCompile(`\b(TODO|FIXME|XXX|HACK)\b`)
