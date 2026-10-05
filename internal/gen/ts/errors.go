package tsgen

import (
	"errors"
	"fmt"
)

var (
	// ErrTarget is an emit whose target is not ts.
	ErrTarget = errors.New("tsgen: not a ts emit")
	// errUnsupported is a construct this generator does not emit.
	errUnsupported = errors.New("tsgen: not supported")
	// ErrMalformed is an IR that stage E should not have produced.
	ErrMalformed = errors.New("tsgen: malformed IR")
	// errCollision is two generated names equal in the module: stage E reports E8005 first (malformed IR).
	errCollision = fmt.Errorf("%w: generated name collision", ErrMalformed)
)

// What this generator refuses or finds malformed.
const (
	unsupportedKind      = "the type kind %s of %s"
	unsupportedNoDisc    = "a dependent value whose discriminant the decoder does not hold (in a map, a literal union or a fn result, or read through a ref), at %s"
	malformedPairs       = "a pairs field whose record is not a two-field record, at %s"
	malformedNoElem      = "a list, optional or table without its element type, at %s"
	malformedNoKey       = "a map or ref without its key type, at %s"
	malformedValue       = "a %T where %s expects %s"
	malformedDecl        = "a record, variant or enum kind without its declaration, at %s"
	malformedKeyField    = "a keyed list without its key field, at %s"
	malformedNoImport    = "package %s is used but has no ts emit"
	malformedFn          = "translated fn %s without its body, plan or vectors"
	malformedVector      = "a vector of %s does not match its signature"
	malformedExpr        = "the expression %T in %s"
	malformedBranch      = "a dependent value of %s whose discriminant selects no branch"
	malformedDependent   = "a dependent type %s without its discriminant or branches"
	malformedNoDisc      = "a dependent field %s whose discriminant is not read from earlier required fields"
	malformedTableIDs    = "table %s has %d ids for %d entries"
	malformedTable       = "the table of %s is not a table of its record"
	collisionFormat      = "%s and %s are both %s"
	malformedNoneMark    = "the none marker %s, a non-empty object or array, at %s"
	malformedCase        = "case %d of a variant, at %s"
	malformedDataValue   = "data value %s that is not a table, a keyed list or a record"
	malformedDomain      = "a %T as a finite parameter value, at %s"
	malformedDomainType  = "a finite parameter that is not an enum, a Bool or a table ref, at %s"
	malformedFields      = "a record value of %d fields for a type of %d, at %s"
	malformedMarker      = "the none marker %s is not JSON, at %s"
	malformedMember      = "member %d of an enum, at %s"
	malformedMemberName  = "member %s of an enum, at %s"
	malformedNoIdent     = "a table row without its identity, at %s"
	malformedNoInstance  = "no precomputed result of %s for a receiver, at %s"
	malformedNoTable     = "no table of %s, at %s"
	malformedNoTag       = "variant %s without a tag"
	malformedNoValue     = "%s without its value, at %s"
	malformedNoWire      = "field %s without a wire path, at %s"
	malformedRead        = "a read of %s that the type does not hold, at %s"
	malformedVariant     = "a variant value that is no case, at %s"
	malformedVectorValue = "a vector value %T for a %s, at %s"
	unsupportedFinite    = "a finite parameter that is not an enum, a Bool or a ref into a table whose ids the file knows, at %s"
)
