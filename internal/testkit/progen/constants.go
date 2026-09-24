package progen

const (
	projectDir = "/p" // where Run places a project in its file system
	rootDir    = "/"
	currentDir = "."

	// splitmix64's constants.
	mixGamma  uint64 = 0x9e3779b97f4a7c15
	mixA      uint64 = 0xbf58476d1ce4e5b9
	mixB      uint64 = 0x94d049bb133111eb
	drawBytes        = 8 // a draw's bytes, read for its low bits

	findingFormat = "%s %s:%d:%d"
	panicFormat   = "panic: %v\n%s"

	// halves is ddmin's split factor.
	halves      = 2
	maxLinkHops = 40 // links EvalSymlinks follows before it calls the path a loop, as Linux does
	shrinkFile  = "shrink.canon"

	keySuite     = "suite"
	keyName      = "name"
	keyCase      = "case"
	keySeed      = "seed"
	keySig       = "sig"
	keyOpen      = "open"
	keyPackages  = "packages"
	keyLayers    = "layers"
	keyWant      = "want"
	keyNote      = "failure"
	keyLink      = "link" // one header line per symbolic link: its path, then its target
	fieldSep     = " "
	headerFormat = "%s %s\n"
	decimal      = 10
	seedBits     = 64
)

// headerKeys are the keys of a counterexample's header, in the order Format writes them.
var headerKeys = []string{keySuite, keyName, keyCase, keySeed, keyOpen, keySig, keyPackages, keyLayers, keyWant, keyNote}
