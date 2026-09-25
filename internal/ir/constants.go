package ir

import (
	"regexp"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
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

// portableBuiltins are the built-in functions of the portable subset, with the kinds their arguments may have (CONFORMANCE.md §2.2, §3); arity is exact, or the least when variadic.
var portableBuiltins = map[string]builtinSpec{
	fnMin: {fn: BuiltinMin, arity: minMinMax, variadic: true, kinds: numericKinds}, fnMax: {fn: BuiltinMax, arity: minMinMax, variadic: true, kinds: numericKinds},
	"abs": {fn: BuiltinAbs, arity: 1, kinds: numericKinds}, "clamp": {fn: BuiltinClamp, arity: clampArity, kinds: numericKinds},
	"floor": {fn: BuiltinFloor, arity: 1, kinds: floatKinds}, "ceil": {fn: BuiltinCeil, arity: 1, kinds: floatKinds},
	"round": {fn: BuiltinRound, arity: 1, kinds: floatKinds},
}

// portableConversions are `Float(i)` and `Int(f)`, with the kind each converts from (CONFORMANCE.md §3).
var portableConversions = map[string]builtinSpec{
	convFloat: {fn: BuiltinFloat, arity: 1, kinds: map[types.Kind]bool{types.Int: true}},
	convInt:   {fn: BuiltinInt, arity: 1, kinds: floatKinds},
}

// The kinds of the portable subset: of arithmetic, of a parameter, read or equality, of a result, of an interpolation (CONFORMANCE.md §2.1, §2.2).
var (
	numericKinds  = map[types.Kind]bool{types.Int: true, types.Float: true, types.Duration: true}
	floatKinds    = map[types.Kind]bool{types.Float: true}
	scalarKinds   = map[types.Kind]bool{types.Bool: true, types.Int: true, types.Float: true, types.String: true, types.Duration: true, types.Enum: true}
	resultKinds   = map[types.Kind]bool{types.Bool: true, types.Int: true, types.Float: true, types.String: true, types.Duration: true, types.Enum: true, types.Ref: true}
	templateKinds = map[types.Kind]bool{types.String: true, types.Int: true, types.Enum: true}
	// untemplated are the kinds E9005 names in a template; any other kind there is E9001.
	untemplated = map[types.Kind]bool{types.Float: true, types.Duration: true}
)

// binaryOps are the binary operators of the portable subset by token; `??` is Coalesce (CONFORMANCE.md §2.2).
var binaryOps = map[syntax.TokenKind]Op{
	syntax.TokPlus: OpAdd, syntax.TokMinus: OpSub, syntax.TokStar: OpMul, syntax.TokSlash: OpDiv,
	syntax.TokPercent: OpMod, syntax.KwAnd: OpAnd, syntax.KwOr: OpOr, syntax.TokEq: OpEq,
	syntax.TokNe: OpNe, syntax.TokLt: OpLt, syntax.TokLe: OpLe, syntax.TokGt: OpGt, syntax.TokGe: OpGe,
}

// unaryOps are the unary operators of the portable subset, with the kinds of their operand.
var unaryOps = map[syntax.TokenKind]unarySpec{
	syntax.TokMinus: {op: OpNeg, kinds: numericKinds},
	syntax.KwNot:    {op: OpNot, kinds: map[types.Kind]bool{types.Bool: true}},
}

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

// fpComposites opens each composite type of §4.4 but the keyed list.
var fpComposites = map[types.Kind]string{
	types.List: "list(", types.Table: "table(", types.Optional: "opt(", types.Map: fpMap,
	types.DepMap: fpMap, types.Ref: "ref(", types.LitUnion: "union(",
}

// fpEncNames is `enc=` of a field (§4.5).
var fpEncNames = [...]string{types.EncPlain: fpNone, types.EncInt: "int", types.EncBits: "bits"}

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

// The words of targets, modes and @cpp(access:) (CODEGEN.md §2.1, CPP-01), by value.
var (
	targetWords = [...]string{TargetGo: check.TargetGo, TargetCpp: check.TargetCpp, TargetTS: check.TargetTS, TargetJSON: check.TargetJSON, TargetView: check.TargetView}
	modeWords   = [...]string{ModeNone: "", ModeBaked: check.ModeBaked, ModeEmbedded: check.ModeEmbedded, ModeData: check.ModeData, ModeTypes: check.ModeTypes}
	// accessWords are syntax's @cpp(access:) symbols, indexed by Access: AccessNone has none.
	accessWords = append([]string{""}, syntax.AccessModes()...)
)

// branchKinds are the kinds a dependent type's branch may have (CODEGEN.md §5.6, E8017).
var branchKinds = map[types.Kind]bool{
	types.Bool: true, types.Int: true, types.Float: true, types.String: true, types.Duration: true,
	types.Enum: true, types.Ref: true,
}

// definesRefused are the emits whose generator refuses a ref into a load.defines table, having no define value getter nor table yet (decisions 180, 194): baked and data go; a mode gen/go does not have reports nothing.
var definesRefused = [TargetView + 1][ModeTypes + 1]bool{
	TargetGo: {ModeBaked: true, ModeData: true},
}

// maxSafeInt is Number.MAX_SAFE_INTEGER, 2^53 - 1 (CODEGEN.md §4.1, E8101).
const maxSafeInt = 1<<53 - 1

// JSONExt ends a file-mode JSON `out` and every directory-mode file name (WIRE.md §8.1).
const JSONExt = ".json"

const underscore = "_"

// pairsOpen and pairsClose delimit the slot index of a `pairs:` key template, `{i}` (WIRE.md §5.14).
const (
	pairsOpen  = "{"
	pairsClose = "}"
)

// recordKinds are the kinds whose values are records: a record, a variant, a case.
var recordKinds = map[types.Kind]bool{types.Record: true, types.Variant: true, types.Case: true}

// The states of a class in classGraph's depth-first order.
const (
	unvisited = iota
	visiting
	visited
)

// identPattern is a plain identifier: a letter or underscore, then letters, digits or underscores (CODEGEN.md §3.5, E8011).
var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// goInitialisms is the closed initialism list of GoCap (CODEGEN.md §3.2).
var goInitialisms = map[string]bool{
	"id": true, "url": true, "api": true, "http": true, "json": true, "ui": true,
	"db": true, "ip": true, "hp": true, "mp": true, "ts": true,
}

// goPredeclared are Go's predeclared identifiers (CODEGEN.md §3.4).
var goPredeclared = map[string]bool{
	"any": true, "append": true, "bool": true, "byte": true, "cap": true, "clear": true,
	"close": true, "comparable": true, "complex": true, "complex64": true, "complex128": true,
	"copy": true, "delete": true, "error": true, boolFalse: true, "float32": true, "float64": true,
	"imag": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"iota": true, "len": true, "make": true, "max": true, "min": true, "new": true, "nil": true,
	"panic": true, "print": true, "println": true, "real": true, "recover": true, "rune": true,
	"string": true, boolTrue: true, "uint": true, "uint8": true, "uint16": true, "uint32": true,
	"uint64": true, "uintptr": true,
}

// goImportNames are the package names generated Go files import (CODEGEN.md §3.4); baked Go writes goStdImports of them.
var goImportNames = map[string]bool{
	goRT: true, goJSON: true, goFmt: true, goIter: true, "os": true, goFilepath: true,
	goAtomic: true, goSync: true, goTime: true, goErrors: true, goStrconv: true, goStrings: true,
	goMath: true, "regexp": true, "embed": true, goSlices: true,
}

// cppOwnNames are the namespaces generated C++ declares itself, beside check's (CODEGEN.md §3.4).
var cppOwnNames = map[string]bool{cppDetail: true, cppConformance: true}

// A C++ name reserved to the implementation: one holding `__`, or starting with `_` and an upper-case letter (CODEGEN.md §3.4).
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
	cppLoad          = "Load"
	cppRefSuffix     = "ref_"
	cppByPrefix      = "by"
	cppAccessSuffix  = "Access"
	cppRunPrefix     = "Run"
	cppVectorSuffix  = "Vector"
	cppShow          = "Show"   // the conformance file's printer of an optional input
	cppDecode        = "Decode" // detail's decoders, one overload per class (CODEGEN.md §7.2)
	cppResolve       = "Resolve"
	cppVariantMember = "value_" // a variant's std::variant (CODEGEN.md §5.5)
)

var (
	// cppEntryMembers are a table entry's id and retired getters and members (CODEGEN.md §5.3).
	cppEntryMembers = []string{"GetId", "id_", "GetRetired", "retired_"}
	// cppContainerMembers are every container's methods and rows (CODEGEN.md §5.9).
	cppContainerMembers = []string{GoLen, GoAt, GoAll, GoFind, "rows_"}
	// cppConformanceOwn are the conformance namespace's own names (CONFORMANCE.md §7.2).
	cppConformanceOwn = []string{"g_code", "Capture"}
	// cppStoreMembers are the store's methods and its atomic pointer (CODEGEN.md §5.11, T9).
	cppStoreMembers = []string{"Current", "Reload", "current_"}
	// cppEnumOverloads are the namespace's ToName and ToWire, one overload per enum (CODEGEN.md §5.2).
	cppEnumOverloads = []string{"ToName", "ToWire"}
)

// The fixed Go names of generated code, which the name plan declares and gen/go's templates write (CODEGEN.md §5.2–§5.9, §6.2).
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

// Generated Go names and the reference layout's own names (CODEGEN.md §3.3, §5.2–§5.10, §6.2; decisions 121, 193).
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

// The standard packages generated Go imports, by the name code uses, in the order the plan declares them (CODEGEN.md §2.8): baked Go's, data mode's, the conformance file's.
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
)

// Data mode's generated names (CODEGEN.md §3.3, §5.9, §5.11, §6.1) and its loaders' fixed locals.
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
	// GoUnexported is a @go(name:) override that is a valid identifier but not exported (E8011, decision 182).
	GoUnexported
	// GoOverrideInvalid is a @go(name:) override that is no Go identifier at all (E8011 `override`).
	GoOverrideInvalid
)

var (
	goStdImports = []string{goRT, goTime, goIter, goSync, goStrconv, goMath, goJSON, goFmt, goStrings, goSlices, goAtomic, goTesting}
	// goDataImports are the packages data mode's loaders and JSON helpers write.
	goDataImports = []string{goRT, goJSON, goFmt, goStrings, goSlices}
	// goStoreMembers are the store's atomic pointer and methods (CODEGEN.md §5.11, T9).
	goStoreMembers = []string{"current", "Current", "Reload"}
	// goJSONHelpers are the JSON reads every data-mode file with decoders writes, in its order (log-2026-09-24 "Loader parity").
	goJSONHelpers = []string{"jsonObject", "jsonKeys", "jsonNeed", "jsonMay", "jsonCell", "jsonRead", "jsonInt", "jsonSlot", "jsonSame"}
	// goDataLocals are the fixed locals of data mode's loaders, decoders and resolvers.
	goDataLocals = []string{
		"name", "path", "raw", "out", "obj", "err", "f", GoRows, "values", "keys", "i", GoIDStore, GoRetiredStore, "dir", "s", "ctx", "tag", "c",
		"key", "k", "r", "ok", "bad", goWant, "dst", "n", "lo", "hi", "v", "kr", "vr", "hasK", "hasV", "first", "empty", "marker", "a", "m", "af", "mf",
	}
	// goVectorOwn are the fields a conformance vector has besides its inputs (CONFORMANCE.md §7.2).
	goVectorOwn = []string{goWant, "code"}
	// goCheckedOps are the operators gen/go writes as checked rt helpers (CONFORMANCE.md §3).
	goCheckedOps       = map[Op]bool{OpAdd: true, OpSub: true, OpMul: true, OpDiv: true, OpMod: true}
	goVariantMembers   = []string{GoKindStore, GoCaseStore, GoKind}
	goIDEnumMethods    = []string{GoString}
	goEnumMethods      = append(slices.Clone(goIDEnumMethods), GoWire)
	goCodesEnumMethods = append(slices.Clone(goEnumMethods), GoCode)
	goContainerMembers = []string{GoRows, GoLen, GoAt, GoAll, GoFind}
	// goPointerKinds are the kinds whose getter returns a pointer, which nil marks absent (CODEGEN.md §4.2, §4.3).
	goPointerKinds = map[types.Kind]bool{types.Record: true, types.Variant: true, types.Case: true}
	// goStdOfKind is the package a type of each kind makes baked Go import; math comes from a -0.0 literal only (decisions 181, 202).
	goStdOfKind = map[types.Kind]string{types.List: goRT, types.Map: goRT, types.DepMap: goRT, types.Duration: goTime}
)
