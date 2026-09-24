package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// capWord is Cap(w) (CODEGEN.md §3.2): the first character upper case, the rest lower case.
func capWord(w string) string {
	if w == "" {
		return w
	}
	return strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
}

// upperCamel is C++ UpperCamel(x) over the words of CODEGEN.md §3.1.
func upperCamel(name string) string {
	var b strings.Builder
	for _, w := range ir.Words(name) {
		b.WriteString(capWord(w))
	}
	return b.String()
}

// override is the @cpp(name:) of an item, or derived when there is none (CODEGEN.md §3.5).
func override(n ir.NameOptions, derived string) string {
	if n.Name != "" {
		return n.Name
	}
	return derived
}

// verbatim is a name as Canon spells it, with `_` added when C++ reserves it (CODEGEN.md §3.4).
func verbatim(name string) string {
	if ir.CppReserved(name) {
		return name + underscore
	}
	return name
}

// member is the storage `f_` of a field or stored fn; `f__` is reserved (CODEGEN.md §3.4, §7.2).
func (g *gen) member(name string) (string, error) {
	if strings.HasSuffix(name, underscore) || strings.Contains(name, reservedRun) {
		return "", fmt.Errorf("%w: storage of %s", errName, name)
	}
	return g.pl.Member(name), nil
}

// scope is one C++ scope of generated names (CODEGEN.md §3.5): a namespace, a class, an enum.
type scope struct {
	what string
	seen map[string]string
}

func newScope(what string) *scope {
	return &scope{what: what, seen: map[string]string{}}
}

// add declares name for origin; a repeat, or a name with `__` or a leading `_X`, is refused.
func (s *scope) add(name, origin string) error {
	if strings.Contains(name, reservedRun) || len(name) > 1 && name[0] == '_' && isUpper(name[1]) {
		return fmt.Errorf("%w: %s (from %s)", errName, name, origin)
	}
	if prev, ok := s.seen[name]; ok {
		return fmt.Errorf("%w: %s %s: %s, %s", errNameCollision, s.what, name, prev, origin)
	}
	s.seen[name] = origin
	return nil
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
