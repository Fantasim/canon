package eval

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// ownerFor is the record x binds to, so x equals the entry it names (TYPES.md §7.5, DECISIONS 410, 486).
func (b *binder) ownerFor(x *value.Ref, c *types.Collection) *value.Record {
	if b.surelyOwner(x) {
		return b.owner
	}
	en := b.e.heldEntry(b.owner, c, x.Key)
	if en == nil || en.Ident.Owner == nil || b.adopts(en) {
		return b.owner
	}
	return en.Ident.Owner
}

// surelyOwner reports x's entry owner's without a lookup: binding, no spread supplied a field;
// rebinding, no other place holds an instance and x, unbound or bound to inst, lies outside the
// field the amendment went through, which alone holds the refs and keys its line wrote.
func (b *binder) surelyOwner(x *value.Ref) bool {
	if b.mode == modeBind {
		return !b.spread
	}
	return !b.held() && !b.inVia && (x.Owner == nil || b.e.instance(x.Owner) == b.inst)
}

// adopts reports en, an entry owner holds, one owner owns once its let settles: made by the
// amendment, or of inst and held at no other place (instance.go's ownerFilter).
func (b *binder) adopts(en *value.Record) bool {
	if b.mode == modeBind {
		return false
	}
	return b.mv.made(en) || b.e.instance(en.Ident.Owner) == b.inst && !b.e.shared(en.Ident.Owner, b.mv)
}
