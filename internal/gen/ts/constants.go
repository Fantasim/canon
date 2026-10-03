package tsgen

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// Files and headers (CODEGEN.md §2.3–§2.5).
const (
	conformanceSuffix         = ".conformance.test.ts"
	jsExt                     = ".js"
	importTypeFormat          = "import type { %s } from \"%s\";"
	importValueFormat         = "import { %s } from \"%s\";"
	importPureFormat          = "import { %s } from \"%s\";\n"
	importNodeTest            = "import { test } from \"node:test\";"
	importNodeAssert          = "import * as assert from \"node:assert/strict\";"
	helperOrigin              = "the helper block"
	importOrigin              = "an import of %s"
	bannerLines               = 3
	identPattern              = `^[A-Za-z_$][A-Za-z0-9_$]*$`
	declPattern               = `^(?:export )?(?:class|function|interface|const) ([A-Za-z_$][A-Za-z0-9_$]*)`
	canonEvalErrorName        = "CanonEvalError"
	canonTableInterface       = "CanonTable"
	cellsPerLine              = 16
	maxSafeInt          int64 = 1<<53 - 1
)

// Punctuation and layout.
const (
	newline      = "\n"
	indent       = "  "
	space        = " "
	dot          = "."
	comma        = ","
	semicolon    = ";"
	listSep      = ", "
	listEnd      = ",\n"
	keyValueSep  = ": "
	underscore   = "_"
	pathSep      = "/"
	parentDir    = ".."
	currentDir   = "./"
	lparen       = "("
	rparen       = ")"
	lbrace       = "{"
	rbrace       = "}"
	lbracket     = "["
	rbracket     = "]"
	plusSep      = " + "
	orSep        = " || "
	unionSep     = " | "
	assignSep    = " = "
	strictEq     = " === "
	notOp        = "!"
	minus        = "-"
	optionalDot  = "?."
	coalesceOp   = " ?? "
	coalesceNull = " ?? null"
	asKw         = " as "
	emptyObject  = "{}"
	emptyString  = `""`
	zeroLit      = "0"
	bigSuffix    = "n"
	negativeZero = "-0"
	decimal      = 10
	float32Bits  = 32
	float64Bits  = 64
	int64Bits    = 64
	pairFields   = 2
	vectorWant   = "want"
	vectorCode   = "code"
	vectorTail   = 2
)

// Doc comments (CODEGEN.md §2.6).
const (
	docOpen           = "/**"
	docClose          = "*/"
	docEnd            = " */"
	docStar           = " * "
	docStarEmpty      = " *"
	commentEnd        = "*/"
	commentEndEscaped = "*\\/"
)

// TypeScript types and literals.
const (
	tsNumber       = "number"
	tsString       = "string"
	tsBoolean      = "boolean"
	tsBigint       = "bigint"
	tsNull         = "null"
	tsUndefined    = "undefined"
	tsUnknown      = "unknown"
	tsNever        = "never"
	boolKeys       = "\"false\" | \"true\""
	boolKeyList    = "[\"false\", \"true\"]"
	identityWire   = "(k) => k"
	codeWireFormat = "(k) => String(%s[k])"
	objectType     = "Record<string, unknown>"
	mathObject     = "Math"
	mathAbs        = "abs"
	stringFn       = "String"
	numberFn       = "Number"
)

// Names of generated items (CODEGEN.md §3.3).
const (
	kindSuffix    = "Kind"
	branchSuffix  = "Branch"
	idSuffix      = "Id"
	membersSuffix = "Members"
	namesSuffix   = "Names"
	indexSuffix   = "Index"
	codesSuffix   = "Codes"
	schemaSuffix  = "Schema"
	kindProp      = "kind"
	idProp        = "id"
	retiredProp   = "retired"
	pureMark      = "$"
	selfParam     = "self"
	readPrefix    = "read"
	decodePrefix  = "decode"
)

// The helper units of CODEGEN.md §8.2 the generator references.
const (
	canonMapName         = "CanonMap"
	canonFreezeName      = "canonFreeze"
	canonTableName       = "canonTable"
	canonEnvelopeName    = "canonEnvelope"
	canonIntName         = "canonInt"
	canonNegName         = "canonNeg"
	canonFName           = "canonF"
	canonF32Name         = "canonF32"
	canonDivDurationName = "canonDivDuration"
	canonCheckRangeName  = "canonCheckRange"
	canonCheckWidthName  = "canonCheckWidth"
)

// The decoder helper units of text/decode.ts.txt.
const (
	decAtName          = "decAt"
	decBigName         = "decBig"
	decBitName         = "decBit"
	decBitsName        = "decBits"
	decBoolName        = "decBool"
	decCodeName        = "decCode"
	decDurationName    = "decDuration"
	decEmptyArrayName  = "decEmptyArray"
	decEmptyObjectName = "decEmptyObject"
	decEnumName        = "decEnum"
	decF32Name         = "decF32"
	decFailName        = "decFail"
	decFloatName       = "decFloat"
	decForeignName     = "decForeign"
	decGetName         = "decGet"
	decIntKeyName      = "decIntKey"
	decBigKeyName      = "decBigKey"
	decIntName         = "decInt"
	decKeyedName       = "decKeyed"
	decListName        = "decList"
	decMapName         = "decMap"
	decObjectName      = "decObject"
	decPairsName       = "decPairs"
	decSameName        = "decSame"
	decStringName      = "decString"
	decTableName       = "decTable"
)

// Declaration shapes (CODEGEN.md §5, §8.3).
const (
	unionFormat          = "export type %s = %s;\n"
	unionOpenFormat      = "export type %s =\n"
	unionMemberFormat    = "  | %s\n"
	membersFormat        = "export const %s: ReadonlyArray<%s> = Object.freeze([%s]);\n"
	recordConstFormat    = "export const %s: Readonly<Record<%s, %s>> = Object.freeze({ %s });\n"
	indexConstFormat     = "export const %s: Readonly<Record<%s, number>> = Object.freeze(%s);\n"
	interfaceFormat      = "export interface %s {\n%s}\n"
	emptyInterfaceFormat = "export interface %s {}\n"
	propFormat           = "  readonly %s: %s;\n"
	dependentFormat      = "export type %s =\n%s;\n"
	branchArmFormat      = "  | { readonly branch: %s; readonly value: %s }"
	bareCaseFormat       = "{ readonly kind: %s }"
	bareLitFormat        = "{ kind: %s }"
	constFormat          = "export const %s = %s;\n"
	typedConstFormat     = "export const %s: %s = %s;\n"
	freezeFormat         = "%s<%s>(%s)"
	frozenConstFormat    = "const %[1]s: %[2]s = %[3]s<%[2]s>(%[4]s);\n"
	getterFormat         = "export function %s(): %s {\n  return %s;\n}\n"
	tableFormat          = "export const %[1]s: CanonTable<%[2]s, %[3]s> = canonTable(%[4]s<ReadonlyArray<%[3]s>>([%[5]s]), (e) => e.%[6]s);\n"
	tableConstFormat     = "const %[1]s: ReadonlyArray<%[2]s> = %[3]s<ReadonlyArray<%[2]s>>([\n%[4]s\n]);\n"
	lookupFormat         = "export function %s(%s): %s {\n  return %s[%s];\n}\n"
	strideFormat         = "%s * %d"
	boolOrdinalFormat    = "(%s ? 1 : 0)"
	indexOfFormat        = "%s[%s]"
	readonlyArrayFormat  = "ReadonlyArray<%s>"
	readonlyMapFormat    = "ReadonlyMap<%s, %s>"
	readonlyRecordFormat = "Readonly<Record<%s, %s>>"
	mapEntryFormat       = "[%s, %s]"
	newMapFormat         = "new CanonMap<%s, %s>([%s])"
	branchValueFormat    = "{ branch: %s, value: %s }"
)

// Translated functions (CONFORMANCE.md §2.3, §3).
const (
	functionFormat  = "export function %s(%s): %s {\n%s}\n"
	callFormat      = "%s(%s)"
	call2Format     = "%s(%s, %s)"
	checkFormat     = "%s(%s, %s, %s)"
	letFormat       = "%sconst %s = %s;\n"
	returnFormat    = "%sreturn %s;\n"
	ifOpenFormat    = "%sif (%s) {\n"
	elseIfFormat    = "%s} else if (%s) {\n"
	condFormat      = "%s ? %s : %s"
	nullGuardFormat = "%s === null ? null : %s(%s)"
)

// Decoders (CODEGEN.md §5.13, §8.1).
const (
	rawParam            = "raw"
	pathParam           = "path"
	idParam             = "id"
	jsonParam           = "json"
	objVar              = "o"
	discVar             = "disc"
	tagVar              = "tag"
	localPrefix         = "v"
	rawPrefix           = "r"
	optionalMark        = "?"
	lambdaRaw           = "x"
	lambdaPath          = "p"
	lambdaKey           = "k"
	lambdaID            = "id"
	pairRawA            = "a"
	pairRawB            = "b"
	pairPathA           = "pa"
	pairPathB           = "pb"
	dollarKey           = "$"
	unknownCase         = "unknown case"
	defaultedFormat     = "%s === undefined ? %s : %s"
	nullOrFormat        = "%s === null ? null : %s"
	lambdaFormat        = "(x, p) => %s"
	tableLambdaFormat   = "(x, p, id) => %s"
	keyLambdaFormat     = "(k, p) => %s"
	pairsLambdaFormat   = "(a, b, pa, pb) => ({ %s })"
	typeArgsFormat      = "<%s>"
	readerFormat        = "function %s(%s): %s {\n%s}\n"
	variantBodyFormat   = "  const %s = %s;\n  const %s = %s;\n  switch (%s) {\n%s    default:\n      return %s;\n  }\n"
	caseArmFormat       = "    case %s:\n      return %s;\n"
	caseLabelFormat     = "    case %s:\n"
	branchReturnFormat  = "      return %s;\n"
	publicDecoderFormat = "export function %s(json: unknown): %s {\n  return %s(%s);\n}\n"
	idReadFormat        = "id ?? decString(decGet(o, \"$id\"), path + \"/$id\")"
	retiredReadFormat   = "decGet(o, \"$retired\") === undefined ? false : decBool(decGet(o, \"$retired\"), path + \"/$retired\")"
	dependentBodyFormat = "  switch (disc) {\n%s    default:\n      return decFail(path, \"no branch for this value\");\n  }\n"
	tableDecoderFormat  = "export function %[1]s(json: unknown): %[2]s {\n  const doc = canonEnvelope(json, %[3]s, %[4]s);\n  return canonTable(canonFreeze(decList(decGet(doc, \"rows\"), \"/rows\", (x, p) => %[5]s)), (e) => e.%[6]s);\n}\n"
	tableTypeFormat     = "CanonTable<%s, %s>"
	looseIDFormat       = "id === undefined ? undefined : "
	looseEntryFormat    = "...(%s === undefined ? {} : { id: %s, retired: %s })"
	valueRaw            = "decGet(doc, \"value\")"
	valuePath           = "\"/value\""
	parseFormat         = "export function %[1]s(text: string): %[2]s {\n  return %[3]s(decParse(text));\n}\n"
	parsePrefix         = "parse"
	decParseName        = "decParse"
	valueDecoderFormat  = "export function %[1]s(json: unknown): %[2]s {\n  const doc = canonEnvelope(json, %[3]s, %[4]s);\n  return canonFreeze(%[5]s);\n}\n"
	retiredFormat       = "%sRetired."
	elseFormat          = "%s} else {\n"
	unknownKindFormat   = "unknown kind %d"
	localFormat         = "const %s = %s;"
	typedLocalFormat    = "const %s: %s = %s;"
)

// Conformance test shapes (CONFORMANCE.md §7.2).
const (
	testFormat = `test("%[1]s conformance", () => {
  const vectors: ReadonlyArray<readonly [%[2]s]> = [
%[3]s
  ];
  for (const [%[4]s] of vectors) {
    const [got, gotCode] = canonCatch(() => %[5]s);
    assert.equal(gotCode, code, %[6]s);
    if (code === "") assert.ok(Object.is(got, want), %[7]s);
  }
});
`
	messageFormat    = "`%s(%s)`"
	failureFormat    = "`%s(%s) = ${got}, canon says ${want}`"
	shownFormat      = "%s=${%s}"
	paramTypeFormat  = "Parameters<typeof %s>[%d]"
	returnTypeFormat = "ReturnType<typeof %s> | undefined"
)

// kindNames name a kind in a message.
var kindNames = [...]string{
	types.Bool: "Bool", types.Int: "Int", types.Float: "Float", types.String: "String", types.Duration: "Duration",
	types.Enum: "enum", types.Record: "record", types.Variant: "variant", types.Case: "case", types.VariantKind: "variant kind",
	types.Optional: "optional", types.List: "list", types.Map: "map", types.DepMap: "dependent map", types.Table: "table",
	types.Ref: "ref", types.LitUnion: "literal union", types.Never: "Never", types.Range: "Range", types.Func: "function",
	types.TypeApp: "dependent type",
}

// alwaysUnits are the helper units every file holds (CODEGEN.md §8.2).
var alwaysUnits = []string{canonEvalErrorName, canonTableInterface}

// nativeOps are the operators TypeScript runs natively (CONFORMANCE.md §3).
var nativeOps = map[ir.Op]string{
	ir.OpEq: "===", ir.OpNe: "!==", ir.OpLt: "<", ir.OpLe: "<=", ir.OpGt: ">", ir.OpGe: ">=", ir.OpAnd: "&&", ir.OpOr: "||",
}

// intOps are the checked helpers of Int and Duration arithmetic; floatOps the operators of Float arithmetic.
var (
	intOps = map[ir.Op]string{
		ir.OpAdd: "canonAdd", ir.OpSub: "canonSub", ir.OpMul: "canonMul", ir.OpDiv: "canonDiv", ir.OpMod: "canonMod",
	}
	floatOps = map[ir.Op]string{ir.OpAdd: "+", ir.OpSub: "-", ir.OpMul: "*", ir.OpDiv: "/", ir.OpMod: "%"}
)

// builtinInt and builtinFloat are the helpers of the built-in functions by operand kind; min and max on integers are Math's.
var (
	builtinInt = map[ir.Builtin]string{
		ir.BuiltinMin: "min", ir.BuiltinMax: "max", ir.BuiltinAbs: "canonAbs", ir.BuiltinClamp: "canonClamp",
		ir.BuiltinFloor: "canonFloor", ir.BuiltinCeil: "canonCeil", ir.BuiltinRound: "canonRound", ir.BuiltinInt: "canonToInt",
	}
	builtinFloat = map[ir.Builtin]string{ir.BuiltinMin: "canonMinF", ir.BuiltinMax: "canonMaxF", ir.BuiltinClamp: "canonClampF"}
)

// reservedLoop are the names of a conformance test's own variables.
var reservedLoop = []string{"got", "want", "code", "gotCode", "vectors", "test", "assert"}

// reservedWords are the names a top-level binding or a parameter may not take (CODEGEN.md §3.4): stage E's list.
var reservedWords = ir.TSReservedWords()
