package check

// The kinds of IMPLEMENTATION-PLAN §4.7, each printed as the plan names it; values in constants.go.
type (
	ObjKind    uint8
	SelKind    uint8
	ConvKind   uint8
	CalleeKind uint8
	LitKind    uint8
)

func (k ObjKind) String() string { return nameOf(objNames[:], k) }

func (k SelKind) String() string { return nameOf(selNames[:], k) }

func (k ConvKind) String() string { return nameOf(convNames[:], k) }

func (k CalleeKind) String() string { return nameOf(calleeNames[:], k) }

func (k LitKind) String() string { return nameOf(litNames[:], k) }

func nameOf[K ~uint8](names []string, k K) string {
	if int(k) >= len(names) {
		return ""
	}
	return names[k]
}
