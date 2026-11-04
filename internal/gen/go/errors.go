package gogen

import (
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var (
	// ErrTarget is an emit whose target is not go.
	ErrTarget = errors.New("gogen: not a go emit")
	// ErrMalformed is an IR that stage E should not have produced.
	ErrMalformed = errors.New("gogen: malformed IR")
	// errNameCollision is two generated names equal in one Go scope: stage E reports it first (malformed IR).
	errNameCollision = fmt.Errorf("%w: generated name collision", ErrMalformed)
	errFormat        = errors.New("gogen: generated Go does not format")
)

// DetailError names, in Subject and Index, what a sentinel refused: a caller compares fields, never the message (go.md §3).
type DetailError struct {
	Subject string
	Index   int

	err     error
	message string
}

func newDetail(err error, subject, format string, args ...any) *DetailError {
	return &DetailError{Subject: subject, Index: -1, err: err, message: fmt.Sprintf(format, args...)}
}

func (e *DetailError) Error() string { return e.err.Error() + ": " + e.message }

func (e *DetailError) Unwrap() error { return e.err }

// What data mode finds malformed (ErrMalformed: stage E refuses it first, DECISIONS 320) and the collision its name check finds.
const (
	packageFnFormat      = "package-level export fn %s in data mode (CODEGEN.md §5.10)"
	lookupParamFormat    = "%s: a finite parameter that is not an enum or a Bool, in data mode (CODEGEN.md §5.10)"
	lookupRefFormat      = "%s: a finite-parameter method whose result holds a ref resolved at load, in data mode"
	dataValueFormat      = "data value %s that is not a table, a keyed list or a record (CODEGEN.md §2.2)"
	unionMalformedFormat = "%s: a literal union whose other arm is not string-wired"
	inlineFormat         = "%s: an optional or non-variant @json(inline) field, in data mode"
	foldFormat           = "%s: an inline variant key equal to another key of its parent but for letter case, in data mode"
	noneMarkerFormat     = "%s: the none marker %s, a non-empty object or array, in data mode"
	dataCollisionFormat  = "%s declares %s twice in data mode"
	caseNoVariantFormat  = "a case %s without its variant"
)

// What types mode finds malformed (ErrMalformed: stage E refuses it first, CODEGEN.md §2.2, §5.13).
const (
	typesStoredFnFormat = "%s: a precomputed or lookup export fn, in types mode, which has no data to read"
	typesComputedFormat = "%s: a computed default, in types mode, which has no data to read"
	typesInputFormat    = "%s: an input field, in types mode, which writes no LoadInputs"
)

// The causes of the dependent-type refusals, each ErrMalformed: a caller tells them apart with errors.Is (go.md §3).
var (
	errDependentNoDisc   = fmt.Errorf("%w", ErrMalformed)
	errDependentDisc     = fmt.Errorf("%w", ErrMalformed)
	errDependentNested   = fmt.Errorf("%w", ErrMalformed)
	errDependentValue    = fmt.Errorf("%w", ErrMalformed)
	errDependentNoBranch = fmt.Errorf("%w", ErrMalformed)
	errDependentNever    = fmt.Errorf("%w", ErrMalformed)
	errDependentUnion    = fmt.Errorf("%w", ErrMalformed)
)

// Map fields (CODEGEN.md §5.9, WIRE.md §5.8; DECISIONS 312): the key-path helper, its local, and the check that a ref key names an entry.
const (
	helperKeyPath    = "jsonKeyPath"
	tempPath         = "kp"
	differs          = " != "
	andSep           = " && "
	unionCheckFormat = "if %s {\nreturn %s\n}\n"
	keyCheckFormat   = "if _, %[1]s := %[2]s(%[3]s); !%[1]s {\nreturn %[4]s\n}\n"
	keyNoneFormat    = "if %s[%s] == nil {\nreturn %s\n}\n"
	canonicalKeyText = "expected a canonical decimal integer key"
	parsedKeyFormat  = "%[1]s, %[2]s := %[3]s(%[4]s)\n"
	findAgainFormat  = "if %[1]s {\n_, %[1]s = %[2]s(%[3]s)\n}\n"
	findKeyFormat    = "_, %[1]s := %[2]s(%[3]s)\n"
	failUnlessFormat = "if !%s {\nreturn %s\n}\n"
)

// Dependent types (CODEGEN.md §5.6): what stage E refuses first (E8019 DependentType; E8012 for a define ref), so meeting one is ErrMalformed, each its own text (decision 194).
const (
	dependentNoDiscFormat   = "%s: a dependent type without its Bool or enum discriminant"
	dependentDiscFormat     = "%s: a dependent value whose discriminant is not read from earlier required fields of its record (ir.DiscFields)"
	dependentNestedFormat   = "%s: a dependent value outside a field of its record and that field's list elements"
	dependentValueFormat    = "%s: a define branch of a dependent type whose define table or key the IR does not hold"
	dependentNoBranchFormat = "%s: a dependent type every arm of which is Never"
	dependentNeverFormat    = "%s: a dependent value whose discriminant selects a Never arm"
	dependentUnionFormat    = "%s: a literal union over a dependent type, in data mode"
)

// Messages of the generator's errors.
const (
	kindFormat   = "%s: a value of kind %s"
	unknownKind  = "an unknown kind"
	unknownMode  = "an unknown mode"
	noKeyType    = "a ref without a key type"
	noEnumFormat = "an enum type without its enum at %s"

	tableEntryFormat = "an entry of a table field without its key at %s"
	tableFieldFormat = "%s: a table field of a record of another package, or of a baked table value's record"
)

// modeNames name the modes in messages (CODEGEN.md §2.1).
var modeNames = [...]string{ir.ModeNone: "none", ir.ModeBaked: "baked", ir.ModeEmbedded: "embedded", ir.ModeData: "data", ir.ModeTypes: "types"}

// kindNames name the type kinds in messages (TYPES.md §2).
var kindNames = [...]string{
	types.Bool: "Bool", types.Int: "Int", types.Float: "Float", types.String: "String",
	types.Duration: "Duration", types.Enum: "enum", types.Record: "record", types.Variant: "variant",
	types.Case: "case", types.VariantKind: "variant kind", types.Optional: "optional",
	types.List: "list", types.Map: "map", types.DepMap: "dependent map", types.Table: "table",
	types.Ref: "ref", types.LitUnion: "literal union", types.Never: "Never", types.Range: "Range",
	types.Func: "function", types.Pair: "pair", types.TypeApp: "dependent type",
	types.DepUnion: "dependent union", types.Define: "define", types.Any: "Any", types.None: "None",
	types.Error: "error",
}

// Optional list elements and map values (CODEGEN.md §4.2, DECISIONS 385): `*T`, nil for none, a present scalar through rt.Ptr.
const (
	ptrFormat          = "%s.Ptr[%s](%s)"
	optionalOpenFormat = "var %[1]s %[2]s\nif string(%[3]s) != \"null\" {\n"
	ifNotNilFormat     = "if %s != nil {\n"
	derefFormat        = "(*%s)"
)

// The expressions of an enum's index and by-wire search (CODEGEN.md §5.2, DECISIONS 430): a value's index is the value converted (uint), or with codes dense in declaration order the code minus the first in uint64; Parse finds a member from its by-wire index i, a member of an opaque type by its position, of a dense enum by its code, of a sparse one in its sorted codes.
const (
	goUint            = "uint"
	convFormat        = "%s(%s)"
	denseIndexFormat  = "%[1]s(%[2]s(%[3]s) - %[4]s)"
	negFirstFormat    = "(%d)"
	foundOpaqueFormat = "%s{%s[i]}"
	foundDenseFormat  = "%[1]s(%[2]s(%[3]s[i]) + %[4]s)"
	foundSparseFormat = "%s[%s[i]]"
	keyedItemFormat   = "%s[%s]"
	codeParam         = "code"
)

// The build and its parts (CODEGEN.md §2.11, §5.9, §6.2): build<Last> makes the data; a body over the budget calls build<Last>_<N> parts that take it. buildWholeLines are the lines the uncut body holds outside its statements: the data's allocation and the return.
const (
	buildWholeLines     = 2
	buildOpenFormat     = "func %s() *%s {\n%s := &%s{}\n"
	buildCallFormat     = "%s(%s)\n"
	buildCloseFormat    = "return %s\n}\n\n"
	buildPartOpenFormat = "func %s(%s *%s) {\n"
)

// A class's decoder and its parts (CODEGEN.md §2.11, §5.13): decode<T> opens the object, calls decode<T>_<N> parts that take the object (when it binds one) and the value (and the whole document for an inline variant), each holding whole fields and ending with `return nil`. decodeReturnLines is that return, which the uncut decoder holds beside its object and key case, and each part adds.
const (
	decodeReturnLines = 1
	partFuncFormat    = "func %s(%s) error {\n"
	partCallFormat    = "if %[1]s := %[2]s(%[3]s); %[1]s != nil {\nreturn %[1]s\n}\n"
	partNameParams    = "%s, %s string"
	partRawParam      = "%s %s"
	partObjParam      = "%s map[string]%s"
	partOutParam      = "%s *%s"
)

// An enum read through the enum's own package (CODEGEN.md §5.10, §5.12, DECISIONS 435): an input reads Parse<E> and refuses a retired member; a lookup's index is the code minus the first (dense codes) or a binary search of the lookup package's sorted keys, whose positions give the declaration index (lookupKeys_<E>, lookupAt_<E>), -1 for another value.
const (
	inputParseEnumFormat = "if %[1]s, %[2]s := %[3]s(%[4]s); %[2]s && !%[1]s.%[5]s() {\n"
	codesDenseFormat     = "%[1]s := -1\nif %[2]s < %[3]d {\n%[1]s = int(%[2]s)\n}\n"
	codesSearchFormat    = "%[1]s := -1\nif k, found := %[2]s.BinarySearch(%[3]s[:], %[4]s); found {\n%[1]s = int(%[5]s[k])\n}\n"
	noLookupTablesFormat = "enum %s without the tables its lookups search, at %s"
)
