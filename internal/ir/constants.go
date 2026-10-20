package ir

import (
	"regexp"
	"strings"
)

// The emit targets of CODEGEN.md §2.1.
const (
	TargetGo Target = iota
	TargetCpp
	TargetTS
	TargetJSON
	TargetView
	TargetText
	targetCount
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
	minMinMax  = 2
	clampArity = 3
	selfPrefix = "self_" // a read named like a declared parameter (CONFORMANCE.md §2.3)
)

// Where a path of self is read.
const (
	ctxValue readCtx = iota
	ctxCoalesce
	ctxIs
)

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
	curDir         = "."
	qnameSep       = "."
	parentDir      = ".." // the path segment above a directory
	severalCopies  = 2    // the fewest copies an emit with a list out writes (CODEGEN.md §2.1)
	pathSep        = "/"
	cppScope       = "::"
	docSep         = "\n\n" // GRAMMAR.md §9.1: package docs of several files
	boolFalse      = "false"
	boolTrue       = "true"
	cppReservedRun = "__"
)

// Limits and fixed spellings stage E composes.
const (
	NoBranch   = -1        // marks a discriminant member whose arm is Never
	maxCells   = 65_536    // the largest lookup table (CODEGEN.md §5.10, E9002)
	maxSafeInt = 1<<53 - 1 // Number.MAX_SAFE_INTEGER (CODEGEN.md §4.1, E8101)
	JSONExt    = ".json"   // a file-mode JSON out's and every directory-mode file name's end (WIRE.md §8.1)
	underscore = "_"
	pairsOpen  = "{" // a pairs: key template's slot index, {i} (WIRE.md §5.14)
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

// A C++ name reserved to the implementation starts `_[A-Z]` or holds cppReservedRun (CODEGEN.md §3.4).
var cppReservedStart = regexp.MustCompile(`^_[A-Z]`)

// Generated C++ names gen/cpp declares (CODEGEN.md §3.3, §5, §7.2, CONFORMANCE.md §7.2).
const (
	cppDetail        = "detail"
	cppConformance   = "conformance"
	cppAsPrefix      = "As"
	cppConstPrefix   = "k"
	cppFromWire      = "FromWire"
	cppCodes         = "Codes" // a closed @codes enum's detail::k<E>Codes, k<E>CodeOrdinals, <E>Ordinal (CODEGEN.md §5.2)
	cppCodeOrdinals  = "CodeOrdinals"
	cppOrdinal       = "Ordinal"
	cppWires         = "Wires" // an opened enum's detail::k<E>Wires, k<E>WireNames, k<E>WireCodes of its .gen.cpp (§5.2)
	cppWireNames     = "WireNames"
	cppWireCodes     = "WireCodes"
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
	CppParse         = "Parse"  // types mode's public static Parse (CODEGEN.md §5.13, DECISIONS 361)
	CppResolve       = "Resolve"
	CppVariantMember = "value_" // a variant's or dependent type's std::variant (§5.5, §5.6)
	CppToName        = "ToName" // the enum helpers, one overload per enum (CODEGEN.md §5.2)
	CppToWire        = "ToWire"
	CppIsKnown       = "IsKnown" // an opened enum's helpers, one overload per enum (DECISIONS 362)
	CppToCode        = "ToCode"
	CppOpenWire      = "wire_"
	CppMatchPattern  = "MatchPattern" // §7.7's pattern search, the input helper a loader calls by name
	CppEnvText       = "EnvText"      // §7.7's variable reader, the input helper each LoadInputs block calls
)

// How another package's same C++ name meets a name: only overloads, TU-locals, namespaces pass.
const (
	meetsNever cppMeet = iota
	meetsOverload
	meetsLocal
	meetsNamespace
)

// The fixed Go names the plan declares and gen/go's templates write (CODEGEN.md §5.2–§6.2).
const (
	GoID            = "ID" // the ID method, and the suffix of an id type and a key getter
	GoRetired       = "Retired"
	GoKind          = "Kind" // the Kind method, and the suffix of a kind enum
	GoString        = "String"
	GoWire          = "Wire"
	goMarshalText   = "MarshalText" // CODEGEN.md §5.2, §5.3 (DECISIONS 379)
	goUnmarshalText = "UnmarshalText"
	goPosition      = "Position"
	goPositionField = "position" // the baked id struct's field (CODEGEN.md §5.3)
	GoCode          = "Code"
	GoKnown         = openKnown
	GoLen           = "Len"
	GoAt            = "At"
	GoAll           = "All"
	GoFind          = "Find"
	GoGet           = "Get" // a table container's Get, and the prefix of an accessor
	GoRows          = "rows"
	GoIDStore       = "id"
	GoRetiredStore  = "retired"
	GoKindStore     = "kind"
	GoCaseStore     = "value"
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

// The standard packages generated Go imports, by the name code uses; GoRT names the runtime's directory too (CODEGEN.md §2.3, §2.8).
const (
	GoRT       = "rt"
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
	goUTF8     = "utf8" // unicode/utf8: a types-mode decoder's document is UTF-8 (WIRE.md §3.1)
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
	cppBranchesSuffix    = "Branches" // detail::k<T><F>Branches (CODEGEN.md §5.6, DECISIONS 350)
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
	branchesPrefix     = "branches" // a build-time branch table, Go and TS (CODEGEN.md §5.6, DECISIONS 350)
	goResolvePrefix    = "resolve"
	goSnapshotSuffix   = "Snapshot"
	goStoreSuffix      = "Store" // also the store variable
	goJSONRowID        = "jsonRowID"
	goJSONTable        = "jsonTable"
	goJSONMap          = "jsonMap"
	goJSONKeyPath      = "jsonKeyPath"
	goPublicDecode     = CppDecode // types mode's public Decode<T> (CODEGEN.md §5.13)
	goTextFileSuffix   = "File"    // a @text fn's types-mode Decode<Fn>File (DECISIONS 340)
	goJSONDocument     = "jsonDocument"
	goPkgSuffix        = "pkg" // the alias of an import Go-named like a predeclared identifier, maxpkg (CODEGEN.md §3.4)
	goJSONDuration     = "jsonDuration"
	goScopeLocals      = "locals"
	goScopeConformance = "conformance imports"
	goInt64Bits        = 64
	goFloat32Bits      = 32
)

// Make hooks, rows and readers of other packages' classes (CODEGEN.md §2.8, §5.9, §5.14; DECISIONS 323).
const (
	hookMake      = "Make"  // Go Make_<T> and MakeCase_<V>_<Case>, C++ detail::<P>Make
	hookCase      = "Case"  // a case-type hook: Go MakeCase_<V>_<Case>, C++ Case_<V>_<Case>
	hookEntry     = "Entry" // an entry hook: Go MakeEntry_<T>, C++ Entry_<T>
	rowSuffix     = "Row"   // <Element>Row, in Go and C++
	goRecord      = "Record"
	goRecordStore = "record"
	cppReadPrefix = "Read"                // a C++ reader of another package's class, Read_<Seg1>_…_<T>
	goHookPrefix  = hookMake + underscore // Make_: the interior _ keeps hooks apart from §3.3's names
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
	// goMacroName is a generated C++ name that common platform headers define as a macro (W8006).
	goMacroName
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

// tsIdentPattern is a TypeScript identifier as generated code spells it: `$` starts a method's pure function.
var tsIdentPattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

const (
	tsHelperOrigin   = "the TypeScript helper block"
	tsModuleScope    = "module"
	tsPureMark       = "$"
	tsIDSuffix       = "Id"
	tsKindSuffix     = "Kind"
	tsBranchSuffix   = "Branch"
	tsDecodePrefix   = "decode"
	tsParsePrefix    = "parse"
	tsReadPrefix     = "read"
	tsSchemaSuffix   = "Schema"
	tsMembersName    = "Members"
	tsNamesName      = "Names"
	tsIndexName      = "Index"
	tsCodesName      = "Codes"
	tsIsPrefix       = "is"    // is<E>Known of an opened enum (DECISIONS 362)
	openKnown        = "Known" // an opened enum's known check: Go's Known method, TypeScript's <E>Known (DECISIONS 339, 362)
	tsFindByPrefix   = "findBy"
	tsContainerScope = "container "
	tsNameSuffix     = "Name"
	tsIDProp         = "id"
	tsCanonRowScope  = "CanonRow of " // the scope of a row of another package's record (CanonRow<T, K>)
	tsRetiredProp    = "retired"
	tsKindProp       = "kind"
)

// The names a baked cpp emit adds (CODEGEN.md §5.3, §5.10, §7.3; decision 293).
const (
	cppIDSuffix    = "Id"
	cppRefData     = "_ref"
	cppCellsSuffix = "Cells"
	cppDataStruct  = "Data"
	cppBuild       = "Build"
	// a dependent type's define value storage (DECISIONS 298): Go's carries an interior `_`, C++'s a final one, as every generated member.
	goDefineStore  = "define_value"
	cppDefineValue = "defineValue_"
)

// ErrInternal messages of a `@text` fn left without a value, or whose value has no JSON form (DECISIONS 308).
const (
	fmtNoText   = "%w: package %s: @text fn %s has no value of its result type"
	fmtTextWire = "%w: package %s: @text fn %s: %w"
)

// cppStdMacroNames, sorted: the macros of [atomics.syn] [cerrno.syn] [cfloat.syn] [clocale.syn] [cmath.syn] [cstddef.syn] [cstdint.syn] [cstdio.syn] [cstdlib.syn] [cstring.syn] (CODEGEN.md §3.4).
var cppStdMacroNames = strings.Fields(`
ATOMIC_BOOL_LOCK_FREE ATOMIC_CHAR16_T_LOCK_FREE ATOMIC_CHAR32_T_LOCK_FREE ATOMIC_CHAR_LOCK_FREE
ATOMIC_FLAG_INIT ATOMIC_INT_LOCK_FREE ATOMIC_LLONG_LOCK_FREE ATOMIC_LONG_LOCK_FREE
ATOMIC_POINTER_LOCK_FREE ATOMIC_SHORT_LOCK_FREE ATOMIC_VAR_INIT ATOMIC_WCHAR_T_LOCK_FREE BUFSIZ
DBL_DECIMAL_DIG DBL_DIG DBL_EPSILON DBL_HAS_SUBNORM DBL_MANT_DIG DBL_MAX DBL_MAX_10_EXP DBL_MAX_EXP
DBL_MIN DBL_MIN_10_EXP DBL_MIN_EXP DBL_TRUE_MIN DECIMAL_DIG E2BIG EACCES EADDRINUSE EADDRNOTAVAIL
EAFNOSUPPORT EAGAIN EALREADY EBADF EBADMSG EBUSY ECANCELED ECHILD ECONNABORTED ECONNREFUSED
ECONNRESET EDEADLK EDESTADDRREQ EDOM EEXIST EFAULT EFBIG EHOSTUNREACH EIDRM EILSEQ EINPROGRESS
EINTR EINVAL EIO EISCONN EISDIR ELOOP EMFILE EMLINK EMSGSIZE ENAMETOOLONG ENETDOWN ENETRESET
ENETUNREACH ENFILE ENOBUFS ENODATA ENODEV ENOENT ENOEXEC ENOLCK ENOLINK ENOMEM ENOMSG ENOPROTOOPT
ENOSPC ENOSR ENOSTR ENOSYS ENOTCONN ENOTDIR ENOTEMPTY ENOTRECOVERABLE ENOTSOCK ENOTSUP ENOTTY ENXIO
EOF EOPNOTSUPP EOVERFLOW EOWNERDEAD EPERM EPIPE EPROTO EPROTONOSUPPORT EPROTOTYPE ERANGE EROFS
ESPIPE ESRCH ETIME ETIMEDOUT ETXTBSY EWOULDBLOCK EXDEV EXIT_FAILURE EXIT_SUCCESS FILENAME_MAX
FLT_DECIMAL_DIG FLT_DIG FLT_EPSILON FLT_EVAL_METHOD FLT_HAS_SUBNORM FLT_MANT_DIG FLT_MAX
FLT_MAX_10_EXP FLT_MAX_EXP FLT_MIN FLT_MIN_10_EXP FLT_MIN_EXP FLT_RADIX FLT_ROUNDS FLT_TRUE_MIN
FOPEN_MAX FP_FAST_FMA FP_FAST_FMAF FP_FAST_FMAL FP_ILOGB0 FP_ILOGBNAN FP_INFINITE FP_NAN FP_NORMAL
FP_SUBNORMAL FP_ZERO HUGE_VAL HUGE_VALF HUGE_VALL INFINITY INT16_C INT16_MAX INT16_MIN INT32_C
INT32_MAX INT32_MIN INT64_C INT64_MAX INT64_MIN INT8_C INT8_MAX INT8_MIN INTMAX_C INTMAX_MAX
INTMAX_MIN INTPTR_MAX INTPTR_MIN INT_FAST16_MAX INT_FAST16_MIN INT_FAST32_MAX INT_FAST32_MIN
INT_FAST64_MAX INT_FAST64_MIN INT_FAST8_MAX INT_FAST8_MIN INT_LEAST16_MAX INT_LEAST16_MIN
INT_LEAST32_MAX INT_LEAST32_MIN INT_LEAST64_MAX INT_LEAST64_MIN INT_LEAST8_MAX INT_LEAST8_MIN
LC_ALL LC_COLLATE LC_CTYPE LC_MONETARY LC_NUMERIC LC_TIME LDBL_DECIMAL_DIG LDBL_DIG LDBL_EPSILON
LDBL_HAS_SUBNORM LDBL_MANT_DIG LDBL_MAX LDBL_MAX_10_EXP LDBL_MAX_EXP LDBL_MIN LDBL_MIN_10_EXP
LDBL_MIN_EXP LDBL_TRUE_MIN L_tmpnam MATH_ERREXCEPT MATH_ERRNO MB_CUR_MAX NAN NULL PTRDIFF_MAX
PTRDIFF_MIN RAND_MAX SEEK_CUR SEEK_END SEEK_SET SIG_ATOMIC_MAX SIG_ATOMIC_MIN SIZE_MAX TMP_MAX
UINT16_C UINT16_MAX UINT32_C UINT32_MAX UINT64_C UINT64_MAX UINT8_C UINT8_MAX UINTMAX_C UINTMAX_MAX
UINTPTR_MAX UINT_FAST16_MAX UINT_FAST32_MAX UINT_FAST64_MAX UINT_FAST8_MAX UINT_LEAST16_MAX
UINT_LEAST32_MAX UINT_LEAST64_MAX UINT_LEAST8_MAX WCHAR_MAX WCHAR_MIN WINT_MAX WINT_MIN _IOFBF
_IOLBF _IONBF errno math_errhandling offsetof stderr stdin stdout`)
