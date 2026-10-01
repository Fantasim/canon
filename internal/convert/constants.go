package convert

const (
	layoutAnchor = "." // a stand-in project directory: paths are compared project-relative (WIRE.md §2.2)
	// fmtAdoptPath and fmtAdoptRoot wrap the usage errors with the paths they name.
	fmtAdoptPath = "%w: %s"
	fmtAdoptRoot = "%w: %s and %s, under %s"
)
