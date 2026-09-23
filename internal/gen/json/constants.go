package jsongen

// jsonExt ends a file-mode `out` and every directory-mode file name (WIRE.md §8.1).
const jsonExt = ".json"

// qnameSep joins a package and a name, as ir.Type.QName does.
const qnameSep = "."

// The wrapping of this package's errors.
const (
	fmtNamed   = "%w: %s"
	fmtContext = "%s: %w"
	fmtCount   = "%w: %q with %d values"
	fmtFile    = "%w: %s written to %s"
	fmtSchema  = "%w: %s, the file has %s"
)
