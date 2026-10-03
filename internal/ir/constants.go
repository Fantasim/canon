package ir

import (
	"regexp"
	resyntax "regexp/syntax"
	"strings"
	"unicode/utf8"
)

// The emit targets of CODEGEN.md §2.1.
const (
	TargetGo Target = iota
	TargetCpp
	TargetTS
	TargetJSON
	TargetView
)

// The modes of a code target; json and view emits have none.
const (
	ModeNone Mode = iota
	ModeBaked
	ModeEmbedded
	ModeData
	ModeTypes
)

// The @cpp(access:) modes of a legacy struct (CPP-01); AccessNone is a generated class.
const (
	AccessNone Access = iota
	AccessFields
	AccessBoth
	AccessGetters
)

// The kinds of export fn (SPEC §9.4).
const (
	FnPrecomputed FnKind = iota
	FnLookup
	FnTranslated
)

// The operators of the portable subset (CONFORMANCE.md §2.2).
const (
	OpAdd Op = iota
	OpSub
	OpMul
	OpDiv
	OpMod
	OpNeg
	OpNot
	OpAnd
	OpOr
	OpEq
	OpNe
	OpLt
	OpLe
	OpGt
	OpGe
)

// The built-in functions of the portable subset (CONFORMANCE.md §2.2).
const (
	BuiltinFloat Builtin = iota
	BuiltinInt
	BuiltinMin
	BuiltinMax
	BuiltinAbs
	BuiltinClamp
	BuiltinFloor
	BuiltinCeil
	BuiltinRound
)

// The built-in functions and conversions of the portable subset, by name (CONFORMANCE.md §2.2).
const (
	fnMin      = "min"
	fnMax      = "max"
	convFloat  = "Float"
	convInt    = "Int"
	minMinMax  = 2 // min and max take two or more arguments
	clampArity = 3
	selfPrefix = "self_" // a read named like a declared parameter (CONFORMANCE.md §2.3)
)

// Where a path of self is read.
const (
	ctxValue readCtx = iota
	ctxCoalesce
	ctxIs
)

// NoBranch marks a discriminant member whose arm is Never.
const NoBranch = -1

const qnameSep = "."

// The tokens of canon-fp v1 (FINGERPRINT.md §4.2).
const (
	fpHeader      = "canon-fp v1\n"
	fpRoot        = "root "
	fpFn          = "fn "
	fpType        = "type @"
	fpRecord      = " record params="
	fpVariant     = " variant tag="
	fpEnum        = " enum wire="
	fpWireString  = "string"
	fpWireCode    = "code"
	fpCodes       = " codes="
	fpCase        = "case "
	fpMember      = "member "
	fpCode        = " code="
	fpField       = "field "
	fpInline      = "inline"
	fpPairs       = "pairs("
	fpOpt         = " opt="
	fpOptNo       = "0"
	fpOptYes      = "1"
	fpNoneKey     = " none="
	fpUnit        = " unit="
	fpEnc         = " enc="
	fpNone        = "-"
	fpNull        = "null"
	fpNever       = "never"
	fpKeyed       = "keyed("
	fpDep         = "dep("
	fpCaseType    = "case("
	fpMap         = "map("
	fpEncInt      = "int"
	fpSourceField = "field"
	fpSourceParam = "param"
	fpSourceKey   = "key"
	fpArgsOpen    = "<"
	fpArgsClose   = ">"
	fpPathOpen    = "["
	fpPathClose   = "]"
	fpComma       = ","
	fpEquals      = "="
	fpClose       = ")"
	fpSpace       = " "
	fpIndent      = "  "
	fpNewline     = "\n"
	fpAt          = "@"
	fpDollar      = "$"
	fpHashChars   = 8 // FINGERPRINT.md §2.1: the first 8 hex digits of the SHA-256
	decimalBase   = 10
)

// Paths, separators and names stage E composes.
const (
	curDir = "."
	// severalCopies is the fewest copies an emit with a list out writes (CODEGEN.md §2.1).
	severalCopies = 2
	pathSep       = "/"
	cppScope      = "::"
	docSep        = "\n\n" // GRAMMAR.md §9.1: package docs of several files
	boolFalse     = "false"
	boolTrue      = "true"
)

// maxCells is the largest lookup table (CODEGEN.md §5.10, E9002).
const maxCells = 65_536

// maxSafeInt is Number.MAX_SAFE_INTEGER, 2^53 - 1 (CODEGEN.md §4.1, E8101).
const maxSafeInt = 1<<53 - 1

// JSONExt ends a file-mode JSON `out` and every directory-mode file name (WIRE.md §8.1).
const JSONExt = ".json"

const underscore = "_"

// pairsOpen and pairsClose delimit a `pairs:` key template's slot index, `{i}` (WIRE.md §5.14).
const (
	pairsOpen  = "{"
	pairsClose = "}"
)

// The states of a class in classGraph's depth-first order.
const (
	unvisited = iota
	visiting
	visited
)

// identPattern is a plain identifier: a letter or `_`, then letters, digits or `_` (E8011).
var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// A C++ name reserved to the implementation holds `__` or starts `_[A-Z]` (CODEGEN.md §3.4).
const cppReservedRun = "__"

var cppReservedStart = regexp.MustCompile(`^_[A-Z]`)

// Generated C++ names gen/cpp declares (CODEGEN.md §3.3, §5, §7.2, CONFORMANCE.md §7.2).
const (
	cppDetail        = "detail"
	cppConformance   = "conformance"
	cppAsPrefix      = "As"
	cppConstPrefix   = "k"
	cppFromWire      = "FromWire"
	cppKey           = "Key"
	cppKeys          = "Keys"
	CppLoad          = "Load" // static Load of records, containers, the snapshot; detail's Load<V>
	cppRefSuffix     = "ref_"
	cppByPrefix      = "by"
	cppAccessSuffix  = "Access"
	cppRunPrefix     = "Run"
	cppVectorSuffix  = "Vector"
	cppShow          = "Show"   // the conformance file's printer of an optional input
	CppDecode        = "Decode" // detail's decoder overloads (CODEGEN.md §7.2), Decode<Alias>
	CppResolve       = "Resolve"
	CppVariantMember = "value_" // a variant's or dependent type's std::variant (§5.5, §5.6)
	CppToName        = "ToName" // the enum helpers, one overload per enum (CODEGEN.md §5.2)
	CppToWire        = "ToWire"
	CppMatchPattern  = "MatchPattern" // §7.7's pattern search, the input helper a loader calls by name
	CppEnvText       = "EnvText"      // §7.7's variable reader, the input helper each LoadInputs block calls
)

// The fixed Go names the plan declares and gen/go's templates write (CODEGEN.md §5.2–§6.2).
const (
	GoID           = "ID" // the ID method, and the suffix of an id type and a key getter
	GoRetired      = "Retired"
	GoKind         = "Kind" // the Kind method, and the suffix of a kind enum
	GoString       = "String"
	GoWire         = "Wire"
	GoCode         = "Code"
	GoLen          = "Len"
	GoAt           = "At"
	GoAll          = "All"
	GoFind         = "Find"
	GoGet          = "Get" // a table container's Get, and the prefix of an accessor
	GoRows         = "rows"
	GoIDStore      = "id"
	GoRetiredStore = "retired"
	GoKindStore    = "kind"
	GoCaseStore    = "value"
)

// Generated Go names and the reference layout's own names (CODEGEN.md §3.3, §5.2–§6.2).
const (
	goAsPrefix         = "As"
	goParsePrefix      = "Parse"
	goMembersSuffix    = "Members"
	goFromCodeSuffix   = "FromCode"
	goFindByPrefix     = "FindBy"
	goIndexSuffix      = "Index"
	goTableSuffix      = "Table"
	goDataSuffix       = "Data"
	goValuesSuffix     = "Values"
	goBuildPrefix      = "build"
	goDataLocal        = "d"
	goKeyStoreSuffix   = "_id"
	goKeysStoreSuffix  = "_ids"
	goOKStoreSuffix    = "_ok"
	goIndexLocalSuffix = "_i"
	goPluralSuffix     = "s"
	goSelf             = "self"
	goScopePackage     = "package"
	goParamsSuffix     = "()"
)

// The standard packages generated Go imports, by the name code uses (CODEGEN.md §2.8).
const (
	goRT       = "rt"
	goTime     = "time"
	goIter     = "iter"
	goSync     = "sync"
	goStrconv  = "strconv"
	goMath     = "math"
	goJSON     = "json"
	goFmt      = "fmt"
	goErrors   = "errors"
	goStrings  = "strings"
	goFilepath = "filepath"
	goAtomic   = "atomic"
	goTesting  = "testing"
	goSlices   = "slices"
	goRegexp   = "regexp"
)

// Names both generators' stores and loaders write (CODEGEN.md §5.11, §6.1).
const (
	storeCurrent = "Current"
	storeReload  = "Reload"
	goOut        = "out"
	goValues     = "values"
)

// Runtime inputs: LoadInputs, Go's input_<T>_<store>, C++'s detail::<P>Inputs (CODEGEN.md §5.12).
const (
	loadInputs           = "LoadInputs"
	inputLineSep         = ": "
	goInputPrefix        = "input_"
	goInputOKSuffix      = "_OK"
	goInputPatternSuffix = "_Pattern"
	goInputsLoaded       = "inputsLoaded_"
	goInputErrs          = "errs"
	cppInputsSuffix      = "Inputs"
	cppInputsLoaded      = "InputsLoaded"
	branchWord           = "Branch" // dependent types (§5.6): TBranch, Go's Branch, C++'s GetBranch
	goBranchStore        = "branch"
	asValueSuffix        = "Value"       // As<Branch>Value of a ref into a load.defines table
	defineValueSuffix    = asValueSuffix // a define ref's value getter, <F>Value (CODEGEN.md §5.8)
	goDefinesPrefix      = "defines"
	goDefineStoreSuffix  = "_value"
	goJSONDefine         = "jsonDefine"
	cppDefinesSuffix     = "Defines"
)

// How another package's same C++ name meets a name: only overloads, TU-locals, namespaces pass.
const (
	meetsNever cppMeet = iota
	meetsOverload
	meetsLocal
	meetsNamespace
)

// The reasons of LoadInputs' failure lines (InputReasonText).
const (
	InputNotSet InputReason = iota
	InputNotValid
	InputOutsideRange
	InputNoMatch
	InputNotMember
)

// inputReasons are the failure-line reasons, byte for byte in every target (CODEGEN.md §5.12).
var inputReasons = [...]string{
	InputNotSet:       "not set",
	InputNotValid:     "not a valid %s",
	InputOutsideRange: "outside its refinement range",
	InputNoMatch:      "does not match its pattern",
	InputNotMember:    "not a member of %s",
}

// inputFieldPlaceholder is E8302's one template placeholder, `{field}` (InputGetterFailure).
const inputFieldPlaceholder = "{%s}"

// Data mode's generated names and its loaders' fixed locals (CODEGEN.md §3.3, §5.9–§6.1).
const (
	goSchemaSuffix     = "Schema"
	goLoadPrefix       = "Load"
	goLoadLocalPrefix  = "load"
	goDecodePrefix     = "decode"
	goResolvePrefix    = "resolve"
	goSnapshotSuffix   = "Snapshot"
	goStoreSuffix      = "Store" // also the store variable
	goJSONRowID        = "jsonRowID"
	goJSONTable        = "jsonTable"
	goScopeLocals      = "locals"
	goScopeConformance = "conformance imports"
	goInt64Bits        = 64
	goFloat32Bits      = 32
)

// Translated fns and the conformance file (CODEGEN.md §3.3, §5.10; CONFORMANCE.md §7).
const (
	goTestPrefix        = "Test"
	goConformanceSuffix = "Conformance"
	goOKSuffix          = "Ok"
	goVectorsScope      = " vectors"
	goTestingLocal      = "t"
	goCanonCatch        = "canonCatch"
	goCanonShow         = "canonShow"
	goWant              = "want" // a vector's expected value, and a loader's local
)

// The kinds of GoNameProblem.
const (
	// GoCollision is two names equal in one scope (E8005).
	GoCollision GoProblemKind = iota
	// GoNotIdentifier is a generated name that is no Go identifier.
	GoNotIdentifier
	// GoUnexported is a @go(name:) override, an identifier but not exported (E8011).
	GoUnexported
	// GoOverrideInvalid is a @go(name:) override that is no Go identifier at all (E8011 `override`).
	GoOverrideInvalid
)

// The code points of `.` (all but `\n`: the Perl flags lack DotNL) and of any, as rune pairs.
var (
	anyRuneNotNL = []rune{0, '\n' - 1, '\n' + 1, utf8.MaxRune}
	anyRune      = []rune{0, utf8.MaxRune}
)

// The states of a pattern automaton (EVALUATION.md §11.3); gen/cpp writes each op as its number.
const (
	PatternAccept PatternOp = iota // the search matches
	PatternSplit                   // continue at Out and at Alt, consuming nothing
	PatternAnchor                  // continue at Out where the position is every one of At
	PatternStep                    // consume one code point within Runes, continue at Out
)

// The positions a PatternAnchor state asserts, as flags.
const (
	PatternAtBegin PatternAt = 1 << iota // the start of the text: `^`
	PatternAtEnd                         // its end: `$`
)

// instStates convert each instruction a reachable state can be, indexed by its op; nil is refused.
var instStates = [...]func(*automatonBuilder, *resyntax.Inst) PatternState{
	resyntax.InstMatch: (*automatonBuilder).accept, resyntax.InstFail: (*automatonBuilder).fail,
	resyntax.InstAlt: (*automatonBuilder).split, resyntax.InstEmptyWidth: (*automatonBuilder).anchor,
	resyntax.InstRune: (*automatonBuilder).runes, resyntax.InstRune1: (*automatonBuilder).rune1,
	resyntax.InstRuneAny: (*automatonBuilder).any, resyntax.InstRuneAnyNotNL: (*automatonBuilder).anyNotNL,
}

// patternEmpty maps Go's assertions to the ones an automaton keeps; any other (multi-line anchors, word boundaries) has no state.
var patternEmpty = []struct {
	op uint32
	at PatternAt
}{{uint32(resyntax.EmptyBeginText), PatternAtBegin}, {uint32(resyntax.EmptyEndText), PatternAtEnd}}

// The names of a TypeScript module that are not the package's (CODEGEN.md §3.4, §8.2): reserved words, predefined type names, and the helper units the generator may write, runtime block and decoders.
var (
	tsReserved   = strings.Fields("await break case catch class const continue debugger default delete do else enum export extends false finally for function if import in instanceof new null return super switch this throw true try typeof var void while with yield implements interface let package private protected public static undefined NaN Infinity arguments eval")
	tsPredefined = strings.Fields("string number boolean symbol bigint object any unknown never void")
	tsHelpers    = strings.Fields("CanonEvalError canonFail canonInt canonAdd canonSub canonMul canonDiv canonMod canonNeg canonAbs canonClamp canonF canonMinF canonMaxF canonClampF canonToInt canonFloor canonCeil canonRound canonDivDuration canonCheckRange canonCheckWidth canonF32 CanonMap CanonTable canonTable canonFreeze canonEnvelope decOrder decTokenKey decToken decText decDecimal decSame decKeys decFail decGet decIsObject decObject decArray decAt decEmptyObject decEmptyArray decBool decBit decInt decBig decFloat decCompare decHalf decF32 decString decDuration decEnum decCode decBits decKeyed decForeign decList decTable decIntKey decBigKey decMap decPairs decParse")
)

// tsIdentPattern is a TypeScript identifier as generated code spells it: `$` starts a method's pure function.
var tsIdentPattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

const (
	tsHelperOrigin = "the TypeScript helper block"
	tsModuleScope  = "module"
	tsPureMark     = "$"
	tsIDSuffix     = "Id"
	tsKindSuffix   = "Kind"
	tsBranchSuffix = "Branch"
	tsDecodePrefix = "decode"
	tsParsePrefix  = "parse"
	tsReadPrefix   = "read"
	tsSchemaSuffix = "Schema"
	tsMembersName  = "Members"
	tsNamesName    = "Names"
	tsIndexName    = "Index"
	tsCodesName    = "Codes"
	tsIDProp       = "id"
	tsRetiredProp  = "retired"
	tsKindProp     = "kind"
)
