package views

// SchemaVersion is the view model's `$schema` (VIEWMODEL.md V1).
const SchemaVersion = "canon-vm/1"

const (
	fmtWrap    = "views: %w"
	fmtPackage = "%w: %q"
)

// The separators of value ids and text references (J8, J9) and of qualified names (J7).
const (
	colon = ":"
	dot   = "."
)

// The edit modes at a value's root and the reasons it has none (VIEWMODEL.md 12.6, API.md 7).
const (
	editCanon      = "canon"
	editJSON       = "json"
	editNone       = "none"
	reasonComputed = "computed"
	reasonFormat   = "format"
)

// loadDirMethod is the `load` form that reads a directory (WIRE.md 6.1): `load.dir`.
const loadDirMethod = "dir"

// usageShare is L7's threshold: a field is in `main` when 4 × set(f) ≥ count (25 %).
const usageShare = 4

// sharedTitle is how many entries share a title that S9 disambiguates.
const sharedTitle = 2

// The fields of an entry of the studio package's `units` table (VIEWMODEL.md 12.9).
const (
	unitSuffix    = "suffix"
	unitScale     = "scale"
	unitThousands = "thousands"
	unitDecimals  = "decimals"
)

// The struct tags and members of api/vm the walk of `requires` reads (12.1).
const (
	jsonTag       = "json"
	jsonSep       = ","
	driversMember = "drivers"
	unitMember    = "unit"
	widgetMember  = "widget"
	assetKind     = "asset"
)

// The members naming something of another package or of the studio, and those skipped (12.1).
var (
	qualifiedMembers = []string{"ref", "of", "enum", "cases", "fn", "element", "type"}
	valueIDMembers   = []string{"collection", "search"}
	studioMembers    = []string{"menu", "icon", "tone"}
	skippedMembers   = []string{"i18n", "findings", "requires", "package", "$schema"}
)

// The shape key of a record (VIEWMODEL.md L19): `v=c/w=c2`.
const (
	shapeIs  = "="
	shapeSep = "/"
)
