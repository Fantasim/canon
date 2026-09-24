package gogen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// slot is how a field, value or precomputed result is stored and read (CODEGEN.md §4.3, §5.8): the plan's layout and names, and what writing it needs.
type slot struct {
	ir.GoSlot
	origin string // Canon name, for messages
	field  string // the Canon field a field's slot reads; "" for any other slot
	doc    string
	fn     *ir.ExportFn // the export fn of a precomputed result
}

// member is one storage field: a struct field or a member of the baked data.
type member struct {
	name, typ string
}

// pair is one member of a composite literal, or one assignment of the baked data.
type pair struct {
	name, expr string
}

// getter is a generated read method or accessor; body is a statement list.
type getter struct {
	name, result, body, doc string
}

func (g *gen) newSlot(origin string, layout ir.GoSlot) *slot {
	return &slot{GoSlot: layout, origin: origin}
}

// refType is the ref TypeRef itself: T, or the element of `[ref T]`.
func (s *slot) refType() ir.TypeRef {
	if s.List {
		return *s.T.Elem
	}
	return s.T
}

func (s *slot) hasMain() bool { return s.Main }

func (s *slot) hasKey() bool { return s.Key }

// needsOK reports an optional slot that nil cannot mark: a pointer main alone can.
func (s *slot) needsOK() bool { return s.OK }

// mainType is what the main getter returns: resolved refs are entries, never keys.
func (g *gen) mainType(s *slot) string {
	defer g.enter(s.origin)()
	switch {
	case s.Ref == nil:
		return g.goType(s.T)
	case s.List:
		return g.rt() + listType + lbracket + pointer + g.typeName(s.Ref.Elem) + rbracket
	}
	return pointer + g.typeName(s.Ref.Elem)
}

func (g *gen) slotKeyType(s *slot) string {
	defer g.enter(s.origin)()
	key := g.goType(s.refType())
	if s.List {
		return g.rt() + listType + lbracket + key + rbracket
	}
	return key
}

// storage is the slot's members: main, key, then the presence flag.
func (g *gen) storage(s *slot) []member {
	var out []member
	if s.hasMain() {
		out = append(out, member{s.Store, g.mainType(s)})
	}
	if s.hasKey() {
		out = append(out, member{s.KeyStore, g.slotKeyType(s)})
	}
	if s.needsOK() {
		out = append(out, member{s.OKStore, goBool})
	}
	return out
}

// getters is the main getter, then the key getter of a ref; recv reads the storage.
func (g *gen) getters(s *slot, recv string) []getter {
	var out []getter
	ok := s.needsOK()
	if s.hasMain() {
		out = append(out, getter{
			name: s.Getter, result: results(g.mainType(s), ok),
			body: returns(recv+dot+s.Store, ok, recv+dot+s.OKStore), doc: s.doc,
		})
	}
	if s.Ref == nil {
		return out
	}
	key := getter{name: s.KeyGetter, result: results(g.slotKeyType(s), ok)}
	if !s.hasMain() {
		key.doc = s.doc
	}
	if s.hasKey() {
		key.body = returns(recv+dot+s.KeyStore, ok, recv+dot+s.OKStore)
	} else {
		key.body = g.derivedKey(s, recv)
		key.result = results(g.slotKeyType(s), s.Optional)
	}
	return append(out, key)
}

// derivedKey reads a resolved ref's key off its entry, as GetInitialStatusID does (§6.2).
func (g *gen) derivedKey(s *slot, recv string) string {
	entry := recv + dot + s.Store
	key := entry + dot + g.targetKey(s.Ref)
	if !s.Optional {
		return returnKw + key
	}
	return ifNilFormat(entry, g.zeroKey(s.refType())) + returnKw + key + listSep + trueLit
}

// targetKey is the storage of the key on a resolved entry: id, or the keyed list's key field.
func (g *gen) targetKey(r *ir.RefTarget) string {
	if !r.Keyed {
		return ir.GoIDStore
	}
	t := g.byValue[r.Value].v.Type
	if t.KeyedBy == nil {
		g.failf(ErrMalformed, "a keyed ref into %s, which is not a keyed list", r.Value)
		return ir.GoIDStore
	}
	return g.names.Slot(g.keyField(t)).Store
}

func ifNilFormat(entry, zero string) string { return fmt.Sprintf(ifNilFormatText, entry, zero) }

func (g *gen) zeroKey(t ir.TypeRef) string {
	if !isTableRef(t.Ref) && t.Key != nil && t.Key.Kind == types.String {
		return emptyString
	}
	return zeroLit
}

// assign is the storage of v, member by member; none leaves every member zero.
func (g *gen) assign(s *slot, v value.Value) []pair {
	defer g.enter(s.origin)()
	if _, none := v.(*value.None); none || v == nil {
		if !s.Optional {
			g.failf(ErrMalformed, "%s has no value", s.origin)
		}
		return nil
	}
	var out []pair
	if s.hasMain() {
		out = append(out, pair{s.Store, g.mainExpr(s, v)})
	}
	if s.hasKey() {
		out = append(out, pair{s.KeyStore, g.keyExpr(s, v)})
	}
	if s.needsOK() {
		out = append(out, pair{s.OKStore, trueLit})
	}
	return out
}

func (g *gen) mainExpr(s *slot, v value.Value) string {
	switch {
	case s.Ref == nil:
		return g.expr(s.T, v)
	case !s.List:
		return g.resolvedExpr(s.Ref, as[value.Ref](g, v).Key)
	}
	elems := as[value.List](g, v).Elems
	elem := pointer + g.typeName(s.Ref.Elem)
	if len(elems) == 0 {
		return g.rt() + listType + lbracket + elem + rbracket + emptyBraces
	}
	items := make([]string, len(elems))
	for i, x := range elems {
		items[i] = g.resolvedExpr(s.Ref, as[value.Ref](g, x).Key)
	}
	return g.rt() + makeList + sliceOf + elem + braced(items) + rparen
}

func (g *gen) keyExpr(s *slot, v value.Value) string {
	if !s.List {
		return g.keyLit(s.T, as[value.Ref](g, v).Key)
	}
	return g.expr(s.T, v)
}

// results is a getter's result list: T, or (T, bool) for an optional one.
func results(t string, ok bool) string {
	if ok {
		return lparen + t + listSep + goBool + rparen
	}
	return t
}

func returns(x string, ok bool, okExpr string) string {
	if ok {
		return returnKw + x + listSep + okExpr
	}
	return returnKw + x
}
