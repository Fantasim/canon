package ir

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/check"
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

// The annotation arguments stage E reads that CODEGEN.md names nowhere else (§3.5, §7.8).
const (
	argType    = "type"
	argValue   = "value"
	flagBigInt = "bigint"
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
	targetWords = [...]string{TargetGo: check.TargetGo, TargetCpp: check.TargetCpp, TargetTS: check.TargetTS, TargetJSON: check.TargetJSON, TargetView: check.TargetView}
	modeWords   = [...]string{ModeNone: "", ModeBaked: check.ModeBaked, ModeEmbedded: check.ModeEmbedded, ModeData: check.ModeData, ModeTypes: check.ModeTypes}
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

const underscore = "_"

// identPattern is a plain identifier: a letter or underscore, then letters, digits or underscores (CODEGEN.md §3.5, E8011).
var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// goInitialisms is the closed initialism list of GoCap (CODEGEN.md §3.2).
var goInitialisms = map[string]bool{
	"id": true, "url": true, "api": true, "http": true, "json": true, "ui": true,
	"db": true, "ip": true, "hp": true, "mp": true, "ts": true,
}

// goReservedLower is what a Go lower-case position escapes: keywords, predeclared identifiers, the packages a generated file imports, and `self` (CODEGEN.md §3.4).
var goReservedLower = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true,
	"defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true, "switch": true, "type": true,
	"var": true, "any": true, "append": true, "bool": true, "byte": true, "cap": true,
	"clear": true, "close": true, "comparable": true, "complex": true, "complex64": true,
	"complex128": true, "copy": true, "delete": true, "error": true, "false": true,
	"float32": true, "float64": true, "imag": true, "int": true, "int8": true, "int16": true,
	"int32": true, "int64": true, "iota": true, "len": true, "make": true, "max": true,
	"min": true, "new": true, "nil": true, "panic": true, "print": true, "println": true,
	"real": true, "recover": true, "rune": true, "string": true, "true": true, "uint": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true, "rt": true,
	"json": true, "fmt": true, "iter": true, "os": true, "filepath": true, "atomic": true,
	"sync": true, "time": true, "errors": true, "strconv": true, "strings": true, "math": true,
	"regexp": true, "embed": true, "self": true,
}

// cppReserved is what a C++ verbatim position escapes: C++20's keywords, its alternative tokens, and the names generated code reserves for itself (CODEGEN.md §3.4).
var cppReserved = map[string]bool{
	"alignas": true, "alignof": true, "and": true, "and_eq": true, "asm": true, "auto": true,
	"bitand": true, "bitor": true, "bool": true, "break": true, "case": true, "catch": true,
	"char": true, "char8_t": true, "char16_t": true, "char32_t": true, "class": true, "compl": true,
	"concept": true, "const": true, "consteval": true, "constexpr": true, "constinit": true,
	"const_cast": true, "continue": true, "co_await": true, "co_return": true, "co_yield": true,
	"decltype": true, "default": true, "delete": true, "do": true, "double": true,
	"dynamic_cast": true, "else": true, "enum": true, "explicit": true, "export": true,
	"extern": true, "false": true, "float": true, "for": true, "friend": true, "goto": true,
	"if": true, "inline": true, "int": true, "long": true, "mutable": true, "namespace": true,
	"new": true, "noexcept": true, "not": true, "not_eq": true, "nullptr": true, "operator": true,
	"or": true, "or_eq": true, "private": true, "protected": true, "public": true, "register": true,
	"reinterpret_cast": true, "requires": true, "return": true, "short": true, "signed": true,
	"sizeof": true, "static": true, "static_assert": true, "static_cast": true, "struct": true,
	"switch": true, "template": true, "this": true, "thread_local": true, "throw": true,
	"true": true, "try": true, "typedef": true, "typeid": true, "typename": true, "union": true,
	"unsigned": true, "using": true, "virtual": true, "void": true, "volatile": true,
	"wchar_t": true, "while": true, "xor": true, "xor_eq": true,
	"detail": true, "conformance": true, "canon": true, "std": true, "nlohmann": true,
}
