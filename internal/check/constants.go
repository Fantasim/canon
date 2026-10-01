package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// NoneIndex is the index MatchInfo.Covers gives the `none` pattern (TYPES.md §12.6).
const NoneIndex = -1

// The object kinds; ObjParam is any parameter, lambdas' included, ObjLocal any other binder.
const (
	ObjConst ObjKind = iota
	ObjLet
	ObjFn
	ObjMethod
	ObjParam
	ObjLocal
	ObjField
	ObjMember
	ObjCase
	ObjEntry
	ObjBuiltin
	ObjTypeName
	ObjPackage
	ObjLayer
	ObjCheck
	ObjTest
	ObjWidget
)

// What `.name` selects (TYPES.md §3.5).
const (
	SelField SelKind = iota
	SelEntry
	SelBuiltinMember
	SelMethod
)

// The conversions of TYPES.md §6.2.
const (
	ConvWrap          ConvKind = iota // T to T?; Inner, if any, first
	ConvDeref                         // ref T to T; E3501 deferred
	ConvEntryToRef                    // T to ref T; E3503 deferred
	ConvIntLitToFloat                 // an integer literal to Float
	ConvCaseToVariant                 // V.c to V
	ConvToList                        // table T or a keyed list to [T], identities kept
	ConvElements                      // per element of a list, map or pair: Key and Inner
	ConvPresent                       // S? to T?: Inner on a present value
)

// What a call calls; CalleeConvert is `Int(f)` (STDLIB.md §2.1), CalleeLambda a function value.
const (
	CalleeFn CalleeKind = iota
	CalleeMethod
	CalleeBuiltin
	CalleeConvert
	CalleeLambda
)

// The classifications of TYPES.md §5.2; LitError is a literal no row classifies.
const (
	LitRecord LitKind = iota
	LitTable
	LitMap
	LitMapComp
	LitError
)

// Names shared by several kinds.
const (
	nameField   = "Field"
	nameMethod  = "Method"
	nameFn      = "Fn"
	nameBuiltin = "Builtin"
	nameEntry   = "Entry"
)

var objNames = [...]string{
	ObjConst: "Const", ObjLet: "Let", ObjFn: nameFn, ObjMethod: nameMethod, ObjParam: "Param",
	ObjLocal: "Local", ObjField: nameField, ObjMember: "Member", ObjCase: "Case", ObjEntry: nameEntry,
	ObjBuiltin: nameBuiltin, ObjTypeName: "TypeName", ObjPackage: "Package", ObjLayer: "Layer",
	ObjCheck: "Check", ObjTest: "Test", ObjWidget: "Widget",
}

var selNames = [...]string{
	SelField: nameField, SelEntry: nameEntry, SelBuiltinMember: "BuiltinMember", SelMethod: nameMethod,
}

var convNames = [...]string{
	ConvWrap: "Wrap", ConvDeref: "Deref", ConvEntryToRef: "EntryToRef", ConvIntLitToFloat: "IntLitToFloat",
	ConvCaseToVariant: "CaseToVariant", ConvToList: "ToList", ConvElements: "Elements", ConvPresent: "Present",
}

var calleeNames = [...]string{
	CalleeFn: nameFn, CalleeMethod: nameMethod, CalleeBuiltin: nameBuiltin, CalleeConvert: "Convert",
	CalleeLambda: "Lambda",
}

var litNames = [...]string{
	LitRecord: "RecordLit", LitTable: "TableLit", LitMap: "MapLit", LitMapComp: "MapComp", LitError: "ErrorLit",
}

// The resolution states of a top-level declaration.
const (
	stateNone resolveState = iota
	stateResolving
	stateDone
)

// The flags of an env: modeJoin types a branch a join may still type (TYPES.md §6.4).
const (
	modeJoin mode = 1 << iota
)

// What a type position allows: a function type, `_`, `stable table`.
const (
	posFn typePos = 1 << iota
	posAny
	posStable
)

// The constraints of STDLIB.md §1.1, and consString for join's elements.
const (
	consNone constraint = iota
	consNum
	consNumD
	consOrd
	consEq
	consKey
	consString
)

// The states of the import cycle walk.
const (
	unvisited visit = iota
	visiting
	visited
)

// noConstant is env.what outside a constant expression.
const noConstant = diag.Kind(^uint8(0))

// Limits and fixed numbers.
const (
	selfID          = 0
	hintDistance    = 2
	pairArity       = 2
	pairCount       = 2
	minPathSegments = 2
	bits64          = 64
	decimalBase     = 10
	maxBit          = int64(1) << 62
	underscoreByte  = '_'
)

// Punctuation and fixed words.
const (
	dot          = "."
	slash        = "/"
	colon        = ":"
	hash         = "#"
	openBracket  = "["
	closeBracket = "]"
	openParen    = "("
	closeParen   = ")"
	unionMark    = "|" // marks a literal of a literal union among canonical keys (TYPES.md §13.2)
	underscore   = "_"
	hyphen       = "-"
	dollar       = "$"
	rootSigil    = "@"
	rootDir      = dot      // the project directory as a display path (API.md §1.3)
	syntaxOwner  = "syntax" // the registry owner of the lexer's and parser's codes (DECISIONS 214)
	layoutAnchor = dot      // a stand-in project directory: a name is read from out as declared
	parentDir    = ".."     // the parent segment of a project-relative path (WIRE.md §2.2)
	projectWord  = "project"
	spaceSep     = " "
	// sharingEntries is the fewest entries of an out list that can share an owning root (CODEGEN.md §2.8).
	sharingEntries = 2
	cppScope       = "::"
	optDotText     = "?."
	forceText      = "!"
	emptyArray     = "[]"
	emptyObject    = "{}"
	indexText      = emptyArray
	seqText        = "Seq("
	expMark        = "e"
	tsSuffix       = ".ts"
	extForbidden   = "./\\"
	itName         = "it"
	selfKey        = "self"
	noneWord       = syntax.PropNone
	trueWord       = "true"
	falseWord      = "false"

	envNamePattern = `^[A-Za-z_][A-Za-z0-9_]*$`
	identPattern   = envNamePattern
)

// Built-in members (STDLIB.md §3).
const (
	idMember      = "id"
	retiredMember = "retired"
	kindMember    = "kind"
	nameMember    = "name"
	indexMember   = "index"
	wireMember    = "wire"
	codeMember    = "code"
	startMember   = "start"
	endMember     = "end"
	defaultTag    = kindMember
)

// View words, properties and studio names (VIEWMODEL.md G16), translation key segments
// (I18N.md K4). The words are internal/syntax's (the grammar owns key syntax); check keeps
// only its own names for them.
const (
	titleWord, subtitleWord, showWord, textWord, stepWord        = syntax.WordTitle, syntax.WordSubtitle, syntax.WordShow, loadText, syntax.PropStep
	fieldSegment, methodWord, caseWord, memberWord, checkSegment = syntax.WordField, syntax.WordMethod, jsonCase, syntax.WordMember, syntax.WordCheck
	keyName                                                      = "key"
)

// Annotations and their arguments (GRAMMAR.md §8.3, WIRE.md §4).
const (
	annotJSON       = "json"
	annotDeprecated = syntax.AnnDeprecated
	annotStable     = "stable"
	annotCodes      = "codes"
	jsonCodes       = annotCodes
	jsonPath        = "path"
	jsonInline      = "inline"
	jsonInt         = "int"
	jsonBits        = "bits"
	jsonUnit        = "unit"
	jsonNone        = noneWord
	jsonPairsName   = "pairs"
	jsonCase        = syntax.ArgCase
	jsonTag         = "tag"
	caseSnake       = "snake"
	caseKebab       = "kebab"
	caseUpperSnake  = "upper_snake"
	letterS         = "s"
)

// The escapes, groups and quantifier spellings of the portable pattern subset (EVALUATION.md §11.3).
const (
	perlClassLetters = "dDwWsS"
	patternEscapes   = perlClassLetters + `\^$.|?*+()[]{}/`
	classEscapes     = patternEscapes + hyphen
	flagGroup        = "(?"
	plainGroup       = flagGroup + colon
	repeatPattern    = `^\{(0|[1-9][0-9]*)(,(0|[1-9][0-9]*)?)?\}`
)

// backslash starts an escape in a regex.
const backslash = '\\'

// The digits a pairs slot starts with: the least, and the least without a leading zero.
const (
	zeroDigit = '0'
	oneDigit  = '1'
)

// pairSlot is the position variable of a `@json(pairs:)` template (WIRE.md §5.14).
var pairSlot = openBrace + paramI + closeBrace

// unitSymbols are the wire units in types.Unit order.
var unitSymbols = [...]string{types.UnitMs: "ms", types.UnitS: letterS, types.UnitM: "m", types.UnitH: "h", types.UnitD: "d"}

// Emit targets and options (CODEGEN.md §2.1, WIRE.md §8.1).
const (
	TargetGo     = "go"
	TargetCpp    = "cpp"
	TargetTS     = "ts"
	TargetJSON   = annotJSON
	TargetView   = "view"
	OptOut       = "out"
	OptMode      = "mode"
	OptPackage   = "package"
	OptNamespace = "namespace"
	OptValues    = methodValues
	ModeBaked    = "baked"
	ModeEmbedded = "embedded"
	ModeData     = "data"
	ModeTypes    = "types"
)

var codeModes = []string{ModeBaked, ModeEmbedded, ModeData, ModeTypes}

// emitSpecs are the targets of CODEGEN.md §2.1 with their options and modes.
var emitSpecs = map[string]emitSpec{
	TargetGo:   {options: []string{OptOut, OptMode, OptPackage, OptValues}, modes: codeModes},
	TargetCpp:  {options: []string{OptOut, OptMode, OptNamespace, OptValues}, modes: codeModes},
	TargetTS:   {options: []string{OptOut, OptMode, OptValues}, modes: codeModes},
	TargetJSON: {options: []string{OptOut, OptValues}},
	TargetView: {options: []string{OptOut}},
}

// Load forms and options (WIRE.md §6.1).
const (
	loadName      = "load"
	loadForm      = ""
	loadDefines   = "defines"
	loadText      = syntax.WordText
	loadCSV       = "csv"
	optionFormat  = "format"
	optionPartial = "partial"
	optionHeader  = "header"
)

// Type parameter names of the built-in signatures.
const (
	nameT    = "T"
	nameU    = "U"
	nameK    = "K"
	nameV    = "V"
	nameKT   = "KT"
	nameR    = "R"
	nameRefT = "ref T"
)

// Built-in types (TYPES.md §3.3 step 6).
const (
	typeInt      = "Int"
	typeInt8     = "Int8"
	typeInt16    = "Int16"
	typeInt32    = "Int32"
	typeUInt8    = "UInt8"
	typeUInt16   = "UInt16"
	typeUInt32   = "UInt32"
	typeUInt64   = "UInt64"
	typeFloat    = "Float"
	typeFloat32  = "Float32"
	typeString   = "String"
	typeBool     = "Bool"
	typeDuration = "Duration"
	typeRange    = "Range"
	typeNever    = "Never"
	typeDefine   = "Define"
)

// Built-in free functions (STDLIB.md §2, §10).
const (
	fnAbs       = "abs"
	fnMin       = methodMin
	fnMax       = methodMax
	fnClamp     = "clamp"
	fnFloor     = "floor"
	fnCeil      = "ceil"
	fnRound     = "round"
	fnSqrt      = "sqrt"
	fnPow       = "pow"
	fnReachable = "reachable"
	fnCycles    = "cycles"
	fnTopoSort  = "topoSort"
	fnFail      = "fail"
	fnWarn      = "warn"
)

// Built-in methods (STDLIB.md §4 to §7, §10).
const (
	methodLen        = "len"
	methodIsEmpty    = "isEmpty"
	methodFirst      = "first"
	methodLast       = "last"
	methodContains   = "contains"
	methodIndexOf    = "indexOf"
	methodMap        = "map"
	methodFilter     = "filter"
	methodFlatMap    = "flatMap"
	methodFlatten    = "flatten"
	methodReverse    = "reverse"
	methodSortBy     = "sortBy"
	methodUnique     = "unique"
	methodEnumerate  = "enumerate"
	methodPairs      = jsonPairsName
	methodZip        = "zip"
	methodIntersect  = "intersect"
	methodUnion      = "union"
	methodDiff       = "diff"
	methodGroupBy    = "groupBy"
	methodToMap      = "toMap"
	methodJoin       = "join"
	methodAny        = "any"
	methodAll        = "all"
	methodCount      = "count"
	methodIsUnique   = "isUnique"
	methodSum        = "sum"
	methodMin        = "min"
	methodMax        = "max"
	methodMinBy      = "minBy"
	methodMaxBy      = "maxBy"
	methodGet        = "get"
	methodFind       = "find"
	methodAt         = "at"
	methodKeys       = "keys"
	methodValues     = "values"
	methodActive     = "active"
	methodStartsWith = "startsWith"
	methodEndsWith   = "endsWith"
	methodSplit      = "split"
	methodTrim       = "trim"
	methodLower      = "lower"
	methodUpper      = "upper"
	methodReplace    = "replace"
	methodMatches    = "matches"
)

// Parameter names of the built-ins.
const (
	paramX       = "x"
	paramY       = "y"
	paramA       = "a"
	paramB       = "b"
	paramF       = "f"
	paramI       = "i"
	paramK       = "k"
	paramS       = letterS
	paramRe      = "re"
	paramSep     = "sep"
	paramPred    = "pred"
	paramOther   = "other"
	paramKeyF    = "keyF"
	paramValF    = "valF"
	paramLo      = "lo"
	paramHi      = "hi"
	paramFrom    = "from"
	paramNext    = "next"
	paramXs      = "xs"
	paramAt      = methodAt
	paramMessage = "message"
)

// compoundOps is the operator of each compound assignment (TYPES.md §7.1).
var compoundOps = map[syntax.TokenKind]syntax.TokenKind{
	syntax.TokAddAssign: syntax.TokPlus, syntax.TokSubAssign: syntax.TokMinus,
	syntax.TokMulAssign: syntax.TokStar, syntax.TokDivAssign: syntax.TokSlash,
}

var (
	allArith = map[syntax.TokenKind]bool{syntax.TokPlus: true, syntax.TokMinus: true, syntax.TokStar: true, syntax.TokSlash: true, syntax.TokPercent: true}
	fourOps  = map[syntax.TokenKind]bool{syntax.TokPlus: true, syntax.TokMinus: true, syntax.TokStar: true, syntax.TokSlash: true}
	addSub   = map[syntax.TokenKind]bool{syntax.TokPlus: true, syntax.TokMinus: true}
	mulOnly  = map[syntax.TokenKind]bool{syntax.TokStar: true}
	divOnly  = map[syntax.TokenKind]bool{syntax.TokSlash: true}
	mulDiv   = map[syntax.TokenKind]bool{syntax.TokStar: true, syntax.TokSlash: true}
	plusOnly = map[syntax.TokenKind]bool{syntax.TokPlus: true}
)

// arithRow is a row of the operator table of TYPES.md §7.1 on scalars.
type arithRow struct {
	ka, kb types.Kind
	ops    map[syntax.TokenKind]bool
	result types.Type
}

var arithRows = []arithRow{
	{ka: types.Int, kb: types.Int, ops: allArith, result: types.IntType},
	{ka: types.Float, kb: types.Float, ops: fourOps, result: types.FloatType},
	{ka: types.Duration, kb: types.Duration, ops: addSub, result: types.DurationType},
	{ka: types.Duration, kb: types.Int, ops: mulDiv, result: types.DurationType},
	{ka: types.Int, kb: types.Duration, ops: mulOnly, result: types.DurationType},
	{ka: types.Duration, kb: types.Duration, ops: divOnly, result: types.FloatType},
	{ka: types.String, kb: types.String, ops: plusOnly, result: types.StringType},
}

const (
	openBrace      = "{"
	closeBrace     = "}"
	newline        = "\n"
	manyPaths      = 2 // E1903 needs to know only "more than one"
	maxLiteralText = 40

	elidedLiteral sourceText = "{ … }"
)

// literalCodes are the lexer's findings inside a literal token (DECISIONS 215).
var literalCodes = []interface{ Def() *diag.Def }{
	diag.E1101, diag.E1102, diag.E1107, diag.E1109, diag.E1110, diag.E1111, diag.E1112, diag.E1113, diag.E1114, diag.E1122, diag.E1124,
}

// The forms of a let's value that the checker reads as written (TYPES.md §4.1, §9.3, §10.2).
const (
	formOther = iota
	formTable
	formList
	formDefines
)
