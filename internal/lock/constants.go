package lock

import "github.com/fantasim/canonlang/internal/diag"

// The kinds of LOCK.md §1, in canonical order (§2.3: enum < field < table).
const (
	KindEnum Kind = iota
	KindField
	KindTable
)

// kindNames are the kinds as canon.lock writes them.
var kindNames = [...]string{"enum", "field", "table"}

// shapes gives, for each kind word, the fields after it (LOCK.md §2.2).
var shapes = map[string]shape{
	"table": {kind: KindTable, fields: 2, retiring: true},
	"enum":  {kind: KindEnum, fields: 3, value: true, retiring: true},
	"field": {kind: KindField, fields: 3, value: true},
}

// mergeMarkers start the lines git writes around a conflict (LOCK.md §2.4).
var mergeMarkers = [...]string{"<<<<<<<", "=======", ">>>>>>>"}

// The file grammar (LOCK.md §2.2).
const (
	versionPrefix    = "# canon.lock v"
	supportedVersion = "1"
	header           = versionPrefix + supportedVersion
	lineBreak        = "\n"
	carriageReturn   = "\r"
	separator        = "  "
	padding          = "     "
	kindWidth        = len(padding)
	space            = " "
	nameSep          = "."
	retiredWord      = "retired"
	jsonQuote        = `"`
	minusSign        = "-"
	underscore       = "_"
	zeroDigit        = "0"
	digitChars       = "0123456789"
	identChars       = "_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" + digitChars
	decimalBase      = 10
	int64Bits        = 64
)

// Why a line of a known kind is refused.
const (
	problemNone problem = iota
	problemSyntax
	problemPackage
)

// Texts of Add's refusals: the sentinel, the reason, then the line Format would write.
const (
	fmtBadFact     = "%w: %w"
	fmtBadFactLine = "%w: %w: %q"
)

// fmtNotStable names the table AddTable refused.
const fmtNotStable = "%w: %s is not a stable table"

// collRules applies LOCK.md §4.1 and §4.2 to one collection, by kind.
var collRules = [...]func(c *comparison){
	KindEnum:  enumRules,
	KindField: fieldRules,
	KindTable: tableRules,
}

// goneKinds are the words E6001 names a gone collection with.
var goneKinds = [...]diag.Kind{KindEnum: diag.KindEnum, KindField: diag.KindField, KindTable: diag.KindTable}
