package cli

import "errors"

// Usage errors (CLI.md §2.5, exit 2); each message names its argument before the sentinel.
var (
	errNoCommand      = errors.New("no command given")
	errUnknownCommand = errors.New("unknown command")
	errNoArgs         = errors.New("takes no arguments")
	errOneArg         = errors.New("takes exactly one argument")
	errBadRoot        = errors.New("want name=dir")
	errRootTwice      = errors.New("a root is given twice")
	errBadFormat      = errors.New("want --format text or json")
	errBadMax         = errors.New("want a count of 0 or more")
	errBadName        = errors.New("not an identifier; give the project name with --name")
	errBadPackage     = errors.New("each segment must be a lowerCamel identifier")
	errExists         = errors.New("already exists")
)
