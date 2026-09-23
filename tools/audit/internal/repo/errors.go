package repo

import "errors"

// errGitCommand wraps git's own stderr text, whatever that text is.
var errGitCommand = errors.New("git command failed")
