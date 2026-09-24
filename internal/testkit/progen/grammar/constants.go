package grammar

import "github.com/fantasim/canonlang/internal/syntax"

// Context flags: what the enclosing constructs allow.
const (
	inFn     ctx = 1 << iota // a function body: return
	inCheck                  // a check block: fail and warn calls
	inTest                   // a test block: expect
	inLoop                   // a for or while body: break and continue
	inHeader                 // depth 0 of a header: no "{" (E1129)
	inInterp                 // an interpolation: no line break (E1112)
	inResult                 // a function's result type: no where
	inClosed                 // followed by a word on its line: no open range "a..", which would read it
)

const (
	indentUnit    = "  "
	space         = " "
	newline       = "\n"
	blankLine     = "\n\n"
	commaSpace    = ", "
	colonSpace    = ": "
	quote         = `"`
	mlQuote       = `"""`
	rawPrefix     = "r"
	docPrefix     = "/// "
	linePrefix    = "// "
	blockOpen     = "/* "
	blockClose    = " */"
	docSlashes    = "///"
	notePrefix    = "c"
	commentMark   = "comment "
	leadingTag    = "leading"
	trailingTag   = "trailing"
	attachedTag   = " doc attached"
	detachedTag   = " doc detached"
	trailingSpace = " \t\r"
	regexSlash    = "/"
	openParen     = "("
	closeParen    = ")"
	openBrack     = "["
	closeBrack    = "]"
	openBrace     = "{"
	closeBrace    = "}"
	braceSpaced   = "{ "
	spacedBrace   = " }"
	emptyBraces   = "{}"
	arrowSpaced   = " -> "
	fatSpaced     = " => "
	assignOp      = " = "
	selfWord      = "self"
	orderedWord   = "ordered"
	keyedWord     = "keyed"
	byWord        = "by"
	inputWord     = "input "
	fromEnv       = " from env "
	atWord        = " at "
	elseWord      = " else "
	fieldWord     = "field"
	valueParam    = "value: "
	siblings      = ", siblings: "
	defaultWord   = " default"
	multiWord     = " multi"
	advanced      = " advanced"
	whenWord      = " when "
	extWord       = "ext"
	loadDir       = "load.dir"
	passesWord    = " passes"
	failsWord     = "fails"
	warnsWord     = "warns"
	failCall      = "fail("
	warnCall      = "warn("
	envName       = `"CANON_GEN_KEY"`
	interpOpen    = "{"
	interpClose   = "}"
	specColon     = ":"
	textPart      = "text"
	anyType       = "_"
	rootPath      = "/"
	corruptFile   = "gen/gen.canon"
	decimalBase   = 10

	// Sizes: how many of each list a generated file holds at most.
	maxDecls     = 12
	maxImports   = 3
	maxItems     = 5
	maxArgs      = 3
	maxStmts     = 4
	maxParts     = 3
	maxDocLines  = 2
	maxMLLines   = 3
	levelCount   = 11 // precedence levels 0 (lambda) to 10 (postfix), GRAMMAR.md §5.11
	levelCoal    = 1
	levelOr      = 2
	levelAnd     = 3
	levelNot     = 4
	levelCompare = 5
	levelRange   = 6
	levelAdd     = 7
	levelMul     = 8
	levelUnary   = 9
	levelPostfix = 10
	oneIn2       = 2
	oneIn3       = 3
	oneIn4       = 4
	oneIn6       = 6
	oneIn8       = 8
)

// Identifier pools: no keyword, contextual ones included (GRAMMAR.md §4.2), no fail or warn.
var (
	lowerNames = []string{"a", "b", "item", "count", "label", "next", "xs", "total", "_tmp", "v2", "stepSize", "it"}
	upperNames = []string{"Item", "Row", "Tone", "Reward", "Status", "Deck", "T", "Point", "Kind"}
	pkgNames   = []string{"gen", "gen.sub", "lib", "lib.core", "a.b.c"}
	// wordPool are WORDs (§4.3 a, b) but those no line may end with (§3.1 rule 2).
	wordPool = []string{"open", "taken", "fire", "IK1_WEAPON", "check", "package", "type", "record", "emit", "for", "if", "_0"}
	// nameable are the reserved words read as names in value position (§4.3 c).
	nameable = []string{"check", "package", "record", "type", "view", "table", "stable", "entry"}
	// dataWords name data: none of E1126's, no modifier, no item keyword, no §3.1 rule 3 word.
	dataWords  = []string{"open", "taken", "fire", "IK1_WEAPON", "package", "type", "_0", "series_1", "done"}
	textRunes  = []string{"a", "Z", "0", " ", "é", "-", "_", "!", ".", "日", "?", "'", ":"}
	escapes    = []string{`\n`, `\t`, `\"`, `\\`, `\{`, `\}`, `\u{1F600}`, `\u{41}`, "{{", "}}"}
	rawRunes   = []string{"a", `\`, "{", "}", " ", "é", "x"}
	specs      = []string{"+", ",", ".2", "+,.3", ",.0", ".20"}
	regexes    = []string{"^[A-Z0-9_]+$", "^II_", "a|b", "[a-z]{1,3}", "(?i)abc", `\d+`, `^x\/y$`, "[/]"}
	durations  = []string{"250ms", "90s", "1h30m", "2d", "0s", "1_000ms", "60s", "1d12h5m3s7ms", "36h"}
	floats     = []string{"1.5", "0.25", "2.5e3", "1e-3", "3E+2", "0.0000001", "0e3"}
	ints       = []string{"0", "7", "42", "1_000_000", "0x1F", "0b1010", "9223372036854775807", "12"}
	emitTarget = []string{"go", "cpp", "ts", "json", "view"}
	emitModes  = []string{"baked", "data", "embedded", "types"}
	viewTexts  = []string{"title", "subtitle", "singular", "plural"}
	// docOpeners start a doc line: canon fmt spaces the second.
	docOpeners = []string{docPrefix, docSlashes}
	// strayBytes are the bytes a corruption inserts (GRAMMAR.md §1, §2).
	strayBytes = []string{"\x01", "\xff", "\ufeff", "\"", `"""`, "{", "}", "\r", "#", "$", "/*", "`", "\x00", "é", "\t", "(", "[", "@"}
)

// Operators by precedence level (GRAMMAR.md §5.11), and the literal words.
var (
	constWords = []syntax.TokenKind{syntax.KwTrue, syntax.KwFalse, syntax.KwNone}
	compareOps = []syntax.TokenKind{syntax.TokEq, syntax.TokNe, syntax.TokLt, syntax.TokLe, syntax.TokGt, syntax.TokGe, syntax.KwIn}
	rangeOps   = []syntax.TokenKind{syntax.TokRange, syntax.TokRangeIncl}
	addOps     = []syntax.TokenKind{syntax.TokPlus, syntax.TokMinus}
	mulOps     = []syntax.TokenKind{syntax.TokStar, syntax.TokSlash, syntax.TokPercent}
	assignOps  = []syntax.TokenKind{syntax.TokAssign, syntax.TokAddAssign, syntax.TokSubAssign, syntax.TokMulAssign, syntax.TokDivAssign}
)

// Annotations by position (GRAMMAR.md §8.3): distinct names, so any subset is valid there.
var (
	fieldAnns   = []string{"@stable", `@deprecated("use b")`, "@since(2)", `@go(name: "Fx")`, "@ts(bigint)"}
	fieldJSON   = []string{`@json("wire")`, `@json(path: "a.b")`, "@json(inline)", "@json(none: 0)", "@json(unit: s)", "@json(int)", "@json(bits)", `@json("w", unit: ms)`, `@json(pairs: ["k{i}", "v{i}"])`}
	memberAnns  = []string{"@deprecated", "@since(1)", `@go(name: "Mx")`}
	caseAnns    = []string{`@json("Case")`, "@deprecated", "@since(3)", "@cpp(value: 3)"}
	recordAnns  = []string{"@json(case: snake)", "@since(1)", `@go(name: "Rx")`}
	enumAnns    = []string{"@codes(UInt8)", "@since(1)", `@cpp(defines: "P_")`}
	variantAnns = []string{`@json(tag: "type", case: kebab)`, "@since(1)", `@go(name: "Vx")`}
	letAnns     = []string{"@reload", "@since(2)", `@go(name: "Lx")`}
	declAnns    = []string{"@since(2)", `@go(name: "Dx")`, `@ts(name: "Tx")`}
	entryAnns   = []string{`@deprecated("gone")`, "@since(1)"}
	methodAnns  = []string{`@go(name: "Str")`, "@since(1)"}
	anyAnns     = []string{"@since(1)"}
)
