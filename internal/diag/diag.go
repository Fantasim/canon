package diag

import "github.com/fantasim/canonlang/internal/source"

// Finding is one reported diagnostic (API.md §4.1); only Builder.Report makes one.
type Finding struct {
	Code       Code
	Severity   Severity
	Span       source.Span
	Pointer    string
	Package    string
	Path       string // API.md F1
	Message    string
	Check      string
	Layer      string
	Related    []Related
	Stack      []Frame
	MoreFrames int // API.md F13
	Reads      []string

	def     *Def // the code and variant whose template made Message, for Restated
	variant int
}

// Related is a location the value was checked against, with its rendered note (API.md F12).
type Related struct {
	Span source.Span
	Note string
}

// Frame is one frame of a Canon call stack; value provenance uses it too.
type Frame struct {
	Fn   string
	Span source.Span
}

// Files resolves spans: a file whose Path is "" is no location (API.md §1.3).
type Files interface {
	Path(id source.FileID) string
	Position(id source.FileID, p source.Pos) (line, col int)
	Content(id source.FileID) []byte
}

// Note is a related location's note (ERRORS.md §1.5), made by NoteSource or NoteCheck.
type Note struct {
	decl     source.Span
	name     string
	template string
}

// NoteSource is the note `source`: the text of the declaration the value was checked against.
func NoteSource(decl source.Span) Note {
	return Note{decl: decl}
}

// NoteCheck is the note `check`, or `checkUnnamed` when name is empty (API.md F4).
func NoteCheck(name string) Note {
	if name == "" {
		return Note{template: checkWord}
	}
	return Note{name: name, template: noteCheckTemplate}
}
