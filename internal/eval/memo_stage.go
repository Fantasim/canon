package eval

import "slices"

// Stage is a later stage that keeps what it made of an entry on the entry's evaluation, so that
// it lives, is counted and is forgotten with it (log-2026-09-29 M4 P12-r).
type Stage uint8

// stageSlot is what one stage keeps on an entry, and its bytes.
type stageSlot struct {
	v    any
	size int
}

// Attach keeps v, which holds about nodes values, reads and findings, as what stage made of the
// record of token's entry; false, v not kept, when the entry is no longer in a live epoch's store.
func (e *Evaluator) Attach(token any, stage Stage, v any, nodes int) bool {
	en, ok := token.(*memoEntry)
	u := e.memo
	if !ok || u == nil {
		return false
	}
	m, g, size := u.m, u.gen, memoNodeBytes*nodes
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.holds(g, en) {
		return false
	}
	old := en.stages[stage].size
	en.stages[stage], en.size, g.bytes = stageSlot{}, en.size-old, g.bytes-old
	if !m.admit(g, size, false) || !m.holds(g, en) {
		return false
	}
	en.stages[stage] = stageSlot{v: v, size: size}
	en.size += size
	g.bytes += size
	return true
}

// Attached is what stage made of the record of token's entry; nil when none is kept, the entry
// was evicted or its epoch forgotten.
func (e *Evaluator) Attached(token any, stage Stage) any {
	en, ok := token.(*memoEntry)
	u := e.memo
	if !ok || u == nil {
		return nil
	}
	u.m.mu.Lock()
	defer u.m.mu.Unlock()
	if !u.m.holds(u.gen, en) {
		return nil
	}
	return en.stages[stage].v
}

// holds reports g a live store keeping en, an entry's or a load.dir element's; m.mu is held.
func (m *Memo) holds(g *memoGen, en *memoEntry) bool {
	if !slices.Contains(m.gens, g) {
		return false
	}
	if en.part != nil {
		return g.holdsPart(en)
	}
	return g.entries[en.key] == en
}
