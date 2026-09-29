package eval

// LoadMemoCounts is the loads e replayed, kept in its memo and ran without keeping them.
func (e *Evaluator) LoadMemoCounts() (hits, stored, unkept int) {
	if e.memo == nil {
		return 0, 0, 0
	}
	s := e.memo.loaded
	return s.hits, s.stored, s.unkept
}
