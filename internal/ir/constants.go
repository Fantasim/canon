package ir

import "regexp"

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
	curDir    = "."
	parentDir = ".."
	pathSep   = "/"
	cppScope  = "::"
	docSep    = "\n\n" // GRAMMAR.md §9.1: package docs of several files
	boolFalse = "false"
	boolTrue  = "true"
)

// maxCells is the largest lookup table (CODEGEN.md §5.10, E9002).
const maxCells = 65_536

// maxSafeInt is Number.MAX_SAFE_INTEGER, 2^53 - 1 (CODEGEN.md §4.1, E8101).
const maxSafeInt = 1<<53 - 1

// definesRefused are the emits refusing a ref into a load.defines table (decisions 180, 194).
var definesRefused = [TargetView + 1][ModeTypes + 1]bool{
	TargetGo: {ModeBaked: true, ModeData: true},
}

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
	asValueSuffix        = "Value" // As<Branch>Value of a ref into a load.defines table
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
