package main

import "os"

const (
	flagSchema = "schema"
	flagOutDir = "out"

	defaultSchema = "spec/viewmodel.schema.json"
	defaultOutDir = "api/vm"
	outFile       = "vm.gen.go"

	usageSchema = "the view-model JSON schema to read"
	usageOutDir = "the directory vm.gen.go is written to"

	exitOK    = 0
	exitFail  = 1
	exitUsage = 2

	filePerm    os.FileMode = 0o600
	fmtFail                 = "vmgen: %v\n"
	fmtAt                   = "%w: %s"
	fmtAtMember             = "%w: %s.%s"
	fmtSchema               = "schema: %w"
)

// nodeKind is the JSON kind of a schema node.
type nodeKind uint8

const (
	kindNull nodeKind = iota
	kindBool
	kindNumber
	kindString
	kindArray
	kindObject
)

// The JSON Schema keywords vmgen reads.
const (
	kwRef         = "$ref"
	kwDefs        = "$defs"
	kwType        = "type"
	kwProperties  = "properties"
	kwRequired    = "required"
	kwAdditional  = "additionalProperties"
	kwOneOf       = "oneOf"
	kwAllOf       = "allOf"
	kwIf          = "if"
	kwThen        = "then"
	kwConst       = "const"
	kwEnum        = "enum"
	kwPattern     = "pattern"
	kwMinLength   = "minLength"
	kwMinimum     = "minimum"
	kwMaximum     = "maximum"
	kwItems       = "items"
	kwDependent   = "dependentRequired"
	kwTitle       = "title"
	kwDescription = "description"

	defsPrefix = "#/$defs/"
	tagMember  = "kind"
	zeroNumber = "0"
	sigil      = "$"
	fieldSep   = "."
	listSep    = ", "

	scalarKinds = 2 // a oneOf of a string and an integer

	// decimalPattern is the string branch of an integer that J10 writes as a decimal string.
	decimalPattern = "^-?[0-9]+$"
)

// knownKeywords are the keywords vmgen accepts: a schema node using another is refused.
var knownKeywords = map[string]bool{
	"$schema": true, "$id": true, kwTitle: true, kwDescription: true, kwDefs: true,
	kwRef: true, kwType: true, kwProperties: true, kwRequired: true, kwAdditional: true,
	kwOneOf: true, kwAllOf: true, kwConst: true, kwEnum: true, kwPattern: true,
	kwMinLength: true, kwMinimum: true, kwMaximum: true, "minItems": true, "maxItems": true,
	"uniqueItems": true, kwItems: true, "propertyNames": true, kwDependent: true,
}

// validationKeywords may appear in a sub-schema that only validates (a oneOf or allOf beside
// an object's members, if, then); conditionKeywords in the member conditions of an if.
var (
	validationKeywords = map[string]bool{
		kwRequired: true, kwProperties: true, kwOneOf: true, kwAllOf: true, kwIf: true, kwThen: true,
	}
	conditionKeywords = map[string]bool{kwConst: true, kwEnum: true}
	unionKeywords     = map[string]bool{kwOneOf: true, kwDescription: true, kwTitle: true}
)

// typeKind is the kind of a generated Go type.
type typeKind uint8

const (
	tString typeKind = iota
	tInt
	tBool
	tNumber
	tScalar
	tTextRef
	tRaw
	tSlice
	tMap
	tStruct
)

// goNames spells the Go type of each kind but tStruct, or the prefix of its element's.
var goNames = [...]string{
	tString: "string", tInt: "int", tBool: "bool", tNumber: "Number", tScalar: "Scalar",
	tTextRef: "TextRef", tRaw: "json.RawMessage", tSlice: "[]", tMap: "map[string]",
}

// scalarOf is the Go kind of a const or enum value of each JSON kind; zeroTexts its zero value.
var (
	scalarOf  = map[nodeKind]typeKind{kindString: tString, kindBool: tBool, kindNumber: tInt}
	zeroTexts = map[nodeKind]string{kindString: "", kindBool: "false", kindNumber: zeroNumber}
)

// pointerable are the kinds a pointer makes optional when their zero value is also a value.
var pointerable = map[typeKind]bool{tString: true, tInt: true, tBool: true}

// jsonTypes maps the "type" keyword's scalar values to their Go kind.
var jsonTypes = map[string]typeKind{
	"string": tString, "integer": tInt, "boolean": tBool, "number": tNumber,
}

const (
	jsonArray  = "array"
	jsonObject = "object"
)

// specialDefs are the definitions a hand-written type of package vm carries.
var specialDefs = map[string]typeKind{"textRef": tTextRef}

// initialisms spell a member or definition name whose Go name is not its capitalization.
var initialisms = map[string]string{"id": "ID", "i18n": "I18N"}

// nameOverride names the struct of the inline object found at site (struct.member).
type nameOverride struct{ site, name string }

// names are the Go names of inline objects whose default (struct + member) reads badly.
var names = []nameOverride{
	{"ViewModel.assets", "Asset"},
	{"TypeDef.params", "Param"},
	{"TypeDef.methods", "Method"},
	{"TypeDef.cases", "Case"},
	{"TypeDef.members", "Member"},
	{"TypeDef.branches", "Branch"},
	{"Control.source", "Source"},
	{"Control.columns", "Column"},
	{"Control.filters", "Filter"},
	{"Section.entries", "Entry"},
	{"View.preview", "Preview"},
	{"View.methods", "MethodView"},
	{"View.shows", "ShowView"},
	{"Value.sources", "Sources"},
	{"Usage.shapes", "Shape"},
	{"SearchIndex.preview", "SearchPreview"},
	{"SearchIndex.rows", "Row"},
	{"Row.tr", "RowText"},
	{"I18N.languages", "Language"},
	{"Finding.related", "Related"},
	{"Finding.stack", "Frame"},
}

const (
	rootName      = "ViewModel"
	fileHeader    = "// Code generated by vmgen from spec/viewmodel.schema.json. DO NOT EDIT.\n\npackage vm\n"
	importJSON    = "\nimport \"encoding/json\"\n"
	docRoot       = "is the view-model document (VIEWMODEL.md §12.1)."
	docDef        = "is %s of spec/viewmodel.schema.json."
	docUnion      = "is %s of spec/viewmodel.schema.json, its %d branches in one struct."
	docInline     = "is the object of %s."
	fmtStruct     = "\n// %s %s\ntype %s struct {\n"
	fmtField      = "\t%s %s `json:\"%s%s\"`%s\n"
	endStruct     = "}\n"
	omitZero      = ",omitzero"
	commentPrefix = " // "
	kindNote      = "kind: "
	viaNote       = "%s only"
	noteSep       = "; "
	ptrPrefix     = "*"
)
