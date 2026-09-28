package encode

// Separators of qualified names, value ids and text keys (VIEWMODEL.md J7–J9).
const (
	dot   = "."
	colon = ":"
)

// The case path of a case field (VIEWMODEL.md L18): `v=c/w=c2`.
const (
	caseIs  = "="
	caseSep = "/"
)

// The element of a define table and the key types of a ref (VIEWMODEL.md J12).
const (
	elemDefine = "$define"
	keyInt     = "int"
	keyString  = "string"
)

// The JSON pieces of the value encoding (VIEWMODEL.md J10).
const (
	valueNull   = "null"
	KeyCase     = "$case" // also the case column of a table of variants (T6a)
	KindField   = "kind"  // a variant's built-in case (`v.kind`), a filter on the case (T6a)
	openArray   = '['
	closeArray  = ']'
	openObject  = '{'
	closeObject = '}'
	comma       = ','
	colonByte   = ':'
)
