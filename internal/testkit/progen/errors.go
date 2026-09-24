package progen

import "errors"

var (
	// errLinkLoop is a symbolic link path that does not end.
	errLinkLoop = errors.New("progen: too many symbolic links")
	// errHeader is a kept counterexample whose header has an unknown key or a malformed value.
	errHeader = errors.New("progen: malformed counterexample header")
	// errSite is a mutation site whose edits cannot be applied to its file.
	errSite = errors.New("progen: invalid mutation site")
)
