package comments

// LongDoc explains far more than a three-line limit allows, spilling into a fourth line
// and beyond, which is exactly the kind of verbose top-level doc comment-decl exists to
// catch so a reviewing agent does not have to read a paragraph before the signature.
// A fourth content line, citing ADR-0012: comment-decl reports it, comment-adr-narration does not.
func LongDoc() {}

const (
	// Grouped has a two-line comment on a single spec inside a parenthesized block,
	// over the one-line field-comment limit.
	Grouped = 1
)

// Boxed has one field whose comment is too long.
type Boxed struct {
	// Value carries the payload; this doc comment runs long enough on purpose to cross
	// the one-line limit FieldCommentLines sets for a struct field.
	Value int
}
