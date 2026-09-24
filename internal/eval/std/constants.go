package std

// The receiver families of STDLIB.md §1.2, and the free functions.
const (
	famFree family = iota
	famList
	famKeyed
	famMap
	famString
	famRange
	famCount
)

// The arithmetic operators of TYPES.md §7.1.
const (
	OpAdd Op = iota
	OpSub
	OpMul
	OpDiv
	OpMod
)

// The built-in names of STDLIB.md.
const (
	bLen        = "len"
	bIsEmpty    = "isEmpty"
	bFirst      = "first"
	bLast       = "last"
	bContains   = "contains"
	bIndexOf    = "indexOf"
	bMap        = "map"
	bFilter     = "filter"
	bFlatMap    = "flatMap"
	bFlatten    = "flatten"
	bReverse    = "reverse"
	bSortBy     = "sortBy"
	bUnique     = "unique"
	bEnumerate  = "enumerate"
	bPairs      = "pairs"
	bZip        = "zip"
	bIntersect  = "intersect"
	bUnion      = "union"
	bDiff       = "diff"
	bGroupBy    = "groupBy"
	bToMap      = "toMap"
	bJoin       = "join"
	bAny        = "any"
	bAll        = "all"
	bCount      = "count"
	bIsUnique   = "isUnique"
	bSum        = "sum"
	bMin        = "min"
	bMax        = "max"
	bMinBy      = "minBy"
	bMaxBy      = "maxBy"
	bGet        = "get"
	bFind       = "find"
	bAt         = "at"
	bKeys       = "keys"
	bValues     = "values"
	bActive     = "active"
	bStartsWith = "startsWith"
	bEndsWith   = "endsWith"
	bSplit      = "split"
	bTrim       = "trim"
	bLower      = "lower"
	bUpper      = "upper"
	bReplace    = "replace"
	bMatches    = "matches"
	bInt        = "Int"
	bFloat      = "Float"
	bString     = "String"
	bAbs        = "abs"
	bClamp      = "clamp"
	bFloor      = "floor"
	bCeil       = "ceil"
	bRound      = "round"
	bSqrt       = "sqrt"
	bPow        = "pow"
	bReachable  = "reachable"
	bCycles     = "cycles"
	bTopoSort   = "topoSort"
)

// The parameter names of STDLIB.md, for arguments given by name.
const (
	pX     = "x"
	pY     = "y"
	pA     = "a"
	pB     = "b"
	pF     = "f"
	pI     = "i"
	pK     = "k"
	pS     = "s"
	pRe    = "re"
	pSep   = "sep"
	pPred  = "pred"
	pLo    = "lo"
	pHi    = "hi"
	pFrom  = "from"
	pNext  = "next"
	pXs    = "xs"
	pOther = "other"
	pKeyF  = "keyF"
	pValF  = "valF"
)

// The text of numbers (STDLIB.md §9.5) and ASCII case mapping (§7).
const (
	minus           = "-"
	plus            = "+"
	point           = "."
	comma           = ","
	zero            = "0"
	zeroDigits      = "0.,"
	exponent        = "e"
	asciiSpace      = " \t\n\v\f\r"
	decimalBase     = 10
	groupSize       = 3
	halfDenominator = 2
	upperA          = 'A'
	upperZ          = 'Z'
	lowerA          = 'a'
	lowerZ          = 'z'
	caseBit         = 0x20
)

// The positions of clamp's bounds.
const (
	argLo = 1
	argHi = 2
)

// The Int range as floats: Int(f) needs -2^63 <= f < 2^63 (EVALUATION.md §6.2).
const (
	minIntFloat = -9223372036854775808.0
	maxIntFloat = 9223372036854775808.0
)
