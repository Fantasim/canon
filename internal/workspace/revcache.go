package workspace

// revised is the revision computed on s, if any: it is a function of the entries s holds, each
// fixed once read, and of the static read set they give, so it never changes (API.md S3,
// DECISIONS 330).
func (s *snapFS) revised() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rev, s.rev != ""
}

// revise keeps rev, computed on s.
func (s *snapFS) revise(rev string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rev = rev
}
