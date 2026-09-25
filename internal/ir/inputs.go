package ir

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/types"
)

// InputReason is why LoadInputs refuses one variable: one failure line `<ENV>: <reason>`, the same bytes in every target (CODEGEN.md §5.12; log-2026-09-24 "W2 runtime inputs", "W2 gen/cpp inputs review").
type InputReason uint8

// InputReasonText is r's reason for an input field of type t: the Kind of `not a valid <Kind>` (a Float32 reads Float), the Canon enum of `not a member of <Enum>`; "" for a type no input has.
func InputReasonText(r InputReason, t TypeRef) string {
	switch r {
	case InputNotValid:
		if kind, ok := inputKinds[t.Kind]; ok {
			return fmt.Sprintf(inputReasons[r], kind)
		}
		return ""
	case InputNotMember:
		if e, ok := t.Named.(*Enum); ok && t.Kind == types.Enum {
			return fmt.Sprintf(inputReasons[r], e.Name)
		}
		return ""
	case InputNotSet, InputOutsideRange, InputNoMatch:
		return inputReasons[r]
	}
	return ""
}

// InputLine is one failure line of LoadInputs, `<ENV>: <reason>` (CODEGEN.md §5.12, §6.3 rt.InputError).
func InputLine(env, reason string) string { return env + inputLineSep + reason }
