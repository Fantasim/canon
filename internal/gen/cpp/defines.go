package cppgen

import (
	_ "embed"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

//go:embed text/defines.txt
var definesText string

// definesFile is <last>.defines.gen.h, nil when no enum has @cpp(defines:) (CODEGEN.md §7.9).
func (g *gen) definesFile() []byte {
	var w writer
	for _, e := range g.ownEnums() {
		for _, m := range e.Members {
			value := m.Code
			if e.Codes == nil {
				value = int64(m.Index)
			}
			w.linef(0, defineFormat, defineName(e.CppDefines, m), value)
		}
	}
	if w.String() == "" {
		return nil
	}
	var out writer
	out.printf(definesText, g.p.Dir, w.String())
	return out.bytes()
}

// ownEnums are the package's enums with @cpp(defines:), in declaration order.
func (g *gen) ownEnums() []*ir.Enum {
	var out []*ir.Enum
	for _, t := range g.p.Types {
		if e, ok := t.(*ir.Enum); ok && e.CppDefines != "" {
			out = append(out, e)
		}
	}
	return out
}

// defineName is a member's macro: its name when it has the prefix, else prefix + name.
func defineName(prefix string, m *ir.EnumMember) string {
	if strings.HasPrefix(m.Name, prefix) {
		return m.Name
	}
	return prefix + m.Name
}

// clashes are the macros spelled like an enumerator the text names (CODEGEN.md §7.9), each once, in enum then member order.
func (g *gen) clashes(text string) []string {
	var out []string
	var named map[string]bool
	seen := map[string]bool{}
	for _, e := range append(g.ownEnums(), g.foreignEnums...) {
		if e.CppDefines == "" {
			continue
		}
		if named == nil {
			named = qualifiedPairs(text)
		}
		_, enum := g.named(e)
		for _, m := range e.Members {
			macro := defineName(e.CppDefines, m)
			if macro == g.pl.Enumerator(m) && named[enum+scopeSep+macro] && !seen[macro] {
				seen[macro] = true
				out = append(out, macro)
			}
		}
	}
	return out
}

// qualifiedPairs is every `A::B` of whole ASCII words in text, the names `\bA::B\b` matches (CODEGEN.md §7.9), in one pass.
func qualifiedPairs(text string) map[string]bool {
	out := map[string]bool{}
	for i := 0; i < len(text); {
		if !wordByte[text[i]] {
			i++
			continue
		}
		end := wordEnd(text, i)
		if next := end + len(scopeSep); strings.HasPrefix(text[end:], scopeSep) && wordEnd(text, next) > next {
			out[text[i:wordEnd(text, next)]] = true
		}
		i = end
	}
	return out
}

// wordEnd is the index after the run of word bytes of text that starts at from.
func wordEnd(text string, from int) int {
	for from < len(text) && wordByte[text[from]] {
		from++
	}
	return from
}

// wordByte marks the bytes of a word, the class a regular expression's \b is made of.
var wordByte = func() (table [byteValues]bool) {
	for i := range len(asciiWord) {
		table[asciiWord[i]] = true
	}
	return table
}()

// namespaceBody writes text inside the namespace, wrapped against the macros it clashes with.
func (g *gen) namespaceBody(out *writer, ns, text string) {
	clash := g.clashes(text)
	for _, m := range clash {
		out.printf(pushMacroFormat, m, m)
	}
	if len(clash) > 0 {
		out.blank()
	}
	out.printf(namespaceOpenFormat, ns)
	out.blank()
	out.write(text)
	out.printf(namespaceCloseFormat, ns)
	if len(clash) > 0 {
		out.blank()
	}
	for _, m := range clash {
		out.printf(popMacroFormat, m)
	}
}

// noteEnum records an imported enum the code names, whose defines may clash too.
func (g *gen) noteEnum(t ir.Type) {
	if e, ok := t.(*ir.Enum); ok && e.Pkg != g.p.Name && !slices.Contains(g.foreignEnums, e) {
		g.foreignEnums = append(g.foreignEnums, e)
	}
}
