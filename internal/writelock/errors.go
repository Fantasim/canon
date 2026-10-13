package writelock

import "errors"

var (
	// ErrBadPackage is a package name that names no plain file of .canon/writers.
	ErrBadPackage = errors.New("not a package name a writer lock can have")
	// ErrUnsafe is a lock path through a link or a special file, never written through.
	ErrUnsafe = errors.New("the writer lock is not a regular file in a regular directory")

	errBusy = errors.New("held by another writer")
)
