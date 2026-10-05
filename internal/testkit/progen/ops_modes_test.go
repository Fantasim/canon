package progen_test

import "github.com/fantasim/canonlang/internal/syntax"

// textOr is the source of n, "" for none.
func textOr(tg target, n syntax.Node) string {
	if n == nil {
		return ""
	}
	return text(tg, n)
}

func annotated(as []*syntax.Annotation, name string) bool {
	for _, a := range as {
		if a.Name != nil && a.Name.Name == name {
			return true
		}
	}
	return false
}
