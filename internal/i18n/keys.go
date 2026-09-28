package i18n

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// namedForm is the two ways a name could end a key (I18N.md K4): bare is valid unless name is
// a reserved segment, in which case kind (kindWord.name) is the only valid one and bare is the
// alternate, wrong, form a translator might write by mistake.
func namedForm(prefix, name, kindWord string) (key, altKey string) {
	bare := join(prefix, name)
	kind := join(prefix, kindWord, name)
	if syntax.IsReservedSegment(name) {
		return kind, bare
	}
	return bare, kind
}

// join concatenates key segments with dots, empty ones skipped.
func join(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, dot)
}
