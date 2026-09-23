package threshold

import "errors"

var (
	errRead      = errors.New("read thresholds")
	errMalformed = errors.New("row is not <key><TAB><value>")
	errUnknown   = errors.New("unknown threshold key")
	errDuplicate = errors.New("threshold key given twice")
	errValue     = errors.New("threshold value is not a positive decimal integer in range")
	errMissing   = errors.New("threshold key missing")
)
