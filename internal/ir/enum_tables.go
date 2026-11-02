package ir

import (
	"math"
	"slices"
)

// DenseCodes reports whether e has @codes, at least one member and each member's code is the previous plus one, never wrapping; first is then the first member's code. The C++ and Go generators and their name plans decide an enum's tables with it (CODEGEN.md §5.2, DECISIONS 430).
func DenseCodes(e *Enum) (first int64, ok bool) {
	if e.Codes == nil || len(e.Members) == 0 {
		return 0, false
	}
	prev := e.Members[0].Code
	for _, m := range e.Members[1:] {
		if prev == math.MaxInt64 || m.Code != prev+1 {
			return 0, false
		}
		prev = m.Code
	}
	return e.Members[0].Code, true
}

// EnumWiresAreNames reports whether no member's wire differs from its name, vacuously with no member: such an enum has no wires table, Wire reads the names (CODEGEN.md §5.2, DECISIONS 430).
func EnumWiresAreNames(e *Enum) bool {
	return !slices.ContainsFunc(e.Members, func(m *EnumMember) bool { return m.Name != m.Wire })
}

// CaseWiresAreNames reports whether no case's wire differs from its name, vacuously with no case: the same test for a variant's kind enum (CODEGEN.md §5.2, §5.5, DECISIONS 430).
func CaseWiresAreNames(v *Variant) bool {
	return !slices.ContainsFunc(v.Cases, func(c *Case) bool { return c.Name != c.Wire })
}
