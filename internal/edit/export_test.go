package edit

import "context"

// RefVisits is how many values Refs visits looking for r's target in s.
func RefVisits(ctx context.Context, s *Snapshot, r Resolved) (int, error) {
	res, err := s.reopen(r)
	if err != nil {
		return 0, err
	}
	tg, ok := s.target(res)
	if !ok {
		return 0, ErrBadOp
	}
	sc := s.newScan(tg)
	err = sc.lets(ctx)
	return sc.visits, err
}
