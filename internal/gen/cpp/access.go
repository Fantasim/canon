package cppgen

import (
	_ "embed"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var (
	//go:embed text/loader_prelude.txt
	loaderPreludeText string
	//go:embed text/value_loader.txt
	valueLoaderText string
	//go:embed text/rows_open.txt
	rowsOpenText string
	//go:embed text/entry_read.txt
	entryReadText string
	//go:embed text/row_decode.txt
	rowDecodeText string
	//go:embed text/container_load.txt
	containerLoadText string
	//go:embed text/snapshot_load_def.txt
	snapshotLoadDefText string
	//go:embed text/reload_def.txt
	reloadDefText string
	//go:embed text/foreign_load.txt
	foreignLoadText string
)

// accessStruct is detail::<P>Access: one loader per value, then the snapshot's (§7.6).
func (g *gen) accessStruct() {
	g.c.printf(structOpenFormat, g.pl.AccessName())
	walks := g.walks()
	for i, v := range g.values {
		if i > 0 {
			g.c.blank()
		}
		leave := g.enter(v.Name)
		g.loader(v, !v.Reload && g.rootWalks(v, walks))
		leave()
	}
	if g.reloads() > 0 {
		g.c.blank()
		g.snapshotLoader(walks)
	}
	g.resolvers(walks)
	g.c.line(closeClass)
	g.c.blank()
}

// loader reads one data file, checks `$schema`, and decodes `rows` or `value` (CODEGEN.md §7.6).
func (g *gen) loader(v *ir.Value, resolve bool) {
	cls := g.valueClass(v)
	g.c.linef(1, loaderOpenFormat, g.pl.AccessLoader(v), cls)
	orders, arg := "", ""
	if g.readsNested() {
		orders, arg = tableOrdersDecl, tableOrdersArg
	}
	g.c.printf(loaderPreludeText, g.pl.SchemaName(v), orders, arg)
	if v.Type.Kind == types.Record {
		g.c.printf(valueLoaderText, g.decodeFunc(v.Type))
		g.c.linef(1, closeBrace)
		return
	}
	s := g.containerSpec(v)
	rec, _ := v.Type.Elem.Named.(*ir.Record)
	key := idMember
	if kf := g.keyFieldOf(v); kf != nil && rec != nil {
		key = g.entryField(rec, kf)
	}
	g.c.printf(rowsOpenText, s.elem, s.key)
	if rec != nil && g.entries[rec] && v.Type.Kind == types.Table {
		g.c.write(entryReadText)
	}
	g.c.printf(rowDecodeText, g.decodeFunc(*v.Type.Elem), key)
	for _, f := range s.stable {
		g.c.linef(depthThree, stableKeyFormat, g.pl.FindBy(f).Keys, g.entryField(rec, f))
	}
	g.c.linef(depthTwo, closeBrace)
	if s.wireKey == "" {
		g.c.lineAt(depthTwo, checkUniqueIDLine)
	} else {
		g.c.linef(depthTwo, checkUniqueFormat, quote(s.wireKey), s.render)
	}
	g.c.linef(depthTwo, fromRowsOutFormat, s.key, s.elem)
	for _, f := range s.stable {
		fb := g.pl.FindBy(f)
		g.c.linef(depthTwo, sortedIndexFormat, fb.Index, fb.Keys)
	}
	if resolve {
		g.resolveRows(depthTwo, v, outVar, outVar, failLoadText)
	}
	g.c.linef(depthTwo, returnTrue)
	g.c.linef(1, closeBrace)
}

// snapshotLoader loads each @reload value from dir + "/" + its file (CODEGEN.md §5.11, §7.6).
func (g *gen) snapshotLoader(walks []class) {
	snap := g.pl.SnapshotName()
	g.c.linef(1, snapshotLoaderOpenFormat, snap, g.pl.AccessSnapshotLoader())
	g.c.linef(depthTwo, makeSnapFormat, snap)
	for _, v := range g.values {
		if !v.Reload {
			continue
		}
		m, err := g.member(v.Name)
		g.fail(err)
		g.c.linef(depthTwo, loadIntoFormat, g.pl.AccessLoader(v), quote(pathSep+dataFile(v)), m)
	}
	for _, v := range g.values {
		if m, err := g.member(v.Name); v.Reload && err == nil && g.rootWalks(v, walks) {
			g.c.linef(depthTwo, openBrace)
			g.c.linef(depthThree, snapDecoderFormat, quote(pathSep+dataFile(v)))
			g.resolveRows(depthThree, v, snapHolder+m, snapCtx, failSnapshotText)
			g.c.linef(depthTwo, closeBrace)
		}
	}
	g.c.linef(depthTwo, returnSnap)
	g.c.linef(1, closeBrace)
}

// outOfLine defines the members the header only declares, in class order (CODEGEN.md §2.7).
func (g *gen) outOfLine() {
	if g.baked() {
		g.bakedDefinitions()
		return
	}
	for _, c := range g.classes {
		if v := g.loaders[c.rec]; c.rec != nil && v != nil {
			g.c.printf(containerLoadText, g.typeName(c.rec), g.pl.AccessName(), g.pl.AccessLoader(v), ir.CppLoad)
		}
	}
	for _, v := range g.values {
		if !v.Reload && v.Type.Kind != types.Record {
			g.c.printf(containerLoadText, g.pl.ContainerName(v), g.pl.AccessName(), g.pl.AccessLoader(v), ir.CppLoad)
		}
	}
	for _, v := range g.values {
		if name := g.pl.ForeignLoader(v); name != "" {
			g.c.printf(foreignLoadText, g.valueClass(v), name, g.pl.AccessName(), g.pl.AccessLoader(v))
		}
	}
	if g.reloads() > 0 {
		snap := g.pl.SnapshotName()
		g.c.printf(snapshotLoadDefText, snap, g.pl.AccessName(), g.pl.AccessSnapshotLoader(), ir.CppLoad)
		g.c.printf(reloadDefText, g.pl.StoreName(), snap, ir.CppLoad)
	}
	g.publicDecoders()
}
