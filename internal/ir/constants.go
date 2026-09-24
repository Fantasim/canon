package ir

import "github.com/fantasim/canonlang/internal/types"

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
	fpSourceField = cgWordField
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

// The annotations, arguments and flags stage E reads (GRAMMAR.md §8.3, CODEGEN.md §3.5, §7.8).
const (
	cgAnnGo      = "go"
	cgAnnCpp     = "cpp"
	cgAnnTS      = "ts"
	cgAnnReload  = "reload"
	cgArgName    = "name"
	cgArgStruct  = "struct"
	cgArgHeader  = "header"
	cgArgAccess  = "access"
	cgWordField  = "field"
	argField     = cgWordField
	argType      = "type"
	cgArgUnit    = "unit"
	argValue     = "value"
	cgArgDefines = "defines"
	flagBigInt   = "bigint"
)

// The emit options of CODEGEN.md §2.1 and WIRE.md §8.1.
const (
	optOut       = "out"
	optMode      = "mode"
	optValues    = "values"
	optPackage   = "package"
	optNamespace = "namespace"
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

// The words of targets, modes and @cpp(access:) (CODEGEN.md §2.1, CPP-01), by value.
var (
	targetWords = [...]string{TargetGo: cgAnnGo, TargetCpp: cgAnnCpp, TargetTS: cgAnnTS, TargetJSON: "json", TargetView: "view"}
	modeWords   = [...]string{ModeNone: "", ModeBaked: "baked", ModeEmbedded: "embedded", ModeData: "data", ModeTypes: "types"}
	accessWords = [...]string{AccessNone: "", AccessFields: "fields", AccessBoth: "both", AccessGetters: "getters"}
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
