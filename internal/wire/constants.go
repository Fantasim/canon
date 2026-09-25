package wire

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
)

// KeySchema is the key of a data file's marker (WIRE.md §8.4).
const KeySchema = "$schema"

// The `$` keys of the data wire (WIRE.md §5.12) and the members of a document (§8.2).
const (
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

// Decoding: exact numbers (WIRE.md §3.3, §5.1), Canon literals in CSV cells (GRAMMAR.md §2.4).
const (
	pointSep      = "."
	minusSign     = "-"
	plusSign      = "+"
	signChars     = "+-"
	exponentChars = "eE"
	fractionChars = ".eE"
	underscore    = "_"
	digitZero     = '0'
	radixLower    = "0123456789abcdef"
	radixUpper    = "0123456789ABCDEF"
	hexBase       = 16
	binBase       = 2
	formCSV       = "load.csv"

	// maxExponent saturates a decimal exponent far past every exact range.
	maxExponent = 1 << 40
	// maxMillisDigits: a number of units with more integer digits exceeds the Duration range.
	maxMillisDigits = 19
	// maxScale: unit ms have at most ten factors of 2 or 5, so a finer number is never whole ms.
	maxScale = 20
	// maxIntDigits: an integer token with more digits is past Int, and is never parsed.
	maxIntDigits = 19
	// maxExactBits: a CSV integer literal with more bits is past every Float, Int and Duration.
	maxExactBits = 1100
	// beyondRange is the decimal text such a literal reads as: out of every numeric range.
	beyondRange = "1e999"
	// hintDistance is the most edits between an unknown enum wire and the member it suggests.
	hintDistance = 2
)

// pairKeys is the number of templates of @json(pairs:), a key and a value (WIRE.md §5.14).
const pairKeys = len(types.Pairs{}.Keys)

// radixPrefixes are the prefixes of Canon's hexadecimal and binary integer literals.
var radixPrefixes = [...]struct {
	prefix string
	base   int
}{{"0x", hexBase}, {"0b", binBase}}

// wordPattern is an identifier or a reserved word: a table key (WIRE.md §5.7, GRAMMAR §2.3).
var wordPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// slotPattern is the slot index a @json(pairs:) key holds (WIRE.md §5.14).
var slotPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

// intKeyPattern is an integer map key or code as the wire writes it (WIRE.md §5.8).
var intKeyPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// jsonKinds names each JSON kind in E7110.
var jsonKinds = [...]diag.Kind{
	jsonsrc.Null: diag.KindNull, jsonsrc.Bool: diag.KindBoolean, jsonsrc.Number: diag.KindNumber,
	jsonsrc.String: diag.KindString, jsonsrc.Array: diag.KindArray, jsonsrc.Object: diag.KindObject,
}

// nullKinds may read a JSON null: an optional, or a dependent type whose branch decides.
var nullKinds = [...]bool{types.Optional: true, types.TypeApp: true, types.Error: false}

// starKinds may read an `at:` `*` level: a list or a map, or a type leading to one.
var starKinds = [...]bool{
	types.List: true, types.Map: true, types.DepMap: true, types.Optional: true,
	types.LitUnion: true, types.TypeApp: true, types.Error: false,
}

// entryState is where a pending entry's second pass is (decode_later.go).
type entryState uint8

const (
	entryWaiting entryState = iota
	entryRunning
	entryDone
)
