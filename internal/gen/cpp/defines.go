package cppgen

import (
	_ "embed"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

//go:embed text/defines.txt
var definesText string

// definesFile is <last>.defines.gen.h, nil when no enum or constant has @cpp(defines:) (CODEGEN.md §7.9, DECISIONS 389, 390); ir validates the names and the types first.
func (g *gen) definesFile() []byte {
	var w writer
	for _, d := range ir.CppDefineLines(g.p) {
		w.linef(0, defineFormat, d.Name, g.defineValue(d))
	}
	if w.String() == "" {
		return nil
	}
	var out writer
	out.printf(definesText, g.p.Dir, w.String())
	return out.bytes()
}

// defineValue is a define's value: a member's code with @codes else its index, a constant's literal (CODEGEN.md §7.9).
func (g *gen) defineValue(d ir.CppDefine) string {
	if d.Const == nil {
		return defineInt(enumDefine(d.Enum, d.Member))
	}
	leave := g.enter(d.Const.Name)
	defer leave()
	switch x := d.Const.V.(type) {
	case *value.Int:
		return defineInt(x.V)
	case *value.Bool:
		return defineInt(boolInt(x.V))
	case *value.Str:
		return quote(x.V)
	case *value.Member:
		if e, ok := d.Const.Type.Named.(*ir.Enum); ok && x.Index >= 0 && x.Index < len(e.Members) {
			return defineInt(enumDefine(e, e.Members[x.Index]))
		}
	}
	g.malformed(fmt.Sprintf(valueFormat, d.Const.V), g.at) // stage E refuses any other type first (E8012 cppDefine)
	return cppInvalid
}

// enumDefine is a member's define value: its code with @codes, else its index.
func enumDefine(e *ir.Enum, m *ir.EnumMember) int64 {
	if e.Codes == nil {
		return int64(m.Index)
	}
	return m.Code
}

// boolInt is 1 for true, else 0.
func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// defineInt is a define's integer: decimal, and the smallest int64 as an expression, since the digits of its absolute value are no int64 literal and the defines header includes nothing (CODEGEN.md §7.9).
func defineInt(v int64) string {
	if v == math.MinInt64 {
		return intMinDefine
	}
	return strconv.FormatInt(v, decimalBase)
}

// macroSet is every macro a file of the emit can meet: the defines of the package and of the packages it reads (CODEGEN.md §7.9, DECISIONS 391), built once.
func (g *gen) macroSet() map[string]bool {
	if g.macros == nil {
		g.macros = map[string]bool{}
		for _, n := range ir.CppDefineNames(g.p) {
			g.macros[n] = true
		}
		for _, ref := range g.p.Imports {
			for _, n := range ref.CppDefines {
				g.macros[n] = true
			}
		}
	}
	return g.macros
}

// macrosIn are the macros the texts name, sorted and distinct (CODEGEN.md §7.9): one pass over each text, each identifier looked up in the set.
func macrosIn(set map[string]bool, texts ...string) []string {
	if len(set) == 0 {
		return nil
	}
	found := map[string]bool{}
	for _, text := range texts {
		for i := 0; i < len(text); {
			if !wordByte[text[i]] {
				i++
				continue
			}
			end := wordEnd(text, i)
			if word := text[i:end]; set[word] {
				found[word] = true
			}
			i = end
		}
	}
	return slices.Sorted(maps.Keys(found))
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

// namespaceBody writes text inside the namespace, then after it, wrapped against the macros the namespace, the text or the rest names (CODEGEN.md §7.9).
func (g *gen) namespaceBody(out *writer, ns, text, after string) {
	wrap := macrosIn(g.macroSet(), ns, text, after)
	for _, m := range wrap {
		out.printf(pushMacroFormat, m, m)
	}
	if len(wrap) > 0 {
		out.blank()
	}
	out.printf(namespaceOpenFormat, ns)
	out.blank()
	out.write(text)
	out.printf(namespaceCloseFormat, ns)
	out.write(after)
	if len(wrap) > 0 {
		out.blank()
	}
	for _, m := range wrap {
		out.printf(popMacroFormat, m)
	}
}
