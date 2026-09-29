package lock

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// comparison is one locked collection compared with the sources.
type comparison struct {
	lock, cur       []Fact
	lockAll, curAll map[coll][]Fact
	name            string
	bag             *diag.Bag
}

// Verify reports every violation of the lock rules to bag, which must not be nil. Only a lock
// read whole is compared, and never with a skipped collection.
func (f *File) Verify(s *Sources, bag *diag.Bag) {
	lockAll, curAll := f.byColl(), s.facts.byColl()
	for _, c := range sortedColls(lockAll) {
		facts := lockAll[c]
		if s.skipped(c) {
			continue
		}
		conflicts(facts, bag)
		if !s.colls[c] {
			first := slices.MinFunc(facts, byLine)
			diag.E6001.AtGone(first.Span, goneKinds[c.kind], c.name).Report(bag)
			continue
		}
		collRules[c.kind](&comparison{lock: facts, cur: curAll[c], lockAll: lockAll, curAll: curAll, name: c.name, bag: bag})
	}
}

// byColl groups the facts by collection, each group in canonical order.
func (f *File) byColl() map[coll][]Fact {
	out := map[coll][]Fact{}
	for _, fact := range f.facts {
		c := coll{kind: fact.Kind, name: fact.writtenName()}
		out[c] = append(out[c], fact)
	}
	return out
}

// sortedColls is the collections in canonical order (LOCK.md §2.3).
func sortedColls(m map[coll][]Fact) []coll {
	return slices.SortedFunc(maps.Keys(m), func(a, b coll) int {
		return cmp.Or(cmp.Compare(a.kind, b.kind), cmp.Compare(a.name, b.name))
	})
}

// byLine orders facts by where the lock states them, then canonically.
func byLine(a, b Fact) int {
	return cmp.Or(cmp.Compare(a.Span.Start, b.Span.Start), compareFacts(a, b))
}

// conflicts reports two facts of one value or one holder, at the second line (LOCK.md §4.2).
func conflicts(facts []Fact, bag *diag.Bag) {
	if len(facts) == 0 || facts[0].Kind == KindTable {
		return
	}
	lines := slices.SortedFunc(slices.Values(facts), byLine)
	byValue, byHolder := map[Value]Fact{}, map[string]Fact{}
	for _, b := range lines {
		a, sameValue := byValue[b.Value]
		if sameValue {
			diag.E6002.AtConflict(b.Span, b.writtenName(), a.line(), b.line()).Report(bag)
		} else {
			byValue[b.Value] = b
		}
		a, sameHolder := byHolder[b.Holder]
		if sameHolder {
			diag.E6002.AtConflict(b.Span, b.qualified(), a.line(), b.line()).Report(bag)
		} else {
			byHolder[b.Holder] = b
		}
	}
}

// valueIndex groups facts by value, each group in the facts' order.
func valueIndex(facts []Fact) map[Value][]Fact {
	out := map[Value][]Fact{}
	for _, f := range facts {
		out[f.Value] = append(out[f.Value], f)
	}
	return out
}

// holders indexes facts by holder, the first of each.
func holders(facts []Fact) map[string]Fact {
	out := map[string]Fact{}
	for _, f := range facts {
		if _, seen := out[f.Holder]; !seen {
			out[f.Holder] = f
		}
	}
	return out
}

// pair is a value and its holder.
type pair struct {
	value  Value
	holder string
}

// pairs is every (value, holder) the facts state.
func pairs(facts []Fact) map[pair]bool {
	out := map[pair]bool{}
	for _, f := range facts {
		out[pair{f.Value, f.Holder}] = true
	}
	return out
}

// tableRules: a locked key is never removed (E6001: held when a new key holds one of its @stable
// values, renamed when exactly one key went and one came), and a retired key never comes back.
func tableRules(c *comparison) {
	locked, now := holders(c.lock), holders(c.cur)
	var gone, added []Fact
	for _, l := range c.lock {
		if _, ok := now[l.Holder]; !ok {
			gone = append(gone, l)
		}
	}
	for _, f := range c.cur {
		if _, ok := locked[f.Holder]; !ok {
			added = append(added, f)
		}
	}
	for _, l := range gone {
		c.gone(l, locked, len(gone) == 1 && len(added) == 1, added)
	}
	for _, l := range c.lock {
		if f, ok := now[l.Holder]; ok && l.Retired && !f.Retired {
			diag.E6002.AtUnretire(f.Span, diag.KindEntry, l.Holder).Report(c.bag)
		}
	}
}

// gone reports a locked key gone from the sources (LOCK.md §4.1).
func (c *comparison) gone(l Fact, locked map[string]Fact, renamed bool, added []Fact) {
	switch h, held := c.stableHeld(l.Holder, locked); {
	case held:
		diag.E6001.AtHeld(l.Span, l.qualified(), diag.KindStableValue, h.Value.arg(), h.Holder).Report(c.bag)
	case renamed:
		diag.E6001.AtRenamed(l.Span, l.qualified(), added[0].Holder, l.Holder).Report(c.bag)
	default:
		diag.E6001.AtRemoved(l.Span, l.qualified()).Report(c.bag)
	}
}

// stableHeld is the first current fact, in canonical order, of a key the lock does not know
// that holds a @stable value the gone key locks.
func (c *comparison) stableHeld(key string, locked map[string]Fact) (Fact, bool) {
	for _, fc := range sortedColls(c.lockAll) {
		if fc.kind != KindField || fc.name[:max(strings.LastIndex(fc.name, nameSep), 0)] != c.name {
			continue
		}
		for _, l := range c.lockAll[fc] {
			if f, ok := newHolder(l, key, c.curAll[fc], locked); ok {
				return f, true
			}
		}
	}
	return Fact{}, false
}

// newHolder is the current fact of a key unknown to the lock holding l's value, when l is key's.
func newHolder(l Fact, key string, cur []Fact, locked map[string]Fact) (Fact, bool) {
	if l.Holder != key {
		return Fact{}, false
	}
	for _, f := range cur {
		if _, known := locked[f.Holder]; !known && f.Value == l.Value {
			return f, true
		}
	}
	return Fact{}, false
}

// enumRules: a member keeps its code (E6002 renumber), is never removed (E6001, held when its
// code now belongs to a new member), never un-retired, and its code is no other's (E6002).
func enumRules(c *comparison) {
	locked, now, known, curValues := holders(c.lock), holders(c.cur), pairs(c.lock), valueIndex(c.cur)
	for _, l := range c.lock {
		f, present := now[l.Holder]
		if !present {
			removed(l, c.cur, locked, c.bag)
			continue
		}
		if l.Retired && !f.Retired {
			diag.E6002.AtUnretire(f.Span, diag.KindMember, l.Holder).Report(c.bag)
		}
		if f.Value != l.Value && !known[pair{f.Value, l.Holder}] {
			diag.E6002.AtRenumber(f.Span, l.Holder, l.Value.Int).Report(c.bag)
		}
		for _, other := range curValues[l.Value] {
			if other.Holder != l.Holder && !known[pair{other.Value, other.Holder}] {
				diag.E6002.AtCodeTaken(other.Span, l.Value.Int, l.Holder).Report(c.bag)
			}
		}
	}
}

// fieldRules: a @stable value never changes and belongs to its entry only (E6002); an entry
// that is gone is the table fact's E6001.
func fieldRules(c *comparison) {
	now, known, curValues := holders(c.cur), pairs(c.lock), valueIndex(c.cur)
	for _, l := range c.lock {
		f, present := now[l.Holder]
		if !present {
			continue
		}
		if f.Value != l.Value && !known[pair{f.Value, l.Holder}] {
			diag.E6002.AtChanged(f.Span, l.Field, l.Holder, l.Value.arg(), f.Value.arg()).Report(c.bag)
		}
		for _, other := range curValues[l.Value] {
			if other.Holder != l.Holder && !known[pair{other.Value, other.Holder}] {
				diag.E6002.AtValueTaken(other.Span, l.Value.arg(), l.Field, l.Holder).Report(c.bag)
			}
		}
	}
}

// removed reports a locked member gone from the sources, and its code's new holder (LOCK.md §4.1).
func removed(l Fact, cur []Fact, locked map[string]Fact, bag *diag.Bag) {
	for _, c := range cur {
		if _, known := locked[c.Holder]; !known && c.Value == l.Value {
			diag.E6001.AtHeld(l.Span, l.qualified(), diag.KindCode, l.Value.arg(), c.Holder).Report(bag)
			return
		}
	}
	diag.E6001.AtRemoved(l.Span, l.qualified()).Report(bag)
}

// qualified is the locked holder's full name: `teamboard.statuses.wont_do`.
func (fact Fact) qualified() string {
	return fact.Name + nameSep + fact.Holder
}

// arg is the value as a finding's argument, in the canonical text form of Canon values.
func (v Value) arg() value.Value {
	if v.IsString {
		return &value.Str{V: v.Str, T: types.StringType}
	}
	return &value.Int{V: v.Int, T: types.IntType}
}
