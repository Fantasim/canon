package viewgen

import "errors"

// What Write refuses (go.md 3: Go errors are sentinels, never matched by text).
var (
	errNil        = errors.New("viewgen: nil view model")
	errType       = errors.New("viewgen: unsupported value")
	errMapKey     = errors.New("viewgen: non-string map key")
	errTextRef    = errors.New("viewgen: text reference has both or neither of key and text")
	errRaw        = errors.New("viewgen: malformed raw value")
	errNumberText = errors.New("viewgen: number text is not WIRE 7.2 canonical")
)
