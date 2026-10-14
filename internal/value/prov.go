package value

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// ProvKind is where a value comes from (EVL-07; API.md OriginKind).
type ProvKind uint8

// Prov is the provenance of a value (EVL-07). Via is the omitting literal of a default or the
// copied value's provenance; the rarely set fields sit behind Ext, nil in the common case. A
// Prov is never written once a value holds it, so values born at one site may share it (DECISIONS 373).
type Prov struct {
	Kind ProvKind
	Span source.Span
	Via  *Prov
	Ext  *ProvExt
}

// ProvExt is a provenance's rarely set fields (DECISIONS 373). Stack holds diag.MaxStackFrames
// frames, innermost first, and MoreFrames counts those cut. Like its Prov, it is never written
// once attached.
type ProvExt struct {
	Pointer    string
	Layer      string
	Stack      []diag.Frame
	MoreFrames int
}

// NewProvExt is x on the heap, nil when x sets nothing (DECISIONS 373).
func NewProvExt(x ProvExt) *ProvExt {
	if x.Pointer == "" && x.Layer == "" && len(x.Stack) == 0 && x.MoreFrames == 0 {
		return nil
	}
	return &x
}

// has reports whether p has rarely set fields.
func (p *Prov) has() bool { return p != nil && p.Ext != nil }

// Pointer is the RFC 6901 pointer of a loaded JSON value, else "".
func (p *Prov) Pointer() string {
	if !p.has() {
		return ""
	}
	return p.Ext.Pointer
}

// Layer is the layer that set the value, else "".
func (p *Prov) Layer() string {
	if !p.has() {
		return ""
	}
	return p.Ext.Layer
}

// Stack is the call stack of a computed value, innermost first (EVALUATION.md §13).
func (p *Prov) Stack() []diag.Frame {
	if !p.has() {
		return nil
	}
	return p.Ext.Stack
}

// MoreFrames is the count of frames cut from Stack.
func (p *Prov) MoreFrames() int {
	if !p.has() {
		return 0
	}
	return p.Ext.MoreFrames
}
