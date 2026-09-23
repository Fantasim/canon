package wire

import "errors"

// Encode refuses what verification refuses (E8102, E3317, E8151) and a value unlike its type.
var (
	ErrNoWire       = errors.New("wire: no JSON encoding for this kind of value")
	ErrShape        = errors.New("wire: value shape differs from its declared type")
	ErrNotWholeUnit = errors.New("wire: duration is a fraction of its field's unit")
	ErrNoneMarker   = errors.New("wire: value encodes as its field's none marker")
	ErrBitsRepeated = errors.New("wire: member repeated in a bits list")
	ErrKeyCollision = errors.New("wire: two keys encode to the same text")
	ErrSchema       = errors.New("wire: $schema is not a generated-file marker")
)
