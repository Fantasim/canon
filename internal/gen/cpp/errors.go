package cppgen

import (
	"errors"

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
	// ErrName is a generated name C++ cannot declare (CODEGEN.md §3.4).
	ErrName = errors.New("cppgen: not a C++ identifier")
	// ErrNameCollision is two generated names equal in one C++ scope (CODEGEN.md §3.5).
	ErrNameCollision = errors.New("cppgen: generated name collision")
)

// What this generator refuses (ErrUnsupported, decision 124) or finds malformed (ErrMalformed).
const (
	nilItem          = "a nil item"
	noElem           = "a list, optional or table without its element type"
	noKey            = "a map or ref without its key type"
	noDecl           = "a record, variant or enum kind without its declaration"
	noKeyField       = "a keyed list without its key field"
	defineRefs       = "a ref into a load.defines table"
	unionNonString   = "a literal union whose wire is not a string"
	optionalElems    = "a list of optional elements"
	dataValueKind    = "a data value that is not a table, a keyed list or a record"
	recursiveTypes   = "a type that holds itself"
	dependentTypes   = "a dependent type"
	packageStoredFn  = "a package-level stored export fn in data mode"
	legacyStructs    = "a legacy struct (@cpp(struct:))"
	inputFields      = "an input field"
	inlineFields     = "an optional or non-variant @json(inline) field"
	optionalStable   = "an optional @stable field"
	methodCalls      = "a call to another export method"
	severalHolders   = "a resolvable ref in a record several emitted values hold"
	fieldlessMethods = "export fns of a case without fields, which has no As accessor"
	lookupParams     = "a finite parameter that is not an enum or a Bool"
	unknownReads     = "a read of self that is not a path of fields"
	mapFields        = "a map field (nlohmann::json does not keep the key order)"
	foreignPairs     = "a pairs field of a record from another package"
	inlineFoldKeys   = "an inline variant key equal to another key of its parent but for letter case"
	snapshotOrigin   = "the snapshot"
	helpersOrigin    = "the JSON loader helpers"
	noneMarkerFormat = "none marker %s"
	typeFormat       = "type %T"
	valueFormat      = "value %T"
	exprFormat       = "expression %T"
	kindNumberFormat = "kind %d"
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
	nullElemFormat        = "if (%s.is_null()) return dec.Fail(%s, \"null\"), false;"
	failureTextFormat     = "%s: %s(%s) = %s [%%.*s], canon says %s [%%.*s]\n"
)
