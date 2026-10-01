package convert

import "errors"

// Usage errors of `canon convert --adopt` with a list `out` (CLI.md §3.10 step 4, DECISIONS 269).
var (
	// ErrAdoptRoot: the adopted path would share an owning root with an entry (E8009 `outRoot`).
	ErrAdoptRoot = errors.New("the adopted file would share an owning root with an out entry")
	// ErrAdoptForm: the entries of the list are directories (E8009 `outForm`).
	ErrAdoptForm = errors.New("the out entries are directories, the adopted file is a file")
	// ErrAdoptPath: a path does not resolve against the declared roots (WIRE.md §2.2).
	ErrAdoptPath = errors.New("a path does not resolve")
)
