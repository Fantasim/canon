package diag

import "github.com/fantasim/canonlang/internal/source"

// Builder is a finding under construction; only a code's constructor makes one.
type Builder struct {
	def     *Def
	variant int
	span    source.Span
	args    []any
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
