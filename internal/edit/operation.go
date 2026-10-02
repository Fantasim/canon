package edit

// Operation is one operation of an edit: each constructor of the API's Op fills the same
// fields, so the two convert one to one. Path is as given, parsed when the operation applies (E21).
type Operation struct {
	Kind  Op
	Path  string // a value path; RenameName's name (API.md E27)
	Value Lit    // Set, Add, Insert, AddEntry; SetCase's fields, or nil
	Key   Lit    // AddEntry's key, Rename's new key
	Index int    // Insert, Move
	Case  string // SetCase
	Name  string // RenameName's new name (API.md E30)
}
