package eval

// LoadMemoCounts is the loads e replayed, kept in its memo and ran without keeping them.
func (e *Evaluator) LoadMemoCounts() (hits, stored, unkept int) {
	if e.memo == nil {
		return 0, 0, 0
	}
	s := e.memo.loaded
	return s.hits, s.stored, s.unkept
}

// PartMemoCounts is the load.dir elements e served from its memo, kept in it and decoded without keeping them.
func (e *Evaluator) PartMemoCounts() (hits, stored, unkept int) {
	if e.memo == nil {
		return 0, 0, 0
	}
	s := e.memo.parted
	return s.hits, s.stored, s.unkept
}
