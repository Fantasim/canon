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

// The forms of a path segment: a root's name, a field or table entry, a key or index.
const (
	segRoot segForm = iota
	segField
	segKey
)

// segOpen and segClose are the punctuation around a segment of each form (API.md P8).
var (
	segOpen  = [...]string{segField: dot, segKey: keyOpen}
	segClose = [...]string{segKey: keyClose}
)

// The punctuation of an application's computed type, the record and its argument values.
const (
	argsOpen  = "("
	argsSep   = ", "
	argsClose = ")"
)

// How a ref's target was reached: forced, poisoned, or a level-1 ref left unbound (eval's E3505).
const (
	reached reach = iota
	poisoned
	unbound
)

// How reading a type argument ended: a value, none there, or a read that aborts the root.
const (
	found read = iota
	missing
	aborted
)

const retiredMark = "retired:"

// fmtUnjudged is ErrUnjudged with why and the application.
const fmtUnjudged = "%w: %s: %s"

// Why a dependent value is unjudged; each stops verification rather than hand on a symbol.
const (
	reasonNoStage = "the evaluator does not evaluate dependent types"
	reasonArity   = "the arguments do not match the parameters"
	reasonArm     = "no arm covers the scrutinee"
	reasonMissing = "an argument has no value"
)

// fmtNoBag names the package Check has no bag for.
const fmtNoBag = "%w: %q"

// codesAnnotation is the annotation E3102's code variant relates (TYPES.md §8.1).
const codesAnnotation = "codes"

// memoEntries bounds the entries a Memo keeps: past it, it forgets them all (as eval's memo does).
const memoEntries = 1 << 20
