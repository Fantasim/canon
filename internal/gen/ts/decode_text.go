package tsgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
)

// textDecoders write `decode<Fn>File(json: unknown): T` and `parse<Fn>File(text: string): T` of each @text fn whose result a types-mode emit decodes: a map, a list or a keyed list read through this file's readers, deep-frozen, its errors naming <Fn>File behind a `$` path (CODEGEN.md §5.13, §8.1, DECISIONS 340, 363).
func (g *gen) textDecoders() []string {
	var out []string
	for _, fn := range ir.TextDecoded(g.p, g.e) {
		out = append(out, g.textDecoder(fn))
	}
	return out
}

// textDecoder is the two functions of one fn.
func (g *gen) textDecoder(fn *ir.ExportFn) string {
	defer g.enter(fn.Name)()
	n := ir.TSTextFunctions(fn)
	origin := g.p.Name + dot + fn.Name
	result := ir.TextResult(fn)
	decode, parse := g.declare(n.Decode, origin), g.declare(n.Parse, origin)
	typ := g.tsType(result, false)
	read := g.dec(result, jsonParam, quote(fmt.Sprintf(rootFormat, n.File)), decCtx{})
	g.use(decParseName)
	return docComment("", fn.Doc) + fmt.Sprintf(publicDecoderFormat, decode, typ, g.helper(canonFreezeName), read) + newline +
		fmt.Sprintf(parseFormat, parse, typ, decode, quote(n.File), quote(dollarKey))
}
