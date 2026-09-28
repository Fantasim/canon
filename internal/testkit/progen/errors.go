package progen

import "errors"

var (
	// errHeader is a kept counterexample whose header has an unknown key or a malformed value.
	errHeader = errors.New("progen: malformed counterexample header")
	// errSite is a mutation site whose edits cannot be applied to its file.
	errSite = errors.New("progen: invalid mutation site")
)
