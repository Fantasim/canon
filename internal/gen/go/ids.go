package gogen

import (
	"fmt"
	"strconv"
	"strings"
)

// writeID writes a baked table's opaque id: a struct holding the entry's position, a var per key, and its methods (CODEGEN.md §5.3, DECISIONS 379).
func (g *gen) writeID(s *enumSpec) {
	g.printf(idDocFormat, s.name, s.value)
	g.printf(idHeaderFormat, s.name, s.under)
	if len(s.consts) > 0 {
		g.printf(inputVarBlockOpen)
		for _, c := range s.consts {
			g.body.WriteString(docFor(c.name, c.doc))
			g.printf(idVarFormat, c.name, s.name, c.value)
		}
		g.printf(closeParenFormat)
	}
	var cases strings.Builder
	for i, c := range s.consts {
		fmt.Fprintf(&cases, idCaseFormat, c.value, strconv.Quote(s.names[i]))
	}
	g.printf(idFormat, s.name, cases.String(), g.use(strconvPkg, strconvPkg), s.value, g.names.ParseName(s.name), g.use(errorsPkg, errorsPkg))
	g.writeParse(s)
}
