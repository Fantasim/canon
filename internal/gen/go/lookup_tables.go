package gogen

import (
	"cmp"
	"math"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// lookupTables writes, for each enum the lookups of this package search the index of, its members sorted by code (an opened enum: by wire) and the declaration index of each, so a lookup finds a member by binary search and the function does not grow with the members (CODEGEN.md §5.10, DECISIONS 435). The first declared of equal keys comes first, so the search finds it.
func (g *gen) lookupTables() {
	for _, e := range g.names.LookupEnums() {
		tables, _ := g.names.LookupTables(e)
		members := make([]string, len(e.Members))
		for i, m := range e.Members {
			members[i] = g.qualify(e.Pkg, g.names.MemberName(e, m))
		}
		idx := orderedMembers(e, ir.OpenEnum(g.p, g.e, e))
		g.writeTable(tables.Keys, g.qualify(e.Pkg, g.goName(e)), pick(members, idx))
		g.writeTable(tables.At, smallestUint(len(e.Members)), indexLiterals(idx))
	}
}

// orderedMembers are the declaration indexes of e's members sorted by value in the order of its Go type: by wire for an opened enum, else by code, as unsigned when the codes' type is.
func orderedMembers(e *ir.Enum, open bool) []int {
	if open {
		return order(len(e.Members), func(a, b int) int { return strings.Compare(e.Members[a].Wire, e.Members[b].Wire) })
	}
	return order(len(e.Members), func(a, b int) int {
		flip := int64(0) // flipping the sign bit makes unsigned order of the codes' bits a signed one
		if !e.Codes.Signed {
			flip = math.MinInt64
		}
		return cmp.Compare(e.Members[a].Code^flip, e.Members[b].Code^flip)
	})
}
