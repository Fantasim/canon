package edit

import (
	"bytes"
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// typedNums is what a source path read as numbers in a JSON source: the bytes, and by span each
// number token's typed-canonical text (API.md M9; FORMATTER.md 14.1).
type typedNums struct {
	content []byte
	texts   map[source.Span]string
}

// oldEntry is the named entry's file as an old value: its bytes without a top-level `$retired`
// member, and whether that member retired it (WIRE.md 5.7, 6.5); the bytes as they are when they
// do not parse.
func (t *dirTable) oldEntry() ([]byte, bool, error) {
	raw, err := t.raw()
	if err != nil {
		return nil, false, err
	}
	doc, parsed := t.parsed()
	if !parsed {
		return raw, false, nil // bytes that do not parse: an old value held in error (carry)
	}
	mark, err := retiredMember()
	if err != nil {
		return nil, false, err
	}
	i := objMember(doc, mark.Key)
	if i < 0 {
		return raw, false, nil
	}
	rest := detached(doc)
	retired := rest.Members[i].Value.Text == mark.Value.Text
	rest.Members = slices.Delete(rest.Members, i, i+1)
	return jsonsrc.Format(rest), retired, nil
}

// carry records op carrying back old, or its value at node n, printed as the valued path prints it
// (API.md E23); none for a value an error finding inside it holds in error (E22; DECISIONS 273),
// the root's refusal for any other it cannot carry (E19). True when it carried.
func (t *dirTable) carry(op Operation, old []byte, ty types.Type, scope *types.Field, n *jsonsrc.Node) (bool, error) {
	typed, at := old, source.Span{End: source.Pos(math.MaxInt32)}
	if n != nil {
		typed, at = old[n.Span.Start:n.Span.End], source.Span{Start: n.Span.Start, End: n.Span.End}
	}
	v, ok := fitted(t.x, FromJSON(slices.Clone(typed)), ty, scope)
	switch {
	case ok:
		lit, err := t.x.a.sourceLit(v, ty, scope, stated{})
		if err != nil {
			return false, err
		}
		op.Value = lit
		t.x.inverse(op)
		return true, nil
	case t.x.a.snap.heldInError(t.x.res.root.pkg.Path, t.display(), at.Start, at.End):
		return false, nil
	}
	return false, t.cause
}

// parsed is the named entry's object as edited so far, false when its file states none.
func (t *dirTable) parsed() (*jsonsrc.Node, bool) {
	doc, err := t.x.a.jsonRoot(t.display())
	return doc, err == nil && doc.Kind == jsonsrc.Object
}

// noteNumbers records the typed-canonical text of each number doc's members state in raw, each
// decoded by the declared type of the field it is, as a load reads them (M9), so the layout check
// judges the file as it judges one whose root has a value.
func (t *dirTable) noteNumbers(doc *jsonsrc.Node, raw []byte) {
	texts := map[source.Span]string{}
	for _, f := range fieldsOf(t.tt.Elem) {
		i := objMember(doc, f.Wire)
		if i < 0 || f.Input != nil || f.Inline || f.Pairs != nil || len(f.WirePath) != 1 {
			continue
		}
		n := doc.Members[i].Value
		tok := raw[n.Span.Start:n.Span.End]
		if v, ok := fitted(t.x, FromJSON(slices.Clone(tok)), f.Type, f); ok {
			numberTexts(texts, v, tok, n.Span.Start)
		}
	}
	if t.x.a.numbers == nil {
		t.x.a.numbers = map[string]typedNums{}
	}
	t.x.a.numbers[t.display()] = typedNums{content: slices.Clone(raw), texts: texts}
}

// numberTexts adds to texts the typed-canonical text of each number token v and its parts state in
// raw, the bytes from offset at of their file (wire.NumberText).
func numberTexts(texts map[source.Span]string, v value.Value, raw []byte, at source.Pos) {
	stack := []value.Value{v}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = append(stack[:len(stack)-1], childValues(x)...)
		p := provOf(x)
		if p == nil || p.Kind != value.ProvJSON || int(p.Span.End) > len(raw) {
			continue
		}
		if text, ok := wire.NumberText(x, string(raw[p.Span.Start:p.Span.End])); ok {
			texts[source.Span{Start: at + p.Span.Start, End: at + p.Span.End}] = text
		}
	}
}

// numbersRead sets in root the typed-canonical number texts a source path read in the JSON
// source at display, when content is what it read (M9).
func (a *applier) numbersRead(display string, content []byte, root *jsonsrc.Node) {
	if nums, ok := a.numbers[display]; ok && bytes.Equal(nums.content, content) {
		setNumbers(root, nums.texts)
	}
}
