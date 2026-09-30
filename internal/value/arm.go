package value

// ArmIndex is what a type-level match selects on: a member's or case's index, false 0, true 1 (TYPES.md §11.2).
func ArmIndex(v Value) (int, bool) {
	switch x := v.(type) {
	case *Member:
		return x.Index, true
	case *CaseKind:
		return x.Index, true
	case *Bool:
		if x.V {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}
