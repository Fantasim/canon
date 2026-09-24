package gogen

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// containers writes the class of every emitted table and keyed list (CODEGEN.md §5.9).
func (g *gen) containers() {
	for _, v := range g.emitted {
		switch {
		case v.Type.Kind == types.Table:
			g.tableContainer(v)
		case v.Type.Kind == types.List && v.Type.KeyedBy != nil:
			g.keyedContainer(v)
		}
	}
}

func (g *gen) tableContainer(v *ir.Value) {
	rec, ok := g.sub(v.Type.Elem).Named.(*ir.Record)
	if !ok {
		return
	}
	name, elem := g.names.ContainerName(v), g.goName(rec)
	g.body.WriteString(docFor(name, v.Doc))
	g.printf(tableContainerFormat, name, elem, g.use(iterPkg, iterPkg), g.idType(rec),
		ir.GoRows, ir.GoLen, ir.GoAt, ir.GoAll, ir.GoFind, ir.GoGet)
	for _, f := range rec.Fields {
		if f.Stable {
			g.findBy(v, elem, f)
		}
	}
}

// findBy is FindBy<F> for a @stable field: a map index built once by the builder replaces a scan.
func (g *gen) findBy(v *ir.Value, elem string, f *ir.Field) {
	defer g.enter(v.Name + dot + f.Name)()
	container, ft := g.names.ContainerName(v), g.goType(f.Type)
	index := g.names.FindByIndex(v, f)
	g.printf(findByIndexFormat, index, g.use(syncPkg, syncPkg), ft, g.names.AccessorName(v), g.names.Slot(f).Store, ir.GoRows)
	g.printf(findByFormat, container, g.names.FindByName(f), ft, elem, index, ir.GoRows)
}

func (g *gen) keyedContainer(v *ir.Value) {
	defer g.enter(v.Name)()
	name := g.names.ContainerName(v)
	key := g.keyField(v.Type)
	g.body.WriteString(docFor(name, v.Doc))
	g.printf(keyedContainerFormat, name, g.listType(v.Type), g.typeName(g.sub(v.Type.Elem).Named),
		g.use(iterPkg, iterPkg), key.Name, g.goType(key.Type), ir.GoRows, ir.GoLen, ir.GoAt, ir.GoAll, ir.GoFind)
}

// values writes the baked data, built once through sync.OnceValue, and its accessors (§6.2).
func (g *gen) values() {
	if len(g.emitted) == 0 {
		return
	}
	d := g.names.Data()
	g.printf(structOpen, d.Type)
	for _, v := range g.emitted {
		for _, m := range g.valueStorage(v) {
			g.printf("%s %s\n", m.name, m.typ)
		}
	}
	g.printf("}\n\nvar %s = %s.OnceValue(%s)\n\n", d.Values, g.use(syncPkg, syncPkg), d.Build)
	g.printf("func %s() *%s {\n%s := &%s{}\n", d.Build, d.Type, g.data, d.Type)
	for _, v := range g.emitted {
		g.allocate(v)
	}
	for _, v := range g.emitted {
		g.fill(v)
	}
	g.printf("return %s\n}\n\n", g.data)
	for _, v := range g.emitted {
		g.accessors(v, d.Values)
	}
}

func isContainer(v *ir.Value) bool {
	return v.Type.Kind == types.Table || (v.Type.Kind == types.List && v.Type.KeyedBy != nil)
}

// valueSlot reads a value that is not a container like a field of its type.
func (g *gen) valueSlot(v *ir.Value) *slot {
	s := g.newSlot(v.Name, g.names.ValueSlot(v))
	s.doc = v.Doc
	return s
}

func (g *gen) valueStorage(v *ir.Value) []member {
	if isContainer(v) {
		return []member{{g.byValue[v.Name].store, g.names.ContainerName(v)}}
	}
	return g.storage(g.valueSlot(v))
}

// allocate gives every container its rows first, so any entry can point at any other.
func (g *gen) allocate(v *ir.Value) {
	defer g.enter(v.Name)()
	store := g.data + dot + g.byValue[v.Name].store + dot + ir.GoRows
	n := strconv.Itoa(len(g.entries(v)))
	switch {
	case v.Type.Kind == types.Table:
		g.printf("%s = make([]%s, %s)\n", store, g.typeName(g.sub(v.Type.Elem).Named), n)
	case isContainer(v):
		key := g.keyField(v.Type)
		keys := make([]string, 0, len(g.entries(v)))
		for _, r := range g.entries(v) {
			keys = append(keys, g.expr(key.Type, g.fieldValue(r, key.Name)))
		}
		g.printf("%s = %s%smake([]%s, %s), %s%s%s)\n", store, g.rt(), makeKeyedList,
			g.typeName(g.sub(v.Type.Elem).Named), n, sliceOf, g.goType(key.Type), braced(keys))
	}
}

// fill writes each entry into its row, or a value into its members.
func (g *gen) fill(v *ir.Value) {
	store := g.data + dot + g.byValue[v.Name].store
	if !isContainer(v) {
		for _, a := range g.assign(g.valueSlot(v), v.V) {
			g.printf("%s.%s = %s\n", g.data, a.name, a.expr)
		}
		return
	}
	rec, ok := g.sub(v.Type.Elem).Named.(*ir.Record)
	if !ok {
		return
	}
	for i, r := range g.entries(v) {
		row := store + dot + ir.GoRows + lbracket + strconv.Itoa(i) + rbracket
		if v.Type.Kind != types.Table {
			row = pointer + store + atCall + strconv.Itoa(i) + rparen
		}
		g.printf("%s = %s\n", row, g.recordLit(rec, r))
	}
}

// accessors are Get<V>, and Get<V>ID for a ref (CODEGEN.md §3.3, §5.9).
func (g *gen) accessors(v *ir.Value, values string) {
	recv := values + callSuffix
	if !isContainer(v) {
		for _, gt := range g.getters(g.valueSlot(v), recv) {
			g.writeFunc(gt)
		}
		return
	}
	g.writeFunc(getter{
		name: g.names.AccessorName(v), result: pointer + g.names.ContainerName(v), doc: v.Doc,
		body: returnKw + ampersand + recv + dot + g.byValue[v.Name].store,
	})
}

// writeFunc writes a package-level getter.
func (g *gen) writeFunc(gt getter) {
	g.writeGetter(funcKw, gt)
}
