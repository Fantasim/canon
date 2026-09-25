package gogen

import (
	_ "embed"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/template"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var (
	//go:embed text/data.txt
	dataText string
	//go:embed text/translated.txt
	translatedText string
	//go:embed text/conformance.txt
	conformanceText string
)

// dataTemplates are the fixed code shapes: data mode's table container, loaders, snapshot and
// store; a translated fn's method and pure function; the conformance file's helpers and tests.
var dataTemplates = parseTemplates(dataText, translatedText, conformanceText)

// parseTemplates parses texts of `define`s into one set.
func parseTemplates(texts ...string) *template.Template {
	set := template.New(dataTemplateSet)
	for _, text := range texts {
		set = template.Must(set.Parse(text))
	}
	return set
}

// members are the fixed member and method names of a container, ir's (CODEGEN.md §5.9).
type members struct{ Rows, Len, At, All, Find string }

var containerMembers = members{Rows: ir.GoRows, Len: ir.GoLen, At: ir.GoAt, All: ir.GoAll, Find: ir.GoFind}

// index is a table's FindBy<F>: its container member, method, key type and the entry's storage.
type index struct{ Member, Method, Type, Store string }

// dataSections are a data-mode file's sections in CODEGEN.md §2.7's order: decoders last.
func (g *gen) dataSections() []func() {
	return []func(){
		g.constants, g.schemas, g.enums, g.kindEnums, g.branchEnums, g.idEnums, g.types, g.containers,
		g.loaders, g.snapshot, g.fns, g.runtimeInputs, g.decoders, g.resolvers, g.loads,
	}
}

// exec writes one template of dataTemplates to the main file.
func (g *gen) exec(name string, data any) { g.execTo(&g.body, name, data) }

func (g *gen) execTo(w io.Writer, name string, data any) {
	if err := dataTemplates.ExecuteTemplate(w, name, data); err != nil {
		g.fail(fmt.Errorf("%w: %w", errFormat, err))
	}
}

// schemas writes each emitted value's schema constant, T2 (CODEGEN.md §3.3, FINGERPRINT.md §2).
func (g *gen) schemas() {
	for _, v := range g.emitted {
		name := g.names.SchemaName(v)
		g.printf(schemaDocFormat, name, dataFile(v))
		g.printf(constDeclFormat, name, strconv.Quote(v.Schema))
	}
}

// dataFile is the file the package's emit json writes v to (WIRE.md §8.1, E8153).
func dataFile(v *ir.Value) string { return v.Name + ir.JSONExt }

// dataTable is a data-mode table container, keyed by the string id, with FindBy indexes (CODEGEN.md §5.3, §5.9).
func (g *gen) dataTable(v *ir.Value, rec *ir.Record) {
	name := g.names.ContainerName(v)
	view := struct {
		Doc, Name, ID, Elem, RT, Iter string
		M                             members
		Indexes                       []index
	}{
		Doc: docFor(name, v.Doc), Name: name, ID: g.idType(rec), Elem: g.goName(rec),
		RT: g.rt(), Iter: g.use(iterPkg, iterPkg), M: containerMembers, Indexes: g.indexes(v, rec),
	}
	g.exec(tableTemplate, view)
}

func (g *gen) indexes(v *ir.Value, rec *ir.Record) []index {
	var out []index
	for _, f := range rec.Fields {
		if f.Stable {
			out = append(out, index{
				Member: g.names.FindByIndex(v, f), Method: g.names.FindByName(f),
				Type: g.goType(f.Type), Store: g.names.Slot(f).Store,
			})
		}
	}
	return out
}

// valueType is the Go type a value loads into: its container, or its record (CODEGEN.md §5.9).
func (g *gen) valueType(v *ir.Value) string {
	if isContainer(v) {
		return g.names.ContainerName(v)
	}
	return g.typeName(v.Type.Named)
}

// loaders writes Load<V> for each emitted value that is not @reload (CODEGEN.md §5.9, E8015).
func (g *gen) loaders() {
	for _, v := range g.emitted {
		if !isContainer(v) && v.Type.Kind != types.Record {
			g.fail(newDetail(ErrMalformed, v.Name, dataValueFormat, v.Name)) // E8015
			continue
		}
		if v.Reload {
			continue
		}
		name := g.names.LoaderName(v)
		g.exec(loaderTemplate, struct {
			Doc, Name, Type, Func string
			L                     locals
		}{docFor(name, v.Doc), name, g.valueType(v), g.names.LoadFunc(v), g.lc})
	}
}

// snapshotValue is one @reload value of the snapshot.
type snapshotValue struct {
	Doc, Store, Type, Getter, Load, File, Resolve string
	Record                                        bool
}

// snapshot writes <P>Snapshot, Load<P>Snapshot, <P>Store and Store (CODEGEN.md §5.11, T4–T6, T9).
func (g *gen) snapshot() {
	var vals []snapshotValue
	var files []string
	for _, v := range g.emitted {
		if !v.Reload {
			continue
		}
		getter := g.names.ContainerName(v)
		sv := snapshotValue{
			Doc: docFor(getter, v.Doc), Store: g.names.ValueStore(v), Type: g.valueType(v), Getter: getter,
			Load: g.names.LoadFunc(v), File: strconv.Quote(pathSep + dataFile(v)), Record: !isContainer(v),
		}
		if key := rootKey(v); key != nil && g.names.NeedsWalk(key) {
			sv.Resolve = g.resolveFunc(key)
		}
		vals = append(vals, sv)
		files = append(files, dataFile(v))
	}
	if len(vals) == 0 {
		return
	}
	names := g.names.Snapshot()
	g.exec(snapshotTemplate, struct {
		Snap, StoreType, Pkg, GoPkg, Files, FMT, Atomic string
		Values                                          []snapshotValue
		L                                               locals
		M                                               members
	}{
		names.Type, names.Store, g.p.Name, g.e.GoPackage, strings.Join(files, listSep),
		g.use(fmtPkg, fmtPkg), g.use(atomicPath, atomicName), vals, g.lc, containerMembers,
	})
}
