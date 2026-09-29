package format

import "github.com/fantasim/canonlang/internal/syntax"

// Width is the width of a line in code points (FORMATTER.md §2).
const Width = 100

const (
	indentUnit    = 2 // FORMATTER.md §3
	blankRun      = 2 // line breaks in a row that make a blank line
	minChainCalls = 2 // FORMATTER.md §7.2
	decimalBase   = 10
)

const (
	space, newlineText, docLead, tripleQuote = " ", "\n", "///", `"""`
	listSep, blockOpen, trailingBlanks       = ", ", "/*", " \t"
	zeroDuration, docField, ordinaryLead     = "0s", "Doc", "////"
)

// durationUnits are the units of GRAMMAR.md §2.5, largest first, in milliseconds.
var durationUnits = [...]struct {
	name   string
	millis int64
}{{"d", 86_400_000}, {"h", 3_600_000}, {"m", 60_000}, {"s", 1_000}, {"ms", 1}}

// badKinds are the nodes that stand for what did not parse.
var badKinds = [syntax.NodeKindCount]bool{
	syntax.KindBadExpr: true, syntax.KindBadType: true, syntax.KindBadStmt: true, syntax.KindBadDecl: true,
}

// Precedence levels of the binary operators (GRAMMAR.md §5.11).
const (
	levelNone uint8 = iota
	levelCoalesce
	levelOr
	levelAnd
	levelCompare
	levelAdditive
	levelMultiplicative
)

var binaryLevel = [syntax.TokenKindCount]uint8{
	syntax.TokCoalesce: levelCoalesce, syntax.KwOr: levelOr, syntax.KwAnd: levelAnd,
	syntax.TokEq: levelCompare, syntax.TokNe: levelCompare, syntax.TokLt: levelCompare,
	syntax.TokLe: levelCompare, syntax.TokGt: levelCompare, syntax.TokGe: levelCompare,
	syntax.KwIn: levelCompare, syntax.TokPlus: levelAdditive, syntax.TokMinus: levelAdditive,
	syntax.TokStar: levelMultiplicative, syntax.TokSlash: levelMultiplicative,
	syntax.TokPercent: levelMultiplicative,
}

// cannotEndItem is DECISIONS 211's rule-2 keyword set: a list item ending on one of these keeps
// the comma after it, whatever the next item starts with.
var cannotEndItem = [syntax.TokenKindCount]bool{
	syntax.KwAnd: true, syntax.KwOr: true, syntax.KwNot: true, syntax.KwIn: true, syntax.KwIs: true,
	syntax.KwElse: true, syntax.KwWhere: true, syntax.KwAs: true,
}

// joinsLine is DECISIONS 211's rule-3 startable set, the tokens that can start a list item
// needing a comma before it.
var joinsLine = [syntax.TokenKindCount]bool{
	syntax.TokDot: true, syntax.KwAnd: true, syntax.KwOr: true, syntax.KwIn: true,
	syntax.KwIs: true, syntax.KwElse: true, syntax.KwWhere: true,
}

// Document kinds (FORMATTER.md §7.1), the index of the printer's tables.
const (
	kText kind = iota
	kConcat
	kLine
	kSoftline
	kHardline
	kIfBreak
	kIndent
	kGroup
	kFlat
	kComment
	kSuffix
	kBlank
	kMultiline
	kRHS
	kindCount
)

const (
	breakMode mode = iota
	flatMode
)

const (
	eolNone eol = iota
	eolComment
	eolSuffix
)

// restFill stands for the columns that follow a node printed alone (Place.Rest).
const restFill = "_"

// The kinds of a Change (FORMATTER.md §13).
const (
	Replace ChangeKind = iota
	Insert
	Remove
	Retire
	Move
	changeKindCount
)

// fragmentPath names the text of a new item while the lexer reads its edges (DECISIONS 211).
const fragmentPath = "fragment.canon"

// carriageReturn and tab are what Rewrite refuses in its input (log-2026-09-29 M4 U1r).
const carriageReturn, tab = "\r", "\t"
