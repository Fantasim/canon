package ir

import (
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// BoundSideText is T19's side of a bound in lower case: "lower bound", "upper bound" or "exclusive upper bound" (CODEGEN.md §5.15).
func BoundSideText(s BoundSide) string { return boundSideTexts[s] }

// BoundSideSentence is BoundSideText with its first letter upper case, for a sentence that starts with it.
func BoundSideSentence(s BoundSide) string {
	w := boundSideTexts[s]
	return strings.ToUpper(w[:1]) + w[1:]
}

// BoundSubject is T19's subject, the same bytes in every target: the field's Canon name, each step of the bound's path before it (the deepest first), a length's prefix before all (CODEGEN.md §5.15).
func BoundSubject(f *Field, b BoundConst) string {
	subject := f.Name
	for _, step := range b.Path {
		subject = boundStepTexts[step] + subject
	}
	switch {
	case b.Length && b.Level.Kind == types.String:
		return boundBytesText + subject
	case b.Length:
		return boundCountText + subject
	}
	return subject
}
