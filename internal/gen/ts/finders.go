package tsgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// finders are the lookups of a container of rec's rows, one per @stable field: its type (an intersection with a `findBy<F>(value)` method each, the rows `row` stand for) and the `{ findBy<F>: "field" }` argument handed to canonTable, which builds an index each once, never a scan (CODEGEN.md §5.9, §8.2). Both are empty when rec has no @stable field.
func (g *gen) finders(rec *ir.Record, row string) (typ, readers string) {
	var sigs, reads []string
	seen := map[string]*ir.Field{}
	for _, f := range rec.Fields {
		if !f.Stable {
			continue
		}
		name := findByPrefix + upperCamel(f.Name)
		if first, dup := seen[name]; dup {
			g.failf(errCollision, collisionFormat, first.Name, f.Name, name)
			continue
		}
		seen[name] = f
		sigs = append(sigs, fmt.Sprintf(findSigFormat, name, g.bind(lowerCamel(f.Name)), g.tsType(f.Type, f.BigInt), row))
		reads = append(reads, fmt.Sprintf(findReadFormat, name, quote(fieldProp(f))))
	}
	if len(sigs) == 0 {
		return "", ""
	}
	return fmt.Sprintf(findersTypeFormat, strings.Join(sigs, findSep)), fmt.Sprintf(findersArgFormat, strings.Join(reads, listSep))
}
