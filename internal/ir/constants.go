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

// maxSafeInt is Number.MAX_SAFE_INTEGER, 2^53 - 1 (CODEGEN.md §4.1, E8101).
const maxSafeInt = 1<<53 - 1

// JSONExt ends a file-mode JSON `out` and every directory-mode file name (WIRE.md §8.1).
const JSONExt = ".json"

const underscore = "_"

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
	goRT: true, "json": true, "fmt": true, goIter: true, "os": true, "filepath": true,
	"atomic": true, goSync: true, goTime: true, "errors": true, goStrconv: true, "strings": true,
	goMath: true, "regexp": true, "embed": true,
}

// cppOwnNames are the names generated C++ reserves for itself (CODEGEN.md §3.4).
var cppOwnNames = map[string]bool{"detail": true, "conformance": true, "canon": true, "std": true, "nlohmann": true}

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

// The standard packages baked Go imports, in the order the plan declares them (CODEGEN.md §2.8).
const (
	goRT      = "rt"
	goTime    = "time"
	goIter    = "iter"
	goSync    = "sync"
	goStrconv = "strconv"
	goMath    = "math"
)

// The kinds of GoNameProblem.
const (
	// GoCollision is two names equal in one scope (E8005).
	GoCollision GoProblemKind = iota
	// GoNotIdentifier is a generated name that is no Go identifier.
	GoNotIdentifier
	// GoUnexported is a @go(name:) override that is not an exported identifier (E8011, decision 182).
	GoUnexported
)

var (
	goStdImports       = []string{goRT, goTime, goIter, goSync, goStrconv, goMath}
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
