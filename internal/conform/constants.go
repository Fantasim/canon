package conform

import "github.com/fantasim/canonlang/internal/types"

// The limits that can cut a vector's evaluation short (CONFORMANCE.md §6.5).
const (
	NoLimit Limit = iota
	StepLimit
	DepthLimit
)

const (
	// stepCap is each vector's own step budget (CONFORMANCE.md §6.5).
	stepCap int64 = 1_000_000
	// maxProduct is the largest cartesian product kept whole (CONFORMANCE.md §6.3).
	maxProduct = 256
	// half and huge are the fractional and large float candidates of CONFORMANCE.md §6.2.
	half = 0.5
	huge = 1e300
)

// toLower and toUpper are the directions math.Nextafter steps a Float bound (DECISIONS 204).
const (
	toLower = -1
	toUpper = 1
)

// floatSeeds are the Float candidates of every parameter but -0.0, added apart (CONFORMANCE.md §6.2).
var floatSeeds = [...]float64{0, 1, -1, half, -half, huge, -huge}

// basicRules are the candidates of each scalar kind a translated parameter may have (CONFORMANCE.md §6.2).
var basicRules = map[types.Kind]candidateRule{
	types.Int: intCandidates, types.Duration: intCandidates, types.Float: floatCandidates,
	types.Bool: boolCandidates, types.String: stringCandidates,
}

// intSeeds are the integer and Duration candidates of every parameter besides its type range.
var intSeeds = [...]int64{0, 1, -1}

// The key of a value in a deduplicating set: a kind prefix, the value, then keyEnd.
const (
	keyInt    = "i"
	keyDur    = "d"
	keyFloat  = "f"
	keyStr    = "s"
	keyBool   = "b"
	keyMember = "m"
	keyCase   = "c"
	keyNone   = "n"
	keyLen    = ":"
	keyEnd    = ";"
	hexBase   = 16
	decBase   = 10
)

// ownerSep joins a variant and its case in an owner name, and an owner and its fn in a label.
const ownerSep = "."

const (
	fmtCanceled = "conform: %w"
	fmtNamed    = "%w: %s"
)
