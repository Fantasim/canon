package wire

import "regexp"

// The `$` keys of the data wire (WIRE.md §5.12) and the members of a document (§8.2).
const (
	keySchema  = "$schema"
	keyRows    = "rows"
	keyValue   = "value"
	keyFns     = "$fns"
	keyID      = "$id"
	keyRetired = "$retired"
	keyDollar  = "$"
)

// pairsIndex is the slot placeholder of an @json(pairs:) template (WIRE.md §5.14).
const pairsIndex = "{i}"

// The JSON punctuation of WIRE.md §7.4.
const (
	textNull     = "null"
	textTrue     = "true"
	textFalse    = "false"
	textZero     = "0"
	textOne      = "1"
	openArray    = "["
	closeArray   = "]"
	openObject   = "{"
	closeObject  = "}"
	sepCompact   = ", "
	sepPretty    = ",\n"
	comma        = ","
	quote        = `"`
	sepKey       = ": "
	newline      = "\n"
	indentSpaces = "  "
)

// Layout of a data file (WIRE.md §8.2): members at depth 1, rows at depth 2.
const (
	memberDepth = 1
	rowDepth    = 2
)

const (
	decimalBase = 10
	float32Bits = 32
	float64Bits = 64
)

// schemaPattern is the generated-file marker a `$schema` must match (WIRE.md §8.4).
var schemaPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)+@[0-9a-f]{8}$`)
