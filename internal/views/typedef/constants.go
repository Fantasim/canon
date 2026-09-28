package typedef

// The kinds of a type definition (VIEWMODEL.md §12.3).
const (
	defRecord   = "record"
	defVariant  = "variant"
	defEnum     = "enum"
	defTypeFunc = "typeFunction"
)

// The kinds of a type expression (VIEWMODEL.md §12.3) that are not a definition's kind.
const (
	exprBool      = "bool"
	exprInt       = "int"
	exprFloat     = "float"
	exprString    = "string"
	exprDuration  = "duration"
	exprList      = "list"
	exprTable     = "table"
	exprMap       = "map"
	exprOptional  = "optional"
	exprRef       = "ref"
	exprUnion     = "union"
	exprAsset     = "asset"
	exprNever     = "never"
	exprDependent = "dependent"
	exprAny       = "any"
)

// wildcard is the `match` of a `_` branch (§12.3 typeFunction).
const wildcard = "_"

// dot joins the fields of a `select` path and of a wire path.
const dot = "."

// annJSONWord is the name of the @json annotation, whose `unit:` is a Duration's wire unit.
const annJSONWord = "json"

// computedDefault is the `default` of a field whose default is not constant (TYP-15).
const computedDefault = `{"computed":true}`

// fmtArgKey is an argument of an applied record's walk key.
const fmtArgKey = "%d:%p:%p/"
