package build

import (
	"sync"

	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// lockMemo keeps a generation's lock readings that reported nothing, each serving only the file
// it read, and each stable table's last lock.Order, serving only facts equal to its own.
type lockMemo struct {
	mu     sync.Mutex
	reads  map[string]lockRead    // by the lock's display path
	orders map[string]*lock.Order // by the table's qualified name
}

// lockRead is a canon.lock read whole with no finding, and the file it read.
type lockRead struct {
	src  *source.File
	file *lock.File
}

// read is the kept reading of src under display, if any.
func (m *lockMemo) read(display string, src *source.File) (*lock.File, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept, ok := m.reads[display]
	return kept.file, ok && kept.src == src
}

// keepRead keeps a reading of src that reported nothing; the file is never changed after.
func (m *lockMemo) keepRead(display string, src *source.File, file *lock.File) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reads == nil {
		m.reads = map[string]lockRead{}
	}
	m.reads[display] = lockRead{src: src, file: file}
}

// order is the table's last kept order, nil when none.
func (m *lockMemo) order(name string) *lock.Order {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.orders[name]
}

// keepOrder keeps the table's latest order; a nil one, from facts refused, forgets it.
func (m *lockMemo) keepOrder(name string, o *lock.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.orders == nil {
		m.orders = map[string]*lock.Order{}
	}
	if o == nil {
		delete(m.orders, name)
		return
	}
	m.orders[name] = o
}

// lockMemo is the run's generation's memo, nil for a run without a cache.
func (r *run) lockMemo() *lockMemo {
	if r.s.gen == nil {
		return nil
	}
	return &r.s.gen.locks
}

// parseLock reads a package's canon.lock, or takes the generation's reading of the same file.
func (r *run) parseLock(display string, src *source.File, pkg string) (*lock.File, bool) {
	m := r.lockMemo()
	if m != nil {
		if file, ok := m.read(display, src); ok {
			return file, true
		}
	}
	file, whole := lock.Parse(src.ID, src.Content, pkg, r.bags[pkg])
	if whole && m != nil {
		m.keepRead(display, src, file)
	}
	return file, whole
}

// addTableFacts adds a stable table's facts to s, from the generation's last order of it.
func (r *run) addTableFacts(s *lock.Sources, name string, table *value.Table) error {
	m := r.lockMemo()
	if m == nil {
		_, err := s.AddTable(name, table, nil)
		return err
	}
	o, err := s.AddTable(name, table, m.order(name))
	m.keepOrder(name, o)
	return err
}
