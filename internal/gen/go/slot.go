package gogen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// slot is how a field, value or precomputed result is stored and read (CODEGEN.md §4.3, §5.8).
type slot struct {
	origin   string // Canon name, for messages
	field    string // the Canon field a field's slot reads; "" for any other slot
	getter   string
	store    string
	doc      string
	t        ir.TypeRef // without the outer `?`
	opt      bool
	ref      *ir.RefTarget // the target of `ref T` or `[ref T]`
	list     bool
	resolved bool
	fn       *ir.ExportFn // the export fn of a precomputed result
}

// member is one storage field: a struct field or a member of the baked data.
type member struct {
	name, typ, origin string
}

// pair is one member of a composite literal, or one assignment of the baked data.
type pair struct {
	name, expr string
}

// getter is a generated read method or accessor; body is a statement list.
type getter struct {
	name, result, body, doc, origin string
}

func (g *gen) newSlot(origin, getterName, store string, t ir.TypeRef, opt bool) *slot {
	s := &slot{origin: origin, getter: getterName, store: store, t: t, opt: opt}
	switch {
	case t.Kind == types.Ref:
		s.ref = t.Ref
	case t.Kind == types.List && t.KeyedBy == nil && t.Elem != nil && t.Elem.Kind == types.Ref:
		s.ref, s.list = t.Elem.Ref, true
	}
	s.resolved = s.ref != nil && g.resolvable(s.ref)
	return s
}

// unwrapOptional splits an IR result type T? into T and true.
func unwrapOptional(t ir.TypeRef) (ir.TypeRef, bool) {
	if t.Kind == types.Optional && t.Elem != nil {
		return *t.Elem, true
	}
	return t, false
}

// refType is the ref TypeRef itself: t, or the element of `[ref T]`.
func (s *slot) refType() ir.TypeRef {
	if s.list {
		return *s.t.Elem
	}
	return s.t
}

func (s *slot) hasMain() bool { return s.ref == nil || s.resolved }

func (s *slot) hasKey() bool { return s.ref != nil && (!s.resolved || s.list) }

func (s *slot) keyStore() string {
	if s.list {
		return s.store + idsStoreSuffix
	}
	return s.store + idStoreSuffix
}

func (s *slot) okStore() string { return s.store + okStoreSuffix }

// mainType is what the main getter returns: resolved refs are entries, never keys.
func (g *gen) mainType(s *slot) string {
	defer g.enter(s.origin)()
	switch {
	case s.ref == nil:
		return g.goType(s.t)
	case s.list:
		return g.rt() + listType + lbracket + pointer + g.typeName(s.ref.Elem) + rbracket
	}
	return pointer + g.typeName(s.ref.Elem)
}

func (g *gen) slotKeyType(s *slot) string {
	defer g.enter(s.origin)()
	key := g.goType(s.refType())
	if s.list {
		return g.rt() + listType + lbracket + key + rbracket
	}
	return key
}

// needsOK reports an optional slot that nil cannot mark: a pointer main alone can.
func (g *gen) needsOK(s *slot) bool {
	if !s.opt {
		return false
	}
	return !s.hasMain() || s.hasKey() || !strings.HasPrefix(g.mainType(s), pointer)
}

// storage is the slot's members: main, key, then the presence flag.
func (g *gen) storage(s *slot) []member {
	var out []member
	if s.hasMain() {
		out = append(out, member{s.store, g.mainType(s), s.origin})
	}
	if s.hasKey() {
		out = append(out, member{s.keyStore(), g.slotKeyType(s), s.origin})
	}
	if g.needsOK(s) {
		out = append(out, member{s.okStore(), goBool, s.origin})
	}
	return out
}

// getters is the main getter, then the key getter of a ref; recv reads the storage.
func (g *gen) getters(s *slot, recv string) []getter {
	var out []getter
	ok := g.needsOK(s)
	if s.hasMain() {
		out = append(out, getter{
			name: s.getter, result: results(g.mainType(s), ok),
			body: returns(recv+dot+s.store, ok, recv+dot+s.okStore()), doc: s.doc, origin: s.origin,
		})
	}
	if s.ref == nil {
		return out
	}
	key := getter{name: s.getter + idSuffixUpper, result: results(g.slotKeyType(s), ok), origin: s.origin}
	if s.list {
		key.name += pluralSuffix
	}
	if !s.hasMain() {
		key.doc = s.doc
	}
	if s.hasKey() {
		key.body = returns(recv+dot+s.keyStore(), ok, recv+dot+s.okStore())
	} else {
		key.body = g.derivedKey(s, recv)
		key.result = results(g.slotKeyType(s), s.opt)
	}
	return append(out, key)
}

// derivedKey reads a resolved ref's key off its entry, as GetInitialStatusID does (§6.2).
func (g *gen) derivedKey(s *slot, recv string) string {
	entry := recv + dot + s.store
	key := entry + dot + g.targetKey(s.ref)
	if !s.opt {
		return returnKw + key
	}
	return ifNilFormat(entry, g.zeroKey(s.refType())) + returnKw + key + listSep + trueLit
}

// targetKey is the storage of the key on a resolved entry: id, or the keyed list's key field.
func (g *gen) targetKey(r *ir.RefTarget) string {
	if !r.Keyed {
		return idStore
	}
	keyed := g.byValue[r.Value].v.Type.KeyedBy
	if keyed == nil {
		g.failf(ErrMalformed, "a keyed ref into %s, which is not a keyed list", r.Value)
		return idStore
	}
	return storageName(keyed.Name)
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
		if !s.opt {
			g.failf(ErrMalformed, "%s has no value", s.origin)
		}
		return nil
	}
	var out []pair
	if s.hasMain() {
		out = append(out, pair{s.store, g.mainExpr(s, v)})
	}
	if s.hasKey() {
		out = append(out, pair{s.keyStore(), g.keyExpr(s, v)})
	}
	if g.needsOK(s) {
		out = append(out, pair{s.okStore(), trueLit})
	}
	return out
}

func (g *gen) mainExpr(s *slot, v value.Value) string {
	switch {
	case s.ref == nil:
		return g.expr(s.t, v)
	case !s.list:
		return g.resolvedExpr(s.ref, as[value.Ref](g, v).Key)
	}
	elems := as[value.List](g, v).Elems
	elem := pointer + g.typeName(s.ref.Elem)
	if len(elems) == 0 {
		return g.rt() + listType + lbracket + elem + rbracket + emptyBraces
	}
	items := make([]string, len(elems))
	for i, x := range elems {
		items[i] = g.resolvedExpr(s.ref, as[value.Ref](g, x).Key)
	}
	return g.rt() + makeList + sliceOf + elem + braced(items) + rparen
}

func (g *gen) keyExpr(s *slot, v value.Value) string {
	if !s.list {
		return g.keyLit(s.t, as[value.Ref](g, v).Key)
	}
	return g.expr(s.t, v)
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
