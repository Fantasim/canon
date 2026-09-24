package conform

import "errors"

var (
	// ErrNoDecl is a translated fn of the IR whose declaration the checked program lacks.
	ErrNoDecl = errors.New("conform: translated fn has no declaration")
	// ErrNoBag is a package given without a findings bag.
	ErrNoBag = errors.New("conform: package has no findings bag")
	// ErrUnsupported is an internal error: a read of self with no candidate rule, which ir refuses at stage E (DECISIONS 204).
	ErrUnsupported = errors.New("conform: read of self has no conformance values")
	// errFailedRead is a precomputed method of self that failed on a test call's receiver: the call gives no vector (meta/decisions/log-2026-09-24.md, conform N1).
	errFailedRead = errors.New("conform: precomputed method of self failed on the receiver")
	// errPoisonedRead is an evaluation with no outcome after an error was reported: it read a poisoned value, so its call or vector is skipped (meta/decisions/log-2026-09-24.md, build wiring review).
	errPoisonedRead = errors.New("conform: evaluation read a poisoned value")
	// ErrNoOutcome is an internal error: an evaluation with no value, code or limit, as a poisoned read gives (DECISIONS 204).
	ErrNoOutcome = errors.New("conform: evaluation has no outcome")
)
