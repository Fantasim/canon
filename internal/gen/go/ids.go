package gogen

// writeID writes a baked table's opaque id: a struct holding the entry's position, a var per key, and its methods (CODEGEN.md §5.3, DECISIONS 379).
func (g *gen) writeID(s *enumSpec) {
	g.printf(idDocFormat, s.name, s.value)
	g.writeStruct(s)
	v := g.enumView(s)
	g.writeTable(v.Names, goString, quoted(s.names))
	g.printf(idFormat, s.name, v.Names, g.use(strconvPkg, strconvPkg), s.value, v.Parse, g.use(errorsPkg, errorsPkg))
	g.writeParse(s, v)
	g.writeRetired(s, v, "", identity(len(s.consts)))
}

// writeStruct writes an opaque type's struct holding its position and a var per member (CODEGEN.md §5.2, §5.3; DECISIONS 379, 402).
func (g *gen) writeStruct(s *enumSpec) {
	g.printf(idHeaderFormat, s.name, s.under)
	if len(s.consts) == 0 {
		return
	}
	g.printf(inputVarBlockOpen)
	for _, c := range s.consts {
		g.body.WriteString(docFor(c.name, c.doc))
		g.printf(idVarFormat, c.name, s.name, c.value)
	}
	g.printf(closeParenFormat)
}

// writeBranch writes a dependent type's branch enum: the struct and vars of an id, String over the branch names, Position, the text methods and Parse<B> over the names, no Retired (CODEGEN.md §5.6, T20; DECISIONS 402).
func (g *gen) writeBranch(s *enumSpec) {
	g.writeStruct(s)
	v := g.enumView(s)
	g.writeTable(v.Names, goString, quoted(s.names))
	g.printf(branchFormat, s.name, v.Names, g.use(strconvPkg, strconvPkg), v.Parse, g.use(errorsPkg, errorsPkg))
	g.writeParse(s, v)
}
