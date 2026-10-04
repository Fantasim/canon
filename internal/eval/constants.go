package eval

// DefaultBudget is the step budget when project.budget is not set (EVALUATION.md §12.2).
const DefaultBudget int64 = 100_000_000

// maxDepth is the most live frames: calls, lambdas, field defaults and where runs (EVALUATION.md §3.3, DECISIONS 195).
const maxDepth = 10_000

// cancelEvery is how many steps pass between two looks at the context.
const cancelEvery = 1 << 12

// The states of a top-level value (EVALUATION.md §3.1, §7.2).
const (
	idle status = iota
	forcing
	done
	poisoned
)

// What a binder does to the refs it walks: bind, rebind to its instance's copy, unbind (refs.go).
const (
	modeBind bindMode = iota
	modeRebind
	modeUnbind
)

// How a statement completes.
const (
	flowNext flow = iota
	flowBreak
	flowContinue
	flowReturn
	flowAbort
)

// The punctuation of value paths (API.md §6.5), stack frames and qualified names.
const (
	dot          = "."
	keyOpen      = "["
	keyClose     = "]"
	callOpen     = "("
	callClose    = ")"
	argSep       = ", "
	quote        = `"`
	tripleQuote  = `"""`
	rawPrefix    = "r"
	itWord       = "it"
	checkKeyword = "check"
)

// operandText is the most bytes of operand text a failed expect comparison shows (EVALUATION.md §10.3).
const operandText = 4 << 10

// The shape of a code in `expect v fails E3204` (EVALUATION.md §10.3).
const codePattern = `^[EW][0-9]{4}$`

// The outcomes of an expect statement (EVALUATION.md §10.3).
const (
	outcomePasses = "passes"
	outcomeFails  = "fails"
	outcomeWarns  = "warns"
)

// float32Max is the smallest magnitude that overflows binary32 when stored (TYPES.md §7.3).
const float32Max = 3.4028235677973366e+38

// boolIndex is the match index of true; false is 0 (check.MatchInfo).
const boolIndex = 1

// The statements of a check block (STDLIB.md §10) and the name of their message parameter.
const (
	failStmt     = "fail"
	warnStmt     = "warn"
	messageParam = "message"
)

// The built-in members of STDLIB.md §3.
const (
	memberID      = "id"
	memberRetired = "retired"
	memberKind    = "kind"
	memberName    = "name"
	memberWire    = "wire"
	memberIndex   = "index"
	memberStart   = "start"
	memberMembers = "members" // E.members, on an enum type (DECISIONS 299)
	keyMagic      = "key"
)

// Numbers: the parts of a float literal, two names bound to a pair, the size from which a
// keyed collection is looked up through an index.
const (
	exponent    = "e"
	decimalBase = 10
	floatBits   = 64
	pairNames   = 2
	indexFrom   = 16
)

// The text of an internal error's parts, and of a named Go error.
const (
	fmtWrap       = "%w: %s"
	fmtWhere      = "%s at %s:%d:%d"
	fmtEvaluating = "evaluating %s.%s"
	fmtMetIn      = "met in %s"
	pathSep       = "/"
)

// The limits that can cut a vector's evaluation short (CONFORMANCE.md §6.5).
const (
	NoLimit Limit = iota
	StepLimit
	DepthLimit
)

// How a dependent value fits a computed type: converted, not at all, or a bare case to default.
const (
	Fits Fit = iota
	NoFit
	BareCase
)

// eqMemoFrom is the composite pairs an equality walk visits before it remembers the pairs it took (value.EqualUpTo's).
const eqMemoFrom = 1024

// vectorCap is a conformance vector's own step cap when VectorMode.Steps is 0 (CONFORMANCE.md §6.5).
const vectorCap int64 = 1_000_000

// maxSafeInt is the largest integer TypeScript holds exactly, ±(2^53 − 1) (CONFORMANCE.md §4).
const maxSafeInt = 1<<53 - 1

// A memo's bounds on one epoch's entries and their approximate bytes (the peak RSS target); the
// bytes counted per value, read or finding an entry keeps, as retained on the benchmark project;
// the separator of the layers in its keys.
const (
	memoCap       = 1 << 20
	memoBytes     = 256 << 20
	memoNodeBytes = 170
	layerSep      = ","
)

// The later stages keeping what they made of an entry on its evaluation: B and C (memo_stage.go).
const (
	Verified Stage = iota
	Checked
	stageCount int = iota
)

// A memo's stores (one per epoch, so per lineage), their bytes together, the forgotten epochs kept.
const (
	memoEpochs    = 4
	memoAllBytes  = 2 * memoBytes
	memoForgotten = 1024
)

// The outcomes of a replay's step, then the tags a fingerprint writes (memo_replay.go, memo_fp.go).
const (
	replayOn replayOutcome = iota
	replayStop
	replayMiss
)

const (
	fpNil = iota
	fpBack
	fpNone
)

// The kinds of node a kept graph copies, each kind allocated at once on a replay (memo_copy.go).
const (
	kindRecord nodeKind = iota
	kindList
	kindMap
	kindTable
	kindPair
	kindRef
	kindCount int = iota
)

// nilSlot is the slot of no value, which every table of copies holds first (memo_graph.go).
const nilSlot slot = 0

// pairParts is how many parts a pair has in a kept graph: A, then B.
const pairParts = 2
