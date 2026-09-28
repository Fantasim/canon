package viewgen

import (
	"fmt"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// buildNumber validates n (MarshalJSON, then WIRE.md 7.2's canonical form, quoted exactly
// past the safe range, VIEWMODEL.md J10) and builds it; Write never rewrites it.
func buildNumber(n vm.Number) (*jsonsrc.Node, error) {
	if _, err := n.MarshalJSON(); err != nil {
		return nil, err
	}
	if err := canonicalNumber(n.Text, n.Quoted); err != nil {
		return nil, err
	}
	if n.Quoted {
		return &jsonsrc.Node{Kind: jsonsrc.String, Text: n.Text}, nil
	}
	return &jsonsrc.Node{Kind: jsonsrc.Number, Text: n.Text}, nil
}

// buildScalarValue validates s (MarshalJSON, wrapping vm.ErrScalar; unquoted, a canonical
// integer inside J10's safe range — a Scalar is never a Float) and builds it.
func buildScalarValue(s vm.Scalar) (*jsonsrc.Node, error) {
	if _, err := s.MarshalJSON(); err != nil {
		return nil, err
	}
	if s.Quoted {
		return &jsonsrc.Node{Kind: jsonsrc.String, Text: s.Text}, nil
	}
	if err := canonicalIntegerText(s.Text); err != nil {
		return nil, err
	}
	if !withinSafeRange(s.Text) {
		return nil, fmt.Errorf("%w: unquoted beyond the safe range: %s", errNumberText, s.Text)
	}
	return &jsonsrc.Node{Kind: jsonsrc.Number, Text: s.Text}, nil
}

// buildTextRef builds a vm.TextRef: its key as a string, or `{"text": "…"}` for the
// language-neutral form (VIEWMODEL.md J9). A TextRef with both or neither refuses.
func buildTextRef(r vm.TextRef) (*jsonsrc.Node, error) {
	if (r.Key == "") == (r.Text == nil) {
		return nil, fmt.Errorf("%w: %q", errTextRef, r.Key)
	}
	if r.Text == nil {
		return &jsonsrc.Node{Kind: jsonsrc.String, Text: r.Key}, nil
	}
	text := &jsonsrc.Node{Kind: jsonsrc.String, Text: *r.Text}
	return &jsonsrc.Node{Kind: jsonsrc.Object, Members: []jsonsrc.Member{{Key: memberText, Value: text}}}, nil
}
