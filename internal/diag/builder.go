package diag

import (
	"slices"

	"github.com/fantasim/canonlang/internal/source"
)

// Builder is a finding under construction; only a code's constructor makes one.
type Builder struct {
	def        *Def
	variant    int
	span       source.Span
	args       []any
	path       string
	pointer    string
	check      string
	layer      string
	related    []relatedNote
	stack      []Frame
	stackCut   int // the frames Stack cut, past MaxStackFrames
	moreFrames int // the frames a provenance had already cut
	reads      []string
}

type relatedNote struct {
	span source.Span
	note Note
}

// newBuilder records a constructor call: its code, variant index, span and arguments.
func newBuilder(def *Def, variant int, span source.Span, args ...any) *Builder {
	return &Builder{def: def, variant: variant, span: span, args: args}
}

// Message is a finding kept as the argument of another code (E1703); only diag makes one.
type Message struct {
	finding *Builder
}

// Message keeps the finding as another code's argument instead of reporting it.
func (b *Builder) Message() Message {
	return Message{finding: b}
}

// Path sets the canonical value path the finding is about (API.md F1).
func (b *Builder) Path(p string) *Builder {
	b.path = p
	return b
}

// Pointer sets the RFC 6901 pointer of the value in a loaded JSON file.
func (b *Builder) Pointer(p string) *Builder {
	b.pointer = p
	return b
}

// Related adds a location the value was checked against, in order (API.md F12).
func (b *Builder) Related(span source.Span, n Note) *Builder {
	b.related = append(b.related, relatedNote{span: span, note: n})
	return b
}

// Check names the check or warn that produced the finding (API.md §4.1).
func (b *Builder) Check(name string) *Builder {
	b.check = name
	return b
}

// Layer names the layer whose amendment produced the value (API.md F11).
func (b *Builder) Layer(name string) *Builder {
	b.layer = name
	return b
}

// Stack sets the call stack, innermost first, cut to MaxStackFrames; a second call replaces
// the first, its cut frames included.
func (b *Builder) Stack(frames []Frame) *Builder {
	kept := min(len(frames), MaxStackFrames)
	b.stack = slices.Clone(frames[:kept])
	b.stackCut = len(frames) - kept
	return b
}

// MoreFrames sets the frames a provenance already cut from its stack; the finding counts them
// with the frames Stack cuts.
func (b *Builder) MoreFrames(n int) *Builder {
	b.moreFrames = max(n, 0)
	return b
}

// Reads sets the fields a one-line record check reads (VIEWMODEL.md J15).
func (b *Builder) Reads(fields []string) *Builder {
	b.reads = slices.Clone(fields)
	return b
}

// Report renders the message with the bag's files and adds the finding (ERRORS.md §2.2).
func (b *Builder) Report(bag *Bag) {
	bag.add(b.finding(bag.files, bag.pkg))
}

// finding renders the builder into the finding a bag of package pkg holds.
func (b *Builder) finding(files Files, pkg string) Finding {
	r := renderer{files: files}
	f := Finding{
		Code:       b.def.Code,
		Severity:   b.def.Severity,
		Span:       b.span,
		Pointer:    b.pointer,
		Package:    pkg,
		Path:       b.path,
		Message:    r.message(b),
		Check:      b.check,
		Layer:      b.layer,
		Stack:      b.stack,
		MoreFrames: b.stackCut + b.moreFrames,
		Reads:      b.reads,
	}
	for _, rel := range b.related {
		f.Related = append(f.Related, Related{Span: rel.span, Note: r.note(rel.note)})
	}
	return f
}
