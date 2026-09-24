package gogen

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// loadView is one load<V>: read, check $schema, decode rows or value, build (CODEGEN.md §5.9, §6.1).
type loadView struct {
	Func, Type, Schema, Decode, RT, JSON, FMT  string
	Elem, Key, KeyStore, IDStore, RetiredStore string
	Resolve                                    string
	DupKey, DupToken                           string
	Record, Table                              bool
	Indexes                                    []index
	L                                          locals
	M                                          members
}

// loads writes load<V> for every emitted value, in declaration order.
func (g *gen) loads() {
	for _, v := range g.emitted {
		if isContainer(v) || v.Type.Kind == types.Record {
			g.load(v)
		}
	}
}

func (g *gen) load(v *ir.Value) {
	defer g.enter(v.Name)()
	rec, ok := rootKey(v).(*ir.Record)
	if !ok {
		g.failf(ErrMalformed, "value %s without its record", v.Name)
		return
	}
	g.ownClass(ir.TypeRef{Kind: types.Record, Named: rec})
	view := loadView{
		Func: g.names.LoadFunc(v), Type: g.valueType(v), Schema: g.names.SchemaName(v), Decode: g.decodeFunc(rec),
		RT: g.rt(), L: g.lc, M: containerMembers, Record: !isContainer(v),
	}
	if !view.Record {
		g.rows(&view, v, rec)
	}
	g.exec(loadTemplate, view)
}

// rows fills a container's loader: its rows, keys, FindBy indexes and its own refs' resolver (CODEGEN.md §5.8).
func (g *gen) rows(view *loadView, v *ir.Value, rec *ir.Record) {
	view.JSON, view.FMT = g.use(jsonPath, jsonPkg), g.use(fmtPkg, fmtPkg)
	view.Elem, view.Table = g.goName(rec), v.Type.Kind == types.Table
	keyExpr := view.L.Keys + lbracket + view.L.At + rbracket
	if view.Table {
		view.Key, view.KeyStore = g.idType(rec), ir.GoIDStore
		view.IDStore, view.RetiredStore = ir.GoIDStore, ir.GoRetiredStore
		view.Indexes = g.indexes(v, rec)
		view.DupKey = strconv.Quote("$id") // where jsonRowID (text/data.txt) writes it
		view.DupToken = g.keyTokenExpr(ir.TypeRef{Kind: types.String}, keyExpr)
	} else {
		kf := g.keyField(v.Type)
		view.Key, view.KeyStore = g.goType(kf.Type), g.keyMember(rec, kf)
		view.DupKey = strconv.Quote(strings.Join(kf.WirePath, dot))
		view.DupToken = g.keyTokenExpr(kf.Type, keyExpr)
	}
	if !v.Reload && g.names.NeedsWalk(rec) {
		view.Resolve = g.resolveFunc(rec)
	}
}
