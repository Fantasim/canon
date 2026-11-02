package fnsize

// byteRange is the size of a lexer's dispatch table, one handler per leading byte; bytes from
// asciiLimit up are identifier bytes (UTF-8). maxRawDelim is the longest C++ raw-string delimiter.
const (
	byteRange   = 256
	asciiLimit  = 0x80
	maxRawDelim = 16
)

// The token classes: kind.
const (
	kindIdent kind = iota
	kindNumber
	kindString
	kindPunct
)

// What a cppKind says a brace opens: an initializer or block (part of the header that follows),
// a namespace, type or enum (declarations inside), a function body.
const (
	cppOther cppKind = iota
	cppScope
	cppFunction
)

// The constructs an unterminated source names in its error.
const (
	whatQuoted   = "quoted literal"
	whatRaw      = "raw literal"
	whatComment  = "comment"
	whatTemplate = "template literal"
	whatExpr     = "template expression"
)

// Token distances: a label is a word and a colon; "template<" opens a template prefix; an
// operator name is "operator" and its symbol; a type literal's first member is a name, a colon
// (or an optional mark and a colon).
const (
	labelTokens          = 2
	templatePrefixLen    = 2
	operatorTokens       = 2
	memberColonOffset    = 2
	memberOptColonOffset = 3
)

// anonymousName is the name of a function no identifier names.
const anonymousName = "(anonymous)"

// The byte classes and comment marks of the lexers.
const (
	spaceBytes      = " \t\r\n\f\v"
	blankBytes      = " \t\r"
	identStartBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_$"
	exponentBytes   = "eEpP"
	lineOpen        = "//"
	blockOpen       = "/*"
	blockClose      = "*/"
	exprOpen        = "${"
	rawMark         = "R"
	doubleQuote     = "\""
)

// The punctuation the scanners compare tokens against.
const (
	punctOpenBrace    = "{"
	punctCloseBrace   = "}"
	punctOpenParen    = "("
	punctCloseParen   = ")"
	punctOpenBracket  = "["
	punctCloseBracket = "]"
	punctSemicolon    = ";"
	punctColon        = ":"
	punctScope        = "::"
	punctFatArrow     = "=>"
	punctThinArrow    = "->"
	punctAssign       = "="
	punctLess         = "<"
	punctGreater      = ">"
	punctTilde        = "~"
	punctAmp          = "&"
	punctAmpAmp       = "&&"
	punctDot          = "."
	tsOptional        = "?"
	punctPipe         = "|"
	punctComma        = ","
	punctStar         = "*"
)

// The words the scanners compare identifiers against.
const (
	wordOperator  = "operator"
	wordTemplate  = "template"
	wordExtern    = "extern"
	wordElse      = "else"
	wordCatch     = "catch"
	wordFinally   = "finally"
	wordTry       = "try"
	wordDo        = "do"
	wordFunction  = "function"
	wordAsync     = "async"
	wordIf        = "if"
	wordFor       = "for"
	wordWhile     = "while"
	wordSwitch    = "switch"
	wordWith      = "with"
	wordConst     = "const"
	wordNoexcept  = "noexcept"
	wordOverride  = "override"
	wordFinal     = "final"
	wordVolatile  = "volatile"
	wordMutable   = "mutable"
	wordPublic    = "public"
	wordPrivate   = "private"
	wordProtected = "protected"
	wordClass     = "class"
	wordStruct    = "struct"
	wordUnion     = "union"
	wordEnum      = "enum"
	wordNamespace = "namespace"
	wordAlignas   = "alignas"
	wordConstexpr = "constexpr"
	wordConsteval = "consteval"
)

// The reasons a C++ header is refused, and the token count of `extern "C"` alone.
const (
	reasonExtern      = `extern "C" holds more than a block`
	reasonAlignas     = "alignas in a declaration header"
	reasonTry         = "function-try-block"
	reasonElaborated  = "type keyword with a parenthesis: an elaborated return type"
	reasonControl     = "a control statement at declaration level"
	externBlockTokens = 2
)

// punctuators lists the multi-character punctuation the lexers read as one token, longest first;
// ">>" is absent so that nested template arguments close one at a time.
var punctuators = []string{
	"...", "===", "!==",
	punctThinArrow, punctScope, "==", "!=", punctAmpAmp, "||", "<<", "++", "--", punctFatArrow, "?.", "??", "<=", ">=",
}

// openers are the brackets matchBrackets pairs; openerOf maps each closer to its opener.
var (
	openers  = []string{punctOpenParen, punctOpenBracket, punctOpenBrace}
	openerOf = map[string]string{
		punctCloseParen:   punctOpenParen,
		punctCloseBracket: punctOpenBracket,
		punctCloseBrace:   punctOpenBrace,
	}
)

// stringPrefixes are the C++ encoding prefixes that may precede a quote; a prefix ending in R
// opens a raw string.
var stringPrefixes = []string{"L", "u", "U", "u8", rawMark, "LR", "uR", "UR", "u8R"}

// regexLeaders are the TypeScript words after which a "/" opens a regular expression.
var regexLeaders = []string{
	"return", "typeof", "case", "in", "of", "delete", "void", "throw", "new", wordElse, wordDo, "yield", "await", "instanceof",
}

// divisionLeaders are the punctuation after which a "/" divides.
var divisionLeaders = []string{punctCloseParen, punctCloseBracket, punctCloseBrace, "++", "--"}

// controlWords start a statement whose brace after a ")" is its block.
var controlWords = []string{wordIf, wordFor, wordWhile, wordSwitch, wordCatch, wordWith, wordElse, wordFunction, wordAsync}

// blockLeaders are the words whose brace is a block of its own.
var blockLeaders = []string{wordElse, wordTry, wordFinally}

// continuationWords join the statement before them: an else, a catch, a finally.
var continuationWords = []string{wordElse, wordCatch, wordFinally}

// cppQualifiers may follow a C++ function's parameter list before its body.
var cppQualifiers = []string{
	wordConst, wordNoexcept, wordOverride, wordFinal, wordVolatile, wordMutable, punctAmp, punctAmpAmp,
}

// controlNames are the names a declaration-level "function" can never have: a control statement
// means the scanner left a function body.
var controlNames = []string{wordIf, wordFor, wordWhile, wordSwitch, wordCatch, wordDo}

// lambdaSpecs may sit between a lambda's parameter list and its body; returnTypePunct is the
// punctuation of a trailing return type; subscriptEnds are what a subscript's "[" follows.
var (
	lambdaSpecs     = []string{wordMutable, wordConstexpr, wordConsteval, wordNoexcept}
	returnTypePunct = []string{punctScope, punctLess, punctGreater, punctStar, punctAmp, punctAmpAmp, punctComma}
	subscriptEnds   = []string{punctCloseBracket, punctCloseParen}
)

// cppScopeWords open a brace that holds declarations or enumerators, not a function body.
var cppScopeWords = []string{wordClass, wordStruct, wordUnion, wordEnum, wordNamespace}

// cppAccessWords are the labels that end a class's declaration header.
var cppAccessWords = []string{wordPublic, wordPrivate, wordProtected}

// bodyEnders are the punctuation a function body's brace can follow besides "=>": the end of a
// parameter list or of a type.
var bodyEnders = []string{punctCloseParen, punctGreater, punctCloseBracket, punctCloseBrace}

// typeGroupEnds close a group inside a type annotation: an object type or a function type;
// typeLeaders are the punctuation that opens such a group inside a type.
var (
	typeGroupEnds = []string{punctCloseParen, punctCloseBrace}
	typeLeaders   = []string{punctColon, punctAmp, punctPipe, punctLess, tsOptional, punctComma}
)

// tsControlWords are the module-level statements whose parenthesis is not a function's.
var tsControlWords = []string{wordIf, wordFor, wordWhile, wordSwitch, wordCatch, wordWith}

// tsTypePunct is the punctuation a TypeScript type annotation can hold between a ")" and a body.
var tsTypePunct = []string{
	punctDot, punctLess, punctGreater, punctComma, punctOpenBracket, punctCloseBracket, punctPipe, punctAmp, tsOptional, punctFatArrow,
}
