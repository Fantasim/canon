package lint

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
)

// authoringNotes reports W0004 (API.md L5): a field with a wire name starting with `_`, of a record or
// case that an output writes (never through a ref), naming the first output that reaches it.
func authoringNotes(p *pass, pkg *check.Package, bag *diag.Bag) {
	p.eachHolder(pkg, func(h holder) {
		origin, written := p.output.From(h.typ)
		if !written {
			return
		}
		for _, df := range h.declaredFields() {
			f := df.field
			if strings.HasPrefix(f.Wire, noteMark) && !f.Inline && f.Pairs == nil && f.Input == nil {
				diag.W0004.At(h.file.Span(df.decl.Name), f.Name, f.Wire, outputName(origin)).Report(bag)
			}
		}
	})
}

// outputName is the output o as a finding names it: an emit by its target and package, a @text fn by its package and name.
func outputName(o edit.Origin) string {
	if o.Text {
		return fmt.Sprintf(fmtOutputText, o.Pkg, o.Name)
	}
	return fmt.Sprintf(fmtOutputEmit, o.Name, o.Pkg)
}
