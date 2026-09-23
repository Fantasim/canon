package diag

import (
	"cmp"
	"slices"

	"github.com/fantasim/canonlang/internal/source"
)

// Located is a finding with every span resolved: what the renderers write and what the API
// converts to and from canon.Finding, so both print through one writer. A location whose Path
// is "" is no location.
type Located struct {
	Code       Code
	Severity   Severity
	Loc        source.Location
	Pointer    string
	Package    string
	Path       string
	Message    string
	Check      string
	Layer      string
	Related    []RelatedLoc
	Stack      []FrameLoc
	MoreFrames int
	Reads      []string
}

// RelatedLoc is a resolved related location with its note (API.md F12).
type RelatedLoc struct {
	Loc  source.Location
	Note string
}

// FrameLoc is a resolved frame of a Canon call stack (API.md F13).
type FrameLoc struct {
	Fn  string
	Loc source.Location
}

// Locate resolves the spans of findings with files; the order is kept.
func Locate(files Files, findings []Finding) []Located {
	out := make([]Located, len(findings))
	for i, f := range findings {
		out[i] = locate(files, f)
	}
	return out
}

func locate(files Files, f Finding) Located {
	l := Located{
		Code: f.Code, Severity: f.Severity, Loc: locOf(files, f.Span),
		Pointer: f.Pointer, Package: f.Package, Path: f.Path, Message: f.Message,
		Check: f.Check, Layer: f.Layer, MoreFrames: f.MoreFrames, Reads: slices.Clone(f.Reads),
	}
	for _, r := range f.Related {
		l.Related = append(l.Related, RelatedLoc{Loc: locOf(files, r.Span), Note: r.Note})
	}
	for _, fr := range f.Stack {
		l.Stack = append(l.Stack, FrameLoc{Fn: fr.Fn, Loc: locOf(files, fr.Span)})
	}
	return l
}

// locOf resolves a span; a span in no file is the zero location.
func locOf(files Files, s source.Span) source.Location {
	path := files.Path(s.File)
	if path == "" {
		return source.Location{}
	}
	l := source.Location{Path: path}
	l.Line, l.Col = files.Position(s.File, s.Start)
	l.EndLine, l.EndCol = files.Position(s.File, s.End)
	return l
}

// compareLocated is a total order: the F2 key (file bytes, line, column, code, message; no
// file first), then every other field, so a sort's result never depends on its input order.
func compareLocated(a, b *Located) int {
	return cmp.Or(
		cmp.Compare(a.Loc.Path, b.Loc.Path),
		cmp.Compare(a.Loc.Line, b.Loc.Line),
		cmp.Compare(a.Loc.Col, b.Loc.Col),
		cmp.Compare(a.Code, b.Code),
		cmp.Compare(a.Message, b.Message),
		cmp.Compare(a.Severity, b.Severity),
		compareLoc(a.Loc, b.Loc),
		cmp.Compare(a.Package, b.Package),
		cmp.Compare(a.Path, b.Path),
		cmp.Compare(a.Pointer, b.Pointer),
		cmp.Compare(a.Check, b.Check),
		cmp.Compare(a.Layer, b.Layer),
		slices.CompareFunc(a.Related, b.Related, compareRelated),
		slices.CompareFunc(a.Stack, b.Stack, compareFrame),
		cmp.Compare(a.MoreFrames, b.MoreFrames),
		slices.Compare(a.Reads, b.Reads),
	)
}

func compareLoc(a, b source.Location) int {
	return cmp.Or(
		cmp.Compare(a.Path, b.Path),
		cmp.Compare(a.Line, b.Line),
		cmp.Compare(a.Col, b.Col),
		cmp.Compare(a.EndLine, b.EndLine),
		cmp.Compare(a.EndCol, b.EndCol),
	)
}

func compareRelated(a, b RelatedLoc) int {
	return cmp.Or(compareLoc(a.Loc, b.Loc), cmp.Compare(a.Note, b.Note))
}

func compareFrame(a, b FrameLoc) int {
	return cmp.Or(cmp.Compare(a.Fn, b.Fn), compareLoc(a.Loc, b.Loc))
}

// duplicate tells two findings reported once (EVALUATION.md §14).
func duplicate(a, b *Located) bool {
	return a.Severity == b.Severity && a.Code == b.Code && a.Loc.Path == b.Loc.Path &&
		a.Loc.Line == b.Loc.Line && a.Loc.Col == b.Loc.Col && a.Message == b.Message
}

// sortLocated sorts a copy of findings in the total order.
func sortLocated(findings []Located) []Located {
	out := slices.Clone(findings)
	slices.SortFunc(out, func(a, b Located) int { return compareLocated(&a, &b) })
	return out
}
