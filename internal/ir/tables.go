package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// nameSet is the set of the given names.
func nameSet(names ...string) map[string]bool {
	s := make(map[string]bool, len(names))
	for _, n := range names {
		s[n] = true
	}
	return s
}

// kindSet is the set of the given kinds.
func kindSet(kinds ...types.Kind) map[types.Kind]bool {
	s := make(map[types.Kind]bool, len(kinds))
	for _, k := range kinds {
		s[k] = true
	}
	return s
}

// The portable subset's kinds: arithmetic, scalar, result, template (CONFORMANCE.md §2.1, §2.2).
var (
	numericKinds = kindSet(types.Int, types.Float, types.Duration)
	floatKinds   = kindSet(types.Float)
	scalarKinds  = kindSet(types.Bool, types.Int, types.Float, types.String, types.Duration,
		types.Enum)
	resultKinds = kindSet(types.Bool, types.Int, types.Float, types.String, types.Duration,
		types.Enum, types.Ref)
	templateKinds = kindSet(types.String, types.Int, types.Enum)
	untemplated   = kindSet(types.Float, types.Duration) // E9005 in a template, else E9001
)

// portableBuiltins are the portable built-ins and their argument kinds (CONFORMANCE.md §2.2).
var portableBuiltins = map[string]builtinSpec{
	fnMin:   {fn: BuiltinMin, arity: minMinMax, variadic: true, kinds: numericKinds},
	fnMax:   {fn: BuiltinMax, arity: minMinMax, variadic: true, kinds: numericKinds},
	"abs":   {fn: BuiltinAbs, arity: 1, kinds: numericKinds},
	"clamp": {fn: BuiltinClamp, arity: clampArity, kinds: numericKinds},
	"floor": {fn: BuiltinFloor, arity: 1, kinds: floatKinds},
	"ceil":  {fn: BuiltinCeil, arity: 1, kinds: floatKinds},
	"round": {fn: BuiltinRound, arity: 1, kinds: floatKinds},
}

// portableConversions are `Float(i)` and `Int(f)` with the kind each reads (CONFORMANCE.md §3).
var portableConversions = map[string]builtinSpec{
	convFloat: {fn: BuiltinFloat, arity: 1, kinds: kindSet(types.Int)},
	convInt:   {fn: BuiltinInt, arity: 1, kinds: floatKinds},
}

// binaryOps are the portable binary operators by token (CONFORMANCE.md §2.2).
var binaryOps = map[syntax.TokenKind]Op{
	syntax.TokPlus: OpAdd, syntax.TokMinus: OpSub, syntax.TokStar: OpMul, syntax.TokSlash: OpDiv,
	syntax.TokPercent: OpMod, syntax.KwAnd: OpAnd, syntax.KwOr: OpOr, syntax.TokEq: OpEq,
	syntax.TokNe: OpNe, syntax.TokLt: OpLt, syntax.TokLe: OpLe, syntax.TokGt: OpGt, syntax.TokGe: OpGe,
}

// unaryOps are the portable unary operators with their operand kinds.
var unaryOps = map[syntax.TokenKind]unarySpec{
	syntax.TokMinus: {op: OpNeg, kinds: numericKinds},
	syntax.KwNot:    {op: OpNot, kinds: kindSet(types.Bool)},
}

// fpComposites opens each composite type of FINGERPRINT.md §4.4 but the keyed list.
var fpComposites = map[types.Kind]string{
	types.List: "list(", types.Table: "table(", types.Optional: "opt(", types.Map: fpMap,
	types.DepMap: fpMap, types.Ref: "ref(", types.LitUnion: "union(",
}

// fpEncNames is `enc=` of a field (FINGERPRINT.md §4.5).
var fpEncNames = [...]string{types.EncPlain: fpNone, types.EncInt: fpEncInt, types.EncBits: "bits"}

// The words of targets, modes and @cpp(access:) (CODEGEN.md §2.1, CPP-01), by value.
var (
	targetWords = [...]string{
		TargetGo: check.TargetGo, TargetCpp: check.TargetCpp, TargetTS: check.TargetTS,
		TargetJSON: check.TargetJSON, TargetView: check.TargetView,
	}
	modeWords = [...]string{
		ModeNone: "", ModeBaked: check.ModeBaked, ModeEmbedded: check.ModeEmbedded,
		ModeData: check.ModeData, ModeTypes: check.ModeTypes,
	}
	accessWords = append([]string{""}, syntax.AccessModes()...) // indexed by Access
)

// branchKinds are the kinds a dependent type's branch may have (CODEGEN.md §5.6, E8017).
var branchKinds = kindSet(types.Bool, types.Int, types.Float, types.String, types.Duration,
	types.Enum, types.Ref)

// recordKinds are the kinds whose values are records: a record, a variant, a case.
var recordKinds = kindSet(types.Record, types.Variant, types.Case)

// goInitialisms is the closed initialism list of GoCap (CODEGEN.md §3.2).
var goInitialisms = nameSet("id", "url", "api", "http", "json", "ui", "db", "ip", "hp", "mp", "ts")

// goPredeclared are Go's predeclared identifiers (CODEGEN.md §3.4).
var goPredeclared = nameSet("any", "append", "bool", "byte", "cap", "clear", "close",
	"comparable", "complex", "complex64", "complex128", "copy", "delete", "error", boolFalse,
	"float32", "float64", "imag", "int", "int8", "int16", "int32", "int64", "iota", "len", "make",
	"max", "min", "new", "nil", "panic", "print", "println", "real", "recover", "rune", "string",
	boolTrue, "uint", "uint8", "uint16", "uint32", "uint64", "uintptr")

// goImportNames are the package names generated Go files import (CODEGEN.md §3.4).
var goImportNames = nameSet(goRT, goJSON, goFmt, goIter, "os", goFilepath, goAtomic, goSync,
	goTime, goErrors, goStrconv, goStrings, goMath, goRegexp, "embed", goSlices)

// cppOwnNames are the namespaces generated C++ declares itself, beside check's (CODEGEN.md §3.4).
var cppOwnNames = nameSet(cppDetail, cppConformance)

// The fixed members of gen/cpp's classes and conformance namespace (CODEGEN.md §5, §7.2).
var (
	cppEntryMembers     = []string{"GetId", "id_", "GetRetired", "retired_"} // §5.3
	cppContainerMembers = []string{GoLen, GoAt, GoAll, GoFind, "rows_"}      // §5.9
	cppConformanceOwn   = []string{"g_code", "Capture"}                      // CONFORMANCE.md §7.2
	cppStoreMembers     = []string{storeCurrent, storeReload, "current_"}    // §5.11
	cppEnumOverloads    = []string{CppToName, CppToWire}                     // §5.2
	cppAccessMembers    = []string{cppDataStruct, cppBuild, GoGet}           // §7.3: a baked emit's access struct
)

// cppInputHelper is a set of §7.7 helpers with the input kinds that use it; nil kinds: always.
type cppInputHelper struct {
	kinds []types.Kind
	names []string
}

// cppInputHelpers are CODEGEN.md §7.7's helpers in gen/cpp's order; an enum uses <E>FromWire.
var cppInputHelpers = []cppInputHelper{
	{nil, []string{CppEnvText}},
	{[]types.Kind{types.Int, types.Float, types.Duration}, []string{"IsDecDigit", "AllDigits"}},
	{[]types.Kind{types.Int}, []string{"ParseIntLiteral"}},
	{[]types.Kind{types.Float}, []string{"ParseFloatLiteral"}},
	{[]types.Kind{types.String}, []string{"ParseStringLiteral"}},
	{[]types.Kind{types.Bool}, []string{"ParseBoolLiteral"}},
	{[]types.Kind{types.Duration}, []string{"DurationDigits", "ParseDurationLiteral"}},
}

// inputKinds name `not a valid <Kind>` of a LoadInputs failure line, a Float32 reading Float.
var inputKinds = map[types.Kind]string{
	types.Int: convInt, types.Float: convFloat, types.Bool: "Bool", types.Duration: "Duration",
	types.String: "String",
}

// The fixed Go names and imports of generated code (CODEGEN.md §2.8, §5.2–§5.11, §6.2).
var (
	goStdImports = []string{
		goRT, goTime, goIter, goSync, goStrconv, goMath, goJSON, goFmt,
		goStrings, goSlices, goAtomic, goErrors, goRegexp, goTesting,
	}
	goDataImports  = []string{goRT, goJSON, goFmt, goStrings, goSlices}
	goStoreMembers = []string{"current", storeCurrent, storeReload}
	// log-2026-09-24 "Loader parity".
	goJSONHelpers = []string{
		"jsonObject", "jsonKeys", "jsonNeed", "jsonMay", "jsonCell",
		"jsonRead", "jsonInt", "jsonSlot", "jsonSame",
	}
	goDataLocals = []string{
		"name", "path", "raw", goOut, "obj", "err", "f", GoRows, goValues,
		"keys", "i", GoIDStore, GoRetiredStore, "dir", "s", "ctx", "tag", "c", "key", "k", "r",
		"ok", "bad", goWant, "dst", "n", "lo", "hi", "v", "kr", "vr", "hasK", "hasV", "first",
		"empty", "marker", "a", "m", "af", "mf", "at", "disc",
	}
	goVectorOwn        = []string{goWant, "code"}
	goCheckedOps       = map[Op]bool{OpAdd: true, OpSub: true, OpMul: true, OpDiv: true, OpMod: true}
	goVariantMembers   = []string{GoKindStore, GoCaseStore, GoKind}
	goIDEnumMethods    = []string{GoString}
	goEnumMethods      = append(slices.Clone(goIDEnumMethods), GoWire)
	goCodesEnumMethods = append(slices.Clone(goEnumMethods), GoCode)
	goContainerMembers = []string{GoRows, GoLen, GoAt, GoAll, GoFind}
	goPointerKinds     = kindSet(types.Record, types.Variant, types.Case, types.TypeApp) // nil marks absent (CODEGEN.md §4.3)
	goStdOfKind        = map[types.Kind]string{
		types.List: goRT, types.Map: goRT, types.DepMap: goRT, types.Duration: goTime,
	}
)
