package edit

// ChangeKind says what an edit does to a file (API.md §8.1).
type ChangeKind uint8

// Change is one file an edit writes, as the API's FileChange reports it: Path is a display
// path, OldPath the path a renamed file had; Before is nil for a created file, After for a
// deleted one.
type Change struct {
	Kind    ChangeKind
	Path    string
	OldPath string
	Before  []byte
	After   []byte
}
