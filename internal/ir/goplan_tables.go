package ir

import "slices"

// TableName is the package-level table of role (GoTableNames...) of the enum, kind enum or id enum named enum (CODEGEN.md §5.2, §5.3, DECISIONS 393).
func (pl *GoNamePlan) TableName(role, enum string) string { return role + goTableSep + enum }

// declareTables declares the tables of roles for the type named enum.
func (pl *GoNamePlan) declareTables(top *nameScope, enum, origin string, item any, roles ...string) {
	for _, role := range roles {
		pl.declare(top, pl.TableName(role, enum), origin, item)
	}
}

// enumTableRoles are the tables gen/go writes for an enum: an opened one searches its members by wire and, with @codes, by code; a closed one reads names and wires by index (by code with @codes) and searches the keys.
func enumTableRoles(e *Enum, open bool) []string {
	switch {
	case open && e.Codes != nil:
		return []string{GoTableKeys, GoTableNames, GoTableNumbers, GoTableCodes, GoTableByCode, GoTableMembers}
	case open:
		return []string{GoTableKeys, GoTableNames, GoTableMembers}
	}
	roles := closedTableRoles(!slices.ContainsFunc(e.Members, func(m *EnumMember) bool { return m.Name != m.Wire }))
	if e.Codes != nil {
		roles = append(roles, GoTableCodes)
	}
	return append(roles, GoTableMembers)
}

// closedTableRoles are the tables of a closed enum or kind enum; one whose wires are all its names has no wires table, Wire reads the names.
func closedTableRoles(wiresAreNames bool) []string {
	if wiresAreNames {
		return []string{GoTableNames, GoTableKeys, GoTableFound}
	}
	return []string{GoTableNames, GoTableWires, GoTableKeys, GoTableFound}
}
