package cppgen

import (
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var (
	// ErrTarget is an emit whose target is not cpp.
	ErrTarget = errors.New("cppgen: not a cpp emit")
	// ErrUnsupported is a mode or construct this generator does not emit yet.
	ErrUnsupported = errors.New("cppgen: not supported")
	// ErrMalformed is an IR that stage E should not have produced.
	ErrMalformed = errors.New("cppgen: malformed IR")
	// errName is a generated name C++ cannot declare: stage E reports it first, so reaching it is malformed IR.
	errName = fmt.Errorf("%w: not a C++ identifier", ErrMalformed)
	// errNameCollision is two generated names equal in one C++ scope: stage E reports it first (malformed IR).
	errNameCollision = fmt.Errorf("%w: generated name collision", ErrMalformed)
)

// What this generator refuses (ErrUnsupported, decision 124) or finds malformed (ErrMalformed).
const (
	nilItem            = "a nil item"
	noElem             = "a list, optional or table without its element type"
	noKey              = "a map or ref without its key type"
	noDecl             = "a record, variant or enum kind without its declaration"
	noKeyField         = "a keyed list without its key field"
	defineRefs         = "a ref into a load.defines table"
	unionNonString     = "a literal union whose wire is not a string"
	unionRef           = "a literal union over a ref, not generated yet"
	unionDependent     = "a literal union over a dependent type"
	optionalElems      = "a list of optional elements"
	dataValueKind      = "a data value that is not a table, a keyed list or a record"
	recursiveTypes     = "a type that holds itself"
	dependentElsewhere = "a dependent type that is not a field's type or its list's elements"
	dependentNoBranch  = "a dependent type every arm of which is Never"
	dependentNoDisc    = "a dependent type without its Bool or enum discriminant"
	dependentBadArm    = "a dependent type whose members and branches disagree"
	dependentBadPath   = "a dependent field whose discriminant is not read from earlier required fields of its class (ir.DiscFields)"
	dependentDefines   = "a dependent type with a branch into a load.defines table"
	packageStoredFn    = "a package-level stored export fn in data mode"
	legacyStructs      = "a legacy struct (@cpp(struct:))"
	inputOutsideRecord = "an input field outside a record"
	inputHelperUnknown = "a runtime input helper this generator has no text for"
	patternOutside     = "an input pattern outside EVALUATION.md §11.3's subset"
	inlineFields       = "an optional or non-variant @json(inline) field"
	methodCalls        = "a call to another export method"
	lookupParams       = "a finite parameter that is not an enum or a Bool"
	unknownReads       = "a read of self that is not a path of fields"
	mapFields          = "a map field (nlohmann::json does not keep the key order)"
	foreignPairs       = "a pairs field of a record from another package"
	inlineFoldKeys     = "an inline variant key equal to another key of its parent but for letter case"
	namePlanProblem    = "a C++ name-plan problem stage E should have refused"
	snapshotOrigin     = "the snapshot"
	noneMarkerFormat   = "none marker %s"
	typeFormat         = "type %T"
	valueFormat        = "value %T"
	exprFormat         = "expression %T"
	kindNumberFormat   = "kind %d"
)

// modeNames and kindNames name a mode and a kind in messages.
var (
	modeNames = [...]string{ir.ModeNone: "none", ir.ModeBaked: "baked", ir.ModeEmbedded: "embedded", ir.ModeData: "data", ir.ModeTypes: "types"}
	kindNames = [...]string{
		types.Case: "case type", types.VariantKind: "variant kind", types.Optional: "optional",
		types.Map: "map", types.DepMap: "dependent map", types.Table: "table-typed field",
		types.Never: "Never", types.Range: "Range", types.Func: "function type", types.Pair: "pair",
		types.TypeApp: "dependent type", types.DepUnion: "dependent union", types.Define: "define",
	}
)

// The load errors generated code reports, and the conformance failure line (CODEGEN.md §7.6; CONFORMANCE.md §7.2).
const (
	failArrayFormat       = "dec.Fail(%s, \"expected an array\");"
	unknownCaseFormat     = "dec.Fail(%s, \"unknown case \" + tag);"
	failEntryFormat       = "if (e == nullptr) return dec.Fail(%s, \"no entry \" + %s), false;"
	resolveOptionalFormat = "if (%s && (%s = %s%s)) == nullptr) return dec.Fail(%s, \"no entry \" + %s), false;"
	resolveFormat         = "if ((%s = %s%s)) == nullptr) return dec.Fail(%s, \"no entry \" + %s), false;"
	failLoadText          = "error = dec.Error(), false"
	failSnapshotText      = "error = dec.Error(), nullptr"
	failMissingFormat     = "dec.Fail(%s, \"missing\");"
	noBranchLine          = "dec.Fail(key, \"no branch for this value\");"
	nullElemFormat        = "if (%s.is_null()) return dec.Fail(%s, \"null\"), false;"
	failureTextFormat     = "%s: %s(%s) = %s [%%.*s], canon says %s [%%.*s]\n"
)

// E8302's signal, and the conditions under which LoadInputs refuses a variable (CODEGEN.md §5.12; EVALUATION.md §11.3).
const (
	inputGetterFormat     = "%s %s() const { if (!%s) canon::OnEvalError(%s, %s); %s }"
	notParsedFormat       = "!%s(%s, %s)"
	outsideFormat         = "%s < %s || %s > %s"
	float32OverflowFormat = "std::fabs(%s) >= 3.4028235677973366e+38"
	noMatchFormat         = "!" + ir.CppMatchPattern + "(%s, %s)"
	isMemberFormat        = "*%s == %s"
	orSep                 = " || "
	notValFormat          = "!%s"
)

// The kinds stage E refuses (E8019) where gen/cpp stores or decodes a type, or writes a constant: meeting one there is ErrMalformed.
var (
	typeRefused  = map[types.Kind]bool{types.Optional: true, types.Table: true, types.DepMap: true, types.Case: true}
	constRefused = map[types.Kind]bool{types.Record: true, types.Variant: true, types.Case: true}
)
