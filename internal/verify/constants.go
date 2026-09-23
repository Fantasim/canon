package verify

// The punctuation of a canonical value path (API.md §6.1); dot also starts an extension.
const (
	dot        = "."
	keyOpen    = "["
	keyClose   = "]"
	underscore = "_"
)

// The separators and relative segments of an asset path (TYPES.md §13.4).
const (
	pathSep   = "/"
	backslash = `\`
	parentDir = ".."
)

const decimalBase = 10

// How a ref's target was reached: forced, poisoned, or a level-1 ref left unbound (eval's E3505).
const (
	reached reach = iota
	poisoned
	unbound
)

// fmtNoBag names the package Check has no bag for.
const fmtNoBag = "%w: %q"

// codesAnnotation is the annotation E3102's code variant relates (TYPES.md §8.1).
const codesAnnotation = "codes"
