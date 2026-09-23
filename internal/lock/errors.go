package lock

import "errors"

var (
	// ErrBadFact is a fact Add refuses: canon.lock could not hold it as written (LOCK.md §2.2).
	ErrBadFact = errors.New("invalid lock fact")

	errKind         = errors.New("unknown kind")
	errName         = errors.New("name is not <package>.<identifier> of the lock's package")
	errField        = errors.New("a field fact names one identifier field, other kinds none")
	errTableValue   = errors.New("a table fact has no value")
	errEnumString   = errors.New("an enum code is an integer")
	errString       = errors.New("value is neither an integer nor a UTF-8 string")
	errHolder       = errors.New("holder is not an identifier")
	errFieldRetired = errors.New("a field fact is not retired: its table fact is")
)
