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

func containerName(v *ir.Value) string { return upperCamel(v.Name) }

func (g *gen) tableContainer(v *ir.Value) {
	rec, ok := g.sub(v.Type.Elem).Named.(*ir.Record)
	if !ok {
		return
	}
	name, elem := containerName(v), g.goName(rec)
	g.declare(name, v.Name)
	g.body.WriteString(docFor(name, v.Doc))
	g.printf(tableContainerFormat, name, elem, g.use(iterPkg, iterPkg), idTypeName(elem))
	methods := newScope(name)
	for _, f := range rec.Fields {
		if f.Stable && !f.Optional {
			g.findBy(methods, name, elem, f)
		}
	}
}

// findBy is FindBy<F> for a @stable field of a table's element (CODEGEN.md §5.9).
func (g *gen) findBy(methods *scope, container, elem string, f *ir.Field) {
	name := findByPrefix + exportedName(f.Go.Name, f.Name)
	g.fail(methods.add(name, container))
	g.printf(findByFormat, container, name, g.goType(f.Type), elem, storageName(f.Name))
}

func (g *gen) keyedContainer(v *ir.Value) {
	name := containerName(v)
	g.declare(name, v.Name)
	key := g.keyField(v.Type)
	g.body.WriteString(docFor(name, v.Doc))
	g.printf(keyedContainerFormat, name, g.listType(v.Type), g.typeName(g.sub(v.Type.Elem).Named),
		g.use(iterPkg, iterPkg), key.Name, g.goType(key.Type))
}

// values writes the baked data, built once through sync.OnceValue, and its accessors (§6.2).
func (g *gen) values() {
	if len(g.emitted) == 0 {
		return
	}
	data, values, build := g.dataNames()
	fields := newScope(data)
	g.printf(structOpen, data)
	for _, v := range g.emitted {
		for _, m := range g.valueStorage(v) {
			g.fail(fields.add(m.name, m.origin))
			g.printf("%s %s\n", m.name, m.typ)
		}
	}
	g.printf("}\n\nvar %s = %s.OnceValue(%s)\n\n", values, g.use(syncPkg, syncPkg), build)
	g.printf("func %s() *%s {\n%s := &%s{}\n", build, data, g.data, data)
	for _, v := range g.emitted {
		g.allocate(v)
	}
	for _, v := range g.emitted {
		g.fill(v)
	}
	g.printf("return %s\n}\n\n", g.data)
	for _, v := range g.emitted {
		g.accessors(v, values)
	}
}

// dataNames are the baked data's type, its OnceValue and its builder (CODEGEN.md §6.2).
func (g *gen) dataNames() (data, values, build string) {
	data, values, build = g.last+dataSuffix, g.last+valuesSuffix, buildPrefix+firstUpper(g.last)
	for _, n := range []string{data, values, build} {
		g.declare(n, g.p.Name)
	}
	return data, values, build
}

func isContainer(v *ir.Value) bool {
	return v.Type.Kind == types.Table || (v.Type.Kind == types.List && v.Type.KeyedBy != nil)
}

// valueSlot reads a value that is not a container like a field of its type.
func (g *gen) valueSlot(v *ir.Value) *slot {
	t, opt := unwrapOptional(v.Type)
	name := v.Go.Name
	if name == "" {
		name = getPrefix + upperCamel(v.Name)
	}
	s := g.newSlot(v.Name, name, g.byValue[v.Name].store, t, opt)
	s.doc = v.Doc
	return s
}

func (g *gen) valueStorage(v *ir.Value) []member {
	if isContainer(v) {
		return []member{{g.byValue[v.Name].store, containerName(v), v.Name}}
	}
	return g.storage(g.valueSlot(v))
}

// allocate gives every container its rows first, so any entry can point at any other.
func (g *gen) allocate(v *ir.Value) {
	store := g.data + dot + g.byValue[v.Name].store + dot + rowsField
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
		row := store + dot + rowsField + lbracket + strconv.Itoa(i) + rbracket
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
			g.declare(gt.name, v.Name)
			g.writeFunc(gt)
		}
		return
	}
	name := v.Go.Name
	if name == "" {
		name = getPrefix + upperCamel(v.Name)
	}
	g.declare(name, v.Name)
	g.writeFunc(getter{
		name: name, result: pointer + containerName(v), doc: v.Doc, origin: v.Name,
		body: returnKw + ampersand + recv + dot + g.byValue[v.Name].store,
	})
}

// writeFunc writes a package-level getter.
func (g *gen) writeFunc(gt getter) {
	g.writeGetter(newScope(gt.name), funcKw, gt)
}
