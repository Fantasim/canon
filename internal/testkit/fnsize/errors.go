package fnsize

import "errors"

var (
	errParse        = errors.New("parse the Go source")
	errUnterminated = errors.New("unterminated comment, string or template")
	errUnbalanced   = errors.New("unbalanced brackets")
	errUnclassified = errors.New("cannot classify the declaration")
)
