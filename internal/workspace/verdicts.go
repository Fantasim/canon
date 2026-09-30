package workspace

import (
	"container/list"
	"sync"

	"github.com/fantasim/canonlang/internal/edit"
)

// verdictMemo is a project's memo of the texts M9 found fixed, least recently used dropped past verdictLimit.
type verdictMemo struct {
	mu      sync.Mutex
	order   *list.List // most recently used first; each element holds an edit.VerdictKey
	entries map[edit.VerdictKey]*list.Element
}

// Fixed reports that key was kept and marks it most recently used.
func (m *verdictMemo) Fixed(key edit.VerdictKey) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	if ok {
		m.order.MoveToFront(e)
	}
	return ok
}

// Keep records key, dropping the least recently used entry past the bound; one held is left as it is.
func (m *verdictMemo) Keep(key edit.VerdictKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.entries[key]; ok {
		return
	}
	if m.entries == nil {
		m.order, m.entries = list.New(), map[edit.VerdictKey]*list.Element{}
	}
	m.entries[key] = m.order.PushFront(key)
	if m.order.Len() > verdictLimit {
		oldest := m.order.Back()
		m.order.Remove(oldest)
		delete(m.entries, oldest.Value.(edit.VerdictKey))
	}
}
