package canon_test

import (
	"errors"
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

func ExamplePathError() {
	err := error(&canon.PathError{Op: 2, Path: "farm.modelTypes[9]", Err: canon.ErrNoPath})
	fmt.Println(err)
	fmt.Println(errors.Is(err, canon.ErrNoPath), errors.Is(err, canon.ErrBadPath))
	// Output: op 2: farm.modelTypes[9]: no value at path
	// true false
}

func ExampleValueError() {
	err := error(&canon.ValueError{Op: 0, Path: "farm.modelTypes[3].maxLevel", Expected: "Int", Got: "String"})
	fmt.Println(err)
	fmt.Println(errors.Is(err, canon.ErrBadValue))
	// Output: op 0: farm.modelTypes[3].maxLevel: value does not fit the type: expected Int, got String
	// true
}

func ExampleNotEditableError() {
	err := error(&canon.NotEditableError{Op: 1, Path: "statuses.open.id", Reason: canon.ReasonPseudo, Detail: "use Rename"})
	fmt.Println(err)
	fmt.Println(errors.Is(err, canon.ErrNotEditable))
	// Output: op 1: statuses.open.id: value is not editable: pseudo: use Rename
	// true
}

func ExampleStaleError() {
	err := error(&canon.StaleError{Files: []string{"teamboard/taxonomy.canon"}})
	fmt.Println(err, errors.Is(err, canon.ErrStale))
	// Output: sources changed since the base revision: [teamboard/taxonomy.canon] true
}

func ExampleRejectedError() {
	err := error(&canon.RejectedError{Findings: make([]canon.Finding, 2)})
	fmt.Println(err, errors.Is(err, canon.ErrRejected))
	// Output: edit rejected: it produces errors: 2 error(s) true
}

func ExampleNotCanonicalError() {
	err := error(&canon.NotCanonicalError{Files: []string{"@resource/Server/farm.json"}})
	fmt.Println(err, errors.Is(err, canon.ErrNotCanonical))
	// Output: file is not in canonical layout: [@resource/Server/farm.json] true
}

func ExampleProjectError() {
	finding := canon.Finding{Severity: canon.SeverityError, Message: "root @resource does not exist"}
	err := error(&canon.ProjectError{Err: canon.ErrProject, Findings: []canon.Finding{finding}})
	fmt.Println(err, errors.Is(err, canon.ErrProject))
	// Output: invalid project: root @resource does not exist true
}

func ExampleSyntaxError() {
	at := canon.Span{File: "teamboard/taxonomy.canon", Line: 3, Col: 9}
	err := error(&canon.SyntaxError{Findings: []canon.Finding{{Span: at, Message: "expected }"}}})
	fmt.Println(err, errors.Is(err, canon.ErrSyntax))
	// Output: teamboard/taxonomy.canon:3:9: expected } true
}

func ExampleInternalError() {
	err := error(&canon.InternalError{Msg: "index out of range"})
	fmt.Println(err, errors.Is(err, canon.ErrInternal))
	// Output: internal compiler error: index out of range true
}
