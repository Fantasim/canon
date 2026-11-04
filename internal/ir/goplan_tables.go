package ir

// TableName is the package-level table of role (GoTableNames...) of the enum, kind enum or id enum named enum (CODEGEN.md §5.2, §5.3, DECISIONS 393).
func (pl *GoNamePlan) TableName(role, enum string) string { return role + underscore + enum }

// declareTables declares the tables of roles for the type named enum.
func (pl *GoNamePlan) declareTables(top *nameScope, enum, origin string, item any, roles ...string) {
	for _, role := range roles {
		pl.declare(top, pl.TableName(role, enum), origin, item)
	}
}

// enumTableRoles are the tables gen/go writes for an enum (CODEGEN.md §5.2, DECISIONS 430): an opened one has its members sorted by wire and indexes into them; a closed one reads names and wires by index (by code with sparse @codes) and searches the by-wire indexes.
func enumTableRoles(e *Enum, open bool) []string {
	if open {
		return openTableRoles(e)
	}
	roles := closedTableRoles(EnumWiresAreNames(e))
	if _, dense := DenseCodes(e); e.Codes != nil && !dense {
		roles = append(roles, GoTableCodes)
	}
	return append(roles, GoTableMembers)
}

// openTableRoles are the tables of an opened enum: the keys, the names only when one differs from its wire, with @codes the codes in key order and, when they are not dense, the sorted codes and the by-code indexes, and the members indexes.
func openTableRoles(e *Enum) []string {
	roles := []string{GoTableKeys}
	if !EnumWiresAreNames(e) {
		roles = append(roles, GoTableNames)
	}
	if e.Codes != nil {
		roles = append(roles, GoTableNumbers)
		if _, dense := DenseCodes(e); !dense {
			roles = append(roles, GoTableCodes, GoTableByCode)
		}
	}
	return append(roles, GoTableMembers)
}

// closedTableRoles are the tables of a closed enum or kind enum; one whose wires are all its names has no wires table, Wire reads the names.
func closedTableRoles(wiresAreNames bool) []string {
	if wiresAreNames {
		return []string{GoTableNames, GoTableByWire}
	}
	return []string{GoTableNames, GoTableWires, GoTableByWire}
}
