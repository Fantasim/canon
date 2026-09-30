package winpaths

// Sep is Windows's separator.
const Sep = `\`

// Lengths and prefixes volumeNameLen reads, as Go's Windows filepath has them.
const (
	driveLen     = 2
	uncPrefixLen = 2
	uncSeps      = 2
	deviceOnly   = 3
	deviceSkip   = 4
	uncDevice    = `\\.\UNC`
	uncDeviceEnd = len(uncDevice) + 1
	devicePrefix = `\\.`
	rootDevice   = `\\?`
	ntPrefix     = `\??`
	slash        = "/"
	colon        = ':'
	backslashCh  = '\\'
)
