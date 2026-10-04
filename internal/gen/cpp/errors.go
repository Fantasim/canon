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
	defineMissing      = "a ref into a load.defines table the package's IR does not hold"
	unionNonString     = "a literal union whose wire is not a string"
	unionRef           = "a literal union over a ref"
	unionDependent     = "a literal union over a dependent type"
	optionalElems      = "a list of optional elements"
	dataValueKind      = "a data value that is not a table, a keyed list or a record"
	recursiveTypes     = "a type that holds itself"
	dependentElsewhere = "a dependent type that is not a field's type or its list's elements"
	dependentNoBranch  = "a dependent type every arm of which is Never"
	dependentNoDisc    = "a dependent type without its Bool or enum discriminant"
	dependentBadArm    = "a dependent type whose members and branches disagree"
	dependentBadPath   = "a dependent field whose discriminant is not read from earlier required fields of its class (ir.DiscFields)"
	packageStoredFn    = "a package-level stored export fn in data or types mode"
	typesStoredFn      = "a stored export fn in types mode"
	typesComputed      = "a computed default in types mode"
	typesInputs        = "an input field in types mode, which writes no LoadInputs"
	dependentDefault   = "a default holding a dependent value"
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
	bakedReload        = "a @reload value in baked mode"
	bakedEntryKey      = "a container entry whose key is not its id, or a ref to no entry"
	bakedNoInstance    = "a stored method without a result for a receiver"
	bakedNoField       = "a record value without one of its fields"
	bakedCells         = "a lookup whose cells do not fill its domains"
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
	typeRefused  = map[types.Kind]bool{types.Optional: true, types.DepMap: true, types.Case: true}
	constRefused = map[types.Kind]bool{types.Record: true, types.Variant: true, types.Case: true, types.Table: true}
)

// Strict loaders: calls of canon_runtime_json.h's detail helpers (CODEGEN.md §7.5).
const (
	jsonDetail         = "canon::json::detail::"
	bracedFormat       = "{%s}"
	keysReturnFormat   = "if (!" + jsonDetail + "Keys(%s, dec, %s)) return false;"
	keysStmtFormat     = jsonDetail + "Keys(%s, dec, %s);"
	objectReturnFormat = "if (!" + jsonDetail + "Object(%s, dec)) return false;"
	stepOpenFormat     = "if (const nlohmann::json* %s = " + jsonDetail + "Step(%s, dec, %s)) {"
	cellOpenFormat     = "if (const nlohmann::json* %s = " + jsonDetail + "Cell(%s, dec, %s, %t)) {"
	intBoolFormat      = "if (dec.AsIntIn(%s, %s, 0, 1, n)) %s = n == 1;"
	bitsTempLine       = "uint64_t n = 0;"
	maskFormat         = "0x%x"
	emptyFlagLine      = "bool empty = false;"
	slotCheckFormat    = "if (!" + jsonDetail + "Slot(%s, dec, %s[s], %s[s], empty)) continue;"
	cellsArrayFormat   = "constexpr const char* cells[] = {%s};"
	cellsKey           = "cells[c]"
	definesOpenFormat  = "constexpr std::array<std::pair<std::string_view, int64_t>, %d> %s = {{" + newline
	defineEntryFormat  = "{%s, %s},"
	definesClose       = "}};"
	defineCallFormat   = jsonDetail + "Define(%s, %s, dec, %s, %s, %s);"
	emplaceCall        = ".emplace()"
	emplaceBackCall    = ".emplace_back()"
)

// Table fields (CODEGEN.md §4.2, §5.8; WIRE.md §5.7): a nested table is a canon::KeyedList of its rows by id, read from an object in file order.
const (
	tableEntriesFormat = "std::vector<" + jsonDetail + "TableEntry> tab%[1]d;"
	tableReadFormat    = "if (" + jsonDetail + "Table(%[1]s, dec, tab%[2]d)) {"
	tableRowsFormat    = "std::vector<%[1]s> tv%[2]d(tab%[2]d.size());"
	tableKeysLine      = "std::vector<std::string> tk%d;"
	tableLoopFormat    = "for (size_t ti%[1]d = 0; ti%[1]d < tab%[1]d.size(); ++ti%[1]d) {"
	tableRowPush       = "dec.Push(*tab%[1]d[ti%[1]d].key);"
	tableDecodeFormat  = "%[1]s(*tab%[2]d[ti%[2]d].row, dec, tv%[2]d[ti%[2]d]);"
	tableIDFormat      = "tv%[1]d[ti%[1]d]." + idMember + " = *tab%[1]d[ti%[1]d].key;"
	tableRetiredFormat = "tv%[1]d[ti%[1]d]." + retiredMember + " = tab%[1]d[ti%[1]d].retired;"
	tableKeyPushFormat = "tk%[1]d.push_back(*tab%[1]d[ti%[1]d].key);"
	tableFromFormat    = "%[1]s = canon::KeyedList<std::string, %[2]s>::FromRows(std::move(tv%[3]d), std::move(tk%[3]d));"
	tableIDExprFormat  = "%s.At(%s)." + idMember
	tableOrdersDecl    = "\n        canon::json::KeyOrders orders;"
	tableOrdersArg     = ", orders"
	tableFieldText     = "a table field of a record of another package, or of no record"
)

// A baked emit's id enums, data, accessors and constexpr lookups (CODEGEN.md §5.3, §5.9, §5.10, §7.3; decision 293).
const (
	dataPrefix            = "d."
	dataGetFormat         = "detail::%s::Get().%s"
	allocateFormat        = "%srows_ = canon::KeyedList<%s, %s>::FromRows(std::vector<%s>(%d), {"
	allocateClose         = "});"
	sortedIndexDataFormat = "%s = canon::detail::SortedIndex(%s);"
	rowRefFormat          = "auto& %s = const_cast<%s&>(%s.rows_.At(%d));"
	rowVarFormat          = "r%d"
	rowAliasFormat        = "auto& %s = %s;"
	emplaceStmtFormat     = "%s.emplace();"
	makeUniqueFormat      = "%s = std::make_unique<%s>();"
	entryAddrFormat       = "&%s.rows_.At(%d)"
	emplaceCaseFormat     = "%s." + ir.CppVariantMember + ".emplace<%d>();"
	caseRefFormat         = "auto& %s = %s." + ir.CppVariantMember + ".emplace<%d>();"
	emplaceBranchFormat   = "%s." + ir.CppVariantMember + ".emplace<%d>(%s);"
	resizeFormat          = "%s.resize(%d);"
	entriesFormat         = "std::vector<%s::Entry> %s(%d);"
	entryFirst            = ".first"
	entrySecond           = ".second"
	fromEntriesMoveFormat = "%s::FromEntries(std::move(%s))"
	keyedRowsFormat       = "%s = canon::KeyedList<%s, %s>::FromRows(std::vector<%s>(%d), {%s});"
	declFormat            = "%s;\n"
	funcLineFormat        = "%s { %s }\n"
	funcOpenFormat        = "%s {\n"
	cellsFormat           = "inline constexpr std::array<%s, %d> %s = %s;\n"
	constexprPrefix       = "inline constexpr "
	maybeUnused           = "[[maybe_unused]] "
	idFromWireDeclFormat  = "std::optional<%s> %s(std::string_view key);\n"
	getFormat             = "const %s& Get(%s id) const { const size_t i = static_cast<size_t>(id); if (i >= rows_.Len()) std::abort(); return rows_.At(i); }"
	// maxInline is the widest initializer written on one line; a longer one puts one item per line.
	maxInline = 80
)
